package views

import (
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
	// Execute(3) + Quit(1) = 4 total items
	totalItems := m.totalMenuItems()
	if totalItems != 4 {
		t.Errorf("expected 4 menu items, got %d", totalItems)
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

	// Navigate down through all 4 items
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after down, got %d", newM.cursor)
	}

	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to be 2 after second down, got %d", newM.cursor)
	}

	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 3 {
		t.Errorf("expected cursor to be 3 after third down, got %d", newM.cursor)
	}

	// Try to navigate past the end (4 items, cursor 3 is last)
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 3 {
		t.Errorf("expected cursor to stay at 3, got %d", newM.cursor)
	}
}

func TestHomeModel_Update_NavigateUp(t *testing.T) {
	m := NewHomeModel("")

	// Move cursor to the end (4 items, so cursor 3)
	m.cursor = 3

	// Navigate up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to be 2 after up, got %d", newM.cursor)
	}

	// Navigate up again
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after second up, got %d", newM.cursor)
	}

	// Try to navigate past the beginning
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to be 0 after third up, got %d", newM.cursor)
	}

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

func TestHomeModel_Update_ShortcutD(t *testing.T) {
	m := NewHomeModel("")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if cmd == nil {
		t.Fatal("expected command from 'd' shortcut")
	}

	msg := cmd()
	if _, ok := msg.(msgs.RunDemoMsg); !ok {
		t.Errorf("expected msgs.RunDemoMsg, got %T", msg)
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
	if !strings.Contains(view, "Create Plan") {
		t.Errorf("expected view to contain Create Plan, got: %s", view)
	}
	if !strings.Contains(view, "Run Plan") {
		t.Errorf("expected view to contain Run Plan, got: %s", view)
	}
	if !strings.Contains(view, "Demo Mode") {
		t.Errorf("expected view to contain Demo Mode, got: %s", view)
	}
}
