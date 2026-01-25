package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

func TestExecutor_GetCommitMessage_UsesAgentSuggestion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an output capture with a suggested commit message
	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}
	oc.Stdout().Write([]byte("Some output\n"))
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: Add user authentication feature\n"))
	oc.Stdout().Write([]byte("More output\n"))
	oc.logFile.Sync()

	task := &plan.Task{
		ID:    "t01",
		Title: "Implement login",
	}

	executor := &Executor{}
	msg := executor.getCommitMessage(task, oc)

	expected := "Add user authentication feature"
	if msg != expected {
		t.Errorf("getCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}

func TestExecutor_GetCommitMessage_FallbackToDefault(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an output capture without a suggested commit message
	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}
	oc.Stdout().Write([]byte("Some output without commit message\n"))
	oc.logFile.Sync()

	task := &plan.Task{
		ID:    "t01",
		Title: "Implement login",
	}

	executor := &Executor{}
	msg := executor.getCommitMessage(task, oc)

	expected := "[rafa] Complete task t01: Implement login"
	if msg != expected {
		t.Errorf("getCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}

func TestExecutor_GetCommitMessage_HandlesNilOutput(t *testing.T) {
	task := &plan.Task{
		ID:    "t02",
		Title: "Fix bug in parser",
	}

	executor := &Executor{}
	msg := executor.getCommitMessage(task, nil)

	expected := "[rafa] Complete task t02: Fix bug in parser"
	if msg != expected {
		t.Errorf("getCommitMessage() = %q, want %q", msg, expected)
	}
}

// setupTestGitRepo creates a test git repository with the proper .rafa/plans structure.
// Returns (repoRoot, planDir).
func setupTestGitRepo(t *testing.T) (string, string) {
	t.Helper()

	// Create temp directory for git repo
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err == nil {
		tmpDir = resolved
	}

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

	// Create .rafa/plans/<id>-<name>/ structure
	planDir := filepath.Join(tmpDir, ".rafa", "plans", "test-plan-id-test")
	if err := os.MkdirAll(planDir, 0755); err != nil {
		t.Fatalf("failed to create plan dir: %v", err)
	}

	// Create .gitignore to match production behavior (ignore lock files)
	gitignore := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(".rafa/**/*.lock\n"), 0644); err != nil {
		t.Fatalf("failed to create .gitignore: %v", err)
	}

	// Create initial commit so repo has a HEAD
	initialFile := filepath.Join(tmpDir, ".gitkeep")
	if err := os.WriteFile(initialFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}
	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir, planDir
}

func TestExecutor_CommitsImplementationAndMetadata(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that simulates agent creating implementation files
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Simulate agent creating implementation files
		implFile := filepath.Join(repoRoot, "implementation.go")
		if err := os.WriteFile(implFile, []byte("package main\n"), 0644); err != nil {
			return err
		}
		return nil
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify workspace is clean after run
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("expected clean workspace after run, got dirty files:\n%s", output)
	}

	// Verify the task commit (HEAD~1) includes both implementation and metadata
	// The plan completion commit (HEAD) only has final metadata
	cmd = exec.Command("git", "show", "--name-only", "--oneline", "HEAD~1")
	cmd.Dir = repoRoot
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git show: %v", err)
	}
	outputStr := string(output)

	// Check implementation file is in task commit
	if !strings.Contains(outputStr, "implementation.go") {
		t.Errorf("expected implementation.go in task commit, got:\n%s", outputStr)
	}

	// Check metadata files are in task commit
	if !strings.Contains(outputStr, ".rafa/plans") {
		t.Errorf("expected .rafa/plans in task commit, got:\n%s", outputStr)
	}

	// Verify the commit message includes [rafa] prefix
	if !strings.Contains(outputStr, "[rafa]") {
		t.Errorf("expected [rafa] prefix in commit message, got:\n%s", outputStr)
	}
}

func TestExecutor_FailedTaskLeavesWorkspaceDirty(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that simulates agent failing after creating files
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Simulate agent creating implementation files then failing
		implFile := filepath.Join(repoRoot, "partial_impl.go")
		if err := os.WriteFile(implFile, []byte("package main\n// incomplete\n"), 0644); err != nil {
			return err
		}
		return errors.New("task failed")
	})

	err := executor.Run(context.Background())

	// Should fail after max attempts
	if err == nil {
		t.Error("expected error after task failure")
	}

	// Verify workspace is still dirty (implementation file not committed)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if !strings.Contains(string(output), "partial_impl.go") {
		t.Errorf("expected partial_impl.go in dirty files, got:\n%s", output)
	}
}

func TestExecutor_AllowDirtySkipsCommits(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Get current commit count
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	commitCountBefore := strings.TrimSpace(string(output))

	// Create executor with allowDirty=true
	executor := New(planDir, p).WithAllowDirty(true)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Simulate agent creating implementation files
		implFile := filepath.Join(repoRoot, "implementation.go")
		if err := os.WriteFile(implFile, []byte("package main\n"), 0644); err != nil {
			return err
		}
		return nil
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify no new commits were made
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot
	output, _ = cmd.Output()
	commitCountAfter := strings.TrimSpace(string(output))

	if commitCountBefore != commitCountAfter {
		t.Errorf("expected no new commits with allowDirty, got %s before and %s after", commitCountBefore, commitCountAfter)
	}

	// Verify workspace is dirty (changes not committed)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ = cmd.Output()
	if !strings.Contains(string(output), "implementation.go") {
		t.Errorf("expected implementation.go in dirty files with allowDirty, got:\n%s", output)
	}
}

