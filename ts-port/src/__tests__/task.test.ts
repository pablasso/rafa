import { describe, it, expect } from "vitest";
import { createTask } from "../core/task.js";

describe("createTask", () => {
  it("creates a task with correct properties", () => {
    const task = createTask(
      "t01",
      "Test Task",
      "A test task description",
      ["Criterion 1", "Criterion 2"]
    );

    expect(task.id).toBe("t01");
    expect(task.title).toBe("Test Task");
    expect(task.description).toBe("A test task description");
    expect(task.acceptanceCriteria).toEqual(["Criterion 1", "Criterion 2"]);
    expect(task.status).toBe("pending");
    expect(task.attempts).toBe(0);
  });
});
