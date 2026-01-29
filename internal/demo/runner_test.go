package demo

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// mockOutputWriter implements executor.OutputWriter for testing.
type mockOutputWriter struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func newMockOutputWriter() *mockOutputWriter {
	return &mockOutputWriter{
		stdout: new(bytes.Buffer),
		stderr: new(bytes.Buffer),
	}
}

func (m *mockOutputWriter) Stdout() io.Writer { return m.stdout }
func (m *mockOutputWriter) Stderr() io.Writer { return m.stderr }

// StdoutBuffer returns the underlying buffer for inspection in tests.
func (m *mockOutputWriter) StdoutBuffer() *bytes.Buffer { return m.stdout }

func TestShouldFail_ScenarioSuccess(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	testCases := []struct {
		taskID  string
		attempt int
	}{
		{"t01", 1},
		{"t02", 1},
		{"t03", 1},
		{"t01", 5},
		{"unknown", 1},
	}

	for _, tc := range testCases {
		task := &plan.Task{ID: tc.taskID}
		if runner.shouldFail(task, tc.attempt) {
			t.Errorf("ScenarioSuccess: shouldFail(%s, %d) = true, want false",
				tc.taskID, tc.attempt)
		}
	}
}

func TestShouldFail_ScenarioFail(t *testing.T) {
	config := NewConfig(ScenarioFail, SpeedFast)
	runner := NewDemoRunner(config)

	testCases := []struct {
		taskID  string
		attempt int
	}{
		{"t01", 1},
		{"t02", 1},
		{"t03", 1},
		{"t01", 5},
		{"unknown", 1},
	}

	for _, tc := range testCases {
		task := &plan.Task{ID: tc.taskID}
		if !runner.shouldFail(task, tc.attempt) {
			t.Errorf("ScenarioFail: shouldFail(%s, %d) = false, want true",
				tc.taskID, tc.attempt)
		}
	}
}

func TestShouldFail_ScenarioRetry(t *testing.T) {
	config := NewConfig(ScenarioRetry, SpeedFast)
	runner := NewDemoRunner(config)

	testCases := []struct {
		taskID   string
		attempt  int
		wantFail bool
	}{
		// First 2 attempts fail, 3rd succeeds
		{"t01", 1, true},
		{"t01", 2, true},
		{"t01", 3, false},
		{"t01", 4, false},
		{"t01", 5, false},
		// Same for any task
		{"t02", 1, true},
		{"t02", 2, true},
		{"t02", 3, false},
		{"t03", 1, true},
		{"t03", 3, false},
		{"unknown", 1, true},
		{"unknown", 3, false},
	}

	for _, tc := range testCases {
		task := &plan.Task{ID: tc.taskID}
		got := runner.shouldFail(task, tc.attempt)
		if got != tc.wantFail {
			t.Errorf("ScenarioRetry: shouldFail(%s, %d) = %v, want %v",
				tc.taskID, tc.attempt, got, tc.wantFail)
		}
	}
}

func TestShouldFail_ScenarioMixed(t *testing.T) {
	config := NewConfig(ScenarioMixed, SpeedFast)
	runner := NewDemoRunner(config)

	testCases := []struct {
		taskID   string
		attempt  int
		wantFail bool
	}{
		// t01 always passes
		{"t01", 1, false},
		{"t01", 2, false},
		{"t01", 5, false},

		// t02 needs retry: fails attempt 1, succeeds attempt 2+
		{"t02", 1, true},
		{"t02", 2, false},
		{"t02", 3, false},
		{"t02", 5, false},

		// t03 always fails
		{"t03", 1, true},
		{"t03", 2, true},
		{"t03", 3, true},
		{"t03", 5, true},

		// t04 always passes
		{"t04", 1, false},
		{"t04", 5, false},

		// t05 always passes
		{"t05", 1, false},

		// Unknown tasks pass
		{"t99", 1, false},
		{"unknown", 1, false},
	}

	for _, tc := range testCases {
		task := &plan.Task{ID: tc.taskID}
		got := runner.shouldFail(task, tc.attempt)
		if got != tc.wantFail {
			t.Errorf("ScenarioMixed: shouldFail(%s, %d) = %v, want %v",
				tc.taskID, tc.attempt, got, tc.wantFail)
		}
	}
}

func TestShouldFail_UnknownScenario(t *testing.T) {
	// Test that unknown scenarios default to not failing
	config := &Config{Scenario: Scenario("unknown"), Speed: SpeedFast}
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "t01"}
	if runner.shouldFail(task, 1) {
		t.Error("Unknown scenario: shouldFail should return false by default")
	}
}

func TestDemoRunner_Run_Success(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	config.TaskDelay = 10 * time.Millisecond // Speed up test
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
			"Criterion 2",
		},
	}

	output := newMockOutputWriter()
	err := runner.Run(context.Background(), task, "plan context", 1, 5, output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Check that output contains expected content
	stdout := output.StdoutBuffer().String()
	if !strings.Contains(stdout, "Working on: Test task") {
		t.Error("Output should contain task title")
	}
	if !strings.Contains(stdout, "All acceptance criteria verified") {
		t.Error("Output should indicate success")
	}
	if !strings.Contains(stdout, "SUGGESTED_COMMIT_MESSAGE:") {
		t.Error("Output should contain commit message")
	}
}

