package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

// paneRect describes a pane's bounding box in screen coordinates (0-indexed).
type paneRect struct {
	x, y, w, h int
}

// contains reports whether the screen coordinate (px, py) is inside the rect.
func (r paneRect) contains(px, py int) bool {
	return px >= r.x && px < r.x+r.w && py >= r.y && py < r.y+r.h
}

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

	// Pane bounding boxes in screen coordinates for mouse wheel hit-testing.
	// Recomputed on WindowSizeMsg / layout changes.
	boundsOutput   paneRect
	boundsActivity paneRect
	boundsProgress paneRect // Progress pane (Tasks viewport lives inside)

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

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	}

	// Pass through to output viewport for scrolling
	var cmd tea.Cmd
	m.output, cmd = m.output.Update(msg)
	return m, cmd
}

// isScrollKey returns true if the key string is a recognized scroll key.
func isScrollKey(key string) bool {
	switch key {
	case "up", "k", "pgup", "ctrl+u", "down", "j", "pgdown", "ctrl+d", "home", "g", "end", "G":
		return true
	}
	return false
}

// handleKeyPress handles keyboard input based on current state.
func (m RunningModel) handleKeyPress(msg tea.KeyMsg) (RunningModel, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case stateRunning:
		switch {
		case key == "ctrl+c":
			// Trigger graceful stop. If the executor isn't wired yet, stay in
			// cancelling state and cancel as soon as ExecutorStartedMsg arrives.
			m.state = stateCancelling
			m.finalMessage = "Stopping... waiting for cleanup."
			if m.cancel != nil {
				m.cancel()
				m.cancel = nil
			}
			return m, nil
		case key == "tab":
			m.focus = m.nextFocus()
			return m, nil
		case isScrollKey(key):
			return m.routeScrollKey(msg)
		}

	case stateCancelling:
		switch {
		case key == "tab":
			m.focus = m.nextFocus()
			return m, nil
		case isScrollKey(key):
			return m.routeScrollKey(msg)
		}

	case stateDone, stateCancelled:
		switch {
		case key == "enter" || key == "h":
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		case key == "q" || key == "ctrl+c":
			return m, tea.Quit
		case key == "tab":
			m.focus = m.nextFocus()
			return m, nil
		case isScrollKey(key):
			return m.routeScrollKey(msg)
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

// handleMouseMsg routes mouse wheel events to the pane under the cursor,
// setting focus to that pane. Non-wheel events are ignored. Falls back to
// the currently focused pane when coordinates don't match any pane.
func (m RunningModel) handleMouseMsg(msg tea.MouseMsg) (RunningModel, tea.Cmd) {
	// Allow mouse wheel scrolling in all run states to keep panes inspectable.
	switch m.state {
	case stateRunning, stateCancelling, stateDone, stateCancelled:
		// proceed
	default:
		return m, nil
	}

	// Only handle wheel events.
	if msg.Button != tea.MouseButtonWheelUp && msg.Button != tea.MouseButtonWheelDown {
		return m, nil
	}

	target := m.hitTestPane(msg.X, msg.Y)
	m.focus = target
	return m.routeMouseScroll(target, msg)
}

// hitTestPane determines which pane the screen coordinate (x, y) falls inside.
// Returns the matching focusPane, or the currently focused pane if ambiguous.
func (m RunningModel) hitTestPane(x, y int) focusPane {
	if m.boundsActivity.contains(x, y) {
		return focusActivity
	}
	if m.boundsProgress.contains(x, y) {
		return focusTasks
	}
	if m.boundsOutput.contains(x, y) {
		return focusOutput
	}
	// Ambiguous / outside all panes — fall back to current focus.
	return m.focus
}

// routeMouseScroll forwards a mouse wheel event to the given pane's viewport.
func (m RunningModel) routeMouseScroll(target focusPane, msg tea.MouseMsg) (RunningModel, tea.Cmd) {
	switch target {
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

// layoutDims holds the computed layout dimensions shared by viewport sizing
// and bounding box calculations. This avoids duplicating layout arithmetic.
type layoutDims struct {
	narrow           bool // true for single-column fallback
	leftWidth        int  // content width of left column (excluding chrome)
	rightWidth       int  // content width of right column (excluding chrome)
	outputContentH   int  // output pane content height
	progressContentH int  // progress pane content height
	activityContentH int  // activity pane content height
	tasksContentH    int  // tasks viewport height (within progress)
	// Outer pane dimensions (including borders) for bounding boxes
	leftOuterW    int
	rightOuterW   int
	outputPaneH   int
	progressPaneH int
	activityPaneH int
}

// computeLayout calculates the layout dimensions for the Run view based on
// the terminal width and height.
func computeLayout(width, height int) layoutDims {
	panelChrome := 4 // 2 border chars + 2*1 padding
	minTwoColWidth := (leftMinWidth + panelChrome) + (outputMinWidth + panelChrome)
	borderPerPane := 2

	var d layoutDims

	if width < minTwoColWidth {
		d.narrow = true
		d.leftWidth = width - panelChrome
		if d.leftWidth < 1 {
			d.leftWidth = 1
		}
		d.rightWidth = d.leftWidth
		d.leftOuterW = width
		d.rightOuterW = width

		availableHeight := height - 2 // title + status bar
		if availableHeight < 3 {
			availableHeight = 3
		}
		contentBudget := availableHeight - 3*borderPerPane
		if contentBudget < 3 {
			contentBudget = 3
		}

		// Output pane has 2 static header lines ("Output" + blank) above the scrollable viewport.
		// outputContentH tracks the viewport height, not the full pane inner height.
		outputPaneInnerH := contentBudget * narrowFallbackOutputPct / 100
		if outputPaneInnerH < 3 {
			// Minimum 3 so the viewport can be at least 1 line tall.
			outputPaneInnerH = 3
		}
		if outputPaneInnerH > contentBudget {
			outputPaneInnerH = contentBudget
		}

		remaining := contentBudget - outputPaneInnerH
		if remaining < 2 {
			// Try to reclaim space from output while keeping its minimum.
			need := 2 - remaining
			if outputPaneInnerH-need >= 3 {
				outputPaneInnerH -= need
				remaining += need
			}
		}

		d.outputContentH = outputPaneInnerH - 2
		if d.outputContentH < 1 {
			d.outputContentH = 1
		}

		d.progressContentH = remaining * narrowFallbackProgressPct / 100
		if d.progressContentH < 1 {
			d.progressContentH = 1
		}
		d.activityContentH = remaining - d.progressContentH
		if d.activityContentH < 1 {
			d.activityContentH = 1
		}

		d.tasksContentH = d.progressContentH - progressStaticLines
		if d.tasksContentH < 1 {
			d.tasksContentH = 1
		}

		// Output pane outer height includes 2 header lines and 2 border lines.
		d.outputPaneH = d.outputContentH + 4
		d.progressPaneH = d.progressContentH + borderPerPane
		d.activityPaneH = d.activityContentH + borderPerPane
	} else {
		d.leftWidth = (width * leftWidthPercent / 100) - panelChrome
		if d.leftWidth < leftMinWidth {
			d.leftWidth = leftMinWidth
		}
		d.rightWidth = width - d.leftWidth - 2*panelChrome
		if d.rightWidth < outputMinWidth {
			d.rightWidth = outputMinWidth
		}

		d.leftOuterW = d.leftWidth + panelChrome
		d.rightOuterW = width - d.leftOuterW
		if d.rightOuterW < 1 {
			d.rightOuterW = 1
		}

		availableHeight := height - 2
		if availableHeight < 4 {
			availableHeight = 4
		}
		d.outputContentH = availableHeight - 2 - 2 // borders + header lines
		if d.outputContentH < 1 {
			d.outputContentH = 1
		}

		contentBudget := availableHeight - 2*borderPerPane
		if contentBudget < 2 {
			contentBudget = 2
		}

		d.progressContentH = contentBudget * progressHeightPct / 100
		if d.progressContentH < progressMinHeight {
			d.progressContentH = progressMinHeight
		}
		d.activityContentH = contentBudget - d.progressContentH
		if d.activityContentH < activityMinHeight {
			d.activityContentH = activityMinHeight
			d.progressContentH = contentBudget - d.activityContentH
			if d.progressContentH < 1 {
				d.progressContentH = 1
			}
		}
		if d.progressContentH+d.activityContentH > contentBudget {
			d.progressContentH = contentBudget - d.activityContentH
			if d.progressContentH < 1 {
				d.progressContentH = 1
			}
		}

		d.tasksContentH = d.progressContentH - progressStaticLines
		if d.tasksContentH < 1 {
			d.tasksContentH = 1
		}

		d.progressPaneH = d.progressContentH + borderPerPane
		d.activityPaneH = d.activityContentH + borderPerPane
		d.outputPaneH = availableHeight
	}

	if d.outputContentH < 1 {
		d.outputContentH = 1
	}
	if d.rightWidth < 10 {
		d.rightWidth = 10
	}

	return d
}

// updateOutputSize recalculates the output viewport size based on window size.
func (m *RunningModel) updateOutputSize() {
	if m.width == 0 || m.height == 0 {
		return
	}

	d := computeLayout(m.width, m.height)

	m.output.SetSize(d.rightWidth, d.outputContentH)

	// Activity viewport: subtract 2 lines for header ("Activity" + "─────")
	activityViewportH := d.activityContentH - 2
	if activityViewportH < 1 {
		activityViewportH = 1
	}
	m.activityView.SetSize(d.leftWidth, activityViewportH)

	// Tasks viewport
	m.tasksView.SetSize(d.leftWidth, d.tasksContentH)

	// Sync viewport content after resize
	m.syncActivityView()
	m.syncTasksView()

	// Recompute pane bounding boxes for mouse hit-testing.
	m.computePaneBounds(d)
}

// computePaneBounds calculates the screen-coordinate bounding boxes for the
// three scrollable panes (Output, Progress/Tasks, Activity). These bounds
// are used for mouse wheel hit-testing.
func (m *RunningModel) computePaneBounds(d layoutDims) {
	titleRows := 1

	if d.narrow {
		y := titleRows
		m.boundsOutput = paneRect{x: 0, y: y, w: d.rightOuterW, h: d.outputPaneH}
		y += d.outputPaneH
		m.boundsProgress = paneRect{x: 0, y: y, w: d.leftOuterW, h: d.progressPaneH}
		y += d.progressPaneH
		m.boundsActivity = paneRect{x: 0, y: y, w: d.leftOuterW, h: d.activityPaneH}
	} else {
		y := titleRows
		m.boundsProgress = paneRect{x: 0, y: y, w: d.leftOuterW, h: d.progressPaneH}
		m.boundsActivity = paneRect{x: 0, y: y + d.progressPaneH, w: d.leftOuterW, h: d.activityPaneH}
		m.boundsOutput = paneRect{x: d.leftOuterW, y: y, w: d.rightOuterW, h: d.outputPaneH}
	}
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
	progressMinHeight         = 14 // Minimum height for Progress pane content (header+metrics+tasks)
	activityMinHeight         = 6  // Minimum height for Activity pane content
	narrowFallbackOutputPct   = 50 // Output pane height % in single-column fallback
	narrowFallbackProgressPct = 60 // Progress pane height % of remaining in narrow fallback
	progressStaticLines       = 8  // Lines used by header + metrics above the tasks viewport
)

// renderRunning renders the split-panel execution view.
func (m RunningModel) renderRunning() string {
	// Ensure viewport content is up-to-date before rendering.
	m.syncActivityView()
	m.syncTasksView()

	var b strings.Builder

	// Title
	// Override TitleStyle's bottom margin here so layout math can reliably
	// assume the title occupies exactly one row.
	title := styles.TitleStyle.Copy().MarginBottom(0).Render(fmt.Sprintf("Running: %s-%s", m.planID, m.planName))
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n")

	d := computeLayout(m.width, m.height)
	if d.narrow {
		b.WriteString(m.renderNarrowLayout(d))
	} else {
		b.WriteString(m.renderWideLayout(d))
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

func renderPane(style lipgloss.Style, innerW, innerH int, content string) string {
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	// lipgloss applies Width/Height before borders, so these dimensions should
	// include padding but exclude borders. Borders are added after sizing.
	padW := style.GetHorizontalPadding()
	padH := style.GetVerticalPadding()
	return style.
		Width(innerW + padW).
		Height(innerH + padH).
		Render(content)
}

// renderWideLayout renders the two-column layout: left (Progress + Activity) | right (Output).
func (m RunningModel) renderWideLayout(d layoutDims) string {
	progressStyle := m.paneStyle(focusTasks).Padding(0, 1)
	activityStyle := m.paneStyle(focusActivity).Padding(0, 1)
	outputStyle := m.paneStyle(focusOutput).Padding(0, 1)

	progressContent := m.renderProgressPane(d.leftWidth, d.progressContentH)
	activityContent := m.renderActivityPane(d.leftWidth, d.activityContentH)
	// Output pane inner height includes 2 static header lines.
	outputInnerH := d.outputContentH + 2
	rightContent := m.renderRightPanel(d.rightWidth, outputInnerH)

	progressPanel := renderPane(progressStyle, d.leftWidth, d.progressContentH, progressContent)
	activityPanel := renderPane(activityStyle, d.leftWidth, d.activityContentH, activityContent)
	rightPanel := renderPane(outputStyle, d.rightWidth, outputInnerH, rightContent)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, progressPanel, activityPanel)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightPanel)
}

// renderNarrowLayout renders a single-column fallback: Output top, Progress middle, Activity bottom.
func (m RunningModel) renderNarrowLayout(d layoutDims) string {
	outputStyle := m.paneStyle(focusOutput).Padding(0, 1)
	progressStyle := m.paneStyle(focusTasks).Padding(0, 1)
	activityStyle := m.paneStyle(focusActivity).Padding(0, 1)

	outputInnerH := d.outputContentH + 2
	outputContent := m.renderRightPanel(d.rightWidth, outputInnerH)
	progressContent := m.renderProgressPane(d.leftWidth, d.progressContentH)
	activityContent := m.renderActivityPane(d.leftWidth, d.activityContentH)

	outputPanel := renderPane(outputStyle, d.rightWidth, outputInnerH, outputContent)
	progressPanel := renderPane(progressStyle, d.leftWidth, d.progressContentH, progressContent)
	activityPanel := renderPane(activityStyle, d.leftWidth, d.activityContentH, activityContent)

	return lipgloss.JoinVertical(lipgloss.Left, outputPanel, progressPanel, activityPanel)
}

// renderProgressPane renders the Progress pane: header + summary + scrollable tasks list.
func (m RunningModel) renderProgressPane(width, height int) string {
	var lines []string

	// Header: Task N/M, Attempt, elapsed, total tokens
	taskValue := fmt.Sprintf("0/%d", m.totalTasks)
	if m.currentTask > 0 && m.currentTask <= len(m.tasks) {
		title := strings.TrimSpace(m.tasks[m.currentTask-1].Title)
		if title != "" {
			taskValue = fmt.Sprintf("%d/%d - %s", m.currentTask, m.totalTasks, title)
		} else {
			taskValue = fmt.Sprintf("%d/%d", m.currentTask, m.totalTasks)
		}
	}
	lines = append(lines, renderProgressStatLine("Task", taskValue, width))

	if m.attempt > 0 {
		lines = append(lines, renderProgressStatLine("Attempt", fmt.Sprintf("%d/%d", m.attempt, m.maxAttempts), width))
	}

	elapsed := time.Since(m.startTime)
	lines = append(lines, renderProgressStatLine("Total time", m.formatDuration(elapsed), width))
	lines = append(lines, renderProgressStatLine("Tokens used", formatTokens(m.totalTokens), width))
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

func renderProgressStatLine(label, value string, width int) string {
	labelText := label + ":"
	if width <= 0 {
		return styles.SubtleStyle.Render(labelText)
	}

	if width <= ansi.StringWidth(labelText) {
		return styles.SubtleStyle.Render(truncateWithEllipsis(labelText, width))
	}

	valueWidth := width - ansi.StringWidth(labelText) - 1
	if valueWidth < 0 {
		valueWidth = 0
	}
	if valueWidth > 0 {
		value = truncateWithEllipsis(value, valueWidth)
	} else {
		value = ""
	}

	line := styles.SubtleStyle.Render(labelText)
	if value != "" {
		line += " " + lipgloss.NewStyle().Bold(true).Render(value)
	}
	return line
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
	var summaryLines []string
	summaryLines = append(summaryLines, styles.SubtleStyle.Render("Task Summary:"))
	for i, task := range m.tasks {
		indicator := m.getTaskIndicator(task.Status, false)
		summaryLines = append(summaryLines, fmt.Sprintf("%s %d. %s", indicator, i+1, task.Title))
	}
	b.WriteString(centerBlock(m.width, strings.Join(summaryLines, "\n")))
	b.WriteString("\n\n")

	// Options
	b.WriteString(m.renderCompletionOptions())
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
	var summaryLines []string
	summaryLines = append(summaryLines, styles.SubtleStyle.Render("Task Summary:"))
	for i, task := range m.tasks {
		indicator := m.getTaskIndicator(task.Status, false)
		summaryLines = append(summaryLines, fmt.Sprintf("%s %d. %s", indicator, i+1, task.Title))
	}
	b.WriteString(centerBlock(m.width, strings.Join(summaryLines, "\n")))
	b.WriteString("\n\n")

	// Options
	b.WriteString(m.renderCompletionOptions())
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

func (m RunningModel) renderCompletionOptions() string {
	options := []struct {
		shortcut string
		label    string
		selected bool
	}{
		{shortcut: "[Enter]", label: "Return to home", selected: true},
		{shortcut: "[q]", label: "Quit", selected: false},
	}

	maxShortcutWidth := 0
	for _, option := range options {
		width := lipgloss.Width(option.shortcut)
		if width > maxShortcutWidth {
			maxShortcutWidth = width
		}
	}

	lines := make([]string, 0, len(options))
	for _, option := range options {
		shortcut := padRight(option.shortcut, maxShortcutWidth)
		line := shortcut + " " + option.label
		if option.selected {
			line = styles.SelectedStyle.Render(shortcut) + " " + option.label
		} else {
			line = styles.SubtleStyle.Render(line)
		}
		lines = append(lines, line)
	}

	return centerBlock(m.width, strings.Join(lines, "\n"))
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
	if ansi.StringWidth(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return ansi.Truncate(s, maxLen, "")
	}
	return ansi.Truncate(s, maxLen, "...")
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
