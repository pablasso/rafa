import { describe, it, expect, beforeEach, vi } from "vitest";
import { RunView, type RunContext } from "../tui/views/run.js";
import type { RafaApp } from "../tui/app.js";
import type { Plan } from "../core/plan.js";
import type { Task } from "../core/task.js";
import type {
  ClaudeEvent,
  ToolUseEventData,
  ToolResultEventData,
  TextEventData,
} from "../core/stream-parser.js";

// Mock RafaApp
function createMockApp(): RafaApp {
  return {
    navigate: vi.fn(),
    requestRender: vi.fn(),
    getCurrentView: vi.fn(() => "run"),
  } as unknown as RafaApp;
}

// Create test tasks
function createTestTasks(): Task[] {
  return [
    {
      id: "t01",
      title: "Set up project",
      description: "Initialize project structure",
      acceptanceCriteria: ["Project compiles"],
      status: "completed",
      attempts: 1,
    },
    {
      id: "t02",
      title: "Implement core feature",
      description: "Build the main functionality",
      acceptanceCriteria: ["Feature works"],
      status: "in_progress",
      attempts: 2,
    },
    {
      id: "t03",
      title: "Add tests",
      description: "Write unit tests",
      acceptanceCriteria: ["Tests pass"],
      status: "pending",
      attempts: 0,
    },
  ];
}

// Create test plan
function createTestPlan(): Plan {
  return {
    id: "abc123",
    name: "test-feature",
    description: "A test feature",
    sourceFile: "docs/designs/test.md",
    createdAt: new Date().toISOString(),
    status: "in_progress",
    tasks: createTestTasks(),
  };
}

