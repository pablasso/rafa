package views

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
	"github.com/pablasso/rafa/internal/util"
)

// createState represents the current state of the creating view.
type createState int

const (
	stateExtracting createState = iota
	stateSuccess
	stateError
)

// PlanCreationErrorMsg is sent when plan creation fails.
type PlanCreationErrorMsg struct {
	Err error
}

// CreatingModel is the model for the plan creation progress view.
type CreatingModel struct {
	state      createState
	spinner    spinner.Model
	sourceFile string   // path to design doc
	planID     string   // set after successful creation
	tasks      []string // task titles after creation
	err        error    // set on failure
	width      int
	height     int
}

// ExtractTasksFunc is the function type for extracting tasks from a design document.
// It can be replaced in tests to mock task extraction.
type ExtractTasksFunc func(ctx context.Context, designContent string) (*plan.TaskExtractionResult, error)

// CreatePlanFolderFunc is the function type for creating a plan folder.
// It can be replaced in tests.
type CreatePlanFolderFunc func(p *plan.Plan) error

// Dependency injection for testing
var (
	extractTasks     ExtractTasksFunc     = ai.ExtractTasks
	createPlanFolder CreatePlanFolderFunc = plan.CreatePlanFolder
)

// NewCreatingModel creates a new CreatingModel for the given source file.
func NewCreatingModel(sourceFile string) CreatingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot // ⣾ style spinner
	s.Style = styles.SelectedStyle

	return CreatingModel{
		state:      stateExtracting,
		spinner:    s,
		sourceFile: sourceFile,
	}
}

// Init implements tea.Model.
func (m CreatingModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startExtraction(),
	)
}

// startExtraction kicks off the background extraction process.
func (m CreatingModel) startExtraction() tea.Cmd {
	return func() tea.Msg {
		// Create a context for the extraction. When the user presses Ctrl+C,
		// the view navigates away but this goroutine continues to completion
		// (the default timeout in ai.ExtractTasks will eventually apply).
		ctx := context.Background()

		// Read the design file
		content, err := os.ReadFile(m.sourceFile)
		if err != nil {
			return PlanCreationErrorMsg{Err: fmt.Errorf("failed to read file: %w", err)}
		}

		// Extract tasks using AI
		extracted, err := extractTasks(ctx, string(content))
		if err != nil {
			return PlanCreationErrorMsg{Err: fmt.Errorf("failed to extract tasks: %w", err)}
		}

		// Generate plan ID
		id, err := util.GenerateShortID()
		if err != nil {
			return PlanCreationErrorMsg{Err: fmt.Errorf("failed to generate plan ID: %w", err)}
		}

		// Determine plan name from extraction or filename
		baseName := extracted.Name
		if baseName == "" {
			base := filepath.Base(m.sourceFile)
			baseName = strings.TrimSuffix(base, filepath.Ext(base))
		}
		baseName = util.ToKebabCase(baseName)

		// Resolve name collisions
		name, err := plan.ResolvePlanName(baseName)
		if err != nil {
			return PlanCreationErrorMsg{Err: fmt.Errorf("failed to resolve plan name: %w", err)}
		}

		// Build the plan
		tasks := make([]plan.Task, len(extracted.Tasks))
		taskTitles := make([]string, len(extracted.Tasks))
		for i, et := range extracted.Tasks {
			tasks[i] = plan.Task{
				ID:                 util.GenerateTaskID(i),
				Title:              et.Title,
				Description:        et.Description,
				AcceptanceCriteria: et.AcceptanceCriteria,
				Status:             plan.TaskStatusPending,
				Attempts:           0,
			}
			taskTitles[i] = et.Title
		}

		// Normalize source path
		sourcePath := normalizeSourcePath(m.sourceFile)

		p := &plan.Plan{
			ID:          id,
			Name:        name,
			Description: extracted.Description,
			SourceFile:  sourcePath,
			CreatedAt:   time.Now(),
			Status:      plan.PlanStatusNotStarted,
			Tasks:       tasks,
		}

		// Create the plan folder
		if err := createPlanFolder(p); err != nil {
			return PlanCreationErrorMsg{Err: fmt.Errorf("failed to create plan: %w", err)}
		}

		planID := fmt.Sprintf("%s-%s", p.ID, p.Name)
		return msgs.PlanCreatedMsg{
			PlanID: planID,
			Tasks:  taskTitles,
		}
	}
}

// normalizeSourcePath converts an absolute path to relative from repo root.
func normalizeSourcePath(filePath string) string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath
	}

	// Find repo root by looking for .rafa directory
	repoRoot, err := findRepoRoot()
	if err != nil {
		return filePath
	}

	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return filePath
	}

	return relPath
}

// findRepoRoot walks up directories looking for .rafa/ folder.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		rafaPath := filepath.Join(dir, ".rafa")
		if info, err := os.Stat(rafaPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(".rafa directory not found")
		}
		dir = parent
	}
}