func TestExecutor_WorkspaceCleanAfterSuccessfulPlanCompletion(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
			{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that simulates agent work
	taskNum := 0
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		taskNum++
		// Simulate agent creating different files for each task
		implFile := filepath.Join(repoRoot, fmt.Sprintf("feature_%d.go", taskNum))
		if err := os.WriteFile(implFile, []byte(fmt.Sprintf("package main\n// Feature %d\n", taskNum)), 0644); err != nil {
			return err
		}
		return nil
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify workspace is clean after full plan completion
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("expected clean workspace after successful plan completion, got dirty files:\n%s", output)
	}

	// Verify plan status is completed
	if p.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", p.Status)
	}
}

func TestExecutor_AgentAccidentallyCommits_HandledGracefully(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that simulates agent accidentally committing
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Simulate agent creating implementation files
		implFile := filepath.Join(repoRoot, "implementation.go")
		if err := os.WriteFile(implFile, []byte("package main\n"), 0644); err != nil {
			return err
		}

		// Agent accidentally commits (which they shouldn't do, but we handle gracefully)
		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = repoRoot
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("git", "commit", "-m", "Agent commit")
		cmd.Dir = repoRoot
		if err := cmd.Run(); err != nil {
			return err
		}

		return nil
	})

	err := executor.Run(context.Background())

	// Should succeed even though agent committed
	if err != nil {
		t.Errorf("expected no error when agent accidentally commits, got: %v", err)
	}

	// Verify workspace is clean after run
	// Executor should have committed remaining metadata (plan status update)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("expected clean workspace after run, got dirty files:\n%s", output)
	}

	// Verify plan completed
	if p.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", p.Status)
	}
}

// =============================================================================
// Integration tests for executor commit flow (per design doc testing section)
// These test names match the acceptance criteria from the design document.
// =============================================================================

// TestRun_ExecutorCommitsAfterTask verifies that the executor commits both
// implementation changes and metadata after a successful task.
func TestRun_ExecutorCommitsAfterTask(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that creates implementation files
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Simulate agent creating implementation files
		implFile := filepath.Join(repoRoot, "implementation.go")
		return os.WriteFile(implFile, []byte("package main\n"), 0644)
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify workspace is clean (executor committed)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("expected clean workspace after run, got dirty files:\n%s", output)
	}

	// Verify the task commit includes both implementation and metadata
	cmd = exec.Command("git", "show", "--name-only", "--oneline", "HEAD~1")
	cmd.Dir = repoRoot
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git show: %v", err)
	}
	outputStr := string(output)

	// Check implementation file is in task commit
	if !strings.Contains(outputStr, "implementation.go") {
		t.Errorf("expected implementation.go in task commit, got:\n%s", outputStr)
	}

	// Check metadata files are in task commit
	if !strings.Contains(outputStr, ".rafa/plans") {
		t.Errorf("expected .rafa/plans in task commit, got:\n%s", outputStr)
	}
}

// TestRun_UsesAgentSuggestedMessage verifies that the executor uses the
// agent's SUGGESTED_COMMIT_MESSAGE when available.
func TestRun_UsesAgentSuggestedMessage(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that outputs a suggested commit message
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Create implementation file
		implFile := filepath.Join(repoRoot, "feature.go")
		if err := os.WriteFile(implFile, []byte("package main\n"), 0644); err != nil {
			return err
		}

		// Write suggested commit message to output
		if output != nil {
			output.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: Add awesome new feature\n"))
		}
		return nil
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify the task commit message uses agent's suggestion
	cmd = exec.Command("git", "log", "-1", "--format=%s", "HEAD~1")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git log: %v", err)
	}
	commitMsg := strings.TrimSpace(string(output))

	if commitMsg != "Add awesome new feature" {
		t.Errorf("expected commit message 'Add awesome new feature', got: %s", commitMsg)
	}
}

// TestRun_DefaultMessageWhenNoSuggestion verifies that the executor falls back
// to a default task-based commit message when the agent doesn't suggest one.
func TestRun_DefaultMessageWhenNoSuggestion(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "t01", Title: "Implement login", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that doesn't output a suggested commit message
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Create implementation file without suggesting commit message
		implFile := filepath.Join(repoRoot, "login.go")
		return os.WriteFile(implFile, []byte("package main\n"), 0644)
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify the task commit message uses default format
	cmd = exec.Command("git", "log", "-1", "--format=%s", "HEAD~1")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get git log: %v", err)
	}
	commitMsg := strings.TrimSpace(string(output))

	expectedMsg := "[rafa] Complete task t01: Implement login"
	if commitMsg != expectedMsg {
		t.Errorf("expected commit message %q, got: %s", expectedMsg, commitMsg)
	}
}

