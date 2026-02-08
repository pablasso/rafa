package views

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

func TestNewFilePickerModel(t *testing.T) {
	tmpDir := t.TempDir()

	m := NewFilePickerModel(tmpDir)

	if m.RepoRoot() != tmpDir {
		t.Errorf("expected repoRoot to be %s, got %s", tmpDir, m.RepoRoot())
	}
	if m.CurrentDirectory() != tmpDir {
		t.Errorf("expected CurrentDirectory to be %s, got %s", tmpDir, m.CurrentDirectory())
	}
	if m.Err() != nil {
		t.Errorf("expected no error, got %v", m.Err())
	}
}

func TestFilePickerModel_Init(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	cmd := m.Init()

	// filepicker.Init returns a command to read the directory
	if cmd == nil {
		t.Error("expected Init() to return a command")
	}
}

func TestFilePickerModel_Update_WindowSizeMsg(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)
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

func TestFilePickerModel_Update_EscapeReturnsGoToHomeMsg(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected command from Escape key")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestFilePickerModel_Update_CtrlCQuits(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestFilePickerModel_View_EmptyDimensions(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got: %s", view)
	}
}

func TestFilePickerModel_View_ContainsTitle(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Select Design Document") {
		t.Error("expected view to contain 'Select Design Document'")
	}
}

func TestFilePickerModel_View_ContainsStatusBar(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "↑↓ Navigate") {
		t.Error("expected view to contain '↑↓ Navigate' in status bar")
	}
	if !strings.Contains(view, "Enter Select") {
		t.Error("expected view to contain 'Enter Select' in status bar")
	}
	if !strings.Contains(view, "Esc Back") {
		t.Error("expected view to contain 'Esc Back' in status bar")
	}
}

func TestFilePickerModel_SetSize(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height to be 50, got %d", m.height)
	}
}

func TestFilePickerModel_View_AdaptsToSize(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

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

	// Views should be different (different widths affect status bar)
	if smallView == largeView {
		t.Error("expected views to differ for different sizes")
	}
}

func TestFilePickerModel_AllowedTypes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	mdFile := filepath.Join(tmpDir, "test.md")
	txtFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test.md: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test.txt: %v", err)
	}

	m := NewFilePickerModel(tmpDir)

	// The filepicker.AllowedTypes should be set to .md only
	// We can verify this by checking the picker's AllowedTypes field
	if len(m.picker.AllowedTypes) != 1 || m.picker.AllowedTypes[0] != ".md" {
		t.Errorf("expected AllowedTypes to be [.md], got %v", m.picker.AllowedTypes)
	}
}

func TestFilePickerModel_StartsInRepoRoot(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	m := NewFilePickerModel(tmpDir)

	// File picker should start in repo root
	if m.CurrentDirectory() != tmpDir {
		t.Errorf("expected current directory to be %s, got %s", tmpDir, m.CurrentDirectory())
	}
}

func TestFilePickerModel_ShowHiddenIsFalse(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	if m.picker.ShowHidden {
		t.Error("expected ShowHidden to be false")
	}
}

func TestFilePickerModel_DirAllowedIsFalse(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	if m.picker.DirAllowed {
		t.Error("expected DirAllowed to be false (only files can be selected)")
	}
}

func TestFilePickerModel_FileAllowedIsTrue(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewFilePickerModel(tmpDir)

	if !m.picker.FileAllowed {
		t.Error("expected FileAllowed to be true")
	}
}

func TestNewPlanFilePickerModel_CuratedViewShowsExpectedLocationAndGrouping(t *testing.T) {
	repoRoot, unplannedPath, plannedPath := setupPlanPickerFixture(t)
	m := NewPlanFilePickerModel(repoRoot)
	m.SetSize(100, 30)

	view := m.View()

	if !strings.Contains(view, "Expected location: docs/designs/") {
		t.Fatal("expected curated subtitle to mention docs/designs/")
	}
	if !strings.Contains(view, "No Plan Yet (1)") {
		t.Fatal("expected No Plan Yet section")
	}
	if !strings.Contains(view, "Already Has Plan (1)") {
		t.Fatal("expected Already Has Plan section")
	}
	if !strings.Contains(view, "already has 1 plan") {
		t.Fatal("expected planned document label to include plan count")
	}

	unplannedName := filepath.Base(unplannedPath)
	plannedName := filepath.Base(plannedPath)
	if strings.Index(view, unplannedName) > strings.Index(view, plannedName) {
		t.Fatalf("expected unplanned doc %q to appear before planned doc %q", unplannedName, plannedName)
	}
}

