/**
 * Execution loop for running plan tasks
 *
 * Implements sequential task execution with retry support.
 * Reference: internal/executor/executor.go
 */

import type { Plan } from "./plan.js";
import type { Task } from "./task.js";
import type { ClaudeEvent, ClaudeAbortController } from "./claude-runner.js";
import type { TextEventData } from "./stream-parser.js";
import { runTask, createAbortController, ClaudeAbortError } from "./claude-runner.js";
import { ProgressLogger } from "./progress.js";
import { PlanLock } from "../utils/lock.js";
import * as git from "../utils/git.js";
import * as plans from "../storage/plans.js";

// Max retry attempts per task
export const MAX_ATTEMPTS = 5;

// Commit message marker for extraction
const COMMIT_MESSAGE_PREFIX = "SUGGESTED_COMMIT_MESSAGE:";

/**
 * Events emitted by the executor for TUI integration
 */
export interface ExecutorEvents {
  onPlanStart?: (plan: Plan) => void;
  onPlanComplete?: (completed: number, total: number, durationMs: number) => void;
  onPlanFailed?: (task: Task, reason: string) => void;
  onPlanCancelled?: (taskId: string) => void;
  onTaskStart?: (taskIndex: number, total: number, task: Task, attempt: number) => void;
  onTaskComplete?: (task: Task) => void;
  onTaskFailed?: (task: Task, attempt: number, error: Error) => void;
  onClaudeEvent?: (event: ClaudeEvent) => void;
  onPlanSaved?: () => void;
}

/**
 * Options for running a plan
 */
export interface ExecutorOptions {
  planDir: string;
  plan: Plan;
  repoRoot?: string;
  allowDirty?: boolean;
  skipPersistence?: boolean;
  events?: ExecutorEvents;
}

/**
 * Executor runs all tasks in a plan with retry support.
 */
export class Executor {
  private readonly planDir: string;
  private plan: Plan;
  private readonly repoRoot: string;
  private readonly allowDirty: boolean;
  private readonly skipPersistence: boolean;
  private readonly events?: ExecutorEvents;
  private readonly lock: PlanLock;
  private readonly logger: ProgressLogger;
  private startTime: number = 0;
  private aborted = false;
  private capturedOutput: string[] = []; // Capture output for commit message extraction
  private currentAbortController: ClaudeAbortController | null = null;

  constructor(options: ExecutorOptions) {
    this.planDir = options.planDir;
    this.plan = options.plan;
    this.repoRoot = options.repoRoot || process.cwd();
    this.allowDirty = options.allowDirty ?? false;
    this.skipPersistence = options.skipPersistence ?? false;
    this.events = options.events;
    this.lock = new PlanLock(this.planDir);
    this.logger = new ProgressLogger(this.planDir);
  }

  /**
   * Abort the execution - kills any running Claude process and stops the loop
   */
  abort(): void {
    this.aborted = true;
    // Kill the current Claude process if running
    if (this.currentAbortController) {
      this.currentAbortController.abort();
    }
  }

  /**
   * Run executes all pending tasks in the plan.
   * Acquires a lock, processes tasks sequentially, and handles retries.
   */
  async run(): Promise<void> {
    // Skip lock acquisition in demo mode (no persistence)
    if (!this.skipPersistence) {
      await this.lock.acquire();
    }

    try {
      await this.runInternal();
    } finally {
      if (!this.skipPersistence) {
        this.lock.release();
      }
    }
  }

