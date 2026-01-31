package views

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/session"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

func TestNewHomeModel_WithExistingRafaDir(t *testing.T) {
	// Create a temp directory to simulate .rafa
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}

	m := NewHomeModel(rafaDir)

	if !m.RafaExists() {
		t.Error("expected RafaExists() to be true when .rafa directory exists")
	}
	if m.Cursor() != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.Cursor())
	}
	// 3 sections with Define(2), Execute(2), and Quit(1) = 5 total items
	totalItems := m.totalMenuItems()
	if totalItems != 5 {
		t.Errorf("expected 5 menu items, got %d", totalItems)
	}
}

func TestNewHomeModel_WithNonExistingRafaDir(t *testing.T) {
	m := NewHomeModel("/nonexistent/path/.rafa")

	if m.RafaExists() {
		t.Error("expected RafaExists() to be false when .rafa directory doesn't exist")
	}
}

func TestNewHomeModel_WithEmptyPath(t *testing.T) {
	m := NewHomeModel("")

	if m.RafaExists() {
		t.Error("expected RafaExists() to be false when path is empty")
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
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

	// Navigate down through all 5 items
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

	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 4 {
		t.Errorf("expected cursor to be 4 after fourth down, got %d", newM.cursor)
	}

	// Try to navigate past the end (5 items, cursor 4 is last)
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 4 {
		t.Errorf("expected cursor to stay at 4, got %d", newM.cursor)
	}
}

func TestHomeModel_Update_NavigateUp(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

	// Move cursor to the end (5 items, so cursor 4)
	m.cursor = 4

	// Navigate up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 3 {
		t.Errorf("expected cursor to be 3 after up, got %d", newM.cursor)
	}

	// Navigate up again
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to be 2 after second up, got %d", newM.cursor)
	}

	// Continue navigating up
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after third up, got %d", newM.cursor)
	}

	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to be 0 after fourth up, got %d", newM.cursor)
	}

	// Try to navigate past the beginning
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", newM.cursor)
	}
}

func TestHomeModel_Update_VimNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

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

func TestHomeModel_Update_ShortcutP(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if cmd == nil {
		t.Fatal("expected command from 'p' shortcut")
	}

	msg := cmd()
	convMsg, ok := msg.(msgs.GoToConversationMsg)
	if !ok {
		t.Fatalf("expected msgs.GoToConversationMsg, got %T", msg)
	}
	if convMsg.Phase != session.PhasePRD {
		t.Errorf("expected Phase to be PhasePRD, got %v", convMsg.Phase)
	}
}

func TestHomeModel_Update_ShortcutD(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if cmd == nil {
		t.Fatal("expected command from 'd' shortcut")
	}

	msg := cmd()
	convMsg, ok := msg.(msgs.GoToConversationMsg)
	if !ok {
		t.Fatalf("expected msgs.GoToConversationMsg, got %T", msg)
	}
	if convMsg.Phase != session.PhaseDesign {
		t.Errorf("expected Phase to be PhaseDesign, got %v", convMsg.Phase)
	}
}

func TestHomeModel_Update_ShortcutC(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

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
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

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
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd == nil {
		t.Fatal("expected command from 'q' shortcut")
	}

	// tea.Quit returns a special quit message
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHomeModel_Update_CtrlC(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHomeModel_Update_EnterOnCreatePRD(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.cursor = 0 // Create PRD

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter on Create PRD")
	}

	msg := cmd()
	convMsg, ok := msg.(msgs.GoToConversationMsg)
	if !ok {
		t.Fatalf("expected msgs.GoToConversationMsg, got %T", msg)
	}
	if convMsg.Phase != session.PhasePRD {
		t.Errorf("expected Phase to be PhasePRD, got %v", convMsg.Phase)
	}
}

func TestHomeModel_Update_EnterOnCreateDesign(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.cursor = 1 // Create Design Doc

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter on Create Design")
	}

	msg := cmd()
	convMsg, ok := msg.(msgs.GoToConversationMsg)
	if !ok {
		t.Fatalf("expected msgs.GoToConversationMsg, got %T", msg)
	}
	if convMsg.Phase != session.PhaseDesign {
		t.Errorf("expected Phase to be PhaseDesign, got %v", convMsg.Phase)
	}
}

func TestHomeModel_Update_EnterOnCreatePlan(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.cursor = 2 // Create Plan

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter on Create Plan")
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

func TestHomeModel_Update_EnterOnRun(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.cursor = 3 // Run Plan

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter on Run")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToPlanListMsg); !ok {
		t.Errorf("expected msgs.GoToPlanListMsg, got %T", msg)
	}
}

func TestHomeModel_Update_EnterOnQuit(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.cursor = 4 // Quit

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter on Quit")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHomeModel_Update_NoRafaDir_OnlyQuitWorks(t *testing.T) {
	m := NewHomeModel("/nonexistent/.rafa")

	// 'c' should do nothing
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil {
		t.Error("expected no command from 'c' when .rafa doesn't exist")
	}

	// 'r' should do nothing
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Error("expected no command from 'r' when .rafa doesn't exist")
	}

	// arrow keys should do nothing
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", newM.cursor)
	}

	// 'q' should still work
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected command from 'q' even when .rafa doesn't exist")
	}
}

