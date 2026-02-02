package views

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/session"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
)

// ConversationState represents the current state of the conversation.
type ConversationState int

const (
	StateConversing ConversationState = iota
	StateReviewing
	StateWaitingApproval
	StateCompleted
	StateCancelled
)

// ActivityEntry represents a single item in the activity timeline.
type ActivityEntry struct {
	Text      string
	Timestamp time.Time
	Indent    int  // Nesting level for tree display
	IsDone    bool // Whether this activity is complete
}

// ConversationConfig holds initialization parameters.
type ConversationConfig struct {
	Phase      session.Phase
	Name       string
	FromDoc    string           // For design docs created from PRD
	ResumeFrom *session.Session // For resuming existing sessions
}

// ConversationModel handles the conversational document creation UI.
type ConversationModel struct {
	config ConversationConfig
	phase  session.Phase

	// Session management
	session      *session.Session
	conversation *ai.Conversation

	// State machine
	state ConversationState

	// Activity timeline (left pane)
	activities   []ActivityEntry
	activitiesMu sync.Mutex

	// Response content (main pane)
	responseText *strings.Builder
	responseView components.OutputViewport

	// Track Write tool targets for auto-review detection
	lastWritePath string

	// Input field
	input      textarea.Model
	inputFocus bool

	// UI state
	spinner    spinner.Model
	isThinking bool

	// Event channels
	eventChan chan ai.StreamEvent

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Dimensions
	width  int
	height int

	// Storage for session persistence (injected for testing)
	storage SessionStorage

	// Conversation starter (injected for testing)
	conversationStarter ConversationStarter
}

// SessionStorage interface for session persistence.
type SessionStorage interface {
	Save(session *session.Session) error
}

// ConversationStarter interface for starting conversations (allows mocking).
type ConversationStarter interface {
	Start(ctx context.Context, config ai.ConversationConfig) (*ai.Conversation, <-chan ai.StreamEvent, error)
}

// DefaultConversationStarter uses the real ai.StartConversation.
type DefaultConversationStarter struct{}

// Start implements ConversationStarter.
func (d DefaultConversationStarter) Start(ctx context.Context, config ai.ConversationConfig) (*ai.Conversation, <-chan ai.StreamEvent, error) {
	return ai.StartConversation(ctx, config)
}

// NewConversationModel creates a new conversation view.
func NewConversationModel(config ConversationConfig) ConversationModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to submit)"
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.Focus()

	ctx, cancel := context.WithCancel(context.Background())

	m := ConversationModel{
		config:              config,
		phase:               config.Phase,
		state:               StateConversing,
		spinner:             s,
		input:               ta,
		inputFocus:          true,
		eventChan:           make(chan ai.StreamEvent, 100),
		ctx:                 ctx,
		cancel:              cancel,
		responseText:        &strings.Builder{},
		responseView:        components.NewOutputViewport(80, 20, 0),
		conversationStarter: DefaultConversationStarter{},
	}

	// Initialize session
	if config.ResumeFrom != nil {
		m.session = config.ResumeFrom
		m.addActivity("Resuming session...", 0)
	} else {
		m.session = &session.Session{
			Phase:        config.Phase,
			Name:         config.Name,
			Status:       session.StatusInProgress,
			CreatedAt:    time.Now(),
			FromDocument: config.FromDoc,
		}
		m.addActivity("Starting session", 0)
	}

	return m
}

// SetStorage sets the session storage for persistence.
func (m *ConversationModel) SetStorage(s SessionStorage) {
	m.storage = s
}

// SetConversationStarter sets the conversation starter (for testing).
func (m *ConversationModel) SetConversationStarter(cs ConversationStarter) {
	m.conversationStarter = cs
}

// Init implements tea.Model.
func (m ConversationModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		textarea.Blink,
		m.startConversation(),
		m.listenForEvents(),
	)
}

// StreamEventMsg wraps a stream event for the Update loop.
type StreamEventMsg struct {
	Event ai.StreamEvent
}

// ConversationErrorMsg indicates an error occurred.
type ConversationErrorMsg struct {
	Err error
}

// ConversationStartedMsg indicates conversation started successfully.
type ConversationStartedMsg struct {
	Conv *ai.Conversation
}

