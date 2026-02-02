/**
 * File locking for concurrent access prevention
 */

export interface Lock {
  pid: number;
  createdAt: string;
}

export async function acquireLock(_lockPath: string): Promise<boolean> {
  // Implementation in Task 8
  return true;
}

export async function releaseLock(_lockPath: string): Promise<void> {
  // Implementation in Task 8
}

export async function isLockStale(_lockPath: string): Promise<boolean> {
  // Implementation in Task 8
  return false;
}
