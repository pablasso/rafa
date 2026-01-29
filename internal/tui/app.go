package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/demo"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/views"
)

// Minimum terminal size requirements
const (
	MinTerminalWidth  = 60
	MinTerminalHeight = 15
)

// Program is the Bubble Tea program instance, accessible for sending messages
// from background goroutines (e.g., executor events).
var Program *tea.Program

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

	// Sub-models for each view
	home       views.HomeModel
	filePicker views.FilePickerModel
	creating   views.CreatingModel
	planList   views.PlanListModel
	running    views.RunningModel

	// Shared state
	repoRoot string
	rafaDir  string
	err      error

	// Demo mode fields
	demoMode   bool
	demoConfig *demo.Config
}

// Option configures the TUI application.
type Option func(*Model)

// WithDemoMode enables demo mode with simulated execution.
func WithDemoMode(config *demo.Config) Option {
	return func(m *Model) {
		m.demoMode = true
		m.demoConfig = config
	}
}

// Run starts the TUI application with optional configuration.
func Run(opts ...Option) error {
	m := initialModel()
	for _, opt := range opts {
		opt(&m)
	}

	Program = tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := Program.Run()
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

	// Initialize the home view
	m.home = views.NewHomeModel(m.rafaDir)

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

// demoStartMsg is sent when we need to start demo mode after receiving window size.
type demoStartMsg struct{}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.demoMode {
		// In demo mode, we need window size before transitioning to running view.
		// Return a command that will trigger the transition after window size is received.
		return func() tea.Msg {
			return demoStartMsg{}
		}
	}
	return m.home.Init()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate to current view
		return m.propagateWindowSize(msg)

	case demoStartMsg:
		// Wait for window size to be set before transitioning
		if m.width > 0 && m.height > 0 {
			return m.transitionToDemoRunning()
		}
		// Window size not yet received, retry after a small delay to avoid busy-loop
		return m, tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg {
			return demoStartMsg{}
		})

	case tea.KeyMsg:
		// Global quit key only from home view
		if msg.String() == "q" && m.currentView == ViewHome {
			return m, tea.Quit
		}
		// Let the current view handle other keys
		return m.delegateToCurrentView(msg)

	// View transition messages
	case msgs.GoToHomeMsg:
		m.currentView = ViewHome
		m.home = views.NewHomeModel(m.rafaDir)
		m.home.SetSize(m.width, m.height)
		return m, m.home.Init()

	case msgs.GoToFilePickerMsg:
		m.currentView = ViewFilePicker
		startDir := m.repoRoot
		if msg.CurrentDir != "" {
			startDir = msg.CurrentDir
		}
		m.filePicker = views.NewFilePickerModel(startDir)
		m.filePicker.SetSize(m.width, m.height)
		return m, m.filePicker.Init()

	case msgs.FileSelectedMsg:
		m.currentView = ViewCreating
		m.creating = views.NewCreatingModel(msg.Path)
		m.creating.SetSize(m.width, m.height)
		return m, m.creating.Init()

	case msgs.GoToPlanListMsg:
		m.currentView = ViewPlanList
		m.planList = views.NewPlanListModel(m.rafaDir)
		m.planList.SetSize(m.width, m.height)
		return m, m.planList.Init()

	case msgs.RunPlanMsg:
		return m.transitionToRunning(msg.PlanID)

	case msgs.ExecutionDoneMsg:
		m.currentView = ViewHome
		m.home = views.NewHomeModel(m.rafaDir)
		m.home.SetSize(m.width, m.height)
		return m, m.home.Init()
	}

	// Delegate all other messages to the current view
	return m.delegateToCurrentView(msg)
}

