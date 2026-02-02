/**
 * Session persistence
 */

import type { Session } from "../core/session.js";

export async function loadSession(_sessionPath: string): Promise<Session | null> {
  // Implementation in Task 12
  return null;
}

export async function saveSession(
  _session: Session,
  _sessionPath: string
): Promise<void> {
  // Implementation in Task 12
}

export async function listSessions(_sessionsDir: string): Promise<Session[]> {
  // Implementation in Task 12
  return [];
}
