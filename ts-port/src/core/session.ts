/**
 * Conversation session management
 */

import { generateId } from "../utils/id.js";

/**
 * Session metadata
 */
export interface Session {
  id: string;
  phase: "prd" | "design";
  name?: string;
  createdAt: string;
  updatedAt?: string;
  claudeSessionId?: string;
}

/**
 * Message in a conversation session
 * Matches JSONL format from docs/designs/rafa-pi-mono-port.md
 */
export interface SessionMessage {
  type: "user" | "assistant" | "tool_use";
  id: string;
  parentId: string | null;
  content?: string; // For user/assistant messages
  tool?: string; // For tool_use
  target?: string; // For tool_use (file path, etc.)
  input?: Record<string, unknown>; // For tool_use
}

/**
 * Creates a new session with generated ID
 */
export function createSession(phase: "prd" | "design", name?: string): Session {
  return {
    id: generateId(),
    phase,
    name,
    createdAt: new Date().toISOString(),
  };
}
