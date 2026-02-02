/**
 * Progress event logging
 */

export type ProgressEventType =
  | "task_started"
  | "task_completed"
  | "task_failed"
  | "task_retrying"
  | "plan_started"
  | "plan_completed"
  | "plan_failed";

export interface ProgressEvent {
  type: ProgressEventType;
  taskId?: string;
  attempt?: number;
  timestamp: string;
  message?: string;
}

export function createProgressEvent(
  type: ProgressEventType,
  taskId?: string,
  attempt?: number,
  message?: string
): ProgressEvent {
  return {
    type,
    taskId,
    attempt,
    timestamp: new Date().toISOString(),
    message,
  };
}