// startConversation initiates the Claude conversation.
func (m *ConversationModel) startConversation() tea.Cmd {
	return func() tea.Msg {
		prompt := m.buildInitialPrompt()

		config := ai.ConversationConfig{
			SessionID:     m.session.SessionID,
			InitialPrompt: prompt,
			SkillName:     string(m.phase),
		}

		conv, events, err := m.conversationStarter.Start(m.ctx, config)
		if err != nil {
			return ConversationErrorMsg{Err: err}
		}

		// Forward events to our channel, handling backpressure
		go m.forwardEvents(events)

		return ConversationStartedMsg{Conv: conv}
	}
}

// forwardEvents reads from a response channel and forwards to the main event channel.
// Critical events (init, done) are never dropped; other events may be dropped if buffer is full.
func (m *ConversationModel) forwardEvents(events <-chan ai.StreamEvent) {
	for event := range events {
		// Critical events must not be dropped
		if event.Type == "init" || event.Type == "done" {
			m.eventChan <- event // Blocking send for critical events
			continue
		}

		// Non-critical events: drop if buffer full
		select {
		case m.eventChan <- event:
		default:
			// Log dropped events for debugging (in production, this is silent)
		}
	}
}

// buildInitialPrompt creates the prompt based on phase.
func (m *ConversationModel) buildInitialPrompt() string {
	switch m.phase {
	case session.PhasePRD:
		return "Use the /prd skill to help the user create a PRD. Guide them through defining the problem, users, and requirements."
	case session.PhaseDesign:
		if m.session.FromDocument != "" {
			return fmt.Sprintf("Use the /technical-design skill to create a technical design document based on the PRD at %s.", m.session.FromDocument)
		}
		return "Use the /technical-design skill to help the user create a technical design document."
	case session.PhasePlanCreate:
		return "Help the user create an execution plan from their design document."
	default:
		return ""
	}
}

// listenForEvents returns a command that waits for stream events.
func (m ConversationModel) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-m.eventChan:
			if !ok {
				return nil
			}
			return StreamEventMsg{Event: event}
		case <-m.ctx.Done():
			return nil
		}
	}
}

// Update implements tea.Model.
func (m ConversationModel) Update(msg tea.Msg) (ConversationModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil

	case spinner.TickMsg:
		if m.isThinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ConversationStartedMsg:
		m.conversation = msg.Conv
		m.isThinking = true
		return m, nil

	case StreamEventMsg:
		if cmd := m.handleStreamEvent(msg.Event); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForEvents())

	case ConversationErrorMsg:
		m.addActivity(fmt.Sprintf("Error: %v", msg.Err), 0)
		m.state = StateCancelled
		m.session.Status = session.StatusCancelled

	case tea.KeyMsg:
		newModel, cmd, handled := m.handleKeyPress(msg)
		if handled {
			return newModel, cmd
		}
		m = newModel
		// Pass unhandled keys to textarea
		if m.inputFocus && !m.isThinking && m.state == StateConversing {
			var inputCmd tea.Cmd
			m.input, inputCmd = m.input.Update(msg)
			return m, inputCmd
		}
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// handleStreamEvent processes events from Claude.
// Returns a tea.Cmd if an async action is needed.
func (m *ConversationModel) handleStreamEvent(event ai.StreamEvent) tea.Cmd {
	switch event.Type {
	case "init":
		// Capture session ID immediately for persistence
		if event.SessionID != "" {
			m.session.SessionID = event.SessionID
		}
		m.isThinking = true

	case "text":
		m.responseText.WriteString(event.Text)
		m.responseView.SetContent(m.responseText.String())

	case "tool_use":
		entry := m.formatToolUseEntry(event.ToolName, event.ToolTarget)
		m.addActivity(entry, 1)
		m.isThinking = true

		// Track Write targets for auto-review detection
		if event.ToolName == "Write" {
			m.lastWritePath = event.ToolTarget
		}

	case "tool_result":
		// Mark last activity as done
		m.markLastActivityDone()

	case "done":
		m.isThinking = false
		if event.SessionID != "" {
			m.session.SessionID = event.SessionID
		}

		// Check if this was a review phase
		if m.state == StateReviewing {
			m.state = StateWaitingApproval
			m.addActivity("Review complete", 0)
		} else if m.shouldAutoReview() {
			return m.triggerAutoReview()
		}

	case "error":
		errorText := event.Text
		if errorText == "" {
			errorText = "Unknown error"
		}
		m.addActivity(fmt.Sprintf("Error: %s", errorText), 0)
		m.isThinking = false
	}
	return nil
}

