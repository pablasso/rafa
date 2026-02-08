package views

import (
	"context"
	"errors"
	"regexp"
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

func TestRunningModel_Update_OutputLineMsg_JoinsChunkBoundaries(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(200, 40)

	var cmd tea.Cmd
	m, cmd = m.Update(OutputLineMsg{Line: "Let me verify"})
	if cmd == nil {
		t.Fatal("expected command from first output chunk")
	}
	m, cmd = m.Update(OutputLineMsg{Line: " everything builds and tests pass."})
	if cmd == nil {
		t.Fatal("expected command from second output chunk")
	}

	if m.output.LineCount() != 1 {
		t.Fatalf("expected 1 logical line after chunk merge, got %d", m.output.LineCount())
	}

	view := m.output.View()
	if !strings.Contains(view, "Let me verify everything builds and tests pass.") {
		t.Fatalf("expected merged phrase in output, got %q", view)
	}
}

func TestRunningModel_Update_OutputLineMsg_ToolMarkerChunkIgnored(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(200, 40)

	m, _ = m.Update(OutputLineMsg{Line: "\n[Tool: Bash]"})
	m, _ = m.Update(OutputLineMsg{Line: "Now let me verify"})

	view := m.output.View()
	if strings.Contains(view, "[Tool: Bash]") {
		t.Fatalf("expected tool marker to be filtered from output, got %q", view)
	}
	if !strings.Contains(view, "Now let me verify") {
		t.Fatalf("expected prose chunk to remain, got %q", view)
	}
}

func TestRunningModel_Update_OutputLineMsg_ToolUseDoesNotSplitContinuation(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(200, 40)

	m, _ = m.Update(OutputLineMsg{Line: "I'll start"})
	m, _ = m.Update(ToolUseMsg{ToolName: "Bash", ToolTarget: "make test"})
	m, _ = m.Update(OutputLineMsg{Line: " by understanding the codebase."})

	view := m.output.View()
	re := regexp.MustCompile(`I'll start by understanding the codebase\.`)
	if !re.MatchString(view) {
		t.Fatalf("expected continuation chunk to remain contiguous, got %q", view)
	}
}

func TestRunningModel_Update_OutputLineMsg_MarkerChunkSeparatesFollowingOutput(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(200, 40)

	m, _ = m.Update(OutputLineMsg{Line: "All checks pass."})
	m, _ = m.Update(OutputLineMsg{Line: "\n[Tool: Read]"})
	m, _ = m.Update(OutputLineMsg{Line: "Now let me inspect the file."})

	view := m.output.View()
	re := regexp.MustCompile(`All checks pass\.[^\n]*\n[^\n]*\nNow let me inspect the file\.`)
	if !re.MatchString(view) {
		t.Fatalf("expected newline separator after marker chunk, got %q", view)
	}
}

func TestRunningModel_Update_OutputLineMsg_AssistantBoundaryChunkSeparatesFollowingOutput(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(200, 40)

	m, _ = m.Update(OutputLineMsg{Line: "First assistant message."})
	m, _ = m.Update(OutputLineMsg{Line: executor.AssistantBoundaryChunk})
	m, _ = m.Update(OutputLineMsg{Line: "Second assistant message."})

	view := m.output.View()
	re := regexp.MustCompile(`First assistant message\.[^\n]*\n[^\n]*\nSecond assistant message\.`)
	if !re.MatchString(view) {
		t.Fatalf("expected blank-line separation across assistant boundary chunk, got %q", view)
	}
}

func TestRunningModel_Update_AssistantBoundarySeparatesFollowingOutput(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(200, 40)

	m, _ = m.Update(OutputLineMsg{Line: "First assistant message."})
	m, _ = m.Update(AssistantBoundaryMsg{})
	m, _ = m.Update(OutputLineMsg{Line: "Second assistant message."})

	view := m.output.View()
	re := regexp.MustCompile(`First assistant message\.[^\n]*\n[^\n]*\nSecond assistant message\.`)
	if !re.MatchString(view) {
		t.Fatalf("expected blank-line separation across assistant boundary, got %q", view)
	}
}

func TestRunningModel_Update_PlanDoneMsg_Success(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.activeToolCount = 1

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
	if newM.activeToolCount != 0 {
		t.Errorf("expected activeToolCount to be reset on completion, got %d", newM.activeToolCount)
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
	if newM.State() != stateCancelling {
		t.Errorf("expected state to be stateCancelling, got %d", newM.State())
	}
	if !cancelled {
		t.Error("expected cancel function to be called")
	}
	if !strings.Contains(newM.FinalMessage(), "Stopping") {
		t.Errorf("expected finalMessage to contain 'Stopping', got %s", newM.FinalMessage())
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

func TestRunningModel_Update_PlanCancelledMsg(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.tasks[0].Status = "completed"
	m.state = stateCancelling
	m.activeToolCount = 1

	newM, cmd := m.Update(PlanCancelledMsg{})

	if cmd != nil {
		t.Error("expected no command from PlanCancelledMsg")
	}
	if newM.State() != stateCancelled {
		t.Errorf("expected state to be stateCancelled, got %d", newM.State())
	}
	if !strings.Contains(newM.FinalMessage(), "Stopped") {
		t.Errorf("expected finalMessage to contain 'Stopped', got %s", newM.FinalMessage())
	}
	if newM.activeToolCount != 0 {
		t.Errorf("expected activeToolCount to be reset on cancellation, got %d", newM.activeToolCount)
	}
}

func TestRunningModel_Update_ExecutorStartedMsg_SetsCancel(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	cancel := func() {}
	newM, cmd := m.Update(ExecutorStartedMsg{Cancel: cancel})

	if cmd != nil {
		t.Error("expected no command from ExecutorStartedMsg")
	}
	if newM.cancel == nil {
		t.Error("expected cancel function to be set")
	}
}

func TestRunningModel_Update_ExecutorStartedMsg_CancelsWhenAlreadyCancelling(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.state = stateCancelling

	cancelled := false
	cancel := func() { cancelled = true }
	newM, _ := m.Update(ExecutorStartedMsg{Cancel: cancel})

	if !cancelled {
		t.Error("expected cancel function to be called immediately")
	}
	if newM.cancel != nil {
		t.Error("expected cancel function to be cleared after cancellation")
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

	// Check for left panel header with task info
	if !strings.Contains(view, "Task 2/3") {
		t.Error("expected view to contain 'Task 2/3'")
	}
	if !strings.Contains(view, "Attempt 1/5") {
		t.Error("expected view to contain 'Attempt 1/5'")
	}

	// Check for Activity section
	if !strings.Contains(view, "Activity") {
		t.Error("expected view to contain 'Activity' header")
	}

	// Check for Usage section
	if !strings.Contains(view, "Usage") {
		t.Error("expected view to contain 'Usage' header")
	}

	// Check for output panel
	if !strings.Contains(view, "Output") {
		t.Error("expected view to contain 'Output' header")
	}

	// Check for compact task list
	if !strings.Contains(view, "Tasks") {
		t.Error("expected view to contain 'Tasks' header")
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
	if newM.State() != stateCancelling {
		t.Errorf("expected state to be stateCancelling, got %d", newM.State())
	}
}

func TestRunningModel_CtrlCWithoutCancel_StaysCancelling(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if newM.State() != stateCancelling {
		t.Errorf("expected state to be stateCancelling, got %d", newM.State())
	}
	if !strings.Contains(newM.FinalMessage(), "Stopping") {
		t.Errorf("expected finalMessage to contain 'Stopping', got %s", newM.FinalMessage())
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

func TestIsToolMarkerLine(t *testing.T) {
	if !isToolMarkerLine("\n[Tool: Read]") {
		t.Fatalf("expected tool marker chunk to be detected")
	}
	if isToolMarkerLine("[Tool: Read]\nmore") {
		t.Fatalf("expected multi-line chunk to not be treated as pure marker")
	}
	if isToolMarkerLine("regular chunk") {
		t.Fatalf("expected regular chunk to not be treated as marker")
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

	// View should contain both Activity (left panel) and Output (right panel) sections
	if !strings.Contains(view, "Activity") {
		t.Error("expected view to contain Activity section")
	}
	if !strings.Contains(view, "Output") {
		t.Error("expected view to contain Output panel")
	}

	// Check that normal borders are used (lipgloss uses ┌ for top-left corner)
	if !strings.Contains(view, "┌") {
		t.Error("expected view to contain normal border character")
	}
}

func TestRunningModel_View_TwoPaneLeftColumn(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusInProgress},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(120, 40)
	m.currentTask = 2
	m.attempt = 1

	// Add an activity so it shows up in the Activity pane
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file.go"})

	view := m.View()

	// The view should contain separate Progress and Activity sections
	if !strings.Contains(view, "Usage") {
		t.Error("expected Progress pane to contain 'Usage' section")
	}
	if !strings.Contains(view, "Tasks") {
		t.Error("expected Progress pane to contain 'Tasks' section")
	}
	if !strings.Contains(view, "Activity") {
		t.Error("expected Activity pane with 'Activity' header")
	}
	if !strings.Contains(view, "Output") {
		t.Error("expected Output pane with 'Output' header")
	}
	// Activity pane should contain tool activity
	if !strings.Contains(view, "Read") {
		t.Error("expected Activity pane to contain 'Read' activity")
	}

	// Should have multiple bordered panes (at least 2 top-left corners for left column)
	cornerCount := strings.Count(view, "┌")
	if cornerCount < 3 {
		t.Errorf("expected at least 3 bordered panes (Progress, Activity, Output), got %d corners", cornerCount)
	}
}

func TestRunningModel_View_NarrowLayoutFallback(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	// Use a narrow width that triggers single-column fallback
	// minTwoColWidth = (24+4) + (36+4) = 68, so 50 should trigger narrow
	m.SetSize(50, 30)
	m.currentTask = 1
	m.attempt = 1

	view := m.View()

	// Should still contain all three sections
	if !strings.Contains(view, "Output") {
		t.Error("expected narrow layout to contain 'Output'")
	}
	if !strings.Contains(view, "Usage") {
		t.Error("expected narrow layout to contain 'Usage'")
	}
	if !strings.Contains(view, "Activity") {
		t.Error("expected narrow layout to contain 'Activity'")
	}
	// Should have bordered panes
	if !strings.Contains(view, "┌") {
		t.Error("expected narrow layout to contain bordered panes")
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

// Activity Timeline Tests

func TestRunningModel_Update_ToolUseMsg_AddsToActivityTimeline(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Send ToolUseMsg
	msg := ToolUseMsg{
		ToolName:   "Read",
		ToolTarget: "/path/to/file.go",
	}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from ToolUseMsg")
	}
	if len(newM.activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(newM.activities))
	}
	if !strings.Contains(newM.activities[0].Text, "Read") {
		t.Errorf("expected activity to contain tool name, got %s", newM.activities[0].Text)
	}
	if !strings.Contains(newM.activities[0].Text, "file.go") {
		t.Errorf("expected activity to contain file name, got %s", newM.activities[0].Text)
	}
	if newM.activities[0].IsDone {
		t.Error("expected activity to not be done initially")
	}
	if newM.activeToolCount != 1 {
		t.Errorf("expected activeToolCount to be 1, got %d", newM.activeToolCount)
	}
}

func TestRunningModel_Update_ToolUseMsg_MultipleToolUses(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Send multiple ToolUseMsgs
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file1.go"})
	m, _ = m.Update(ToolUseMsg{ToolName: "Edit", ToolTarget: "/file2.go"})
	m, _ = m.Update(ToolUseMsg{ToolName: "Bash", ToolTarget: "go test"})

	if len(m.activities) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(m.activities))
	}
	if !strings.Contains(m.activities[0].Text, "Read") {
		t.Errorf("expected first activity to be Read, got %s", m.activities[0].Text)
	}
	if !strings.Contains(m.activities[1].Text, "Edit") {
		t.Errorf("expected second activity to be Edit, got %s", m.activities[1].Text)
	}
	if !strings.Contains(m.activities[2].Text, "Bash") {
		t.Errorf("expected third activity to be Bash, got %s", m.activities[2].Text)
	}
}

func TestRunningModel_Update_ToolResultMsg_MarksLastActivityDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Add a tool use first
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file.go"})

	// Now send tool result
	newM, cmd := m.Update(ToolResultMsg{})

	if cmd != nil {
		t.Error("expected no command from ToolResultMsg")
	}
	if len(newM.activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(newM.activities))
	}
	if !newM.activities[0].IsDone {
		t.Error("expected activity to be marked done")
	}
	if newM.activeToolCount != 0 {
		t.Errorf("expected activeToolCount to be 0, got %d", newM.activeToolCount)
	}
}

func TestRunningModel_Update_ToolResultMsg_MarksOnlyLastActivityDone(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Add multiple tool uses
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file1.go"})
	m, _ = m.Update(ToolResultMsg{})
	m, _ = m.Update(ToolUseMsg{ToolName: "Edit", ToolTarget: "/file2.go"})

	// First activity should be done, second should not
	if !m.activities[0].IsDone {
		t.Error("expected first activity to be done")
	}
	if m.activities[1].IsDone {
		t.Error("expected second activity to not be done yet")
	}

	// Mark second as done
	m, _ = m.Update(ToolResultMsg{})
	if !m.activities[1].IsDone {
		t.Error("expected second activity to be done now")
	}
}

func TestRunningModel_Update_ToolResultMsg_NoActivities(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Send tool result without any activities (should not panic)
	newM, _ := m.Update(ToolResultMsg{})

	if len(newM.activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(newM.activities))
	}
	if newM.activeToolCount != 0 {
		t.Errorf("expected activeToolCount to remain 0, got %d", newM.activeToolCount)
	}
}

func TestRunningModel_View_OutputThinkingIndicator_ShownOnlyWhileToolRunning(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 30)
	m.currentTask = 1
	m.attempt = 1

	rightBefore := m.renderRightPanel(56, 20)
	if strings.Contains(rightBefore, m.spinner.View()) {
		t.Fatalf("did not expect spinner before tool use")
	}

	// Tool active: indicator should appear.
	m, _ = m.Update(OutputLineMsg{Line: "Now let me verify"})
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file.go"})
	rightDuring := m.renderRightPanel(56, 20)

	inlineSpinner := regexp.MustCompile(regexp.QuoteMeta("Now let me verify") + `[^\n]*\n[^\n]*\n` + regexp.QuoteMeta(m.spinner.View()))
	if !inlineSpinner.MatchString(rightDuring) {
		t.Fatalf("expected spinner after output text with a blank separator, got %q", rightDuring)
	}

	// Tool finished: indicator should disappear immediately.
	m, _ = m.Update(ToolResultMsg{})
	rightAfter := m.renderRightPanel(56, 20)
	if strings.Contains(rightAfter, m.spinner.View()) {
		t.Fatalf("did not expect spinner after tool result")
	}
}

func TestInsertInlineSpinner_AfterLastText(t *testing.T) {
	got := insertInlineSpinner("line one\n\n\n", "*")
	want := "line one\n\n*\n"
	if got != want {
		t.Fatalf("insertInlineSpinner() = %q, want %q", got, want)
	}
}

func TestInsertInlineSpinner_NoVisibleText(t *testing.T) {
	got := insertInlineSpinner("\n\n", "*")
	want := "*\n\n"
	if got != want {
		t.Fatalf("insertInlineSpinner() = %q, want %q", got, want)
	}
}

// Usage Extraction Tests

func TestRunningModel_Update_UsageMsg_ExtractsUsage(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	msg := UsageMsg{
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.05,
	}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from UsageMsg")
	}
	if newM.taskTokens != 1500 {
		t.Errorf("expected taskTokens to be 1500, got %d", newM.taskTokens)
	}
	if newM.totalTokens != 1500 {
		t.Errorf("expected totalTokens to be 1500, got %d", newM.totalTokens)
	}
	if newM.estimatedCost != 0.05 {
		t.Errorf("expected estimatedCost to be 0.05, got %f", newM.estimatedCost)
	}
}

func TestRunningModel_Update_UsageMsg_AccumulatesTotalTokens(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// First usage message
	m, _ = m.Update(UsageMsg{InputTokens: 1000, OutputTokens: 500, CostUSD: 0.05})
	// Second usage message
	m, _ = m.Update(UsageMsg{InputTokens: 2000, OutputTokens: 1000, CostUSD: 0.10})

	// taskTokens should be from the latest message
	if m.taskTokens != 3000 {
		t.Errorf("expected taskTokens to be 3000, got %d", m.taskTokens)
	}
	// totalTokens should accumulate
	if m.totalTokens != 4500 {
		t.Errorf("expected totalTokens to be 4500, got %d", m.totalTokens)
	}
	// cost should accumulate (use tolerance for floating point comparison)
	expectedCost := 0.15
	if m.estimatedCost < expectedCost-0.001 || m.estimatedCost > expectedCost+0.001 {
		t.Errorf("expected estimatedCost to be approximately 0.15, got %f", m.estimatedCost)
	}
}

func TestRunningModel_Update_UsageMsg_EstimatesCostWhenZero(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	msg := UsageMsg{
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0, // Zero cost should trigger estimation
	}

	newM, _ := m.Update(msg)

	// Should have estimated cost based on tokens
	expectedCost := estimateCost(1500)
	if newM.estimatedCost != expectedCost {
		t.Errorf("expected estimated cost %f, got %f", expectedCost, newM.estimatedCost)
	}
}

// formatTokens Tests

func TestFormatTokens_SmallNumbers(t *testing.T) {
	tests := []struct {
		tokens   int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{100, "100"},
		{999, "999"},
	}

	for _, tt := range tests {
		result := formatTokens(tt.tokens)
		if result != tt.expected {
			t.Errorf("formatTokens(%d) = %s, want %s", tt.tokens, result, tt.expected)
		}
	}
}

func TestFormatTokens_ThousandFormat(t *testing.T) {
	tests := []struct {
		tokens   int64
		expected string
	}{
		{1000, "1.0k"},
		{1500, "1.5k"},
		{12400, "12.4k"},
		{99999, "100.0k"},
		{100000, "100.0k"},
		{500000, "500.0k"},
		{999999, "1000.0k"},
	}

	for _, tt := range tests {
		result := formatTokens(tt.tokens)
		if result != tt.expected {
			t.Errorf("formatTokens(%d) = %s, want %s", tt.tokens, result, tt.expected)
		}
	}
}

func TestFormatTokens_MillionFormat(t *testing.T) {
	tests := []struct {
		tokens   int64
		expected string
	}{
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{12400000, "12.4M"},
	}

	for _, tt := range tests {
		result := formatTokens(tt.tokens)
		if result != tt.expected {
			t.Errorf("formatTokens(%d) = %s, want %s", tt.tokens, result, tt.expected)
		}
	}
}

// estimateCost Tests

func TestEstimateCost(t *testing.T) {
	// Test basic estimation
	cost := estimateCost(1000)
	if cost <= 0 {
		t.Errorf("expected positive cost, got %f", cost)
	}

	// More tokens should cost more
	cost1k := estimateCost(1000)
	cost10k := estimateCost(10000)
	if cost10k <= cost1k {
		t.Errorf("expected cost10k (%f) > cost1k (%f)", cost10k, cost1k)
	}

	// Zero tokens should be zero cost
	zeroCost := estimateCost(0)
	if zeroCost != 0 {
		t.Errorf("expected zero cost for zero tokens, got %f", zeroCost)
	}
}

// Left Panel Rendering Tests

func TestRunningModel_View_RendersActivityTimeline(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 30)
	m.currentTask = 1
	m.attempt = 1

	// Add some activities
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file.go"})
	m, _ = m.Update(ToolResultMsg{})
	m, _ = m.Update(ToolUseMsg{ToolName: "Edit", ToolTarget: "/file.go"})

	view := m.View()

	// Should contain Activity header
	if !strings.Contains(view, "Activity") {
		t.Error("expected view to contain 'Activity' header")
	}
	// Should contain tool names
	if !strings.Contains(view, "Read") {
		t.Error("expected view to contain 'Read' activity")
	}
	if !strings.Contains(view, "Edit") {
		t.Error("expected view to contain 'Edit' activity")
	}
}

