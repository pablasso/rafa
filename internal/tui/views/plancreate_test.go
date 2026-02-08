package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

type mockPlanCreateConversationStarter struct {
	calls      int
	lastConfig ai.ConversationConfig
	err        error
}

func (m *mockPlanCreateConversationStarter) Start(ctx context.Context, config ai.ConversationConfig) (*ai.Conversation, <-chan ai.StreamEvent, error) {
	m.calls++
	m.lastConfig = config
	if m.err != nil {
		return nil, nil, m.err
	}

	events := make(chan ai.StreamEvent)
	close(events)
	return nil, events, nil
}

func collectCmdMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	return collectMessage(cmd())
}

func collectMessage(msg tea.Msg) []tea.Msg {
	switch m := msg.(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, sub := range m {
			out = append(out, collectCmdMessages(sub)...)
		}
		return out
	default:
		return []tea.Msg{m}
	}
}

func TestPlanCreateModel_InitStartsExtraction(t *testing.T) {
	tmp := t.TempDir()
	designPath := filepath.Join(tmp, "design.md")
	if err := os.WriteFile(designPath, []byte("# Design"), 0o644); err != nil {
		t.Fatalf("failed to write design file: %v", err)
	}

	m := NewPlanCreateModel(designPath)
	starter := &mockPlanCreateConversationStarter{}
	m.SetConversationStarter(starter)

	if m.State() != PlanCreateStateExtracting {
		t.Fatalf("expected initial state extracting, got %d", m.State())
	}

	msgs := collectCmdMessages(m.Init())
	if starter.calls != 1 {
		t.Fatalf("expected extraction to start once in Init, got %d", starter.calls)
	}

	foundConversationStarted := false
	for _, msg := range msgs {
		if _, ok := msg.(PlanCreateConversationStartedMsg); ok {
			foundConversationStarted = true
			break
		}
	}
	if !foundConversationStarted {
		t.Fatal("expected Init command batch to include PlanCreateConversationStartedMsg")
	}
}

func TestPlanCreateModel_BuildExtractionPrompt_OneShot(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	prompt := m.buildExtractionPrompt("# Test")

	if strings.Contains(prompt, "USER INSTRUCTIONS:") {
		t.Fatal("prompt should not include user instructions section")
	}
	if strings.Contains(prompt, "Do not include the JSON until the user explicitly approves") {
		t.Fatal("prompt should not require explicit user approval")
	}
	if !strings.Contains(prompt, "PLAN_APPROVED_JSON:") {
		t.Fatal("prompt should require PLAN_APPROVED_JSON marker")
	}
}

func TestPlanCreateModel_Update_SavedMsgStaysOnSuccessScreen(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	updated, cmd := m.Update(PlanCreateSavedMsg{PlanID: "abc123-test-plan"})

	if cmd != nil {
		t.Fatal("expected no command after plan save")
	}
	if updated.State() != PlanCreateStateCompleted {
		t.Fatalf("expected completed state, got %d", updated.State())
	}
	if updated.SavedPlanID() != "abc123-test-plan" {
		t.Fatalf("expected saved plan ID abc123-test-plan, got %s", updated.SavedPlanID())
	}
	if updated.IsThinking() {
		t.Fatal("expected extraction to stop after save")
	}
	select {
	case <-updated.ctx.Done():
		// expected: extraction context cancelled after save
	default:
		t.Fatal("expected extraction context to be cancelled after save")
	}

	updated.SetSize(120, 30)
	view := updated.View()
	if !strings.Contains(view, "Plan created: abc123-test-plan") {
		t.Fatalf("expected success view to include created plan ID, got: %s", view)
	}
	if !strings.Contains(view, "You can run it anytime from Home > Run Plan.") {
		t.Fatalf("expected success view to include run-later guidance, got: %s", view)
	}
	if strings.Contains(view, "Starting execution...") {
		t.Fatalf("did not expect auto-run text in success view")
	}
	if !strings.Contains(view, "[Enter] Home") {
		t.Fatalf("expected success action bar to show Enter home option, got: %s", view)
	}
}

func TestPlanCreateModel_HandleKeyPress_RealCompletedEnterReturnsHome(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.state = PlanCreateStateCompleted
	m.mode = PlanCreateModeReal

	updated, cmd, handled := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	if !handled {
		t.Fatal("expected enter key to be handled in real completed state")
	}
	if updated.State() != PlanCreateStateCompleted {
		t.Fatalf("expected state to remain completed, got %d", updated.State())
	}
	if cmd == nil {
		t.Fatal("expected enter key to return home command in real completed state")
	}
	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Fatalf("expected GoToHomeMsg, got %T", msg)
	}
}