// TestRun_TaskFailure_NothingCommitted verifies that when a task fails,
// no commit is made and the workspace remains dirty.
func TestRun_TaskFailure_NothingCommitted(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Get initial commit count
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	commitCountBefore := strings.TrimSpace(string(output))

	// Create executor with a runner that creates files then fails
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Create partial implementation file
		implFile := filepath.Join(repoRoot, "partial.go")
		if err := os.WriteFile(implFile, []byte("package main\n// incomplete\n"), 0644); err != nil {
			return err
		}
		return errors.New("task failed: tests not passing")
	})

	err := executor.Run(context.Background())

	// Should fail after max attempts
	if err == nil {
		t.Error("expected error after task failure")
	}

	// Verify no new commits were made
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot
	output, _ = cmd.Output()
	commitCountAfter := strings.TrimSpace(string(output))

	if commitCountBefore != commitCountAfter {
		t.Errorf("expected no new commits on failure, got %s before and %s after", commitCountBefore, commitCountAfter)
	}

	// Verify workspace is dirty (implementation file not committed)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ = cmd.Output()
	if !strings.Contains(string(output), "partial.go") {
		t.Errorf("expected partial.go in dirty files, got:\n%s", output)
	}
}

// TestRun_AgentAccidentallyCommits_Handled verifies that the executor
// handles the case where an agent accidentally commits its own changes.
func TestRun_AgentAccidentallyCommits_Handled(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that accidentally commits
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		// Simulate agent creating and committing files (which they shouldn't)
		implFile := filepath.Join(repoRoot, "impl.go")
		if err := os.WriteFile(implFile, []byte("package main\n"), 0644); err != nil {
			return err
		}

		cmd := exec.Command("git", "add", "-A")
		cmd.Dir = repoRoot
		if err := cmd.Run(); err != nil {
			return err
		}
		cmd = exec.Command("git", "commit", "-m", "Agent's accidental commit")
		cmd.Dir = repoRoot
		return cmd.Run()
	})

	err := executor.Run(context.Background())

	// Should succeed - executor handles agent commits gracefully
	if err != nil {
		t.Errorf("expected no error when agent commits, got: %v", err)
	}

	// Verify workspace is clean (executor committed remaining metadata)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("expected clean workspace after run, got dirty files:\n%s", output)
	}

	// Verify plan completed successfully
	if p.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", p.Status)
	}
}

// TestRun_AllowDirty_NoCommits verifies that with --allow-dirty flag,
// the executor does not make any commits.
func TestRun_AllowDirty_NoCommits(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Get initial commit count
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	commitCountBefore := strings.TrimSpace(string(output))

	// Create executor with allowDirty=true
	executor := New(planDir, p).WithAllowDirty(true)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		implFile := filepath.Join(repoRoot, "feature.go")
		return os.WriteFile(implFile, []byte("package main\n"), 0644)
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify no new commits were made
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoRoot
	output, _ = cmd.Output()
	commitCountAfter := strings.TrimSpace(string(output))

	if commitCountBefore != commitCountAfter {
		t.Errorf("expected no new commits with allowDirty, got %s before and %s after", commitCountBefore, commitCountAfter)
	}

	// Verify workspace is dirty (changes not committed)
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ = cmd.Output()
	if !strings.Contains(string(output), "feature.go") {
		t.Errorf("expected feature.go in dirty files with allowDirty, got:\n%s", output)
	}
}

// TestRun_WorkspaceCleanAfterSuccess verifies that after a successful plan
// execution, the workspace is clean (all changes committed).
func TestRun_WorkspaceCleanAfterSuccess(t *testing.T) {
	repoRoot, planDir := setupTestGitRepo(t)

	p := &plan.Plan{
		ID:          "test-plan-id",
		Name:        "Test Plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
			{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusPending},
			{ID: "task-3", Title: "Task 3", Status: plan.TaskStatusPending},
		},
	}
	if err := plan.SavePlan(planDir, p); err != nil {
		t.Fatalf("failed to save test plan: %v", err)
	}

	// Commit the initial plan.json so workspace is clean before Run
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoRoot
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add plan")
	cmd.Dir = repoRoot
	cmd.Run()

	// Create executor with a runner that creates different files for each task
	taskNum := 0
	executor := New(planDir, p)
	executor.runner = runnerFunc(func(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
		taskNum++
		implFile := filepath.Join(repoRoot, fmt.Sprintf("module_%d.go", taskNum))
		return os.WriteFile(implFile, []byte(fmt.Sprintf("package main\n// Module %d\n", taskNum)), 0644)
	})

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify workspace is clean after successful plan completion
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, _ := cmd.Output()
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("expected clean workspace after successful plan, got dirty files:\n%s", output)
	}

	// Verify plan completed
	if p.Status != plan.PlanStatusCompleted {
		t.Errorf("expected plan status completed, got: %s", p.Status)
	}

	// Verify all tasks completed
	for i, task := range p.Tasks {
		if task.Status != plan.TaskStatusCompleted {
			t.Errorf("task %d: expected status completed, got: %s", i, task.Status)
		}
	}
}
