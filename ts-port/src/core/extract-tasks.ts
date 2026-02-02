/**
 * Task extraction from design documents using Claude CLI
 *
 * This module provides the ability to extract tasks from markdown design
 * documents using Claude Code CLI in non-interactive mode.
 */

import { spawn } from "node:child_process";

/**
 * Extracted task from Claude's response
 */
export interface ExtractedTask {
  title: string;
  description: string;
  acceptanceCriteria: string[];
}

/**
 * Result of task extraction from a design document
 */
export interface TaskExtractionResult {
  name: string;
  description: string;
  tasks: ExtractedTask[];
}

/**
 * Error thrown when task extraction fails
 */
export class TaskExtractionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "TaskExtractionError";
  }
}

/**
 * Claude CLI response structure when using --output-format json
 */
interface ClaudeResponse {
  type: string;
  result: string;
  is_error: boolean;
}

/**
 * Default timeout for task extraction (5 minutes)
 */
const DEFAULT_EXTRACTION_TIMEOUT_MS = 5 * 60 * 1000;

/**
 * Builds the extraction prompt for a design document.
 * This matches the Go version's prompt exactly.
 */
function buildExtractionPrompt(designContent: string): string {
  return `You are a technical project planner. Analyze this design document and extract discrete implementation tasks.

DESIGN DOCUMENT:
${designContent}

OUTPUT REQUIREMENTS:
Return a JSON object with this exact structure:
{
  "name": "kebab-case-name-from-document",
  "description": "One sentence describing the overall goal",
  "tasks": [
    {
      "title": "Short imperative title (e.g., 'Implement user login endpoint')",
      "description": "Detailed description of what needs to be done. Include relevant context from the design.",
      "acceptanceCriteria": [
        "Specific, verifiable criterion (e.g., 'npm test passes')",
        "Another measurable criterion",
        "Prefer runnable checks over prose"
      ]
    }
  ]
}

TASK GUIDELINES:
- Each task should use roughly 50-60% of an AI agent's context window
- Tasks must be completable in sequence (later tasks can depend on earlier ones)
- Acceptance criteria must be verifiable by the agent itself
- Prefer criteria that can be verified with commands (tests, type checks, lint)
- Include 2-5 acceptance criteria per task
- Order tasks by implementation dependency

TESTING REQUIREMENTS:
- Tasks that implement new functionality MUST include writing unit tests as part of their acceptance criteria
- If the design document has a "Testing" section, incorporate those test scenarios into the relevant task acceptance criteria
- Test acceptance criteria should be specific (e.g., "Unit tests exist for checkGitRepo() covering: in git repo, not in git repo")
- Do NOT use vague criteria like "tests pass" - specify what tests should be written

COMPLETENESS CHECK:
- Every section of the design document must be covered by at least one task
- Sections like "Testing", "Edge Cases", "Error Handling", "Validation" contain specific scenarios that MUST appear in acceptance criteria
- If the design specifies a specific behavior, error message, or scenario, it should be verifiable in the acceptance criteria
- When in doubt, include the requirement rather than omit it

Return ONLY the JSON, no markdown formatting or explanation.`;
}

/**
 * Strips markdown code block markers from a string
 */
function stripMarkdownCodeBlocks(s: string): string {
  s = s.trim();
  // Check for ```json or ``` at start
  if (s.startsWith("```json")) {
    s = s.slice(7);
  } else if (s.startsWith("```")) {
    s = s.slice(3);
  }
  // Check for ``` at end
  if (s.endsWith("```")) {
    s = s.slice(0, -3);
  }
  return s.trim();
}

/**
 * Extracts JSON from potentially noisy Claude output
 */
function extractJSON(data: string): string {
  // First, try to parse as Claude Code CLI response wrapper
  try {
    const claudeResp = JSON.parse(data) as ClaudeResponse;
    if (claudeResp.type === "result") {
      if (claudeResp.is_error) {
        throw new TaskExtractionError(
          "Claude returned an error: " + claudeResp.result,
        );
      }
      // Extract the result field and process it
      data = claudeResp.result;
    }
  } catch (e) {
    // Not a Claude response wrapper, continue with raw data
    if (e instanceof TaskExtractionError) {
      throw e;
    }
  }

  // Strip markdown code blocks if present
  let str = stripMarkdownCodeBlocks(data);

  // Try direct parse
  try {
    JSON.parse(str);
    return str;
  } catch {
    // Continue to fallback
  }

  // Find JSON object boundaries as fallback
  const start = str.indexOf("{");
  const end = str.lastIndexOf("}");

  if (start === -1 || end === -1 || start >= end) {
    throw new TaskExtractionError("No JSON object found in response");
  }

  const extracted = str.slice(start, end + 1);
  try {
    JSON.parse(extracted);
    return extracted;
  } catch {
    throw new TaskExtractionError("Extracted content is not valid JSON");
  }
}

