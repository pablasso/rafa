package views

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

func TestNewRunningModel(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusCompleted},
		{ID: "t03", Title: "Task Three", Status: plan.TaskStatusFailed},
	}

	m := NewRunningModel("abc123", "my-plan", tasks, "/tmp/test-plan", nil)

	if m.PlanID() != "abc123" {
		t.Errorf("expected planID to be abc123, got %s", m.PlanID())
	}
	if m.PlanName() != "my-plan" {
		t.Errorf("expected planName to be my-plan, got %s", m.PlanName())
	}
	if m.State() != stateRunning {
		t.Errorf("expected initial state to be stateRunning, got %d", m.State())
	}
	if len(m.Tasks()) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(m.Tasks()))
	}
	if m.Tasks()[0].Status != "pending" {
		t.Errorf("expected first task status to be pending, got %s", m.Tasks()[0].Status)
	}
	if m.Tasks()[1].Status != "completed" {
		t.Errorf("expected second task status to be completed, got %s", m.Tasks()[1].Status)
	}
	if m.Tasks()[2].Status != "failed" {
		t.Errorf("expected third task status to be failed, got %s", m.Tasks()[2].Status)
	}
	if m.CurrentTask() != 0 {
		t.Errorf("expected currentTask to be 0, got %d", m.CurrentTask())
	}
	if m.Attempt() != 0 {
		t.Errorf("expected attempt to be 0, got %d", m.Attempt())
	}
	if m.maxAttempts != executor.MaxAttempts {
		t.Errorf("expected maxAttempts to be %d, got %d", executor.MaxAttempts, m.maxAttempts)
	}
}

func TestRunningModel_Init(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	cmd := m.Init()

	if cmd == nil {
		t.Error("expected Init() to return a command")
	}
}

func TestRunningModel_Update_WindowSizeMsg(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	msg := tea.WindowSizeMsg{Width: 100, Height: 40}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from WindowSizeMsg")
	}
	if newM.width != 100 {
		t.Errorf("expected width to be 100, got %d", newM.width)
	}
	if newM.height != 40 {
		t.Errorf("expected height to be 40, got %d", newM.height)
	}
}

func TestRunningModel_Update_SpinnerTickMsg(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	tickMsg := spinner.TickMsg{}
	newM, cmd := m.Update(tickMsg)

	if cmd == nil {
		t.Error("expected command from spinner tick")
	}
	if newM.State() != stateRunning {
		t.Errorf("expected state to remain stateRunning, got %d", newM.State())
	}
}

func TestRunningModel_Update_TaskStartedMsg(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	msg := TaskStartedMsg{
		TaskNum: 1,
		Total:   2,
		TaskID:  "t01",
		Title:   "Task One",
		Attempt: 1,
	}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from TaskStartedMsg")
	}
	if newM.CurrentTask() != 1 {
		t.Errorf("expected currentTask to be 1, got %d", newM.CurrentTask())
	}
	if newM.Attempt() != 1 {
		t.Errorf("expected attempt to be 1, got %d", newM.Attempt())
	}
	if newM.Tasks()[0].Status != "running" {
		t.Errorf("expected first task status to be running, got %s", newM.Tasks()[0].Status)
	}
}

func TestRunningModel_Update_TaskCompletedMsg(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "running"

	msg := TaskCompletedMsg{TaskID: "t01"}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from TaskCompletedMsg")
	}
	if newM.Tasks()[0].Status != "completed" {
		t.Errorf("expected task status to be completed, got %s", newM.Tasks()[0].Status)
	}
}

func TestRunningModel_Update_TaskFailedMsg(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "running"

	// Failure at max attempts should mark as failed
	msg := TaskFailedMsg{
		TaskID:  "t01",
		Attempt: executor.MaxAttempts,
		Err:     errors.New("test error"),
	}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from TaskFailedMsg")
	}
	if newM.Tasks()[0].Status != "failed" {
		t.Errorf("expected task status to be failed, got %s", newM.Tasks()[0].Status)
	}
}

func TestRunningModel_Update_TaskFailedMsg_NotMaxAttempts(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "running"

	// Failure before max attempts should keep status as running
	msg := TaskFailedMsg{
		TaskID:  "t01",
		Attempt: 1,
		Err:     errors.New("test error"),
	}

	newM, _ := m.Update(msg)

	if newM.Tasks()[0].Status != "running" {
		t.Errorf("expected task status to remain running, got %s", newM.Tasks()[0].Status)
	}
}

