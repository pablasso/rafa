package views

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pablasso/rafa/internal/tui/components"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/styles"
)

type filePickerMode int

const (
	filePickerModeBrowse filePickerMode = iota
	filePickerModeDesignCurated
	filePickerModeDesignBrowse
)

type designDocEntry struct {
	Name         string
	AbsolutePath string
	RelativePath string
	PlanCount    int
}

type curatedRow struct {
	Text      string
	IsHeading bool
}

// FilePickerModel is the model for the file picker view.
type FilePickerModel struct {
	picker   filepicker.Model
	repoRoot string
	width    int
	height   int
	err      error

	mode          filePickerMode
	designDocs    []designDocEntry
	unplannedDocs int
	cursor        int
	offset        int
}

// NewFilePickerModel creates a new browse-only FilePickerModel starting in startDir.
func NewFilePickerModel(startDir string) FilePickerModel {
	fp := newBubbleFilePicker(startDir)

	return FilePickerModel{
		picker:   fp,
		repoRoot: startDir,
		mode:     filePickerModeBrowse,
	}
}

// NewPlanFilePickerModel creates a plan-creation picker that starts in curated docs/designs mode.
func NewPlanFilePickerModel(repoRoot string) FilePickerModel {
	m := NewFilePickerModel(repoRoot)
	m.repoRoot = repoRoot
	m.mode = filePickerModeDesignCurated
	m.refreshDesignDocs()
	m.clampCursor()
	return m
}

func newBubbleFilePicker(startDir string) filepicker.Model {
	fp := filepicker.New()
	fp.CurrentDirectory = startDir
	fp.AllowedTypes = []string{".md"} // Filter to markdown files
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.DirAllowed = false // Don't allow selecting directories, only files
	fp.FileAllowed = true // Allow selecting files

	// Customize styles to match our teal color scheme
	fp.Styles.Cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FAFAF"))
	fp.Styles.Directory = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FAFAF"))
	fp.Styles.File = lipgloss.NewStyle()
	fp.Styles.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FAFAF")).Bold(true)
	fp.Styles.DisabledCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	fp.Styles.DisabledFile = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	fp.Styles.DisabledSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

	return fp
}

// Init implements tea.Model.
func (m FilePickerModel) Init() tea.Cmd {
	if m.mode == filePickerModeDesignCurated {
		return nil
	}
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
		if m.mode == filePickerModeDesignCurated {
			m.ensureCursorVisible()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.GoToHomeMsg{} }
		case "ctrl+c":
			return m, tea.Quit
		case "d":
			if m.mode == filePickerModeDesignBrowse {
				m.mode = filePickerModeDesignCurated
				m.refreshDesignDocs()
				m.clampCursor()
				m.ensureCursorVisible()
				return m, nil
			}
		}
	}

	if m.mode == filePickerModeDesignCurated {
		keyMsg, ok := msg.(tea.KeyMsg)
		if !ok {
			return m, nil
		}
		return m.updateCuratedMode(keyMsg)
	}

	// Delegate browse modes to bubble filepicker.
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	// Check if a file was selected.
	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		absPath, err := filepath.Abs(path)
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, func() tea.Msg { return msgs.FileSelectedMsg{Path: absPath} }
	}

	// Check if user tried to select a non-.md file (disabled by AllowedTypes filter).
	if didSelect, _ := m.picker.DidSelectDisabledFile(msg); didSelect {
		return m, cmd
	}

	return m, cmd
}

func (m FilePickerModel) updateCuratedMode(msg tea.KeyMsg) (FilePickerModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "down", "j":
		if m.cursor < len(m.designDocs)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
	case "enter":
		if len(m.designDocs) == 0 {
			return m, nil
		}
		selected := m.designDocs[m.cursor]
		return m, func() tea.Msg { return msgs.FileSelectedMsg{Path: selected.AbsolutePath} }
	case "b":
		m.mode = filePickerModeDesignBrowse
		m.picker.CurrentDirectory = m.repoRoot
		m.picker.Path = ""
		return m, m.picker.Init()
	}
	return m, nil
}

