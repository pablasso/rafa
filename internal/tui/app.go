package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// View represents the different screens in the TUI.
type View int

const (
	ViewHome View = iota
	ViewFilePicker
	ViewCreating
	ViewPlanList
	ViewRunning
)

// Model is the main Bubble Tea model that orchestrates all views.
type Model struct {
	currentView View
	width       int
	height      int

	// Sub-models for each view (to be implemented in later tasks)
	// home       homeModel
	// filePicker filePickerModel
	// creating   creatingModel
	// planList   planListModel
	// running    runningModel

	// Shared state
	repoRoot string
	rafaDir  string
	err      error
}

// Run starts the TUI application.
func Run() error {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

func initialModel() Model {
	m := Model{
		currentView: ViewHome,
	}

	// Detect repository root by looking for .git directory
	cwd, err := os.Getwd()
	if err != nil {
		m.err = err
		return m
	}

	m.repoRoot = findRepoRoot(cwd)
	if m.repoRoot != "" {
		m.rafaDir = filepath.Join(m.repoRoot, ".rafa")
	}

	return m
}

func findRepoRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	return "Rafa TUI - Press q to quit"
}
