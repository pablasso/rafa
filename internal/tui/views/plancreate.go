package views

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
	"github.com/pablasso/rafa/internal/util"
)

// PlanCreateState represents the current state of the plan creation flow.
type PlanCreateState int

const (
	PlanCreateStateInstructions PlanCreateState = iota // Initial state: user can enter instructions
	PlanCreateStateExtracting                          // Claude is extracting tasks
	PlanCreateStateConversing                          // User can refine tasks
	PlanCreateStateCompleted                           // Plan saved successfully
	PlanCreateStateCancelled                           // User cancelled
	PlanCreateStateError                               // Error occurred
)

// Note: Uses ActivityEntry type defined in conversation.go (same package)

// PlanCreateModel handles the conversational plan creation UI.
type PlanCreateModel struct {
	sourceFile string // Path to design document

	// State machine
	state PlanCreateState

	// Activity timeline (left pane)
	activities   []ActivityEntry
	activitiesMu sync.Mutex

	// Response content (main pane)
	responseText strings.Builder
	responseView components.OutputViewport

	// Input field
	input      textarea.Model
	inputFocus bool

	// UI state
	spinner    spinner.Model
	isThinking bool
	errorMsg   string

	// Conversation state
	conversation *ai.Conversation
	eventChan    chan ai.StreamEvent

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Extracted plan data
	extractedPlan *plan.TaskExtractionResult
	savedPlanID   string // Set after successful save

	// Dimensions
	width  int
	height int

	// Conversation starter (injected for testing)
	conversationStarter ConversationStarter
}

// NewPlanCreateModel creates a new plan creation model.
func NewPlanCreateModel(sourceFile string) PlanCreateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	ta := textarea.New()
	ta.Placeholder = "Optional: Enter any instructions or constraints for task extraction..."
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Focus()

	ctx, cancel := context.WithCancel(context.Background())

	return PlanCreateModel{
		sourceFile:          sourceFile,
		state:               PlanCreateStateInstructions,
		spinner:             s,
		input:               ta,
		inputFocus:          true,
		eventChan:           make(chan ai.StreamEvent, 100),
		ctx:                 ctx,
		cancel:              cancel,
		responseView:        components.NewOutputViewport(80, 20, 0),
		conversationStarter: DefaultConversationStarter{},
	}
}

// SetConversationStarter sets the conversation starter (for testing).
func (m *PlanCreateModel) SetConversationStarter(cs ConversationStarter) {
	m.conversationStarter = cs
}

// Init implements tea.Model.
func (m PlanCreateModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		textarea.Blink,
	)
}

// PlanCreateStartMsg signals the extraction should begin.
type PlanCreateStartMsg struct{}

// PlanCreateConversationStartedMsg indicates the conversation started.
type PlanCreateConversationStartedMsg struct {
	Conv *ai.Conversation
}

// PlanCreateStreamEventMsg wraps stream events.
type PlanCreateStreamEventMsg struct {
	Event ai.StreamEvent
}

// PlanCreateErrorMsg indicates an error occurred.
type PlanCreateErrorMsg struct {
	Err error
}

// PlanCreateSavedMsg indicates the plan was saved successfully.
type PlanCreateSavedMsg struct {
	PlanID string
	Tasks  []string
}

// startExtraction begins the conversational extraction.
func (m *PlanCreateModel) startExtraction(instructions string) tea.Cmd {
	return func() tea.Msg {
		// Read the design document
		content, err := os.ReadFile(m.sourceFile)
		if err != nil {
			return PlanCreateErrorMsg{Err: fmt.Errorf("failed to read design file: %w", err)}
		}

		prompt := m.buildExtractionPrompt(string(content), instructions)

		config := ai.ConversationConfig{
			InitialPrompt: prompt,
		}

		conv, events, err := m.conversationStarter.Start(m.ctx, config)
		if err != nil {
			return PlanCreateErrorMsg{Err: err}
		}

		// Forward events to our channel
		go m.forwardEvents(events)

		return PlanCreateConversationStartedMsg{Conv: conv}
	}
}