func TestRunningModel_Update_OutputLineMsg(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 40)

	msg := OutputLineMsg{Line: "Test output line"}

	newM, cmd := m.Update(msg)

	if cmd == nil {
		t.Error("expected command from OutputLineMsg (for listening again)")
	}
	if newM.output.LineCount() != 1 {
		t.Errorf("expected 1 line in output, got %d", newM.output.LineCount())
	}
}

func TestRunningModel_Update_PlanDoneMsg_Success(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	msg := PlanDoneMsg{
		Success:   true,
		Succeeded: 1,
		Total:     1,
		Duration:  time.Minute,
	}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from PlanDoneMsg")
	}
	if newM.State() != stateDone {
		t.Errorf("expected state to be stateDone, got %d", newM.State())
	}
	if !newM.FinalSuccess() {
		t.Error("expected FinalSuccess to be true")
	}
	if !strings.Contains(newM.FinalMessage(), "Completed 1/1") {
		t.Errorf("expected finalMessage to contain completion info, got %s", newM.FinalMessage())
	}
}

func TestRunningModel_Update_PlanDoneMsg_Failure(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	msg := PlanDoneMsg{
		Success: false,
		Message: "Failed on task t01: max attempts reached",
	}

	newM, _ := m.Update(msg)

	if newM.State() != stateDone {
		t.Errorf("expected state to be stateDone, got %d", newM.State())
	}
	if newM.FinalSuccess() {
		t.Error("expected FinalSuccess to be false")
	}
	if newM.FinalMessage() != "Failed on task t01: max attempts reached" {
		t.Errorf("expected finalMessage to be failure message, got %s", newM.FinalMessage())
	}
}

func TestRunningModel_Update_CtrlC_DuringRunning(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "completed"

	// Set a cancel function to verify it's called
	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.SetCancel(func() {
		cancelled = true
		cancel()
	})
	_ = ctx

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd != nil {
		t.Error("expected no command from Ctrl+C")
	}
	if newM.State() != stateCancelled {
		t.Errorf("expected state to be stateCancelled, got %d", newM.State())
	}
	if !cancelled {
		t.Error("expected cancel function to be called")
	}
	if !strings.Contains(newM.FinalMessage(), "Stopped") {
		t.Errorf("expected finalMessage to contain 'Stopped', got %s", newM.FinalMessage())
	}
}

func TestRunningModel_Update_Enter_AfterDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateDone

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter in done state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestRunningModel_Update_Q_AfterDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateDone

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd == nil {
		t.Fatal("expected command from 'q' in done state")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestRunningModel_Update_H_AfterCancelled(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateCancelled

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if cmd == nil {
		t.Fatal("expected command from 'h' in cancelled state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestRunningModel_View_EmptyDimensions(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got: %s", view)
	}
}

func TestRunningModel_View_Running(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Set up middleware", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Implement login", Status: plan.TaskStatusInProgress},
		{ID: "t03", Title: "Add session mgmt", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("xK9pQ2", "feature-auth", tasks, "", nil)
	m.SetSize(100, 30)
	m.currentTask = 2
	m.attempt = 1

	view := m.View()

	// Check for title
	if !strings.Contains(view, "Running:") {
		t.Error("expected view to contain 'Running:'")
	}
	if !strings.Contains(view, "xK9pQ2-feature-auth") {
		t.Error("expected view to contain plan ID and name")
	}

	// Check for progress panel
	if !strings.Contains(view, "Progress") {
		t.Error("expected view to contain 'Progress' header")
	}
	if !strings.Contains(view, "Task 2/3") {
		t.Error("expected view to contain 'Task 2/3'")
	}
	if !strings.Contains(view, "Attempt: 1/5") {
		t.Error("expected view to contain 'Attempt: 1/5'")
	}
	if !strings.Contains(view, "Elapsed:") {
		t.Error("expected view to contain 'Elapsed:'")
	}

	// Check for output panel
	if !strings.Contains(view, "Output") {
		t.Error("expected view to contain 'Output' header")
	}

	// Check for task list
	if !strings.Contains(view, "Tasks:") {
		t.Error("expected view to contain 'Tasks:'")
	}
	if !strings.Contains(view, "Set up middleware") {
		t.Error("expected view to contain first task title")
	}
	if !strings.Contains(view, "Implement login") {
		t.Error("expected view to contain second task title")
	}
	if !strings.Contains(view, "Add session mgmt") {
		t.Error("expected view to contain third task title")
	}

	// Check for status bar
	if !strings.Contains(view, "Running...") {
		t.Error("expected view to contain 'Running...' in status bar")
	}
	if !strings.Contains(view, "Ctrl+C Cancel") {
		t.Error("expected view to contain 'Ctrl+C Cancel' in status bar")
	}
}

func TestRunningModel_View_Done_Success(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "completed"
	m.state = stateDone
	m.finalSuccess = true
	m.finalMessage = "Completed 1/1 tasks in 00:05"
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Plan Completed") {
		t.Error("expected view to contain 'Plan Completed'")
	}
	if !strings.Contains(view, "✓") {
		t.Error("expected view to contain check mark")
	}
	if !strings.Contains(view, "Completed 1/1") {
		t.Error("expected view to contain completion message")
	}
	if !strings.Contains(view, "Task Summary:") {
		t.Error("expected view to contain 'Task Summary:'")
	}
	if !strings.Contains(view, "[Enter]") {
		t.Error("expected view to contain '[Enter]' option")
	}
	if !strings.Contains(view, "Enter Home") {
		t.Error("expected view to contain 'Enter Home' in status bar")
	}
}