describe("RunView", () => {
  let view: RunView;
  let mockApp: RafaApp;

  beforeEach(() => {
    mockApp = createMockApp();
    view = new RunView(mockApp);
  });

  describe("activation", () => {
    it("initializes with plan context", () => {
      const plan = createTestPlan();
      const context: RunContext = {
        planId: plan.id,
        planName: plan.name,
        planDir: "/test/plans/abc123-test-feature",
        plan,
      };

      view.activate(context);

      // Should be able to render without errors
      const lines = view.render(100);
      expect(lines.length).toBeGreaterThan(0);
      expect(lines.some((l) => l.includes("test-feature"))).toBe(true);
    });

    it("clears previous state on activation", () => {
      // Add some events
      view.processEvent({
        type: "text",
        data: { text: "Some output", isPartial: false } as TextEventData,
        raw: {} as any,
      });

      // Reactivate
      view.activate({ plan: createTestPlan() });

      // Activity log and output should be clear
      expect(view.getActivityLog().getEvents()).toHaveLength(0);
      expect(view.getOutputStream().getContent()).toBe("");
    });
  });

  describe("split-pane layout", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("renders task progress in left pane", () => {
      const lines = view.render(100);
      const content = lines.join("\n");

      // Should show tasks with their IDs
      expect(content).toContain("t01");
      expect(content).toContain("t02");
      expect(content).toContain("t03");
    });

    it("renders activity log section", () => {
      const lines = view.render(100);
      const content = lines.join("\n");

      // Should have Activity section header
      expect(content).toContain("Activity");
    });

    it("renders output section", () => {
      const lines = view.render(100);
      const content = lines.join("\n");

      // Should have Output section header
      expect(content).toContain("Output");
    });

    it("uses vertical separator between panes", () => {
      const lines = view.render(100);

      // Should have │ separator in content lines
      const contentLines = lines.filter((l) => l.includes("│"));
      expect(contentLines.length).toBeGreaterThan(0);
    });

    it("renders header with plan info", () => {
      const lines = view.render(100);
      const header = lines[0];

      expect(header).toContain("test-feature");
      expect(header).toContain("abc123");
    });

    it("renders footer with controls", () => {
      const lines = view.render(100);
      const footer = lines[lines.length - 1];

      expect(footer).toContain("Esc");
      expect(footer).toContain("Ctrl+C");
    });
  });

  describe("real-time updates", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("updates activity log when tool_use event arrives", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "toolu_123",
          name: "Read",
          input: { file_path: "/test/file.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      };

      view.processEvent(event);

      const events = view.getActivityLog().getEvents();
      expect(events).toHaveLength(1);
      expect(events[0].name).toBe("Read");
      expect(events[0].status).toBe("running");
    });

    it("updates activity log when tool_result event arrives", () => {
      // First add tool use
      view.processEvent({
        type: "tool_use",
        data: {
          id: "toolu_456",
          name: "Write",
          input: { file_path: "/test/output.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      });

      // Then complete it
      view.processEvent({
        type: "tool_result",
        data: {
          toolUseId: "toolu_456",
          content: "Done",
          isError: false,
        } as ToolResultEventData,
        raw: {} as any,
      });

      const events = view.getActivityLog().getEvents();
      expect(events[0].status).toBe("done");
    });

    it("updates output stream when text event arrives", () => {
      const event: ClaudeEvent = {
        type: "text",
        data: {
          text: "Processing files...",
          isPartial: false,
        } as TextEventData,
        raw: {} as any,
      };

      view.processEvent(event);

      expect(view.getOutputStream().getContent()).toBe("Processing files...");
    });

    it("appends multiple text events to output stream", () => {
      view.processEvent({
        type: "text",
        data: { text: "Line 1\n", isPartial: false } as TextEventData,
        raw: {} as any,
      });
      view.processEvent({
        type: "text",
        data: { text: "Line 2\n", isPartial: false } as TextEventData,
        raw: {} as any,
      });

      const content = view.getOutputStream().getContent();
      expect(content).toContain("Line 1");
      expect(content).toContain("Line 2");
    });
  });

  describe("current task highlighting", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("highlights the current task", () => {
      view.setCurrentTask("t02");

      const lines = view.render(100);
      const content = lines.join("\n");

      // Should have a marker for current task
      // The task progress component uses "> " prefix for current task
      expect(content).toContain("> ");
    });

    it("updates highlight when current task changes", () => {
      view.setCurrentTask("t02");
      let lines = view.render(100);
      let content = lines.join("\n");

      // Find line with t02 - it should be marked
      expect(content).toContain("t02");

      // Change current task
      view.setCurrentTask("t03");
      lines = view.render(100);
      content = lines.join("\n");

      // t03 should now be the current task
      expect(content).toContain("t03");
    });
  });

  describe("task status updates", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("updates task list when tasks change", () => {
      const updatedTasks: Task[] = [
        {
          id: "t01",
          title: "Set up project",
          description: "Initialize project structure",
          acceptanceCriteria: ["Project compiles"],
          status: "completed",
          attempts: 1,
        },
        {
          id: "t02",
          title: "Implement core feature",
          description: "Build the main functionality",
          acceptanceCriteria: ["Feature works"],
          status: "completed", // Changed from in_progress
          attempts: 2,
        },
        {
          id: "t03",
          title: "Add tests",
          description: "Write unit tests",
          acceptanceCriteria: ["Tests pass"],
          status: "in_progress", // Changed from pending
          attempts: 1,
        },
      ];

      view.updateTasks(updatedTasks);

      const lines = view.render(100);
      const content = lines.join("\n");

      // Should show updated status - completed tasks show ✓
      expect(content).toContain("✓");
    });
  });

  describe("running state", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("shows running status when execution is active", () => {
      view.setRunning(true);

      const lines = view.render(100);
      const content = lines.join("\n");

      expect(content).toContain("Running");
    });

    it("shows stop control when running", () => {
      view.setRunning(true);

      const lines = view.render(100);
      const footer = lines[lines.length - 1];

      expect(footer).toContain("Stop");
    });

    it("shows navigation controls when not running", () => {
      view.setRunning(false);

      const lines = view.render(100);
      const footer = lines[lines.length - 1];

      expect(footer).toContain("Esc");
      expect(footer).toContain("Back");
    });
  });

  describe("activity log clearing", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("clears activity log for new task attempt", () => {
      // Add some events
      view.processEvent({
        type: "tool_use",
        data: {
          id: "toolu_old",
          name: "Read",
          input: { file_path: "/old.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      });

      expect(view.getActivityLog().getEvents()).toHaveLength(1);

      // Clear for new attempt
      view.clearActivityLog();

      expect(view.getActivityLog().getEvents()).toHaveLength(0);
    });
  });

  describe("render dimensions", () => {
    beforeEach(() => {
      view.activate({ plan: createTestPlan() });
    });

    it("renders within specified width", () => {
      const width = 80;
      const lines = view.render(width);

      // All lines should be within width (accounting for some variance in rendering)
      for (const line of lines) {
        // Allow some flexibility for Unicode characters
        expect(line.length).toBeLessThanOrEqual(width + 5);
      }
    });

    it("handles narrow widths gracefully", () => {
      const width = 60;
      const lines = view.render(width);

      // Should still render something
      expect(lines.length).toBeGreaterThan(0);
    });

    it("handles wide widths gracefully", () => {
      const width = 200;
      const lines = view.render(width);

      // Should still render something
      expect(lines.length).toBeGreaterThan(0);
    });
  });
});
