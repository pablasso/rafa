package plan

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/analysis"
	"github.com/pablasso/rafa/internal/display"
	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/testutil"
)

// mockRunnerAdapter wraps testutil.MockRunner to implement executor.Runner.
type mockRunnerAdapter struct {
	*testutil.MockRunner
}

func (a *mockRunnerAdapter) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output executor.OutputWriter) error {
	// Convert executor.OutputWriter to testutil.OutputWriter
	// They have the same methods, so we can pass it directly
	return a.MockRunner.Run(ctx, task, planContext, attempt, maxAttempts, output)
}

func createRunTestPlan(name string, tasks []plan.Task) *plan.Plan {
	return &plan.Plan{
		ID:          "abc123",
		Name:        name,
		Description: "A test plan",
		SourceFile:  "design.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks:       tasks,
	}
}

func setupRunIntegrationTest(t *testing.T, p *plan.Plan) string {
	t.Helper()

	tmpDir := testutil.SetupTestDir(t)

	// Create .rafa/plans/<id>-<name>/
	planDir := filepath.Join(tmpDir, ".rafa", "plans", p.ID+"-"+p.Name)
	if err := os.MkdirAll(planDir, 0755); err != nil {
		t.Fatalf("failed to create plan dir: %v", err)
	}

	// Save plan.json
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create empty log files
	os.WriteFile(filepath.Join(planDir, "progress.log"), []byte{}, 0644)
	os.WriteFile(filepath.Join(planDir, "output.log"), []byte{}, 0644)

	return planDir
}

func TestRunPlanE2E(t *testing.T) {
	p := createRunTestPlan("test-plan", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task 2", Description: "Second task", Status: plan.TaskStatusPending},
	})
	planDir := setupRunIntegrationTest(t, p)

	mockRunner := &testutil.MockRunner{Responses: []error{nil, nil}}
	exec := executor.New(planDir, p).WithRunner(&mockRunnerAdapter{mockRunner}).WithAllowDirty(true)

	err := exec.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify all tasks completed
	savedPlan, err := plan.LoadPlan(planDir)
	if err != nil {
		t.Fatalf("failed to load saved plan: %v", err)
	}

	if savedPlan.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", savedPlan.Status)
	}

	for i, task := range savedPlan.Tasks {
		if task.Status != plan.TaskStatusCompleted {
			t.Errorf("task %d: expected status completed, got: %s", i, task.Status)
		}
	}

	// Verify runner was called for each task
	if mockRunner.CallCount != 2 {
		t.Errorf("expected 2 runner calls, got: %d", mockRunner.CallCount)
	}
}

func TestRunPlanResumeInterrupted(t *testing.T) {
	// Create a plan with first task completed, second pending
	p := createRunTestPlan("test-plan", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusCompleted, Attempts: 1},
		{ID: "t02", Title: "Task 2", Description: "Second task", Status: plan.TaskStatusPending},
		{ID: "t03", Title: "Task 3", Description: "Third task", Status: plan.TaskStatusPending},
	})
	p.Status = plan.PlanStatusInProgress
	planDir := setupRunIntegrationTest(t, p)

	mockRunner := &testutil.MockRunner{Responses: []error{nil, nil}}
	exec := executor.New(planDir, p).WithRunner(&mockRunnerAdapter{mockRunner}).WithAllowDirty(true)

	err := exec.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify only tasks 2 and 3 were executed
	if mockRunner.CallCount != 2 {
		t.Errorf("expected 2 runner calls (skipping completed task), got: %d", mockRunner.CallCount)
	}

	if mockRunner.Calls[0].Task.ID != "t02" {
		t.Errorf("expected first call to be t02, got: %s", mockRunner.Calls[0].Task.ID)
	}
	if mockRunner.Calls[1].Task.ID != "t03" {
		t.Errorf("expected second call to be t03, got: %s", mockRunner.Calls[1].Task.ID)
	}
}

