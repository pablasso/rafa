import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import {
  loadPlan,
  savePlan,
  listPlans,
  findPlanFolder,
  resolvePlanName,
  createPlanFolder,
  firstPendingTask,
  allTasksCompleted,
  PlanValidationError,
} from "../storage/plans.js";
import type { Plan } from "../core/plan.js";

const TEST_DIR = path.join(process.cwd(), "test-temp-plans");

async function createTestDir() {
  await fs.mkdir(path.join(TEST_DIR, ".rafa", "plans"), { recursive: true });
}

async function cleanupTestDir() {
  try {
    await fs.rm(TEST_DIR, { recursive: true, force: true });
  } catch {
    // Ignore errors
  }
}

function createTestPlan(overrides: Partial<Plan> = {}): Plan {
  return {
    id: "abc123",
    name: "test-plan",
    description: "A test plan",
    sourceFile: "docs/designs/test.md",
    createdAt: "2024-01-01T00:00:00.000Z",
    status: "not_started",
    tasks: [
      {
        id: "t01",
        title: "Task 1",
        description: "First task",
        acceptanceCriteria: ["Criterion 1"],
        status: "pending",
        attempts: 0,
      },
    ],
    ...overrides,
  };
}

describe("plan storage", () => {
  beforeEach(async () => {
    await cleanupTestDir();
    await createTestDir();
  });

  afterEach(async () => {
    await cleanupTestDir();
  });

  describe("savePlan and loadPlan", () => {
    it("saves and loads a plan correctly", async () => {
      const plan = createTestPlan();
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "abc123-test-plan");
      await fs.mkdir(planDir, { recursive: true });

      await savePlan(planDir, plan);
      const loaded = await loadPlan(planDir);

      expect(loaded.id).toBe(plan.id);
      expect(loaded.name).toBe(plan.name);
      expect(loaded.description).toBe(plan.description);
      expect(loaded.sourceFile).toBe(plan.sourceFile);
      expect(loaded.createdAt).toBe(plan.createdAt);
      expect(loaded.status).toBe(plan.status);
      expect(loaded.tasks).toHaveLength(1);
      expect(loaded.tasks[0].id).toBe("t01");
    });

    it("preserves task details through save/load cycle", async () => {
      const plan = createTestPlan({
        tasks: [
          {
            id: "t01",
            title: "Complex Task",
            description: "A task with multiple criteria",
            acceptanceCriteria: ["First", "Second", "Third"],
            status: "in_progress",
            attempts: 2,
          },
        ],
      });
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "abc123-test-plan");
      await fs.mkdir(planDir, { recursive: true });

      await savePlan(planDir, plan);
      const loaded = await loadPlan(planDir);

      expect(loaded.tasks[0].acceptanceCriteria).toEqual([
        "First",
        "Second",
        "Third",
      ]);
      expect(loaded.tasks[0].status).toBe("in_progress");
      expect(loaded.tasks[0].attempts).toBe(2);
    });

    it("throws when plan directory does not exist", async () => {
      await expect(
        loadPlan(path.join(TEST_DIR, "nonexistent"))
      ).rejects.toThrow("Plan not found");
    });
  });

  describe("plan validation", () => {
    it("rejects malformed JSON", async () => {
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "bad-plan");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(path.join(planDir, "plan.json"), "{ invalid json");

      await expect(loadPlan(planDir)).rejects.toThrow(PlanValidationError);
    });

    it("rejects plan with missing id", async () => {
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "bad-plan");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(
        path.join(planDir, "plan.json"),
        JSON.stringify({
          name: "test",
          description: "",
          sourceFile: "",
          createdAt: "",
          status: "not_started",
          tasks: [],
        })
      );

      await expect(loadPlan(planDir)).rejects.toThrow(
        "Plan has invalid or missing 'id'"
      );
    });

    it("rejects plan with invalid status", async () => {
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "bad-plan");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(
        path.join(planDir, "plan.json"),
        JSON.stringify({
          id: "abc",
          name: "test",
          description: "",
          sourceFile: "",
          createdAt: "",
          status: "invalid_status",
          tasks: [],
        })
      );

      await expect(loadPlan(planDir)).rejects.toThrow(
        "Plan has invalid 'status'"
      );
    });

    it("rejects task with missing acceptanceCriteria", async () => {
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "bad-plan");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(
        path.join(planDir, "plan.json"),
        JSON.stringify({
          id: "abc",
          name: "test",
          description: "",
          sourceFile: "",
          createdAt: "",
          status: "not_started",
          tasks: [
            {
              id: "t01",
              title: "Task",
              description: "Desc",
              status: "pending",
              attempts: 0,
            },
          ],
        })
      );

      await expect(loadPlan(planDir)).rejects.toThrow(
        "invalid or missing 'acceptanceCriteria'"
      );
    });

    it("rejects task with invalid status", async () => {
      const planDir = path.join(TEST_DIR, ".rafa", "plans", "bad-plan");
      await fs.mkdir(planDir, { recursive: true });
      await fs.writeFile(
        path.join(planDir, "plan.json"),
        JSON.stringify({
          id: "abc",
          name: "test",
          description: "",
          sourceFile: "",
          createdAt: "",
          status: "not_started",
          tasks: [
            {
              id: "t01",
              title: "Task",
              description: "Desc",
              acceptanceCriteria: [],
              status: "invalid",
              attempts: 0,
            },
          ],
        })
      );

      await expect(loadPlan(planDir)).rejects.toThrow(
        "invalid 'status': invalid"
      );
    });
  });

  describe("listPlans", () => {
    it("returns empty array when no plans directory exists", async () => {
      const emptyDir = path.join(TEST_DIR, "empty");
      await fs.mkdir(emptyDir, { recursive: true });

      const plans = await listPlans(emptyDir);
      expect(plans).toEqual([]);
    });

    it("lists all valid plans", async () => {
      const plan1 = createTestPlan({ id: "aaa111", name: "plan-one" });
      const plan2 = createTestPlan({ id: "bbb222", name: "plan-two" });

      await createPlanFolder(plan1, TEST_DIR);
      await createPlanFolder(plan2, TEST_DIR);

      const plans = await listPlans(TEST_DIR);
      expect(plans).toHaveLength(2);
      expect(plans.map((p) => p.name).sort()).toEqual(["plan-one", "plan-two"]);
    });

    it("skips invalid plan directories", async () => {
      const validPlan = createTestPlan({ id: "valid1", name: "valid-plan" });
      await createPlanFolder(validPlan, TEST_DIR);

      // Create an invalid plan directory
      const invalidDir = path.join(
        TEST_DIR,
        ".rafa",
        "plans",
        "invalid-broken"
      );
      await fs.mkdir(invalidDir, { recursive: true });
      await fs.writeFile(path.join(invalidDir, "plan.json"), "invalid json");

      const plans = await listPlans(TEST_DIR);
      expect(plans).toHaveLength(1);
      expect(plans[0].name).toBe("valid-plan");
    });
  });

  describe("findPlanFolder", () => {
    it("finds plan by name suffix", async () => {
      const plan = createTestPlan({ id: "xyz789", name: "my-feature" });
      await createPlanFolder(plan, TEST_DIR);

      const folder = await findPlanFolder("my-feature", TEST_DIR);
      expect(folder).toContain("xyz789-my-feature");
    });

    it("throws when plan not found", async () => {
      await expect(findPlanFolder("nonexistent", TEST_DIR)).rejects.toThrow(
        "Plan not found: nonexistent"
      );
    });

    it("throws when multiple plans match", async () => {
      const plan1 = createTestPlan({ id: "aaa111", name: "feature" });
      const plan2 = createTestPlan({ id: "bbb222", name: "feature" });

      await createPlanFolder(plan1, TEST_DIR);
      await createPlanFolder(plan2, TEST_DIR);

      await expect(findPlanFolder("feature", TEST_DIR)).rejects.toThrow(
        "Multiple plans match"
      );
    });
  });

  describe("resolvePlanName", () => {
    it("returns baseName when no collision", async () => {
      const name = await resolvePlanName("new-plan", TEST_DIR);
      expect(name).toBe("new-plan");
    });

    it("appends suffix when name exists", async () => {
      const plan = createTestPlan({ id: "aaa111", name: "my-plan" });
      await createPlanFolder(plan, TEST_DIR);

      const name = await resolvePlanName("my-plan", TEST_DIR);
      expect(name).toBe("my-plan-2");
    });

    it("increments suffix for multiple collisions", async () => {
      const plan1 = createTestPlan({ id: "aaa111", name: "my-plan" });
      const plan2 = createTestPlan({ id: "bbb222", name: "my-plan-2" });
      await createPlanFolder(plan1, TEST_DIR);
      await createPlanFolder(plan2, TEST_DIR);

      const name = await resolvePlanName("my-plan", TEST_DIR);
      expect(name).toBe("my-plan-3");
    });
  });

  describe("createPlanFolder", () => {
    it("creates folder with plan.json and log files", async () => {
      const plan = createTestPlan();
      const folder = await createPlanFolder(plan, TEST_DIR);

      expect(folder).toContain("abc123-test-plan");

      const planJson = await fs.readFile(
        path.join(folder, "plan.json"),
        "utf-8"
      );
      expect(JSON.parse(planJson).id).toBe("abc123");

      await fs.access(path.join(folder, "progress.jsonl"));
      await fs.access(path.join(folder, "output.log"));
    });
  });

  describe("firstPendingTask", () => {
    it("returns index of first pending task", () => {
      const plan = createTestPlan({
        tasks: [
          {
            id: "t01",
            title: "Task 1",
            description: "",
            acceptanceCriteria: [],
            status: "completed",
            attempts: 1,
          },
          {
            id: "t02",
            title: "Task 2",
            description: "",
            acceptanceCriteria: [],
            status: "pending",
            attempts: 0,
          },
        ],
      });

      expect(firstPendingTask(plan)).toBe(1);
    });

    it("resets failed task to pending and returns its index", () => {
      const plan = createTestPlan({
        tasks: [
          {
            id: "t01",
            title: "Task 1",
            description: "",
            acceptanceCriteria: [],
            status: "completed",
            attempts: 1,
          },
          {
            id: "t02",
            title: "Task 2",
            description: "",
            acceptanceCriteria: [],
            status: "failed",
            attempts: 3,
          },
        ],
      });

      const idx = firstPendingTask(plan);
      expect(idx).toBe(1);
      expect(plan.tasks[1].status).toBe("pending");
      expect(plan.tasks[1].attempts).toBe(3); // Attempts preserved
    });

    it("returns -1 when all tasks completed", () => {
      const plan = createTestPlan({
        tasks: [
          {
            id: "t01",
            title: "Task 1",
            description: "",
            acceptanceCriteria: [],
            status: "completed",
            attempts: 1,
          },
        ],
      });

      expect(firstPendingTask(plan)).toBe(-1);
    });
  });

  describe("allTasksCompleted", () => {
    it("returns true when all tasks completed", () => {
      const plan = createTestPlan({
        tasks: [
          {
            id: "t01",
            title: "Task 1",
            description: "",
            acceptanceCriteria: [],
            status: "completed",
            attempts: 1,
          },
          {
            id: "t02",
            title: "Task 2",
            description: "",
            acceptanceCriteria: [],
            status: "completed",
            attempts: 1,
          },
        ],
      });

      expect(allTasksCompleted(plan)).toBe(true);
    });

    it("returns false when any task not completed", () => {
      const plan = createTestPlan({
        tasks: [
          {
            id: "t01",
            title: "Task 1",
            description: "",
            acceptanceCriteria: [],
            status: "completed",
            attempts: 1,
          },
          {
            id: "t02",
            title: "Task 2",
            description: "",
            acceptanceCriteria: [],
            status: "pending",
            attempts: 0,
          },
        ],
      });

      expect(allTasksCompleted(plan)).toBe(false);
    });
  });
});
