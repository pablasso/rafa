/**
 * Git operations
 * Reference: internal/git/git.go
 */

import { execFile, exec } from "child_process";
import { promisify } from "util";

const execFileAsync = promisify(execFile);
const execAsync = promisify(exec);

export interface GitStatus {
  clean: boolean;
  files: string[];
}

/**
 * Get the git workspace status for the given directory.
 * If dir is empty or undefined, uses the current working directory.
 */
export async function getStatus(dir?: string): Promise<GitStatus> {
  const options = dir ? { cwd: dir } : {};

  try {
    const { stdout } = await execFileAsync("git", ["status", "--porcelain"], options);

    const files: string[] = [];
    const lines = stdout.split("\n");

    for (const line of lines) {
      if (line.trim() === "") {
        continue;
      }
      // git status --porcelain format: XY filename
      // XY is the status (2 chars), followed by a space and filename
      // e.g., "?? file.txt", " M file.txt", "A  file.txt"
      if (line.length > 3) {
        files.push(line.slice(3));
      } else {
        // Unexpected format, include the whole line as the filename
        // to avoid silently dropping entries
        files.push(line.trim());
      }
    }

    return {
      clean: files.length === 0,
      files,
    };
  } catch (error) {
    throw new Error(`git status failed: ${error}`);
  }
}

/**
 * Returns true if the git workspace has no uncommitted changes.
 * It checks both staged and unstaged changes, as well as untracked files.
 */
export async function isClean(dir?: string): Promise<boolean> {
  const status = await getStatus(dir);
  return status.clean;
}

/**
 * Returns a list of files with uncommitted changes.
 * This includes modified, staged, and untracked files.
 */
export async function getDirtyFiles(dir?: string): Promise<string[]> {
  const status = await getStatus(dir);
  return status.files;
}

/**
 * Stages specified files with 'git add'.
 */
export async function add(files: string[], dir?: string): Promise<void> {
  if (files.length === 0) {
    return;
  }

  const options = dir ? { cwd: dir } : {};

  try {
    await execFileAsync("git", ["add", ...files], options);
  } catch (error) {
    throw new Error(`git add failed: ${error}`);
  }
}

/**
 * Stages all changes with 'git add -A'.
 */
export async function addAll(dir?: string): Promise<void> {
  const options = dir ? { cwd: dir } : {};

  try {
    await execFileAsync("git", ["add", "-A"], options);
  } catch (error) {
    throw new Error(`git add -A failed: ${error}`);
  }
}

/**
 * Creates a commit with the given message.
 * Assumes files are already staged.
 */
export async function commit(message: string, dir?: string): Promise<void> {
  const options = dir ? { cwd: dir } : {};

  try {
    await execFileAsync("git", ["commit", "-m", message], options);
  } catch (error) {
    throw new Error(`git commit failed: ${error}`);
  }
}

/**
 * Stages all changes with 'git add -A' and commits them with the given message.
 * Returns without error if there are no changes to commit.
 */
export async function commitAll(message: string, dir?: string): Promise<void> {
  const options = dir ? { cwd: dir } : {};

  // Stage all changes
  try {
    await execFileAsync("git", ["add", "-A"], options);
  } catch (error) {
    throw new Error(`git add -A failed: ${error}`);
  }

  // Check if there are staged changes
  try {
    await execAsync("git diff --cached --quiet", options);
    // Exit code 0 means no staged changes, so we're done
    return;
  } catch {
    // Non-zero exit code means there are staged changes, continue to commit
  }

  // Commit the staged changes
  try {
    await execFileAsync("git", ["commit", "-m", message], options);
  } catch (error) {
    throw new Error(`git commit failed: ${error}`);
  }
}
