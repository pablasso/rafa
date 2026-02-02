/**
 * File locking for concurrent access prevention
 * Reference: internal/plan/lock.go
 */

import * as fs from "fs";
import * as path from "path";

const LOCK_FILE_NAME = "run.lock";

/**
 * Manages a lock file to prevent concurrent runs of the same plan.
 */
export class PlanLock {
  private path: string;

  constructor(planDir: string) {
    this.path = path.join(planDir, LOCK_FILE_NAME);
  }

  /**
   * Attempts to acquire the lock.
   * Throws an error if the lock is held by another running process.
   * Stale locks (from dead processes) are automatically cleaned up.
   */
  async acquire(): Promise<void> {
    const pid = process.pid;

    // Try atomic creation with O_EXCL
    try {
      const fd = fs.openSync(
        this.path,
        fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_WRONLY,
        0o644,
      );
      fs.writeSync(fd, String(pid));
      fs.closeSync(fd);
      return;
    } catch (err: unknown) {
      // If error is not "file exists", rethrow
      if ((err as NodeJS.ErrnoException).code !== "EEXIST") {
        throw new Error(`Failed to create lock file: ${err}`);
      }
    }

    // Lock file exists - check if it's stale
    let data: string;
    try {
      data = fs.readFileSync(this.path, "utf-8");
    } catch (err) {
      throw new Error(`Failed to read existing lock file: ${err}`);
    }

    const pidStr = data.trim();
    const existingPid = parseInt(pidStr, 10);

    if (isNaN(existingPid)) {
      // Invalid PID in lock file - treat as stale
      try {
        fs.unlinkSync(this.path);
      } catch (err) {
        throw new Error(`Failed to remove invalid lock file: ${err}`);
      }
      return this.retryAcquire();
    }

    // Check if process is still running
    if (processExists(existingPid)) {
      throw new Error(`Plan is already running (PID ${existingPid})`);
    }

    // Process is dead - remove stale lock and retry
    try {
      fs.unlinkSync(this.path);
    } catch (err) {
      throw new Error(`Failed to remove stale lock file: ${err}`);
    }

    return this.retryAcquire();
  }

  /**
   * Attempts to acquire the lock after removing a stale lock.
   * Only tries once to avoid infinite loops.
   */
  private retryAcquire(): void {
    const pid = process.pid;

    try {
      const fd = fs.openSync(
        this.path,
        fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_WRONLY,
        0o644,
      );
      fs.writeSync(fd, String(pid));
      fs.closeSync(fd);
    } catch (err: unknown) {
      if ((err as NodeJS.ErrnoException).code === "EEXIST") {
        throw new Error("Lock acquired by another process during retry");
      }
      throw new Error(`Failed to create lock file on retry: ${err}`);
    }
  }

  /**
   * Removes the lock file.
   * Returns without error if the lock file doesn't exist (idempotent).
   */
  release(): void {
    try {
      fs.unlinkSync(this.path);
    } catch (err: unknown) {
      if ((err as NodeJS.ErrnoException).code !== "ENOENT") {
        throw new Error(`Failed to remove lock file: ${err}`);
      }
    }
  }

  /**
   * Checks if the lock file exists and returns the PID if it does.
   * Returns null if no lock exists.
   */
  getLockInfo(): { pid: number } | null {
    try {
      const data = fs.readFileSync(this.path, "utf-8");
      const pid = parseInt(data.trim(), 10);
      if (isNaN(pid)) {
        return null;
      }
      return { pid };
    } catch {
      return null;
    }
  }

  /**
   * Checks if the current lock is stale (held by a dead process).
   */
  isStale(): boolean {
    const info = this.getLockInfo();
    if (!info) {
      return false; // No lock exists
    }
    return !processExists(info.pid);
  }
}

/**
 * Checks if a process with the given PID is running.
 * Uses kill with signal 0, which checks for process existence without sending a signal.
 */
export function processExists(pid: number): boolean {
  try {
    // Signal 0 doesn't send a signal, just checks if process exists
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

// Legacy function exports for backwards compatibility with existing interface

/**
 * @deprecated Use PlanLock class instead
 */
export async function acquireLock(lockPath: string): Promise<boolean> {
  const lock = new PlanLock(path.dirname(lockPath));
  try {
    await lock.acquire();
    return true;
  } catch {
    return false;
  }
}

/**
 * @deprecated Use PlanLock class instead
 */
export async function releaseLock(lockPath: string): Promise<void> {
  const lock = new PlanLock(path.dirname(lockPath));
  lock.release();
}

/**
 * @deprecated Use PlanLock class instead
 */
export async function isLockStale(lockPath: string): Promise<boolean> {
  const lock = new PlanLock(path.dirname(lockPath));
  return lock.isStale();
}
