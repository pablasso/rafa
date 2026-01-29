package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/demo"
	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
)

// runState represents the current state of the running view.
type runState int

const (
	stateRunning runState = iota
	stateDone
	stateCancelled
)

// TaskDisplay holds display information for a task.
type TaskDisplay struct {
	Title  string
	Status string // "pending", "running", "completed", "failed"
}

// RunningModel is the model for the execution monitor view.
type RunningModel struct {
	state       runState
	planID      string
	planName    string
	tasks       []TaskDisplay
	currentTask int // 1-indexed current task number
	totalTasks  int
	attempt     int
	maxAttempts int
	startTime   time.Time

	spinner spinner.Model
	output  components.OutputViewport

	// For receiving events from executor
	outputChan chan string
	cancel     context.CancelFunc // Set when executor starts

	// Plan execution context
	planDir string
	plan    *plan.Plan

	// Demo mode fields
	demoMode   bool
	demoConfig *demo.Config

	// Final status
	finalSuccess bool
	finalMessage string

	width  int
	height int
}

// Message types for executor events

// TaskStartedMsg is sent when a task begins execution.
type TaskStartedMsg struct {
	TaskNum int
	Total   int
	TaskID  string
	Title   string
	Attempt int
}

// TaskCompletedMsg is sent when a task completes successfully.
type TaskCompletedMsg struct {
	TaskID string
}

// TaskFailedMsg is sent when a task attempt fails.
type TaskFailedMsg struct {
	TaskID  string
	Attempt int
	Err     error
}

// OutputLineMsg contains a line of output from the executor.
type OutputLineMsg struct {
	Line string
}

// PlanDoneMsg signals that the plan execution has finished.
type PlanDoneMsg struct {
	Success   bool
	Message   string
	Succeeded int
	Total     int
	Duration  time.Duration
}

// tickMsg is used for elapsed time updates.
type tickMsg time.Time

// NewRunningModel creates a new RunningModel for executing a plan.
func NewRunningModel(planID, planName string, tasks []plan.Task, planDir string, p *plan.Plan) RunningModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	taskDisplays := make([]TaskDisplay, len(tasks))
	for i, t := range tasks {
		status := "pending"
		if t.Status == plan.TaskStatusCompleted {
			status = "completed"
		} else if t.Status == plan.TaskStatusFailed {
			status = "failed"
		} else if t.Status == plan.TaskStatusInProgress {
			status = "running"
		}
		taskDisplays[i] = TaskDisplay{
			Title:  t.Title,
			Status: status,
		}
	}

	return RunningModel{
		state:       stateRunning,
		planID:      planID,
		planName:    planName,
		tasks:       taskDisplays,
		currentTask: 0,
		totalTasks:  len(tasks),
		attempt:     0,
		maxAttempts: executor.MaxAttempts,
		startTime:   time.Now(),
		spinner:     s,
		output:      components.NewOutputViewport(80, 20, 0), // Will be resized
		outputChan:  make(chan string, 100),                  // Buffered channel
		planDir:     planDir,
		plan:        p,
	}
}

// NewRunningModelWithDemo creates a new RunningModel for demo mode execution.
// In demo mode, no plan directory is used and execution is simulated.
func NewRunningModelWithDemo(planID, planName string, tasks []plan.Task, p *plan.Plan, config *demo.Config) RunningModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	taskDisplays := make([]TaskDisplay, len(tasks))
	for i, t := range tasks {
		taskDisplays[i] = TaskDisplay{
			Title:  t.Title,
			Status: "pending",
		}
	}

	return RunningModel{
		state:       stateRunning,
		planID:      planID,
		planName:    planName,
		tasks:       taskDisplays,
		currentTask: 0,
		totalTasks:  len(tasks),
		attempt:     0,
		maxAttempts: executor.MaxAttempts,
		startTime:   time.Now(),
		spinner:     s,
		output:      components.NewOutputViewport(80, 20, 0), // Will be resized
		outputChan:  make(chan string, 100),                  // Buffered channel
		planDir:     "",                                      // No plan directory in demo mode
		plan:        p,
		demoMode:    true,
		demoConfig:  config,
	}
}

// Init implements tea.Model.
func (m RunningModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.tickCmd(),
		m.listenForOutput(),
	)
}

