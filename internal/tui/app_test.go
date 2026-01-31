package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/session"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

func TestModel_View_TerminalTooSmall(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		expectSmall bool
	}{
		{
			name:        "exactly minimum size",
			width:       MinTerminalWidth,
			height:      MinTerminalHeight,
			expectSmall: false,
		},
		{
			name:        "width too small",
			width:       MinTerminalWidth - 1,
			height:      MinTerminalHeight,
			expectSmall: true,
		},
		{
			name:        "height too small",
			width:       MinTerminalWidth,
			height:      MinTerminalHeight - 1,
			expectSmall: true,
		},
		{
			name:        "both dimensions too small",
			width:       MinTerminalWidth - 10,
			height:      MinTerminalHeight - 5,
			expectSmall: true,
		},
		{
			name:        "larger than minimum",
			width:       100,
			height:      50,
			expectSmall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel()
			m.width = tt.width
			m.height = tt.height
			// Set size on the home view to avoid empty view
			m.home.SetSize(tt.width, tt.height)

			view := m.View()

			if tt.expectSmall {
				if !strings.Contains(view, "Terminal too small") {
					t.Error("expected view to contain 'Terminal too small'")
				}
				if !strings.Contains(view, "Minimum:") {
					t.Error("expected view to contain 'Minimum:'")
				}
				if !strings.Contains(view, "Current:") {
					t.Error("expected view to contain 'Current:'")
				}
			} else {
				if strings.Contains(view, "Terminal too small") {
					t.Error("did not expect view to contain 'Terminal too small'")
				}
			}
		})
	}
}

func TestModel_renderTerminalTooSmall_ShowsDimensions(t *testing.T) {
	m := initialModel()
	m.width = 50
	m.height = 10

	view := m.renderTerminalTooSmall()

	// Check that both minimum and current dimensions are shown
	if !strings.Contains(view, "60x15") {
		t.Error("expected minimum dimensions 60x15 to be shown")
	}
	if !strings.Contains(view, "50x10") {
		t.Error("expected current dimensions 50x10 to be shown")
	}
}

func TestModel_GoToConversationMsg_TransitionsToViewConversation(t *testing.T) {
	tests := []struct {
		name  string
		phase session.Phase
	}{
		{
			name:  "PRD phase",
			phase: session.PhasePRD,
		},
		{
			name:  "Design phase",
			phase: session.PhaseDesign,
		},
		{
			name:  "Plan create phase",
			phase: session.PhasePlanCreate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel()
			m.width = 100
			m.height = 50

			// Send GoToConversationMsg
			msg := msgs.GoToConversationMsg{Phase: tt.phase}
			updated, _ := m.Update(msg)
			updatedModel := updated.(Model)

			// Verify transition to ViewConversation
			if updatedModel.currentView != ViewConversation {
				t.Errorf("expected currentView to be ViewConversation, got %v", updatedModel.currentView)
			}

			// Verify conversation model was created with correct phase
			sess := updatedModel.conversation.Session()
			if sess == nil {
				t.Fatal("expected conversation session to be non-nil")
			}
			if sess.Phase != tt.phase {
				t.Errorf("expected session phase %v, got %v", tt.phase, sess.Phase)
			}
		})
	}
}