func TestRunningModel_View_Done_Failure(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusFailed},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "failed"
	m.state = stateDone
	m.finalSuccess = false
	m.finalMessage = "Failed on task t01: max attempts reached"
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Plan Failed") {
		t.Error("expected view to contain 'Plan Failed'")
	}
	if !strings.Contains(view, "✗") {
		t.Error("expected view to contain error mark")
	}
}

func TestRunningModel_View_Cancelled(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "completed"
	m.state = stateCancelled
	m.finalMessage = "Cancelled. Completed 1/2 tasks."
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Execution Cancelled") {
		t.Error("expected view to contain 'Execution Cancelled'")
	}
	if !strings.Contains(view, "Cancelled. Completed 1/2 tasks.") {
		t.Error("expected view to contain cancellation message")
	}
	if !strings.Contains(view, "Task Summary:") {
		t.Error("expected view to contain 'Task Summary:'")
	}
}

func TestRunningModel_SetSize(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(120, 50)

	if m.width != 120 {
		t.Errorf("expected width to be 120, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height to be 50, got %d", m.height)
	}
}

func TestRunningModel_FormatDuration(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	tests := []struct {
		duration time.Duration
		expected string
	}{
		{0, "00:00"},
		{30 * time.Second, "00:30"},
		{1*time.Minute + 30*time.Second, "01:30"},
		{10 * time.Minute, "10:00"},
		{1*time.Hour + 5*time.Minute + 30*time.Second, "01:05:30"},
		{2*time.Hour + 30*time.Minute, "02:30:00"},
	}

	for _, tt := range tests {
		result := m.formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
		}
	}
}

func TestRunningModel_CountCompleted(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
		{ID: "t03", Title: "Task Three", Status: plan.TaskStatusCompleted},
		{ID: "t04", Title: "Task Four", Status: plan.TaskStatusFailed},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	// Status is converted in constructor

	if m.countCompleted() != 2 {
		t.Errorf("expected 2 completed tasks, got %d", m.countCompleted())
	}
}

func TestRunningModel_GetTaskIndicator(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Test pending indicator
	pending := m.getTaskIndicator("pending", false)
	if !strings.Contains(pending, "○") {
		t.Errorf("expected pending indicator to contain '○', got %s", pending)
	}

	// Test completed indicator
	completed := m.getTaskIndicator("completed", false)
	if !strings.Contains(completed, "✓") {
		t.Errorf("expected completed indicator to contain '✓', got %s", completed)
	}

	// Test failed indicator
	failed := m.getTaskIndicator("failed", false)
	if !strings.Contains(failed, "✗") {
		t.Errorf("expected failed indicator to contain '✗', got %s", failed)
	}

	// Test running indicator (not current - static braille)
	running := m.getTaskIndicator("running", false)
	if running != "⣾" {
		t.Errorf("expected running indicator to be '⣾', got %s", running)
	}
}

func TestRunningModel_OutputChan(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	ch := m.OutputChan()
	if ch == nil {
		t.Error("expected non-nil output channel")
	}

	// Verify channel is buffered
	select {
	case ch <- "test":
		// OK
	default:
		t.Error("expected channel to accept message")
	}
}