// buildExtractionPrompt creates the prompt for task extraction.
func (m *PlanCreateModel) buildExtractionPrompt(designContent, instructions string) string {
	var sb strings.Builder
	sb.WriteString(`You are helping create an execution plan from a technical design document.

DESIGN DOCUMENT:
`)
	sb.WriteString(designContent)
	sb.WriteString(`

`)
	if instructions != "" {
		sb.WriteString("USER INSTRUCTIONS:\n")
		sb.WriteString(instructions)
		sb.WriteString("\n\n")
	}

	sb.WriteString(`Extract discrete implementation tasks from this design document and present them to the user.

For each task, show:
1. Task number and title
2. Brief description
3. Acceptance criteria (as a bulleted list)

Present the tasks in implementation order. Size each task to be completable by an AI agent in a single session (roughly 50-60% of context window).

After presenting the tasks, ask the user if they want to:
- Approve the plan as-is
- Modify any tasks (split, merge, add, remove, or edit)
- Add more acceptance criteria
- Reorder tasks

Be conversational and helpful. When the user says they approve or are satisfied, respond with ONLY the following JSON (no other text):

PLAN_APPROVED_JSON:
{
  "name": "kebab-case-plan-name",
  "description": "One sentence description",
  "tasks": [
    {
      "title": "Task title",
      "description": "Detailed description",
      "acceptanceCriteria": ["criterion 1", "criterion 2"]
    }
  ]
}

Do not include the JSON until the user explicitly approves.`)

	return sb.String()
}

// forwardEvents reads from a response channel and forwards to the main event channel.
func (m *PlanCreateModel) forwardEvents(events <-chan ai.StreamEvent) {
	for event := range events {
		// Critical events must not be dropped
		if event.Type == "init" || event.Type == "done" {
			m.eventChan <- event
			continue
		}

		// Non-critical events: drop if buffer full
		select {
		case m.eventChan <- event:
		default:
		}
	}
}

// listenForEvents returns a command that waits for stream events.
func (m PlanCreateModel) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-m.eventChan:
			if !ok {
				return nil
			}
			return PlanCreateStreamEventMsg{Event: event}
		case <-m.ctx.Done():
			return nil
		}
	}
}