  private async runInternal(): Promise<void> {
    // Check workspace cleanliness before starting
    // Skip in demo mode (skipPersistence implies allowDirty)
    if (!this.allowDirty) {
      const status = await git.getStatus(this.repoRoot);
      // Filter out our lock file from the dirty files list
      const dirtyFiles = this.filterOutLockFile(status.files);
      if (dirtyFiles.length > 0) {
        throw this.workspaceDirtyError(dirtyFiles);
      }
    }

    // Check if all tasks are already completed
    if (plans.allTasksCompleted(this.plan)) {
      this.events?.onPlanComplete?.(
        this.plan.tasks.length,
        this.plan.tasks.length,
        0
      );
      return;
    }

    // Find first pending task
    const firstIdx = plans.firstPendingTask(this.plan);
    if (firstIdx === -1) {
      this.events?.onPlanComplete?.(
        this.countCompleted(),
        this.plan.tasks.length,
        0
      );
      return;
    }

    // If re-running a failed plan, reset attempts on the blocking task
    if (this.plan.status === "failed") {
      const task = this.plan.tasks[firstIdx];
      if (task.attempts >= MAX_ATTEMPTS) {
        task.attempts = 0;
        task.status = "pending";
      }
    }

    // Update plan status if not started
    if (this.plan.status === "not_started") {
      this.plan.status = "in_progress";
      if (!this.skipPersistence) {
        await plans.savePlan(this.planDir, this.plan);
        this.events?.onPlanSaved?.();
      }
    }

    // Log plan started and record start time
    this.startTime = Date.now();
    this.events?.onPlanStart?.(this.plan);
    if (!this.skipPersistence) {
      await this.logger.planStarted(this.plan.id);
    }

    // Build plan context once
    const planContext = this.buildPlanContext();

    // Execute tasks from first pending
    for (let i = firstIdx; i < this.plan.tasks.length; i++) {
      const task = this.plan.tasks[i];

      // Skip completed tasks (for resume scenarios)
      if (task.status === "completed") {
        continue;
      }

      try {
        await this.executeTask(task, i, planContext);
      } catch (err) {
        if (this.aborted) {
          // Aborted - reset task to pending
          task.status = "pending";
          if (!this.skipPersistence) {
            await plans.savePlan(this.planDir, this.plan);
            this.events?.onPlanSaved?.();
            await this.logger.planCancelled(task.id);
          }
          this.events?.onPlanCancelled?.(task.id);
          return;
        }

        // Max attempts reached - mark plan as failed
        this.plan.status = "failed";
        if (!this.skipPersistence) {
          await plans.savePlan(this.planDir, this.plan);
          this.events?.onPlanSaved?.();
          await this.logger.planFailed(task.id, task.attempts);
        }
        this.events?.onPlanFailed?.(
          task,
          `failed after ${task.attempts} attempts`
        );
        throw new Error(`task ${task.id} failed after ${task.attempts} attempts`);
      }
    }

    // All tasks completed
    this.plan.status = "completed";
    if (!this.skipPersistence) {
      await plans.savePlan(this.planDir, this.plan);
      this.events?.onPlanSaved?.();
    }

    const durationMs = Date.now() - this.startTime;
    if (!this.skipPersistence) {
      await this.logger.planCompleted(
        this.plan.tasks.length,
        this.countCompleted(),
        durationMs
      );
    }

    // Commit any remaining metadata (plan completion status)
    if (!this.allowDirty) {
      const msg = `[rafa] Complete plan: ${this.plan.name} (${this.plan.tasks.length} tasks)`;
      try {
        await git.commitAll(msg, this.repoRoot);
      } catch {
        // Warning: failed to commit plan completion - non-critical
      }
    }

    this.events?.onPlanComplete?.(
      this.countCompleted(),
      this.plan.tasks.length,
      durationMs
    );
  }

  /**
   * Execute a single task with retry logic
   */
  private async executeTask(
    task: Task,
    idx: number,
    planContext: string
  ): Promise<void> {
    while (task.attempts < MAX_ATTEMPTS) {
      // Check for abort before starting
      if (this.aborted) {
        throw new Error("aborted");
      }

      // Increment attempts and set in_progress
      task.attempts++;
      task.status = "in_progress";
      if (!this.skipPersistence) {
        await plans.savePlan(this.planDir, this.plan);
        this.events?.onPlanSaved?.();
      }

      // Emit task start event
      this.events?.onTaskStart?.(
        idx + 1,
        this.plan.tasks.length,
        task,
        task.attempts
      );

      // Log task started
      if (!this.skipPersistence) {
        await this.logger.taskStarted(task.id, task.attempts);
      }

      // Clear captured output for this attempt
      this.capturedOutput = [];

      // Build the prompt for this task
      const prompt = this.buildPrompt(task, planContext, task.attempts, MAX_ATTEMPTS);

      // Create a new abort controller for this task attempt
      this.currentAbortController = createAbortController();

      try {
        // Run the task with fresh Claude session (no --resume)
        await runTask({
          prompt,
          cwd: this.repoRoot,
          abortController: this.currentAbortController,
          onEvent: (event: ClaudeEvent) => {
            // Capture text output for commit message extraction
            this.captureEventText(event);
            // Forward to TUI
            this.events?.onClaudeEvent?.(event);
          },
        });

        // Task succeeded - update metadata and commit everything
        task.status = "completed";
        if (!this.skipPersistence) {
          await plans.savePlan(this.planDir, this.plan);
          this.events?.onPlanSaved?.();
          await this.logger.taskCompleted(task.id);
        }

        // Commit all changes (implementation + metadata) unless allowDirty
        if (!this.allowDirty) {
          const commitMsg = this.getCommitMessage(task);
          await git.commitAll(commitMsg, this.repoRoot);

          // Verify workspace is clean after commit
          const status = await git.getStatus(this.repoRoot);
          if (!status.clean) {
            throw new Error(
              `workspace not clean after commit (possibly git hooks modified files): ${status.files.join(", ")}`
            );
          }
        }

        this.events?.onTaskComplete?.(task);
        this.currentAbortController = null;
        return;
      } catch (err) {
        this.currentAbortController = null;

        // Check if this was an abort
        if (this.aborted || err instanceof ClaudeAbortError) {
          throw new Error("aborted");
        }

        // Task failed
        const error = err instanceof Error ? err : new Error(String(err));

        if (!this.skipPersistence) {
          await this.logger.taskFailed(task.id, task.attempts);
        }
        this.events?.onTaskFailed?.(task, task.attempts, error);

        // Check if max attempts reached
        if (task.attempts >= MAX_ATTEMPTS) {
          task.status = "failed";
          if (!this.skipPersistence) {
            await plans.savePlan(this.planDir, this.plan);
            this.events?.onPlanSaved?.();
          }
          throw new Error("max attempts reached");
        }

        // Spinning up fresh agent for retry (no --resume)
        // Loop continues to next attempt
      }
    }

    throw new Error("max attempts reached");
  }

