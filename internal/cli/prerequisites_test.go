package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrerequisiteError(t *testing.T) {
	t.Run("formats error with check, message, and help", func(t *testing.T) {
		err := &PrerequisiteError{
			Check:   "Test Check",
			Message: "Something went wrong",
			Help:    "Try doing X to fix it.",
		}

		expected := "Test Check: Something went wrong\n\nTry doing X to fix it."
		if err.Error() != expected {
			t.Errorf("got %q, want %q", err.Error(), expected)
		}
	})
}

func TestCheckGitRepo(t *testing.T) {
	t.Run("in git repo returns nil", func(t *testing.T) {
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

		err := checkGitRepo()
		if err != nil {
			t.Errorf("expected nil error in git repo, got: %v", err)
		}
	})

	t.Run("not in git repo returns PrerequisiteError", func(t *testing.T) {
		// Create a temp dir without git
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		err := checkGitRepo()
		if err == nil {
			t.Error("expected error when not in git repo, got nil")
		}

		prereqErr, ok := err.(*PrerequisiteError)
		if !ok {
			t.Fatalf("expected *PrerequisiteError, got %T", err)
		}

		if prereqErr.Message != "Not a git repository" {
			t.Errorf("got message %q, want %q", prereqErr.Message, "Not a git repository")
		}

		expectedHelp := "Rafa requires a git repository. Run 'git init' first."
		if prereqErr.Help != expectedHelp {
			t.Errorf("got help %q, want %q", prereqErr.Help, expectedHelp)
		}
	})
}

func TestIsInitialized(t *testing.T) {
	t.Run("returns true when .rafa directory exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa directory
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		if !IsInitialized() {
			t.Error("expected IsInitialized() to return true when .rafa exists")
		}
	})

	t.Run("returns false when .rafa does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		if IsInitialized() {
			t.Error("expected IsInitialized() to return false when .rafa does not exist")
		}
	})

	t.Run("returns false when .rafa is a file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa as a file
		os.WriteFile(filepath.Join(tmpDir, ".rafa"), []byte("not a dir"), 0644)

		if IsInitialized() {
			t.Error("expected IsInitialized() to return false when .rafa is a file")
		}
	})
}

func TestRequireInitialized(t *testing.T) {
	t.Run("returns nil when initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa directory
		os.MkdirAll(filepath.Join(tmpDir, ".rafa"), 0755)

		err := RequireInitialized()
		if err != nil {
			t.Errorf("expected nil error when initialized, got: %v", err)
		}
	})

	t.Run("returns error when not initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		err := RequireInitialized()
		if err == nil {
			t.Error("expected error when not initialized, got nil")
		}

		expected := "rafa is not initialized. Run 'rafa init' first."
		if err.Error() != expected {
			t.Errorf("got %q, want %q", err.Error(), expected)
		}
	})
}
