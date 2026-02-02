/**
 * Session persistence with JSONL format
 *
 * Sessions are stored in .rafa/sessions/ as JSONL files.
 * Each line is a separate JSON object representing either
 * session metadata or conversation messages.
 *
 * Format (from docs/designs/rafa-pi-mono-port.md):
 * {"type":"session","id":"abc123","phase":"prd","createdAt":"...","claudeSessionId":"..."}
 * {"type":"user","id":"m1","parentId":null,"content":"I want to build..."}
 * {"type":"assistant","id":"m2","parentId":"m1","content":"Let me ask..."}
 * {"type":"tool_use","id":"m3","parentId":"m2","tool":"Read","target":"..."}
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import type { Dirent } from "node:fs";
import type { Session, SessionMessage } from "../core/session.js";
import { generateId } from "../utils/id.js";

const RAFA_DIR = ".rafa";
const SESSIONS_DIR = "sessions";

/**
 * JSONL entry types
 */
export type SessionEntryType = "session" | "user" | "assistant" | "tool_use";

/**
 * Base interface for all JSONL entries
 */
export interface SessionEntry {
  type: SessionEntryType;
  id: string;
}

/**
 * Session metadata entry (first line of file)
 */
export interface SessionMetadataEntry extends SessionEntry {
  type: "session";
  phase: "prd" | "design";
  name?: string;
  createdAt: string;
  updatedAt?: string;
  claudeSessionId?: string;
}

/**
 * User message entry
 */
export interface UserMessageEntry extends SessionEntry {
  type: "user";
  parentId: string | null;
  content: string;
}

/**
 * Assistant message entry
 */
export interface AssistantMessageEntry extends SessionEntry {
  type: "assistant";
  parentId: string | null;
  content: string;
}

/**
 * Tool use entry
 */
export interface ToolUseEntry extends SessionEntry {
  type: "tool_use";
  parentId: string | null;
  tool: string;
  target?: string;
  input?: Record<string, unknown>;
}

/**
 * Union of all session entry types
 */
export type AnySessionEntry =
  | SessionMetadataEntry
  | UserMessageEntry
  | AssistantMessageEntry
  | ToolUseEntry;

/**
 * Session with messages - returned when loading a session
 */
export interface LoadedSession extends Session {
  messages: SessionMessage[];
}

/**
 * Error thrown when session resume fails
 */
export class SessionResumeError extends Error {
  constructor(
    message: string,
    public readonly sessionPath: string,
    public readonly cause?: Error,
  ) {
    super(message);
    this.name = "SessionResumeError";
  }
}

/**
 * Gets the path to the sessions directory
 */
export function getSessionsDir(workDir: string = process.cwd()): string {
  return path.join(workDir, RAFA_DIR, SESSIONS_DIR);
}

/**
 * Gets the filename for a session
 * Format: <phase>-<name>.jsonl (e.g., "prd-user-auth.jsonl")
 */
export function getSessionFilename(
  phase: "prd" | "design",
  name: string,
): string {
  return `${phase}-${name}.jsonl`;
}

/**
 * Gets the full path to a session file
 */
export function getSessionPath(
  phase: "prd" | "design",
  name: string,
  workDir: string = process.cwd(),
): string {
  return path.join(getSessionsDir(workDir), getSessionFilename(phase, name));
}

/**
 * Ensures the sessions directory exists
 */
export async function ensureSessionsDir(
  workDir: string = process.cwd(),
): Promise<string> {
  const sessionsDir = getSessionsDir(workDir);
  await fs.mkdir(sessionsDir, { recursive: true });
  return sessionsDir;
}

/**
 * Parses a JSONL file line by line
 */
async function parseJsonlFile(filePath: string): Promise<AnySessionEntry[]> {
  const content = await fs.readFile(filePath, "utf-8");
  const lines = content.split("\n").filter((line) => line.trim());
  const entries: AnySessionEntry[] = [];

  for (const line of lines) {
    try {
      const entry = JSON.parse(line) as AnySessionEntry;
      entries.push(entry);
    } catch {
      // Skip malformed message lines silently - corruption is handled
      // at a higher level (e.g., entriesToSession checks metadata)
    }
  }

  return entries;
}

/**
 * Converts session entries to a LoadedSession
 */