// tickCmd returns a command that sends tick messages for elapsed time updates.
func (m RunningModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// listenForOutput returns a command that waits for output from the channel.
func (m RunningModel) listenForOutput() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-m.outputChan
		if !ok {
			return nil
		}
		return OutputLineMsg{Line: line}
	}
}

// OutputChan returns the output channel for executor integration.
func (m *RunningModel) OutputChan() chan string {
	return m.outputChan
}

// SetCancel sets the cancellation function for graceful shutdown.
func (m *RunningModel) SetCancel(cancel context.CancelFunc) {
	m.cancel = cancel
}

// StartExecutor creates a command that starts plan execution in a goroutine.
// It creates the executor with events integration and output capture.
// In demo mode, it injects DemoRunner and skips persistence operations.
func (m *RunningModel) StartExecutor(program *tea.Program) tea.Cmd {
	return func() tea.Msg {
		// Guard against nil program
		if program == nil {
			return PlanDoneMsg{Success: false, Message: "Internal error: program is nil"}
		}

		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel

		// Create events handler to send messages to TUI
		events := NewRunningModelEvents(program)

		// In demo mode, use simplified execution without file-based output capture
		if m.demoMode {
			// Create demo runner with config
			demoRunner := demo.NewDemoRunner(m.demoConfig)

			// Create output capture for demo mode (no file, just channel streaming)
			output := executor.NewOutputCaptureForDemo(m.outputChan)

			// Create executor with demo runner injected
			// Note: planDir is empty in demo mode, but WithSkipPersistence prevents file operations
			exec := executor.New(m.planDir, m.plan).
				WithRunner(demoRunner).
				WithEvents(events).
				WithOutput(output).
				WithSkipPersistence(true) // Skips git checks, commits, and file persistence

			// Run in background goroutine
			go func() {
				defer output.Close()
				defer close(m.outputChan)

				// Run executor and send error as message if it fails
				if err := exec.Run(ctx); err != nil {
					// Only send error if context wasn't cancelled (user didn't press Ctrl+C)
					if ctx.Err() == nil {
						program.Send(PlanDoneMsg{
							Success: false,
							Message: err.Error(),
						})
					}
				}
			}()

			return nil
		}

		// Normal execution mode with file-based output capture
		output, err := executor.NewOutputCaptureWithEvents(m.planDir, m.outputChan)
		if err != nil {
			return PlanDoneMsg{Success: false, Message: fmt.Sprintf("Failed to create output capture: %v", err)}
		}

		// Create executor with events integration and output capture
		exec := executor.New(m.planDir, m.plan).
			WithEvents(events).
			WithOutput(output).
			WithAllowDirty(false)

		// Run in background goroutine
		go func() {
			defer output.Close()
			defer close(m.outputChan)

			// Run executor and send error as message if it fails
			if err := exec.Run(ctx); err != nil {
				// Only send error if context wasn't cancelled (user didn't press Ctrl+C)
				if ctx.Err() == nil {
					program.Send(PlanDoneMsg{
						Success: false,
						Message: err.Error(),
					})
				}
			}
		}()

		return nil
	}
}