// transitionToRunning loads the plan and transitions to the running view.
func (m Model) transitionToRunning(planID string) (tea.Model, tea.Cmd) {
	// planID format is "shortID-name", extract just shortID for directory lookup
	// But we also need to handle full directory names like "ABC123-my-plan"
	planDir := filepath.Join(m.rafaDir, "plans", planID)

	// Load the plan
	p, err := plan.LoadPlan(planDir)
	if err != nil {
		// If load fails, return to home with error (for now just go home)
		m.currentView = ViewHome
		m.home = views.NewHomeModel(m.rafaDir)
		m.home.SetSize(m.width, m.height)
		return m, m.home.Init()
	}

	// Extract the short ID and name for display
	// The planID passed in is already in the format "shortID-name"
	parts := strings.SplitN(planID, "-", 2)
	shortID := parts[0]
	planName := p.Name
	if len(parts) > 1 {
		planName = parts[1]
	}

	m.currentView = ViewRunning
	m.running = views.NewRunningModel(shortID, planName, p.Tasks, planDir, p)
	m.running.SetSize(m.width, m.height)

	// Start the executor in a background goroutine
	return m, tea.Batch(
		m.running.Init(),
		m.running.StartExecutor(Program),
	)
}

// transitionToDemoRunning creates an in-memory demo plan and transitions to the running view.
func (m Model) transitionToDemoRunning() (tea.Model, tea.Cmd) {
	// Create demo plan in memory with configured task count
	demoPlan := demo.CreateDemoPlanWithTaskCount(m.demoConfig.TaskCount)

	m.currentView = ViewRunning
	m.running = views.NewRunningModelWithDemo("DEMO", demoPlan.Name, demoPlan.Tasks, demoPlan, m.demoConfig)
	m.running.SetSize(m.width, m.height)

	// Start the executor in a background goroutine
	// StartExecutor detects demo mode and injects DemoRunner automatically
	return m, tea.Batch(
		m.running.Init(),
		m.running.StartExecutor(Program),
	)
}

// propagateWindowSize sends the window size message to the current view.
func (m Model) propagateWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewHome:
		m.home.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.home, cmd = m.home.Update(msg)
		return m, cmd
	case ViewFilePicker:
		m.filePicker.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)
		return m, cmd
	case ViewCreating:
		m.creating.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.creating, cmd = m.creating.Update(msg)
		return m, cmd
	case ViewPlanList:
		m.planList.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.planList, cmd = m.planList.Update(msg)
		return m, cmd
	case ViewRunning:
		m.running.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.running, cmd = m.running.Update(msg)
		return m, cmd
	}
	return m, nil
}

// delegateToCurrentView passes messages to the current view's Update function.
func (m Model) delegateToCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewHome:
		var cmd tea.Cmd
		m.home, cmd = m.home.Update(msg)
		return m, cmd
	case ViewFilePicker:
		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)
		return m, cmd
	case ViewCreating:
		var cmd tea.Cmd
		m.creating, cmd = m.creating.Update(msg)
		return m, cmd
	case ViewPlanList:
		var cmd tea.Cmd
		m.planList, cmd = m.planList.Update(msg)
		return m, cmd
	case ViewRunning:
		var cmd tea.Cmd
		m.running, cmd = m.running.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	// Check for minimum terminal size
	if m.width < MinTerminalWidth || m.height < MinTerminalHeight {
		return m.renderTerminalTooSmall()
	}

	switch m.currentView {
	case ViewHome:
		return m.home.View()
	case ViewFilePicker:
		return m.filePicker.View()
	case ViewCreating:
		return m.creating.View()
	case ViewPlanList:
		return m.planList.View()
	case ViewRunning:
		return m.running.View()
	}
	return "Unknown view"
}

// renderTerminalTooSmall displays a warning when terminal is below minimum size.
func (m Model) renderTerminalTooSmall() string {
	msg := fmt.Sprintf(
		"Terminal too small.\nMinimum: %dx%d\nCurrent: %dx%d",
		MinTerminalWidth, MinTerminalHeight,
		m.width, m.height,
	)
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		msg,
	)
}