// Update implements tea.Model.
func (m PlanCreateModel) Update(msg tea.Msg) (PlanCreateModel, tea.Cmd) {
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

	case PlanCreateConversationStartedMsg:
		m.conversation = msg.Conv
		m.isThinking = true
		return m, m.listenForEvents()

	case PlanCreateStreamEventMsg:
		if cmd := m.handleStreamEvent(msg.Event); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForEvents())

	case PlanCreateErrorMsg:
		m.state = PlanCreateStateError
		m.errorMsg = msg.Err.Error()
		m.isThinking = false
		m.addActivity(fmt.Sprintf("Error: %v", msg.Err), 0)

	case PlanCreateSavedMsg:
		m.state = PlanCreateStateCompleted
		m.savedPlanID = msg.PlanID
		m.addActivity(fmt.Sprintf("✓ Plan saved: %s", msg.PlanID), 0)

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	// Update textarea
	if m.inputFocus && !m.isThinking && (m.state == PlanCreateStateInstructions || m.state == PlanCreateStateConversing) {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleStreamEvent processes events from Claude.
func (m *PlanCreateModel) handleStreamEvent(event ai.StreamEvent) tea.Cmd {
	switch event.Type {
	case "init":
		m.isThinking = true

	case "text":
		m.responseText.WriteString(event.Text)
		m.responseView.AddLine(event.Text)

		// Check if Claude has sent the approved plan JSON
		if strings.Contains(m.responseText.String(), "PLAN_APPROVED_JSON:") {
			if result := m.tryParseApprovedPlan(); result != nil {
				m.extractedPlan = result
				// Auto-save the plan
				return m.savePlan()
			}
		}

	case "tool_use":
		entry := fmt.Sprintf("Using %s", event.ToolName)
		if event.ToolTarget != "" {
			entry += fmt.Sprintf(": %s", shortenPath(event.ToolTarget, 30))
		}
		m.addActivity(entry, 1)
		m.isThinking = true

	case "tool_result":
		m.markLastActivityDone()

	case "done":
		m.isThinking = false
		if m.state == PlanCreateStateExtracting {
			m.state = PlanCreateStateConversing
			m.input.Placeholder = "Type to refine tasks, or say 'approve' when ready..."
			m.addActivity("Tasks extracted", 0)
		}
	}
	return nil
}

// tryParseApprovedPlan attempts to parse the approved plan JSON from the response.
func (m *PlanCreateModel) tryParseApprovedPlan() *plan.TaskExtractionResult {
	text := m.responseText.String()
	marker := "PLAN_APPROVED_JSON:"
	idx := strings.LastIndex(text, marker)
	if idx == -1 {
		return nil
	}

	jsonStart := idx + len(marker)
	jsonText := strings.TrimSpace(text[jsonStart:])

	// Find the JSON object
	start := strings.Index(jsonText, "{")
	if start == -1 {
		return nil
	}

	// Find matching closing brace
	depth := 0
	end := -1
	for i := start; i < len(jsonText); i++ {
		switch jsonText[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end != -1 {
			break
		}
	}

	if end == -1 {
		return nil
	}

	jsonStr := jsonText[start:end]

	var result plan.TaskExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}

	if err := result.Validate(); err != nil {
		return nil
	}

	return &result
}

// savePlan saves the extracted plan using existing infrastructure.
func (m *PlanCreateModel) savePlan() tea.Cmd {
	return func() tea.Msg {
		// Check for cancellation
		if m.ctx.Err() != nil {
			return PlanCreateErrorMsg{Err: fmt.Errorf("cancelled")}
		}

		if m.extractedPlan == nil {
			return PlanCreateErrorMsg{Err: fmt.Errorf("no plan to save")}
		}

		// Generate plan ID
		id, err := util.GenerateShortID()
		if err != nil {
			return PlanCreateErrorMsg{Err: fmt.Errorf("failed to generate plan ID: %w", err)}
		}

		// Determine plan name
		baseName := m.extractedPlan.Name
		if baseName == "" {
			base := filepath.Base(m.sourceFile)
			baseName = strings.TrimSuffix(base, filepath.Ext(base))
		}
		baseName = util.ToKebabCase(baseName)

		// Resolve name collisions
		name, err := plan.ResolvePlanName(baseName)
		if err != nil {
			return PlanCreateErrorMsg{Err: fmt.Errorf("failed to resolve plan name: %w", err)}
		}

		// Build tasks
		tasks := make([]plan.Task, len(m.extractedPlan.Tasks))
		taskTitles := make([]string, len(m.extractedPlan.Tasks))
		for i, et := range m.extractedPlan.Tasks {
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
			Description: m.extractedPlan.Description,
			SourceFile:  sourcePath,
			CreatedAt:   time.Now(),
			Status:      plan.PlanStatusNotStarted,
			Tasks:       tasks,
		}

		// Create the plan folder
		if err := plan.CreatePlanFolder(p); err != nil {
			return PlanCreateErrorMsg{Err: fmt.Errorf("failed to create plan: %w", err)}
		}

		planID := fmt.Sprintf("%s-%s", p.ID, p.Name)
		return PlanCreateSavedMsg{
			PlanID: planID,
			Tasks:  taskTitles,
		}
	}
}

// handleKeyPress processes keyboard input.
func (m PlanCreateModel) handleKeyPress(msg tea.KeyMsg) (PlanCreateModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.conversation != nil {
			m.conversation.Stop()
		}
		m.cancel()
		m.state = PlanCreateStateCancelled
		return m, nil

	case "ctrl+enter":
		switch m.state {
		case PlanCreateStateInstructions:
			// Start extraction with optional instructions
			instructions := m.input.Value()
			m.state = PlanCreateStateExtracting
			m.isThinking = true
			m.addActivity("Starting task extraction...", 0)
			if instructions != "" {
				m.addActivity(fmt.Sprintf("Instructions: %s", truncate(instructions, 40)), 1)
			}
			m.input.Reset()
			m.input.Placeholder = "Type to refine tasks, or say 'approve' when ready..."
			return m, m.startExtraction(instructions)

		case PlanCreateStateConversing:
			if !m.isThinking && m.input.Value() != "" {
				return m.sendMessage()
			}
		}

	case "enter":
		// In instructions state, Enter alone also starts extraction (for convenience)
		if m.state == PlanCreateStateInstructions && !m.isThinking {
			instructions := m.input.Value()
			m.state = PlanCreateStateExtracting
			m.isThinking = true
			m.addActivity("Starting task extraction...", 0)
			if instructions != "" {
				m.addActivity(fmt.Sprintf("Instructions: %s", truncate(instructions, 40)), 1)
			}
			m.input.Reset()
			m.input.Placeholder = "Type to refine tasks, or say 'approve' when ready..."
			return m, m.startExtraction(instructions)
		}

	case "r":
		if m.state == PlanCreateStateCompleted && m.savedPlanID != "" {
			return m, func() tea.Msg { return msgs.RunPlanMsg{PlanID: m.savedPlanID} }
		}

	case "h", "m":
		if m.state == PlanCreateStateCompleted || m.state == PlanCreateStateCancelled || m.state == PlanCreateStateError {
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		}

	case "q":
		if m.state == PlanCreateStateCompleted || m.state == PlanCreateStateCancelled || m.state == PlanCreateStateError {
			m.cancel()
			return m, tea.Quit
		}
	}

	return m, nil
}

// sendMessage sends user input to Claude for task refinement.
func (m PlanCreateModel) sendMessage() (PlanCreateModel, tea.Cmd) {
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
			return PlanCreateErrorMsg{Err: err}
		}
		go m.forwardEvents(events)
		return nil
	}
}