func TestRunningModel_View_RendersUsageSection(t *testing.T) {
	tasks := []plan.Task{{ID: "t01", Title: "Task", Status: plan.TaskStatusPending}}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 30)
	m.currentTask = 1
	m.attempt = 1

	// Add some usage
	m, _ = m.Update(UsageMsg{InputTokens: 5000, OutputTokens: 2000, CostUSD: 0.10})

	view := m.View()

	// Should contain Usage header
	if !strings.Contains(view, "Usage") {
		t.Error("expected view to contain 'Usage' header")
	}
	// Should contain Task and Plan labels
	if !strings.Contains(view, "Task:") {
		t.Error("expected view to contain 'Task:' label")
	}
	if !strings.Contains(view, "Plan:") {
		t.Error("expected view to contain 'Plan:' label")
	}
	// Should contain Cost label
	if !strings.Contains(view, "Cost:") {
		t.Error("expected view to contain 'Cost:' label")
	}
	// Should contain formatted tokens
	if !strings.Contains(view, "7.0k") {
		t.Error("expected view to contain formatted token count '7.0k'")
	}
}

func TestRunningModel_View_RendersTaskList(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusInProgress},
		{ID: "t03", Title: "Task Three", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.SetSize(100, 30)
	m.currentTask = 2
	m.attempt = 1

	view := m.View()

	// Should contain Tasks header
	if !strings.Contains(view, "Tasks") {
		t.Error("expected view to contain 'Tasks' header")
	}
	// Should contain task titles in the list
	if !strings.Contains(view, "1. Task One") {
		t.Error("expected view to contain '1. Task One'")
	}
	if !strings.Contains(view, "2. Task Two") {
		t.Error("expected view to contain '2. Task Two'")
	}
	if !strings.Contains(view, "3. Task Three") {
		t.Error("expected view to contain '3. Task Three'")
	}
	// Should contain status indicators
	if !strings.Contains(view, "✓") {
		t.Error("expected view to contain checkmark for completed task")
	}
	if !strings.Contains(view, "▶") {
		t.Error("expected view to contain play indicator for running task")
	}
	if !strings.Contains(view, "○") {
		t.Error("expected view to contain circle for pending task")
	}
}

