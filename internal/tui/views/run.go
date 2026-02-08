package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	stateCancelling
	stateDone
	stateCancelled
)

// TaskDisplay holds display information for a task.
type TaskDisplay struct {
	Title  string
	Status string // "pending", "running", "completed", "failed"
}

// focusPane identifies which scrollable region has keyboard focus in the Run view.
type focusPane int

const (
	focusOutput   focusPane = iota // Output pane (right column, default)
	focusActivity                  // Activity pane (bottom-left)
	focusTasks                     // Tasks list within the Progress pane (top-left)
)

// maxActivityEntries caps the in-memory activity entries to prevent unbounded growth.
const maxActivityEntries = 2000

// RunActivityEntry represents a single item in the activity timeline.
// Similar to ActivityEntry in conversation.go but specific to plan execution.
type RunActivityEntry struct {
	Text        string
	Timestamp   time.Time
	IsDone      bool // Whether this activity is complete
	IsSeparator bool // Whether this is a task/attempt separator line
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

	// Scrollable viewports for Activity and Tasks panes
	activityView components.ScrollViewport
	tasksView    components.ScrollViewport

	// Focus state for keyboard scroll routing
	focus focusPane // defaults to focusOutput (zero value)

	// When true, tasksView auto-follows the current task
	tasksAutoFollow bool

	// For receiving events from executor
	outputChan chan string
	cancel     context.CancelFunc // Set when executor starts

	// Plan execution context
	planDir string
	plan    *plan.Plan

	// Final status
	finalSuccess bool
	finalMessage string

	// Activity timeline for showing tool usage
	activities []RunActivityEntry

	// Number of currently running tools for output thinking indicator.
	activeToolCount int

	// When true, insert a separator before the next non-marker output chunk.
	// This preserves readability when tool marker lines are hidden.
	pendingOutputSeparator bool

	// Token/cost tracking
	taskTokens    int64   // Tokens used in current task
	totalTokens   int64   // Cumulative tokens across all tasks in plan
	estimatedCost float64 // Estimated cost in USD

	// Demo mode indicator
	demoMode bool
	warning  string

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

// OutputLineMsg contains a chunk of output from the executor stream.
type OutputLineMsg struct {
	Line string
}

// AssistantBoundaryMsg marks the end of an assistant message.
type AssistantBoundaryMsg struct{}

// PlanDoneMsg signals that the plan execution has finished.
type PlanDoneMsg struct {
	Success   bool
	Message   string
	Succeeded int
	Total     int
	Duration  time.Duration
}

// ExecutorStartedMsg signals that the executor has started and provides a cancel handle.
type ExecutorStartedMsg struct {
	Cancel context.CancelFunc
}

// PlanCancelledMsg signals that cancellation completed and cleanup finished.
type PlanCancelledMsg struct{}

// ToolUseMsg indicates a tool is being used during task execution.
type ToolUseMsg struct {
	ToolName   string
	ToolTarget string // File path, pattern, or description depending on tool
}

// ToolResultMsg indicates a tool has completed execution.
type ToolResultMsg struct{}

// UsageMsg contains token usage information from a result event.
type UsageMsg struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
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

	output := components.NewOutputViewport(80, 20, 0) // Will be resized
	output.SetShowScrollbar(true)

	return RunningModel{
		state:           stateRunning,
		planID:          planID,
		planName:        planName,
		tasks:           taskDisplays,
		currentTask:     0,
		totalTasks:      len(tasks),
		attempt:         0,
		maxAttempts:     executor.MaxAttempts,
		startTime:       time.Now(),
		spinner:         s,
		output:          output,
		activityView:    components.NewScrollViewport(20, 6, 0), // Will be resized
		tasksView:       components.NewScrollViewport(20, 4, 0), // Will be resized
		tasksAutoFollow: true,
		outputChan:      make(chan string, 100), // Buffered channel
		planDir:         planDir,
		plan:            p,
	}
}