// addActivity adds an entry to the activity timeline.
func (m *PlanCreateModel) addActivity(text string, indent int) {
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
func (m *PlanCreateModel) markLastActivityDone() {
	m.activitiesMu.Lock()
	defer m.activitiesMu.Unlock()
	if len(m.activities) > 0 {
		m.activities[len(m.activities)-1].IsDone = true
	}
}

// updateLayout recalculates component sizes.
func (m *PlanCreateModel) updateLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	rightWidth := (m.width * 75 / 100) - 2

	// Height: total - title(2) - input(5) - action bar(1) - borders
	panelHeight := m.height - 10
	if panelHeight < 5 {
		panelHeight = 5
	}

	m.responseView.SetSize(rightWidth, panelHeight)
	m.input.SetWidth(m.width - 4)
}

// View implements tea.Model.
func (m PlanCreateModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Create Plan")
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

// renderActivityPanel returns the activity timeline view.
func (m PlanCreateModel) renderActivityPanel(width, height int) string {
	var lines []string
	lines = append(lines, styles.SubtleStyle.Render("Activity"))
	lines = append(lines, "")

	// Show source file
	sourceDisplay := shortenPath(m.sourceFile, width-4)
	lines = append(lines, styles.SubtleStyle.Render("Source: "+sourceDisplay))
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
func (m PlanCreateModel) renderResponsePanel(width, height int) string {
	var lines []string
	lines = append(lines, styles.SubtleStyle.Render("Response"))
	lines = append(lines, "")

	// Render output viewport
	outputView := m.responseView.View()
	lines = append(lines, outputView)

	return strings.Join(lines, "\n")
}

// renderInput returns the input field.
func (m PlanCreateModel) renderInput() string {
	switch m.state {
	case PlanCreateStateInstructions:
		return m.input.View() + "\n" + styles.SubtleStyle.Render("Press Enter to start extraction (or type instructions first)")

	case PlanCreateStateExtracting:
		return styles.SubtleStyle.Render("Extracting tasks from design document...")

	case PlanCreateStateConversing:
		if m.isThinking {
			return styles.SubtleStyle.Render("Waiting for Claude...")
		}
		return m.input.View()

	case PlanCreateStateCompleted:
		return m.renderCompletionMessage()

	case PlanCreateStateCancelled:
		return styles.SubtleStyle.Render("Plan creation cancelled.")

	case PlanCreateStateError:
		return styles.ErrorStyle.Render(fmt.Sprintf("Error: %s", m.errorMsg))
	}

	return ""
}

// renderCompletionMessage shows success and next steps.
func (m PlanCreateModel) renderCompletionMessage() string {
	var lines []string
	lines = append(lines, styles.SuccessStyle.Render(fmt.Sprintf("✓ Plan saved: %s", m.savedPlanID)))
	lines = append(lines, "")
	lines = append(lines, styles.SubtleStyle.Render("Next: Run the plan with 'rafa plan run' or press [r]"))
	return strings.Join(lines, "\n")
}

// renderActionBar returns the bottom action bar.
func (m PlanCreateModel) renderActionBar() string {
	var items []string

	switch m.state {
	case PlanCreateStateInstructions:
		items = []string{"Enter Start", "Ctrl+C Cancel"}
	case PlanCreateStateExtracting:
		items = []string{"Extracting...", "Ctrl+C Cancel"}
	case PlanCreateStateConversing:
		items = []string{"Ctrl+Enter Send", "Ctrl+C Cancel"}
	case PlanCreateStateCompleted:
		items = []string{"✓ Complete", "[r] Run", "[h] Home", "[q] Quit"}
	case PlanCreateStateCancelled, PlanCreateStateError:
		items = []string{"[h] Home", "[q] Quit"}
	}

	return components.NewStatusBar().Render(m.width, items)
}

// SetSize updates dimensions.
func (m *PlanCreateModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.updateLayout()
}

// State returns the current state.
func (m PlanCreateModel) State() PlanCreateState {
	return m.state
}

// SourceFile returns the source file path.
func (m PlanCreateModel) SourceFile() string {
	return m.sourceFile
}

// SavedPlanID returns the saved plan ID (empty if not saved yet).
func (m PlanCreateModel) SavedPlanID() string {
	return m.savedPlanID
}

// Activities returns a copy of the activities.
func (m PlanCreateModel) Activities() []ActivityEntry {
	m.activitiesMu.Lock()
	defer m.activitiesMu.Unlock()
	result := make([]ActivityEntry, len(m.activities))
	copy(result, m.activities)
	return result
}

// IsThinking returns whether the model is waiting for Claude.
func (m PlanCreateModel) IsThinking() bool {
	return m.isThinking
}