/**
 * Validates the extraction result structure
 */
function validateExtractionResult(data: unknown): TaskExtractionResult {
  if (typeof data !== "object" || data === null) {
    throw new TaskExtractionError("Extraction result is not an object");
  }

  const d = data as Record<string, unknown>;

  if (typeof d.name !== "string" || d.name.length === 0) {
    throw new TaskExtractionError(
      "Extraction result has invalid or missing 'name'",
    );
  }

  if (typeof d.description !== "string" || d.description.length === 0) {
    throw new TaskExtractionError(
      "Extraction result has invalid or missing 'description'",
    );
  }

  if (!Array.isArray(d.tasks) || d.tasks.length === 0) {
    throw new TaskExtractionError(
      "Extraction result has invalid or empty 'tasks' array",
    );
  }

  const tasks: ExtractedTask[] = [];
  for (let i = 0; i < d.tasks.length; i++) {
    const t = d.tasks[i] as Record<string, unknown>;

    if (typeof t !== "object" || t === null) {
      throw new TaskExtractionError(`Task at index ${i} is not an object`);
    }

    if (typeof t.title !== "string" || t.title.length === 0) {
      throw new TaskExtractionError(
        `Task at index ${i} has invalid or missing 'title'`,
      );
    }

    if (typeof t.description !== "string" || t.description.length === 0) {
      throw new TaskExtractionError(
        `Task at index ${i} has invalid or missing 'description'`,
      );
    }

    if (!Array.isArray(t.acceptanceCriteria) || t.acceptanceCriteria.length === 0) {
      throw new TaskExtractionError(
        `Task at index ${i} has invalid or empty 'acceptanceCriteria'`,
      );
    }

    for (let j = 0; j < t.acceptanceCriteria.length; j++) {
      if (typeof t.acceptanceCriteria[j] !== "string") {
        throw new TaskExtractionError(
          `Task at index ${i} has invalid acceptanceCriteria at index ${j}`,
        );
      }
    }

    tasks.push({
      title: t.title,
      description: t.description,
      acceptanceCriteria: t.acceptanceCriteria as string[],
    });
  }

  return {
    name: d.name,
    description: d.description,
    tasks,
  };
}

/**
 * Extracts tasks from a design document using Claude CLI
 *
 * @param designContent The markdown content of the design document
 * @param timeoutMs Optional timeout in milliseconds (default: 5 minutes)
 * @returns The extracted tasks and plan metadata
 * @throws TaskExtractionError if extraction fails
 */
export async function extractTasks(
  designContent: string,
  timeoutMs: number = DEFAULT_EXTRACTION_TIMEOUT_MS,
): Promise<TaskExtractionResult> {
  const prompt = buildExtractionPrompt(designContent);

  return new Promise((resolve, reject) => {
    // Execute claude CLI with the prompt
    // --dangerously-skip-permissions is required for non-interactive use
    const proc = spawn(
      "claude",
      ["-p", prompt, "--output-format", "json", "--dangerously-skip-permissions"],
      {
        stdio: ["ignore", "pipe", "pipe"],
      },
    );

    let stdout = "";
    let stderr = "";
    let resolved = false;

    const timeout = setTimeout(() => {
      if (!resolved) {
        resolved = true;
        proc.kill();
        reject(new TaskExtractionError("Task extraction timed out"));
      }
    }, timeoutMs);

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
        reject(
          new TaskExtractionError(
            "Claude Code CLI not found. Install it: https://claude.ai/code",
          ),
        );
      } else {
        reject(
          new TaskExtractionError(
            `Failed to execute claude command: ${error.message}`,
          ),
        );
      }
    });

    proc.on("close", (code) => {
      if (resolved) return;
      resolved = true;
      cleanup();

      if (code !== 0) {
        reject(
          new TaskExtractionError(
            `Claude command failed: ${stderr || `exit code ${code}`}`,
          ),
        );
        return;
      }

      try {
        const jsonStr = extractJSON(stdout);
        const parsed = JSON.parse(jsonStr) as unknown;
        const result = validateExtractionResult(parsed);
        resolve(result);
      } catch (e) {
        if (e instanceof TaskExtractionError) {
          reject(e);
        } else {
          reject(
            new TaskExtractionError(
              `Failed to parse Claude response: ${e instanceof Error ? e.message : String(e)}`,
            ),
          );
        }
      }
    });
  });
}