// formatToolUseEntry formats a tool use entry for the activity timeline.
// For Task tool, target is the subagent description; for other tools it's the file path or pattern.
func (m *ConversationModel) formatToolUseEntry(toolName, target string) string {
	entry := fmt.Sprintf("Using %s", toolName)
	if target != "" {
		entry += fmt.Sprintf(": %s", shortenPath(target, 30))
	}
	return entry
}

// handleKeyPress processes keyboard input.
// Returns (model, cmd, handled) - if handled is false, the key should be passed to textarea.
func (m ConversationModel) handleKeyPress(msg tea.KeyMsg) (ConversationModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c":
		if m.conversation != nil {
			m.conversation.Stop()
		}
		m.cancel()
		m.state = StateCancelled
		m.session.Status = session.StatusCancelled
		return m, nil, true

	case "a":
		if m.state == StateWaitingApproval {
			model, cmd := m.handleApprove()
			return model, cmd, true
		}

	case "c":
		if m.state == StateWaitingApproval {
			if m.conversation != nil {
				m.conversation.Stop()
			}
			m.cancel()
			m.state = StateCancelled
			m.session.Status = session.StatusCancelled
			return m, nil, true
		}

	case "m":
		// Return to home menu from completed or cancelled state
		if m.state == StateCompleted || m.state == StateCancelled {
			return m, m.returnToHomeCmd(), true
		}

	case "q":
		// Quit from completed or cancelled states
		if m.state == StateCompleted || m.state == StateCancelled {
			m.cancel()
			return m, tea.Quit, true
		}

	case "enter":
		if !m.isThinking && m.input.Value() != "" && m.state == StateConversing {
			model, cmd := m.sendMessage()
			return model, cmd, true
		}

	case "shift+enter", "ctrl+j":
		if m.state == StateConversing {
			m.input.InsertString("\n")
			return m, nil, true
		}
	}

	// Key not handled by us - let textarea process it
	return m, nil, false
}

// returnToHomeCmd returns a command to navigate back to the home menu.
func (m ConversationModel) returnToHomeCmd() tea.Cmd {
	return func() tea.Msg {
		return msgs.GoToHomeMsg{}
	}
}

// sendMessage sends user input to Claude.
func (m ConversationModel) sendMessage() (ConversationModel, tea.Cmd) {
	if m.conversation == nil {
		return m, nil
	}

	message := m.input.Value()
	m.input.Reset()

	m.addActivity(fmt.Sprintf("You: %s", truncate(message, 40)), 0)
	m.isThinking = true

	// Clear response view for new response
	m.responseText.Reset()
	m.responseView.Clear()

	return m, func() tea.Msg {
		events, err := m.conversation.SendMessage(message)
		if err != nil {
			return ConversationErrorMsg{Err: err}
		}
		// Forward events from this response
		go m.forwardEvents(events)
		return nil
	}
}

// shouldAutoReview determines if auto-review should trigger.
// Uses Write tool detection instead of fragile text matching.
func (m *ConversationModel) shouldAutoReview() bool {
	// Check if we detected a Write to docs/prds/ or docs/designs/
	return m.lastWritePath != "" && (strings.HasPrefix(m.lastWritePath, "docs/prds/") ||
		strings.HasPrefix(m.lastWritePath, "docs/designs/"))
}

// triggerAutoReview starts the automatic review phase.
// Returns a tea.Cmd to properly handle the async operation.
func (m *ConversationModel) triggerAutoReview() tea.Cmd {
	if m.conversation == nil {
		return nil
	}

	m.state = StateReviewing
	m.addActivity("Running automatic review...", 0)
	m.isThinking = true

	// Clear response view for review output
	m.responseText.Reset()
	m.responseView.Clear()

	var reviewPrompt string
	switch m.phase {
	case session.PhasePRD:
		reviewPrompt = "Now use /prd-review to review what you created. Address any critical issues you find."
	case session.PhaseDesign:
		reviewPrompt = "Now use /technical-design-review to review what you created. Address any critical issues you find."
	default:
		reviewPrompt = "Now review what you created and address any critical issues."
	}

	return func() tea.Msg {
		events, err := m.conversation.SendMessage(reviewPrompt)
		if err != nil {
			return ConversationErrorMsg{Err: err}
		}
		go m.forwardEvents(events)
		return nil
	}
}

