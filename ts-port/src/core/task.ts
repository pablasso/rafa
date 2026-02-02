/**
 * Task types
 */

export type TaskStatus = "pending" | "in_progress" | "completed" | "failed";

export interface Task {
  id: string;
  title: string;
  description: string;
  acceptanceCriteria: string[];
  status: TaskStatus;
  attempts: number;
}

export function createTask(
  id: string,
  title: string,
  description: string,
  acceptanceCriteria: string[]
): Task {
  return {
    id,
    title,
    description,
    acceptanceCriteria,
    status: "pending",
    attempts: 0,
  };
}
