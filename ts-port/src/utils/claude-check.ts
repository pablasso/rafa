/**
 * Claude CLI availability check
 */

import { spawn } from "node:child_process";

export interface ClaudeCliCheckResult {
  available: boolean;
  message: string;
  version?: string;
}

const CHECK_TIMEOUT_MS = 5000;

/**
 * Check if the Claude CLI is available and working
 */
export async function checkClaudeCli(): Promise<ClaudeCliCheckResult> {
  return new Promise((resolve) => {
    const proc = spawn("claude", ["--version"], {
      stdio: ["ignore", "pipe", "pipe"],
    });

    let stdout = "";
    let stderr = "";
    let resolved = false;

    const timeout = setTimeout(() => {
      if (!resolved) {
        resolved = true;
        proc.kill();
        resolve({
          available: false,
          message:
            "Claude CLI check timed out. Please ensure claude is installed and responding.",
        });
      }
    }, CHECK_TIMEOUT_MS);

    const cleanup = () => {
      clearTimeout(timeout);
    };

    proc.stdout.on("data", (data: Buffer) => {
      stdout += data.toString();
    });

    proc.stderr.on("data", (data: Buffer) => {
      stderr += data.toString();
    });

    proc.on("error", (error: NodeJS.ErrnoException) => {
      if (resolved) return;
      resolved = true;
      cleanup();

      if (error.code === "ENOENT") {
        resolve({
          available: false,
          message: `Claude CLI not found. Please install it first:

  npm install -g @anthropic-ai/claude-code

For more information, visit: https://docs.anthropic.com/claude-code`,
        });
      } else {
        resolve({
          available: false,
          message: `Failed to run Claude CLI: ${error.message}`,
        });
      }
    });

    proc.on("close", (code) => {
      if (resolved) return;
      resolved = true;
      cleanup();

      if (code === 0) {
        const version = stdout.trim();
        resolve({
          available: true,
          message: `Claude CLI found: ${version}`,
          version,
        });
      } else {
        resolve({
          available: false,
          message:
            `Claude CLI check failed (exit code ${code}): ${stderr || stdout}`.trim(),
        });
      }
    });
  });
}
