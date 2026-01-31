package views

import (
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/styles"
)

// FileNamingState represents the current state of the file naming view.
type FileNamingState int

const (
	// StateConfirm shows the suggested filename with confirmation options.
	StateConfirm FileNamingState = iota
	// StateEdit allows editing the filename.
	StateEdit
	// StateOverwriteConfirm shows when file exists and requires overwrite confirmation.
	StateOverwriteConfirm
)

// FileNamingResult represents the outcome of the file naming interaction.
type FileNamingResult int

const (
	// ResultPending means no decision has been made yet.
	ResultPending FileNamingResult = iota
	// ResultSave means the file should be saved.
	ResultSave
	// ResultCancel means the operation was cancelled.
	ResultCancel
)

// FileNamingModel handles the file naming confirmation UI.
type FileNamingModel struct {
	// Suggested filename (extracted from Claude's response)
	suggestedFilename string

	// Content to be saved (passed from conversation)
	content string

	// Current state
	state FileNamingState

	// Result of the interaction
	result FileNamingResult

	// Final filename (may be edited by user)
	finalFilename string

	// Whether the file already exists
	fileExists bool

	// Text input for editing
	input textinput.Model

	// Dimensions
	width  int
	height int

	// File system abstraction for testing
	fileChecker FileExistsChecker
}

// FileExistsChecker abstracts file existence checks for testing.
type FileExistsChecker interface {
	Exists(path string) bool
}

// DefaultFileChecker uses os.Stat to check file existence.
type DefaultFileChecker struct{}

// Exists checks if a file exists.
func (d DefaultFileChecker) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FileNamingConfig holds initialization parameters.
type FileNamingConfig struct {
	ClaudeResponse string // The response text from Claude to extract filename from
	Content        string // The content to be saved
}

// NewFileNamingModel creates a new file naming view.
func NewFileNamingModel(config FileNamingConfig) FileNamingModel {
	filename := ExtractFilename(config.ClaudeResponse)

	ti := textinput.New()
	ti.Placeholder = "Enter filename..."
	ti.CharLimit = 256
	ti.Width = 60

	m := FileNamingModel{
		suggestedFilename: filename,
		content:           config.Content,
		state:             StateConfirm,
		result:            ResultPending,
		finalFilename:     filename,
		input:             ti,
		fileChecker:       DefaultFileChecker{},
	}

	// Check if file exists
	m.checkFileExists()

	return m
}

// SetFileChecker sets a custom file existence checker (for testing).
func (m *FileNamingModel) SetFileChecker(fc FileExistsChecker) {
	m.fileChecker = fc
	m.checkFileExists()
}

// checkFileExists updates the fileExists flag.
func (m *FileNamingModel) checkFileExists() {
	if m.finalFilename != "" && m.fileChecker != nil {
		m.fileExists = m.fileChecker.Exists(m.finalFilename)
	}
}

// Init implements tea.Model.
func (m FileNamingModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m FileNamingModel) Update(msg tea.Msg) (FileNamingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	// Update text input if in edit mode
	if m.state == StateEdit {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKeyPress processes keyboard input based on current state.
func (m FileNamingModel) handleKeyPress(msg tea.KeyMsg) (FileNamingModel, tea.Cmd) {
	switch m.state {
	case StateConfirm:
		return m.handleConfirmKeys(msg)
	case StateEdit:
		return m.handleEditKeys(msg)
	case StateOverwriteConfirm:
		return m.handleOverwriteKeys(msg)
	}
	return m, nil
}

// handleConfirmKeys handles keys in the confirmation state.
func (m FileNamingModel) handleConfirmKeys(msg tea.KeyMsg) (FileNamingModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.fileExists {
			// File exists, need overwrite confirmation
			m.state = StateOverwriteConfirm
		} else {
			// No conflict, save directly
			m.result = ResultSave
		}
		return m, nil

	case "e":
		m.state = StateEdit
		m.input.SetValue(m.finalFilename)
		m.input.Focus()
		return m, textinput.Blink

	case "c", "esc":
		m.result = ResultCancel
		return m, nil
	}
	return m, nil
}

// handleEditKeys handles keys in the edit state.
func (m FileNamingModel) handleEditKeys(msg tea.KeyMsg) (FileNamingModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.finalFilename = m.input.Value()
		m.checkFileExists()
		m.state = StateConfirm
		m.input.Blur()
		return m, nil

	case "esc":
		// Cancel edit, revert to original
		m.state = StateConfirm
		m.input.Blur()
		return m, nil
	}

	// Pass through to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleOverwriteKeys handles keys in the overwrite confirmation state.
func (m FileNamingModel) handleOverwriteKeys(msg tea.KeyMsg) (FileNamingModel, tea.Cmd) {
	switch msg.String() {
	case "o":
		// Confirm overwrite
		m.result = ResultSave
		return m, nil

	case "e":
		// Edit filename instead
		m.state = StateEdit
		m.input.SetValue(m.finalFilename)
		m.input.Focus()
		return m, textinput.Blink

	case "c", "esc":
		m.result = ResultCancel
		return m, nil
	}
	return m, nil
}

// View implements tea.Model.
func (m FileNamingModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// Title
	title := styles.TitleStyle.Render("Save Document")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title))
	b.WriteString("\n\n")

	// Content based on state
	switch m.state {
	case StateConfirm:
		b.WriteString(m.renderConfirmView())
	case StateEdit:
		b.WriteString(m.renderEditView())
	case StateOverwriteConfirm:
		b.WriteString(m.renderOverwriteView())
	}

	// Add spacing
	b.WriteString("\n\n")

	// Action bar
	b.WriteString(m.renderActionBar())

	return b.String()
}