func TestRunningModel_TaskStartedMsg_ActivitiesPersistAcrossTasks(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Add some activities from first task
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file.go"})
	m, _ = m.Update(UsageMsg{InputTokens: 1000, OutputTokens: 500, CostUSD: 0.05})

	if len(m.activities) != 1 {
		t.Fatalf("expected 1 activity before TaskStartedMsg, got %d", len(m.activities))
	}
	if m.taskTokens != 1500 {
		t.Errorf("expected taskTokens to be 1500 before task start, got %d", m.taskTokens)
	}
	if m.activeToolCount != 1 {
		t.Errorf("expected activeToolCount to be 1 before task start, got %d", m.activeToolCount)
	}

	// Start second task
	m, _ = m.Update(TaskStartedMsg{TaskNum: 2, Total: 2, TaskID: "t02", Title: "Task Two", Attempt: 1})

	// Activities should NOT be cleared — they accumulate across the plan
	// There should be the original activity + the separator = 2 entries
	if len(m.activities) != 2 {
		t.Errorf("expected 2 activities (1 tool use + 1 separator), got %d", len(m.activities))
	}
	// First entry should be the original tool use
	if !strings.Contains(m.activities[0].Text, "Read") {
		t.Errorf("expected first activity to contain 'Read', got %s", m.activities[0].Text)
	}
	// Second entry should be the separator
	if !m.activities[1].IsSeparator {
		t.Error("expected second activity to be a separator")
	}
	if !strings.Contains(m.activities[1].Text, "Task 2/2") {
		t.Errorf("expected separator to contain 'Task 2/2', got %s", m.activities[1].Text)
	}
	if !strings.Contains(m.activities[1].Text, "Task Two") {
		t.Errorf("expected separator to contain 'Task Two', got %s", m.activities[1].Text)
	}
	if !strings.Contains(m.activities[1].Text, "Attempt 1/") {
		t.Errorf("expected separator to contain attempt info, got %s", m.activities[1].Text)
	}
	// taskTokens should be reset
	if m.taskTokens != 0 {
		t.Errorf("expected taskTokens to be 0 after task start, got %d", m.taskTokens)
	}
	if m.activeToolCount != 0 {
		t.Errorf("expected activeToolCount to be 0 after task start, got %d", m.activeToolCount)
	}
	// totalTokens should NOT be reset
	if m.totalTokens != 1500 {
		t.Errorf("expected totalTokens to remain 1500, got %d", m.totalTokens)
	}
}