func TestHomeModel_View_EmptyDimensions(t *testing.T) {
	m := NewHomeModel("")

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got: %s", view)
	}
}

func TestHomeModel_View_Normal(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.SetSize(120, 30)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "R A F A") {
		t.Error("expected view to contain 'R A F A'")
	}

	// Check for tagline
	if !strings.Contains(view, "Task Loop Runner for AI") {
		t.Error("expected view to contain 'Task Loop Runner for AI'")
	}

	// Check for section headers
	if !strings.Contains(view, "Define") {
		t.Error("expected view to contain 'Define' section header")
	}
	if !strings.Contains(view, "Execute") {
		t.Error("expected view to contain 'Execute' section header")
	}

	// Check for menu items with new structure
	if !strings.Contains(view, "[p]") {
		t.Error("expected view to contain '[p]' shortcut")
	}
	if !strings.Contains(view, "[d]") {
		t.Error("expected view to contain '[d]' shortcut")
	}
	if !strings.Contains(view, "[c]") {
		t.Error("expected view to contain '[c]' shortcut")
	}
	if !strings.Contains(view, "[r]") {
		t.Error("expected view to contain '[r]' shortcut")
	}
	if !strings.Contains(view, "[q]") {
		t.Error("expected view to contain '[q]' shortcut")
	}

	// Check for menu item labels
	if !strings.Contains(view, "Create PRD") {
		t.Error("expected view to contain 'Create PRD'")
	}
	if !strings.Contains(view, "Create Design Doc") {
		t.Error("expected view to contain 'Create Design Doc'")
	}
	if !strings.Contains(view, "Create Plan") {
		t.Error("expected view to contain 'Create Plan'")
	}
	if !strings.Contains(view, "Run Plan") {
		t.Error("expected view to contain 'Run Plan'")
	}
	if !strings.Contains(view, "Quit") {
		t.Error("expected view to contain 'Quit'")
	}

	// Check for status bar items
	if !strings.Contains(view, "↑↓ Navigate") {
		t.Error("expected view to contain '↑↓ Navigate' in status bar")
	}
	if !strings.Contains(view, "Enter Select") {
		t.Error("expected view to contain 'Enter Select' in status bar")
	}
	if !strings.Contains(view, "q Quit") {
		t.Error("expected view to contain 'q Quit' in status bar")
	}
}

func TestHomeModel_View_HasDescriptions(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)
	m.SetSize(120, 30)

	view := m.View()

	// Check for descriptions
	if !strings.Contains(view, "Start a new product requirements document") {
		t.Error("expected view to contain PRD description")
	}
	if !strings.Contains(view, "Create a technical design from PRD") {
		t.Error("expected view to contain Design description")
	}
	if !strings.Contains(view, "Generate execution plan from design") {
		t.Error("expected view to contain Plan description")
	}
	if !strings.Contains(view, "Execute an existing plan") {
		t.Error("expected view to contain Run description")
	}
}

func TestHomeModel_View_NoRafa(t *testing.T) {
	m := NewHomeModel("/nonexistent/.rafa")
	m.SetSize(80, 24)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "R A F A") {
		t.Error("expected view to contain 'R A F A'")
	}

	// Check for warning messages
	if !strings.Contains(view, "No .rafa/ directory found") {
		t.Error("expected view to contain 'No .rafa/ directory found'")
	}
	if !strings.Contains(view, "Run 'rafa init' first") {
		t.Error("expected view to contain 'Run 'rafa init' first'")
	}

	// Should NOT contain menu items
	if strings.Contains(view, "Create PRD") {
		t.Error("expected view NOT to contain 'Create PRD' when .rafa doesn't exist")
	}
	if strings.Contains(view, "Create Plan") {
		t.Error("expected view NOT to contain 'Create Plan' when .rafa doesn't exist")
	}

	// Status bar should only show quit
	if !strings.Contains(view, "q Quit") {
		t.Error("expected view to contain 'q Quit' in status bar")
	}
	// Should not show navigation in status bar
	if strings.Contains(view, "↑↓ Navigate") {
		t.Error("expected view NOT to contain '↑↓ Navigate' when .rafa doesn't exist")
	}
}

func TestHomeModel_SetSize(t *testing.T) {
	m := NewHomeModel("")
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height to be 50, got %d", m.height)
	}
}

func TestHomeModel_View_AdaptsToSize(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.Mkdir(rafaDir, 0755); err != nil {
		t.Fatalf("failed to create test .rafa dir: %v", err)
	}
	m := NewHomeModel(rafaDir)

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

	// Views should be different (different padding/centering)
	if smallView == largeView {
		t.Error("expected views to differ for different sizes")
	}
}
