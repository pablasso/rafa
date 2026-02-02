/**
 * Plan storage operations
 *
 * Implements load/save/list operations for plans, maintaining compatibility
 * with existing Go-generated plan files.
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import type { Dirent } from "node:fs";
import type { Plan } from "../core/plan.js";
import type { Task } from "../core/task.js";
import { migrateIfNeeded } from "./migration.js";

const RAFA_DIR = ".rafa";
const PLANS_DIR = "plans";

/**
 * Gets the path to the .rafa directory
 */
function getRafaDir(workDir: string = process.cwd()): string {
  return path.join(workDir, RAFA_DIR);
}

/**
 * Plan validation error
 */
export class PlanValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PlanValidationError";
  }
}

/**
 * Valid plan status values
 */
const VALID_PLAN_STATUSES = [
  "not_started",
  "in_progress",
  "completed",
  "failed",
] as const;

/**
 * Valid task status values
 */
const VALID_TASK_STATUSES = [
  "pending",
  "in_progress",
  "completed",
  "failed",
] as const;

/**
 * Validates a task object
 */
function validateTask(task: unknown, index: number): Task {
  if (typeof task !== "object" || task === null) {
    throw new PlanValidationError(`Task at index ${index} is not an object`);
  }

  const t = task as Record<string, unknown>;

  if (typeof t.id !== "string" || t.id.length === 0) {
    throw new PlanValidationError(
      `Task at index ${index} has invalid or missing 'id'`
    );
  }

  if (typeof t.title !== "string") {
    throw new PlanValidationError(
      `Task '${t.id}' has invalid or missing 'title'`
    );
  }

  if (typeof t.description !== "string") {
    throw new PlanValidationError(
      `Task '${t.id}' has invalid or missing 'description'`
    );
  }

  if (!Array.isArray(t.acceptanceCriteria)) {
    throw new PlanValidationError(
      `Task '${t.id}' has invalid or missing 'acceptanceCriteria'`
    );
  }

  for (let i = 0; i < t.acceptanceCriteria.length; i++) {
    if (typeof t.acceptanceCriteria[i] !== "string") {
      throw new PlanValidationError(
        `Task '${t.id}' has invalid acceptanceCriteria at index ${i}`
      );
    }
  }

  if (
    typeof t.status !== "string" ||
    !VALID_TASK_STATUSES.includes(t.status as (typeof VALID_TASK_STATUSES)[number])
  ) {
    throw new PlanValidationError(
      `Task '${t.id}' has invalid 'status': ${t.status}`
    );
  }

  if (typeof t.attempts !== "number" || t.attempts < 0) {
    throw new PlanValidationError(
      `Task '${t.id}' has invalid 'attempts': ${t.attempts}`
    );
  }

  return {
    id: t.id,
    title: t.title,
    description: t.description,
    acceptanceCriteria: t.acceptanceCriteria as string[],
    status: t.status as Task["status"],
    attempts: t.attempts,
  };
}

/**
 * Validates a plan object parsed from JSON
 */
function validatePlan(data: unknown): Plan {
  if (typeof data !== "object" || data === null) {
    throw new PlanValidationError("Plan is not an object");
  }

  const p = data as Record<string, unknown>;

  if (typeof p.id !== "string" || p.id.length === 0) {
    throw new PlanValidationError("Plan has invalid or missing 'id'");
  }

  if (typeof p.name !== "string" || p.name.length === 0) {
    throw new PlanValidationError("Plan has invalid or missing 'name'");
  }

  if (typeof p.description !== "string") {
    throw new PlanValidationError("Plan has invalid or missing 'description'");
  }

  if (typeof p.sourceFile !== "string") {
    throw new PlanValidationError("Plan has invalid or missing 'sourceFile'");
  }

  if (typeof p.createdAt !== "string") {
    throw new PlanValidationError("Plan has invalid or missing 'createdAt'");
  }

  if (
    typeof p.status !== "string" ||
    !VALID_PLAN_STATUSES.includes(p.status as (typeof VALID_PLAN_STATUSES)[number])
  ) {
    throw new PlanValidationError(`Plan has invalid 'status': ${p.status}`);
  }

  if (!Array.isArray(p.tasks)) {
    throw new PlanValidationError("Plan has invalid or missing 'tasks' array");
  }

  const tasks: Task[] = [];
  for (let i = 0; i < p.tasks.length; i++) {
    tasks.push(validateTask(p.tasks[i], i));
  }

  return {
    id: p.id,
    name: p.name,
    description: p.description,
    sourceFile: p.sourceFile,
    createdAt: p.createdAt,
    status: p.status as Plan["status"],
    tasks,
  };
}

/**
 * Gets the path to the plans directory
 */
export function getPlansDir(workDir: string = process.cwd()): string {
  return path.join(workDir, RAFA_DIR, PLANS_DIR);
}

/**
 * Gets the path to a plan folder given the full folder name (e.g., "abc123-my-plan")
 */
export function getPlanDir(
  folderName: string,
  workDir: string = process.cwd()
): string {
  return path.join(getPlansDir(workDir), folderName);
}

/**
 * Gets the path to plan.json in a plan directory
 */
export function getPlanJsonPath(planDir: string): string {
  return path.join(planDir, "plan.json");
}

/**
 * Finds a plan folder by name suffix in .rafa/plans/
 * Returns the full path to the plan folder.
 */
