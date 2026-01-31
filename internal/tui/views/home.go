package views

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/session"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
)

// MenuItem represents a menu option in the home view.
type MenuItem struct {
	Label       string
	Shortcut    string
	Description string
}

// MenuSection represents a group of related menu items.
type MenuSection struct {
	Title string
	Items []MenuItem
}

// HomeModel is the model for the home view landing screen.
type HomeModel struct {
	sections   []MenuSection
	cursor     int
	rafaExists bool
	width      int
	height     int
	errorMsg   string // Temporary error message to display
}

// NewHomeModel creates a new HomeModel, checking if rafaDir exists.
func NewHomeModel(rafaDir string) HomeModel {
	rafaExists := false
	if rafaDir != "" {
		if _, err := os.Stat(rafaDir); err == nil {
			rafaExists = true
		}
	}

	return HomeModel{
		sections: []MenuSection{
			{
				Title: "Define",
				Items: []MenuItem{
					{Label: "Create PRD", Shortcut: "p", Description: "Start a new product requirements document"},
					{Label: "Create Design Doc", Shortcut: "d", Description: "Create a technical design from PRD"},
				},
			},
			{
				Title: "Execute",
				Items: []MenuItem{
					{Label: "Create Plan", Shortcut: "c", Description: "Generate execution plan from design"},
					{Label: "Run Plan", Shortcut: "r", Description: "Execute an existing plan"},
				},
			},
			{
				Title: "",
				Items: []MenuItem{
					{Label: "Quit", Shortcut: "q", Description: ""},
				},
			},
		},
		cursor:     0,
		rafaExists: rafaExists,
	}
}

// Init implements tea.Model.
func (m HomeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m HomeModel) Update(msg tea.Msg) (HomeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// If .rafa doesn't exist, only handle quit
		if !m.rafaExists {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			return m, func() tea.Msg { return msgs.GoToConversationMsg{Phase: session.PhasePRD} }
		case "d":
			return m, func() tea.Msg { return msgs.GoToConversationMsg{Phase: session.PhaseDesign} }
		case "c":
			return m, func() tea.Msg { return msgs.GoToFilePickerMsg{ForPlanCreation: true} }
		case "r":
			return m, func() tea.Msg { return msgs.GoToPlanListMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			totalItems := m.totalMenuItems()
			if m.cursor < totalItems-1 {
				m.cursor++
			}
		case "enter":
			return m.selectCurrentItem()
		}
	}
	return m, nil
}

// totalMenuItems returns the total number of menu items across all sections.
func (m HomeModel) totalMenuItems() int {
	total := 0
	for _, section := range m.sections {
		total += len(section.Items)
	}
	return total
}

