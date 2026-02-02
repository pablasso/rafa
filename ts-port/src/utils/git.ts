/**
 * Git operations
 */

export interface GitStatus {
  clean: boolean;
  branch: string;
  staged: string[];
  unstaged: string[];
}

export async function getGitStatus(): Promise<GitStatus> {
  // Implementation in Task 7
  return {
    clean: true,
    branch: "main",
    staged: [],
    unstaged: [],
  };
}

export async function gitAdd(_files: string[]): Promise<void> {
  // Implementation in Task 7
}

export async function gitCommit(_message: string): Promise<void> {
  // Implementation in Task 7
}
