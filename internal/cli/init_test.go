package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunInit(t *testing.T) {
	t.Run("successful init creates directories", func(t *testing.T) {
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
