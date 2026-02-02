/**
 * Plan creation CLI command
 *
 * Creates an executable plan from a technical design or PRD markdown file
 * by extracting tasks using Claude CLI.
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import { createPlan } from "../core/plan.js";
import { createTask } from "../core/task.js";
import { extractTasks, TaskExtractionError } from "../core/extract-tasks.js";
import {
  createPlanFolder,
  resolvePlanName,
  getPlansDir,
} from "../storage/plans.js";
import { generateId } from "../utils/id.js";
import { toKebabCase, generateTaskId } from "../utils/string.js";

const RAFA_DIR = ".rafa";

export interface PlanCreateOptions {
  filePath: string;
  name?: string;
  dryRun?: boolean;
}

export interface PlanCreateResult {
  success: boolean;
  message: string;
  planId?: string;
  planName?: string;
  taskCount?: number;
}

/**
 * Finds the repo root by walking up directories looking for .rafa/
 */
async function findRepoRoot(startDir: string = process.cwd()): Promise<string> {
  let dir = startDir;
  while (true) {
    const rafaPath = path.join(dir, RAFA_DIR);
    try {
      const stat = await fs.stat(rafaPath);
      if (stat.isDirectory()) {
        return dir;
      }
    } catch {
      // Directory doesn't exist, continue walking up
    }

    const parent = path.dirname(dir);
    if (parent === dir) {
      // Reached filesystem root
      throw new Error(".rafa directory not found");
    }
    dir = parent;
  }
}

/**
 * Normalizes a source path to be relative from the repo root
 */
async function normalizeSourcePath(filePath: string): Promise<string> {
  try {
    const absPath = path.resolve(filePath);
    const repoRoot = await findRepoRoot();
    return path.relative(repoRoot, absPath);
  } catch {
    return filePath;
  }
}

/**
 * Validates the input options before creating a plan
 */
async function validateInputs(opts: PlanCreateOptions): Promise<void> {
  // Check .rafa/ exists (skip for dry-run)
  if (!opts.dryRun) {
    try {
      await findRepoRoot();
    } catch {
      throw new Error("rafa not initialized. Run `rafa init` first");
    }
  }

  // Verify file has .md extension
  if (!opts.filePath.toLowerCase().endsWith(".md")) {
    throw new Error(`file must be markdown (.md): ${opts.filePath}`);
  }

  // Verify file exists and is not empty
  let fileStat;
  try {
    fileStat = await fs.stat(opts.filePath);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      throw new Error(`file not found: ${opts.filePath}`);
    }
    throw err;
  }

  if (fileStat.size === 0) {
    throw new Error(`design document is empty: ${opts.filePath}`);
  }
}

/**
 * Determines the plan base name based on priority:
 * --name flag > AI-extracted name > filename without extension
 */
function determinePlanBaseName(
  opts: PlanCreateOptions,
  extractedName: string,
): string {
  if (opts.name) {
    return toKebabCase(opts.name);
  }

  if (extractedName) {
    return toKebabCase(extractedName);
  }

  // Use filename without extension
  const base = path.basename(opts.filePath);
  const name = base.replace(/\.[^/.]+$/, "");
  return toKebabCase(name);
}

/**
 * Prints a preview of the plan (for dry-run mode)
 */
function printDryRunPreview(
  planName: string,
  sourcePath: string,
  tasks: Array<{ id: string; title: string; acceptanceCriteria: string[] }>,
): void {
  console.log();
  console.log("Plan preview (dry run - nothing saved):");
  console.log();
  console.log(`  Name: ${planName}`);
  console.log(`  Source: ${sourcePath}`);
  console.log(`  Tasks: ${tasks.length}`);
  console.log();

  for (const task of tasks) {
    console.log(`  ${task.id}: ${task.title}`);
    for (const ac of task.acceptanceCriteria) {
      console.log(`       - ${ac}`);
    }
  }

  console.log();
  console.log("To create this plan, run without --dry-run.");
}

/**
 * Creates a plan from a design document
 */
export async function runPlanCreate(
  opts: PlanCreateOptions,
): Promise<PlanCreateResult> {
  // Validate inputs
  try {
    await validateInputs(opts);
  } catch (err) {
    return {
      success: false,
      message: err instanceof Error ? err.message : String(err),
    };
  }

  // Read design file
  let content: string;
  try {
    content = await fs.readFile(opts.filePath, "utf-8");
  } catch (err) {
    return {
      success: false,
      message: `failed to read file: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  console.log(`Creating plan from: ${opts.filePath}`);
  console.log("Extracting tasks...");

  // Extract tasks via Claude CLI
  let extracted;
  try {
    extracted = await extractTasks(content);
  } catch (err) {
    if (err instanceof TaskExtractionError) {
      return {
        success: false,
        message: `failed to extract tasks: ${err.message}`,
      };
    }
    return {
      success: false,
      message: `failed to extract tasks: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  // Generate plan ID (6-char random)
  const id = generateId();

  // Resolve plan name with collision detection
  const baseName = determinePlanBaseName(opts, extracted.name);
  let name: string;
  try {
    name = await resolvePlanName(baseName);
  } catch (err) {
    return {
      success: false,
      message: `failed to resolve plan name: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  // Normalize source path to be relative from repo root
  const sourcePath = await normalizeSourcePath(opts.filePath);

  // Convert extracted tasks to Task objects with sequential IDs
  const tasks = extracted.tasks.map((et, i) =>
    createTask(generateTaskId(i), et.title, et.description, et.acceptanceCriteria),
  );

  // Handle dry-run
  if (opts.dryRun) {
    printDryRunPreview(name, sourcePath, tasks);
    return {
      success: true,
      message: "Dry run completed",
      planName: name,
      taskCount: tasks.length,
    };
  }

  // Create the plan
  const plan = createPlan(id, name, extracted.description, sourcePath, tasks);

  // Create plan folder with plan.json and log files
  try {
    await createPlanFolder(plan);
  } catch (err) {
    return {
      success: false,
      message: `failed to create plan folder: ${err instanceof Error ? err.message : String(err)}`,
    };
  }

  // Print success message
  console.log();
  console.log(`Plan created: ${id}-${name}`);
  console.log();
  console.log(`  ${tasks.length} tasks extracted:`);
  console.log();

  for (const task of tasks) {
    console.log(`  ${task.id}: ${task.title}`);
  }

  console.log();
  console.log(`Run \`rafa plan run ${name}\` to start execution.`);

  return {
    success: true,
    message: `Plan created: ${id}-${name}`,
    planId: id,
    planName: name,
    taskCount: tasks.length,
  };
}