func TestRunningModel_TaskStartedMsg_SeparatorContainsTaskAndAttemptInfo(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Implement auth", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Add tests", Status: plan.TaskStatusPending},
		{ID: "t03", Title: "Session management", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Start task 1
	m, _ = m.Update(TaskStartedMsg{TaskNum: 1, Total: 3, TaskID: "t01", Title: "Implement auth", Attempt: 1})
	m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/auth.go"})
	m, _ = m.Update(ToolResultMsg{})

	// Start task 2
	m, _ = m.Update(TaskStartedMsg{TaskNum: 2, Total: 3, TaskID: "t02", Title: "Add tests", Attempt: 1})
	m, _ = m.Update(ToolUseMsg{ToolName: "Edit", ToolTarget: "/auth_test.go"})

	// Start task 3, attempt 2
	m, _ = m.Update(TaskStartedMsg{TaskNum: 3, Total: 3, TaskID: "t03", Title: "Session management", Attempt: 2})

	// Should have: separator1, Read, separator2, Edit, separator3 = 5 entries
	if len(m.activities) != 5 {
		t.Fatalf("expected 5 activities, got %d", len(m.activities))
	}

	// Check separators
	sep1 := m.activities[0]
	if !sep1.IsSeparator {
		t.Error("expected first entry to be a separator")
	}
	if !strings.Contains(sep1.Text, "Task 1/3") || !strings.Contains(sep1.Text, "Implement auth") || !strings.Contains(sep1.Text, "Attempt 1/") {
		t.Errorf("separator 1 content unexpected: %s", sep1.Text)
	}

	sep2 := m.activities[2]
	if !sep2.IsSeparator {
		t.Error("expected third entry to be a separator")
	}
	if !strings.Contains(sep2.Text, "Task 2/3") || !strings.Contains(sep2.Text, "Add tests") {
		t.Errorf("separator 2 content unexpected: %s", sep2.Text)
	}

	sep3 := m.activities[4]
	if !sep3.IsSeparator {
		t.Error("expected fifth entry to be a separator")
	}
	if !strings.Contains(sep3.Text, "Task 3/3") || !strings.Contains(sep3.Text, "Session management") || !strings.Contains(sep3.Text, "Attempt 2/") {
		t.Errorf("separator 3 content unexpected: %s", sep3.Text)
	}

	// Non-separator entries should not have IsSeparator set
	if m.activities[1].IsSeparator {
		t.Error("expected Read activity to not be a separator")
	}
	if m.activities[3].IsSeparator {
		t.Error("expected Edit activity to not be a separator")
	}
}