// renderConfirmView renders the confirmation view.
func (m FileNamingModel) renderConfirmView() string {
	var b strings.Builder

	b.WriteString("Suggested filename:\n\n")
	b.WriteString("  ")
	b.WriteString(styles.SelectedStyle.Render(m.finalFilename))
	b.WriteString("\n")

	if m.fileExists {
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render("⚠ File already exists!"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderEditView renders the edit view.
func (m FileNamingModel) renderEditView() string {
	var b strings.Builder

	b.WriteString("Edit filename:\n\n")
	b.WriteString("  ")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	return b.String()
}

// renderOverwriteView renders the overwrite confirmation view.
func (m FileNamingModel) renderOverwriteView() string {
	var b strings.Builder

	b.WriteString("File already exists:\n\n")
	b.WriteString("  ")
	b.WriteString(styles.SelectedStyle.Render(m.finalFilename))
	b.WriteString("\n\n")
	b.WriteString(styles.ErrorStyle.Render("⚠ This will overwrite the existing file!"))
	b.WriteString("\n")

	return b.String()
}

// renderActionBar renders the bottom action bar.
func (m FileNamingModel) renderActionBar() string {
	var items []string

	switch m.state {
	case StateConfirm:
		if m.fileExists {
			items = []string{"Enter View overwrite", "[e] Edit name", "[c] Cancel"}
		} else {
			items = []string{"Enter Save", "[e] Edit name", "[c] Cancel"}
		}
	case StateEdit:
		items = []string{"Enter Confirm", "Esc Cancel edit"}
	case StateOverwriteConfirm:
		items = []string{"[o] Overwrite", "[e] Edit name", "[c] Cancel"}
	}

	return components.NewStatusBar().Render(m.width, items)
}

// SetSize updates the model dimensions.
func (m *FileNamingModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = width - 10
	if m.input.Width < 20 {
		m.input.Width = 20
	}
}

// Getters

// SuggestedFilename returns the extracted suggested filename.
func (m FileNamingModel) SuggestedFilename() string {
	return m.suggestedFilename
}

// FinalFilename returns the current filename (may be edited).
func (m FileNamingModel) FinalFilename() string {
	return m.finalFilename
}

// Content returns the content to be saved.
func (m FileNamingModel) Content() string {
	return m.content
}

// State returns the current state.
func (m FileNamingModel) State() FileNamingState {
	return m.state
}

// Result returns the result of the interaction.
func (m FileNamingModel) Result() FileNamingResult {
	return m.result
}

// FileExists returns whether the file already exists.
func (m FileNamingModel) FileExists() bool {
	return m.fileExists
}

// IsEditing returns whether the user is editing the filename.
func (m FileNamingModel) IsEditing() bool {
	return m.state == StateEdit
}

// Filename extraction

// filenamePatterns are regex patterns to extract filenames from Claude's response.
// Order matters: more specific patterns first.
var filenamePatterns = []*regexp.Regexp{
	// Pattern: 'saving this as `<filename>`'
	regexp.MustCompile(`saving this as\s+` + "`" + `([^` + "`" + `]+)` + "`"),
	// Pattern: 'filename: <filename>'
	regexp.MustCompile(`(?i)filename:\s*([^\s\n]+)`),
	// Pattern: 'saved to `<filename>`'
	regexp.MustCompile(`saved to\s+` + "`" + `([^` + "`" + `]+)` + "`"),
	// Pattern: 'writing to `<filename>`'
	regexp.MustCompile(`writing to\s+` + "`" + `([^` + "`" + `]+)` + "`"),
	// Pattern: file path in backticks that looks like a path (contains / or ends in common extension)
	regexp.MustCompile("`" + `((?:[^` + "`" + `]+/)?[^` + "`" + `]+\.(?:md|txt|json|yaml|yml|go|py|js|ts))` + "`"),
}

// ExtractFilename extracts a filename from Claude's response text.
// Returns empty string if no filename is found.
func ExtractFilename(response string) string {
	for _, pattern := range filenamePatterns {
		matches := pattern.FindStringSubmatch(response)
		if len(matches) > 1 {
			filename := strings.TrimSpace(matches[1])
			if filename != "" {
				return filename
			}
		}
	}
	return ""
}
