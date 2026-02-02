/**
 * Conversation session management
 */

import { generateId } from "../utils/id.js";

export interface Session {
  id: string;
  phase: "prd" | "design";
  createdAt: string;
  claudeSessionId?: string;
}

export function createSession(phase: "prd" | "design"): Session {
  return {
    id: generateId(),
    phase,
    createdAt: new Date().toISOString(),
  };
}
