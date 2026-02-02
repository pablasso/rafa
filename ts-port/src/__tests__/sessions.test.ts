import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "node:fs/promises";
import * as path from "node:path";
import {
  loadSession,
  loadSessionByName,
  createSessionFile,
  saveSession,
  listSessions,
  deleteSession,
  appendUserMessage,
  appendAssistantMessage,
  appendToolUse,
  updateSessionMetadata,
  sessionExists,
  resolveSessionName,
  tryResumeSession,
  startFreshSession,
  getSessionsDir,
  getSessionPath,
  SessionResumeError,
  type LoadedSession,
} from "../storage/sessions.js";
import type { Session } from "../core/session.js";

const TEST_DIR = path.join(process.cwd(), "test-temp-sessions");

async function createTestDir() {
  await fs.mkdir(path.join(TEST_DIR, ".rafa", "sessions"), { recursive: true });
}

async function cleanupTestDir() {
  try {
    await fs.rm(TEST_DIR, { recursive: true, force: true });
  } catch {
    // Ignore errors
  }
}

function createTestSession(overrides: Partial<Session> = {}): Session {
  return {
    id: "abc123",
    phase: "prd",
    name: "test-session",
    createdAt: "2024-01-01T00:00:00.000Z",
    ...overrides,
  };
}