func TestRunningModel_ActivityEntriesCappedAt2000(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)

	// Add more than maxActivityEntries activities
	for i := 0; i < 2010; i++ {
		m, _ = m.Update(ToolUseMsg{ToolName: "Read", ToolTarget: "/file.go"})
	}

	if len(m.activities) != 2000 {
		t.Errorf("expected activities to be capped at 2000, got %d", len(m.activities))
	}
}

// Activity Entry Type Tests

func TestRunActivityEntry_Structure(t *testing.T) {
	now := time.Now()
	entry := RunActivityEntry{
		Text:      "Read: file.go",
		Timestamp: now,
		IsDone:    true,
	}

	if entry.Text != "Read: file.go" {
		t.Errorf("expected Text to be 'Read: file.go', got %s", entry.Text)
	}
	if entry.Timestamp != now {
		t.Errorf("expected Timestamp to be %v, got %v", now, entry.Timestamp)
	}
	if !entry.IsDone {
		t.Error("expected IsDone to be true")
	}
}

// formatToolUseEntry Tests

func TestFormatToolUseEntry(t *testing.T) {
	tests := []struct {
		toolName string
		target   string
	}{
		{"Read", "/path/to/file.go"},
		{"Edit", "short.go"},
		{"Bash", "go test ./..."},
		{"Task", "Search codebase"},
		{"Read", ""},
	}

	for _, tt := range tests {
		result := formatToolUseEntry(tt.toolName, tt.target)
		// Should always start with tool name
		if !strings.HasPrefix(result, tt.toolName) {
			t.Errorf("formatToolUseEntry(%q, %q) = %q, expected to start with %q", tt.toolName, tt.target, result, tt.toolName)
		}
		// If target is non-empty, should contain ":"
		if tt.target != "" && !strings.Contains(result, ":") {
			t.Errorf("formatToolUseEntry(%q, %q) = %q, expected to contain ':'", tt.toolName, tt.target, result)
		}
		// If target is empty, should just be tool name
		if tt.target == "" && result != tt.toolName {
			t.Errorf("formatToolUseEntry(%q, %q) = %q, expected %q", tt.toolName, tt.target, result, tt.toolName)
		}
	}
}