func (m *FilePickerModel) refreshDesignDocs() {
	pattern := filepath.Join(m.repoRoot, "docs", "designs", "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		m.err = err
		m.designDocs = nil
		m.unplannedDocs = 0
		return
	}

	planCounts := loadPlanSourceCounts(m.repoRoot)

	unplanned := make([]designDocEntry, 0, len(matches))
	planned := make([]designDocEntry, 0, len(matches))

	for _, match := range matches {
		absPath, err := filepath.Abs(match)
		if err != nil {
			continue
		}

		relPath, err := normalizeRepoRelativePath(m.repoRoot, absPath)
		if err != nil {
			continue
		}

		entry := designDocEntry{
			Name:         filepath.Base(absPath),
			AbsolutePath: absPath,
			RelativePath: relPath,
			PlanCount:    planCounts[relPath],
		}

		if entry.PlanCount > 0 {
			planned = append(planned, entry)
			continue
		}
		unplanned = append(unplanned, entry)
	}

	sort.Slice(unplanned, func(i, j int) bool {
		return strings.ToLower(unplanned[i].Name) < strings.ToLower(unplanned[j].Name)
	})
	sort.Slice(planned, func(i, j int) bool {
		return strings.ToLower(planned[i].Name) < strings.ToLower(planned[j].Name)
	})

	all := make([]designDocEntry, 0, len(unplanned)+len(planned))
	all = append(all, unplanned...)
	all = append(all, planned...)

	m.designDocs = all
	m.unplannedDocs = len(unplanned)
}

