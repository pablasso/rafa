package tui

import (
	"context"
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
	ViewPlanCreate
	ViewPlanList
	ViewRunning
)

// Model is the main Bubble Tea model that orchestrates all views.
type Model struct {
	currentView View
	width       int
	height      int

	initCmd tea.Cmd

	// Sub-models for each view
	home       views.HomeModel
	filePicker views.FilePickerModel
	planCreate views.PlanCreateModel
	planList   views.PlanListModel
	running    views.RunningModel

	// Shared state
	repoRoot string
	rafaDir  string
	err      error
}

// Run starts the TUI application.
func Run(opts Options) error {
	m := initialModelWithOptions(opts)

	Program = tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := Program.Run()
	return err
}

func initialModel() Model {
	return initialModelWithOptions(Options{})
}

func initialModelWithOptions(opts Options) Model {
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

	if opts.Demo != nil {
		m = m.withDemoStartup(*opts.Demo)
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

// hasMarkdownFiles checks if a directory exists and contains at least one .md file.
func hasMarkdownFiles(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}
	pattern := filepath.Join(dir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return false
	}
	return len(matches) > 0
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	var base tea.Cmd
	switch m.currentView {
	case ViewHome:
		base = m.home.Init()
	case ViewFilePicker:
		base = m.filePicker.Init()
	case ViewPlanCreate:
		base = m.planCreate.Init()
	case ViewPlanList:
		base = m.planList.Init()
	case ViewRunning:
		base = m.running.Init()
	}

	if m.initCmd == nil {
		return base
	}
	if base == nil {
		return m.initCmd
	}
	return tea.Batch(base, m.initCmd)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate to current view
		return m.propagateWindowSize(msg)

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
		// For plan creation, start in docs/designs/ and check for .md files
		if msg.ForPlanCreation {
			designsDir := filepath.Join(m.repoRoot, "docs", "designs")
			// Check if docs/designs/ exists and has .md files
			if hasMarkdownFiles(designsDir) {
				startDir = designsDir
			} else {
				// No design docs exist, show error and return to home
				m.currentView = ViewHome
				m.home = views.NewHomeModel(m.rafaDir)
				m.home.SetSize(m.width, m.height)
				m.home.SetError("No design documents found in docs/designs/. Create a design first.")
				return m, m.home.Init()
			}
		}
		m.filePicker = views.NewFilePickerModel(startDir)
		m.filePicker.SetSize(m.width, m.height)
		return m, m.filePicker.Init()

	case msgs.FileSelectedMsg:
		m.currentView = ViewPlanCreate
		m.planCreate = views.NewPlanCreateModel(msg.Path)
		m.planCreate.SetSize(m.width, m.height)
		return m, m.planCreate.Init()

	case msgs.GoToPlanListMsg:
		m.currentView = ViewPlanList
		m.planList = views.NewPlanListModel(m.rafaDir)
		m.planList.SetSize(m.width, m.height)
		return m, m.planList.Init()

	case msgs.RunPlanMsg:
		return m.transitionToRunning(msg.PlanID)

	}

	// Delegate all other messages to the current view
	return m.delegateToCurrentView(msg)
}

func (m Model) withDemoStartup(opts DemoOptions) Model {
	mode := opts.Mode
	if mode == "" {
		mode = demo.ModeRun
	}

	switch mode {
	case demo.ModeCreate:
		return m.withDemoCreateStartup(opts)
	case demo.ModeRun:
		return m.withDemoRunStartup(opts)
	default:
		// Unknown mode should never happen (validated by CLI), but keep a safe fallback.
		return m.withDemoRunStartup(opts)
	}
}

func (m Model) withDemoRunStartup(opts DemoOptions) Model {
	warning := ""

	baseDataset, err := demo.LoadDefaultDataset()
	if err != nil {
		warning = fmt.Sprintf("Warning: using fallback demo data (%v)", err)
		baseDataset = demo.FallbackDataset()
	}

	maxTasks, err := demo.MaxTasksForPreset(opts.Preset)
	if err != nil {
		warning = fmt.Sprintf("Warning: %v", err)
		maxTasks = 0
	}

	dataset, err := demo.ApplyScenario(baseDataset, opts.Scenario, maxTasks)
	if err != nil {
		warning = fmt.Sprintf("Warning: %v", err)
		dataset = baseDataset
	}

	config, err := demo.NewConfig(opts.Preset, dataset)
	if err != nil {
		warning = fmt.Sprintf("Warning: %v", err)
		config = demo.Config{Preset: opts.Preset}
	}

	m.currentView = ViewRunning
	m.running = views.NewRunningModelForDemo("DEMO", dataset.Plan.Name, dataset.Plan.Tasks, dataset.Plan, warning)
	m.running.SetSize(m.width, m.height)

	playback := demo.NewPlayback(dataset, config)
	runner := &m.running

	m.initCmd = func() tea.Msg {
		if Program == nil {
			return nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		runner.SetCancel(cancel)
		playback.Run(ctx, Program, runner.OutputChan())
		return nil
	}

	return m
}

func (m Model) withDemoCreateStartup(opts DemoOptions) Model {
	warning := ""

	dataset, err := demo.LoadDefaultCreateDataset()
	if err != nil {
		warning = fmt.Sprintf("Warning: using fallback create demo data (%v)", err)
		dataset = demo.FallbackCreateDataset()
	}

	config, err := demo.NewCreateReplayConfig(opts.Preset, dataset)
	if err != nil {
		warning = fmt.Sprintf("Warning: %v", err)
		config = demo.CreateReplayConfig{
			Preset:     opts.Preset,
			EventDelay: 50 * time.Millisecond,
		}
	}

	sourceFile := dataset.SourceFile
	if sourceFile == "" {
		sourceFile = "docs/designs/demo-mode-reborn.md"
	}

	m.currentView = ViewPlanCreate
	m.planCreate = views.NewPlanCreateModelForDemoUnsaved(
		sourceFile,
		demo.NewCreateReplayStarter(dataset, config),
		warning,
	)
	m.planCreate.SetSize(m.width, m.height)

	return m
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
	case ViewPlanCreate:
		m.planCreate.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.planCreate, cmd = m.planCreate.Update(msg)
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
	case ViewPlanCreate:
		var cmd tea.Cmd
		m.planCreate, cmd = m.planCreate.Update(msg)
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
	case ViewPlanCreate:
		return m.planCreate.View()
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