// shortenPathForActivity Tests

func TestShortenPathForActivity(t *testing.T) {
	tests := []struct {
		path   string
		maxLen int
	}{
		{"short.go", 20},
		{"/very/long/path/to/file.go", 20},
		{"/a/b/c.go", 25},
		{"toolongtofitanywayatall", 10},
	}

	for _, tt := range tests {
		result := shortenPathForActivity(tt.path, tt.maxLen)
		// Result should not exceed maxLen
		if len(result) > tt.maxLen {
			t.Errorf("shortenPathForActivity(%q, %d) = %q (len=%d), exceeds maxLen", tt.path, tt.maxLen, result, len(result))
		}
		// If path is short enough, should be unchanged
		if len(tt.path) <= tt.maxLen && result != tt.path {
			t.Errorf("shortenPathForActivity(%q, %d) = %q, expected unchanged", tt.path, tt.maxLen, result)
		}
	}
}

// Message Type Tests

func TestToolUseMsg_Structure(t *testing.T) {
	msg := ToolUseMsg{
		ToolName:   "Read",
		ToolTarget: "/path/to/file.go",
	}

	if msg.ToolName != "Read" {
		t.Errorf("expected ToolName to be 'Read', got %s", msg.ToolName)
	}
	if msg.ToolTarget != "/path/to/file.go" {
		t.Errorf("expected ToolTarget to be '/path/to/file.go', got %s", msg.ToolTarget)
	}
}