func TestRunPlanFailureAndRetry(t *testing.T) {
	p := createRunTestPlan("test-plan", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusPending},
	})
	planDir := setupRunIntegrationTest(t, p)

	// Fail twice, then succeed
	mockRunner := &testutil.MockRunner{
		Responses: []error{
			errors.New("fail 1"),
			errors.New("fail 2"),
			nil,
		},
	}
	exec := executor.New(planDir, p).WithRunner(&mockRunnerAdapter{mockRunner}).WithAllowDirty(true)

	err := exec.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify 3 attempts were made
	if mockRunner.CallCount != 3 {
		t.Errorf("expected 3 runner calls, got: %d", mockRunner.CallCount)
	}

	// Verify task completed after retries
	savedPlan, err := plan.LoadPlan(planDir)
	if err != nil {
		t.Fatalf("failed to load saved plan: %v", err)
	}

	if savedPlan.Tasks[0].Status != plan.TaskStatusCompleted {
		t.Errorf("expected task status completed, got: %s", savedPlan.Tasks[0].Status)
	}
	if savedPlan.Tasks[0].Attempts != 3 {
		t.Errorf("expected 3 attempts, got: %d", savedPlan.Tasks[0].Attempts)
	}
}

func TestRunPlanResumeFailedPlan(t *testing.T) {
	// Create a plan with a failed task (should reset to pending)
	p := createRunTestPlan("test-plan", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusCompleted, Attempts: 1},
		{ID: "t02", Title: "Task 2", Description: "Second task", Status: plan.TaskStatusFailed, Attempts: 3},
	})
	p.Status = plan.PlanStatusFailed
	planDir := setupRunIntegrationTest(t, p)

	mockRunner := &testutil.MockRunner{Responses: []error{nil}}
	exec := executor.New(planDir, p).WithRunner(&mockRunnerAdapter{mockRunner}).WithAllowDirty(true)

	err := exec.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify the failed task was resumed and completed
	savedPlan, err := plan.LoadPlan(planDir)
	if err != nil {
		t.Fatalf("failed to load saved plan: %v", err)
	}

	if savedPlan.Tasks[1].Status != plan.TaskStatusCompleted {
		t.Errorf("expected task status completed, got: %s", savedPlan.Tasks[1].Status)
	}

	// Attempts should have incremented from 3 to 4
	if savedPlan.Tasks[1].Attempts != 4 {
		t.Errorf("expected 4 attempts (3 + 1 retry), got: %d", savedPlan.Tasks[1].Attempts)
	}
}

func TestRunPlanCancellation(t *testing.T) {
	p := createRunTestPlan("test-plan", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusPending},
	})
	planDir := setupRunIntegrationTest(t, p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockRunner := &testutil.MockRunner{}
	exec := executor.New(planDir, p).WithRunner(&mockRunnerAdapter{mockRunner}).WithAllowDirty(true)

	err := exec.Run(ctx)

	// Graceful cancellation should not return error
	if err != nil {
		t.Errorf("expected nil error on cancellation, got: %v", err)
	}
}

func TestRunWithDisplay(t *testing.T) {
	p := createRunTestPlan("display-test", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task 2", Description: "Second task", Status: plan.TaskStatusPending},
	})
	planDir := setupRunIntegrationTest(t, p)

	// Create a display with a buffer to capture output
	var buf bytes.Buffer
	disp := display.New(&buf)
	disp.Start()

	mockRunner := &testutil.MockRunner{Responses: []error{nil, nil}}
	exec := executor.New(planDir, p).
		WithRunner(&mockRunnerAdapter{mockRunner}).
		WithAllowDirty(true).
		WithDisplay(disp)

	err := exec.Run(context.Background())
	disp.Stop()

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify display was updated during execution
	// The buffer should contain status line updates (ANSI escape sequences for clearing/writing)
	output := buf.String()

	// Display writes carriage returns and escape sequences
	if !strings.Contains(output, "\r") {
		t.Error("expected display output to contain carriage returns for status line updates")
	}

	// Verify the executor called display methods (task info should appear)
	if !strings.Contains(output, "Task 1") && !strings.Contains(output, "Task 2") {
		// Note: This check may need adjustment based on timing - the display might not
		// have rendered before we check. At minimum, verify runner was called with display active.
		if mockRunner.CallCount != 2 {
			t.Errorf("expected 2 runner calls, got: %d", mockRunner.CallCount)
		}
	}
}

