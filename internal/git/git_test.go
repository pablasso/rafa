package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository and returns its path.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir
}

func TestIsClean(t *testing.T) {
	t.Parallel()
	t.Run("empty repo is clean", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !clean {
			t.Error("expected empty repo to be clean")
		}
	})

	t.Run("untracked file makes repo dirty", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create untracked file
		if err := os.WriteFile(filepath.Join(dir, "newfile.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clean {
			t.Error("expected repo with untracked file to be dirty")
		}
	})

	t.Run("staged file makes repo dirty", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create and stage file
		if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		cmd := exec.Command("git", "add", "staged.txt")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to stage file: %v", err)
		}

		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clean {
			t.Error("expected repo with staged file to be dirty")
		}
	})

	t.Run("modified tracked file makes repo dirty", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create, stage, and commit file
		if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		cmd := exec.Command("git", "add", "tracked.txt")
		cmd.Dir = dir
		cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "initial")
		cmd.Dir = dir
		cmd.Run()

		// Modify file
		if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644); err != nil {
			t.Fatalf("failed to modify file: %v", err)
		}

		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clean {
			t.Error("expected repo with modified file to be dirty")
		}
	})

	t.Run("committed changes leave repo clean", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create, stage, and commit file
		if err := os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		cmd := exec.Command("git", "add", "committed.txt")
		cmd.Dir = dir
		cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "add file")
		cmd.Dir = dir
		cmd.Run()

		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !clean {
			t.Error("expected repo with only committed changes to be clean")
		}
	})
}

func TestGetDirtyFiles(t *testing.T) {
	t.Parallel()
	t.Run("empty repo has no dirty files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		files, err := GetDirtyFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected no dirty files, got %v", files)
		}
	})

	t.Run("returns untracked files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		files, err := GetDirtyFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0] != "untracked.txt" {
			t.Errorf("expected [untracked.txt], got %v", files)
		}
	})

	t.Run("returns multiple dirty files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create multiple files
		os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content"), 0644)
		os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content"), 0644)
		os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
		os.WriteFile(filepath.Join(dir, "subdir", "file3.txt"), []byte("content"), 0644)

		files, err := GetDirtyFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 3 {
			t.Errorf("expected 3 dirty files, got %d: %v", len(files), files)
		}
	})

	t.Run("returns staged files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		cmd := exec.Command("git", "add", "staged.txt")
		cmd.Dir = dir
		cmd.Run()

		files, err := GetDirtyFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0] != "staged.txt" {
			t.Errorf("expected [staged.txt], got %v", files)
		}
	})

	t.Run("returns modified tracked files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create, stage, and commit file
		os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644)
		cmd := exec.Command("git", "add", "tracked.txt")
		cmd.Dir = dir
		cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "initial")
		cmd.Dir = dir
		cmd.Run()

		// Modify file
		os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644)

		files, err := GetDirtyFiles(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(files) != 1 || files[0] != "tracked.txt" {
			t.Errorf("expected [tracked.txt], got %v", files)
		}
	})
}

func TestGetStatus(t *testing.T) {
	t.Parallel()
	t.Run("returns combined status", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create untracked file
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)

		status, err := GetStatus(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Clean {
			t.Error("expected status.Clean to be false")
		}
		if len(status.Files) != 1 {
			t.Errorf("expected 1 file, got %d", len(status.Files))
		}
	})

	t.Run("clean repo returns empty files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		status, err := GetStatus(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !status.Clean {
			t.Error("expected status.Clean to be true")
		}
		if len(status.Files) != 0 {
			t.Errorf("expected 0 files, got %d", len(status.Files))
		}
	})
}

func TestCommitAll(t *testing.T) {
	t.Parallel()

	t.Run("commits changes successfully", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create some files
		os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
		os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)

		err := CommitAll(dir, "test commit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify repo is clean
		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error checking clean: %v", err)
		}
		if !clean {
			t.Error("expected repo to be clean after commit")
		}

		// Verify commit was created
		cmd := exec.Command("git", "log", "--oneline", "-1")
		cmd.Dir = dir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to get git log: %v", err)
		}
		if !strings.Contains(string(output), "test commit") {
			t.Errorf("expected commit message 'test commit', got: %s", output)
		}
	})

	t.Run("returns nil when nothing to commit", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create initial commit so repo has a HEAD
		os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial"), 0644)
		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = dir
		cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "initial")
		cmd.Dir = dir
		cmd.Run()

		// Now try to commit with no changes
		err := CommitAll(dir, "should not appear")
		if err != nil {
			t.Fatalf("expected nil error when nothing to commit, got: %v", err)
		}

		// Verify no new commit was created
		cmd = exec.Command("git", "log", "--oneline", "-1")
		cmd.Dir = dir
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("failed to get git log: %v", err)
		}
		if strings.Contains(string(output), "should not appear") {
			t.Error("commit should not have been created when there were no changes")
		}
	})

	t.Run("returns error for invalid directory", func(t *testing.T) {
		t.Parallel()
		err := CommitAll("/nonexistent/invalid/directory", "test")
		if err == nil {
			t.Error("expected error for invalid directory, got nil")
		}
	})

	t.Run("commits staged and unstaged changes", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create and stage a file
		os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0644)
		cmd := exec.Command("git", "add", "staged.txt")
		cmd.Dir = dir
		cmd.Run()

		// Create an unstaged file
		os.WriteFile(filepath.Join(dir, "unstaged.txt"), []byte("unstaged"), 0644)

		err := CommitAll(dir, "both files")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify repo is clean
		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error checking clean: %v", err)
		}
		if !clean {
			files, _ := GetDirtyFiles(dir)
			t.Errorf("expected repo to be clean, dirty files: %v", files)
		}
	})

	t.Run("commits modified tracked files", func(t *testing.T) {
		t.Parallel()
		dir := setupTestRepo(t)

		// Create and commit a file
		os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644)
		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = dir
		cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "initial")
		cmd.Dir = dir
		cmd.Run()

		// Modify the file
		os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644)

		err := CommitAll(dir, "modify tracked")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify repo is clean
		clean, err := IsClean(dir)
		if err != nil {
			t.Fatalf("unexpected error checking clean: %v", err)
		}
		if !clean {
			t.Error("expected repo to be clean after committing modified file")
		}
	})
}