func TestPlanCreateModel_HandleKeyPress_RealCompletedIgnoresNonEnter(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "h", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}},
		{name: "m", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")}},
		{name: "q", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
		{name: "ctrl+c", msg: tea.KeyMsg{Type: tea.KeyCtrlC}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewPlanCreateModel("design.md")
			m.state = PlanCreateStateCompleted
			m.mode = PlanCreateModeReal

			updated, cmd, handled := m.handleKeyPress(tt.msg)
			if !handled {
				t.Fatal("expected key to be handled in real completed state")
			}
			if cmd != nil {
				t.Fatalf("expected no command for %s in real completed state", tt.name)
			}
			if updated.State() != PlanCreateStateCompleted {
				t.Fatalf("expected state to remain completed, got %d", updated.State())
			}
		})
	}
}

func TestPlanCreateModel_HandleStreamEvent_DoneWithoutValidJSONSetsError(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.state = PlanCreateStateExtracting
	m.isThinking = true

	cmd := m.handleStreamEvent(ai.StreamEvent{Type: "done"})
	if cmd != nil {
		t.Fatal("expected no command when done arrives without valid JSON")
	}
	if m.State() != PlanCreateStateError {
		t.Fatalf("expected error state, got %d", m.State())
	}
	if !strings.Contains(m.errorMsg, "valid plan JSON") {
		t.Fatalf("expected error message to mention valid plan JSON, got %q", m.errorMsg)
	}
}

func TestPlanCreateModel_HandleKeyPress_RetryRestartsExtraction(t *testing.T) {
	tmp := t.TempDir()
	designPath := filepath.Join(tmp, "design.md")
	if err := os.WriteFile(designPath, []byte("# Design"), 0o644); err != nil {
		t.Fatalf("failed to write design file: %v", err)
	}

	m := NewPlanCreateModel(designPath)
	starter := &mockPlanCreateConversationStarter{}
	m.SetConversationStarter(starter)
	m.state = PlanCreateStateError
	m.errorMsg = "boom"
	m.isThinking = false
	m.responseText.WriteString("old content")

	retryKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	updated, cmd, handled := m.handleKeyPress(retryKey)
	if !handled {
		t.Fatal("expected retry key to be handled in error state")
	}
	if updated.State() != PlanCreateStateExtracting {
		t.Fatalf("expected extracting state after retry, got %d", updated.State())
	}
	if !updated.IsThinking() {
		t.Fatal("expected model to be thinking after retry")
	}
	if cmd == nil {
		t.Fatal("expected retry to return extraction command")
	}

	msg := cmd()
	if _, ok := msg.(PlanCreateConversationStartedMsg); !ok {
		t.Fatalf("expected PlanCreateConversationStartedMsg from retry, got %T", msg)
	}
	if starter.calls != 1 {
		t.Fatalf("expected one extraction start on retry, got %d", starter.calls)
	}
}

func TestPlanCreateModel_DemoUnsavedApprovedJSON_DoesNotAutoRun(t *testing.T) {
	m := NewPlanCreateModelForDemoUnsaved("docs/designs/plan-create-command.md", nil, "")
	m.state = PlanCreateStateExtracting
	m.isThinking = true

	text := "PLAN_APPROVED_JSON:\n{\n  \"name\": \"demo-plan\",\n  \"description\": \"demo\",\n  \"tasks\": [\n    {\n      \"title\": \"Task one\",\n      \"description\": \"desc\",\n      \"acceptanceCriteria\": [\"criterion\"]\n    }\n  ]\n}\n"
	cmd := m.handleStreamEvent(ai.StreamEvent{Type: "text", Text: text})
	if cmd != nil {
		t.Fatal("expected no command for demo-unsaved completion")
	}
	if m.State() != PlanCreateStateCompleted {
		t.Fatalf("expected completed state, got %d", m.State())
	}
	if m.SavedPlanID() != "" {
		t.Fatalf("expected empty saved plan ID in demo-unsaved mode, got %q", m.SavedPlanID())
	}
	if m.extractedPlan == nil {
		t.Fatal("expected extracted plan to be set")
	}

	m.SetSize(120, 30)
	view := m.View()
	if !strings.Contains(view, "Demo extraction complete") {
		t.Fatalf("expected demo completion message, got: %s", view)
	}
	if !strings.Contains(view, "[DEMO]") {
		t.Fatalf("expected demo status indicator in action bar, got: %s", view)
	}
	if strings.Contains(view, "Starting execution...") {
		t.Fatalf("did not expect auto-run text in demo-unsaved completion view")
	}
}

