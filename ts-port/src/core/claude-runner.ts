/**
 * Claude CLI invocation
 *
 * Provides two modes:
 * - runTask: Fresh session for task execution, auto-approves all permissions
 * - runConversation: Resumable sessions for PRD/Design creation
 */

import { spawn, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import {
  parseClaudeEvent,
  extractSessionId,
  type ClaudeEvent,
  type ClaudeEventType,
  type SystemEventData,
} from "./stream-parser.js";

// Re-export event types for convenience
export type { ClaudeEvent, ClaudeEventType };

/**
 * AbortController-like interface for cancelling Claude runs
 */
export interface ClaudeAbortController {
  abort(): void;
  readonly aborted: boolean;
}

/**
 * Creates an abort controller for Claude runs
 */
export function createAbortController(): ClaudeAbortController {
  let aborted = false;
  let childProcess: ChildProcess | null = null;

  return {
    abort() {
      aborted = true;
      // Capture the reference to avoid race condition in setTimeout
      const proc = childProcess;
      if (proc) {
        // Send SIGTERM for graceful shutdown, then SIGKILL if needed
        proc.kill("SIGTERM");
        // Give Claude a moment to clean up, then force kill
        setTimeout(() => {
          if (proc && !proc.killed) {
            proc.kill("SIGKILL");
          }
        }, 1000);
      }
    },
    get aborted() {
      return aborted;
    },
    // Internal: set the child process to control
    set _process(proc: ChildProcess | null) {
      childProcess = proc;
    },
  } as ClaudeAbortController & { _process: ChildProcess | null };
}

export interface TaskRunOptions {
  prompt: string;
  onEvent: (event: ClaudeEvent) => void;
  cwd?: string;
  abortController?: ClaudeAbortController;
}

export interface ConversationRunOptions extends TaskRunOptions {
  sessionId?: string;
}

export interface RunResult {
  sessionId: string;
  exitCode: number;
}

/**
 * Run a task in a fresh Claude session
 *
 * Uses flags for task execution:
 * - -p: Prompt
 * - --output-format stream-json: JSONL output for parsing
 * - --dangerously-skip-permissions: Auto-approve all tool use
 * - --verbose: Include additional context
 * - --include-partial-messages: Stream partial responses
 */
export async function runTask(options: TaskRunOptions): Promise<void> {
  const args = [
    "-p",
    options.prompt,
    "--output-format",
    "stream-json",
    "--dangerously-skip-permissions",
    "--verbose",
    "--include-partial-messages",
  ];

  const { exitCode } = await runClaude(
    args,
    options.onEvent,
    options.cwd,
    options.abortController,
  );

  // Check if aborted
  if (options.abortController?.aborted) {
    throw new ClaudeAbortError("Task was aborted");
  }

  if (exitCode !== 0) {
    throw new ClaudeError(`Claude exited with code ${exitCode}`, exitCode);
  }
}

/**
 * Run a conversation with Claude, optionally resuming a previous session
 *
 * Uses flags for conversation mode:
 * - -p: Prompt
 * - --output-format stream-json: JSONL output for parsing
 * - --dangerously-skip-permissions: Auto-approve all tool use
 * - --verbose: Include additional context
 * - --resume: Resume previous session (if sessionId provided)
 *
 * Returns the session ID for future resume
 */
export async function runConversation(
  options: ConversationRunOptions,
): Promise<string> {
  const args = [
    "-p",
    options.prompt,
    "--output-format",
    "stream-json",
    "--dangerously-skip-permissions",
    "--verbose",
  ];

  if (options.sessionId) {
    args.push("--resume", options.sessionId);
  }

  let capturedSessionId: string | null = null;

  const wrappedOnEvent = (event: ClaudeEvent) => {
    // Capture session ID from system events
    const sessionId = extractSessionId(event);
    if (sessionId) {
      capturedSessionId = sessionId;
    }

    // Pass through to caller
    options.onEvent(event);
  };

  const { exitCode } = await runClaude(args, wrappedOnEvent, options.cwd);

  if (exitCode !== 0) {
    throw new ClaudeError(`Claude exited with code ${exitCode}`, exitCode);
  }

  if (!capturedSessionId) {
    throw new ClaudeError("No session ID found in Claude output", exitCode);
  }

  return capturedSessionId;
}

/**
 * Core Claude CLI runner
 */
async function runClaude(
  args: string[],
  onEvent: (event: ClaudeEvent) => void,
  cwd?: string,
  abortController?: ClaudeAbortController,
): Promise<RunResult> {
  return new Promise((resolve, reject) => {
    // Check if already aborted before starting
    if (abortController?.aborted) {
      resolve({ sessionId: "", exitCode: -1 });
      return;
    }

    const proc = spawn("claude", args, {
      cwd,
      stdio: ["ignore", "pipe", "pipe"],
    });

    // Allow abort controller to kill the process
    if (abortController && "_process" in abortController) {
      (abortController as { _process: ChildProcess | null })._process = proc;
    }

    let sessionId = "";
    let stderrOutput = "";

    // Create readline interface for line-by-line parsing
    const rl = createInterface({
      input: proc.stdout,
      crlfDelay: Infinity,
    });

    rl.on("line", (line: string) => {
      if (!line.trim()) return;

      try {
        const event = parseClaudeEvent(line);

        // Capture session ID
        if (event.type === "system") {
          sessionId = (event.data as SystemEventData).sessionId;
        }

        onEvent(event);
      } catch (err) {
        // Log parse errors but continue processing
        console.error("Failed to parse Claude event:", err, "Line:", line);
      }
    });

    // Collect stderr for error reporting
    proc.stderr.on("data", (data: Buffer) => {
      stderrOutput += data.toString();
    });

    proc.on("error", (err) => {
      reject(new ClaudeError(`Failed to spawn claude: ${err.message}`, -1));
    });

    proc.on("close", (code) => {
      // Clear the process reference
      if (abortController && "_process" in abortController) {
        (abortController as { _process: ChildProcess | null })._process = null;
      }

      const exitCode = code ?? 0;

      // Don't log stderr if aborted - that's expected
      if (exitCode !== 0 && stderrOutput && !abortController?.aborted) {
        console.error("Claude stderr:", stderrOutput);
      }

      resolve({ sessionId, exitCode });
    });
  });
}

/**
 * Custom error class for Claude-related errors
 */
export class ClaudeError extends Error {
  constructor(
    message: string,
    public readonly exitCode: number,
  ) {
    super(message);
    this.name = "ClaudeError";
  }
}

/**
 * Error thrown when a Claude run is aborted
 */
export class ClaudeAbortError extends Error {
  constructor(message: string = "Claude run was aborted") {
    super(message);
    this.name = "ClaudeAbortError";
  }
}

/**
 * Check if Claude CLI is available
 */
export async function isClaudeAvailable(): Promise<boolean> {
  return new Promise((resolve) => {
    const proc = spawn("claude", ["--version"], {
      stdio: ["ignore", "pipe", "pipe"],
    });

    proc.on("error", () => {
      resolve(false);
    });

    proc.on("close", (code) => {
      resolve(code === 0);
    });
  });
}