  /**
   * Build the task prompt template matching Go version.
   * Reference: internal/executor/runner.go lines 54-94
   */
  private buildPrompt(
    task: Task,
    planContext: string,
    attempt: number,
    maxAttempts: number
  ): string {
    const lines: string[] = [];

    lines.push("You are executing a task as part of an automated plan.");
    lines.push("");
    lines.push("## Context");
    lines.push(planContext);
    lines.push("");

    lines.push("## Your Task");
    lines.push(`**ID**: ${task.id}`);
    lines.push(`**Title**: ${task.title}`);
    lines.push(`**Attempt**: ${attempt} of ${maxAttempts}`);
    lines.push(`**Description**: ${task.description}`);
    lines.push("");

    // Add retry note if not first attempt
    if (attempt > 1) {
      lines.push("**Note**: Previous attempts to complete this task failed. ");
      lines.push("Consider alternative approaches or investigate what went wrong. ");
      lines.push("Review any uncommitted changes from previous attempts - you may be able to continue from where they left off. ");
      lines.push("Use `git status` and `git diff` to see what was changed.");
      lines.push("");
    }

    lines.push("## Acceptance Criteria");
    lines.push("You MUST verify ALL of the following before considering the task complete:");
    for (let i = 0; i < task.acceptanceCriteria.length; i++) {
      lines.push(`${i + 1}. ${task.acceptanceCriteria[i]}`);
    }
    lines.push("");

    lines.push("## Instructions");
    lines.push("1. Implement the task as described");
    lines.push("2. Verify ALL acceptance criteria are met");
    lines.push("3. If you need additional context on requirements or implementation details, consult the Source document listed in the Context section above");
    lines.push("4. Before finalizing, perform a code review of your changes. If you have a code review skill available (e.g., `/code-review`), use it to review your implementation and assess what findings are worth addressing vs. acceptable trade-offs");
    lines.push("5. DO NOT commit your changes - the orchestrator will handle the commit");
    lines.push("6. Output a suggested commit message in this exact format: SUGGESTED_COMMIT_MESSAGE: <your descriptive commit message>");
    lines.push("");

    lines.push("IMPORTANT: Leave changes uncommitted. The orchestrator will commit after validating. Do not declare success unless ALL acceptance criteria are met.");

    return lines.join("\n");
  }

  /**
   * Build plan context string
   */
  private buildPlanContext(): string {
    return `Plan: ${this.plan.name}\nDescription: ${this.plan.description}\nSource: ${this.plan.sourceFile}`;
  }

  /**
   * Capture text from Claude events for commit message extraction
   *
   * ClaudeEvent is a simplified wrapper around raw events.
   * Events with type "text" contain TextEventData with the text content.
   */
  private captureEventText(event: ClaudeEvent): void {
    // Capture text events (both partial streaming and complete messages)
    if (event.type === "text") {
      const textData = event.data as TextEventData;
      if (textData.text) {
        this.capturedOutput.push(textData.text);
      }
    }
  }

  /**
   * Extract commit message from captured output or fall back to default
   */
  private getCommitMessage(task: Task): string {
    // Search captured output for commit message
    for (let i = this.capturedOutput.length - 1; i >= 0; i--) {
      const text = this.capturedOutput[i];
      const idx = text.indexOf(COMMIT_MESSAGE_PREFIX);
      if (idx !== -1) {
        let msg = text.slice(idx + COMMIT_MESSAGE_PREFIX.length);
        // Take up to the first newline
        const nlIdx = msg.indexOf("\n");
        if (nlIdx !== -1) {
          msg = msg.slice(0, nlIdx);
        }
        msg = msg.trim();
        if (msg) {
          return msg;
        }
      }
    }

    // Fall back to default message format
    return `[rafa] Complete task ${task.id}: ${task.title}`;
  }

  /**
   * Count completed tasks
   */
  private countCompleted(): number {
    return this.plan.tasks.filter((t) => t.status === "completed").length;
  }

  /**
   * Filter out the lock file from dirty files list
   */
  private filterOutLockFile(files: string[]): string[] {
    const lockPath = `.rafa/plans/${this.plan.id}-${this.plan.name}/run.lock`;
    return files.filter((f) => f !== lockPath);
  }

  /**
   * Create workspace dirty error
   */
  private workspaceDirtyError(files: string[]): Error {
    const fileList = files.map((f) => `  ${f}`).join("\n");
    return new Error(
      `workspace has uncommitted changes before starting plan\n\nModified files:\n${fileList}\n\nPlease commit or stash your changes before running the plan.\nOr use --allow-dirty to skip this check (not recommended).`
    );
  }
}