// Update implements tea.Model.
func (m RunningModel) Update(msg tea.Msg) (RunningModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateOutputSize()
		return m, nil

	case spinner.TickMsg:
		if m.state == stateRunning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tickMsg:
		if m.state == stateRunning {
			return m, m.tickCmd()
		}
		return m, nil

	case TaskStartedMsg:
		m.currentTask = msg.TaskNum
		m.attempt = msg.Attempt
		// Update task status
		if msg.TaskNum > 0 && msg.TaskNum <= len(m.tasks) {
			m.tasks[msg.TaskNum-1].Status = "running"
		}
		return m, nil

	case TaskCompletedMsg:
		// Find and update the completed task
		for i := range m.tasks {
			if m.tasks[i].Status == "running" {
				m.tasks[i].Status = "completed"
				break
			}
		}
		return m, nil

	case TaskFailedMsg:
		// Mark current task as failed if max attempts reached
		if msg.Attempt >= m.maxAttempts {
			for i := range m.tasks {
				if m.tasks[i].Status == "running" {
					m.tasks[i].Status = "failed"
					break
				}
			}
		}
		return m, nil

	case OutputLineMsg:
		m.output.AddLine(msg.Line)
		return m, m.listenForOutput()

	case PlanDoneMsg:
		m.state = stateDone
		m.finalSuccess = msg.Success
		if msg.Success {
			m.finalMessage = fmt.Sprintf("Completed %d/%d tasks in %s",
				msg.Succeeded, msg.Total, m.formatDuration(msg.Duration))
		} else {
			m.finalMessage = msg.Message
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	// Pass through to output viewport for scrolling
	var cmd tea.Cmd
	m.output, cmd = m.output.Update(msg)
	return m, cmd
}

// handleKeyPress handles keyboard input based on current state.
func (m RunningModel) handleKeyPress(msg tea.KeyMsg) (RunningModel, tea.Cmd) {
	switch m.state {
	case stateRunning:
		switch msg.String() {
		case "ctrl+c":
			// Trigger graceful stop - cancels the executor context
			if m.cancel != nil {
				m.cancel()
			}
			m.state = stateCancelled
			m.finalMessage = fmt.Sprintf("Stopped. Completed %d/%d tasks.",
				m.countCompleted(), m.totalTasks)
			return m, nil
		case "up", "k", "pgup", "ctrl+u", "down", "j", "pgdown", "ctrl+d", "home", "g", "end", "G":
			// Pass scroll keys to output viewport
			var cmd tea.Cmd
			m.output, cmd = m.output.Update(msg)
			return m, cmd
		}

	case stateDone, stateCancelled:
		switch msg.String() {
		case "enter", "h":
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

// updateOutputSize recalculates the output viewport size based on window size.
func (m *RunningModel) updateOutputSize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	// Right panel is 60% of width minus borders
	rightWidth := (m.width * 60 / 100) - 4

	// Height: total - title(2) - bottom border(1) - status bar(1) - borders(2)
	outputHeight := m.height - 6

	if outputHeight < 3 {
		outputHeight = 3
	}
	if rightWidth < 10 {
		rightWidth = 10
	}

	m.output.SetSize(rightWidth, outputHeight)
}

// View implements tea.Model.
func (m RunningModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	switch m.state {
	case stateRunning:
		return m.renderRunning()
	case stateDone:
		return m.renderDone()
	case stateCancelled:
		return m.renderCancelled()
	}

	return ""
}

// renderRunning renders the split-panel execution view.
func (m RunningModel) renderRunning() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render(fmt.Sprintf("Running: %s-%s", m.planID, m.planName))
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n")

	// Calculate panel dimensions
	leftWidth := (m.width * 40 / 100) - 2
	rightWidth := (m.width * 60 / 100) - 2
	panelHeight := m.height - 4 // title + status bar + borders

	if panelHeight < 5 {
		panelHeight = 5
	}

	// Build left panel content
	leftContent := m.renderLeftPanel(leftWidth, panelHeight-2)

	// Build right panel content
	rightContent := m.renderRightPanel(rightWidth, panelHeight-2)

	// Style panels with borders - inherit border color from BoxStyle
	leftPanelStyle := styles.BoxStyle.Copy().
		Width(leftWidth).
		Height(panelHeight-2).
		Padding(0, 1) // Override padding for tighter layout

	rightPanelStyle := styles.BoxStyle.Copy().
		Width(rightWidth).
		Height(panelHeight-2).
		Padding(0, 1)

	leftPanel := leftPanelStyle.Render(leftContent)
	rightPanel := rightPanelStyle.Render(rightContent)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	b.WriteString(panels)
	b.WriteString("\n")

	// Status bar
	statusItems := []string{"Running...", "Ctrl+C Cancel"}
	if m.demoMode {
		statusItems = append([]string{"[DEMO]"}, statusItems...)
	}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// renderLeftPanel renders the progress panel.
func (m RunningModel) renderLeftPanel(width, height int) string {
	var lines []string

	// Header
	lines = append(lines, styles.SubtleStyle.Render("Progress"))
	lines = append(lines, "")

	// Current task info
	if m.currentTask > 0 && m.currentTask <= len(m.tasks) {
		taskInfo := fmt.Sprintf("Task %d/%d: %s", m.currentTask, m.totalTasks, m.tasks[m.currentTask-1].Title)
		if len(taskInfo) > width {
			taskInfo = taskInfo[:width-3] + "..."
		}
		lines = append(lines, taskInfo)
	} else {
		lines = append(lines, fmt.Sprintf("Task 0/%d: Starting...", m.totalTasks))
	}

	// Attempt info
	if m.attempt > 0 {
		lines = append(lines, fmt.Sprintf("Attempt: %d/%d", m.attempt, m.maxAttempts))
	}

	// Elapsed time
	elapsed := time.Since(m.startTime)
	lines = append(lines, fmt.Sprintf("Elapsed: %s", m.formatDuration(elapsed)))
	lines = append(lines, "")

	// Progress bar
	completed := m.countCompleted()
	progressWidth := width - 6
	if progressWidth < 5 {
		progressWidth = 5
	}
	progress := components.NewProgress(completed, m.totalTasks, progressWidth)
	lines = append(lines, progress.View())
	lines = append(lines, "")

	// Task list header
	lines = append(lines, styles.SubtleStyle.Render("Tasks:"))

	// Task list with status indicators
	for i, task := range m.tasks {
		indicator := m.getTaskIndicator(task.Status, i+1 == m.currentTask)
		taskLine := fmt.Sprintf("%s %d. %s", indicator, i+1, task.Title)
		if len(taskLine) > width {
			taskLine = taskLine[:width-3] + "..."
		}
		// Add arrow for current task
		if i+1 == m.currentTask && task.Status == "running" {
			taskLine += " ←"
		}
		lines = append(lines, taskLine)
	}

	// Join lines and pad to height
	content := strings.Join(lines, "\n")
	lineCount := len(lines)
	if lineCount < height {
		content += strings.Repeat("\n", height-lineCount)
	}

	return content
}

// renderRightPanel renders the output panel.
func (m RunningModel) renderRightPanel(width, height int) string {
	var lines []string

	// Header
	lines = append(lines, styles.SubtleStyle.Render("Output"))
	lines = append(lines, "")

	// Update output viewport size
	outputHeight := height - 2
	if outputHeight < 1 {
		outputHeight = 1
	}

	// Render output viewport
	outputView := m.output.View()

	// Add output content
	lines = append(lines, outputView)

	return strings.Join(lines, "\n")
}

// getTaskIndicator returns the status indicator for a task.
func (m RunningModel) getTaskIndicator(status string, isCurrent bool) string {
	switch status {
	case "completed":
		return styles.SuccessStyle.Render("✓")
	case "failed":
		return styles.ErrorStyle.Render("✗")
	case "running":
		if isCurrent {
			return m.spinner.View()
		}
		return "⣾"
	default: // pending
		return styles.SubtleStyle.Render("○")
	}
}

// renderDone renders the completion view.
func (m RunningModel) renderDone() string {
	var b strings.Builder

	// Title
	var title string
	if m.finalSuccess {
		title = styles.SuccessStyle.Render("Plan Completed")
	} else {
		title = styles.ErrorStyle.Render("Plan Failed")
	}
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n\n")

	// Result message
	var resultLine string
	if m.finalSuccess {
		checkMark := styles.SuccessStyle.Render("✓")
		resultLine = fmt.Sprintf("%s %s", checkMark, m.finalMessage)
	} else {
		errorMark := styles.ErrorStyle.Render("✗")
		resultLine = fmt.Sprintf("%s %s", errorMark, m.finalMessage)
	}
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, resultLine))
	b.WriteString("\n\n")

	// Task summary
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styles.SubtleStyle.Render("Task Summary:")))
	b.WriteString("\n")

	for i, task := range m.tasks {
		indicator := m.getTaskIndicator(task.Status, false)
		taskLine := fmt.Sprintf("%s %d. %s", indicator, i+1, task.Title)
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, taskLine))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Options
	homeOption := styles.SelectedStyle.Render("[Enter]") + " Return to home"
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, homeOption))
	b.WriteString("\n")

	// Fill remaining space
	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar
	statusItems := []string{"Enter Home", "q Quit"}
	if m.demoMode {
		statusItems = append([]string{"[DEMO]"}, statusItems...)
	}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// renderCancelled renders the cancelled view.
