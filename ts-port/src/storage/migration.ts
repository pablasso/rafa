/**
 * Migration from Go version to TypeScript version
 *
 * Handles automatic migration when TypeScript Rafa encounters
 * data created by the Go version. Reference: docs/designs/rafa-pi-mono-port.md
 *
 * Migration runs automatically on first access to .rafa/ when no version
 * file exists (indicating Go-version data).
 */

import * as fs from "node:fs/promises";
import * as path from "node:path";
import { existsSync } from "node:fs";

const VERSION_FILE = "version";
const CURRENT_VERSION = 2;
const RUNTIME = "typescript";

export interface VersionInfo {
  version: number;
  runtime: string;
}

/**
 * Checks if migration is needed by looking for the version file.
 * Returns true if:
 * - .rafa/ directory exists
 * - .rafa/version file does NOT exist (Go version)
 */
export async function needsMigration(rafaDir: string): Promise<boolean> {
  // Check if .rafa/ directory exists
  if (!existsSync(rafaDir)) {
    return false;
  }

  // Check if version file exists
  const versionPath = path.join(rafaDir, VERSION_FILE);
  return !existsSync(versionPath);
}

/**
 * Reads the current version info from .rafa/version
 * Returns null if file doesn't exist
 */
export async function getVersionInfo(
  rafaDir: string,
): Promise<VersionInfo | null> {
  const versionPath = path.join(rafaDir, VERSION_FILE);
  try {
    const content = await fs.readFile(versionPath, "utf-8");
    return JSON.parse(content) as VersionInfo;
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      return null;
    }
    throw err;
  }
}

/**
 * Writes the version info to .rafa/version
 */
export async function writeVersionFile(rafaDir: string): Promise<void> {
  const versionPath = path.join(rafaDir, VERSION_FILE);
  const versionInfo: VersionInfo = {
    version: CURRENT_VERSION,
    runtime: RUNTIME,
  };
  await fs.writeFile(
    versionPath,
    JSON.stringify(versionInfo, null, 2) + "\n",
    "utf-8",
  );
}

/**
 * Renames progress.log files to progress.jsonl in all plan directories
 */
async function migrateProgressLogs(rafaDir: string): Promise<number> {
  const plansDir = path.join(rafaDir, "plans");

  // Check if plans directory exists
  if (!existsSync(plansDir)) {
    return 0;
  }

  // List all plan directories
  const entries = await fs.readdir(plansDir, { withFileTypes: true });

  let migrated = 0;
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;

    const progressLogPath = path.join(plansDir, entry.name, "progress.log");

    // Check if progress.log exists in this plan directory
    if (!existsSync(progressLogPath)) continue;

    const newPath = path.join(plansDir, entry.name, "progress.jsonl");
    try {
      await fs.rename(progressLogPath, newPath);
      migrated++;
    } catch (err) {
      // Log warning but continue with other files
      console.warn(`Warning: Failed to rename ${progressLogPath}: ${err}`);
    }
  }

  return migrated;
}

/**
 * Clears the old sessions directory.
 * Sessions are gitignored and ephemeral, so we just remove and recreate.
 */
async function clearSessions(rafaDir: string): Promise<void> {
  const sessionsDir = path.join(rafaDir, "sessions");

  if (!existsSync(sessionsDir)) {
    return;
  }

  // Remove the entire sessions directory and recreate it empty
  await fs.rm(sessionsDir, { recursive: true });
  await fs.mkdir(sessionsDir, { recursive: true });
}

/**
 * Result of migration operation
 */
export interface MigrationResult {
  migrated: boolean;
  progressLogsMigrated: number;
  sessionCleared: boolean;
}

/**
 * Runs the migration from Go version to TypeScript version.
 *
 * This function:
 * 1. Renames progress.log → progress.jsonl in all plan directories
 * 2. Clears old sessions directory (they're gitignored anyway)
 * 3. Writes .rafa/version with {"version": 2, "runtime": "typescript"}
 */
export async function runMigration(rafaDir: string): Promise<MigrationResult> {
  // Migrate progress logs
  const progressLogsMigrated = await migrateProgressLogs(rafaDir);

  // Clear sessions
  const sessionsDir = path.join(rafaDir, "sessions");
  const sessionCleared = existsSync(sessionsDir);
  await clearSessions(rafaDir);

  // Write version file
  await writeVersionFile(rafaDir);

  return {
    migrated: true,
    progressLogsMigrated,
    sessionCleared,
  };
}

/**
 * Ensures the .rafa/ directory is migrated if needed.
 * This should be called on first access to .rafa/.
 *
 * Returns true if migration was performed, false otherwise.
 */
export async function migrateIfNeeded(rafaDir: string): Promise<boolean> {
  if (!(await needsMigration(rafaDir))) {
    return false;
  }

  await runMigration(rafaDir);
  return true;
}
