/**
 * Plan types and operations
 */

import type { Task } from "./task.js";

export type PlanStatus = "not_started" | "in_progress" | "completed" | "failed";

export interface Plan {
  id: string;
  name: string;
  description: string;
  sourceFile: string;
  createdAt: string;
  status: PlanStatus;
  tasks: Task[];
}

export function createPlan(
  id: string,
  name: string,
  description: string,
  sourceFile: string,
  tasks: Task[]
): Plan {
  return {
    id,
    name,
    description,
    sourceFile,
    createdAt: new Date().toISOString(),
    status: "not_started",
    tasks,
  };
}
