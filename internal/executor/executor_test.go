package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// mockRunnerCall records the arguments of a mockRunner.Run call.
type mockRunnerCall struct {
	Task        *plan.Task
	PlanContext string
	Attempt     int
	MaxAttempts int
	Output      OutputWriter
}

// mockRunner is a test double for Runner.
type mockRunner struct {
	Responses []error
	CallCount int
	Calls     []mockRunnerCall
}

// Run records the call and returns the next error from Responses.
func (m *mockRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
	m.Calls = append(m.Calls, mockRunnerCall{
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

func createTestPlan(tasks []plan.Task) *plan.Plan {
	return &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks:       tasks,
	}
}

func createTestPlanDir(t *testing.T, p *plan.Plan) string {
	t.Helper()
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err == nil {
		tmpDir = resolved
	}

	if err := plan.SavePlan(tmpDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}
	return tmpDir
}

func TestExecutor_AllTasksComplete(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusCompleted},
		{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusCompleted},
	})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if mockRunner.CallCount != 0 {
		t.Errorf("expected no runner calls, got: %d", mockRunner.CallCount)
	}
}

func TestExecutor_NoPendingTasks(t *testing.T) {
	// Empty task list
	p := createTestPlan([]plan.Task{})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if mockRunner.CallCount != 0 {
		t.Errorf("expected no runner calls, got: %d", mockRunner.CallCount)
	}
}

func TestExecutor_RunsSingleTask(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{Responses: []error{nil}}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if mockRunner.CallCount != 1 {
		t.Errorf("expected 1 runner call, got: %d", mockRunner.CallCount)
	}
	if p.Tasks[0].Status != plan.TaskStatusCompleted {
		t.Errorf("expected task status completed, got: %s", p.Tasks[0].Status)
	}
	if p.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", p.Status)
	}
}

func TestExecutor_RunsMultipleTasks(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusPending},
		{ID: "task-3", Title: "Task 3", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{Responses: []error{nil, nil, nil}}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if mockRunner.CallCount != 3 {
		t.Errorf("expected 3 runner calls, got: %d", mockRunner.CallCount)
	}

	// Verify tasks were executed in order
	for i, call := range mockRunner.Calls {
		expectedID := p.Tasks[i].ID
		if call.Task.ID != expectedID {
			t.Errorf("call %d: expected task %s, got %s", i, expectedID, call.Task.ID)
		}
	}

	// Verify all tasks completed
	for i, task := range p.Tasks {
		if task.Status != plan.TaskStatusCompleted {
			t.Errorf("task %d: expected status completed, got: %s", i, task.Status)
		}
	}
}

func TestExecutor_RetriesOnFailure(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	// Fail twice, then succeed
	mockRunner := &mockRunner{
		Responses: []error{
			errors.New("fail 1"),
			errors.New("fail 2"),
			nil,
		},
	}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if mockRunner.CallCount != 3 {
		t.Errorf("expected 3 runner calls, got: %d", mockRunner.CallCount)
	}
	if p.Tasks[0].Attempts != 3 {
		t.Errorf("expected 3 attempts, got: %d", p.Tasks[0].Attempts)
	}
	if p.Tasks[0].Status != plan.TaskStatusCompleted {
		t.Errorf("expected task status completed, got: %s", p.Tasks[0].Status)
	}
}

func TestExecutor_StopsAfterMaxAttempts(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	// Fail all attempts
	responses := make([]error, MaxAttempts)
	for i := range responses {
		responses[i] = errors.New("fail")
	}
	mockRunner := &mockRunner{Responses: responses}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err == nil {
		t.Error("expected error after max attempts")
	}
	if !strings.Contains(err.Error(), "failed after") {
		t.Errorf("expected 'failed after' in error, got: %v", err)
	}
	if mockRunner.CallCount != MaxAttempts {
		t.Errorf("expected %d runner calls, got: %d", MaxAttempts, mockRunner.CallCount)
	}
	if p.Tasks[0].Status != plan.TaskStatusFailed {
		t.Errorf("expected task status failed, got: %s", p.Tasks[0].Status)
	}
	if p.Status != plan.PlanStatusFailed {
		t.Errorf("expected plan status failed, got: %s", p.Status)
	}
}