// NewRunningModelForDemo creates a running model for demo playback.
func NewRunningModelForDemo(planID, planName string, tasks []plan.Task, p *plan.Plan, warning string) RunningModel {
	model := NewRunningModel(planID, planName, tasks, "", p)
	model.demoMode = true
	model.warning = warning
	return model
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
func (m *RunningModel) StartExecutor(program *tea.Program) tea.Cmd {
	return func() tea.Msg {
		// Guard against nil program
		if program == nil {
			return PlanDoneMsg{Success: false, Message: "Internal error: program is nil"}
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Create events handler to send messages to TUI
		events := NewRunningModelEvents(program)

		// Normal execution mode with file-based output capture
		output, err := executor.NewOutputCaptureWithEventsAndHooks(
			m.planDir,
			m.outputChan,
			executor.StreamHooks{
				OnToolUse: func(toolName, toolTarget string) {
					program.Send(ToolUseMsg{
						ToolName:   toolName,
						ToolTarget: toolTarget,
					})
				},
				OnToolResult: func() {
					program.Send(ToolResultMsg{})
				},
				OnUsage: func(inputTokens, outputTokens int64, costUSD float64) {
					program.Send(UsageMsg{
						InputTokens:  inputTokens,
						OutputTokens: outputTokens,
						CostUSD:      costUSD,
					})
				},
			},
		)
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
				return
			}

			// Executor exited cleanly after cancellation.
			if ctx.Err() != nil {
				program.Send(PlanCancelledMsg{})
			}
		}()

		return ExecutorStartedMsg{Cancel: cancel}
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
		if m.state == stateRunning || m.state == stateCancelling {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			// Re-sync activity view to update spinner animation on last entry
			m.syncActivityView()
			return m, cmd
		}
		return m, nil

	case tickMsg:
		if m.state == stateRunning || m.state == stateCancelling {
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
		// Reset per-task counters without clearing plan-wide activity history
		m.resetTaskUsage()
		m.pendingOutputSeparator = false
		// Append a separator line for this task/attempt
		separator := fmt.Sprintf("── Task %d/%d: %s (Attempt %d/%d) ──",
			msg.TaskNum, msg.Total, msg.Title, msg.Attempt, m.maxAttempts)
		m.activities = append(m.activities, RunActivityEntry{
			Text:        separator,
			Timestamp:   time.Now(),
			IsDone:      true,
			IsSeparator: true,
		})
		m.trimActivities()
		m.syncActivityView()
		m.syncTasksView()
		return m, nil

	case TaskCompletedMsg:
		// Find and update the completed task
		for i := range m.tasks {
			if m.tasks[i].Status == "running" {
				m.tasks[i].Status = "completed"
				break
			}
		}
		m.syncTasksView()
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
		m.syncTasksView()
		return m, nil

	case ToolUseMsg:
		// Add tool use to activity timeline
		m.addActivity(msg.ToolName, msg.ToolTarget)
		m.activeToolCount++
		m.syncActivityView()
		return m, nil

	case ToolResultMsg:
		// Mark last activity as done
		m.markLastActivityDone()
		if m.activeToolCount > 0 {
			m.activeToolCount--
		}
		m.syncActivityView()
		return m, nil

	case UsageMsg:
		// Update token tracking from result event
		taskTokens := msg.InputTokens + msg.OutputTokens
		m.taskTokens = taskTokens
		m.totalTokens += taskTokens
		if msg.CostUSD > 0 {
			m.estimatedCost += msg.CostUSD
		} else {
			// Estimate cost if not provided
			m.estimatedCost = estimateCost(m.totalTokens)
		}
		return m, nil

	case OutputLineMsg:
		if isAssistantBoundaryChunk(msg.Line) {
			if m.output.LineCount() > 0 {
				m.pendingOutputSeparator = true
			}
			return m, m.listenForOutput()
		}
		if isToolMarkerLine(msg.Line) {
			if m.output.LineCount() > 0 {
				m.pendingOutputSeparator = true
			}
			return m, m.listenForOutput()
		}
		chunk := msg.Line
		if m.pendingOutputSeparator && chunk != "" {
			switch {
			case strings.HasPrefix(chunk, "\n\n"):
				// Already separated by a blank line.
			case strings.HasPrefix(chunk, "\n"):
				chunk = "\n" + chunk
			default:
				chunk = "\n\n" + chunk
			}
		}
		m.pendingOutputSeparator = false
		m.output.AppendChunk(chunk)
		return m, m.listenForOutput()

	case AssistantBoundaryMsg:
		if m.output.LineCount() > 0 {
			m.pendingOutputSeparator = true
		}
		return m, nil

	case PlanDoneMsg:
		m.state = stateDone
		m.activeToolCount = 0
		m.finalSuccess = msg.Success
		if msg.Success {
			m.finalMessage = fmt.Sprintf("Completed %d/%d tasks in %s",
				msg.Succeeded, msg.Total, m.formatDuration(msg.Duration))
		} else {
			m.finalMessage = msg.Message
		}
		return m, nil

	case ExecutorStartedMsg:
		m.cancel = msg.Cancel
		if m.state == stateCancelling && m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		return m, nil

	case PlanCancelledMsg:
		m.state = stateCancelled
		m.activeToolCount = 0
		m.finalMessage = fmt.Sprintf("Stopped. Completed %d/%d tasks.",
			m.countCompleted(), m.totalTasks)
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
			// Trigger graceful stop. If the executor isn't wired yet, stay in
			// cancelling state and cancel as soon as ExecutorStartedMsg arrives.
			m.state = stateCancelling
			m.finalMessage = "Stopping... waiting for cleanup."
			if m.cancel != nil {
				m.cancel()
				m.cancel = nil
			}
			return m, nil
		case "tab":
			m.focus = m.nextFocus()
			return m, nil
		case "up", "k", "pgup", "ctrl+u", "down", "j", "pgdown", "ctrl+d", "home", "g", "end", "G":
			return m.routeScrollKey(msg)
		}

	case stateCancelling:
		switch msg.String() {
		case "tab":
			m.focus = m.nextFocus()
			return m, nil
		case "up", "k", "pgup", "ctrl+u", "down", "j", "pgdown", "ctrl+d", "home", "g", "end", "G":
			return m.routeScrollKey(msg)
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

// nextFocus cycles focus: Output → Activity → Tasks → Output.
func (m RunningModel) nextFocus() focusPane {
	switch m.focus {
	case focusOutput:
		return focusActivity
	case focusActivity:
		return focusTasks
	default:
		return focusOutput
	}
}

// routeScrollKey routes a scroll key event to the currently focused pane's viewport.
func (m RunningModel) routeScrollKey(msg tea.KeyMsg) (RunningModel, tea.Cmd) {
	switch m.focus {
	case focusActivity:
		var cmd tea.Cmd
		m.activityView, cmd = m.activityView.Update(msg)
		return m, cmd
	case focusTasks:
		var cmd tea.Cmd
		m.tasksView, cmd = m.tasksView.Update(msg)
		m.updateTasksAutoFollow()
		return m, cmd
	default: // focusOutput
		var cmd tea.Cmd
		m.output, cmd = m.output.Update(msg)
		return m, cmd
	}
}

// paneStyle returns BoxStyle or FocusedBoxStyle depending on whether pane
// matches the current focus.
func (m RunningModel) paneStyle(pane focusPane) lipgloss.Style {
	if m.focus == pane {
		return styles.FocusedBoxStyle.Copy()
	}
	return styles.BoxStyle.Copy()
}

// focusLabel returns a display name for the given focus pane.
func focusLabel(f focusPane) string {
	switch f {
	case focusOutput:
		return "Output"
	case focusActivity:
		return "Activity"
	case focusTasks:
		return "Tasks"
	default:
		return "Output"
	}
}

// updateOutputSize recalculates the output viewport size based on window size.
func (m *RunningModel) updateOutputSize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	panelChrome := 4 // 2 border chars + 2*1 padding
	minTwoColWidth := (leftMinWidth + panelChrome) + (outputMinWidth + panelChrome)

	var rightWidth, outputHeight int
	var leftWidth, activityContentH, tasksContentH int

	if m.width < minTwoColWidth {
		// Narrow/single-column fallback
		rightWidth = m.width - panelChrome
		leftWidth = m.width - panelChrome
		// Approximate: output gets ~50% of available height minus borders
		availableHeight := m.height - 2      // title + status bar
		contentBudget := availableHeight - 6 // 3 panes * 2 border lines
		outputHeight = contentBudget * narrowFallbackOutputPct / 100

		remaining := contentBudget - outputHeight
		progressContentH := remaining * narrowFallbackProgressPct / 100
		if progressContentH < 1 {
			progressContentH = 1
		}
		activityContentH = remaining - progressContentH
		if activityContentH < 1 {
			activityContentH = 1
		}

		// Tasks height within progress pane (subtract static header lines)
		tasksContentH = progressContentH - progressStaticLines
		if tasksContentH < 1 {
			tasksContentH = 1
		}
	} else {
		// Two-column layout
		leftWidth = (m.width * leftWidthPercent / 100) - panelChrome
		if leftWidth < leftMinWidth {
			leftWidth = leftMinWidth
		}
		rightWidth = m.width - leftWidth - 2*panelChrome
		if rightWidth < outputMinWidth {
			rightWidth = outputMinWidth
		}
		// Output pane height = total available - 2 border lines, minus 2 for header+spacer in renderRightPanel
		availableHeight := m.height - 2
		outputHeight = availableHeight - 2 - 2 // borders + header lines

		// Calculate left column split
		leftTotalHeight := availableHeight
		contentBudget := leftTotalHeight - 4 // 2 borders per pane * 2 panes
		if contentBudget < 2 {
			contentBudget = 2
		}

		progressContentH := contentBudget * progressHeightPct / 100
		if progressContentH < progressMinHeight {
			progressContentH = progressMinHeight
		}
		activityContentH = contentBudget - progressContentH
		if activityContentH < activityMinHeight {
			activityContentH = activityMinHeight
			progressContentH = contentBudget - activityContentH
			if progressContentH < 1 {
				progressContentH = 1
			}
		}
		if progressContentH+activityContentH > contentBudget {
			progressContentH = contentBudget - activityContentH
			if progressContentH < 1 {
				progressContentH = 1
			}
		}

		// Tasks height within progress pane (subtract static header lines)
		tasksContentH = progressContentH - progressStaticLines
		if tasksContentH < 1 {
			tasksContentH = 1
		}
	}

	if outputHeight < 1 {
		outputHeight = 1
	}
	if rightWidth < 10 {
		rightWidth = 10
	}

	m.output.SetSize(rightWidth, outputHeight)

	// Activity viewport: subtract 2 lines for header ("Activity" + "─────")
	activityViewportH := activityContentH - 2
	if activityViewportH < 1 {
		activityViewportH = 1
	}
	m.activityView.SetSize(leftWidth, activityViewportH)

	// Tasks viewport
	m.tasksView.SetSize(leftWidth, tasksContentH)

	// Sync viewport content after resize
	m.syncActivityView()
	m.syncTasksView()
}

// View implements tea.Model.
func (m RunningModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	switch m.state {
	case stateRunning:
		return m.renderRunning()
	case stateCancelling:
		return m.renderRunning()
	case stateDone:
		return m.renderDone()
	case stateCancelled:
		return m.renderCancelled()
	}

	return ""
}

// Layout constants for panel sizing.
const (
	leftWidthPercent          = 30 // Left column gets ~30% of total width
	leftMinWidth              = 24 // Minimum width for left column content
	outputMinWidth            = 36 // Minimum width for output column content
	progressHeightPct         = 45 // Progress pane gets ~45% of left column height
	progressMinHeight         = 16 // Minimum height for Progress pane content (header+usage+tasks)
	activityMinHeight         = 6  // Minimum height for Activity pane content
	narrowFallbackOutputPct   = 50 // Output pane height % in single-column fallback
	narrowFallbackProgressPct = 60 // Progress pane height % of remaining in narrow fallback
	progressStaticLines       = 12 // Lines used by header + usage above the tasks viewport
)

// renderRunning renders the split-panel execution view.
func (m RunningModel) renderRunning() string {
	// Ensure viewport content is up-to-date before rendering.
	m.syncActivityView()
	m.syncTasksView()

	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render(fmt.Sprintf("Running: %s-%s", m.planID, m.planName))
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n")

	// Reserve space for title line (1) + status bar (1)
	availableHeight := m.height - 2
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Calculate minimum total width needed for two-column layout.
	// Each bordered pane has 2 border chars + 2*1 padding = 4 extra width.
	panelChrome := 4
	minTwoColWidth := (leftMinWidth + panelChrome) + (outputMinWidth + panelChrome)

	if m.width < minTwoColWidth {
		b.WriteString(m.renderNarrowLayout(availableHeight))
	} else {
		b.WriteString(m.renderWideLayout(availableHeight))
	}

	b.WriteString("\n")

	// Status bar with focus indicator and scroll hints
	focusHint := "Focus: " + focusLabel(m.focus)
	statusItems := []string{"Running...", focusHint, "Tab Focus", "↑↓ Scroll", "Ctrl+C Cancel"}
	if m.state == stateCancelling {
		statusItems = []string{"Stopping...", focusHint, "Tab Focus", "↑↓ Scroll"}
	}
	if m.demoMode {
		statusItems = append([]string{"[DEMO]"}, statusItems...)
		if m.warning != "" {
			statusItems = append([]string{m.warning}, statusItems...)
		}
	}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// renderWideLayout renders the two-column layout: left (Progress + Activity) | right (Output).
func (m RunningModel) renderWideLayout(availableHeight int) string {
	panelChrome := 4 // 2 border chars + 2*1 padding per pane

	// Calculate column widths (content area, excluding borders/padding)
	leftWidth := (m.width * leftWidthPercent / 100) - panelChrome
	if leftWidth < leftMinWidth {
		leftWidth = leftMinWidth
	}

	rightWidth := m.width - leftWidth - 2*panelChrome
	if rightWidth < outputMinWidth {
		rightWidth = outputMinWidth
	}

	// Split left column vertically into Progress (top) and Activity (bottom).
	// Each bordered pane has 2 vertical border lines, so total height for
	// two stacked panes = progressContentH + activityContentH + 4 (2 borders each).
	leftTotalHeight := availableHeight
	// We have two panes stacked, each with 2 lines of vertical border.
	contentBudget := leftTotalHeight - 4 // 2 top/bottom borders per pane = 4 total
	if contentBudget < 2 {
		contentBudget = 2
	}

	progressContentH := contentBudget * progressHeightPct / 100
	if progressContentH < progressMinHeight {
		progressContentH = progressMinHeight
	}
	activityContentH := contentBudget - progressContentH
	if activityContentH < activityMinHeight {
		activityContentH = activityMinHeight
		progressContentH = contentBudget - activityContentH
		if progressContentH < 1 {
			progressContentH = 1
		}
	}
	// Guard against total exceeding budget (when both minimums exceed budget).
	if progressContentH+activityContentH > contentBudget {
		progressContentH = contentBudget - activityContentH
		if progressContentH < 1 {
			progressContentH = 1
		}
	}

	// Build Progress pane (top-left) — highlighted when focusTasks is active
	progressContent := m.renderProgressPane(leftWidth, progressContentH)
	progressStyle := m.paneStyle(focusTasks).
		Width(leftWidth).
		Height(progressContentH).
		Padding(0, 1)
	progressPanel := progressStyle.Render(progressContent)

	// Build Activity pane (bottom-left)
	activityContent := m.renderActivityPane(leftWidth, activityContentH)
	activityStyle := m.paneStyle(focusActivity).
		Width(leftWidth).
		Height(activityContentH).
		Padding(0, 1)
	activityPanel := activityStyle.Render(activityContent)

	// Stack Progress and Activity vertically
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, progressPanel, activityPanel)

	// Build Output pane (right)
	outputContentH := leftTotalHeight - 2 // single pane: 2 border lines
	if outputContentH < 1 {
		outputContentH = 1
	}
	rightContent := m.renderRightPanel(rightWidth, outputContentH)
	rightStyle := m.paneStyle(focusOutput).
		Width(rightWidth).
		Height(outputContentH).
		Padding(0, 1)
	rightPanel := rightStyle.Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightPanel)
}

// renderNarrowLayout renders a single-column fallback: Output top, Progress middle, Activity bottom.
func (m RunningModel) renderNarrowLayout(availableHeight int) string {
	panelChrome := 2            // vertical borders per pane (top + bottom)
	contentWidth := m.width - 4 // 2 border chars + 2*1 padding
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Distribute height among three stacked panes (3 * 2 border lines = 6).
	contentBudget := availableHeight - 3*panelChrome
	if contentBudget < 3 {
		contentBudget = 3
	}

	outputContentH := contentBudget * narrowFallbackOutputPct / 100
	if outputContentH < 3 {
		outputContentH = 3
	}
	remaining := contentBudget - outputContentH
	progressContentH := remaining * narrowFallbackProgressPct / 100
	if progressContentH < 1 {
		progressContentH = 1
	}
	activityContentH := remaining - progressContentH
	if activityContentH < 1 {
		activityContentH = 1
	}

	// Output (top)
	outputContent := m.renderRightPanel(contentWidth, outputContentH)
	outputPanel := m.paneStyle(focusOutput).
		Width(contentWidth).
		Padding(0, 1).
		Height(outputContentH).Render(outputContent)

	// Progress (middle)
	progressContent := m.renderProgressPane(contentWidth, progressContentH)
	progressPanel := m.paneStyle(focusTasks).
		Width(contentWidth).
		Padding(0, 1).
		Height(progressContentH).Render(progressContent)

	// Activity (bottom)
	activityContent := m.renderActivityPane(contentWidth, activityContentH)
	activityPanel := m.paneStyle(focusActivity).
		Width(contentWidth).
		Padding(0, 1).
		Height(activityContentH).Render(activityContent)

	return lipgloss.JoinVertical(lipgloss.Left, outputPanel, progressPanel, activityPanel)
}

// renderProgressPane renders the Progress pane: header + usage + scrollable tasks list.
func (m RunningModel) renderProgressPane(width, height int) string {
	var lines []string

	// Header: Task N/M, Attempt, Elapsed
	taskLine := fmt.Sprintf("Task 0/%d", m.totalTasks)
	if m.currentTask > 0 && m.currentTask <= len(m.tasks) {
		title := strings.TrimSpace(m.tasks[m.currentTask-1].Title)
		if title != "" {
			taskLine = fmt.Sprintf("Task %d/%d: %s", m.currentTask, m.totalTasks, title)
		} else {
			taskLine = fmt.Sprintf("Task %d/%d", m.currentTask, m.totalTasks)
		}
	}
	taskLine = truncateWithEllipsis(taskLine, width)
	lines = append(lines, taskLine)

	if m.attempt > 0 {
		lines = append(lines, fmt.Sprintf("Attempt %d/%d", m.attempt, m.maxAttempts))
	}

	elapsed := time.Since(m.startTime)
	lines = append(lines, m.formatDuration(elapsed))
	lines = append(lines, "")

	// Usage section
	lines = append(lines, styles.SubtleStyle.Render("Usage"))
	lines = append(lines, "─────")
	lines = append(lines, fmt.Sprintf("Task:  %s", formatTokens(m.taskTokens)))
	lines = append(lines, fmt.Sprintf("Plan:  %s", formatTokens(m.totalTokens)))
	lines = append(lines, fmt.Sprintf("Cost:  $%.2f", m.estimatedCost))
	lines = append(lines, "")

	// Task list header (static)
	lines = append(lines, styles.SubtleStyle.Render("Tasks"))
	lines = append(lines, "─────")

	// Render the scrollable tasks viewport below the static header
	staticContent := strings.Join(lines, "\n")
	tasksContent := m.tasksView.View()

	// Combine static header with scrollable tasks
	result := staticContent + "\n" + tasksContent

	// Pad to fill the full height
	resultLines := strings.Count(result, "\n") + 1
	if resultLines < height {
		result += strings.Repeat("\n", height-resultLines)
	}

	return result
}

// renderActivityPane renders the Activity pane: header + scrollable activity timeline.
func (m RunningModel) renderActivityPane(width, height int) string {
	// Activity header (static)
	header := styles.SubtleStyle.Render("Activity") + "\n" + "─────"

	// Render the scrollable activity viewport below the header
	activityContent := m.activityView.View()

	result := header + "\n" + activityContent

	// Pad to fill the full height
	resultLines := strings.Count(result, "\n") + 1
	if resultLines < height {
		result += strings.Repeat("\n", height-resultLines)
	}

	return result
}

func (m RunningModel) renderTaskList(width, maxLines int) []string {
	if width <= 0 || maxLines <= 0 || len(m.tasks) == 0 {
		return nil
	}

	if maxLines > len(m.tasks) {
		maxLines = len(m.tasks)
	}

	currentIdx := m.currentTask - 1
	if currentIdx < 0 {
		currentIdx = 0
	}
	if currentIdx >= len(m.tasks) {
		currentIdx = len(m.tasks) - 1
	}

	start := currentIdx - (maxLines / 2)
	if start < 0 {
		start = 0
	}
	if start > len(m.tasks)-maxLines {
		start = len(m.tasks) - maxLines
	}
	end := start + maxLines

	var lines []string
	for i := start; i < end; i++ {
		task := m.tasks[i]
		title := strings.TrimSpace(task.Title)
		if title == "" {
			title = "(untitled)"
		}

		isCurrent := i+1 == m.currentTask
		indicator := m.getTaskIndicator(task.Status, isCurrent)

		prefix := fmt.Sprintf("%s %d. ", indicator, i+1)
		prefixWidth := lipgloss.Width(prefix)
		if prefixWidth >= width {
			lines = append(lines, truncateWithEllipsis(prefix, width))
			continue
		}

		availableTitleWidth := width - prefixWidth
		if availableTitleWidth < 0 {
			availableTitleWidth = 0
		}
		title = truncateWithEllipsis(title, availableTitleWidth)

		lines = append(lines, prefix+title)
	}

	return lines
}

// renderRightPanel renders the output panel.
func (m RunningModel) renderRightPanel(width, height int) string {
	var lines []string

	// Header
	lines = append(lines, styles.SubtleStyle.Render("Output"))
	lines = append(lines, "")

	// Get raw viewport content (without scrollbar) so we can apply the
	// inline spinner before the scrollbar column is appended.
	outputView := m.output.ViewContent()
	if m.state == stateRunning && m.isToolRunning() {
		outputView = insertInlineSpinner(outputView, m.spinner.View())
	}
	outputView = m.output.ComposeWithScrollbar(outputView)

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
			return styles.SelectedStyle.Render("▶")
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

// addActivity adds an entry to the activity timeline.
func (m *RunningModel) addActivity(toolName, toolTarget string) {
	entry := formatToolUseEntry(toolName, toolTarget)
	m.activities = append(m.activities, RunActivityEntry{
		Text:      entry,
		Timestamp: time.Now(),
		IsDone:    false,
	})
	m.trimActivities()
}

// trimActivities caps the in-memory activity entries to maxActivityEntries,
// dropping the oldest entries when the cap is exceeded.
func (m *RunningModel) trimActivities() {
	if len(m.activities) > maxActivityEntries {
		trimmed := make([]RunActivityEntry, maxActivityEntries)
		copy(trimmed, m.activities[len(m.activities)-maxActivityEntries:])
		m.activities = trimmed
	}
}

// syncActivityView converts activity entries to []string lines and feeds them
// into the activityView ScrollViewport.
func (m *RunningModel) syncActivityView() {
	contentWidth := m.activityView.ContentWidth()

	var lines []string
	if len(m.activities) == 0 {
		lines = append(lines, styles.SubtleStyle.Render("  (waiting...)"))
	} else {
		for i, entry := range m.activities {
			var line string
			if entry.IsSeparator {
				line = styles.SubtleStyle.Render(entry.Text)
			} else {
				indicator := "├─"
				if entry.IsDone {
					indicator = styles.SuccessStyle.Render("✓")
				} else if i == len(m.activities)-1 && m.state == stateRunning {
					indicator = m.spinner.View()
				}
				line = fmt.Sprintf("%s %s", indicator, entry.Text)
			}
			if contentWidth > 0 && lipgloss.Width(line) > contentWidth {
				line = truncateWithEllipsis(line, contentWidth)
			}
			lines = append(lines, line)
		}
	}

	m.activityView.SetLines(lines)
}

// syncTasksView converts the tasks list to []string lines, feeds them into the
// tasksView ScrollViewport, and applies auto-follow for the current task.
func (m *RunningModel) syncTasksView() {
	contentWidth := m.tasksView.ContentWidth()

	var lines []string
	for i, task := range m.tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			title = "(untitled)"
		}

		isCurrent := i+1 == m.currentTask
		indicator := m.getTaskIndicator(task.Status, isCurrent)

		prefix := fmt.Sprintf("%s %d. ", indicator, i+1)
		prefixWidth := lipgloss.Width(prefix)
		if contentWidth > 0 && prefixWidth >= contentWidth {
			lines = append(lines, truncateWithEllipsis(prefix, contentWidth))
			continue
		}

		availableTitleWidth := contentWidth - prefixWidth
		if availableTitleWidth < 0 {
			availableTitleWidth = 0
		}
		title = truncateWithEllipsis(title, availableTitleWidth)

		lines = append(lines, prefix+title)
	}

	m.tasksView.SetLines(lines)

	// Auto-follow: ensure the current task is visible when auto-scroll is enabled.
	// The tasksView's autoScroll state serves as the auto-follow toggle:
	// user scrolling disables it, reaching the end re-enables it.
	if m.tasksAutoFollow && m.currentTask > 0 && m.currentTask <= len(m.tasks) {
		m.tasksView.EnsureVisible(m.currentTask-1, false)
	}
}

