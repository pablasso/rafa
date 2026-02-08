package views

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

func TestNewHomeModel_MenuItems(t *testing.T) {
	m := NewHomeModel("")

	if m.Cursor() != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.Cursor())
	}
	// Execute(2) + Quit(1) = 3 total items
	totalItems := m.totalMenuItems()
	if totalItems != 3 {
		t.Errorf("expected 3 menu items, got %d", totalItems)
	}
}

func TestNewHomeModel_MenuOrder_RunPlanFirst(t *testing.T) {
	m := NewHomeModel("")

	if len(m.sections) == 0 || len(m.sections[0].Items) < 2 {
		t.Fatalf("expected at least two menu items in first section")
	}

	first := m.sections[0].Items[0]
	second := m.sections[0].Items[1]

	if first.Label != "Run Plan" || first.Shortcut != "r" {
		t.Fatalf("expected first item to be Run Plan [r], got %s [%s]", first.Label, first.Shortcut)
	}
	if second.Label != "Create Plan" || second.Shortcut != "c" {
		t.Fatalf("expected second item to be Create Plan [c], got %s [%s]", second.Label, second.Shortcut)
	}
}

func TestHomeModel_Init(t *testing.T) {
	m := NewHomeModel("")
	cmd := m.Init()

	if cmd != nil {
		t.Error("expected Init() to return nil")
	}
}

func TestHomeModel_Update_WindowSizeMsg(t *testing.T) {
	m := NewHomeModel("")
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

func TestHomeModel_Update_NavigateDown(t *testing.T) {
	m := NewHomeModel("")

	// Navigate down through all 3 items
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after down, got %d", newM.cursor)
	}

	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to be 2 after second down, got %d", newM.cursor)
	}

	// Try to navigate past the end (3 items, cursor 2 is last)
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", newM.cursor)
	}
}

func TestHomeModel_Update_NavigateUp(t *testing.T) {
	m := NewHomeModel("")

	// Move cursor to the end (3 items, so cursor 2)
	m.cursor = 2

	// Navigate up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after up, got %d", newM.cursor)
	}

	// Navigate up again
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to be 0 after second up, got %d", newM.cursor)
	}

	// Try to navigate past the beginning
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", newM.cursor)
	}
}

func TestHomeModel_Update_VimNavigation(t *testing.T) {
	m := NewHomeModel("")

	// Navigate down with j
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after 'j', got %d", newM.cursor)
	}

	// Navigate up with k
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to be 0 after 'k', got %d", newM.cursor)
	}
}

func TestHomeModel_Update_ShortcutC(t *testing.T) {
	m := NewHomeModel("")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if cmd == nil {
		t.Fatal("expected command from 'c' shortcut")
	}

	msg := cmd()
	fpMsg, ok := msg.(msgs.GoToFilePickerMsg)
	if !ok {
		t.Fatalf("expected msgs.GoToFilePickerMsg, got %T", msg)
	}
	if !fpMsg.ForPlanCreation {
		t.Error("expected ForPlanCreation to be true")
	}
}

func TestHomeModel_Update_ShortcutR(t *testing.T) {
	m := NewHomeModel("")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if cmd == nil {
		t.Fatal("expected command from 'r' shortcut")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToPlanListMsg); !ok {
		t.Errorf("expected msgs.GoToPlanListMsg, got %T", msg)
	}
}

func TestHomeModel_Update_ShortcutQ(t *testing.T) {
	m := NewHomeModel("")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd == nil {
		t.Fatal("expected command from 'q' shortcut")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHomeModel_Update_CtrlC(t *testing.T) {
	m := NewHomeModel("")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHomeModel_View_NoSize(t *testing.T) {
	m := NewHomeModel("")
	if m.View() != "" {
		t.Error("expected empty view when width/height are 0")
	}
}

func TestHomeModel_View_RendersMenu(t *testing.T) {
	m := NewHomeModel("")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Task Loop Runner for Claude Code") {
		t.Errorf("expected view to contain updated subtitle, got: %s", view)
	}
	if !strings.Contains(view, "Create Plan") {
		t.Errorf("expected view to contain Create Plan, got: %s", view)
	}
	if !strings.Contains(view, "Run Plan") {
		t.Errorf("expected view to contain Run Plan, got: %s", view)
	}
}

func TestHomeModel_View_MenuDescriptionsAligned(t *testing.T) {
	m := NewHomeModel("")
	m.SetSize(100, 24)

	view := stripANSI(m.View())
	lines := strings.Split(view, "\n")

	runDescriptionStart := -1
	createDescriptionStart := -1

	for _, line := range lines {
		if strings.Contains(line, "Execute an existing plan") {
			runDescriptionStart = strings.Index(line, "Execute an existing plan")
		}
		if strings.Contains(line, "Generate execution plan from design") {
			createDescriptionStart = strings.Index(line, "Generate execution plan from design")
		}
	}

	if runDescriptionStart == -1 {
		t.Fatal("expected run plan description line in view")
	}
	if createDescriptionStart == -1 {
		t.Fatal("expected create plan description line in view")
	}
	if runDescriptionStart != createDescriptionStart {
		t.Fatalf("expected descriptions to start at same column, got run=%d create=%d", runDescriptionStart, createDescriptionStart)
	}
}

func stripANSI(s string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansi.ReplaceAllString(s, "")
}