func TestPlanCreateModel_DemoUnsavedCompleted_RKeyReplays(t *testing.T) {
	m := NewPlanCreateModelForDemoUnsaved("missing-design.md", nil, "")
	starter := &mockPlanCreateConversationStarter{}
	m.SetConversationStarter(starter)
	m.state = PlanCreateStateCompleted
	result := demoExtractionResult()
	m.extractedPlan = &result

	retryKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	updated, cmd, handled := m.handleKeyPress(retryKey)
	if !handled {
		t.Fatal("expected replay key to be handled in demo completed state")
	}
	if updated.State() != PlanCreateStateExtracting {
		t.Fatalf("expected extracting state after replay, got %d", updated.State())
	}
	if cmd == nil {
		t.Fatal("expected replay command")
	}
	msg := cmd()
	if _, ok := msg.(PlanCreateConversationStartedMsg); !ok {
		t.Fatalf("expected PlanCreateConversationStartedMsg, got %T", msg)
	}
	if starter.calls != 1 {
		t.Fatalf("expected one starter call, got %d", starter.calls)
	}
}

func TestPlanCreateModel_DemoInitDoesNotRequireSourceFile(t *testing.T) {
	m := NewPlanCreateModelForDemoUnsaved("does/not/exist.md", nil, "")
	starter := &mockPlanCreateConversationStarter{}
	m.SetConversationStarter(starter)

	msgs := collectCmdMessages(m.Init())
	if starter.calls != 1 {
		t.Fatalf("expected extraction starter to run, got %d", starter.calls)
	}
	foundConversationStarted := false
	for _, msg := range msgs {
		if _, ok := msg.(PlanCreateConversationStartedMsg); ok {
			foundConversationStarted = true
			break
		}
	}
	if !foundConversationStarted {
		t.Fatal("expected PlanCreateConversationStartedMsg from demo init")
	}
}

// --- Scroll, Focus, and Scrollbar Tests ---

func TestPlanCreateModel_DefaultFocusIsResponse(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected default focus to be planCreateFocusResponse (%d), got %d", planCreateFocusResponse, m.Focus())
	}
}

func TestPlanCreateModel_TabTogglesFocus(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Default: focusResponse
	if m.Focus() != planCreateFocusResponse {
		t.Fatalf("expected initial focus to be planCreateFocusResponse, got %d", m.Focus())
	}

	// Tab → focusActivity
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus() != planCreateFocusActivity {
		t.Errorf("expected focus after first Tab to be planCreateFocusActivity (%d), got %d", planCreateFocusActivity, m.Focus())
	}

	// Tab → focusResponse (wraps around)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected focus after second Tab to be planCreateFocusResponse (%d), got %d", planCreateFocusResponse, m.Focus())
	}
}

func TestPlanCreateModel_TabDoesNotToggleAfterCompleted(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.state = PlanCreateStateCompleted
	m.mode = PlanCreateModeReal

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Tab should have no effect in completed state
	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected focus to remain planCreateFocusResponse in completed state, got %d", m.Focus())
	}
}

func TestPlanCreateModel_ScrollKeysRouteToFocusedPane_Response(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add content to the response so scrolling has an effect
	for i := 0; i < 100; i++ {
		m.responseView.AddLine("Response line")
	}

	// Default focus is Response — scroll up should be handled
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Verify model is still valid (no panic, state unchanged, focus unchanged)
	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected focus to remain planCreateFocusResponse, got %d", m.Focus())
	}

	// Auto-scroll should be disabled after scrolling up
	if m.responseView.AutoScroll() {
		t.Error("expected responseView autoScroll to be disabled after scroll up")
	}
}

func TestPlanCreateModel_ScrollKeysRouteToFocusedPane_Activity(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add activity content
	for i := 0; i < 50; i++ {
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_use", ToolName: "Read", ToolTarget: "/file.go"})
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_result"})
	}

	// Switch focus to Activity
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.Focus() != planCreateFocusActivity {
		t.Fatalf("expected planCreateFocusActivity, got %d", m.Focus())
	}

	// Scroll up in activity should disable auto-scroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.activityView.AutoScroll() {
		t.Error("expected activityView autoScroll to be disabled after scroll up")
	}

	// End key ('G') should re-enable auto-scroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if !m.activityView.AutoScroll() {
		t.Error("expected activityView autoScroll to be re-enabled after G")
	}
}

