// Package testutil provides testing utilities for the rafa project.
package testutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// MockCommandFunc creates a mock command that outputs the given response.
// Usage: ai.CommandContext = testutil.MockCommandFunc(jsonResponse)
func MockCommandFunc(output string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "-n", output)
	}
}

// SetupTestDir creates a temp directory, resolves symlinks (for macOS),
// changes to it, and registers cleanup to restore the original working directory.
// Returns the resolved temp directory path.
func SetupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	// Resolve symlinks for macOS (/var -> /private/var)
	if resolved, err := filepath.EvalSymlinks(tmpDir); err != nil {
		t.Logf("warning: could not resolve symlinks for temp dir: %v", err)
	} else {
		tmpDir = resolved
	}

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.Chdir(originalWd)
	})

	return tmpDir
}
