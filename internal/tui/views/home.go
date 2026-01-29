package views

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
)

// MenuItem represents a menu option in the home view.
type MenuItem struct {
	Label    string
	Shortcut string
}

// HomeModel is the model for the home view landing screen.
type HomeModel struct {
	menuItems  []MenuItem
	cursor     int
	rafaExists bool
	width      int
	height     int
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
		menuItems: []MenuItem{
			{Label: "Create new plan", Shortcut: "c"},
			{Label: "Run existing plan", Shortcut: "r"},
			{Label: "Quit", Shortcut: "q"},
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
		case "c":
			return m, func() tea.Msg { return msgs.GoToFilePickerMsg{} }
		case "r":
			return m, func() tea.Msg { return msgs.GoToPlanListMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}
		case "enter":
			return m.selectCurrentItem()
		}
	}
	return m, nil
}

// selectCurrentItem returns the appropriate message based on the selected menu item.
func (m HomeModel) selectCurrentItem() (HomeModel, tea.Cmd) {
	switch m.cursor {
	case 0: // Create new plan
		return m, func() tea.Msg { return msgs.GoToFilePickerMsg{} }
	case 1: // Run existing plan
		return m, func() tea.Msg { return msgs.GoToPlanListMsg{} }
	case 2: // Quit
		return m, tea.Quit
	}
	return m, nil
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

	// Menu items
	var menuLines []string
	for i, item := range m.menuItems {
		shortcut := "[" + item.Shortcut + "]"
		line := shortcut + " " + item.Label

		if i == m.cursor {
			line = styles.SelectedStyle.Render(line)
		} else {
			line = styles.SubtleStyle.Render(line)
		}
		menuLines = append(menuLines, line)
	}

	menu := strings.Join(menuLines, "\n")

	// Calculate vertical centering
	// Status bar takes 1 line at bottom
	statusBarHeight := 1
	contentHeight := 7 // title + tagline + spacing + 3 menu items + spacing
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
