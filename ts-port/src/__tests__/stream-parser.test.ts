import { describe, it, expect } from "vitest";
import {
  parseClaudeEvent,
  extractSessionId,
  parseJsonlLines,
  type ClaudeEvent,
  type SystemEventData,
  type TextEventData,
  type ToolUseEventData,
  type ToolResultEventData,
  type DoneEventData,
  type ErrorEventData,
} from "../core/stream-parser.js";

describe("stream-parser", () => {
  describe("parseClaudeEvent", () => {
    it("parses system init events", () => {
      const line = JSON.stringify({
        type: "system",
        subtype: "init",
        session_id: "test-session-123",
        cwd: "/test/path",
        tools: ["Read", "Write", "Bash"],
        model: "claude-opus-4-5-20251101",
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("system");
      const data = event.data as SystemEventData;
      expect(data.sessionId).toBe("test-session-123");
      expect(data.cwd).toBe("/test/path");
      expect(data.tools).toEqual(["Read", "Write", "Bash"]);
      expect(data.model).toBe("claude-opus-4-5-20251101");
    });

    it("parses assistant text events", () => {
      const line = JSON.stringify({
        type: "assistant",
        session_id: "test-session",
        message: {
          model: "claude-opus-4-5-20251101",
          id: "msg_123",
          role: "assistant",
          content: [{ type: "text", text: "Hello, world!" }],
          stop_reason: "end_turn",
        },
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("text");
      const data = event.data as TextEventData;
      expect(data.text).toBe("Hello, world!");
      expect(data.isPartial).toBe(false);
    });

    it("parses assistant tool_use events", () => {
      const line = JSON.stringify({
        type: "assistant",
        session_id: "test-session",
        message: {
          model: "claude-opus-4-5-20251101",
          id: "msg_123",
          role: "assistant",
          content: [
            {
              type: "tool_use",
              id: "toolu_123",
              name: "Read",
              input: { file_path: "/test/file.txt" },
            },
          ],
          stop_reason: "tool_use",
        },
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("tool_use");
      const data = event.data as ToolUseEventData;
      expect(data.id).toBe("toolu_123");
      expect(data.name).toBe("Read");
      expect(data.input).toEqual({ file_path: "/test/file.txt" });
    });

    it("parses user tool_result events", () => {
      const line = JSON.stringify({
        type: "user",
        session_id: "test-session",
        message: {
          role: "user",
          content: [
            {
              type: "tool_result",
              tool_use_id: "toolu_123",
              content: "File contents here",
              is_error: false,
            },
          ],
        },
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("tool_result");
      const data = event.data as ToolResultEventData;
      expect(data.toolUseId).toBe("toolu_123");
      expect(data.content).toBe("File contents here");
      expect(data.isError).toBe(false);
    });

    it("parses user tool_result error events", () => {
      const line = JSON.stringify({
        type: "user",
        session_id: "test-session",
        message: {
          role: "user",
          content: [
            {
              type: "tool_result",
              tool_use_id: "toolu_456",
              content: "<tool_use_error>File does not exist.</tool_use_error>",
              is_error: true,
            },
          ],
        },
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("tool_result");
      const data = event.data as ToolResultEventData;
      expect(data.toolUseId).toBe("toolu_456");
      expect(data.isError).toBe(true);
    });

    it("parses result success events", () => {
      const line = JSON.stringify({
        type: "result",
        subtype: "success",
        is_error: false,
        session_id: "test-session",
        result: "Task completed successfully",
        duration_ms: 5000,
        num_turns: 3,
        total_cost_usd: 0.05,
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("done");
      const data = event.data as DoneEventData;
      expect(data.result).toBe("Task completed successfully");
      expect(data.durationMs).toBe(5000);
      expect(data.numTurns).toBe(3);
      expect(data.totalCostUsd).toBe(0.05);
    });

    it("parses result error events", () => {
      const line = JSON.stringify({
        type: "result",
        subtype: "error",
        is_error: true,
        session_id: "test-session",
        error: "Something went wrong",
        duration_ms: 1000,
        num_turns: 1,
        total_cost_usd: 0.01,
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("error");
      const data = event.data as ErrorEventData;
      expect(data.message).toBe("Something went wrong");
    });

    it("parses stream_event text_delta", () => {
      const line = JSON.stringify({
        type: "stream_event",
        session_id: "test-session",
        event: {
          type: "content_block_delta",
          index: 0,
          delta: { type: "text_delta", text: "partial text" },
        },
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("text");
      const data = event.data as TextEventData;
      expect(data.text).toBe("partial text");
      expect(data.isPartial).toBe(true);
    });

    it("parses stream_event tool_use start", () => {
      const line = JSON.stringify({
        type: "stream_event",
        session_id: "test-session",
        event: {
          type: "content_block_start",
          index: 0,
          content_block: {
            type: "tool_use",
            id: "toolu_789",
            name: "Bash",
            input: {},
          },
        },
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("tool_use");
      const data = event.data as ToolUseEventData;
      expect(data.id).toBe("toolu_789");
      expect(data.name).toBe("Bash");
    });

    it("handles unknown event types gracefully", () => {
      const line = JSON.stringify({
        type: "unknown_type",
        some_data: "value",
      });

      const event = parseClaudeEvent(line);

      expect(event.type).toBe("text");
      const data = event.data as TextEventData;
      expect(data.text).toBe("");
      expect(data.isPartial).toBe(true);
    });
  });

  describe("extractSessionId", () => {
    it("extracts session ID from system events", () => {
      const event: ClaudeEvent = {
        type: "system",
        data: {
          sessionId: "extracted-session-id",
          model: "claude-opus-4-5-20251101",
          tools: [],
          cwd: "/test",
        } as SystemEventData,
        raw: { type: "system" } as any,
      };

      expect(extractSessionId(event)).toBe("extracted-session-id");
    });

    it("returns null for non-system events", () => {
      const event: ClaudeEvent = {
        type: "text",
        data: { text: "hello", isPartial: false } as TextEventData,
        raw: { type: "assistant" } as any,
      };

      expect(extractSessionId(event)).toBeNull();
    });
  });

  describe("parseJsonlLines", () => {
    it("yields complete lines", () => {
      const buffer = '{"a":1}\n{"b":2}\n';
      const gen = parseJsonlLines(buffer);

      const lines: string[] = [];
      let result = gen.next();
      while (!result.done) {
        lines.push(result.value);
        result = gen.next();
      }

      expect(lines).toEqual(['{"a":1}', '{"b":2}']);
      expect(result.value).toBe(""); // remaining buffer is empty
    });

    it("returns incomplete line as remaining", () => {
      const buffer = '{"a":1}\n{"incomplete';
      const gen = parseJsonlLines(buffer);

      const lines: string[] = [];
      let result = gen.next();
      while (!result.done) {
        lines.push(result.value);
        result = gen.next();
      }

      expect(lines).toEqual(['{"a":1}']);
      expect(result.value).toBe('{"incomplete');
    });

    it("skips empty lines", () => {
      const buffer = '{"a":1}\n\n{"b":2}\n';
      const gen = parseJsonlLines(buffer);

      const lines: string[] = [];
      let result = gen.next();
      while (!result.done) {
        lines.push(result.value);
        result = gen.next();
      }

      expect(lines).toEqual(['{"a":1}', '{"b":2}']);
    });
  });
});