// selectCurrentItem returns the appropriate message based on the selected menu item.
func (m HomeModel) selectCurrentItem() (HomeModel, tea.Cmd) {
	// Map cursor position to shortcut
	shortcut := m.getShortcutAtCursor()
	switch shortcut {
	case "p":
		return m, func() tea.Msg { return msgs.GoToConversationMsg{Phase: session.PhasePRD} }
	case "d":
		return m, func() tea.Msg { return msgs.GoToConversationMsg{Phase: session.PhaseDesign} }
	case "c":
		return m, func() tea.Msg { return msgs.GoToFilePickerMsg{ForPlanCreation: true} }
	case "r":
		return m, func() tea.Msg { return msgs.GoToPlanListMsg{} }
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// getShortcutAtCursor returns the shortcut key for the currently selected item.
func (m HomeModel) getShortcutAtCursor() string {
	idx := 0
	for _, section := range m.sections {
		for _, item := range section.Items {
			if idx == m.cursor {
				return item.Shortcut
			}
			idx++
		}
	}
	return ""
}

// View implements tea.Model.
func (m HomeModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var content string
	if m.rafaExists {
		content = m.renderNormalView()
	} else {
		content = m.renderNoRafaView()
	}

	return content
}

// renderHeader returns the centered title and tagline.
func (m HomeModel) renderHeader() (titleLine, taglineLine string) {
	title := styles.TitleStyle.Render("R A F A")
	tagline := styles.SubtleStyle.Render("Task Loop Runner for AI")

	titleLine = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	taglineLine = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, tagline)
	return titleLine, taglineLine
}

// renderNormalView renders the home view with menu options.
func (m HomeModel) renderNormalView() string {
	var b strings.Builder

	titleLine, taglineLine := m.renderHeader()

	// Build menu with sections
	var menuLines []string
	cursorIdx := 0

	for sectionIdx, section := range m.sections {
		// Add section header if it has a title
		if section.Title != "" {
			sectionHeader := styles.SectionStyle.Render(section.Title)
			menuLines = append(menuLines, sectionHeader)
		}

		// Render items in this section
		for _, item := range section.Items {
			shortcut := "[" + item.Shortcut + "]"
			line := shortcut + " " + item.Label

			// Add description if present
			if item.Description != "" {
				line += "  " + styles.SubtleStyle.Render(item.Description)
			}

			if cursorIdx == m.cursor {
				// For selected items, style the main part but keep description subtle
				mainPart := "[" + item.Shortcut + "] " + item.Label
				line = styles.SelectedStyle.Render(mainPart)
				if item.Description != "" {
					line += "  " + styles.SubtleStyle.Render(item.Description)
				}
			} else {
				line = styles.SubtleStyle.Render(line)
			}
			menuLines = append(menuLines, line)
			cursorIdx++
		}

		// Add spacing between sections (except after the last one)
		if sectionIdx < len(m.sections)-1 {
			menuLines = append(menuLines, "")
		}
	}

	menu := strings.Join(menuLines, "\n")

	// Calculate vertical centering
	// Status bar takes 1 line at bottom
	statusBarHeight := 1
	// Count content lines: title + tagline + spacing + menu lines + spacing + error (if any)
	contentHeight := 2 + 2 + len(menuLines)
	if m.errorMsg != "" {
		contentHeight += 2 // error line + spacing
	}
	availableHeight := m.height - statusBarHeight

	topPadding := (availableHeight - contentHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	// Build content
	b.WriteString(strings.Repeat("\n", topPadding))

	menuBlock := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, menu)

	b.WriteString(titleLine)
	b.WriteString("\n")
	b.WriteString(taglineLine)
	b.WriteString("\n\n")
	b.WriteString(menuBlock)

	// Show error message if present
	if m.errorMsg != "" {
		b.WriteString("\n\n")
		errorLine := styles.ErrorStyle.Render(m.errorMsg)
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, errorLine))
	}

	// Calculate remaining space for bottom padding (above status bar)
	currentLines := topPadding + contentHeight
	bottomPadding := availableHeight - currentLines
	if bottomPadding < 0 {
		bottomPadding = 0
	}
	b.WriteString(strings.Repeat("\n", bottomPadding))

	// Status bar
	statusItems := []string{"↑↓ Navigate", "Enter Select", "q Quit"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// renderNoRafaView renders the view when .rafa/ directory doesn't exist.
func (m HomeModel) renderNoRafaView() string {
	var b strings.Builder

	titleLine, taglineLine := m.renderHeader()

	// Warning message
	warning1 := styles.ErrorStyle.Render("No .rafa/ directory found.")
	warning2 := styles.SubtleStyle.Render("Run 'rafa init' first to initialize this repository.")

	warning1Line := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, warning1)
	warning2Line := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, warning2)

	// Calculate vertical centering
	statusBarHeight := 1
	contentHeight := 6 // title + tagline + spacing + 2 warning lines
	availableHeight := m.height - statusBarHeight

	topPadding := (availableHeight - contentHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	// Build content
	b.WriteString(strings.Repeat("\n", topPadding))

	b.WriteString(titleLine)
	b.WriteString("\n")
	b.WriteString(taglineLine)
	b.WriteString("\n\n")
	b.WriteString(warning1Line)
	b.WriteString("\n")
	b.WriteString(warning2Line)

	// Calculate remaining space for bottom padding
	currentLines := topPadding + contentHeight
	bottomPadding := availableHeight - currentLines
	if bottomPadding < 0 {
		bottomPadding = 0
	}
	b.WriteString(strings.Repeat("\n", bottomPadding))

	// Status bar (simplified for no-rafa view)
	statusItems := []string{"q Quit"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// SetSize updates the model dimensions.
func (m *HomeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// RafaExists returns whether the .rafa directory exists.
func (m HomeModel) RafaExists() bool {
	return m.rafaExists
}

// Cursor returns the current cursor position.
func (m HomeModel) Cursor() int {
	return m.cursor
}

// SetError sets an error message to display temporarily.
func (m *HomeModel) SetError(msg string) {
	m.errorMsg = msg
}

// Error returns the current error message.
func (m HomeModel) Error() string {
	return m.errorMsg
}