export async function findPlanFolder(
  name: string,
  workDir: string = process.cwd()
): Promise<string> {
  // Migrate from Go version if needed
  await migrateIfNeeded(getRafaDir(workDir));

  const plansPath = getPlansDir(workDir);

  let entries: Dirent[];
  try {
    entries = await fs.readdir(plansPath, { withFileTypes: true });
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      throw new Error("No plans found. Run 'rafa plan create <design.md>' first");
    }
    throw new Error(`Failed to read plans directory: ${err}`);
  }

  const suffix = `-${name}`;
  const matches: string[] = [];

  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    if (entry.name.endsWith(suffix)) {
      matches.push(entry.name);
    }
  }

  if (matches.length === 0) {
    throw new Error(`Plan not found: ${name}`);
  }

  if (matches.length > 1) {
    throw new Error(`Multiple plans match '${name}': ${matches.join(", ")}`);
  }

  return path.join(plansPath, matches[0]);
}

/**
 * Checks for name collisions in the plans directory and returns a unique name.
 * If the baseName is not taken, it returns as-is. If taken, it appends -2, -3, etc.
 */
export async function resolvePlanName(
  baseName: string,
  workDir: string = process.cwd()
): Promise<string> {
  const plansPath = getPlansDir(workDir);

  let entries: Dirent[];
  try {
    entries = await fs.readdir(plansPath, { withFileTypes: true });
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      // Directory doesn't exist yet, so no collisions possible
      return baseName;
    }
    throw new Error(`Failed to read plans directory: ${err}`);
  }

  // Build a set of existing names (extracted from folder names)
  const existingNames = new Set<string>();
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    // Folder format is <id>-<name>, so we split on first hyphen
    const idx = entry.name.indexOf("-");
    if (idx !== -1) {
      existingNames.add(entry.name.slice(idx + 1));
    }
  }

  // If baseName is not taken, return it
  if (!existingNames.has(baseName)) {
    return baseName;
  }

  // Find a unique suffix
  for (let suffixNum = 2; ; suffixNum++) {
    const candidate = `${baseName}-${suffixNum}`;
    if (!existingNames.has(candidate)) {
      return candidate;
    }
  }
}

/**
 * Loads a plan from a plan directory.
 * Returns the plan or throws an error if the file doesn't exist or is invalid.
 */
export async function loadPlan(planDir: string): Promise<Plan> {
  const planPath = getPlanJsonPath(planDir);

  let data: string;
  try {
    data = await fs.readFile(planPath, "utf-8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      throw new Error(`Plan not found at ${planPath}`);
    }
    throw new Error(`Failed to read plan.json: ${err}`);
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(data);
  } catch (err) {
    throw new PlanValidationError(`Invalid JSON in plan.json: ${err}`);
  }

  return validatePlan(parsed);
}

/**
 * Saves a plan to a plan directory using atomic write (temp file + rename).
 */
export async function savePlan(planDir: string, plan: Plan): Promise<void> {
  const planPath = getPlanJsonPath(planDir);
  const tmpPath = `${planPath}.tmp.${process.pid}`;

  // Marshal with 2-space indent to match Go version
  const data = JSON.stringify(plan, null, 2);

  try {
    // Write to temp file
    await fs.writeFile(tmpPath, data, "utf-8");

    // Rename temp file to plan.json (atomic operation)
    await fs.rename(tmpPath, planPath);
  } catch (err) {
    // Clean up temp file on failure
    try {
      await fs.unlink(tmpPath);
    } catch {
      // Ignore cleanup errors
    }
    throw new Error(`Failed to save plan: ${err}`);
  }
}

/**
 * Creates a new plan folder with plan.json and log files.
 * Returns the path to the created folder.
 */
export async function createPlanFolder(
  plan: Plan,
  workDir: string = process.cwd()
): Promise<string> {
  const folderName = `${plan.id}-${plan.name}`;
  const folderPath = path.join(getPlansDir(workDir), folderName);

  // Create directory structure
  await fs.mkdir(folderPath, { recursive: true });

  // Write plan.json
  await savePlan(folderPath, plan);

  // Create empty log files
  await fs.writeFile(path.join(folderPath, "progress.jsonl"), "", "utf-8");
  await fs.writeFile(path.join(folderPath, "output.log"), "", "utf-8");

  return folderPath;
}

/**
 * Lists all plans from .rafa/plans/
 */
export async function listPlans(workDir: string = process.cwd()): Promise<Plan[]> {
  // Migrate from Go version if needed
  await migrateIfNeeded(getRafaDir(workDir));

  const plansPath = getPlansDir(workDir);

  let entries: Dirent[];
  try {
    entries = await fs.readdir(plansPath, { withFileTypes: true });
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      // No plans directory yet
      return [];
    }
    throw new Error(`Failed to read plans directory: ${err}`);
  }

  const plans: Plan[] = [];
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;

    const planDir = path.join(plansPath, entry.name);
    try {
      const plan = await loadPlan(planDir);
      plans.push(plan);
    } catch {
      // Skip invalid or corrupted plan directories
      continue;
    }
  }

  return plans;
}

/**
 * Finds the first pending task in a plan.
 * If a failed task is found, its status is reset to pending (attempts are preserved).
 * Returns the task index, or -1 if all tasks are completed.
 */
export function firstPendingTask(plan: Plan): number {
  for (let i = 0; i < plan.tasks.length; i++) {
    const task = plan.tasks[i];
    switch (task.status) {
      case "pending":
      case "in_progress":
        return i;
      case "failed":
        // Reset failed task to pending, preserving attempts
        task.status = "pending";
        return i;
    }
  }
  return -1;
}

/**
 * Returns true if all tasks in the plan have status completed.
 */
export function allTasksCompleted(plan: Plan): boolean {
  return plan.tasks.every((task) => task.status === "completed");
}