func (m RunningModel) renderCancelled() string {
	var b strings.Builder

	// Title
	title := styles.SubtleStyle.Render("Execution Cancelled")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n\n")

	// Message
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, m.finalMessage))
	b.WriteString("\n\n")

	// Task summary
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styles.SubtleStyle.Render("Task Summary:")))
	b.WriteString("\n")

	for i, task := range m.tasks {
		indicator := m.getTaskIndicator(task.Status, false)
		taskLine := fmt.Sprintf("%s %d. %s", indicator, i+1, task.Title)
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, taskLine))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Options
	homeOption := styles.SelectedStyle.Render("[Enter]") + " Return to home"
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, homeOption))
	b.WriteString("\n")

	// Fill remaining space
	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar
	statusItems := []string{"Enter Home", "q Quit"}
	if m.demoMode {
		statusItems = append([]string{"[DEMO]"}, statusItems...)
	}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// countCompleted returns the number of completed tasks.
func (m RunningModel) countCompleted() int {
	count := 0
	for _, task := range m.tasks {
		if task.Status == "completed" {
			count++
		}
	}
	return count
}

// formatDuration formats a duration as MM:SS or HH:MM:SS.
func (m RunningModel) formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	mins := d / time.Minute
	d -= mins * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, mins, s)
	}
	return fmt.Sprintf("%02d:%02d", mins, s)
}

