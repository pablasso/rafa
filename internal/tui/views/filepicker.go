package views

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/tui"
	"github.com/pablasso/rafa/internal/tui/components"
)

// FileSelectedMsg is sent when a file is selected.
type FileSelectedMsg struct {
	Path string // absolute path to selected file
}

// GoToHomeMsg signals transition back to home view.
type GoToHomeMsg struct{}

// FilePickerModel is the model for the file picker view.
type FilePickerModel struct {
	picker   filepicker.Model
	repoRoot string
	width    int
	height   int
	err      error
}

// NewFilePickerModel creates a new FilePickerModel starting in the repository root.
func NewFilePickerModel(repoRoot string) FilePickerModel {
	fp := filepicker.New()
	fp.CurrentDirectory = repoRoot
	fp.AllowedTypes = []string{".md"} // Filter to markdown files
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.DirAllowed = false // Don't allow selecting directories, only files
	fp.FileAllowed = true // Allow selecting files

	return FilePickerModel{
		picker:   fp,
		repoRoot: repoRoot,
	}
}

// Init implements tea.Model.
func (m FilePickerModel) Init() tea.Cmd {
	return m.picker.Init()
}

// Update implements tea.Model.
func (m FilePickerModel) Update(msg tea.Msg) (FilePickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve space for title (2 lines) and status bar (1 line)
		m.picker.Height = m.height - 4
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return GoToHomeMsg{} }
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	// Delegate to filepicker
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	// Check if a file was selected
	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		// Ensure the path is absolute
		absPath, err := filepath.Abs(path)
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, func() tea.Msg { return FileSelectedMsg{Path: absPath} }
	}

	// Check if user tried to select a non-.md file (disabled by AllowedTypes filter)
	if didSelect, _ := m.picker.DidSelectDisabledFile(msg); didSelect {
		// Selection ignored, only .md files are allowed
		return m, cmd
	}

	return m, cmd
}

// View implements tea.Model.
func (m FilePickerModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Title
	title := tui.TitleStyle.Render("Select Design Document")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	b.WriteString("\n\n")

	// File picker
	pickerView := m.picker.View()
	b.WriteString(pickerView)

	// Calculate how many lines we've used
	lines := strings.Count(b.String(), "\n") + 1
	// Fill remaining space before status bar
	remainingLines := m.height - lines - 1 // -1 for status bar
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar
	statusItems := []string{"↑↓ Navigate", "Enter Select", "Esc Back"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

// SetSize updates the model dimensions.
func (m *FilePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Reserve space for title (2 lines) and status bar (1 line)
	m.picker.Height = height - 4
}

// CurrentDirectory returns the current directory being displayed.
func (m FilePickerModel) CurrentDirectory() string {
	return m.picker.CurrentDirectory
}

// RepoRoot returns the repository root directory.
func (m FilePickerModel) RepoRoot() string {
	return m.repoRoot
}

// Err returns any error that occurred.
func (m FilePickerModel) Err() error {
	return m.err
}
