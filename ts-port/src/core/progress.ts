/**
 * Progress event logging
 *
 * Implements JSONL progress event logging for plan execution.
 * Events are appended to progress.jsonl in the plan directory.
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";

const PROGRESS_FILE_NAME = "progress.jsonl";

/**
 * Event type constants for progress logging.
 * Matches Go version: internal/plan/progress.go
 */
export const EventType = {
  PlanStarted: "plan_started",
  PlanCompleted: "plan_completed",
  PlanCancelled: "plan_cancelled",
  PlanFailed: "plan_failed",
  TaskStarted: "task_started",
  TaskCompleted: "task_completed",
  TaskFailed: "task_failed",
} as const;

export type ProgressEventType = (typeof EventType)[keyof typeof EventType];

/**
 * Progress event data structure
 */
export interface ProgressEvent {
  timestamp: string;
  event: ProgressEventType;
  data?: Record<string, unknown>;
}

/**
 * ProgressLogger writes progress events to a JSONL file.
 * All writes are append-only.
 */
export class ProgressLogger {
  private readonly filePath: string;

  constructor(planDir: string) {
    this.filePath = path.join(planDir, PROGRESS_FILE_NAME);
  }

  /**
   * Appends a progress event to the log file.
   */
  async log(
    event: ProgressEventType,
    data?: Record<string, unknown>
  ): Promise<void> {
    const entry: ProgressEvent = {
      timestamp: new Date().toISOString(),
      event,
      data,
    };

    const jsonLine = JSON.stringify(entry) + "\n";

    // Open file in append mode, create if not exists
    const handle = await fs.open(this.filePath, "a");
    try {
      await handle.write(jsonLine);
    } finally {
      await handle.close();
    }
  }

  /**
   * Logs a plan_started event.
   */
  async planStarted(planId: string): Promise<void> {
    return this.log(EventType.PlanStarted, { plan_id: planId });
  }

  /**
   * Logs a task_started event.
   */
  async taskStarted(taskId: string, attempt: number): Promise<void> {
    return this.log(EventType.TaskStarted, { task_id: taskId, attempt });
  }

  /**
   * Logs a task_completed event.
   */
  async taskCompleted(taskId: string): Promise<void> {
    return this.log(EventType.TaskCompleted, { task_id: taskId });
  }

  /**
   * Logs a task_failed event.
   */
  async taskFailed(taskId: string, attempt: number): Promise<void> {
    return this.log(EventType.TaskFailed, { task_id: taskId, attempt });
  }

  /**
   * Logs a plan_completed event with summary statistics.
   */
  async planCompleted(
    totalTasks: number,
    succeededTasks: number,
    durationMs: number
  ): Promise<void> {
    return this.log(EventType.PlanCompleted, {
      total_tasks: totalTasks,
      succeeded_tasks: succeededTasks,
      duration_ms: durationMs,
    });
  }

  /**
   * Logs a plan_cancelled event.
   */
  async planCancelled(lastTaskId: string): Promise<void> {
    return this.log(EventType.PlanCancelled, { last_task_id: lastTaskId });
  }

  /**
   * Logs a plan_failed event.
   */
  async planFailed(taskId: string, attempts: number): Promise<void> {
    return this.log(EventType.PlanFailed, { task_id: taskId, attempts });
  }

  /**
   * Gets the path to the progress file.
   */
  getFilePath(): string {
    return this.filePath;
  }
}
