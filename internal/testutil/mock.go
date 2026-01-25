// Package testutil provides testing utilities for the rafa project.
package testutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pablasso/rafa/internal/plan"
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

// OutputWriter provides writers for capturing command output.
// This interface matches executor.OutputWriter.
type OutputWriter interface {
	Stdout() io.Writer
	Stderr() io.Writer
}

// MockRunnerCall records the arguments of a MockRunner.Run call.
type MockRunnerCall struct {
	Task        *plan.Task
	PlanContext string
	Attempt     int
	MaxAttempts int
	Output      OutputWriter
}

// MockRunner is a test double for executor.Runner.
// Note: This type must be adapted when used with executor.Runner
// since the OutputWriter interfaces are structurally equivalent but
// defined in different packages.
type MockRunner struct {
	Responses []error
	CallCount int
	Calls     []MockRunnerCall
}

// Run records the call and returns the next error from Responses.
func (m *MockRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
	m.Calls = append(m.Calls, MockRunnerCall{
		Task:        task,
		PlanContext: planContext,
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
		Output:      output,
	})

	var err error
	if m.CallCount < len(m.Responses) {
		err = m.Responses[m.CallCount]
	}
	m.CallCount++
	return err
}
