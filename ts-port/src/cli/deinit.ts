/**
 * Deinit command - removes Rafa from the current repository
 *
 * Removes .rafa/ directory and uninstalls skills from .claude/skills/
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import * as readline from "node:readline/promises";
import { stdin, stdout } from "node:process";
import { uninstallSkills } from "../core/skills.js";
import { isInitialized, getRafaDir, getSkillsDir, removeFromGitignore } from "./init.js";

const GITIGNORE_LOCK_ENTRY = ".rafa/**/*.lock";
const GITIGNORE_SESSIONS_ENTRY = ".rafa/sessions/";

/**
 * Options for the deinit command
 */
export interface DeinitOptions {
  workDir?: string;
  force?: boolean; // Skip confirmation prompt
  confirm?: () => Promise<boolean>; // Custom confirmation function (for testing)
}

/**
 * Result of the deinit command
 */
export interface DeinitResult {
  success: boolean;
  message: string;
}

/**
 * Calculates directory statistics
 */
async function calculateDirStats(
  dir: string
): Promise<{ planCount: number; totalSize: number }> {
  let planCount = 0;
  let totalSize = 0;

  // Count plans
  const plansDir = path.join(dir, "plans");
  try {
    const entries = await fs.readdir(plansDir);
    planCount = entries.length;
  } catch {
    // Ignore if plans directory doesn't exist
  }

  // Calculate total size
  async function walkDir(currentDir: string): Promise<void> {
    try {
      const entries = await fs.readdir(currentDir, { withFileTypes: true });
      for (const entry of entries) {
        const fullPath = path.join(currentDir, entry.name);
        if (entry.isDirectory()) {
          await walkDir(fullPath);
        } else {
          try {
            const stat = await fs.stat(fullPath);
            totalSize += stat.size;
          } catch {
            // Ignore files we can't stat
          }
        }
      }
    } catch {
      // Ignore directories we can't read
    }
  }

  await walkDir(dir);

  return { planCount, totalSize };
}

/**
 * Formats a size in bytes to a human-readable string
 */
function formatSize(bytes: number): string {
  const KB = 1024;
  const MB = KB * 1024;

  if (bytes >= MB) {
    return `${(bytes / MB).toFixed(1)}MB`;
  } else if (bytes >= KB) {
    return `${(bytes / KB).toFixed(1)}KB`;
  } else {
    return `${bytes}B`;
  }
}

/**
 * Prompts the user for confirmation
 */
async function promptConfirmation(message: string): Promise<boolean> {
  const rl = readline.createInterface({ input: stdin, output: stdout });
  try {
    const answer = await rl.question(message);
    const response = answer.trim().toLowerCase();
    return response === "y" || response === "yes";
  } finally {
    rl.close();
  }
}

/**
 * Runs the deinit command
 */
export async function runDeinit(options: DeinitOptions = {}): Promise<DeinitResult> {
  const workDir = options.workDir ?? process.cwd();
  const rafaDir = getRafaDir(workDir);
  const skillsDir = getSkillsDir(workDir);

  // Check if initialized
  if (!(await isInitialized(workDir))) {
    return {
      success: false,
      message: "Rafa is not initialized in this repository",
    };
  }

  // Verify .rafa is a directory
  try {
    const stat = await fs.stat(rafaDir);
    if (!stat.isDirectory()) {
      return {
        success: false,
        message: ".rafa exists but is not a directory",
      };
    }
  } catch (err) {
    return {
      success: false,
      message: `Failed to check .rafa directory: ${err instanceof Error ? err.message : err}`,
    };
  }

  // Calculate what will be deleted
  const { planCount, totalSize } = await calculateDirStats(rafaDir);

  // Show confirmation unless --force
  if (!options.force) {
    const confirmFn = options.confirm ?? promptConfirmation;
    const confirmed = await confirmFn(
      `This will delete .rafa/ (${planCount} plans, ${formatSize(totalSize)}) and remove skills from .claude/skills/. Continue? [y/N] `
    );

    if (!confirmed) {
      return {
        success: true,
        message: "Aborted.",
      };
    }
  }

  // Remove skills
  try {
    await uninstallSkills(skillsDir);
  } catch (err) {
    console.warn(
      `Warning: failed to remove skills: ${err instanceof Error ? err.message : err}`
    );
  }

  // Remove the directory
  try {
    await fs.rm(rafaDir, { recursive: true, force: true });
  } catch (err) {
    return {
      success: false,
      message: `Failed to remove .rafa/: ${err instanceof Error ? err.message : err}`,
    };
  }

  // Remove gitignore entries
  try {
    await removeFromGitignore(GITIGNORE_LOCK_ENTRY, workDir);
    await removeFromGitignore(GITIGNORE_SESSIONS_ENTRY, workDir);
  } catch (err) {
    return {
      success: false,
      message: `Failed to update .gitignore: ${err instanceof Error ? err.message : err}`,
    };
  }

  return {
    success: true,
    message:
      "Rafa has been removed from this repository (skills uninstalled from .claude/skills/).",
  };
}