func TestPlanFilePickerModel_KeyBTogglesToBrowseAndDReturnsToCurated(t *testing.T) {
	repoRoot, _, _ := setupPlanPickerFixture(t)
	m := NewPlanFilePickerModel(repoRoot)
	m.SetSize(100, 30)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated
	if cmd == nil {
		t.Fatal("expected command when switching to browse mode")
	}

	// Process directory-read message to fully initialize browse view.
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated

	browseView := m.View()
	if !strings.Contains(browseView, "d Design Docs") {
		t.Fatal("expected browse mode status bar to contain d Design Docs")
	}
	if strings.Contains(browseView, "Expected location: docs/designs/") {
		t.Fatal("did not expect curated subtitle in browse mode")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated
	curatedView := m.View()

	if !strings.Contains(curatedView, "Expected location: docs/designs/") {
		t.Fatal("expected to return to curated mode after pressing d")
	}
	if !strings.Contains(curatedView, "b Browse") {
		t.Fatal("expected curated status bar to contain b Browse")
	}
}

func TestPlanFilePickerModel_EnterOnPlannedDocStillSelects(t *testing.T) {
	repoRoot, _, plannedPath := setupPlanPickerFixture(t)
	m := NewPlanFilePickerModel(repoRoot)
	m.SetSize(100, 30)

	// Move from first unplanned doc to first planned doc.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated
	if cmd == nil {
		t.Fatal("expected enter to return a selection command")
	}

	msg := cmd()
	fileMsg, ok := msg.(msgs.FileSelectedMsg)
	if !ok {
		t.Fatalf("expected msgs.FileSelectedMsg, got %T", msg)
	}
	if fileMsg.Path != plannedPath {
		t.Fatalf("expected planned path %q, got %q", plannedPath, fileMsg.Path)
	}
}

func TestPlanFilePickerModel_ShowsInlineWarningForPlannedSelection(t *testing.T) {
	repoRoot, _, _ := setupPlanPickerFixture(t)
	m := NewPlanFilePickerModel(repoRoot)
	m.SetSize(100, 30)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated
	view := m.View()

	if !strings.Contains(view, "already has a plan; selecting creates another") {
		t.Fatal("expected inline warning for planned document selection")
	}
}

func setupPlanPickerFixture(t *testing.T) (repoRoot, unplannedPath, plannedPath string) {
	t.Helper()

	repoRoot = t.TempDir()
	designsDir := filepath.Join(repoRoot, "docs", "designs")
	plansDir := filepath.Join(repoRoot, ".rafa", "plans")

	if err := os.MkdirAll(designsDir, 0o755); err != nil {
		t.Fatalf("failed to create designs dir: %v", err)
	}
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	unplannedPath = filepath.Join(designsDir, "alpha.md")
	plannedPath = filepath.Join(designsDir, "bravo.md")

	if err := os.WriteFile(unplannedPath, []byte("# Alpha"), 0o644); err != nil {
		t.Fatalf("failed to write unplanned doc: %v", err)
	}
	if err := os.WriteFile(plannedPath, []byte("# Bravo"), 0o644); err != nil {
		t.Fatalf("failed to write planned doc: %v", err)
	}

	planDir := filepath.Join(plansDir, "abc123-demo")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("failed to create plan dir: %v", err)
	}

	planPayload := map[string]any{
		"id":         "abc123",
		"name":       "demo",
		"sourceFile": "docs/designs/bravo.md",
	}
	planJSON, err := json.Marshal(planPayload)
	if err != nil {
		t.Fatalf("failed to marshal plan json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "plan.json"), planJSON, 0o644); err != nil {
		t.Fatalf("failed to write plan.json: %v", err)
	}

	return repoRoot, unplannedPath, plannedPath
}