func TestExecutor_ResumesFromPending(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusCompleted, Attempts: 1},
		{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusCompleted, Attempts: 1},
		{ID: "task-3", Title: "Task 3", Status: plan.TaskStatusPending},
		{ID: "task-4", Title: "Task 4", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{Responses: []error{nil, nil}}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if mockRunner.CallCount != 2 {
		t.Errorf("expected 2 runner calls (skipping completed tasks), got: %d", mockRunner.CallCount)
	}

	// Verify only task-3 and task-4 were executed
	if mockRunner.Calls[0].Task.ID != "task-3" {
		t.Errorf("expected first call to be task-3, got: %s", mockRunner.Calls[0].Task.ID)
	}
	if mockRunner.Calls[1].Task.ID != "task-4" {
		t.Errorf("expected second call to be task-4, got: %s", mockRunner.Calls[1].Task.ID)
	}
}

func TestExecutor_CancellationHandled(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	mockRunner := &mockRunner{}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(ctx)

	// Should return nil on cancellation (graceful handling)
	if err != nil {
		t.Errorf("expected nil error on cancellation, got: %v", err)
	}
}

func TestExecutor_CancellationResetsTaskStatus(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	ctx, cancel := context.WithCancel(context.Background())

	// Create executor with a runner that cancels context during execution
	executor := New(planDir, p).WithAllowDirty(true)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		cancel() // Cancel context during task execution
		return context.Canceled
	})

	err := executor.Run(ctx)

	// Should return nil on cancellation
	if err != nil {
		t.Errorf("expected nil error on cancellation, got: %v", err)
	}

	// Reload plan from disk to verify state was saved
	savedPlan, loadErr := plan.LoadPlan(planDir)
	if loadErr != nil {
		t.Fatalf("failed to load saved plan: %v", loadErr)
	}

	if savedPlan.Tasks[0].Status != plan.TaskStatusPending {
		t.Errorf("expected task status pending after cancel, got: %s", savedPlan.Tasks[0].Status)
	}
}

func TestExecutor_AcquiresLock(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	lockPath := filepath.Join(planDir, "run.lock")
	lockAcquired := false

	// Mock runner that checks for lock file
	mockRunner := &mockRunner{
		Responses: []error{nil},
	}

	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	// Override runner to check for lock during execution
	originalRunner := executor.runner
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		if _, err := os.Stat(lockPath); err == nil {
			lockAcquired = true
		}
		return originalRunner.Run(ctx, task, planContext, attempt, maxAttempts, output)
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !lockAcquired {
		t.Error("expected lock file to exist during execution")
	}
}

func TestExecutor_ReleasesLockOnComplete(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{Responses: []error{nil}}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	lockPath := filepath.Join(planDir, "run.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed after completion")
	}
}

func TestExecutor_ReleasesLockOnCancel(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockRunner := &mockRunner{}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	_ = executor.Run(ctx)

	lockPath := filepath.Join(planDir, "run.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed after cancellation")
	}
}

func TestExecutor_ReleasesLockOnFailure(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	// Fail all attempts
	responses := make([]error, MaxAttempts)
	for i := range responses {
		responses[i] = errors.New("fail")
	}
	mockRunner := &mockRunner{Responses: responses}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	_ = executor.Run(context.Background())

	lockPath := filepath.Join(planDir, "run.lock")
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed after failure")
	}
}

func TestExecutor_ConcurrentRunBlocked(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	// Create lock file with current PID to simulate our process holding the lock
	// This works because the lock manager checks if the process exists
	lockPath := filepath.Join(planDir, "run.lock")
	currentPID := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(lockPath, []byte(currentPID), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	mockRunner := &mockRunner{}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err == nil {
		t.Error("expected error when lock is held")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("expected 'already running' in error, got: %v", err)
	}
}

func TestExecutor_SavesStateAfterEachTask(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	saveCount := 0
	mockRunner := &mockRunner{Responses: []error{nil, nil}}
	executor := New(planDir, p).
		WithRunner(mockRunner).
		WithAllowDirty(true).
		WithSaveHook(func() { saveCount++ })

	_ = executor.Run(context.Background())

	// Expected saves: plan->InProgress, task1->InProgress, task1->Completed,
	//                 task2->InProgress, task2->Completed, plan->Completed
	if saveCount != 6 {
		t.Errorf("expected 6 saves, got: %d", saveCount)
	}
}

func TestExecutor_LogsProgressEvents(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	mockRunner := &mockRunner{Responses: []error{nil}}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Read progress log
	logPath := filepath.Join(planDir, "progress.log")
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open progress log: %v", err)
	}
	defer f.Close()

	var events []plan.ProgressEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event plan.ProgressEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("failed to parse progress event: %v", err)
		}
		events = append(events, event)
	}

	// Verify expected events
	expectedEvents := []string{
		plan.EventPlanStarted,
		plan.EventTaskStarted,
		plan.EventTaskCompleted,
		plan.EventPlanCompleted,
	}

	if len(events) != len(expectedEvents) {
		t.Errorf("expected %d events, got: %d", len(expectedEvents), len(events))
		for i, e := range events {
			t.Logf("event %d: %s", i, e.Event)
		}
	}

	for i, expected := range expectedEvents {
		if i >= len(events) {
			break
		}
		if events[i].Event != expected {
			t.Errorf("event %d: expected %s, got: %s", i, expected, events[i].Event)
		}
	}
}