// updateTasksAutoFollow syncs the tasksAutoFollow flag with the tasksView's
// auto-scroll state. Call this after forwarding user scroll events to tasksView.
func (m *RunningModel) updateTasksAutoFollow() {
	m.tasksAutoFollow = m.tasksView.AutoScroll()
}

// markLastActivityDone marks the last activity entry as done.
func (m *RunningModel) markLastActivityDone() {
	if len(m.activities) > 0 {
		m.activities[len(m.activities)-1].IsDone = true
	}
}

// resetTaskUsage resets per-task counters without clearing the plan-wide activity history.
func (m *RunningModel) resetTaskUsage() {
	m.activeToolCount = 0
	m.taskTokens = 0
}

func (m RunningModel) isToolRunning() bool {
	return m.activeToolCount > 0
}

// formatToolUseEntry formats a tool use entry for the activity timeline.
func formatToolUseEntry(toolName, target string) string {
	entry := toolName
	if target != "" {
		shortened := shortenPathForActivity(target, 25)
		entry += ": " + shortened
	}
	return entry
}

// shortenPathForActivity truncates a path/target to fit in the activity timeline.
func shortenPathForActivity(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		lastTwo := strings.Join(parts[len(parts)-2:], "/")
		shortened := ".../" + lastTwo
		if len(shortened) <= maxLen {
			return shortened
		}
	}
	return path[:maxLen-3] + "..."
}

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func isToolMarkerLine(chunk string) bool {
	trimmed := strings.TrimSpace(chunk)
	if trimmed == "" || strings.Contains(trimmed, "\n") {
		return false
	}
	if !strings.HasPrefix(trimmed, "[Tool: ") || !strings.HasSuffix(trimmed, "]") {
		return false
	}
	toolName := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "[Tool: "), "]"))
	return toolName != ""
}

