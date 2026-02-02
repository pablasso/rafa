import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import * as childProcess from "node:child_process";
import { EventEmitter } from "node:events";
import { extractTasks, TaskExtractionError } from "../core/extract-tasks.js";

// Mock child_process
vi.mock("node:child_process", () => ({
  spawn: vi.fn(),
}));

const mockSpawn = vi.mocked(childProcess.spawn);

class MockProcess extends EventEmitter {
  stdout = new EventEmitter();
  stderr = new EventEmitter();
  killed = false;

  kill() {
    this.killed = true;
  }
}

describe("extractTasks", () => {
  let mockProcess: MockProcess;

  beforeEach(() => {
    mockProcess = new MockProcess();
    mockSpawn.mockReturnValue(mockProcess as any);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("extracts tasks from valid Claude response", async () => {
    const extractPromise = extractTasks("# Design Doc\n\nSome content");

    // Simulate Claude CLI response
    const response = JSON.stringify({
      type: "result",
      result: JSON.stringify({
        name: "my-feature",
        description: "Implement a new feature",
        tasks: [
          {
            title: "Implement core logic",
            description: "Build the main functionality",
            acceptanceCriteria: ["Tests pass", "Types check"],
          },
        ],
      }),
      is_error: false,
    });

    // Emit response and close
    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    const result = await extractPromise;

    expect(result.name).toBe("my-feature");
    expect(result.description).toBe("Implement a new feature");
    expect(result.tasks).toHaveLength(1);
    expect(result.tasks[0].title).toBe("Implement core logic");
    expect(result.tasks[0].acceptanceCriteria).toEqual([
      "Tests pass",
      "Types check",
    ]);
  });

  it("handles raw JSON response (not wrapped)", async () => {
    const extractPromise = extractTasks("# Design");

    const response = JSON.stringify({
      name: "raw-feature",
      description: "A raw response",
      tasks: [
        {
          title: "Task 1",
          description: "Do something",
          acceptanceCriteria: ["Criterion 1"],
        },
      ],
    });

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    const result = await extractPromise;
    expect(result.name).toBe("raw-feature");
  });

  it("handles response with markdown code blocks", async () => {
    const extractPromise = extractTasks("# Design");

    const jsonContent = JSON.stringify({
      name: "markdown-feature",
      description: "Wrapped in markdown",
      tasks: [
        {
          title: "Task 1",
          description: "Do something",
          acceptanceCriteria: ["Criterion 1"],
        },
      ],
    });
    const response = "```json\n" + jsonContent + "\n```";

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    const result = await extractPromise;
    expect(result.name).toBe("markdown-feature");
  });

  it("throws TaskExtractionError when Claude CLI not found", async () => {
    const extractPromise = extractTasks("# Design");

    const error = new Error("spawn claude ENOENT") as NodeJS.ErrnoException;
    error.code = "ENOENT";
    mockProcess.emit("error", error);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("Claude Code CLI not found");
  });

  it("throws TaskExtractionError when Claude returns error", async () => {
    const extractPromise = extractTasks("# Design");

    const response = JSON.stringify({
      type: "result",
      result: "Something went wrong",
      is_error: true,
    });

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("Claude returned an error");
  });

  it("throws TaskExtractionError when Claude CLI exits non-zero", async () => {
    const extractPromise = extractTasks("# Design");

    mockProcess.stderr.emit("data", Buffer.from("Error: API limit exceeded"));
    mockProcess.emit("close", 1);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("Claude command failed");
  });

  it("throws TaskExtractionError for invalid JSON response", async () => {
    const extractPromise = extractTasks("# Design");

    mockProcess.stdout.emit("data", Buffer.from("not json at all"));
    mockProcess.emit("close", 0);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("No JSON object found");
  });

  it("throws TaskExtractionError when name is missing", async () => {
    const extractPromise = extractTasks("# Design");

    const response = JSON.stringify({
      description: "Missing name",
      tasks: [],
    });

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("invalid or missing 'name'");
  });

  it("throws TaskExtractionError when tasks array is empty", async () => {
    const extractPromise = extractTasks("# Design");

    const response = JSON.stringify({
      name: "empty-tasks",
      description: "No tasks",
      tasks: [],
    });

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("invalid or empty 'tasks'");
  });

  it("throws TaskExtractionError when task is missing title", async () => {
    const extractPromise = extractTasks("# Design");

    const response = JSON.stringify({
      name: "bad-task",
      description: "Task missing title",
      tasks: [
        {
          description: "No title here",
          acceptanceCriteria: ["Criterion"],
        },
      ],
    });

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("invalid or missing 'title'");
  });

  it("throws TaskExtractionError when task has empty acceptanceCriteria", async () => {
    const extractPromise = extractTasks("# Design");

    const response = JSON.stringify({
      name: "no-criteria",
      description: "Task with no criteria",
      tasks: [
        {
          title: "Task 1",
          description: "Has no criteria",
          acceptanceCriteria: [],
        },
      ],
    });

    mockProcess.stdout.emit("data", Buffer.from(response));
    mockProcess.emit("close", 0);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow(
      "invalid or empty 'acceptanceCriteria'",
    );
  });

  it("times out when Claude takes too long", async () => {
    vi.useFakeTimers();

    const extractPromise = extractTasks("# Design", 1000); // 1 second timeout

    // Advance time past the timeout
    vi.advanceTimersByTime(1500);

    await expect(extractPromise).rejects.toThrow(TaskExtractionError);
    await expect(extractPromise).rejects.toThrow("timed out");

    vi.useRealTimers();
  });

  it("calls spawn with correct arguments", async () => {
    const extractPromise = extractTasks("# My Design\n\nContent here");

    mockProcess.stdout.emit(
      "data",
      Buffer.from(
        JSON.stringify({
          name: "test",
          description: "Test",
          tasks: [
            {
              title: "T1",
              description: "D1",
              acceptanceCriteria: ["C1"],
            },
          ],
        }),
      ),
    );
    mockProcess.emit("close", 0);

    await extractPromise;

    expect(mockSpawn).toHaveBeenCalledWith(
      "claude",
      expect.arrayContaining([
        "-p",
        expect.stringContaining("My Design"),
        "--output-format",
        "json",
        "--dangerously-skip-permissions",
      ]),
      expect.objectContaining({
        stdio: ["ignore", "pipe", "pipe"],
      }),
    );
  });
});