// Update implements tea.Model.
func (m CreatingModel) Update(msg tea.Msg) (CreatingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case msgs.PlanCreatedMsg:
		m.state = stateSuccess
		m.planID = msg.PlanID
		m.tasks = msg.Tasks
		return m, nil

	case PlanCreationErrorMsg:
		m.state = stateError
		m.err = msg.Err
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleKeyPress handles keyboard input based on current state.
func (m CreatingModel) handleKeyPress(msg tea.KeyMsg) (CreatingModel, tea.Cmd) {
	switch m.state {
	case stateExtracting:
		// During extraction, only Ctrl+C to cancel
		if msg.String() == "ctrl+c" {
			return m, func() tea.Msg { return msgs.GoToFilePickerMsg{} }
		}

	case stateSuccess:
		switch msg.String() {
		case "r":
			return m, func() tea.Msg { return msgs.RunPlanMsg{PlanID: m.planID} }
		case "h":
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case stateError:
		switch msg.String() {
		case "r":
			// Retry - reset state and restart extraction
			m.state = stateExtracting
			m.err = nil
			return m, tea.Batch(
				m.spinner.Tick,
				m.startExtraction(),
			)
		case "b":
			return m, func() tea.Msg { return msgs.GoToFilePickerMsg{} }
		case "h":
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View implements tea.Model.
func (m CreatingModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	switch m.state {
	case stateExtracting:
		return m.renderExtracting()
	case stateSuccess:
		return m.renderSuccess()
	case stateError:
		return m.renderError()
	}

	return ""
}

// renderExtracting renders the view during task extraction.
func (m CreatingModel) renderExtracting() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Creating Plan")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n\n")

	// Source file
	sourceLabel := styles.SubtleStyle.Render("Source: ")
	sourcePath := m.sourceFile
	// Truncate if too long
	maxPathLen := m.width - 10
	if maxPathLen > 0 && len(sourcePath) > maxPathLen {
		sourcePath = "..." + sourcePath[len(sourcePath)-maxPathLen+3:]
	}
	sourceLine := sourceLabel + sourcePath
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, sourceLine))
	b.WriteString("\n\n")

	// Spinner with message
	spinnerView := m.spinner.View() + " Extracting tasks from design..."
	spinnerLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, spinnerView)
	b.WriteString(spinnerLine)
	b.WriteString("\n")

	// Fill remaining space
	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1 // -1 for status bar
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar
	statusItems := []string{"Please wait...", "Ctrl+C Cancel"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// renderSuccess renders the view after successful plan creation.
func (m CreatingModel) renderSuccess() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Plan Created")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n\n")

	// Success message
	checkMark := styles.SuccessStyle.Render("✓")
	successMsg := fmt.Sprintf("%s Plan created: %s", checkMark, m.planID)
	successLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, successMsg)
	b.WriteString(successLine)
	b.WriteString("\n\n")

	// Tasks header
	tasksHeader := styles.SubtleStyle.Render("Tasks:")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, tasksHeader))
	b.WriteString("\n")

	// Task list (centered block)
	var taskLines []string
	for i, taskTitle := range m.tasks {
		taskLine := fmt.Sprintf("  %d. %s", i+1, taskTitle)
		taskLines = append(taskLines, taskLine)
	}
	taskBlock := strings.Join(taskLines, "\n")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, taskBlock))
	b.WriteString("\n\n")

	// Options
	runOption := styles.SelectedStyle.Render("[r]") + " Run this plan now"
	homeOption := styles.SubtleStyle.Render("[h]") + " Return to home"
	optionsLine := runOption + "    " + homeOption
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, optionsLine))
	b.WriteString("\n")

	// Fill remaining space
	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar
	statusItems := []string{"r Run plan", "h Home", "q Quit"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// renderError renders the view when plan creation fails.
func (m CreatingModel) renderError() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Plan Creation Failed")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n\n")

	// Error message
	errorMark := styles.ErrorStyle.Render("✗")
	errorMsg := fmt.Sprintf("%s Error: Failed to extract tasks", errorMark)
	errorLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, errorMsg)
	b.WriteString(errorLine)
	b.WriteString("\n\n")

	// Error details
	if m.err != nil {
		detailsLabel := styles.SubtleStyle.Render("Details: ")
		errStr := m.err.Error()
		// Truncate if too long
		maxErrLen := m.width - 15
		if maxErrLen > 0 && len(errStr) > maxErrLen {
			errStr = errStr[:maxErrLen-3] + "..."
		}
		detailsLine := detailsLabel + errStr
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, detailsLine))
		b.WriteString("\n\n")
	}

	// Options
	retryOption := styles.SelectedStyle.Render("[r]") + " Retry"
	backOption := styles.SubtleStyle.Render("[b]") + " Back to file picker"
	homeOption := styles.SubtleStyle.Render("[h]") + " Home"
	optionsLine := retryOption + "    " + backOption + "    " + homeOption
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, optionsLine))
	b.WriteString("\n")

	// Fill remaining space
	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar
	statusItems := []string{"r Retry", "b Back", "h Home"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// SetSize updates the model dimensions.
func (m *CreatingModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// State returns the current state of the model.
func (m CreatingModel) State() createState {
	return m.state
}

// PlanID returns the created plan ID (empty if not yet created).
func (m CreatingModel) PlanID() string {
	return m.planID
}

// Tasks returns the task titles (empty if not yet created).
func (m CreatingModel) Tasks() []string {
	return m.tasks
}

// Err returns the error if in error state.
func (m CreatingModel) Err() error {
	return m.err
}

// SourceFile returns the source file path.
func (m CreatingModel) SourceFile() string {
	return m.sourceFile
}