func TestPlanCreateModel_ScrollKeysDoNotAffectUnfocusedPanes(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add lots of content to both panes
	for i := 0; i < 50; i++ {
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_use", ToolName: "Read", ToolTarget: "/file.go"})
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_result"})
		m.responseView.AddLine("Response line")
	}

	// Focus is on Response (default). Verify Activity autoScroll is still true
	if !m.activityView.AutoScroll() {
		t.Fatal("expected activityView autoScroll to be true before any scroll events")
	}

	// Scroll up in Response — should only affect Response, not Activity
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	if !m.activityView.AutoScroll() {
		t.Error("expected activityView autoScroll to remain true when scrolling in unfocused pane")
	}
	if m.responseView.AutoScroll() {
		t.Error("expected responseView autoScroll to be disabled after scrolling up in focused Response pane")
	}
}

func TestPlanCreateModel_ResponseScrollbarPresent(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	if !m.responseView.ShowScrollbar() {
		t.Error("expected response viewport to have scrollbar enabled")
	}
}

func TestPlanCreateModel_ActivityUsesScrollViewport(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add some activity entries
	m.addActivity("Task started", 0)
	m.addActivity("Using Read: /file.go", 1)

	// The activity view should render content with a scrollbar column
	view := m.activityView.View()
	if view == "" {
		t.Error("expected activityView to render non-empty content")
	}
}

func TestPlanCreateModel_View_FocusedPaneBorderHighlight(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// With default focus on Response, the response pane should have focused styling
	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}

	// Verify status bar shows focus hint
	if !strings.Contains(view, "Focus: Response") {
		t.Errorf("expected status bar to show 'Focus: Response', got view that doesn't contain it")
	}

	// Switch focus to Activity
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	view = m.View()
	if !strings.Contains(view, "Focus: Activity") {
		t.Errorf("expected status bar to show 'Focus: Activity' after Tab")
	}
}

func TestPlanCreateModel_View_StatusBarShowsScrollHints(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)
	view := m.View()

	if !strings.Contains(view, "Tab Focus") {
		t.Error("expected status bar to show 'Tab Focus' hint")
	}
	if !strings.Contains(view, "Scroll") {
		t.Error("expected status bar to show scroll hint")
	}
}

func TestPlanCreateModel_MouseWheelRoutesToResponsePane(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add enough response content so scrolling has an effect
	for i := 0; i < 100; i++ {
		m.responseView.AddLine("Response line")
	}

	// Mouse wheel inside the Response pane bounds
	mouseX := m.boundsResponse.x + m.boundsResponse.w/2
	mouseY := m.boundsResponse.y + m.boundsResponse.h/2

	mouseMsg := tea.MouseMsg{
		X:      mouseX,
		Y:      mouseY,
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	}

	m, _ = m.Update(mouseMsg)

	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected focus to be planCreateFocusResponse after mouse wheel in Response, got %d", m.Focus())
	}
}

func TestPlanCreateModel_MouseWheelRoutesToActivityPane(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add activity content
	for i := 0; i < 50; i++ {
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_use", ToolName: "Read", ToolTarget: "/file.go"})
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_result"})
	}

	// Mouse wheel inside the Activity pane bounds
	mouseX := m.boundsActivity.x + m.boundsActivity.w/2
	mouseY := m.boundsActivity.y + m.boundsActivity.h/2

	mouseMsg := tea.MouseMsg{
		X:      mouseX,
		Y:      mouseY,
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	}

	m, _ = m.Update(mouseMsg)

	// Focus should be set to Activity
	if m.Focus() != planCreateFocusActivity {
		t.Errorf("expected focus to be planCreateFocusActivity after mouse wheel in Activity, got %d", m.Focus())
	}

	// Auto-scroll should be disabled after scrolling up
	if m.activityView.AutoScroll() {
		t.Error("expected activityView autoScroll to be disabled after wheel up")
	}
}

