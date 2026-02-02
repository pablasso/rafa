import { describe, it, expect } from "vitest";
import { createPlan } from "../core/plan.js";
import { createTask } from "../core/task.js";

describe("createPlan", () => {
  it("creates a plan with correct properties", () => {
    const task = createTask("t01", "Task 1", "Description", ["Criterion"]);
    const plan = createPlan(
      "abc123",
      "test-plan",
      "A test plan",
      "docs/designs/test.md",
      [task]
    );

    expect(plan.id).toBe("abc123");
    expect(plan.name).toBe("test-plan");
    expect(plan.description).toBe("A test plan");
    expect(plan.sourceFile).toBe("docs/designs/test.md");
    expect(plan.status).toBe("not_started");
    expect(plan.tasks).toHaveLength(1);
    expect(plan.createdAt).toMatch(/^\d{4}-\d{2}-\d{2}T/);
  });
});
