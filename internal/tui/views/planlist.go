package views

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
)

// PlanSummary contains summary information about a plan for display.
type PlanSummary struct {
	ID        string
	Name      string
	TaskCount int
	Status    string // "not_started", "in_progress", "completed", "failed"
	Completed int    // for in_progress: how many tasks are done
	Locked    bool   // true if plan has a .lock file (running elsewhere)
}

// PlanListModel is the model for the plan selection view.
type PlanListModel struct {
	plans        []PlanSummary
	cursor       int
	rafaDir      string
	width        int
	height       int
	lockedErrMsg string // temporary error message when trying to select locked plan
}

// NewPlanListModel creates a new PlanListModel and loads plans from the rafaDir.
func NewPlanListModel(rafaDir string) PlanListModel {
	m := PlanListModel{
		rafaDir: rafaDir,
	}
	m.plans = m.loadPlans()
	return m
}

// loadPlans reads plan data from .rafa/plans/*/plan.json files.
func (m PlanListModel) loadPlans() []PlanSummary {
	var summaries []PlanSummary

	plansPath := filepath.Join(m.rafaDir, "plans")
	entries, err := os.ReadDir(plansPath)
	if err != nil {
		return summaries
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		planDir := filepath.Join(plansPath, entry.Name())
		planJSONPath := filepath.Join(planDir, "plan.json")
		data, err := os.ReadFile(planJSONPath)
		if err != nil {
			continue
		}

		var p plan.Plan
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}

		// Count completed tasks
		completed := 0
		for _, task := range p.Tasks {
			if task.Status == plan.TaskStatusCompleted {
				completed++
			}
		}

		// Check if plan is locked
		locked := isLocked(planDir)

		summaries = append(summaries, PlanSummary{
			ID:        p.ID,
			Name:      p.Name,
			TaskCount: len(p.Tasks),
			Status:    p.Status,
			Completed: completed,
			Locked:    locked,
		})
	}

	return summaries
}

// isLocked checks if a plan directory has a .lock file indicating it's running elsewhere.
func isLocked(planDir string) bool {
	lockFile := filepath.Join(planDir, ".lock")
	_, err := os.Stat(lockFile)
	return err == nil
}

