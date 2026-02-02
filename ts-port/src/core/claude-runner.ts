/**
 * Claude CLI invocation
 */

export interface ClaudeEvent {
  type: "text" | "tool_use" | "tool_result" | "system" | "done" | "error";
  data: unknown;
}

export interface TaskRunOptions {
  prompt: string;
  onEvent: (event: ClaudeEvent) => void;
}

export interface ConversationRunOptions extends TaskRunOptions {
  sessionId?: string;
}

export async function runTask(_options: TaskRunOptions): Promise<void> {
  // Implementation in Task 4
}

export async function runConversation(
  _options: ConversationRunOptions
): Promise<string> {
  // Implementation in Task 4
  return "";
}
