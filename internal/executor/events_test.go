package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// mockEvents implements ExecutorEvents for testing.
type mockEvents struct {
	taskStarts    []taskStartEvent
	taskCompletes []*plan.Task
	taskFails     []taskFailEvent
	planCompletes []planCompleteEvent
	planFails     []planFailEvent
	outputs       []string
}

type taskStartEvent struct {
	taskNum int
	total   int
	task    *plan.Task
	attempt int
}

type taskFailEvent struct {
	task    *plan.Task
	attempt int
	err     error
}

type planCompleteEvent struct {
	succeeded int
	total     int
	duration  time.Duration
}

type planFailEvent struct {
	task   *plan.Task
	reason string
}

func (m *mockEvents) OnTaskStart(taskNum, total int, task *plan.Task, attempt int) {
	m.taskStarts = append(m.taskStarts, taskStartEvent{taskNum, total, task, attempt})
}

func (m *mockEvents) OnTaskComplete(task *plan.Task) {
	m.taskCompletes = append(m.taskCompletes, task)
}

func (m *mockEvents) OnTaskFailed(task *plan.Task, attempt int, err error) {
	m.taskFails = append(m.taskFails, taskFailEvent{task, attempt, err})
}

func (m *mockEvents) OnOutput(line string) {
	m.outputs = append(m.outputs, line)
}

func (m *mockEvents) OnPlanComplete(succeeded, total int, duration time.Duration) {
	m.planCompletes = append(m.planCompletes, planCompleteEvent{succeeded, total, duration})
}

func (m *mockEvents) OnPlanFailed(task *plan.Task, reason string) {
	m.planFails = append(m.planFails, planFailEvent{task, reason})
}

func TestExecutor_WithEvents_EmitsOnTaskStart(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	events := &mockEvents{}
	runner := &mockRunner{Responses: []error{nil}}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true).
		WithEvents(events)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(events.taskStarts) != 1 {
		t.Fatalf("expected 1 OnTaskStart event, got: %d", len(events.taskStarts))
	}

	start := events.taskStarts[0]
	if start.taskNum != 1 {
		t.Errorf("expected taskNum=1, got: %d", start.taskNum)
	}
	if start.total != 1 {
		t.Errorf("expected total=1, got: %d", start.total)
	}
	if start.task.ID != "task-1" {
		t.Errorf("expected task ID='task-1', got: %s", start.task.ID)
	}
	if start.attempt != 1 {
		t.Errorf("expected attempt=1, got: %d", start.attempt)
	}
}

func TestExecutor_WithEvents_EmitsOnTaskComplete(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	events := &mockEvents{}
	runner := &mockRunner{Responses: []error{nil}}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true).
		WithEvents(events)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(events.taskCompletes) != 1 {
		t.Fatalf("expected 1 OnTaskComplete event, got: %d", len(events.taskCompletes))
	}

	if events.taskCompletes[0].ID != "task-1" {
		t.Errorf("expected task ID='task-1', got: %s", events.taskCompletes[0].ID)
	}
}

func TestExecutor_WithEvents_EmitsOnTaskFailed(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	events := &mockEvents{}
	// Fail twice, then succeed
	runner := &mockRunner{
		Responses: []error{
			errors.New("fail 1"),
			errors.New("fail 2"),
			nil,
		},
	}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true).
		WithEvents(events)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Should have 2 OnTaskFailed events (for the 2 failures)
	if len(events.taskFails) != 2 {
		t.Fatalf("expected 2 OnTaskFailed events, got: %d", len(events.taskFails))
	}

	if events.taskFails[0].attempt != 1 {
		t.Errorf("expected first failure attempt=1, got: %d", events.taskFails[0].attempt)
	}
	if events.taskFails[1].attempt != 2 {
		t.Errorf("expected second failure attempt=2, got: %d", events.taskFails[1].attempt)
	}
}

func TestExecutor_WithEvents_EmitsOnPlanComplete(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	events := &mockEvents{}
	runner := &mockRunner{Responses: []error{nil, nil}}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true).
		WithEvents(events)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if len(events.planCompletes) != 1 {
		t.Fatalf("expected 1 OnPlanComplete event, got: %d", len(events.planCompletes))
	}

	complete := events.planCompletes[0]
	if complete.succeeded != 2 {
		t.Errorf("expected succeeded=2, got: %d", complete.succeeded)
	}
	if complete.total != 2 {
		t.Errorf("expected total=2, got: %d", complete.total)
	}
	if complete.duration <= 0 {
		t.Errorf("expected positive duration, got: %v", complete.duration)
	}
}

func TestExecutor_WithEvents_EmitsOnPlanFailed(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	events := &mockEvents{}
	// Fail all attempts
	responses := make([]error, MaxAttempts)
	for i := range responses {
		responses[i] = errors.New("fail")
	}
	runner := &mockRunner{Responses: responses}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true).
		WithEvents(events)

	err := executor.Run(context.Background())

	if err == nil {
		t.Error("expected error after max attempts")
	}

	if len(events.planFails) != 1 {
		t.Fatalf("expected 1 OnPlanFailed event, got: %d", len(events.planFails))
	}

	fail := events.planFails[0]
	if fail.task.ID != "task-1" {
		t.Errorf("expected task ID='task-1', got: %s", fail.task.ID)
	}
	if fail.reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestExecutor_NilEvents_DoesNotCrash(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	// Don't set events - should be nil by default
	runner := &mockRunner{Responses: []error{nil}}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true)

	// This should not crash
	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestExecutor_NilEvents_TaskFailure_DoesNotCrash(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	// Don't set events and have tasks fail
	responses := make([]error, MaxAttempts)
	for i := range responses {
		responses[i] = errors.New("fail")
	}
	runner := &mockRunner{Responses: responses}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true)

	// This should not crash (even with nil events)
	err := executor.Run(context.Background())

	if err == nil {
		t.Error("expected error after max attempts")
	}
}

func TestExecutor_WithEvents_MultipleTasks(t *testing.T) {
	p := createTestPlan([]plan.Task{
		{ID: "task-1", Title: "Task 1", Status: plan.TaskStatusPending},
		{ID: "task-2", Title: "Task 2", Status: plan.TaskStatusPending},
		{ID: "task-3", Title: "Task 3", Status: plan.TaskStatusPending},
	})
	planDir := createTestPlanDir(t, p)

	events := &mockEvents{}
	runner := &mockRunner{Responses: []error{nil, nil, nil}}
	executor := New(planDir, p).
		WithRunner(runner).
		WithAllowDirty(true).
		WithEvents(events)

	err := executor.Run(context.Background())

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Should have 3 task start events
	if len(events.taskStarts) != 3 {
		t.Errorf("expected 3 OnTaskStart events, got: %d", len(events.taskStarts))
	}

	// Verify task numbers are correct
	for i, start := range events.taskStarts {
		expectedNum := i + 1
		if start.taskNum != expectedNum {
			t.Errorf("event %d: expected taskNum=%d, got: %d", i, expectedNum, start.taskNum)
		}
		if start.total != 3 {
			t.Errorf("event %d: expected total=3, got: %d", i, start.total)
		}
	}

	// Should have 3 task complete events
	if len(events.taskCompletes) != 3 {
		t.Errorf("expected 3 OnTaskComplete events, got: %d", len(events.taskCompletes))
	}

	// Should have 1 plan complete event
	if len(events.planCompletes) != 1 {
		t.Errorf("expected 1 OnPlanComplete event, got: %d", len(events.planCompletes))
	}
}