function entriesToSession(
  entries: AnySessionEntry[],
  filePath: string,
): LoadedSession {
  if (entries.length === 0) {
    throw new SessionResumeError("Session file is empty", filePath);
  }

  const firstEntry = entries[0];
  if (firstEntry.type !== "session") {
    throw new SessionResumeError(
      "Session file does not start with session metadata",
      filePath,
    );
  }

  const metadata = firstEntry as SessionMetadataEntry;
  const messages: SessionMessage[] = [];

  for (let i = 1; i < entries.length; i++) {
    const entry = entries[i];
    if (
      entry.type === "user" ||
      entry.type === "assistant" ||
      entry.type === "tool_use"
    ) {
      messages.push({
        type: entry.type,
        id: entry.id,
        parentId: entry.parentId,
        content:
          entry.type === "tool_use"
            ? undefined
            : (entry as UserMessageEntry | AssistantMessageEntry).content,
        tool:
          entry.type === "tool_use" ? (entry as ToolUseEntry).tool : undefined,
        target:
          entry.type === "tool_use"
            ? (entry as ToolUseEntry).target
            : undefined,
        input:
          entry.type === "tool_use" ? (entry as ToolUseEntry).input : undefined,
      });
    }
  }

  return {
    id: metadata.id,
    phase: metadata.phase,
    name: metadata.name,
    createdAt: metadata.createdAt,
    updatedAt: metadata.updatedAt,
    claudeSessionId: metadata.claudeSessionId,
    messages,
  };
}

/**
 * Loads a session from a JSONL file
 *
 * @param sessionPath - Full path to the session file
 * @returns LoadedSession with metadata and messages, or null if file doesn't exist
 * @throws SessionResumeError if file exists but is corrupted/invalid
 */
export async function loadSession(
  sessionPath: string,
): Promise<LoadedSession | null> {
  try {
    const entries = await parseJsonlFile(sessionPath);
    return entriesToSession(entries, sessionPath);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return null;
    }
    if (err instanceof SessionResumeError) {
      throw err;
    }
    throw new SessionResumeError(
      `Failed to load session: ${(err as Error).message}`,
      sessionPath,
      err as Error,
    );
  }
}

/**
 * Loads a session by phase and name
 *
 * @param phase - Session phase (prd or design)
 * @param name - Session name
 * @param workDir - Working directory (defaults to cwd)
 * @returns LoadedSession or null if not found
 * @throws SessionResumeError if session is corrupted
 */
export async function loadSessionByName(
  phase: "prd" | "design",
  name: string,
  workDir: string = process.cwd(),
): Promise<LoadedSession | null> {
  const sessionPath = getSessionPath(phase, name, workDir);
  return loadSession(sessionPath);
}

/**
 * Creates a new session file with initial metadata
 *
 * @param session - Session to create (use createSession from core/session.ts to create the object)
 * @param name - Name for the session file
 * @param workDir - Working directory (defaults to cwd)
 * @returns Path to the created session file
 */
export async function createSessionFile(
  session: Session,
  name: string,
  workDir: string = process.cwd(),
): Promise<string> {
  await ensureSessionsDir(workDir);

  const sessionPath = getSessionPath(session.phase, name, workDir);

  const metadata: SessionMetadataEntry = {
    type: "session",
    id: session.id,
    phase: session.phase,
    name,
    createdAt: session.createdAt,
    claudeSessionId: session.claudeSessionId,
  };

  await fs.writeFile(sessionPath, JSON.stringify(metadata) + "\n", "utf-8");
  return sessionPath;
}

/**
 * Appends a message entry to a session file
 *
 * @param sessionPath - Full path to the session file
 * @param entry - Message entry to append
 */
export async function appendMessage(
  sessionPath: string,
  entry: UserMessageEntry | AssistantMessageEntry | ToolUseEntry,
): Promise<void> {
  const handle = await fs.open(sessionPath, "a");
  try {
    await handle.write(JSON.stringify(entry) + "\n");
  } finally {
    await handle.close();
  }
}

/**
 * Appends a user message to a session
 *
 * @param sessionPath - Full path to the session file
 * @param content - Message content
 * @param parentId - ID of the parent message (null for root messages)
 * @returns Generated message ID
 */
export async function appendUserMessage(
  sessionPath: string,
  content: string,
  parentId: string | null = null,
): Promise<string> {
  const id = `m${generateId()}`;
  const entry: UserMessageEntry = {
    type: "user",
    id,
    parentId,
    content,
  };
  await appendMessage(sessionPath, entry);
  return id;
}

/**
 * Appends an assistant message to a session
 *
 * @param sessionPath - Full path to the session file
 * @param content - Message content
 * @param parentId - ID of the parent message
 * @returns Generated message ID
 */
export async function appendAssistantMessage(
  sessionPath: string,
  content: string,
  parentId: string | null,
): Promise<string> {
  const id = `m${generateId()}`;
  const entry: AssistantMessageEntry = {
    type: "assistant",
    id,
    parentId,
    content,
  };
  await appendMessage(sessionPath, entry);
  return id;
}