// Init implements tea.Model.
func (m PlanListModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m PlanListModel) Update(msg tea.Msg) (PlanListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle empty state
		if len(m.plans) == 0 {
			switch msg.String() {
			case "c":
				return m, func() tea.Msg { return msgs.GoToFilePickerMsg{} }
			case "esc":
				return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		// Handle normal state with plans
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		case "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.lockedErrMsg = "" // Clear error on navigation
			}
		case "down", "j":
			if m.cursor < len(m.plans)-1 {
				m.cursor++
				m.lockedErrMsg = "" // Clear error on navigation
			}
		case "enter":
			if m.cursor < len(m.plans) {
				selectedPlan := m.plans[m.cursor]
				if selectedPlan.Locked {
					m.lockedErrMsg = "Plan is running elsewhere"
					return m, nil
				}
				// Send full plan ID in format "shortID-name" to match directory naming
				fullPlanID := fmt.Sprintf("%s-%s", selectedPlan.ID, selectedPlan.Name)
				return m, func() tea.Msg { return msgs.RunPlanMsg{PlanID: fullPlanID} }
			}
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m PlanListModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if len(m.plans) == 0 {
		return m.renderEmptyView()
	}

	return m.renderNormalView()
}

// renderNormalView renders the view when plans exist.
func (m PlanListModel) renderNormalView() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Select Plan to Run")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)

	// Plan list
	var planLines []string
	for i, p := range m.plans {
		line := m.formatPlanLine(i, p)
		planLines = append(planLines, line)
	}

	planList := strings.Join(planLines, "\n")

	// Calculate vertical centering (add 2 for potential error message)
	statusBarHeight := 1
	contentHeight := 2 + len(m.plans) // title + spacing + plans
	if m.lockedErrMsg != "" {
		contentHeight += 2 // error message + spacing
	}
	availableHeight := m.height - statusBarHeight

	topPadding := (availableHeight - contentHeight) / 3 // bias towards top
	if topPadding < 0 {
		topPadding = 0
	}

	// Build content
	b.WriteString(strings.Repeat("\n", topPadding))
	b.WriteString(titleLine)
	b.WriteString("\n\n")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, planList))

	// Show locked error message if present
	if m.lockedErrMsg != "" {
		b.WriteString("\n\n")
		errLine := styles.ErrorStyle.Render(m.lockedErrMsg)
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, errLine))
	}

	// Calculate remaining lines for bottom padding
	currentLines := topPadding + contentHeight
	bottomPadding := availableHeight - currentLines
	if bottomPadding < 0 {
		bottomPadding = 0
	}
	b.WriteString(strings.Repeat("\n", bottomPadding))

	// Status bar
	statusItems := []string{"↑↓ Navigate", "Enter Run", "Esc Back"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// formatPlanLine formats a single plan line for display.
func (m PlanListModel) formatPlanLine(index int, p PlanSummary) string {
	// Selection indicator - show lock icon for locked plans
	var indicator string
	if p.Locked {
		indicator = styles.SubtleStyle.Render("*")
	} else if index == m.cursor {
		indicator = "●"
	} else {
		indicator = "○"
	}

	// Plan ID-Name
	idName := fmt.Sprintf("%s-%s", p.ID, p.Name)

	// Task count
	taskCountStr := fmt.Sprintf("%d tasks", p.TaskCount)
	if p.TaskCount == 1 {
		taskCountStr = "1 task"
	}

	// Status - append "(locked)" for locked plans
	var statusStr string
	switch p.Status {
	case plan.PlanStatusInProgress:
		statusStr = fmt.Sprintf("in_progress (%d/%d)", p.Completed, p.TaskCount)
	default:
		statusStr = p.Status
	}
	if p.Locked {
		statusStr = "running (locked)"
	}

	// Build the line with alignment
	// Format: ● idName       taskCount   status
	line := fmt.Sprintf("%s %-30s %10s   %s", indicator, idName, taskCountStr, statusStr)

	// Apply styling based on selection and status
	if p.Locked {
		// Locked plans are always shown in subtle style
		line = styles.SubtleStyle.Render(line)
	} else if index == m.cursor {
		line = styles.SelectedStyle.Render(line)
	} else if p.Status == plan.PlanStatusCompleted {
		line = styles.SubtleStyle.Render(line)
	}

	return line
}

// renderEmptyView renders the view when no plans exist.
func (m PlanListModel) renderEmptyView() string {
	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Select Plan to Run")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)

	// Message
	msg1 := "No plans found."
	msg2 := "Press 'c' to create a new plan, or Esc to go back."
	msg1Line := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, msg1)
	msg2Line := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, styles.SubtleStyle.Render(msg2))

	// Calculate vertical centering
	statusBarHeight := 1
	contentHeight := 5 // title + spacing + msg1 + spacing + msg2
	availableHeight := m.height - statusBarHeight

	topPadding := (availableHeight - contentHeight) / 3
	if topPadding < 0 {
		topPadding = 0
	}

	// Build content
	b.WriteString(strings.Repeat("\n", topPadding))
	b.WriteString(titleLine)
	b.WriteString("\n\n")
	b.WriteString(msg1Line)
	b.WriteString("\n\n")
	b.WriteString(msg2Line)

	// Calculate remaining lines for bottom padding
	currentLines := topPadding + contentHeight
	bottomPadding := availableHeight - currentLines
	if bottomPadding < 0 {
		bottomPadding = 0
	}
	b.WriteString(strings.Repeat("\n", bottomPadding))

	// Status bar
	statusItems := []string{"c Create plan", "Esc Back"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// SetSize updates the model dimensions.
func (m *PlanListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Plans returns the list of plan summaries.
func (m PlanListModel) Plans() []PlanSummary {
	return m.plans
}

// Cursor returns the current cursor position.
func (m PlanListModel) Cursor() int {
	return m.cursor
}

// RafaDir returns the rafa directory path.
func (m PlanListModel) RafaDir() string {
	return m.rafaDir
}

// LockedErrMsg returns the current locked error message (if any).
func (m PlanListModel) LockedErrMsg() string {
	return m.lockedErrMsg
}