func loadPlanSourceCounts(repoRoot string) map[string]int {
	counts := map[string]int{}
	plansDir := filepath.Join(repoRoot, ".rafa", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return counts
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		planPath := filepath.Join(plansDir, entry.Name(), "plan.json")
		data, err := os.ReadFile(planPath)
		if err != nil {
			continue
		}

		var payload struct {
			SourceFile string `json:"sourceFile"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}

		normalized, err := normalizeRepoRelativePath(repoRoot, payload.SourceFile)
		if err != nil || normalized == "" {
			continue
		}
		counts[normalized]++
	}

	return counts
}

func normalizeRepoRelativePath(repoRoot, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}

	var target string
	if filepath.IsAbs(path) {
		target = path
	} else {
		target = filepath.Join(rootAbs, path)
	}

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(rel), nil
}

// View implements tea.Model.
func (m FilePickerModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.mode == filePickerModeDesignCurated {
		return m.renderCuratedDesignDocsView()
	}

	return m.renderBrowseView()
}

func (m FilePickerModel) renderBrowseView() string {
	var b strings.Builder

	// Title.
	title := styles.TitleStyle.Render("Select Design Document")
	titleLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title)
	b.WriteString(titleLine)
	if m.mode == filePickerModeDesignBrowse {
		subtitle := styles.SubtleStyle.Render("Browse mode (press d to return to docs/designs/)")
		b.WriteString("\n")
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, subtitle))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n\n")
	}

	// File picker.
	pickerView := m.picker.View()
	b.WriteString(pickerView)

	// Calculate how many lines we've used.
	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1 // -1 for status bar
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	// Status bar.
	statusItems := []string{"↑↓ Navigate", "Enter Select", "Esc Back"}
	if m.mode == filePickerModeDesignBrowse {
		statusItems = []string{"↑↓ Navigate", "Enter Select", "d Design Docs", "Esc Back"}
	}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

func (m FilePickerModel) renderCuratedDesignDocsView() string {
	var b strings.Builder

	title := styles.TitleStyle.Render("Select Design Document")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title))
	b.WriteString("\n\n")

	subtitle := styles.SubtleStyle.Render("Expected location: docs/designs/")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, subtitle))
	b.WriteString("\n\n")

	warningVisible := m.selectedDocHasPlan()
	rows := m.buildCuratedRows()
	visibleRows := m.curatedVisibleRows(warningVisible)
	start, end := m.curatedWindow(len(rows), visibleRows)
	rows = rows[start:end]

	rowLines := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.IsHeading {
			rowLines = append(rowLines, styles.SectionStyle.Render(row.Text))
			continue
		}
		rowLines = append(rowLines, row.Text)
	}

	if len(rowLines) == 0 {
		rowLines = append(rowLines, styles.SubtleStyle.Render("No design documents found in docs/designs/."))
	}

	listBlock := strings.Join(rowLines, "\n")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, listBlock))

	if warningVisible {
		b.WriteString("\n\n")
		warning := styles.ErrorStyle.Render("Warning: this design doc already has a plan; selecting creates another.")
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, warning))
	}

	lines := strings.Count(b.String(), "\n") + 1
	remainingLines := m.height - lines - 1 // -1 for status bar
	if remainingLines > 0 {
		b.WriteString(strings.Repeat("\n", remainingLines))
	}

	statusItems := []string{"↑↓ Navigate", "Enter Select", "b Browse", "Esc Back"}
	b.WriteString(components.NewStatusBar().Render(m.width, statusItems))

	return b.String()
}

func (m FilePickerModel) buildCuratedRows() []curatedRow {
	rows := make([]curatedRow, 0, m.totalCuratedRows())

	if m.unplannedDocs > 0 {
		rows = append(rows, curatedRow{Text: fmt.Sprintf("No Plan Yet (%d)", m.unplannedDocs), IsHeading: true})
		for i := 0; i < m.unplannedDocs; i++ {
			rows = append(rows, curatedRow{Text: m.formatCuratedItem(i)})
		}
	}

	planned := len(m.designDocs) - m.unplannedDocs
	if planned > 0 {
		rows = append(rows, curatedRow{Text: fmt.Sprintf("Already Has Plan (%d)", planned), IsHeading: true})
		for i := m.unplannedDocs; i < len(m.designDocs); i++ {
			rows = append(rows, curatedRow{Text: m.formatCuratedItem(i)})
		}
	}

	return rows
}

func (m FilePickerModel) formatCuratedItem(index int) string {
	if index < 0 || index >= len(m.designDocs) {
		return ""
	}

	entry := m.designDocs[index]
	indicator := "○"
	if index == m.cursor {
		indicator = "●"
	}

	main := fmt.Sprintf("%s %s", indicator, entry.Name)
	if entry.PlanCount == 0 {
		if index == m.cursor {
			return styles.SelectedStyle.Render(main)
		}
		return main
	}

	label := fmt.Sprintf("already has %d %s", entry.PlanCount, pluralizePlan(entry.PlanCount))
	if index == m.cursor {
		return styles.SelectedStyle.Render(main) + "  " + styles.SubtleStyle.Render(label)
	}
	return main + "  " + styles.SubtleStyle.Render(label)
}

func pluralizePlan(count int) string {
	if count == 1 {
		return "plan"
	}
	return "plans"
}

func (m *FilePickerModel) clampCursor() {
	if len(m.designDocs) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.designDocs) {
		m.cursor = len(m.designDocs) - 1
	}
}

func (m *FilePickerModel) ensureCursorVisible() {
	visibleRows := m.curatedVisibleRows(m.selectedDocHasPlan())
	if visibleRows <= 0 {
		return
	}

	selectedRow := m.selectedCuratedRow()
	if selectedRow < m.offset {
		m.offset = selectedRow
	}
	if selectedRow >= m.offset+visibleRows {
		m.offset = selectedRow - visibleRows + 1
	}

	maxOffset := m.totalCuratedRows() - visibleRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m FilePickerModel) totalCuratedRows() int {
	rows := 0
	if m.unplannedDocs > 0 {
		rows += 1 + m.unplannedDocs
	}
	planned := len(m.designDocs) - m.unplannedDocs
	if planned > 0 {
		rows += 1 + planned
	}
	return rows
}

func (m FilePickerModel) selectedCuratedRow() int {
	if len(m.designDocs) == 0 {
		return 0
	}

	if m.cursor < m.unplannedDocs {
		return 1 + m.cursor
	}

	row := 0
	if m.unplannedDocs > 0 {
		row += 1 + m.unplannedDocs
	}
	row++ // planned heading
	row += m.cursor - m.unplannedDocs
	return row
}

func (m FilePickerModel) selectedDocHasPlan() bool {
	if len(m.designDocs) == 0 {
		return false
	}
	return m.cursor >= m.unplannedDocs
}

func (m FilePickerModel) curatedVisibleRows(warningVisible bool) int {
	warningRows := 0
	if warningVisible {
		warningRows = 2
	}
	// 4 lines for title/subtitle block and 1 line for status bar.
	visibleRows := m.height - 4 - warningRows - 1
	if visibleRows < 1 {
		return 1
	}
	return visibleRows
}

func (m FilePickerModel) curatedWindow(totalRows, visibleRows int) (int, int) {
	start := m.offset
	if start < 0 {
		start = 0
	}
	if start > totalRows {
		start = totalRows
	}

	end := start + visibleRows
	if end > totalRows {
		end = totalRows
	}
	return start, end
}

// SetSize updates the model dimensions.
func (m *FilePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Reserve space for title (2 lines) and status bar (1 line).
	m.picker.Height = height - 4
	if m.mode == filePickerModeDesignCurated {
		m.ensureCursorVisible()
	}
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