func isAssistantBoundaryChunk(chunk string) bool {
	return chunk == executor.AssistantBoundaryChunk
}

// insertInlineSpinner injects a spinner directly after the last visible text:
// last text line, empty line, spinner icon.
func insertInlineSpinner(outputView, spinnerIcon string) string {
	if outputView == "" {
		return spinnerIcon
	}

	lines := strings.Split(outputView, "\n")
	if len(lines) == 0 {
		return spinnerIcon
	}

	lastTextIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastTextIdx = i
			break
		}
	}

	if lastTextIdx == -1 {
		lines[0] = spinnerIcon
		return strings.Join(lines, "\n")
	}

	separatorIdx := lastTextIdx + 1
	spinnerIdx := lastTextIdx + 2

	if spinnerIdx < len(lines) {
		lines[separatorIdx] = ""
		lines[spinnerIdx] = spinnerIcon
		return strings.Join(lines, "\n")
	}

	if separatorIdx < len(lines) {
		lines[separatorIdx] = spinnerIcon
		return strings.Join(lines, "\n")
	}

	lines[len(lines)-1] = spinnerIcon
	return strings.Join(lines, "\n")
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

// formatTokens formats token counts in a human-readable format (e.g., "12.4k").
func formatTokens(tokens int64) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// estimateCost estimates the cost in USD based on token count.
// Uses approximate Claude pricing: ~$0.003/1K input tokens, ~$0.015/1K output tokens.
// Since we don't distinguish input/output here, uses a blended rate of ~$0.008/1K tokens.
func estimateCost(tokens int64) float64 {
	// Blended rate: approximately $0.008 per 1K tokens
	return float64(tokens) * 0.000008
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