func TestModel_GoToFilePickerMsg_ForPlanCreation_StartsInDocsDesigns(t *testing.T) {
	// Create a temp directory structure with docs/designs/ containing .md files
	tmpDir := t.TempDir()
	designsDir := filepath.Join(tmpDir, "docs", "designs")
	err := os.MkdirAll(designsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create designs dir: %v", err)
	}

	// Create a .md file
	mdFile := filepath.Join(designsDir, "test-design.md")
	err = os.WriteFile(mdFile, []byte("# Test Design"), 0644)
	if err != nil {
		t.Fatalf("failed to create .md file: %v", err)
	}

	// Create .git directory so findRepoRoot works
	err = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	if err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	// Create model with the temp dir as repo root
	m := Model{
		currentView: ViewHome,
		repoRoot:    tmpDir,
		rafaDir:     filepath.Join(tmpDir, ".rafa"),
		width:       100,
		height:      50,
	}

	// Send GoToFilePickerMsg with ForPlanCreation=true
	msg := msgs.GoToFilePickerMsg{ForPlanCreation: true}
	updated, _ := m.Update(msg)
	updatedModel := updated.(Model)

	// Verify transition to ViewFilePicker
	if updatedModel.currentView != ViewFilePicker {
		t.Errorf("expected currentView to be ViewFilePicker, got %v", updatedModel.currentView)
	}

	// The file picker should have been initialized (we can't check the internal path directly,
	// but we can verify it was created)
}

func TestModel_GoToFilePickerMsg_ForPlanCreation_ShowsErrorWhenNoDesignDocs(t *testing.T) {
	// Create a temp directory structure WITHOUT docs/designs/ or .md files
	tmpDir := t.TempDir()

	// Create .git directory so findRepoRoot works
	err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	if err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	// Create .rafa directory so home view initializes properly
	rafaDir := filepath.Join(tmpDir, ".rafa")
	err = os.MkdirAll(rafaDir, 0755)
	if err != nil {
		t.Fatalf("failed to create .rafa dir: %v", err)
	}

	// Create model with the temp dir as repo root
	m := Model{
		currentView: ViewHome,
		repoRoot:    tmpDir,
		rafaDir:     rafaDir,
		width:       100,
		height:      50,
	}

	// Send GoToFilePickerMsg with ForPlanCreation=true
	msg := msgs.GoToFilePickerMsg{ForPlanCreation: true}
	updated, _ := m.Update(msg)
	updatedModel := updated.(Model)

	// Verify it returned to ViewHome (not ViewFilePicker)
	if updatedModel.currentView != ViewHome {
		t.Errorf("expected currentView to be ViewHome when no design docs, got %v", updatedModel.currentView)
	}

	// Verify error was set on home model
	homeError := updatedModel.home.Error()
	if homeError == "" {
		t.Error("expected error message to be set on home model")
	}
	if !strings.Contains(homeError, "No design documents found") {
		t.Errorf("expected error to mention no design documents, got: %s", homeError)
	}
}

func TestModel_WindowSizePropagatesTo_ConversationView(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 50

	// First transition to conversation view
	goToMsg := msgs.GoToConversationMsg{Phase: session.PhasePRD}
	updated, _ := m.Update(goToMsg)
	m = updated.(Model)

	// Now send a window size message
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 60}
	updated, _ = m.Update(windowMsg)
	m = updated.(Model)

	// Verify model's dimensions were updated
	if m.width != 120 {
		t.Errorf("expected model width 120, got %d", m.width)
	}
	if m.height != 60 {
		t.Errorf("expected model height 60, got %d", m.height)
	}

	// Verify conversation view is still the current view after size change
	if m.currentView != ViewConversation {
		t.Errorf("expected currentView to remain ViewConversation, got %v", m.currentView)
	}
}

func TestHasMarkdownFiles(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) error
		expected bool
	}{
		{
			name: "directory with .md files",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.md"), []byte("# Test"), 0644)
			},
			expected: true,
		},
		{
			name: "directory without .md files",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "test.txt"), []byte("text"), 0644)
			},
			expected: false,
		},
		{
			name:     "empty directory",
			setup:    func(dir string) error { return nil },
			expected: false,
		},
		{
			name: "non-existent directory",
			setup: func(dir string) error {
				// Remove the directory
				return os.RemoveAll(dir)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testDir := filepath.Join(tmpDir, "test")
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("failed to create test dir: %v", err)
			}

			if err := tt.setup(testDir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			result := hasMarkdownFiles(testDir)
			if result != tt.expected {
				t.Errorf("hasMarkdownFiles() = %v, want %v", result, tt.expected)
			}
		})
	}
}
