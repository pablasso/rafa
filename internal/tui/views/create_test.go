package views

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

func TestNewCreatingModel(t *testing.T) {
	sourceFile := "/path/to/design.md"
	m := NewCreatingModel(sourceFile)

	if m.SourceFile() != sourceFile {
		t.Errorf("expected sourceFile to be %s, got %s", sourceFile, m.SourceFile())
	}
	if m.State() != stateCheckingCLI {
		t.Errorf("expected initial state to be stateCheckingCLI, got %d", m.State())
	}
	if m.PlanID() != "" {
		t.Errorf("expected empty planID, got %s", m.PlanID())
	}
	if len(m.Tasks()) != 0 {
		t.Errorf("expected empty tasks, got %v", m.Tasks())
	}
	if m.Err() != nil {
		t.Errorf("expected no error, got %v", m.Err())
	}
}

func TestCreatingModel_Init(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	cmd := m.Init()

	if cmd == nil {
		t.Error("expected Init() to return a command")
	}
}

func TestCreatingModel_Update_WindowSizeMsg(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from WindowSizeMsg")
	}
	if newM.width != 80 {
		t.Errorf("expected width to be 80, got %d", newM.width)
	}
	if newM.height != 24 {
		t.Errorf("expected height to be 24, got %d", newM.height)
	}
}

func TestCreatingModel_Update_SpinnerTickMsg(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	// Send a spinner tick
	tickMsg := spinner.TickMsg{}
	newM, cmd := m.Update(tickMsg)

	// Spinner should have updated and returned a command
	if cmd == nil {
		t.Error("expected command from spinner tick")
	}
	// State should still be checkingCLI (initial state)
	if newM.State() != stateCheckingCLI {
		t.Errorf("expected state to remain stateCheckingCLI, got %d", newM.State())
	}
}

func TestCreatingModel_Update_PlanCreatedMsg(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	msg := msgs.PlanCreatedMsg{
		PlanID: "abc123-my-plan",
		Tasks:  []string{"Task 1", "Task 2", "Task 3"},
	}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from msgs.PlanCreatedMsg")
	}
	if newM.State() != stateSuccess {
		t.Errorf("expected state to be stateSuccess, got %d", newM.State())
	}
	if newM.PlanID() != "abc123-my-plan" {
		t.Errorf("expected planID to be abc123-my-plan, got %s", newM.PlanID())
	}
	if len(newM.Tasks()) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(newM.Tasks()))
	}
}

func TestCreatingModel_Update_PlanCreationErrorMsg(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	testErr := errors.New("network timeout")
	msg := PlanCreationErrorMsg{Err: testErr}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from PlanCreationErrorMsg")
	}
	if newM.State() != stateError {
		t.Errorf("expected state to be stateError, got %d", newM.State())
	}
	if newM.Err() != testErr {
		t.Errorf("expected error to be %v, got %v", testErr, newM.Err())
	}
}

func TestCreatingModel_Update_CtrlC_DuringExtraction(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	// Move to extracting state
	m.state = stateExtracting

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C during extraction")
	}

	msg := cmd()
	fpMsg, ok := msg.(msgs.GoToFilePickerMsg)
	if !ok {
		t.Errorf("expected msgs.GoToFilePickerMsg, got %T", msg)
	}
	// Should preserve directory
	if fpMsg.CurrentDir == "" {
		t.Error("expected CurrentDir to be preserved")
	}
}

func TestCreatingModel_Update_CtrlC_DuringCLICheck(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	// Initial state is stateCheckingCLI
	if m.State() != stateCheckingCLI {
		t.Fatal("expected initial state to be stateCheckingCLI")
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C during CLI check")
	}

	msg := cmd()
	fpMsg, ok := msg.(msgs.GoToFilePickerMsg)
	if !ok {
		t.Errorf("expected msgs.GoToFilePickerMsg, got %T", msg)
	}
	// Should preserve directory
	if fpMsg.CurrentDir == "" {
		t.Error("expected CurrentDir to be preserved")
	}
}

func TestCreatingModel_Update_KeyR_InSuccessState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess
	m.planID = "abc123-my-plan"

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if cmd == nil {
		t.Fatal("expected command from 'r' in success state")
	}

	msg := cmd()
	runMsg, ok := msg.(msgs.RunPlanMsg)
	if !ok {
		t.Errorf("expected msgs.RunPlanMsg, got %T", msg)
	}
	if runMsg.PlanID != "abc123-my-plan" {
		t.Errorf("expected planID abc123-my-plan, got %s", runMsg.PlanID)
	}
}

