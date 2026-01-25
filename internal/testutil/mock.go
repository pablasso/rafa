// Package testutil provides testing utilities for the rafa project.
package testutil

import (
	"context"
	"fmt"
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

// MockCommandFuncFail creates a mock command that exits with a non-zero exit code.
// Usage: ai.CommandContext = testutil.MockCommandFuncFail(exitCode)
func MockCommandFuncFail(exitCode int) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("exit %d", exitCode))
	}
}

// MockCommandFuncSleep creates a mock command that sleeps for the given duration.
// Useful for testing context cancellation.
// Usage: ai.CommandContext = testutil.MockCommandFuncSleep("10")
func MockCommandFuncSleep(seconds string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", seconds)
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
