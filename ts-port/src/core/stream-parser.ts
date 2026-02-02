/**
 * Parse stream-json output from Claude CLI
 *
 * The Claude CLI with --output-format stream-json emits JSONL events.
 * Event types discovered from actual output:
 * - system (init): Contains session_id, tools, model info
 * - stream_event: Streaming partial content (message_start, content_block_start/delta/stop, message_delta, message_stop)
 * - assistant: Complete assistant message with content blocks
 * - user: Tool results
 * - result: Final result with success/error status
 */

// Raw event types from Claude CLI stream-json output
export interface RawSystemEvent {
  type: "system";
  subtype: "init";
  session_id: string;
  cwd: string;
  tools: string[];
  model: string;
  [key: string]: unknown;
}

export interface RawStreamEvent {
  type: "stream_event";
  session_id: string;
  event: {
    type:
      | "message_start"
      | "content_block_start"
      | "content_block_delta"
      | "content_block_stop"
      | "message_delta"
      | "message_stop";
    index?: number;
    content_block?: {
      type: "tool_use" | "text";
      id?: string;
      name?: string;
      text?: string;
      input?: Record<string, unknown>;
    };
    delta?: {
      type: "text_delta" | "input_json_delta";
      text?: string;
      partial_json?: string;
      stop_reason?: string;
    };
    message?: {
      model: string;
      id: string;
      role: string;
      content: unknown[];
    };
    [key: string]: unknown;
  };
}

export interface RawAssistantEvent {
  type: "assistant";
  session_id: string;
  message: {
    model: string;
    id: string;
    role: "assistant";
    content: Array<{
      type: "text" | "tool_use";
      text?: string;
      id?: string;
      name?: string;
      input?: Record<string, unknown>;
    }>;
    stop_reason: string | null;
    [key: string]: unknown;
  };
}

export interface RawUserEvent {
  type: "user";
  session_id: string;
  message: {
    role: "user";
    content: Array<{
      type: "tool_result";
      tool_use_id: string;
      content: string;
      is_error?: boolean;
    }>;
  };
  tool_use_result?: string | { type: string; [key: string]: unknown };
}

export interface RawResultEvent {
  type: "result";
  subtype: "success" | "error";
  is_error: boolean;
  session_id: string;
  result?: string;
  error?: string;
  duration_ms: number;
  num_turns: number;
  total_cost_usd: number;
  [key: string]: unknown;
}

export type RawClaudeEvent =
  | RawSystemEvent
  | RawStreamEvent
  | RawAssistantEvent
  | RawUserEvent
  | RawResultEvent
  | { type: string; [key: string]: unknown };

// Simplified event types for consumers
export type ClaudeEventType =
  | "system"
  | "tool_use"
  | "tool_result"
  | "text"
  | "done"
  | "error";

export interface SystemEventData {
  sessionId: string;
  model: string;
  tools: string[];
  cwd: string;
}

export interface ToolUseEventData {
  id: string;
  name: string;
  input: Record<string, unknown>;
}

export interface ToolResultEventData {
  toolUseId: string;
  content: string;
  isError: boolean;
}

export interface TextEventData {
  text: string;
  isPartial: boolean;
}

export interface DoneEventData {
  result?: string;
  durationMs: number;
  numTurns: number;
  totalCostUsd: number;
}

export interface ErrorEventData {
  message: string;
}

export type ClaudeEventData =
  | SystemEventData
  | ToolUseEventData
  | ToolResultEventData
  | TextEventData
  | DoneEventData
  | ErrorEventData;

export interface ClaudeEvent {
  type: ClaudeEventType;
  data: ClaudeEventData;
  raw: RawClaudeEvent;
}

/**
 * Parse a single line of JSONL from Claude CLI stream-json output
 */
export function parseClaudeEvent(line: string): ClaudeEvent {
  const raw = JSON.parse(line) as RawClaudeEvent;
  return convertRawEvent(raw);
}

// Type guards for raw events
function isSystemEvent(raw: RawClaudeEvent): raw is RawSystemEvent {
  return raw.type === "system" && "session_id" in raw;
}

function isStreamEvent(raw: RawClaudeEvent): raw is RawStreamEvent {
  return raw.type === "stream_event" && "event" in raw;
}

function isAssistantEvent(raw: RawClaudeEvent): raw is RawAssistantEvent {
  return raw.type === "assistant" && "message" in raw;
}

function isUserEvent(raw: RawClaudeEvent): raw is RawUserEvent {
  return raw.type === "user" && "message" in raw;
}

function isResultEvent(raw: RawClaudeEvent): raw is RawResultEvent {
  return raw.type === "result" && "is_error" in raw;
}