describe("session storage", () => {
  beforeEach(async () => {
    await cleanupTestDir();
    await createTestDir();
  });

  afterEach(async () => {
    await cleanupTestDir();
  });

  describe("getSessionsDir and getSessionPath", () => {
    it("returns correct sessions directory path", () => {
      const dir = getSessionsDir(TEST_DIR);
      expect(dir).toBe(path.join(TEST_DIR, ".rafa", "sessions"));
    });

    it("returns correct session file path", () => {
      const sessionPath = getSessionPath("prd", "my-feature", TEST_DIR);
      expect(sessionPath).toBe(
        path.join(TEST_DIR, ".rafa", "sessions", "prd-my-feature.jsonl"),
      );
    });
  });

  describe("createSession and loadSession", () => {
    it("creates and loads a session correctly", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const loaded = await loadSession(sessionPath);
      expect(loaded).not.toBeNull();
      expect(loaded!.id).toBe(session.id);
      expect(loaded!.phase).toBe(session.phase);
      expect(loaded!.name).toBe(session.name);
      expect(loaded!.createdAt).toBe(session.createdAt);
      expect(loaded!.messages).toEqual([]);
    });

    it("stores claudeSessionId when provided", async () => {
      const session = createTestSession({ claudeSessionId: "claude-123" });
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const loaded = await loadSession(sessionPath);
      expect(loaded!.claudeSessionId).toBe("claude-123");
    });

    it("returns null for non-existent session", async () => {
      const result = await loadSession(
        path.join(TEST_DIR, ".rafa", "sessions", "nonexistent.jsonl"),
      );
      expect(result).toBeNull();
    });

    it("throws SessionResumeError for corrupted session", async () => {
      const sessionPath = path.join(
        TEST_DIR,
        ".rafa",
        "sessions",
        "prd-corrupted.jsonl",
      );
      await fs.writeFile(sessionPath, "invalid json");

      await expect(loadSession(sessionPath)).rejects.toBeInstanceOf(
        SessionResumeError,
      );
    });

    it("throws for session file without metadata line", async () => {
      const sessionPath = path.join(
        TEST_DIR,
        ".rafa",
        "sessions",
        "prd-empty.jsonl",
      );
      await fs.writeFile(sessionPath, "");

      await expect(loadSession(sessionPath)).rejects.toThrow(
        "Session file is empty",
      );
    });
  });

  describe("loadSessionByName", () => {
    it("loads a session by phase and name", async () => {
      const session = createTestSession();
      await createSessionFile(session, "my-feature", TEST_DIR);

      const loaded = await loadSessionByName("prd", "my-feature", TEST_DIR);
      expect(loaded).not.toBeNull();
      expect(loaded!.id).toBe(session.id);
    });

    it("returns null when session does not exist", async () => {
      const loaded = await loadSessionByName("prd", "nonexistent", TEST_DIR);
      expect(loaded).toBeNull();
    });
  });

  describe("appendUserMessage", () => {
    it("appends a user message to session", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const msgId = await appendUserMessage(sessionPath, "Hello, Claude!");

      const loaded = await loadSession(sessionPath);
      expect(loaded!.messages).toHaveLength(1);
      expect(loaded!.messages[0].type).toBe("user");
      expect(loaded!.messages[0].id).toBe(msgId);
      expect(loaded!.messages[0].content).toBe("Hello, Claude!");
      expect(loaded!.messages[0].parentId).toBeNull();
    });

    it("appends user message with parentId", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const msg1Id = await appendUserMessage(sessionPath, "First message");
      const msg2Id = await appendUserMessage(sessionPath, "Follow up", msg1Id);

      const loaded = await loadSession(sessionPath);
      expect(loaded!.messages).toHaveLength(2);
      expect(loaded!.messages[1].parentId).toBe(msg1Id);
      expect(loaded!.messages[1].id).toBe(msg2Id);
    });
  });

  describe("appendAssistantMessage", () => {
    it("appends an assistant message to session", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const userMsgId = await appendUserMessage(sessionPath, "Hello");
      const assistantMsgId = await appendAssistantMessage(
        sessionPath,
        "Hi there!",
        userMsgId,
      );

      const loaded = await loadSession(sessionPath);
      expect(loaded!.messages).toHaveLength(2);
      expect(loaded!.messages[1].type).toBe("assistant");
      expect(loaded!.messages[1].id).toBe(assistantMsgId);
      expect(loaded!.messages[1].content).toBe("Hi there!");
      expect(loaded!.messages[1].parentId).toBe(userMsgId);
    });
  });

  describe("appendToolUse", () => {
    it("appends a tool use entry to session", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const assistantMsgId = await appendAssistantMessage(
        sessionPath,
        "Let me read that file",
        null,
      );
      const toolId = await appendToolUse(
        sessionPath,
        "Read",
        assistantMsgId,
        "/path/to/file.ts",
        { encoding: "utf-8" },
      );

      const loaded = await loadSession(sessionPath);
      expect(loaded!.messages).toHaveLength(2);
      expect(loaded!.messages[1].type).toBe("tool_use");
      expect(loaded!.messages[1].id).toBe(toolId);
      expect(loaded!.messages[1].tool).toBe("Read");
      expect(loaded!.messages[1].target).toBe("/path/to/file.ts");
      expect(loaded!.messages[1].input).toEqual({ encoding: "utf-8" });
      expect(loaded!.messages[1].parentId).toBe(assistantMsgId);
    });
  });

  describe("updateSessionMetadata", () => {
    it("updates claudeSessionId", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      await updateSessionMetadata(sessionPath, {
        claudeSessionId: "new-claude-id",
      });

      const loaded = await loadSession(sessionPath);
      expect(loaded!.claudeSessionId).toBe("new-claude-id");
    });

    it("updates updatedAt timestamp", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      await updateSessionMetadata(sessionPath, {
        updatedAt: "2024-06-15T12:00:00.000Z",
      });

      const loaded = await loadSession(sessionPath);
      expect(loaded!.updatedAt).toBe("2024-06-15T12:00:00.000Z");
    });

    it("preserves existing messages when updating metadata", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      await appendUserMessage(sessionPath, "Hello");
      await appendAssistantMessage(sessionPath, "Hi!", null);

      await updateSessionMetadata(sessionPath, {
        claudeSessionId: "updated-id",
      });

      const loaded = await loadSession(sessionPath);
      expect(loaded!.messages).toHaveLength(2);
      expect(loaded!.claudeSessionId).toBe("updated-id");
    });
  });

  describe("saveSession", () => {
    it("saves a complete session with messages", async () => {
      const session: LoadedSession = {
        id: "xyz789",
        phase: "design",
        name: "my-design",
        createdAt: "2024-01-01T00:00:00.000Z",
        claudeSessionId: "claude-456",
        messages: [
          {
            type: "user",
            id: "m1",
            parentId: null,
            content: "Create a design for...",
          },
          {
            type: "assistant",
            id: "m2",
            parentId: "m1",
            content: "Let me help with that.",
          },
          {
            type: "tool_use",
            id: "m3",
            parentId: "m2",
            tool: "Read",
            target: "/file.ts",
          },
        ],
      };

      const sessionPath = path.join(
        TEST_DIR,
        ".rafa",
        "sessions",
        "design-my-design.jsonl",
      );
      await saveSession(session, sessionPath);

      const loaded = await loadSession(sessionPath);
      expect(loaded!.id).toBe("xyz789");
      expect(loaded!.phase).toBe("design");
      expect(loaded!.claudeSessionId).toBe("claude-456");
      expect(loaded!.messages).toHaveLength(3);
      expect(loaded!.messages[0].content).toBe("Create a design for...");
      expect(loaded!.messages[2].tool).toBe("Read");
    });
  });

  describe("listSessions", () => {
    it("returns empty array when no sessions directory exists", async () => {
      const emptyDir = path.join(TEST_DIR, "empty");
      await fs.mkdir(emptyDir, { recursive: true });

      const sessions = await listSessions(emptyDir);
      expect(sessions).toEqual([]);
    });

    it("lists all valid sessions", async () => {
      const session1 = createTestSession({ id: "aaa111", phase: "prd" });
      const session2 = createTestSession({ id: "bbb222", phase: "design" });

      await createSessionFile(session1, "session-one", TEST_DIR);
      await createSessionFile(session2, "session-two", TEST_DIR);

      const sessions = await listSessions(TEST_DIR);
      expect(sessions).toHaveLength(2);
    });

    it("skips invalid session files", async () => {
      const validSession = createTestSession();
      await createSessionFile(validSession, "valid-session", TEST_DIR);

      // Create an invalid session file
      await fs.writeFile(
        path.join(TEST_DIR, ".rafa", "sessions", "prd-invalid.jsonl"),
        "invalid json",
      );

      const sessions = await listSessions(TEST_DIR);
      expect(sessions).toHaveLength(1);
      expect(sessions[0].id).toBe("abc123");
    });

    it("sorts sessions by most recent first", async () => {
      const session1 = createTestSession({
        id: "aaa111",
        createdAt: "2024-01-01T00:00:00.000Z",
      });
      const session2 = createTestSession({
        id: "bbb222",
        createdAt: "2024-06-01T00:00:00.000Z",
      });

      await createSessionFile(session1, "older", TEST_DIR);
      await createSessionFile(session2, "newer", TEST_DIR);

      const sessions = await listSessions(TEST_DIR);
      expect(sessions[0].id).toBe("bbb222"); // Newer first
      expect(sessions[1].id).toBe("aaa111");
    });
  });

  describe("deleteSession", () => {
    it("deletes an existing session", async () => {
      const session = createTestSession();
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      const result = await deleteSession(sessionPath);
      expect(result).toBe(true);

      const loaded = await loadSession(sessionPath);
      expect(loaded).toBeNull();
    });

    it("returns false when session does not exist", async () => {
      const result = await deleteSession(
        path.join(TEST_DIR, ".rafa", "sessions", "nonexistent.jsonl"),
      );
      expect(result).toBe(false);
    });
  });

  describe("sessionExists", () => {
    it("returns true when session exists", async () => {
      const session = createTestSession();
      await createSessionFile(session, "test-session", TEST_DIR);

      const exists = await sessionExists("prd", "test-session", TEST_DIR);
      expect(exists).toBe(true);
    });

    it("returns false when session does not exist", async () => {
      const exists = await sessionExists("prd", "nonexistent", TEST_DIR);
      expect(exists).toBe(false);
    });
  });

  describe("resolveSessionName", () => {
    it("returns baseName when no collision", async () => {
      const name = await resolveSessionName("prd", "new-session", TEST_DIR);
      expect(name).toBe("new-session");
    });

    it("appends suffix when name exists", async () => {
      const session = createTestSession();
      await createSessionFile(session, "my-session", TEST_DIR);

      const name = await resolveSessionName("prd", "My Session", TEST_DIR);
      expect(name).toBe("my-session-2");
    });

    it("increments suffix for multiple collisions", async () => {
      const session1 = createTestSession({ id: "aaa111" });
      const session2 = createTestSession({ id: "bbb222" });
      await createSessionFile(session1, "my-session", TEST_DIR);
      await createSessionFile(session2, "my-session-2", TEST_DIR);

      const name = await resolveSessionName("prd", "My Session", TEST_DIR);
      expect(name).toBe("my-session-3");
    });

    it("normalizes spaces to hyphens", async () => {
      const name = await resolveSessionName(
        "prd",
        "User Auth Feature",
        TEST_DIR,
      );
      expect(name).toBe("user-auth-feature");
    });
  });

  describe("tryResumeSession", () => {
    it("returns success with session when found", async () => {
      const session = createTestSession({ claudeSessionId: "claude-123" });
      await createSessionFile(session, "test-session", TEST_DIR);

      const result = await tryResumeSession("prd", "test-session", TEST_DIR);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.session.id).toBe("abc123");
        expect(result.session.claudeSessionId).toBe("claude-123");
      }
    });

    it("returns failure when session not found", async () => {
      const result = await tryResumeSession("prd", "nonexistent", TEST_DIR);
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.message).toContain("Session not found");
      }
    });

    it("returns failure for corrupted session", async () => {
      const sessionPath = path.join(
        TEST_DIR,
        ".rafa",
        "sessions",
        "prd-corrupted.jsonl",
      );
      await fs.writeFile(sessionPath, "invalid json");

      const result = await tryResumeSession("prd", "corrupted", TEST_DIR);
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error).toBeInstanceOf(SessionResumeError);
      }
    });
  });

  describe("startFreshSession", () => {
    it("creates a new session", async () => {
      const result = await startFreshSession("prd", "fresh-start", TEST_DIR);

      expect(result.session.phase).toBe("prd");
      expect(result.session.name).toBe("fresh-start");
      expect(result.sessionPath).toContain("prd-fresh-start.jsonl");

      const loaded = await loadSession(result.sessionPath);
      expect(loaded).not.toBeNull();
    });

    it("deletes existing session before creating new one", async () => {
      // Create existing session with some messages
      const oldSession = createTestSession({ id: "old-id" });
      const oldPath = await createSessionFile(
        oldSession,
        "my-session",
        TEST_DIR,
      );
      await appendUserMessage(oldPath, "Old message");

      // Start fresh
      const result = await startFreshSession("prd", "my-session", TEST_DIR);

      expect(result.session.id).not.toBe("old-id");
      const loaded = await loadSession(result.sessionPath);
      expect(loaded!.messages).toHaveLength(0);
    });
  });

  describe("JSONL format compliance", () => {
    it("produces format matching design spec", async () => {
      const session = createTestSession({ claudeSessionId: "abc123" });
      const sessionPath = await createSessionFile(
        session,
        "test-session",
        TEST_DIR,
      );

      await appendUserMessage(sessionPath, "I want to build...");

      // Read raw file content
      const content = await fs.readFile(sessionPath, "utf-8");
      const lines = content.trim().split("\n");

      expect(lines).toHaveLength(2);

      // First line: session metadata
      const metadata = JSON.parse(lines[0]);
      expect(metadata.type).toBe("session");
      expect(metadata.id).toBe("abc123");
      expect(metadata.phase).toBe("prd");

      // Second line: user message
      const userMsg = JSON.parse(lines[1]);
      expect(userMsg.type).toBe("user");
      expect(userMsg.parentId).toBeNull();
      expect(userMsg.content).toBe("I want to build...");
    });
  });
});
