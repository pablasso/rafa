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
	PlanCreateStateExtracting PlanCreateState = iota // Claude is extracting tasks
	PlanCreateStateCompleted                         // Extraction completed (saved or demo-only)
	PlanCreateStateCancelled                         // User cancelled
	PlanCreateStateError                             // Error occurred
)

// PlanCreateMode controls whether plan creation is real or demo-unsaved.
type PlanCreateMode int

const (
	PlanCreateModeReal PlanCreateMode = iota
	PlanCreateModeDemoUnsaved
)

// ActivityEntry represents a single item in the activity timeline.
type ActivityEntry struct {
	Text      string
	Timestamp time.Time
	Indent    int  // Nesting level for tree display
	IsDone    bool // Whether this activity is complete
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

// PlanCreateModel handles the conversational plan creation UI.
type PlanCreateModel struct {
	sourceFile string // Path to design document

	// State machine
	state PlanCreateState

	// Activity timeline (left pane)
	activities   []ActivityEntry
	activitiesMu sync.Mutex

	// Response content (main pane)
	responseText *strings.Builder
	responseView components.OutputViewport

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

	// Mode and demo metadata
	mode    PlanCreateMode
	warning string
}

// NewPlanCreateModel creates a new plan creation model.
func NewPlanCreateModel(sourceFile string) PlanCreateModel {
	return newPlanCreateModel(sourceFile, PlanCreateModeReal, DefaultConversationStarter{}, "")
}

// NewPlanCreateModelForDemoUnsaved creates a plan-create model backed by replayed events.
func NewPlanCreateModelForDemoUnsaved(sourceFile string, starter ConversationStarter, warning string) PlanCreateModel {
	if starter == nil {
		starter = DefaultConversationStarter{}
	}
	return newPlanCreateModel(sourceFile, PlanCreateModeDemoUnsaved, starter, warning)
}

func newPlanCreateModel(sourceFile string, mode PlanCreateMode, starter ConversationStarter, warning string) PlanCreateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	ctx, cancel := context.WithCancel(context.Background())

	return PlanCreateModel{
		sourceFile:          sourceFile,
		state:               PlanCreateStateExtracting,
		spinner:             s,
		isThinking:          true,
		eventChan:           make(chan ai.StreamEvent, 100),
		ctx:                 ctx,
		cancel:              cancel,
		responseText:        &strings.Builder{},
		responseView:        components.NewOutputViewport(80, 20, 0),
		activities:          []ActivityEntry{{Text: "Starting task extraction...", Timestamp: time.Now(), Indent: 0, IsDone: false}},
		conversationStarter: starter,
		mode:                mode,
		warning:             warning,
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
		m.startExtraction(),
	)
}

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

