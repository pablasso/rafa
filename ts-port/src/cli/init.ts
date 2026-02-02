/**
 * Init command - initializes Rafa in the current repository
 *
 * Creates .rafa/ directory structure, installs skills from GitHub,
 * and updates .gitignore.
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import { installSkills, uninstallSkills, SkillsInstallError } from "../core/skills.js";
import { checkClaudeCli } from "../utils/claude-check.js";

const RAFA_DIR = ".rafa";
const GITIGNORE_PATH = ".gitignore";
const GITIGNORE_LOCK_ENTRY = ".rafa/**/*.lock";
const GITIGNORE_SESSIONS_ENTRY = ".rafa/sessions/";
const CLAUDE_SKILLS_DIR = ".claude/skills";

/**
 * Checks if Rafa is already initialized in the current directory
 */
export async function isInitialized(workDir: string = process.cwd()): Promise<boolean> {
  const rafaPath = path.join(workDir, RAFA_DIR);
  try {
    const stat = await fs.stat(rafaPath);
    return stat.isDirectory();
  } catch {
    return false;
  }
}

/**
 * Gets the path to the .rafa directory
 */
export function getRafaDir(workDir: string = process.cwd()): string {
  return path.join(workDir, RAFA_DIR);
}

/**
 * Gets the path to the settings file
 */
export function getSettingsPath(workDir: string = process.cwd()): string {
  return path.join(workDir, RAFA_DIR, "settings.json");
}

/**
 * Gets the path to the skills directory
 */
export function getSkillsDir(workDir: string = process.cwd()): string {
  return path.join(workDir, CLAUDE_SKILLS_DIR);
}

/**
 * Adds an entry to .gitignore if not already present
 */
async function addToGitignore(
  entry: string,
  workDir: string = process.cwd()
): Promise<void> {
  const gitignorePath = path.join(workDir, GITIGNORE_PATH);

  // Read existing content
  let content = "";
  try {
    content = await fs.readFile(gitignorePath, "utf-8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "ENOENT") {
      throw err;
    }
  }

  // Check if entry already exists
  if (content) {
    const lines = content.split("\n");
    for (const line of lines) {
      if (line.trim() === entry) {
        return; // Already present
      }
    }
  }

  // Build new content
  let newContent = content;
  if (newContent && !newContent.endsWith("\n")) {
    newContent += "\n";
  }
  newContent += entry + "\n";

  await fs.writeFile(gitignorePath, newContent, "utf-8");
}

/**
 * Removes an entry from .gitignore
 */
async function removeFromGitignore(
  entry: string,
  workDir: string = process.cwd()
): Promise<void> {
  const gitignorePath = path.join(workDir, GITIGNORE_PATH);

  let content: string;
  try {
    content = await fs.readFile(gitignorePath, "utf-8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return; // Nothing to remove
    }
    throw err;
  }

  const lines = content.split("\n");
  const newLines = lines.filter((line) => line.trim() !== entry);

  // Don't modify .gitignore if removing the entry would leave it empty
  const newContent = newLines.join("\n");
  if (newContent.trim() === "") {
    return;
  }

  await fs.writeFile(gitignorePath, newContent, "utf-8");
}

/**
 * Options for the init command
 */
export interface InitOptions {
  workDir?: string;
  skipSkills?: boolean; // For testing
}

/**
 * Result of the init command
 */
export interface InitResult {
  success: boolean;
  message: string;
}

/**
 * Runs the init command
 */
export async function runInit(options: InitOptions = {}): Promise<InitResult> {
  const workDir = options.workDir ?? process.cwd();
  const rafaDir = getRafaDir(workDir);
  const skillsDir = getSkillsDir(workDir);

  // Check Claude CLI prerequisite
  const cliCheck = await checkClaudeCli();
  if (!cliCheck.available) {
    return {
      success: false,
      message: cliCheck.message,
    };
  }

  // Check if already initialized
  if (await isInitialized(workDir)) {
    return {
      success: false,
      message: "Rafa is already initialized in this repository",
    };
  }

  // Track what's been created for cleanup on failure
  let skillsInstalled = false;
  let success = false;

  const cleanup = async () => {
    if (!success) {
      // Clean up all partial state on failure
      try {
        await fs.rm(rafaDir, { recursive: true, force: true });
      } catch {
        // Ignore cleanup errors
      }
      if (skillsInstalled) {
        try {
          await uninstallSkills(skillsDir);
        } catch {
          // Ignore cleanup errors
        }
      }
      try {
        await removeFromGitignore(GITIGNORE_LOCK_ENTRY, workDir);
        await removeFromGitignore(GITIGNORE_SESSIONS_ENTRY, workDir);
      } catch {
        // Ignore cleanup errors
      }
    }
  };

  try {
    // Create .rafa directory structure
    const dirs = [
      rafaDir,
      path.join(rafaDir, "plans"),
      path.join(rafaDir, "sessions"),
    ];

    for (const dir of dirs) {
      await fs.mkdir(dir, { recursive: true });
    }

    // Create empty settings.json
    const settingsPath = getSettingsPath(workDir);
    const defaultSettings = {
      defaultMaxAttempts: 5,
    };
    await fs.writeFile(
      settingsPath,
      JSON.stringify(defaultSettings, null, 2) + "\n",
      "utf-8"
    );

    // Install skills from GitHub (unless skipped for testing)
    if (!options.skipSkills) {
      console.log("Installing skills from github.com/pablasso/skills... ");
      try {
        await installSkills({ targetDir: skillsDir });
        skillsInstalled = true;
        console.log("done");
      } catch (err) {
        console.log("failed");
        if (err instanceof SkillsInstallError) {
          await cleanup();
          return {
            success: false,
            message: `Failed to install skills: ${err.message}. Please check your internet connection and try again.`,
          };
        }
        throw err;
      }
    }

    // Add gitignore entries
    await addToGitignore(GITIGNORE_LOCK_ENTRY, workDir);
    await addToGitignore(GITIGNORE_SESSIONS_ENTRY, workDir);

    // Mark success to prevent cleanup
    success = true;

    return {
      success: true,
      message: `Initialized Rafa in ${rafaDir}

Next steps:
  1. Run: rafa prd             # Create a PRD
  2. Run: rafa design          # Create a technical design
  3. Run: rafa plan create     # Create an execution plan`,
    };
  } catch (err) {
    await cleanup();
    throw err;
  }
}

// Export for testing
export { addToGitignore, removeFromGitignore };
