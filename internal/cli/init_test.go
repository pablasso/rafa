package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunInit(t *testing.T) {
	t.Run("successful init creates directories and updates gitignore", func(t *testing.T) {
		// Create a temp dir with git initialized
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// Run init command
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("runInit failed: %v", err)
		}

		// Verify .rafa directory was created
		rafaInfo, err := os.Stat(".rafa")
		if err != nil {
			t.Fatalf("expected .rafa directory to exist, got error: %v", err)
		}
		if !rafaInfo.IsDir() {
			t.Error("expected .rafa to be a directory")
		}

		// Verify .rafa/plans directory was created
		plansInfo, err := os.Stat(filepath.Join(".rafa", "plans"))
		if err != nil {
			t.Fatalf("expected .rafa/plans directory to exist, got error: %v", err)
		}
		if !plansInfo.IsDir() {
			t.Error("expected .rafa/plans to be a directory")
		}

		// Verify .gitignore contains the lock file pattern
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist, got error: %v", err)
		}
		if string(content) != ".rafa/**/*.lock\n" {
			t.Errorf("expected .gitignore to contain lock pattern, got %q", string(content))
		}
	})

	t.Run("init outside git repo fails", func(t *testing.T) {
		// Create a temp dir without git
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Run init command
		err := runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error when not in git repo, got nil")
		}

		// Verify it's a PrerequisiteError about git
		prereqErr, ok := err.(*PrerequisiteError)
		if !ok {
			t.Fatalf("expected *PrerequisiteError, got %T: %v", err, err)
		}

		if prereqErr.Check != "Git repository" {
			t.Errorf("expected Check to be 'Git repository', got %q", prereqErr.Check)
		}

		// Verify .rafa was not created
		if _, err := os.Stat(".rafa"); err == nil {
			t.Error("expected .rafa directory to not exist after failed init")
		}
	})

	t.Run("double init fails", func(t *testing.T) {
		// Create a temp dir with git initialized
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Initialize a real git repository
		cmd := exec.Command("git", "init")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		// First init should succeed
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("first runInit failed: %v", err)
		}

		// Second init should fail
		err = runInit(nil, nil)
		if err == nil {
			t.Fatal("expected error on second init, got nil")
		}

		expectedErr := "rafa is already initialized in this repository"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})
}

func TestAddToGitignore(t *testing.T) {
	t.Run("creates gitignore if not exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		err := addToGitignore("test-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to exist: %v", err)
		}
		if string(content) != "test-entry\n" {
			t.Errorf("expected 'test-entry\\n', got %q", string(content))
		}
	})

	t.Run("appends to existing gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("existing-entry\n"), 0644)

		err := addToGitignore("new-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		expected := "existing-entry\nnew-entry\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("adds newline if file doesn't end with one", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("no-trailing-newline"), 0644)

		err := addToGitignore("new-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		expected := "no-trailing-newline\nnew-entry\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("skips if entry already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("existing-entry\n"), 0644)

		err := addToGitignore("existing-entry")
		if err != nil {
			t.Fatalf("addToGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		if string(content) != "existing-entry\n" {
			t.Errorf("expected unchanged content, got %q", string(content))
		}
	})
}

func TestRemoveFromGitignore(t *testing.T) {
	t.Run("removes entry from gitignore", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("keep-me\nremove-me\nalso-keep\n"), 0644)

		err := removeFromGitignore("remove-me")
		if err != nil {
			t.Fatalf("removeFromGitignore failed: %v", err)
		}

		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("failed to read .gitignore: %v", err)
		}
		expected := "keep-me\nalso-keep\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("does nothing if gitignore doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		err := removeFromGitignore("any-entry")
		if err != nil {
			t.Fatalf("removeFromGitignore should not fail: %v", err)
		}
	})

	t.Run("leaves gitignore if only rafa entry would remain", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		os.WriteFile(".gitignore", []byte("only-entry\n"), 0644)

		err := removeFromGitignore("only-entry")
		if err != nil {
			t.Fatalf("removeFromGitignore failed: %v", err)
		}

		// File should still exist (we don't delete it)
		content, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf("expected .gitignore to still exist: %v", err)
		}
		// Content should be unchanged since removing would make it empty
		if string(content) != "only-entry\n" {
			t.Errorf("expected unchanged content when result would be empty, got %q", string(content))
		}
	})
}