/**
 * Appends a tool use entry to a session
 *
 * @param sessionPath - Full path to the session file
 * @param tool - Tool name
 * @param parentId - ID of the parent message
 * @param target - Optional target (file path, etc.)
 * @param input - Optional tool input
 * @returns Generated entry ID
 */
export async function appendToolUse(
  sessionPath: string,
  tool: string,
  parentId: string | null,
  target?: string,
  input?: Record<string, unknown>,
): Promise<string> {
  const id = `m${generateId()}`;
  const entry: ToolUseEntry = {
    type: "tool_use",
    id,
    parentId,
    tool,
    target,
    input,
  };
  await appendMessage(sessionPath, entry);
  return id;
}

/**
 * Updates session metadata (claudeSessionId, updatedAt)
 *
 * This rewrites the entire file to update the first line.
 * For frequent updates, consider caching in memory.
 *
 * @param sessionPath - Full path to the session file
 * @param updates - Fields to update
 */
export async function updateSessionMetadata(
  sessionPath: string,
  updates: {
    claudeSessionId?: string;
    updatedAt?: string;
  },
): Promise<void> {
  const content = await fs.readFile(sessionPath, "utf-8");
  const lines = content.split("\n");

  if (lines.length === 0 || !lines[0].trim()) {
    throw new SessionResumeError("Session file is empty", sessionPath);
  }

  // Parse and update the first line (metadata)
  const metadata = JSON.parse(lines[0]) as SessionMetadataEntry;

  if (updates.claudeSessionId !== undefined) {
    metadata.claudeSessionId = updates.claudeSessionId;
  }
  if (updates.updatedAt !== undefined) {
    metadata.updatedAt = updates.updatedAt;
  } else {
    metadata.updatedAt = new Date().toISOString();
  }

  // Rewrite file with updated metadata
  lines[0] = JSON.stringify(metadata);
  await fs.writeFile(sessionPath, lines.join("\n"), "utf-8");
}

/**
 * Saves a complete session (metadata + messages) to a file using atomic write.
 * Use this for initial creation or full rewrites.
 *
 * @param session - Session with messages to save
 * @param sessionPath - Full path to the session file
 */
export async function saveSession(
  session: LoadedSession,
  sessionPath: string,
): Promise<void> {
  const dir = path.dirname(sessionPath);
  await fs.mkdir(dir, { recursive: true });

  const lines: string[] = [];

  // First line: session metadata
  const metadata: SessionMetadataEntry = {
    type: "session",
    id: session.id,
    phase: session.phase,
    name: session.name,
    createdAt: session.createdAt,
    updatedAt: session.updatedAt || new Date().toISOString(),
    claudeSessionId: session.claudeSessionId,
  };
  lines.push(JSON.stringify(metadata));

  // Subsequent lines: messages
  for (const msg of session.messages) {
    if (msg.type === "user") {
      const entry: UserMessageEntry = {
        type: "user",
        id: msg.id,
        parentId: msg.parentId,
        content: msg.content || "",
      };
      lines.push(JSON.stringify(entry));
    } else if (msg.type === "assistant") {
      const entry: AssistantMessageEntry = {
        type: "assistant",
        id: msg.id,
        parentId: msg.parentId,
        content: msg.content || "",
      };
      lines.push(JSON.stringify(entry));
    } else if (msg.type === "tool_use") {
      const entry: ToolUseEntry = {
        type: "tool_use",
        id: msg.id,
        parentId: msg.parentId,
        tool: msg.tool || "",
        target: msg.target,
        input: msg.input,
      };
      lines.push(JSON.stringify(entry));
    }
  }

  // Use atomic write: write to temp file, then rename
  const tmpPath = `${sessionPath}.tmp.${process.pid}`;
  const data = lines.join("\n") + "\n";

  try {
    await fs.writeFile(tmpPath, data, "utf-8");
    await fs.rename(tmpPath, sessionPath);
  } catch (err) {
    // Clean up temp file on failure
    try {
      await fs.unlink(tmpPath);
    } catch {
      // Ignore cleanup errors
    }
    throw new Error(`Failed to save session: ${err}`);
  }
}

/**
 * Lists all sessions from .rafa/sessions/
 *
 * @param workDir - Working directory (defaults to cwd)
 * @returns Array of session metadata (without messages for efficiency)
 */