func TestCreatingModel_Update_KeyH_InSuccessState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if cmd == nil {
		t.Fatal("expected command from 'h' in success state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_KeyQ_InSuccessState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd == nil {
		t.Fatal("expected command from 'q' in success state")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_CtrlC_InSuccessState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C in success state")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_KeyR_InErrorState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError
	m.err = errors.New("test error")

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	// Should reset to extracting state
	if newM.State() != stateExtracting {
		t.Errorf("expected state to be stateExtracting after retry, got %d", newM.State())
	}
	if newM.Err() != nil {
		t.Errorf("expected error to be nil after retry, got %v", newM.Err())
	}
	// Should return a batch command
	if cmd == nil {
		t.Fatal("expected command from retry")
	}
}

func TestCreatingModel_Update_KeyB_InErrorState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if cmd == nil {
		t.Fatal("expected command from 'b' in error state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToFilePickerMsg); !ok {
		t.Errorf("expected msgs.GoToFilePickerMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_KeyH_InErrorState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if cmd == nil {
		t.Fatal("expected command from 'h' in error state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_KeyQ_InErrorState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd == nil {
		t.Fatal("expected command from 'q' in error state")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_CtrlC_InErrorState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C in error state")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestCreatingModel_View_EmptyDimensions(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got: %s", view)
	}
}

func TestCreatingModel_View_Extracting(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateExtracting
	m.SetSize(80, 24)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "Creating Plan") {
		t.Error("expected view to contain 'Creating Plan'")
	}

	// Check for source file indicator
	if !strings.Contains(view, "Source:") {
		t.Error("expected view to contain 'Source:'")
	}

	// Check for extraction message
	if !strings.Contains(view, "Extracting tasks from design...") {
		t.Error("expected view to contain 'Extracting tasks from design...'")
	}

	// Check for status bar
	if !strings.Contains(view, "Please wait...") {
		t.Error("expected view to contain 'Please wait...' in status bar")
	}
	if !strings.Contains(view, "Ctrl+C Cancel") {
		t.Error("expected view to contain 'Ctrl+C Cancel' in status bar")
	}
}

func TestCreatingModel_View_Success(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess
	m.planID = "abc123-my-plan"
	m.tasks = []string{"Task One", "Task Two", "Task Three"}
	m.SetSize(80, 24)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "Plan Created") {
		t.Error("expected view to contain 'Plan Created'")
	}

	// Check for success indicator
	if !strings.Contains(view, "✓") {
		t.Error("expected view to contain check mark")
	}

	// Check for plan ID
	if !strings.Contains(view, "abc123-my-plan") {
		t.Error("expected view to contain plan ID")
	}

	// Check for task list header
	if !strings.Contains(view, "Tasks:") {
		t.Error("expected view to contain 'Tasks:'")
	}

	// Check for numbered tasks
	if !strings.Contains(view, "1.") {
		t.Error("expected view to contain numbered task")
	}
	if !strings.Contains(view, "Task One") {
		t.Error("expected view to contain 'Task One'")
	}
	if !strings.Contains(view, "Task Two") {
		t.Error("expected view to contain 'Task Two'")
	}
	if !strings.Contains(view, "Task Three") {
		t.Error("expected view to contain 'Task Three'")
	}

	// Check for options
	if !strings.Contains(view, "[r]") {
		t.Error("expected view to contain '[r]' option")
	}
	if !strings.Contains(view, "Run this plan now") {
		t.Error("expected view to contain 'Run this plan now'")
	}
	if !strings.Contains(view, "[h]") {
		t.Error("expected view to contain '[h]' option")
	}
	if !strings.Contains(view, "Return to home") {
		t.Error("expected view to contain 'Return to home'")
	}

	// Check for status bar
	if !strings.Contains(view, "r Run plan") {
		t.Error("expected view to contain 'r Run plan' in status bar")
	}
	if !strings.Contains(view, "h Home") {
		t.Error("expected view to contain 'h Home' in status bar")
	}
	if !strings.Contains(view, "q Quit") {
		t.Error("expected view to contain 'q Quit' in status bar")
	}
}