func TestDemoRunner_Run_Failure(t *testing.T) {
	config := NewConfig(ScenarioFail, SpeedFast)
	config.TaskDelay = 10 * time.Millisecond // Speed up test
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
			"Criterion 2",
		},
	}

	output := newMockOutputWriter()
	err := runner.Run(context.Background(), task, "plan context", 1, 5, output)

	if err == nil {
		t.Error("Run() error = nil, want error")
	}

	// Check that output contains failure indication
	stdout := output.StdoutBuffer().String()
	if !strings.Contains(stdout, "FAILED") {
		t.Error("Output should indicate failure")
	}
	if !strings.Contains(stdout, "Task requires retry") {
		t.Error("Output should suggest retry")
	}
}

func TestDemoRunner_Run_ContextCancellation(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedSlow)
	// Keep a longer delay to ensure context cancellation happens mid-stream
	config.TaskDelay = 5 * time.Second
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	output := newMockOutputWriter()
	err := runner.Run(ctx, task, "plan context", 1, 5, output)

	// Should complete without error even with cancelled context
	// (the output streaming is cancelled but Run still completes)
	if err != nil {
		t.Errorf("Run() error = %v, want nil (context cancellation handled gracefully)", err)
	}
}

func TestStreamOutput_ContextCancellationStopsStreaming(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedNormal)
	// Use a long delay per line to ensure we can cancel mid-stream
	config.TaskDelay = 10 * time.Second
	runner := NewDemoRunner(config)

	// Task that generates many lines of output
	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
			"Criterion 2",
			"Criterion 3",
			"Criterion 4",
			"Criterion 5",
		},
	}

	// Cancel context after a short time - should stop streaming
	ctx, cancel := context.WithCancel(context.Background())
	output := newMockOutputWriter()

	// Generate expected output to count lines
	expectedLines := runner.generateOutput(task, 1)
	totalExpectedLines := len(expectedLines)

	// Start streaming in a goroutine
	done := make(chan struct{})
	go func() {
		runner.streamOutput(ctx, task, 1, output)
		close(done)
	}()

	// Wait a bit for some output to be written, then cancel
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for streamOutput to return
	select {
	case <-done:
		// Good - streamOutput returned
	case <-time.After(500 * time.Millisecond):
		t.Fatal("streamOutput did not return promptly after context cancellation")
	}

	// Verify that NOT all lines were written (streaming was stopped)
	outputStr := output.StdoutBuffer().String()
	linesWritten := strings.Count(outputStr, "\n")

	// Should have written fewer lines than total expected
	// (since we cancelled early with a long delay)
	if linesWritten >= totalExpectedLines {
		t.Errorf("Context cancellation should stop streaming: wrote %d lines, expected fewer than %d",
			linesWritten, totalExpectedLines)
	}

	// Verify at least some output was written before cancellation
	if linesWritten == 0 {
		t.Error("Expected some output to be written before cancellation")
	}
}

func TestDemoRunner_Run_RetryAttempt(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	config.TaskDelay = 10 * time.Millisecond // Speed up test
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	output := newMockOutputWriter()
	err := runner.Run(context.Background(), task, "plan context", 2, 5, output)

	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Check that output contains retry context
	stdout := output.StdoutBuffer().String()
	if !strings.Contains(stdout, "Retrying task (attempt 2)") {
		t.Error("Output should indicate retry attempt")
	}
	if !strings.Contains(stdout, "Analyzing previous failure") {
		t.Error("Output should contain retry context")
	}
}

func TestGenerateCommitMessage(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	testCases := []struct {
		taskID  string
		title   string
		wantMsg string
	}{
		{"t01", "Set up project structure", "Set up project structure with configuration files"},
		{"t02", "Implement core data models", "Implement core data models and interfaces"},
		{"t03", "Add business logic layer", "Add business logic layer with validation"},
		{"t04", "Create API endpoints", "Create REST API endpoints with handlers"},
		{"t05", "Write documentation", "Add documentation and usage examples"},
		{"t99", "Custom task title", "Complete custom task title"},
		{"unknown", "Another task", "Complete another task"},
	}

	for _, tc := range testCases {
		task := &plan.Task{ID: tc.taskID, Title: tc.title}
		got := runner.generateCommitMessage(task)
		if got != tc.wantMsg {
			t.Errorf("generateCommitMessage(%s) = %q, want %q",
				tc.taskID, got, tc.wantMsg)
		}
	}
}

func TestGenerateWorkLines(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	// Test known task IDs return specific work lines
	knownIDs := []string{"t01", "t02", "t03", "t04", "t05"}
	for _, id := range knownIDs {
		task := &plan.Task{ID: id}
		lines := runner.generateWorkLines(task)
		if len(lines) == 0 {
			t.Errorf("generateWorkLines(%s) returned empty slice", id)
		}
	}

	// Test unknown task ID returns fallback
	unknownTask := &plan.Task{ID: "unknown"}
	lines := runner.generateWorkLines(unknownTask)
	if len(lines) != 3 {
		t.Errorf("generateWorkLines(unknown) returned %d lines, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "Analyzing requirements") {
		t.Error("Fallback should contain 'Analyzing requirements'")
	}
}

func TestNewDemoRunner(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	if runner == nil {
		t.Fatal("NewDemoRunner returned nil")
	}
	if runner.config != config {
		t.Error("Runner config not set correctly")
	}
}