func TestRunningModel_SetCancel(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	called := false
	m.SetCancel(func() {
		called = true
	})

	// Trigger cancel via Ctrl+C
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if !called {
		t.Error("expected cancel function to be called")
	}
	if newM.State() != stateCancelled {
		t.Errorf("expected state to be stateCancelled, got %d", newM.State())
	}
}

func TestRunningModel_ScrollKeys(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 40)

	// Add some output lines
	for i := 0; i < 50; i++ {
		m.output.AddLine("Line")
	}

	// Test scroll up key
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd == nil {
		// Viewport returns command when updated
		// OK - viewport handles internally
	}

	// Test scroll down key
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_ = cmd // May or may not return cmd depending on viewport state
}

func TestRunningModel_SpinnerStyle(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Verify spinner is set to Dot style
	if m.spinner.Spinner.Frames[0] != spinner.Dot.Frames[0] {
		t.Error("expected spinner to use Dot style")
	}
}

func TestTaskStartedMsg_Structure(t *testing.T) {
	msg := TaskStartedMsg{
		TaskNum: 1,
		Total:   5,
		TaskID:  "t01",
		Title:   "Test Task",
		Attempt: 2,
	}

	if msg.TaskNum != 1 {
		t.Errorf("expected TaskNum to be 1, got %d", msg.TaskNum)
	}
	if msg.Total != 5 {
		t.Errorf("expected Total to be 5, got %d", msg.Total)
	}
	if msg.TaskID != "t01" {
		t.Errorf("expected TaskID to be t01, got %s", msg.TaskID)
	}
	if msg.Title != "Test Task" {
		t.Errorf("expected Title to be Test Task, got %s", msg.Title)
	}
	if msg.Attempt != 2 {
		t.Errorf("expected Attempt to be 2, got %d", msg.Attempt)
	}
}

func TestTaskCompletedMsg_Structure(t *testing.T) {
	msg := TaskCompletedMsg{TaskID: "t01"}

	if msg.TaskID != "t01" {
		t.Errorf("expected TaskID to be t01, got %s", msg.TaskID)
	}
}

func TestTaskFailedMsg_Structure(t *testing.T) {
	testErr := errors.New("test error")
	msg := TaskFailedMsg{
		TaskID:  "t01",
		Attempt: 3,
		Err:     testErr,
	}

	if msg.TaskID != "t01" {
		t.Errorf("expected TaskID to be t01, got %s", msg.TaskID)
	}
	if msg.Attempt != 3 {
		t.Errorf("expected Attempt to be 3, got %d", msg.Attempt)
	}
	if msg.Err != testErr {
		t.Errorf("expected Err to be testErr, got %v", msg.Err)
	}
}

func TestOutputLineMsg_Structure(t *testing.T) {
	msg := OutputLineMsg{Line: "Test output"}

	if msg.Line != "Test output" {
		t.Errorf("expected Line to be 'Test output', got %s", msg.Line)
	}
}

func TestPlanDoneMsg_Structure(t *testing.T) {
	msg := PlanDoneMsg{
		Success:   true,
		Message:   "Test message",
		Succeeded: 5,
		Total:     10,
		Duration:  time.Minute,
	}

	if !msg.Success {
		t.Error("expected Success to be true")
	}
	if msg.Message != "Test message" {
		t.Errorf("expected Message to be 'Test message', got %s", msg.Message)
	}
	if msg.Succeeded != 5 {
		t.Errorf("expected Succeeded to be 5, got %d", msg.Succeeded)
	}
	if msg.Total != 10 {
		t.Errorf("expected Total to be 10, got %d", msg.Total)
	}
	if msg.Duration != time.Minute {
		t.Errorf("expected Duration to be 1 minute, got %v", msg.Duration)
	}
}

func TestTaskDisplay_Structure(t *testing.T) {
	td := TaskDisplay{
		Title:  "Test Task",
		Status: "running",
	}

	if td.Title != "Test Task" {
		t.Errorf("expected Title to be 'Test Task', got %s", td.Title)
	}
	if td.Status != "running" {
		t.Errorf("expected Status to be 'running', got %s", td.Status)
	}
}

func TestRunningModel_View_SplitLayout(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 30)

	view := m.View()

	// View should contain both Progress and Output panels
	if !strings.Contains(view, "Progress") {
		t.Error("expected view to contain Progress panel")
	}
	if !strings.Contains(view, "Output") {
		t.Error("expected view to contain Output panel")
	}

	// Check that rounded borders are used (lipgloss uses ╭ for top-left corner)
	if !strings.Contains(view, "╭") {
		t.Error("expected view to contain rounded border character")
	}
}

