import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import {
  ProgressLogger,
  EventType,
  type ProgressEvent,
} from "../core/progress.js";

describe("ProgressLogger", () => {
  let testDir: string;
  let progressFilePath: string;

  beforeEach(() => {
    testDir = fs.mkdtempSync(path.join(os.tmpdir(), "progress-test-"));
    progressFilePath = path.join(testDir, "progress.jsonl");
  });

  afterEach(() => {
    fs.rmSync(testDir, { recursive: true, force: true });
  });

  /**
   * Helper to read all events from the progress file
   */
  function readEvents(): ProgressEvent[] {
    if (!fs.existsSync(progressFilePath)) {
      return [];
    }
    const content = fs.readFileSync(progressFilePath, "utf-8");
    return content
      .split("\n")
      .filter((line) => line.trim())
      .map((line) => JSON.parse(line));
  }

  describe("log", () => {
    it("creates progress.jsonl file on first write", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.log(EventType.PlanStarted, { plan_id: "test-123" });

      expect(fs.existsSync(progressFilePath)).toBe(true);
    });

    it("writes events as JSONL (one JSON object per line)", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.log(EventType.PlanStarted, { plan_id: "p1" });
      await logger.log(EventType.TaskStarted, { task_id: "t1", attempt: 1 });

      const content = fs.readFileSync(progressFilePath, "utf-8");
      const lines = content.split("\n").filter((l) => l.trim());
      expect(lines).toHaveLength(2);

      // Each line should be valid JSON
      expect(() => JSON.parse(lines[0])).not.toThrow();
      expect(() => JSON.parse(lines[1])).not.toThrow();
    });

    it("includes timestamp in ISO format", async () => {
      const before = new Date().toISOString();
      const logger = new ProgressLogger(testDir);
      await logger.log(EventType.PlanStarted, { plan_id: "test" });
      const after = new Date().toISOString();

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].timestamp).toMatch(/^\d{4}-\d{2}-\d{2}T/);
      expect(events[0].timestamp >= before).toBe(true);
      expect(events[0].timestamp <= after).toBe(true);
    });

    it("appends events (does not overwrite)", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.log(EventType.TaskStarted, { task_id: "t1", attempt: 1 });
      await logger.log(EventType.TaskCompleted, { task_id: "t1" });
      await logger.log(EventType.TaskStarted, { task_id: "t2", attempt: 1 });

      const events = readEvents();
      expect(events).toHaveLength(3);
      expect(events[0].event).toBe("task_started");
      expect(events[1].event).toBe("task_completed");
      expect(events[2].event).toBe("task_started");
    });

    it("preserves data from multiple logger instances", async () => {
      const logger1 = new ProgressLogger(testDir);
      await logger1.log(EventType.PlanStarted, { plan_id: "p1" });

      // Simulate a new process reading the same file
      const logger2 = new ProgressLogger(testDir);
      await logger2.log(EventType.TaskStarted, { task_id: "t1", attempt: 1 });

      const events = readEvents();
      expect(events).toHaveLength(2);
    });
  });

  describe("event helper methods", () => {
    it("planStarted logs plan_started with plan_id", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.planStarted("abc123");

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("plan_started");
      expect(events[0].data).toEqual({ plan_id: "abc123" });
    });

    it("taskStarted logs task_started with task_id and attempt", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.taskStarted("t01", 2);

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("task_started");
      expect(events[0].data).toEqual({ task_id: "t01", attempt: 2 });
    });

    it("taskCompleted logs task_completed with task_id", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.taskCompleted("t01");

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("task_completed");
      expect(events[0].data).toEqual({ task_id: "t01" });
    });

    it("taskFailed logs task_failed with task_id and attempt", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.taskFailed("t01", 3);

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("task_failed");
      expect(events[0].data).toEqual({ task_id: "t01", attempt: 3 });
    });

    it("planCompleted logs plan_completed with statistics", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.planCompleted(10, 8, 12500);

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("plan_completed");
      expect(events[0].data).toEqual({
        total_tasks: 10,
        succeeded_tasks: 8,
        duration_ms: 12500,
      });
    });

    it("planCancelled logs plan_cancelled with last_task_id", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.planCancelled("t05");

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("plan_cancelled");
      expect(events[0].data).toEqual({ last_task_id: "t05" });
    });

    it("planFailed logs plan_failed with task_id and attempts", async () => {
      const logger = new ProgressLogger(testDir);
      await logger.planFailed("t03", 5);

      const events = readEvents();
      expect(events).toHaveLength(1);
      expect(events[0].event).toBe("plan_failed");
      expect(events[0].data).toEqual({ task_id: "t03", attempts: 5 });
    });
  });

  describe("getFilePath", () => {
    it("returns the path to progress.jsonl", () => {
      const logger = new ProgressLogger(testDir);
      expect(logger.getFilePath()).toBe(progressFilePath);
    });
  });

  describe("full execution flow", () => {
    it("logs a complete plan execution sequence", async () => {
      const logger = new ProgressLogger(testDir);

      // Simulate a plan run
      await logger.planStarted("plan-123");
      await logger.taskStarted("t01", 1);
      await logger.taskCompleted("t01");
      await logger.taskStarted("t02", 1);
      await logger.taskFailed("t02", 1);
      await logger.taskStarted("t02", 2);
      await logger.taskCompleted("t02");
      await logger.planCompleted(2, 2, 5000);

      const events = readEvents();
      expect(events).toHaveLength(8);
      expect(events.map((e) => e.event)).toEqual([
        "plan_started",
        "task_started",
        "task_completed",
        "task_started",
        "task_failed",
        "task_started",
        "task_completed",
        "plan_completed",
      ]);
    });
  });
});

describe("EventType constants", () => {
  it("has correct event type values matching Go version", () => {
    expect(EventType.PlanStarted).toBe("plan_started");
    expect(EventType.PlanCompleted).toBe("plan_completed");
    expect(EventType.PlanCancelled).toBe("plan_cancelled");
    expect(EventType.PlanFailed).toBe("plan_failed");
    expect(EventType.TaskStarted).toBe("task_started");
    expect(EventType.TaskCompleted).toBe("task_completed");
    expect(EventType.TaskFailed).toBe("task_failed");
  });
});