func TestExecutor_BuildPlanContext(t *testing.T) {
	p := &plan.Plan{
		Name:        "My Plan",
		Description: "Plan description",
		SourceFile:  "/path/to/design.md",
	}
	planDir := t.TempDir()

	executor := New(planDir, p)
	ctx := executor.buildPlanContext()

	if !strings.Contains(ctx, "My Plan") {
		t.Error("expected plan name in context")
	}
	if !strings.Contains(ctx, "Plan description") {
		t.Error("expected plan description in context")
	}
	if !strings.Contains(ctx, "/path/to/design.md") {
		t.Error("expected source file in context")
	}
}

func TestExecutor_FormatDuration(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "00:30"},
		{90 * time.Second, "01:30"},
		{5 * time.Minute, "05:00"},
		{65 * time.Minute, "01:05:00"},
		{2*time.Hour + 30*time.Minute + 45*time.Second, "02:30:45"},
	}

	for _, tt := range tests {
		result := executor.formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v): expected %s, got %s", tt.duration, tt.expected, result)
		}
	}
}

func TestExecutor_CountCompleted(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Status: plan.TaskStatusCompleted},
		{ID: "task-2", Status: plan.TaskStatusPending},
		{ID: "task-3", Status: plan.TaskStatusCompleted},
		{ID: "task-4", Status: plan.TaskStatusFailed},
	})

	executor := &Executor{plan: p}
	count := executor.countCompleted()

	if count != 2 {
		t.Errorf("expected 2 completed tasks, got: %d", count)
	}
}

func TestExecutor_RerunFailedPlanResetsAttempts(t *testing.T) {
	// Simulate a plan that previously failed with max attempts reached
	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusFailed, // Plan already failed
		Tasks: []plan.Task{
			{
				ID:       "task-1",
				Title:    "Task 1",
				Status:   plan.TaskStatusInProgress, // Left in_progress after failure
				Attempts: MaxAttempts,               // Already at max attempts
			},
			{
				ID:       "task-2",
				Title:    "Task 2",
				Status:   plan.TaskStatusPending,
				Attempts: 0,
			},
		},
	}
	planDir := createTestPlanDir(t, p)

	// This time the task succeeds
	mockRunner := &mockRunner{Responses: []error{nil, nil}}
	executor := New(planDir, p).WithRunner(mockRunner).WithAllowDirty(true)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error when re-running failed plan, got: %v", err)
	}

	// Verify runner was called (attempts were reset, allowing the task to run)
	if mockRunner.CallCount != 2 {
		t.Errorf("expected 2 runner calls, got: %d", mockRunner.CallCount)
	}

	// Verify task-1's attempts were reset (should be 1 after successful run)
	if p.Tasks[0].Attempts != 1 {
		t.Errorf("expected task-1 attempts to be 1 after reset and success, got: %d", p.Tasks[0].Attempts)
	}

	// Verify both tasks completed
	if p.Tasks[0].Status != plan.TaskStatusCompleted {
		t.Errorf("expected task-1 status completed, got: %s", p.Tasks[0].Status)
	}
	if p.Tasks[1].Status != plan.TaskStatusCompleted {
		t.Errorf("expected task-2 status completed, got: %s", p.Tasks[1].Status)
	}

	// Verify plan completed
	if p.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", p.Status)
	}
}

// runnerFunc is a function adapter for the Runner interface.
type runnerFunc func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error

func (f runnerFunc) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
	return f(ctx, task, planContext, attempt, maxAttempts, output)
}