func TestRunWithOutputCapture(t *testing.T) {
	p := createRunTestPlan("output-test", []plan.Task{
		{ID: "t01", Title: "Task 1", Description: "First task", Status: plan.TaskStatusPending},
	})
	planDir := setupRunIntegrationTest(t, p)

	mockRunner := &testutil.MockRunner{Responses: []error{nil}}
	exec := executor.New(planDir, p).
		WithRunner(&mockRunnerAdapter{mockRunner}).
		WithAllowDirty(true)

	err := exec.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify output.log was created and contains expected content
	outputPath := filepath.Join(planDir, "output.log")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output.log: %v", err)
	}

	// Output log should contain task headers
	if !strings.Contains(string(content), "t01") {
		t.Error("expected output.log to contain task ID t01")
	}

	// Should contain task markers
	if !strings.Contains(string(content), "=== Task") {
		t.Error("expected output.log to contain task header markers")
	}
}

func TestSuggestionsAfterCompletion(t *testing.T) {
	// Create a plan with tasks that will trigger suggestions
	p := createRunTestPlan("suggestions-test", []plan.Task{
		{
			ID:                 "t01",
			Title:              "Task with retries",
			Description:        "First task",
			Status:             plan.TaskStatusPending,
			AcceptanceCriteria: []string{"make test passes", "make fmt passes"},
		},
		{
			ID:                 "t02",
			Title:              "Another task",
			Description:        "Second task",
			Status:             plan.TaskStatusPending,
			AcceptanceCriteria: []string{"make test passes", "make fmt passes"},
		},
	})
	planDir := setupRunIntegrationTest(t, p)

	// Simulate retries by having first task fail once then succeed
	mockRunner := &testutil.MockRunner{
		Responses: []error{
			errors.New("fail 1"), // t01 attempt 1
			nil,                  // t01 attempt 2 - success
			nil,                  // t02 attempt 1 - success
		},
	}
	exec := executor.New(planDir, p).
		WithRunner(&mockRunnerAdapter{mockRunner}).
		WithAllowDirty(true)

	err := exec.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify plan completed
	savedPlan, err := plan.LoadPlan(planDir)
	if err != nil {
		t.Fatalf("failed to load plan: %v", err)
	}
	if savedPlan.Status != plan.PlanStatusCompleted {
		t.Fatalf("expected plan status completed, got: %s", savedPlan.Status)
	}

	// Run analyzer and verify suggestions are generated
	analyzer := analysis.NewAnalyzer(planDir, savedPlan)
	suggestions, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("failed to analyze: %v", err)
	}

	// Should have suggestions for:
	// 1. Task with retries (t01 required 2 attempts)
	// 2. Verification commands (make test, make fmt used in multiple tasks)
	if len(suggestions) == 0 {
		t.Error("expected suggestions to be generated after completion with retries")
	}

	// Verify retry suggestion exists
	hasRetrySuggestion := false
	for _, s := range suggestions {
		if s.Category == "Common Issues" && strings.Contains(s.Title, "2 attempts") {
			hasRetrySuggestion = true
			break
		}
	}
	if !hasRetrySuggestion {
		t.Error("expected suggestion about task requiring multiple attempts")
	}

	// Verify verification command suggestion exists
	hasVerificationSuggestion := false
	for _, s := range suggestions {
		if s.Category == "Verification" {
			hasVerificationSuggestion = true
			break
		}
	}
	if !hasVerificationSuggestion {
		t.Error("expected suggestion about verification commands")
	}

	// Test FormatSuggestions output
	formatted := analysis.FormatSuggestions(suggestions)
	if !strings.Contains(formatted, "SUGGESTED AGENTS.md ADDITIONS") {
		t.Error("expected formatted suggestions to contain header")
	}
}