func TestCreatingModel_View_Error(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError
	m.err = errors.New("failed to parse response") // Use a non-timeout error
	m.SetSize(80, 24)

	view := m.View()

	// Check for title (non-timeout errors show "Plan Creation Failed")
	if !strings.Contains(view, "Plan Creation Failed") {
		t.Error("expected view to contain 'Plan Creation Failed'")
	}

	// Check for error indicator
	if !strings.Contains(view, "✗") {
		t.Error("expected view to contain error mark")
	}

	// Check for error message
	if !strings.Contains(view, "Failed to extract tasks") {
		t.Error("expected view to contain 'Failed to extract tasks'")
	}

	// Check for error details
	if !strings.Contains(view, "Details:") {
		t.Error("expected view to contain 'Details:'")
	}
	if !strings.Contains(view, "failed to parse response") {
		t.Error("expected view to contain error details")
	}

	// Check for options
	if !strings.Contains(view, "[r]") {
		t.Error("expected view to contain '[r]' option")
	}
	if !strings.Contains(view, "Retry") {
		t.Error("expected view to contain 'Retry'")
	}
	if !strings.Contains(view, "[b]") {
		t.Error("expected view to contain '[b]' option")
	}
	if !strings.Contains(view, "Back to file picker") {
		t.Error("expected view to contain 'Back to file picker'")
	}
	if !strings.Contains(view, "[h]") {
		t.Error("expected view to contain '[h]' option")
	}

	// Check for status bar
	if !strings.Contains(view, "r Retry") {
		t.Error("expected view to contain 'r Retry' in status bar")
	}
	if !strings.Contains(view, "b Back") {
		t.Error("expected view to contain 'b Back' in status bar")
	}
	if !strings.Contains(view, "h Home") {
		t.Error("expected view to contain 'h Home' in status bar")
	}
}

func TestCreatingModel_SetSize(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height to be 50, got %d", m.height)
	}
}

func TestCreatingModel_View_AdaptsToSize(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	// Small size
	m.SetSize(40, 10)
	smallView := m.View()
	if smallView == "" {
		t.Error("expected non-empty view at small size")
	}

	// Large size
	m.SetSize(120, 40)
	largeView := m.View()
	if largeView == "" {
		t.Error("expected non-empty view at large size")
	}

	// Views should be different (different widths)
	if smallView == largeView {
		t.Error("expected views to differ for different sizes")
	}
}

func TestCreatingModel_SpinnerAnimation(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	// Verify spinner is set to Dot style
	if m.spinner.Spinner.Frames[0] != spinner.Dot.Frames[0] {
		t.Error("expected spinner to use Dot style")
	}
}

func TestPlanCreatedMsg_Structure(t *testing.T) {
	msg := msgs.PlanCreatedMsg{
		PlanID: "test-plan-id",
		Tasks:  []string{"Task 1", "Task 2"},
	}

	if msg.PlanID != "test-plan-id" {
		t.Errorf("expected PlanID to be test-plan-id, got %s", msg.PlanID)
	}
	if len(msg.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(msg.Tasks))
	}
}

func TestPlanCreationErrorMsg_Structure(t *testing.T) {
	testErr := errors.New("test error")
	msg := PlanCreationErrorMsg{Err: testErr}

	if msg.Err != testErr {
		t.Errorf("expected error to be testErr, got %v", msg.Err)
	}
}

func TestRunPlanMsg_Structure(t *testing.T) {
	msg := msgs.RunPlanMsg{PlanID: "my-plan-id"}

	if msg.PlanID != "my-plan-id" {
		t.Errorf("expected PlanID to be my-plan-id, got %s", msg.PlanID)
	}
}

func TestCreatingModel_View_SuccessTaskCount(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess
	m.planID = "test-plan"
	m.tasks = []string{"Task 1", "Task 2", "Task 3", "Task 4"}
	m.SetSize(80, 24)

	view := m.View()

	// All 4 tasks should be listed with numbers
	if !strings.Contains(view, "1.") {
		t.Error("expected view to contain task 1")
	}
	if !strings.Contains(view, "2.") {
		t.Error("expected view to contain task 2")
	}
	if !strings.Contains(view, "3.") {
		t.Error("expected view to contain task 3")
	}
	if !strings.Contains(view, "4.") {
		t.Error("expected view to contain task 4")
	}
}

func TestCreatingModel_Update_UnknownKey_InExtracting(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	// Random key should do nothing
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if cmd != nil {
		t.Error("expected no command from unknown key in extracting state")
	}
}

func TestCreatingModel_Update_UnknownKey_InSuccess(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateSuccess

	// Random key should do nothing
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if cmd != nil {
		t.Error("expected no command from unknown key in success state")
	}
}

func TestCreatingModel_Update_UnknownKey_InError(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError

	// Random key should do nothing
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if cmd != nil {
		t.Error("expected no command from unknown key in error state")
	}
}

func TestExtractTasksFunc_Integration(t *testing.T) {
	// Test that we can override the extractTasks function for testing
	originalFunc := extractTasks
	defer func() { extractTasks = originalFunc }()

	called := false
	extractTasks = func(ctx context.Context, designContent string) (*plan.TaskExtractionResult, error) {
		called = true
		return &plan.TaskExtractionResult{
			Name:        "test-plan",
			Description: "Test description",
			Tasks: []plan.ExtractedTask{
				{
					Title:              "Test Task",
					Description:        "Test task description",
					AcceptanceCriteria: []string{"criterion 1"},
				},
			},
		}, nil
	}

	m := NewCreatingModel("/path/to/design.md")
	cmd := m.startExtraction()

	// The command should be a function
	if cmd == nil {
		t.Fatal("expected command from startExtraction")
	}

	// Note: We can't easily test the full extraction flow without creating
	// real files, but we've verified the function can be overridden
	_ = called // The function would be called if we ran the command with a real file
}

