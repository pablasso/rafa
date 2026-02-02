/**
 * Parse stream-json output from Claude CLI
 */

import type { ClaudeEvent } from "./claude-runner.js";

export function parseClaudeEvent(line: string): ClaudeEvent {
  const data = JSON.parse(line) as Record<string, unknown>;

  // Basic event type detection - will be refined in Task 4
  if (data.type === "system") {
    return { type: "system", data };
  }
  if (data.type === "tool_use") {
    return { type: "tool_use", data };
  }
  if (data.type === "tool_result") {
    return { type: "tool_result", data };
  }
  if (data.type === "text" || data.type === "content_block_delta") {
    return { type: "text", data };
  }
  if (data.type === "done" || data.type === "message_stop") {
    return { type: "done", data };
  }

  return { type: "text", data };
}
