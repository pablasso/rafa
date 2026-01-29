package views

import (
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