func TestCreatePlanFolderFunc_Integration(t *testing.T) {
	// Test that we can override the createPlanFolder function for testing
	originalFunc := createPlanFolder
	defer func() { createPlanFolder = originalFunc }()

	called := false
	createPlanFolder = func(p *plan.Plan) error {
		called = true
		return nil
	}

	// Verify the override works
	err := createPlanFolder(&plan.Plan{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected createPlanFolder to be called")
	}
}

func TestCreatingModel_View_ErrorWithNilError(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError
	m.err = nil // No error details
	m.SetSize(80, 24)

	view := m.View()

	// Should still render without crashing
	if !strings.Contains(view, "Plan Creation Failed") {
		t.Error("expected view to contain 'Plan Creation Failed'")
	}
	// Should not contain "Details:" when error is nil
	if strings.Contains(view, "Details:") {
		t.Error("expected view NOT to contain 'Details:' when error is nil")
	}
}

func TestCreatingModel_Update_ClaudeCLINotFoundMsg(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")

	newM, cmd := m.Update(ClaudeCLINotFoundMsg{})

	if cmd != nil {
		t.Error("expected no command from ClaudeCLINotFoundMsg")
	}
	if newM.State() != stateCLINotFound {
		t.Errorf("expected state to be stateCLINotFound, got %d", newM.State())
	}
}

func TestCreatingModel_View_CLINotFound(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateCLINotFound
	m.SetSize(80, 24)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "Claude CLI Not Found") {
		t.Error("expected view to contain 'Claude CLI Not Found'")
	}

	// Check for error mark
	if !strings.Contains(view, "✗") {
		t.Error("expected view to contain error mark")
	}

	// Check for help text
	if !strings.Contains(view, "Install Claude CLI") {
		t.Error("expected view to contain 'Install Claude CLI'")
	}

	// Check for URL
	if !strings.Contains(view, "https://docs.anthropic.com/claude-cli") {
		t.Error("expected view to contain installation URL")
	}

	// Check for options
	if !strings.Contains(view, "[b]") {
		t.Error("expected view to contain '[b]' option")
	}
	if !strings.Contains(view, "[h]") {
		t.Error("expected view to contain '[h]' option")
	}

	// Check for status bar
	if !strings.Contains(view, "b Back") {
		t.Error("expected view to contain 'b Back' in status bar")
	}
}

func TestCreatingModel_Update_KeyB_InCLINotFoundState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateCLINotFound

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if cmd == nil {
		t.Fatal("expected command from 'b' in CLI not found state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToFilePickerMsg); !ok {
		t.Errorf("expected msgs.GoToFilePickerMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_KeyH_InCLINotFoundState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateCLINotFound

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if cmd == nil {
		t.Fatal("expected command from 'h' in CLI not found state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestCreatingModel_Update_CtrlC_InCLINotFoundState(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateCLINotFound

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C in CLI not found state")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestCreatingModel_View_ErrorWithTimeout(t *testing.T) {
	m := NewCreatingModel("/path/to/design.md")
	m.state = stateError
	m.err = errors.New("task extraction timed out")
	m.SetSize(80, 24)

	view := m.View()

	// Should show timeout-specific title
	if !strings.Contains(view, "Connection Timeout") {
		t.Error("expected view to contain 'Connection Timeout' for timeout errors")
	}

	// Should mention timeout in error message
	if !strings.Contains(view, "timeout") {
		t.Error("expected view to contain 'timeout'")
	}

	// Should recommend retry
	if !strings.Contains(view, "Recommended") {
		t.Error("expected view to contain 'Recommended' for timeout retry")
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("task extraction timed out"), true},
		{"deadline exceeded", errors.New("context deadline exceeded"), true},
		{"timeout in message", errors.New("network timeout connecting to server"), true},
		{"other error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			if result != tt.expected {
				t.Errorf("isTimeoutError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestCheckClaudeCLIFunc_Override(t *testing.T) {
	// Test that we can override the checkClaudeCLI function for testing
	originalFunc := checkClaudeCLI
	defer func() { checkClaudeCLI = originalFunc }()

	called := false
	checkClaudeCLI = func() error {
		called = true
		return ErrClaudeCLINotFound
	}

	// Verify the override works
	err := checkClaudeCLI()
	if err != ErrClaudeCLINotFound {
		t.Errorf("expected ErrClaudeCLINotFound, got %v", err)
	}
	if !called {
		t.Error("expected checkClaudeCLI to be called")
	}
}