func TestToolResultMsg_Structure(t *testing.T) {
	// ToolResultMsg is empty, just verify it can be created
	msg := ToolResultMsg{}
	_ = msg // Ensure it compiles
}

func TestUsageMsg_Structure(t *testing.T) {
	msg := UsageMsg{
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.05,
	}

	if msg.InputTokens != 1000 {
		t.Errorf("expected InputTokens to be 1000, got %d", msg.InputTokens)
	}
	if msg.OutputTokens != 500 {
		t.Errorf("expected OutputTokens to be 500, got %d", msg.OutputTokens)
	}
	if msg.CostUSD != 0.05 {
		t.Errorf("expected CostUSD to be 0.05, got %f", msg.CostUSD)
	}
}

// renderTaskList Tests

func TestRenderTaskList(t *testing.T) {
	tasks := []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusCompleted},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusInProgress},
		{ID: "t03", Title: "Task Three", Status: plan.TaskStatusFailed},
		{ID: "t04", Title: "Task Four", Status: plan.TaskStatusPending},
	}
	m := NewRunningModel("abc123", "my-plan", tasks, "", nil)
	m.currentTask = 2

	result := strings.Join(m.renderTaskList(50, 10), "\n")

	// Should contain checkmark for completed
	if !strings.Contains(result, "✓") {
		t.Error("expected task list to contain checkmark")
	}
	// Should contain play indicator for current running task
	if !strings.Contains(result, "▶") {
		t.Error("expected task list to contain play indicator")
	}
	// Should contain X for failed
	if !strings.Contains(result, "✗") {
		t.Error("expected task list to contain X for failed")
	}
	// Should contain circle for pending
	if !strings.Contains(result, "○") {
		t.Error("expected task list to contain circle for pending")
	}
}