// handleApprove processes the approval action.
// Saves document using the lastWritePath (or default pattern), persists session,
// and shows completion message with next steps.
func (m ConversationModel) handleApprove() (ConversationModel, tea.Cmd) {
	m.state = StateCompleted
	m.session.Status = session.StatusCompleted

	// Set document path from the last Write tool target
	if m.lastWritePath != "" {
		m.session.DocumentPath = m.lastWritePath
	}

	// Persist session if storage is available
	if m.storage != nil {
		m.storage.Save(m.session)
	}

	// Add completion activity with checkmark
	if m.session.DocumentPath != "" {
		m.addActivity(fmt.Sprintf("✓ Document saved to %s", m.session.DocumentPath), 0)
	} else {
		m.addActivity("✓ Session completed", 0)
	}

	return m, nil
}

// addActivity adds an entry to the activity timeline.
func (m *ConversationModel) addActivity(text string, indent int) {
	m.activitiesMu.Lock()
	defer m.activitiesMu.Unlock()
	m.activities = append(m.activities, ActivityEntry{
		Text:      text,
		Timestamp: time.Now(),
		Indent:    indent,
		IsDone:    false,
	})
}

// markLastActivityDone marks the last activity entry as done.
func (m *ConversationModel) markLastActivityDone() {
	m.activitiesMu.Lock()
	defer m.activitiesMu.Unlock()
	if len(m.activities) > 0 {
		m.activities[len(m.activities)-1].IsDone = true
	}
}

// updateLayout recalculates component sizes.
func (m *ConversationModel) updateLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	rightWidth := (m.width * 75 / 100) - 2

	// Height: total - title(2) - input(5) - action bar(1) - borders
	panelHeight := m.height - 10
	if panelHeight < 5 {
		panelHeight = 5
	}

	// Account for panel padding (0, 1) and border (1 char each side)
	// Padding: 1 left + 1 right = 2 chars
	// Border: 1 left + 1 right = 2 chars
	// Total: 4 chars less for content
	viewportWidth := rightWidth - 4
	if viewportWidth < 20 {
		viewportWidth = 20
	}

	// Account for "Response" header (1 line) + empty line (1 line) + border (2 lines)
	viewportHeight := panelHeight - 4
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	m.responseView.SetSize(viewportWidth, viewportHeight)
	// Account for InputStyle border (2) + padding (2) = 4 extra chars
	m.input.SetWidth(m.width - 8)
}

// View implements tea.Model.
func (m ConversationModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Title
	title := m.renderTitle()
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title))
	b.WriteString("\n")

	// Calculate panel dimensions
	leftWidth := (m.width * 25 / 100) - 2
	rightWidth := (m.width * 75 / 100) - 2
	panelHeight := m.height - 10
	if panelHeight < 5 {
		panelHeight = 5
	}

	// Main panels
	leftPanel := m.renderActivityPanel(leftWidth, panelHeight)
	rightPanel := m.renderResponsePanel(rightWidth, panelHeight)

	leftStyle := styles.BoxStyle.Copy().
		Width(leftWidth).
		Height(panelHeight).
		Padding(0, 1)

	rightStyle := styles.BoxStyle.Copy().
		Width(rightWidth).
		Height(panelHeight).
		Padding(0, 1)

	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(leftPanel),
		rightStyle.Render(rightPanel),
	)
	b.WriteString(panels)
	b.WriteString("\n")

	// Input field
	b.WriteString(m.renderInput())
	b.WriteString("\n")

	// Action bar
	b.WriteString(m.renderActionBar())

	return b.String()
}

// renderTitle returns the title bar.
func (m ConversationModel) renderTitle() string {
	var phase string
	switch m.phase {
	case session.PhasePRD:
		phase = "Creating PRD"
	case session.PhaseDesign:
		phase = "Creating Design"
	case session.PhasePlanCreate:
		phase = "Creating Plan"
	}

	if m.session.Name != "" {
		phase += ": " + m.session.Name
	}

	return styles.TitleStyle.Render("Rafa - " + phase)
}

