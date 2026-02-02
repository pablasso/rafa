import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { Executor, MAX_ATTEMPTS, type ExecutorEvents } from "../core/executor.js";
import { createPlan } from "../core/plan.js";
import { createTask, type Task } from "../core/task.js";
import type { Plan } from "../core/plan.js";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import * as os from "node:os";

// Mock the claude-runner module
vi.mock("../core/claude-runner.js", () => ({
  runTask: vi.fn(),
}));

// Mock git operations
vi.mock("../utils/git.js", () => ({
  getStatus: vi.fn(),
  commitAll: vi.fn(),
}));

// Import after mocking
import { runTask } from "../core/claude-runner.js";
import * as git from "../utils/git.js";

describe("Executor", () => {
  let tempDir: string;
  let planDir: string;
  let plan: Plan;

  beforeEach(async () => {
    // Create temp directory for tests
    tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "rafa-executor-test-"));
    planDir = path.join(tempDir, ".rafa", "plans", "abc123-test-plan");
    await fs.mkdir(planDir, { recursive: true });

    // Create test plan with tasks
    plan = createPlan(
      "abc123",
      "test-plan",
      "A test plan",
      "docs/designs/test.md",
      [
        createTask("t01", "First task", "Do the first thing", ["Criterion 1"]),
        createTask("t02", "Second task", "Do the second thing", ["Criterion 2"]),
      ]
    );

    // Reset mocks
    vi.resetAllMocks();

    // Default mock implementations
    vi.mocked(git.getStatus).mockResolvedValue({ clean: true, files: [] });
    vi.mocked(git.commitAll).mockResolvedValue(undefined);
    vi.mocked(runTask).mockResolvedValue(undefined);
  });

  afterEach(async () => {
    // Cleanup temp directory
    try {
      await fs.rm(tempDir, { recursive: true, force: true });
    } catch {
      // Ignore cleanup errors
    }
  });

  describe("MAX_ATTEMPTS constant", () => {
    it("equals 5", () => {
      expect(MAX_ATTEMPTS).toBe(5);
    });
  });

  describe("run()", () => {
    it("executes tasks sequentially", async () => {
      const taskOrder: string[] = [];
      const events: ExecutorEvents = {
        onTaskStart: (_idx, _total, task) => {
          taskOrder.push(task.id);
        },
      };

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
        events,
      });

      await executor.run();

      expect(taskOrder).toEqual(["t01", "t02"]);
    });

    it("skips completed tasks", async () => {
      // Mark first task as completed
      plan.tasks[0].status = "completed";

      const taskOrder: string[] = [];
      const events: ExecutorEvents = {
        onTaskStart: (_idx, _total, task) => {
          taskOrder.push(task.id);
        },
      };

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
        events,
      });

      await executor.run();

      expect(taskOrder).toEqual(["t02"]);
    });

    it("emits onPlanComplete when all tasks succeed", async () => {
      let completed = false;
      let completedCount = 0;
      let totalCount = 0;

      const events: ExecutorEvents = {
        onPlanComplete: (c, t) => {
          completed = true;
          completedCount = c;
          totalCount = t;
        },
      };

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
        events,
      });

      await executor.run();

      expect(completed).toBe(true);
      expect(completedCount).toBe(2);
      expect(totalCount).toBe(2);
    });

    it("returns early when all tasks already completed", async () => {
      plan.tasks[0].status = "completed";
      plan.tasks[1].status = "completed";

      let planCompleteEmitted = false;
      const events: ExecutorEvents = {
        onPlanComplete: () => {
          planCompleteEmitted = true;
        },
        onTaskStart: () => {
          throw new Error("Should not start any tasks");
        },
      };

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
        events,
      });

      await executor.run();

      expect(planCompleteEmitted).toBe(true);
      expect(runTask).not.toHaveBeenCalled();
    });
  });

  describe("retry logic", () => {
    it("retries failed tasks up to MAX_ATTEMPTS times", async () => {
      let attempts = 0;
      vi.mocked(runTask).mockImplementation(async () => {
        attempts++;
        if (attempts < MAX_ATTEMPTS) {
          throw new Error("Task failed");
        }
        // Succeed on final attempt
      });

      const executor = new Executor({
        planDir,
        plan: createPlan("abc123", "test-plan", "desc", "source.md", [
          createTask("t01", "Flaky task", "Sometimes fails", ["Criterion"]),
        ]),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
      });

      await executor.run();

      expect(attempts).toBe(MAX_ATTEMPTS);
    });

    it("fails after MAX_ATTEMPTS exhausted", async () => {
      vi.mocked(runTask).mockRejectedValue(new Error("Always fails"));

      let failedTask: Task | undefined;
      let failedReason = "";
      const events: ExecutorEvents = {
        onPlanFailed: (task, reason) => {
          failedTask = task;
          failedReason = reason;
        },
      };

      const executor = new Executor({
        planDir,
        plan: createPlan("abc123", "test-plan", "desc", "source.md", [
          createTask("t01", "Failing task", "Always fails", ["Criterion"]),
        ]),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
        events,
      });

      await expect(executor.run()).rejects.toThrow(
        "task t01 failed after 5 attempts"
      );

      expect(failedTask?.id).toBe("t01");
      expect(failedReason).toBe("failed after 5 attempts");
    });

    it("starts each retry with fresh Claude session (no --resume)", async () => {
      const promptsSeen: string[] = [];
      vi.mocked(runTask).mockImplementation(async (options) => {
        promptsSeen.push(options.prompt);
        if (promptsSeen.length < 3) {
          throw new Error("Retry needed");
        }
      });

      const executor = new Executor({
        planDir,
        plan: createPlan("abc123", "test-plan", "desc", "source.md", [
          createTask("t01", "Task", "Description", ["Criterion"]),
        ]),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
      });

      await executor.run();

      // Each attempt should have received its own prompt (fresh session)
      expect(promptsSeen.length).toBe(3);
      expect(promptsSeen[0]).toContain("**Attempt**: 1 of 5");
      expect(promptsSeen[1]).toContain("**Attempt**: 2 of 5");
      expect(promptsSeen[2]).toContain("**Attempt**: 3 of 5");

      // Retry prompts should include the retry note
      expect(promptsSeen[1]).toContain("Previous attempts to complete this task failed");
      expect(promptsSeen[2]).toContain("Previous attempts to complete this task failed");
    });
  });

  describe("workspace cleanliness", () => {
    it("rejects dirty workspace by default", async () => {
      vi.mocked(git.getStatus).mockResolvedValue({
        clean: false,
        files: ["modified-file.ts"],
      });

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: false,
      });

      await expect(executor.run()).rejects.toThrow(
        /workspace has uncommitted changes/
      );
    });

    it("allows dirty workspace with allowDirty option", async () => {
      vi.mocked(git.getStatus).mockResolvedValue({
        clean: false,
        files: ["modified-file.ts"],
      });

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
      });

      // Should not throw
      await executor.run();
    });

    it("ignores lock file when checking workspace cleanliness", async () => {
      let callCount = 0;
      // First call (before execution): return only lock file as modified
      // Subsequent calls (after commit): return clean
      vi.mocked(git.getStatus).mockImplementation(async () => {
        callCount++;
        if (callCount === 1) {
          return {
            clean: false,
            files: [".rafa/plans/abc123-test-plan/run.lock"],
          };
        }
        return { clean: true, files: [] };
      });
      // Ensure runTask succeeds
      vi.mocked(runTask).mockResolvedValue(undefined);

      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: false,
      });

      // Should not throw - lock file is filtered out on initial check
      await executor.run();
    });
  });

  describe("auto-commit", () => {
    it("commits with [rafa] prefix on task success", async () => {
      const executor = new Executor({
        planDir,
        plan: createPlan("abc123", "test-plan", "desc", "source.md", [
          createTask("t01", "My task", "Description", ["Criterion"]),
        ]),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: false,
      });

      await executor.run();

      expect(git.commitAll).toHaveBeenCalledWith(
        "[rafa] Complete task t01: My task",
        tempDir
      );
    });

    it("extracts commit message from SUGGESTED_COMMIT_MESSAGE", async () => {
      vi.mocked(runTask).mockImplementation(async (options) => {
        // Simulate Claude outputting a suggested commit message
        // ClaudeEvent has type "text" with TextEventData
        const event = {
          type: "text" as const,
          data: {
            text: "Done! SUGGESTED_COMMIT_MESSAGE: [rafa] Add new feature X\nAll criteria met.",
            isPartial: false,
          },
          raw: {} as never,
        };
        options.onEvent(event);
      });

      const executor = new Executor({
        planDir,
        plan: createPlan("abc123", "test-plan", "desc", "source.md", [
          createTask("t01", "Task", "Description", ["Criterion"]),
        ]),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: false,
      });

      await executor.run();

      expect(git.commitAll).toHaveBeenCalledWith(
        "[rafa] Add new feature X",
        tempDir
      );
    });

    it("skips commit when allowDirty is true", async () => {
      const executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
      });

      await executor.run();

      expect(git.commitAll).not.toHaveBeenCalled();
    });
  });

  describe("prompt template", () => {
    it("matches Go version format", async () => {
      let capturedPrompt = "";
      vi.mocked(runTask).mockImplementation(async (options) => {
        capturedPrompt = options.prompt;
      });

      const executor = new Executor({
        planDir,
        plan: createPlan(
          "abc123",
          "test-plan",
          "A test description",
          "docs/designs/test.md",
          [
            createTask("t01", "First task", "Task description here", [
              "First criterion",
              "Second criterion",
            ]),
          ]
        ),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
      });

      await executor.run();

      // Check required sections from Go version (runner.go lines 54-94)
      expect(capturedPrompt).toContain("You are executing a task as part of an automated plan.");
      expect(capturedPrompt).toContain("## Context");
      expect(capturedPrompt).toContain("Plan: test-plan");
      expect(capturedPrompt).toContain("Description: A test description");
      expect(capturedPrompt).toContain("Source: docs/designs/test.md");
      expect(capturedPrompt).toContain("## Your Task");
      expect(capturedPrompt).toContain("**ID**: t01");
      expect(capturedPrompt).toContain("**Title**: First task");
      expect(capturedPrompt).toContain("**Attempt**: 1 of 5");
      expect(capturedPrompt).toContain("**Description**: Task description here");
      expect(capturedPrompt).toContain("## Acceptance Criteria");
      expect(capturedPrompt).toContain("1. First criterion");
      expect(capturedPrompt).toContain("2. Second criterion");
      expect(capturedPrompt).toContain("## Instructions");
      expect(capturedPrompt).toContain("DO NOT commit your changes");
      expect(capturedPrompt).toContain("SUGGESTED_COMMIT_MESSAGE:");
    });

    it("includes retry note on subsequent attempts", async () => {
      let capturedPrompts: string[] = [];
      vi.mocked(runTask).mockImplementation(async (options) => {
        capturedPrompts.push(options.prompt);
        if (capturedPrompts.length < 2) {
          throw new Error("Retry");
        }
      });

      const executor = new Executor({
        planDir,
        plan: createPlan("abc123", "test-plan", "desc", "source.md", [
          createTask("t01", "Task", "Description", ["Criterion"]),
        ]),
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
      });

      await executor.run();

      // First attempt should not have retry note
      expect(capturedPrompts[0]).not.toContain("Previous attempts to complete this task failed");

      // Second attempt should have retry note
      expect(capturedPrompts[1]).toContain("Previous attempts to complete this task failed");
      expect(capturedPrompts[1]).toContain("git status");
      expect(capturedPrompts[1]).toContain("git diff");
    });
  });

  describe("abort", () => {
    it("can be aborted mid-execution", async () => {
      let runTaskCallCount = 0;
      let executor: Executor;

      vi.mocked(runTask).mockImplementation(async () => {
        runTaskCallCount++;
        // Simulate some work
        await new Promise((resolve) => setTimeout(resolve, 10));
      });

      const events: ExecutorEvents = {
        onTaskStart: (_idx, _total, task) => {
          // Abort after first task starts (before second)
          if (task.id === "t01") {
            executor.abort();
          }
        },
      };

      executor = new Executor({
        planDir,
        plan,
        repoRoot: tempDir,
        skipPersistence: true,
        allowDirty: true,
        events,
      });

      // The first task will complete (abort is checked after task runs),
      // but second task won't start because abort is checked before each task
      await executor.run();

      // First task runs, then abort prevents second task from running
      expect(runTaskCallCount).toBe(1);
    });
  });
});