func TestPlanCreateModel_MouseWheelSetsFocusAndScrolls(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Start with focus on Response (default)
	if m.Focus() != planCreateFocusResponse {
		t.Fatalf("expected initial focus to be planCreateFocusResponse, got %d", m.Focus())
	}

	// Add enough activity for scrolling to matter
	for i := 0; i < 50; i++ {
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_use", ToolName: "Read", ToolTarget: "/file.go"})
		m.handleStreamEvent(ai.StreamEvent{Type: "tool_result"})
	}

	// Scroll in Activity pane — should switch focus from Response to Activity
	mouseMsg := tea.MouseMsg{
		X:      m.boundsActivity.x + 2,
		Y:      m.boundsActivity.y + 2,
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	}
	m, _ = m.Update(mouseMsg)

	if m.Focus() != planCreateFocusActivity {
		t.Errorf("expected focus to switch to planCreateFocusActivity, got %d", m.Focus())
	}
}

func TestPlanCreateModel_MouseWheelFallbackToFocusedPane(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Set focus to Activity
	m.focus = planCreateFocusActivity

	// Mouse wheel at coordinates outside all pane bounds (e.g., in title row)
	mouseMsg := tea.MouseMsg{
		X:      m.width / 2,
		Y:      0, // title row — outside all panes
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	}

	m, _ = m.Update(mouseMsg)

	// Should fall back to the currently focused pane (Activity)
	if m.Focus() != planCreateFocusActivity {
		t.Errorf("expected focus to remain planCreateFocusActivity on ambiguous coords, got %d", m.Focus())
	}
}

func TestPlanCreateModel_MouseWheelIgnoredAfterCompleted(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)
	m.state = PlanCreateStateCompleted
	m.mode = PlanCreateModeReal

	mouseMsg := tea.MouseMsg{
		X:      m.boundsActivity.x + 2,
		Y:      m.boundsActivity.y + 2,
		Button: tea.MouseButtonWheelUp,
		Action: tea.MouseActionPress,
	}

	m, _ = m.Update(mouseMsg)

	// Focus should not change in completed state
	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected focus to remain planCreateFocusResponse in completed state, got %d", m.Focus())
	}
}

func TestPlanCreateModel_MouseNonWheelIgnored(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	mouseMsg := tea.MouseMsg{
		X:      m.boundsActivity.x + 2,
		Y:      m.boundsActivity.y + 2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}

	m, _ = m.Update(mouseMsg)

	// Non-wheel events should not change focus
	if m.Focus() != planCreateFocusResponse {
		t.Errorf("expected focus to remain planCreateFocusResponse for non-wheel mouse event, got %d", m.Focus())
	}
}

func TestPlanCreateModel_ResponseAutoScrollFollowsBottomDuringStreaming(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Simulate streaming text that exceeds viewport
	for i := 0; i < 100; i++ {
		m.handleStreamEvent(ai.StreamEvent{Type: "text", Text: "Line of response text\n"})
	}

	// Auto-scroll should be true (following bottom during streaming)
	if !m.responseView.AutoScroll() {
		t.Error("expected responseView autoScroll to be true during streaming")
	}

	// Scroll up to pause auto-scroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.responseView.AutoScroll() {
		t.Error("expected responseView autoScroll to be paused after user scroll up")
	}
}

func TestPlanCreateModel_ActivityAutoScrollFollowsBottomAsEntriesArrive(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	m.SetSize(120, 30)

	// Add many activity entries
	for i := 0; i < 50; i++ {
		m.addActivity(fmt.Sprintf("Activity %d", i), 0)
	}

	// Auto-scroll should be true (following bottom)
	if !m.activityView.AutoScroll() {
		t.Error("expected activityView autoScroll to be true as new entries arrive")
	}

	// Switch focus to Activity and scroll up to pause
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.activityView.AutoScroll() {
		t.Error("expected activityView autoScroll to be paused after user scroll up")
	}
}

func TestPlanCreateFocusLabel(t *testing.T) {
	tests := []struct {
		focus planCreateFocus
		want  string
	}{
		{planCreateFocusResponse, "Response"},
		{planCreateFocusActivity, "Activity"},
		{planCreateFocus(99), "Response"}, // unknown defaults to Response
	}
	for _, tt := range tests {
		got := planCreateFocusLabel(tt.focus)
		if got != tt.want {
			t.Errorf("planCreateFocusLabel(%d) = %q, want %q", tt.focus, got, tt.want)
		}
	}
}

func demoExtractionResult() plan.TaskExtractionResult {
	return plan.TaskExtractionResult{
		Name:        "demo-plan",
		Description: "demo",
		Tasks: []plan.ExtractedTask{
			{
				Title:              "Task one",
				Description:        "desc",
				AcceptanceCriteria: []string{"criterion"},
			},
		},
	}
}
