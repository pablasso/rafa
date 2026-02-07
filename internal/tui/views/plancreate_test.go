package views

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/ai"
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
	return &ai.Conversation{}, events, nil
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

func TestPlanCreateModel_Update_SavedMsgAutoRunsPlan(t *testing.T) {
	m := NewPlanCreateModel("design.md")
	updated, cmd := m.Update(PlanCreateSavedMsg{PlanID: "abc123-test-plan"})

	if cmd == nil {
		t.Fatal("expected command to auto-run saved plan")
	}
	if updated.State() != PlanCreateStateCompleted {
		t.Fatalf("expected completed state, got %d", updated.State())
	}
	if updated.IsThinking() {
		t.Fatal("expected extraction to stop after save")
	}
	select {
	case <-updated.ctx.Done():
		// expected: extraction context cancelled before auto-run
	default:
		t.Fatal("expected extraction context to be cancelled after save")
	}

	msg := cmd()
	runMsg, ok := msg.(msgs.RunPlanMsg)
	if !ok {
		t.Fatalf("expected RunPlanMsg, got %T", msg)
	}
	if runMsg.PlanID != "abc123-test-plan" {
		t.Fatalf("expected plan ID abc123-test-plan, got %s", runMsg.PlanID)
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