/**
 * Convert a raw Claude event to a simplified ClaudeEvent
 */
export function convertRawEvent(raw: RawClaudeEvent): ClaudeEvent {
  if (isSystemEvent(raw)) {
    return {
      type: "system",
      data: {
        sessionId: raw.session_id,
        model: raw.model,
        tools: raw.tools,
        cwd: raw.cwd,
      } as SystemEventData,
      raw,
    };
  }

  if (isStreamEvent(raw)) {
    return parseStreamEvent(raw);
  }

  if (isAssistantEvent(raw)) {
    return parseAssistantEvent(raw);
  }

  if (isUserEvent(raw)) {
    return parseUserEvent(raw);
  }

  if (isResultEvent(raw)) {
    return parseResultEvent(raw);
  }

  // Unknown event type - treat as text with empty content
  return {
    type: "text",
    data: { text: "", isPartial: true } as TextEventData,
    raw,
  };
}

function parseStreamEvent(raw: RawStreamEvent): ClaudeEvent {
  const { event } = raw;

  // Handle text streaming
  if (
    event.type === "content_block_delta" &&
    event.delta?.type === "text_delta"
  ) {
    return {
      type: "text",
      data: {
        text: event.delta.text ?? "",
        isPartial: true,
      } as TextEventData,
      raw,
    };
  }

  // Handle tool use start (partial)
  if (
    event.type === "content_block_start" &&
    event.content_block?.type === "tool_use"
  ) {
    return {
      type: "tool_use",
      data: {
        id: event.content_block.id ?? "",
        name: event.content_block.name ?? "",
        input: event.content_block.input ?? {},
      } as ToolUseEventData,
      raw,
    };
  }

  // Handle message stop
  if (event.type === "message_stop") {
    return {
      type: "done",
      data: {
        durationMs: 0,
        numTurns: 0,
        totalCostUsd: 0,
      } as DoneEventData,
      raw,
    };
  }

  // Other stream events (message_start, content_block_stop, etc.) - pass through as text
  return {
    type: "text",
    data: { text: "", isPartial: true } as TextEventData,
    raw,
  };
}

function parseAssistantEvent(raw: RawAssistantEvent): ClaudeEvent {
  const content = raw.message.content;

  // Check for tool_use in content
  for (const block of content) {
    if (block.type === "tool_use") {
      return {
        type: "tool_use",
        data: {
          id: block.id ?? "",
          name: block.name ?? "",
          input: block.input ?? {},
        } as ToolUseEventData,
        raw,
      };
    }
  }

  // Check for text in content
  for (const block of content) {
    if (block.type === "text") {
      return {
        type: "text",
        data: {
          text: block.text ?? "",
          isPartial: false,
        } as TextEventData,
        raw,
      };
    }
  }

  // Empty content
  return {
    type: "text",
    data: { text: "", isPartial: false } as TextEventData,
    raw,
  };
}

function parseUserEvent(raw: RawUserEvent): ClaudeEvent {
  const content = raw.message.content;

  // Tool results
  for (const block of content) {
    if (block.type === "tool_result") {
      return {
        type: "tool_result",
        data: {
          toolUseId: block.tool_use_id,
          content: block.content,
          isError: block.is_error ?? false,
        } as ToolResultEventData,
        raw,
      };
    }
  }

  // Fallback
  return {
    type: "text",
    data: { text: "", isPartial: false } as TextEventData,
    raw,
  };
}

function parseResultEvent(raw: RawResultEvent): ClaudeEvent {
  if (raw.is_error || raw.subtype === "error") {
    return {
      type: "error",
      data: {
        message: raw.error ?? "Unknown error",
      } as ErrorEventData,
      raw,
    };
  }

  return {
    type: "done",
    data: {
      result: raw.result,
      durationMs: raw.duration_ms,
      numTurns: raw.num_turns,
      totalCostUsd: raw.total_cost_usd,
    } as DoneEventData,
    raw,
  };
}

/**
 * Parse JSONL lines from a stream
 */
export function* parseJsonlLines(buffer: string): Generator<string, string> {
  let remaining = buffer;

  while (true) {
    const newlineIndex = remaining.indexOf("\n");
    if (newlineIndex === -1) {
      // Return remaining buffer (incomplete line)
      return remaining;
    }

    const line = remaining.substring(0, newlineIndex);
    remaining = remaining.substring(newlineIndex + 1);

    if (line.trim()) {
      yield line;
    }
  }
}

/**
 * Extract session ID from a system event
 */
export function extractSessionId(event: ClaudeEvent): string | null {
  if (event.type === "system") {
    return (event.data as SystemEventData).sessionId;
  }
  return null;
}