// SetSize updates the model dimensions.
func (m *RunningModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.updateOutputSize()
}

// State returns the current state of the model.
func (m RunningModel) State() runState {
	return m.state
}

// PlanID returns the plan ID.
func (m RunningModel) PlanID() string {
	return m.planID
}

// PlanName returns the plan name.
func (m RunningModel) PlanName() string {
	return m.planName
}

// Tasks returns the task display list.
func (m RunningModel) Tasks() []TaskDisplay {
	return m.tasks
}

// CurrentTask returns the current task number (1-indexed).
func (m RunningModel) CurrentTask() int {
	return m.currentTask
}

// Attempt returns the current attempt number.
func (m RunningModel) Attempt() int {
	return m.attempt
}

// FinalSuccess returns whether the plan completed successfully.
func (m RunningModel) FinalSuccess() bool {
	return m.finalSuccess
}

// FinalMessage returns the final status message.
func (m RunningModel) FinalMessage() string {
	return m.finalMessage
}

// Implement ExecutorEvents interface

// RunningModelEvents wraps RunningModel to implement ExecutorEvents.
// It sends Bubble Tea messages through a program reference.
type RunningModelEvents struct {
	program *tea.Program
}

// NewRunningModelEvents creates an ExecutorEvents implementation that
// sends messages to the given Bubble Tea program.
func NewRunningModelEvents(program *tea.Program) *RunningModelEvents {
	return &RunningModelEvents{program: program}
}

// OnTaskStart implements ExecutorEvents.
func (e *RunningModelEvents) OnTaskStart(taskNum, total int, task *plan.Task, attempt int) {
	e.program.Send(TaskStartedMsg{
		TaskNum: taskNum,
		Total:   total,
		TaskID:  task.ID,
		Title:   task.Title,
		Attempt: attempt,
	})
}

// OnTaskComplete implements ExecutorEvents.
func (e *RunningModelEvents) OnTaskComplete(task *plan.Task) {
	e.program.Send(TaskCompletedMsg{
		TaskID: task.ID,
	})
}

// OnTaskFailed implements ExecutorEvents.
func (e *RunningModelEvents) OnTaskFailed(task *plan.Task, attempt int, err error) {
	e.program.Send(TaskFailedMsg{
		TaskID:  task.ID,
		Attempt: attempt,
		Err:     err,
	})
}

// OnOutput implements ExecutorEvents.
func (e *RunningModelEvents) OnOutput(line string) {
	// Output is handled via OutputCaptureWithEvents channel
	// This method can optionally send additional messages
}

// OnPlanComplete implements ExecutorEvents.
func (e *RunningModelEvents) OnPlanComplete(succeeded, total int, duration time.Duration) {
	e.program.Send(PlanDoneMsg{
		Success:   true,
		Succeeded: succeeded,
		Total:     total,
		Duration:  duration,
	})
}

// OnPlanFailed implements ExecutorEvents.
func (e *RunningModelEvents) OnPlanFailed(task *plan.Task, reason string) {
	e.program.Send(PlanDoneMsg{
		Success: false,
		Message: fmt.Sprintf("Failed on task %s: %s", task.ID, reason),
	})
}

// Verify interface compliance
var _ executor.ExecutorEvents = (*RunningModelEvents)(nil)