// startExtraction begins the extraction conversation.
func (m *PlanCreateModel) startExtraction() tea.Cmd {
	return func() tea.Msg {
		// Read the design document
		content, err := os.ReadFile(m.sourceFile)
		if err != nil && m.mode == PlanCreateModeReal {
			return PlanCreateErrorMsg{Err: fmt.Errorf("failed to read design file: %w", err)}
		}

		designContent := string(content)
		if err != nil && m.mode == PlanCreateModeDemoUnsaved {
			designContent = fmt.Sprintf("# Demo source context\n\nSource file: %s", m.sourceFile)
		}

		prompt := m.buildExtractionPrompt(designContent)

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
func (m *PlanCreateModel) buildExtractionPrompt(designContent string) string {
	var sb strings.Builder
	sb.WriteString(`You are helping create an execution plan from a technical design document.

DESIGN DOCUMENT:
`)
	sb.WriteString(designContent)
	sb.WriteString(`

`)
	sb.WriteString(`Extract discrete implementation tasks from this design document.

For each task, show:
1. Task number and title
2. Brief description
3. Acceptance criteria (as a bulleted list)

Present the tasks in implementation order. Size each task to be completable by an AI agent in a single session (roughly 50-60% of context window).

Respond with ONLY the following JSON payload prefixed by PLAN_APPROVED_JSON: (no additional text):

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

Requirements:
- The response must include PLAN_APPROVED_JSON:
- Return valid JSON only after the marker
- Include at least one task
- Every task must include non-empty title and at least one acceptance criterion`)

	return sb.String()
}

// forwardEvents reads from a response channel and forwards to the main event channel.
func (m *PlanCreateModel) forwardEvents(events <-chan ai.StreamEvent) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			// Critical events must not be dropped while active, but should not block after cancellation.
			if event.Type == "init" || event.Type == "done" {
				select {
				case m.eventChan <- event:
				case <-m.ctx.Done():
					return
				}
				continue
			}

			// Non-critical events: drop if buffer full.
			select {
			case m.eventChan <- event:
			case <-m.ctx.Done():
				return
			default:
			}
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
		m.addActivity(fmt.Sprintf("✓ Plan created: %s", msg.PlanID), 0)
		m.stopExtractionSession()
		return m, nil

	case tea.KeyMsg:
		newModel, cmd, handled := m.handleKeyPress(msg)
		if handled {
			return newModel, cmd
		}
		return newModel, nil
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
		m.responseView.SetContent(m.responseText.String())

		// Check if Claude has sent the approved plan JSON
		if m.state == PlanCreateStateExtracting && strings.Contains(m.responseText.String(), "PLAN_APPROVED_JSON:") {
			if result := m.tryParseApprovedPlan(); result != nil {
				m.extractedPlan = result

				if m.mode == PlanCreateModeDemoUnsaved {
					m.state = PlanCreateStateCompleted
					m.savedPlanID = ""
					m.addActivity("✓ Demo extraction complete (plan not saved)", 0)
					m.stopExtractionSession()
					return nil
				}

				// Auto-save the plan in real mode.
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
		if m.state == PlanCreateStateExtracting && m.savedPlanID == "" && m.extractedPlan == nil {
			m.state = PlanCreateStateError
			m.errorMsg = "Extraction finished without a valid plan JSON. Press 'r' to retry."
			m.addActivity("Extraction failed: no valid plan JSON", 0)
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
// Returns (model, cmd, handled).
func (m PlanCreateModel) handleKeyPress(msg tea.KeyMsg) (PlanCreateModel, tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		if m.state == PlanCreateStateCompleted && m.mode == PlanCreateModeReal {
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }, true
		}

	case "ctrl+c":
		if m.state == PlanCreateStateCompleted && m.mode == PlanCreateModeReal {
			return m, nil, true
		}
		m.stopExtractionSession()
		m.state = PlanCreateStateCancelled
		return m, nil, true

	case "r":
		if m.state == PlanCreateStateError {
			m.resetForRetry()
			return m, m.startExtraction(), true
		}
		if m.state == PlanCreateStateCompleted && m.mode == PlanCreateModeDemoUnsaved {
			m.resetForRetry()
			return m, m.startExtraction(), true
		}

	case "h", "m":
		if m.state == PlanCreateStateCompleted && m.mode == PlanCreateModeReal {
			return m, nil, true
		}
		if m.state == PlanCreateStateCompleted || m.state == PlanCreateStateCancelled || m.state == PlanCreateStateError {
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }, true
		}

	case "q":
		if m.state == PlanCreateStateCompleted && m.mode == PlanCreateModeReal {
			return m, nil, true
		}
		if m.state == PlanCreateStateCompleted || m.state == PlanCreateStateCancelled || m.state == PlanCreateStateError {
			m.cancel()
			return m, tea.Quit, true
		}
	}

	// Key not handled by us.
	return m, nil, false
}

// stopExtractionSession stops any active conversation and cancels extraction context.
func (m *PlanCreateModel) stopExtractionSession() {
	if m.conversation != nil {
		_ = m.conversation.Stop()
		m.conversation = nil
	}
	m.cancel()
	m.isThinking = false
}

// resetForRetry clears extraction state and prepares for another attempt.
func (m *PlanCreateModel) resetForRetry() {
	m.stopExtractionSession()
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.state = PlanCreateStateExtracting
	m.errorMsg = ""
	m.isThinking = true
	m.extractedPlan = nil
	m.savedPlanID = ""
	m.responseText.Reset()
	m.responseView.Clear()
	m.addActivity("Retrying task extraction...", 0)
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

	// Height: total - title(2) - status(1) - action bar(1) - spacing/borders
	panelHeight := m.height - 6
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
	panelHeight := m.height - 6
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

	// Status line
	b.WriteString(m.renderStatusLine())
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

// renderStatusLine returns current flow status.
func (m PlanCreateModel) renderStatusLine() string {
	switch m.state {
	case PlanCreateStateExtracting:
		if m.mode == PlanCreateModeDemoUnsaved {
			lines := []string{
				styles.SubtleStyle.Render("Replaying create-plan demo (plan will not be saved)."),
			}
			if m.warning != "" {
				lines = append(lines, styles.SubtleStyle.Render(m.warning))
			}
			return strings.Join(lines, "\n")
		}
		return styles.SubtleStyle.Render("Extracting tasks from design document...")

	case PlanCreateStateCompleted:
		if m.mode == PlanCreateModeDemoUnsaved {
			return m.renderDemoCompletionMessage()
		}
		return m.renderRealCompletionMessage()

	case PlanCreateStateCancelled:
		return styles.SubtleStyle.Render("Plan creation cancelled.")

	case PlanCreateStateError:
		return styles.ErrorStyle.Render(fmt.Sprintf("Error: %s", m.errorMsg))
	}

	return ""
}

// renderCompletionMessage shows success and next steps.
func (m PlanCreateModel) renderRealCompletionMessage() string {
	var lines []string
	lines = append(lines, styles.SuccessStyle.Render(fmt.Sprintf("✓ Plan created: %s", m.savedPlanID)))
	lines = append(lines, styles.SubtleStyle.Render("You can run it anytime from Home > Run Plan."))
	return strings.Join(lines, "\n")
}

func (m PlanCreateModel) renderDemoCompletionMessage() string {
	var lines []string
	lines = append(lines, styles.SuccessStyle.Render("✓ Demo extraction complete"))
	if m.extractedPlan != nil {
		lines = append(lines, styles.SubtleStyle.Render(fmt.Sprintf("%d tasks extracted (demo only, not saved).", len(m.extractedPlan.Tasks))))
	} else {
		lines = append(lines, styles.SubtleStyle.Render("Demo complete (no files written)."))
	}
	if m.warning != "" {
		lines = append(lines, styles.SubtleStyle.Render(m.warning))
	}
	return strings.Join(lines, "\n")
}

// renderActionBar returns the bottom action bar.
func (m PlanCreateModel) renderActionBar() string {
	var items []string

	switch m.state {
	case PlanCreateStateExtracting:
		if m.mode == PlanCreateModeDemoUnsaved {
			items = []string{"Replaying demo...", "Ctrl+C Cancel"}
		} else {
			items = []string{"Extracting...", "Ctrl+C Cancel"}
		}
	case PlanCreateStateCompleted:
		if m.mode == PlanCreateModeDemoUnsaved {
			items = []string{"✓ Demo Complete", "Not saved", "[r] Replay", "[h] Home", "[q] Quit"}
		} else {
			items = []string{"✓ Plan created", "[Enter] Home"}
		}
	case PlanCreateStateCancelled:
		items = []string{"[h] Home", "[q] Quit"}
	case PlanCreateStateError:
		items = []string{"[r] Retry", "[h] Home", "[q] Quit"}
	}

	if m.mode == PlanCreateModeDemoUnsaved {
		items = append([]string{"[DEMO]"}, items...)
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

// normalizeSourcePath converts an absolute path to relative from repo root.
func normalizeSourcePath(filePath string) string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath
	}

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

// shortenPath truncates a path to show only the last parts if it exceeds maxLen.
// For paths > maxLen, shows ".../last/two.go".
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