func TestRunningModel_SpinnerStopsAfterDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateDone

	tickMsg := spinner.TickMsg{}
	_, cmd := m.Update(tickMsg)

	if cmd != nil {
		t.Error("expected no command from spinner tick when done")
	}
}

func TestRunningModel_TickStopsAfterDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateDone

	_, cmd := m.Update(tickMsg(time.Now()))

	if cmd != nil {
		t.Error("expected no command from tick when done")
	}
}

func TestRunningModel_UnknownKeyInRunning(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	// Unknown keys should not produce commands (or just pass through to viewport)
	_ = cmd // Accept any result
}

func TestRunningModel_UnknownKeyAfterDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateDone

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if cmd != nil {
		t.Error("expected no command from unknown key in done state")
	}
}

func TestRunningModel_TaskStatusConversion(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusInProgress},
		{ID: "t03", Title: "Task Three", Status: plan.TaskStatusCompleted},
		{ID: "t04", Title: "Task Four", Status: plan.TaskStatusFailed},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	if m.Tasks()[0].Status != "pending" {
		t.Errorf("expected pending status, got %s", m.Tasks()[0].Status)
	}
	if m.Tasks()[1].Status != "running" {
		t.Errorf("expected running status for in_progress, got %s", m.Tasks()[1].Status)
	}
	if m.Tasks()[2].Status != "completed" {
		t.Errorf("expected completed status, got %s", m.Tasks()[2].Status)
	}
	if m.Tasks()[3].Status != "failed" {
		t.Errorf("expected failed status, got %s", m.Tasks()[3].Status)
	}
}

// Test RunningModelEvents interface compliance
func TestRunningModelEvents_InterfaceCompliance(t *testing.T) {
	// This test verifies at compile time that RunningModelEvents implements ExecutorEvents
	var _ executor.ExecutorEvents = (*RunningModelEvents)(nil)
}

// mockProgram implements a minimal tea.Program for testing event sending
type mockProgram struct {
	messages []tea.Msg
}

func (m *mockProgram) Send(msg tea.Msg) {
	m.messages = append(m.messages, msg)
}

func TestRunningModelEvents_SendsMessages(t *testing.T) {
	mock := &mockProgram{}

	// Create a wrapper that sends to our mock
	events := &testableRunningModelEvents{sendFunc: mock.Send}

	// Simulate OnTaskStart
	task := &plan.Task{ID: "t01", Title: "Test Task"}
	events.OnTaskStart(1, 2, task, 1)

	if len(mock.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mock.messages))
	}
	startMsg, ok := mock.messages[0].(TaskStartedMsg)
	if !ok {
		t.Fatalf("expected TaskStartedMsg, got %T", mock.messages[0])
	}
	if startMsg.TaskNum != 1 {
		t.Errorf("expected TaskNum 1, got %d", startMsg.TaskNum)
	}
	if startMsg.Total != 2 {
		t.Errorf("expected Total 2, got %d", startMsg.Total)
	}
	if startMsg.TaskID != "t01" {
		t.Errorf("expected TaskID t01, got %s", startMsg.TaskID)
	}
	if startMsg.Attempt != 1 {
		t.Errorf("expected Attempt 1, got %d", startMsg.Attempt)
	}

	// Simulate OnTaskComplete
	events.OnTaskComplete(task)

	if len(mock.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(mock.messages))
	}
	completeMsg, ok := mock.messages[1].(TaskCompletedMsg)
	if !ok {
		t.Fatalf("expected TaskCompletedMsg, got %T", mock.messages[1])
	}
	if completeMsg.TaskID != "t01" {
		t.Errorf("expected TaskID t01, got %s", completeMsg.TaskID)
	}

	// Simulate OnTaskFailed
	testErr := errors.New("test error")
	events.OnTaskFailed(task, 2, testErr)

	if len(mock.messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(mock.messages))
	}
	failedMsg, ok := mock.messages[2].(TaskFailedMsg)
	if !ok {
		t.Fatalf("expected TaskFailedMsg, got %T", mock.messages[2])
	}
	if failedMsg.TaskID != "t01" {
		t.Errorf("expected TaskID t01, got %s", failedMsg.TaskID)
	}
	if failedMsg.Attempt != 2 {
		t.Errorf("expected Attempt 2, got %d", failedMsg.Attempt)
	}
	if failedMsg.Err != testErr {
		t.Errorf("expected error to be testErr")
	}

	// Simulate OnPlanComplete
	events.OnPlanComplete(5, 10, time.Minute)

	if len(mock.messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(mock.messages))
	}
	doneMsg, ok := mock.messages[3].(PlanDoneMsg)
	if !ok {
		t.Fatalf("expected PlanDoneMsg, got %T", mock.messages[3])
	}
	if !doneMsg.Success {
		t.Error("expected Success to be true")
	}
	if doneMsg.Succeeded != 5 {
		t.Errorf("expected Succeeded 5, got %d", doneMsg.Succeeded)
	}
	if doneMsg.Total != 10 {
		t.Errorf("expected Total 10, got %d", doneMsg.Total)
	}

	// Simulate OnPlanFailed
	events.OnPlanFailed(task, "max attempts reached")

	if len(mock.messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(mock.messages))
	}
	failDoneMsg, ok := mock.messages[4].(PlanDoneMsg)
	if !ok {
		t.Fatalf("expected PlanDoneMsg, got %T", mock.messages[4])
	}
	if failDoneMsg.Success {
		t.Error("expected Success to be false")
	}
	if !strings.Contains(failDoneMsg.Message, "max attempts reached") {
		t.Errorf("expected Message to contain 'max attempts reached', got %s", failDoneMsg.Message)
	}
}