export async function listSessions(
  workDir: string = process.cwd(),
): Promise<Session[]> {
  const sessionsDir = getSessionsDir(workDir);

  let entries: Dirent[];
  try {
    entries = await fs.readdir(sessionsDir, { withFileTypes: true });
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return [];
    }
    throw new Error(`Failed to read sessions directory: ${err}`);
  }

  const sessions: Session[] = [];

  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith(".jsonl")) {
      continue;
    }

    const sessionPath = path.join(sessionsDir, entry.name);
    try {
      // Read only the first line for metadata
      const content = await fs.readFile(sessionPath, "utf-8");
      const firstLine = content.split("\n")[0];
      if (!firstLine?.trim()) continue;

      const metadata = JSON.parse(firstLine) as SessionMetadataEntry;
      if (metadata.type !== "session") continue;

      sessions.push({
        id: metadata.id,
        phase: metadata.phase,
        name: metadata.name,
        createdAt: metadata.createdAt,
        updatedAt: metadata.updatedAt,
        claudeSessionId: metadata.claudeSessionId,
      });
    } catch {
      // Skip invalid session files
      continue;
    }
  }

  // Sort by updatedAt or createdAt, most recent first
  sessions.sort((a, b) => {
    const aTime = a.updatedAt || a.createdAt;
    const bTime = b.updatedAt || b.createdAt;
    return bTime.localeCompare(aTime);
  });

  return sessions;
}

/**
 * Deletes a session file
 *
 * @param sessionPath - Full path to the session file
 * @returns true if deleted, false if file didn't exist
 */
export async function deleteSession(sessionPath: string): Promise<boolean> {
  try {
    await fs.unlink(sessionPath);
    return true;
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return false;
    }
    throw new Error(`Failed to delete session: ${err}`);
  }
}

/**
 * Checks if a session exists
 *
 * @param phase - Session phase
 * @param name - Session name
 * @param workDir - Working directory
 * @returns true if session exists
 */
export async function sessionExists(
  phase: "prd" | "design",
  name: string,
  workDir: string = process.cwd(),
): Promise<boolean> {
  const sessionPath = getSessionPath(phase, name, workDir);
  try {
    await fs.access(sessionPath);
    return true;
  } catch {
    return false;
  }
}

/**
 * Generates a unique session name, appending -2, -3, etc. if needed
 *
 * @param phase - Session phase
 * @param baseName - Base name to use
 * @param workDir - Working directory
 * @returns Unique session name
 */
export async function resolveSessionName(
  phase: "prd" | "design",
  baseName: string,
  workDir: string = process.cwd(),
): Promise<string> {
  // Normalize the name (lowercase, hyphens instead of spaces)
  let name = baseName.toLowerCase().replace(/\s+/g, "-");

  if (!(await sessionExists(phase, name, workDir))) {
    return name;
  }

  // Find a unique suffix
  for (let i = 2; ; i++) {
    const candidate = `${name}-${i}`;
    if (!(await sessionExists(phase, candidate, workDir))) {
      return candidate;
    }
  }
}

/**
 * Options for resuming a session
 */
export interface ResumeSessionResult {
  success: true;
  session: LoadedSession;
  sessionPath: string;
}

export interface ResumeSessionFailure {
  success: false;
  error: SessionResumeError;
  sessionPath: string;
}

export type ResumeResult = ResumeSessionResult | ResumeSessionFailure;

/**
 * Attempts to resume a session, returning either the loaded session
 * or an error with context for offering "start fresh" option
 *
 * @param phase - Session phase
 * @param name - Session name
 * @param workDir - Working directory
 * @returns ResumeResult with success status and either session or error
 */
export async function tryResumeSession(
  phase: "prd" | "design",
  name: string,
  workDir: string = process.cwd(),
): Promise<ResumeResult> {
  const sessionPath = getSessionPath(phase, name, workDir);

  try {
    const session = await loadSession(sessionPath);
    if (!session) {
      return {
        success: false,
        error: new SessionResumeError("Session not found", sessionPath),
        sessionPath,
      };
    }
    return { success: true, session, sessionPath };
  } catch (err) {
    if (err instanceof SessionResumeError) {
      return { success: false, error: err, sessionPath };
    }
    return {
      success: false,
      error: new SessionResumeError(
        `Failed to resume session: ${(err as Error).message}`,
        sessionPath,
        err as Error,
      ),
      sessionPath,
    };
  }
}

/**
 * Starts a fresh session, deleting any existing one with the same name
 *
 * @param phase - Session phase
 * @param name - Session name
 * @param workDir - Working directory
 * @returns New session metadata and path
 */
export async function startFreshSession(
  phase: "prd" | "design",
  name: string,
  workDir: string = process.cwd(),
): Promise<{ session: Session; sessionPath: string }> {
  const sessionPath = getSessionPath(phase, name, workDir);

  // Delete existing session if present
  await deleteSession(sessionPath);

  // Create new session
  const session: Session = {
    id: generateId(),
    phase,
    name,
    createdAt: new Date().toISOString(),
  };

  await createSessionFile(session, name, workDir);
  return { session, sessionPath };
}