// renderActivityPanel returns the activity timeline view.
func (m ConversationModel) renderActivityPanel(width, height int) string {
	var lines []string
	lines = append(lines, styles.SubtleStyle.Render("Activity"))
	lines = append(lines, "")

	m.activitiesMu.Lock()
	activities := make([]ActivityEntry, len(m.activities))
	copy(activities, m.activities)
	m.activitiesMu.Unlock()

	for i, entry := range activities {
		prefix := ""
		if entry.Indent > 0 {
			prefix = "├─ "
		}

		indicator := "○"
		if entry.IsDone {
			indicator = styles.SuccessStyle.Render("✓")
		} else if i == len(activities)-1 && m.isThinking {
			indicator = m.spinner.View()
		}

		line := fmt.Sprintf("%s%s %s", prefix, indicator, entry.Text)
		// Truncate if too long
		if len(line) > width {
			line = line[:width-3] + "..."
		}
		lines = append(lines, line)
	}

	// Pad to height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// renderResponsePanel returns the Claude response view.
func (m ConversationModel) renderResponsePanel(width, height int) string {
	var lines []string
	lines = append(lines, styles.SubtleStyle.Render("Response"))
	lines = append(lines, "")

	// Render output viewport
	outputView := m.responseView.View()
	lines = append(lines, outputView)

	return strings.Join(lines, "\n")
}

// renderInput returns the input field.
func (m ConversationModel) renderInput() string {
	if m.isThinking {
		return styles.SubtleStyle.Render("Waiting for Claude...")
	}

	if m.state == StateWaitingApproval {
		return styles.SubtleStyle.Render("Review complete. Approve or revise.")
	}

	if m.state == StateCompleted {
		return m.renderCompletionMessage()
	}

	if m.state == StateCancelled {
		return styles.SubtleStyle.Render("Session cancelled.")
	}

	inputStyle := styles.InputStyle.Copy().Width(m.width - 4)
	return inputStyle.Render(m.input.View())
}

// renderCompletionMessage returns the completion message with document path and next steps.
func (m ConversationModel) renderCompletionMessage() string {
	var lines []string

	// Show document saved message with checkmark
	if m.session.DocumentPath != "" {
		savedMsg := fmt.Sprintf("✓ %s saved to %s", m.phaseDocumentType(), m.session.DocumentPath)
		lines = append(lines, styles.SuccessStyle.Render(savedMsg))
	} else {
		lines = append(lines, styles.SuccessStyle.Render("✓ Session completed!"))
	}

	// Show next steps based on phase
	lines = append(lines, "")
	lines = append(lines, m.renderNextSteps())

	return strings.Join(lines, "\n")
}

// phaseDocumentType returns a human-readable document type for the current phase.
func (m ConversationModel) phaseDocumentType() string {
	switch m.phase {
	case session.PhasePRD:
		return "PRD"
	case session.PhaseDesign:
		return "Design"
	case session.PhasePlanCreate:
		return "Plan"
	default:
		return "Document"
	}
}

// renderNextSteps returns next steps guidance based on the current phase.
func (m ConversationModel) renderNextSteps() string {
	var steps []string

	switch m.phase {
	case session.PhasePRD:
		steps = append(steps, "Next: Create a technical design with 'rafa design'")
	case session.PhaseDesign:
		steps = append(steps, "Next: Create an execution plan with 'rafa plan create'")
	case session.PhasePlanCreate:
		steps = append(steps, "Next: Run the plan with 'rafa plan run'")
	}

	return styles.SubtleStyle.Render(strings.Join(steps, "\n"))
}

// renderActionBar returns the bottom action bar.
func (m ConversationModel) renderActionBar() string {
	var items []string

	switch m.state {
	case StateWaitingApproval:
		items = []string{"[a] Approve", "[c] Cancel", "(type to revise)"}
	case StateCompleted:
		items = []string{"✓ Complete", "[m] Menu", "[q] Quit"}
	case StateCancelled:
		items = []string{"Cancelled", "[m] Menu", "[q] Quit"}
	default:
		items = []string{"Enter Submit", "Ctrl+C Cancel"}
	}

	return components.NewStatusBar().Render(m.width, items)
}

// SetSize updates dimensions.
func (m *ConversationModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.updateLayout()
}

// State returns the current state.
func (m ConversationModel) State() ConversationState {
	return m.state
}

// Session returns the current session.
func (m ConversationModel) Session() *session.Session {
	return m.session
}

// Activities returns a copy of the activities.
func (m ConversationModel) Activities() []ActivityEntry {
	m.activitiesMu.Lock()
	defer m.activitiesMu.Unlock()
	result := make([]ActivityEntry, len(m.activities))
	copy(result, m.activities)
	return result
}

// IsThinking returns whether the model is waiting for Claude.
func (m ConversationModel) IsThinking() bool {
	return m.isThinking
}

// Helper functions

// shortenPath truncates a path to show only the last parts if it exceeds maxLen.
// For paths > maxLen, shows ".../last/two.go"
func shortenPath(path string, maxLen int) string {
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
		// If still too long, just truncate
		return path[:maxLen-3] + "..."
	}
	return path[:maxLen-3] + "..."
}

// truncate shortens a string to max length with ellipsis.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
