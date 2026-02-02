import { describe, it, expect, beforeEach } from "vitest";
import {
  ActivityLogComponent,
  type ActivityEvent,
} from "../tui/components/activity-log.js";
import type { ClaudeEvent, ToolUseEventData, ToolResultEventData } from "../core/stream-parser.js";

describe("ActivityLogComponent", () => {
  let component: ActivityLogComponent;

  beforeEach(() => {
    component = new ActivityLogComponent();
  });

  describe("processEvent", () => {
    it("adds a tool_use event as running", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "toolu_123",
          name: "Read",
          input: { file_path: "/test/file.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      };

      component.processEvent(event);

      const events = component.getEvents();
      expect(events).toHaveLength(1);
      expect(events[0].id).toBe("toolu_123");
      expect(events[0].name).toBe("Read");
      expect(events[0].target).toBe("/test/file.ts");
      expect(events[0].status).toBe("running");
    });

    it("completes a tool_use when receiving tool_result", () => {
      // Add tool use
      const toolUse: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "toolu_456",
          name: "Write",
          input: { file_path: "/test/output.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(toolUse);

      // Complete with result
      const toolResult: ClaudeEvent = {
        type: "tool_result",
        data: {
          toolUseId: "toolu_456",
          content: "File written",
          isError: false,
        } as ToolResultEventData,
        raw: {} as any,
      };
      component.processEvent(toolResult);

      const events = component.getEvents();
      expect(events).toHaveLength(1);
      expect(events[0].status).toBe("done");
      expect(events[0].duration).toBeDefined();
    });

    it("marks tool as error when result has isError", () => {
      const toolUse: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "toolu_789",
          name: "Bash",
          input: { command: "npm test" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(toolUse);

      const toolResult: ClaudeEvent = {
        type: "tool_result",
        data: {
          toolUseId: "toolu_789",
          content: "Error: tests failed",
          isError: true,
        } as ToolResultEventData,
        raw: {} as any,
      };
      component.processEvent(toolResult);

      const events = component.getEvents();
      expect(events[0].status).toBe("error");
    });

    it("ignores non-tool events", () => {
      const textEvent: ClaudeEvent = {
        type: "text",
        data: { text: "Hello", isPartial: false },
        raw: {} as any,
      };
      component.processEvent(textEvent);

      expect(component.getEvents()).toHaveLength(0);
    });
  });

  describe("extractTarget", () => {
    it("extracts file_path from input", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t1",
          name: "Read",
          input: { file_path: "/path/to/file.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      expect(component.getEvents()[0].target).toBe("/path/to/file.ts");
    });

    it("extracts pattern from Glob input", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t2",
          name: "Glob",
          input: { pattern: "*.ts" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      expect(component.getEvents()[0].target).toBe("*.ts");
    });

    it("extracts path from input when no file_path", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t2b",
          name: "SomeTool",
          input: { path: "/src/dir" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      expect(component.getEvents()[0].target).toBe("/src/dir");
    });

    it("extracts command from Bash input", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t3",
          name: "Bash",
          input: { command: "npm run build" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      expect(component.getEvents()[0].target).toBe("npm run build");
    });

    it("truncates long bash commands", () => {
      const longCommand = "npm run test -- --coverage --verbose --watch --reporter=html";
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t4",
          name: "Bash",
          input: { command: longCommand },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      expect(component.getEvents()[0].target.length).toBeLessThanOrEqual(31);
      expect(component.getEvents()[0].target.endsWith("…")).toBe(true);
    });

    it("extracts description from Task input", () => {
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t5",
          name: "Task",
          input: { description: "Explore codebase" },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      expect(component.getEvents()[0].target).toBe("Explore codebase");
    });

    it("shortens long file paths", () => {
      const longPath = "/very/long/path/that/exceeds/the/maximum/allowed/length/for/display/purposes/file.ts";
      const event: ClaudeEvent = {
        type: "tool_use",
        data: {
          id: "t6",
          name: "Read",
          input: { file_path: longPath },
        } as ToolUseEventData,
        raw: {} as any,
      };
      component.processEvent(event);

      const target = component.getEvents()[0].target;
      expect(target).toBe("purposes/file.ts");
    });
  });

  describe("clear", () => {
    it("removes all events", () => {
      component.addEvent({
        id: "t1",
        name: "Read",
        target: "file.ts",
        status: "done",
        startTime: Date.now(),
      });
      component.addEvent({
        id: "t2",
        name: "Write",
        target: "output.ts",
        status: "running",
        startTime: Date.now(),
      });

      component.clear();

      expect(component.getEvents()).toHaveLength(0);
    });
  });

  describe("history truncation", () => {
    it("limits events to MAX_EVENTS", () => {
      // Add more than 100 events
      for (let i = 0; i < 110; i++) {
        component.addEvent({
          id: `t${i}`,
          name: "Read",
          target: `file${i}.ts`,
          status: "done",
          startTime: Date.now(),
        });
      }

      const events = component.getEvents();
      expect(events.length).toBeLessThanOrEqual(100);
      // Should keep the most recent events
      expect(events[events.length - 1].id).toBe("t109");
    });
  });

  describe("render", () => {
    it("returns placeholder when no events", () => {
      const lines = component.render(80);

      expect(lines).toHaveLength(1);
      expect(lines[0]).toContain("no activity");
    });

    it("renders events with tree structure", () => {
      component.addEvent({
        id: "t1",
        name: "Read",
        target: "auth.ts",
        status: "done",
        startTime: Date.now() - 2000,
        duration: 2000,
      });
      component.addEvent({
        id: "t2",
        name: "Write",
        target: "login.ts",
        status: "running",
        startTime: Date.now(),
      });

      const lines = component.render(80);

      // Should have tree connectors
      expect(lines.some((l) => l.includes("├─"))).toBe(true);
      expect(lines.some((l) => l.includes("└─"))).toBe(true);

      // Should show tool names
      expect(lines.some((l) => l.includes("Read"))).toBe(true);
      expect(lines.some((l) => l.includes("Write"))).toBe(true);

      // Should show status
      expect(lines.some((l) => l.includes("done"))).toBe(true);
      expect(lines.some((l) => l.includes("running"))).toBe(true);
    });

    it("shows status icons", () => {
      component.addEvent({
        id: "t1",
        name: "Read",
        target: "file.ts",
        status: "done",
        startTime: Date.now(),
        duration: 100,
      });
      component.addEvent({
        id: "t2",
        name: "Bash",
        target: "test",
        status: "error",
        startTime: Date.now(),
        duration: 500,
      });
      component.addEvent({
        id: "t3",
        name: "Write",
        target: "out.ts",
        status: "running",
        startTime: Date.now(),
      });

      const lines = component.render(80);
      const joined = lines.join("\n");

      expect(joined).toContain("✓"); // done
      expect(joined).toContain("✗"); // error
      expect(joined).toContain("◐"); // running
    });

    it("formats duration in seconds", () => {
      component.addEvent({
        id: "t1",
        name: "Read",
        target: "file.ts",
        status: "done",
        startTime: Date.now(),
        duration: 2500,
      });

      const lines = component.render(80);
      const joined = lines.join("\n");

      expect(joined).toContain("3s"); // rounded from 2500ms
    });

    it("formats duration in minutes for long operations", () => {
      component.addEvent({
        id: "t1",
        name: "Task",
        target: "explore",
        status: "done",
        startTime: Date.now(),
        duration: 125000, // 2 minutes 5 seconds
      });

      const lines = component.render(80);
      const joined = lines.join("\n");

      expect(joined).toContain("2m5s");
    });

    it("respects width parameter", () => {
      component.addEvent({
        id: "t1",
        name: "Read",
        target: "very/long/path/to/a/deeply/nested/file.ts",
        status: "done",
        startTime: Date.now(),
        duration: 1000,
      });

      const lines = component.render(80);

      // Should render without errors and produce output
      expect(lines.length).toBeGreaterThan(0);
      // All lines should be finite length
      for (const line of lines) {
        expect(line.length).toBeLessThanOrEqual(80);
      }
    });
  });

  describe("addEvent", () => {
    it("adds events directly", () => {
      const event: ActivityEvent = {
        id: "manual-1",
        name: "Custom",
        target: "target",
        status: "running",
        startTime: Date.now(),
      };

      component.addEvent(event);

      expect(component.getEvents()).toHaveLength(1);
      expect(component.getEvents()[0]).toEqual(event);
    });

    it("tracks running events for completion", () => {
      // Add running event
      component.addEvent({
        id: "track-1",
        name: "Read",
        target: "file.ts",
        status: "running",
        startTime: Date.now(),
      });

      // Complete via processEvent
      const toolResult: ClaudeEvent = {
        type: "tool_result",
        data: {
          toolUseId: "track-1",
          content: "done",
          isError: false,
        } as ToolResultEventData,
        raw: {} as any,
      };
      component.processEvent(toolResult);

      expect(component.getEvents()[0].status).toBe("done");
    });
  });
});