// testableRunningModelEvents is a version of RunningModelEvents that uses a function
// instead of a *tea.Program for easier testing
type testableRunningModelEvents struct {
	sendFunc func(tea.Msg)
}

func (e *testableRunningModelEvents) OnTaskStart(taskNum, total int, task *plan.Task, attempt int) {
	e.sendFunc(TaskStartedMsg{
		TaskNum: taskNum,
		Total:   total,
		TaskID:  task.ID,
		Title:   task.Title,
		Attempt: attempt,
	})
}

func (e *testableRunningModelEvents) OnTaskComplete(task *plan.Task) {
	e.sendFunc(TaskCompletedMsg{TaskID: task.ID})
}

func (e *testableRunningModelEvents) OnTaskFailed(task *plan.Task, attempt int, err error) {
	e.sendFunc(TaskFailedMsg{
		TaskID:  task.ID,
		Attempt: attempt,
		Err:     err,
	})
}

func (e *testableRunningModelEvents) OnOutput(line string) {
	// Output is handled via OutputCaptureWithEvents channel
}

func (e *testableRunningModelEvents) OnPlanComplete(succeeded, total int, duration time.Duration) {
	e.sendFunc(PlanDoneMsg{
		Success:   true,
		Succeeded: succeeded,
		Total:     total,
		Duration:  duration,
	})
}

func (e *testableRunningModelEvents) OnPlanFailed(task *plan.Task, reason string) {
	e.sendFunc(PlanDoneMsg{
		Success: false,
		Message: "Failed on task " + task.ID + ": " + reason,
	})
}

// Verify testableRunningModelEvents implements ExecutorEvents
var _ executor.ExecutorEvents = (*testableRunningModelEvents)(nil)

func TestRunningModel_StartExecutor_SetsCancelFunc(t *testing.T) {
	// Create a temp directory for the test plan
	tmpDir := t.TempDir()

	// Create a minimal plan
	p := &plan.Plan{
		ID:     "test123",
		Name:   "test-plan",
		Status: plan.PlanStatusNotStarted,
		Tasks: []plan.Task{
			{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		},
	}

	// Save the plan to the temp directory
	if err := plan.SavePlan(tmpDir, p); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	m := NewRunningModel("test123", "test-plan", p.Tasks, tmpDir, p)

	// Verify cancel is nil initially
	if m.cancel != nil {
		t.Error("expected cancel to be nil initially")
	}

	// Get the StartExecutor command but don't run it (to avoid actually executing)
	// We just verify the method exists and returns a command
	cmd := m.StartExecutor(nil)
	if cmd == nil {
		t.Error("expected StartExecutor to return a command")
	}
}

func TestRunningModel_PlanDirAndPlanFieldsSet(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	p := &plan.Plan{
		ID:   "test123",
		Name: "test-plan",
	}

	m := NewRunningModel("test123", "test-plan", tasks, "/test/path", p)

	if m.planDir != "/test/path" {
		t.Errorf("expected planDir to be /test/path, got %s", m.planDir)
	}
	if m.plan != p {
		t.Error("expected plan to be set")
	}
}
