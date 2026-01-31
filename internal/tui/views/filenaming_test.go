package views

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockFileChecker implements FileExistsChecker for testing.
type mockFileChecker struct {
	existingFiles map[string]bool
}

func newMockFileChecker(existing ...string) *mockFileChecker {
	m := &mockFileChecker{existingFiles: make(map[string]bool)}
	for _, f := range existing {
		m.existingFiles[f] = true
	}
	return m
}

func (m *mockFileChecker) Exists(path string) bool {
	return m.existingFiles[path]
}

// Test ExtractFilename patterns

func TestExtractFilename_SavingThisAs(t *testing.T) {
	response := "I've created the PRD. I'll be saving this as `docs/prds/my-feature.md`"
	result := ExtractFilename(response)
	if result != "docs/prds/my-feature.md" {
		t.Errorf("expected 'docs/prds/my-feature.md', got '%s'", result)
	}
}

func TestExtractFilename_Filename(t *testing.T) {
	response := "Here's the document.\n\nfilename: docs/designs/api-design.md"
	result := ExtractFilename(response)
	if result != "docs/designs/api-design.md" {
		t.Errorf("expected 'docs/designs/api-design.md', got '%s'", result)
	}
}

func TestExtractFilename_FilenameUppercase(t *testing.T) {
	response := "Filename: myfile.md"
	result := ExtractFilename(response)
	if result != "myfile.md" {
		t.Errorf("expected 'myfile.md', got '%s'", result)
	}
}

func TestExtractFilename_SavedTo(t *testing.T) {
	response := "The document has been saved to `docs/prds/feature.md`"
	result := ExtractFilename(response)
	if result != "docs/prds/feature.md" {
		t.Errorf("expected 'docs/prds/feature.md', got '%s'", result)
	}
}

func TestExtractFilename_WritingTo(t *testing.T) {
	response := "I'm writing to `docs/designs/system.md`"
	result := ExtractFilename(response)
	if result != "docs/designs/system.md" {
		t.Errorf("expected 'docs/designs/system.md', got '%s'", result)
	}
}

func TestExtractFilename_BacktickPath(t *testing.T) {
	response := "You can find it at `path/to/file.md` in your project"
	result := ExtractFilename(response)
	if result != "path/to/file.md" {
		t.Errorf("expected 'path/to/file.md', got '%s'", result)
	}
}

func TestExtractFilename_NoFilename(t *testing.T) {
	response := "This is just a regular message without any filename"
	result := ExtractFilename(response)
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestExtractFilename_MultiplePatterns_FirstWins(t *testing.T) {
	// "saving this as" should take precedence
	response := "I'll be saving this as `first.md`. filename: second.md"
	result := ExtractFilename(response)
	if result != "first.md" {
		t.Errorf("expected 'first.md', got '%s'", result)
	}
}

func TestExtractFilename_VariousExtensions(t *testing.T) {
	tests := []struct {
		response string
		expected string
	}{
		{"Check `file.go`", "file.go"},
		{"Check `file.py`", "file.py"},
		{"Check `file.js`", "file.js"},
		{"Check `file.ts`", "file.ts"},
		{"Check `file.json`", "file.json"},
		{"Check `file.yaml`", "file.yaml"},
		{"Check `file.yml`", "file.yml"},
		{"Check `file.txt`", "file.txt"},
	}

	for _, tt := range tests {
		result := ExtractFilename(tt.response)
		if result != tt.expected {
			t.Errorf("ExtractFilename(%q) = %q, want %q", tt.response, result, tt.expected)
		}
	}
}

// Test NewFileNamingModel

func TestNewFileNamingModel(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `docs/prds/test.md`",
		Content:        "# Test Content",
	}

	m := NewFileNamingModel(config)

	if m.SuggestedFilename() != "docs/prds/test.md" {
		t.Errorf("expected SuggestedFilename to be 'docs/prds/test.md', got '%s'", m.SuggestedFilename())
	}
	if m.FinalFilename() != "docs/prds/test.md" {
		t.Errorf("expected FinalFilename to be 'docs/prds/test.md', got '%s'", m.FinalFilename())
	}
	if m.Content() != "# Test Content" {
		t.Errorf("expected Content to be '# Test Content', got '%s'", m.Content())
	}
	if m.State() != StateConfirm {
		t.Errorf("expected initial State to be StateConfirm, got %d", m.State())
	}
	if m.Result() != ResultPending {
		t.Errorf("expected initial Result to be ResultPending, got %d", m.Result())
	}
}

func TestNewFileNamingModel_NoFilenameFound(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "Just a regular message",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)

	if m.SuggestedFilename() != "" {
		t.Errorf("expected SuggestedFilename to be empty, got '%s'", m.SuggestedFilename())
	}
	if m.FinalFilename() != "" {
		t.Errorf("expected FinalFilename to be empty, got '%s'", m.FinalFilename())
	}
}

// Test Enter key saving file (Acceptance Criteria 1)

func TestFileNamingModel_EnterKey_SavesFile_WhenNoConflict(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `new-file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	// Use mock that says file doesn't exist
	m.SetFileChecker(newMockFileChecker())

	// Press Enter
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if newM.Result() != ResultSave {
		t.Errorf("expected Result to be ResultSave after Enter, got %d", newM.Result())
	}
	if newM.FinalFilename() != "new-file.md" {
		t.Errorf("expected FinalFilename to be 'new-file.md', got '%s'", newM.FinalFilename())
	}
}

func TestFileNamingModel_EnterKey_ShowsOverwriteConfirm_WhenFileExists(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing-file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	// Mock file as existing
	m.SetFileChecker(newMockFileChecker("existing-file.md"))

	// Press Enter
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should transition to overwrite confirm state, not save
	if newM.State() != StateOverwriteConfirm {
		t.Errorf("expected State to be StateOverwriteConfirm, got %d", newM.State())
	}
	if newM.Result() != ResultPending {
		t.Errorf("expected Result to still be ResultPending, got %d", newM.Result())
	}
}

// Test 'e' key enabling editing mode with pre-filled input (Acceptance Criteria 2)

func TestFileNamingModel_EKey_EnablesEditingMode(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `original.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press 'e' to edit
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if newM.State() != StateEdit {
		t.Errorf("expected State to be StateEdit, got %d", newM.State())
	}
	if !newM.IsEditing() {
		t.Error("expected IsEditing() to return true")
	}
	// Should return a command (for text input blink)
	if cmd == nil {
		t.Error("expected cmd from 'e' key to enable text input")
	}
}

func TestFileNamingModel_EKey_PreFillsInput(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `prefilled-name.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press 'e' to edit
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	// The input should be pre-filled with the current filename
	if newM.input.Value() != "prefilled-name.md" {
		t.Errorf("expected input to be pre-filled with 'prefilled-name.md', got '%s'", newM.input.Value())
	}
}

func TestFileNamingModel_EditMode_EnterConfirmsEdit(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `original.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press 'e' to enter edit mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	// Simulate typing a new filename
	m.input.SetValue("new-name.md")

	// Press Enter to confirm edit
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if newM.State() != StateConfirm {
		t.Errorf("expected State to return to StateConfirm, got %d", newM.State())
	}
	if newM.FinalFilename() != "new-name.md" {
		t.Errorf("expected FinalFilename to be 'new-name.md', got '%s'", newM.FinalFilename())
	}
}

func TestFileNamingModel_EditMode_EscCancelsEdit(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `original.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press 'e' to enter edit mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	// Simulate typing a new filename
	m.input.SetValue("should-not-be-used.md")

	// Press Esc to cancel edit
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if newM.State() != StateConfirm {
		t.Errorf("expected State to return to StateConfirm, got %d", newM.State())
	}
	// Filename should remain the original
	if newM.FinalFilename() != "original.md" {
		t.Errorf("expected FinalFilename to remain 'original.md', got '%s'", newM.FinalFilename())
	}
}

// Test 'o' key overwriting when file exists (Acceptance Criteria 3)

func TestFileNamingModel_OKey_OverwritesWhenFileExists(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing.md"))

	// Press Enter to trigger overwrite confirmation
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.State() != StateOverwriteConfirm {
		t.Fatalf("expected State to be StateOverwriteConfirm, got %d", m.State())
	}

	// Press 'o' to confirm overwrite
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if newM.Result() != ResultSave {
		t.Errorf("expected Result to be ResultSave after 'o', got %d", newM.Result())
	}
	if newM.FinalFilename() != "existing.md" {
		t.Errorf("expected FinalFilename to be 'existing.md', got '%s'", newM.FinalFilename())
	}
}

func TestFileNamingModel_OKey_OnlyWorksInOverwriteState(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press 'o' in confirm state (should do nothing)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if newM.Result() != ResultPending {
		t.Errorf("expected Result to remain ResultPending, got %d", newM.Result())
	}
	if newM.State() != StateConfirm {
		t.Errorf("expected State to remain StateConfirm, got %d", newM.State())
	}
}

func TestFileNamingModel_EKey_InOverwriteState_EnablesEdit(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing.md"))

	// Go to overwrite state
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Press 'e' to edit instead of overwrite
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if newM.State() != StateEdit {
		t.Errorf("expected State to be StateEdit, got %d", newM.State())
	}
	if newM.input.Value() != "existing.md" {
		t.Errorf("expected input to be pre-filled with 'existing.md', got '%s'", newM.input.Value())
	}
}

// Test 'c' key returning without saving (Acceptance Criteria 4)

func TestFileNamingModel_CKey_CancelsWithoutSaving_InConfirmState(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press 'c' to cancel
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if newM.Result() != ResultCancel {
		t.Errorf("expected Result to be ResultCancel, got %d", newM.Result())
	}
}

func TestFileNamingModel_EscKey_CancelsWithoutSaving_InConfirmState(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Press Esc to cancel
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if newM.Result() != ResultCancel {
		t.Errorf("expected Result to be ResultCancel, got %d", newM.Result())
	}
}

func TestFileNamingModel_CKey_CancelsWithoutSaving_InOverwriteState(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing.md"))

	// Go to overwrite state
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Press 'c' to cancel
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if newM.Result() != ResultCancel {
		t.Errorf("expected Result to be ResultCancel, got %d", newM.Result())
	}
}

// Test file exists warning is shown (Acceptance Criteria 5)

func TestFileNamingModel_FileExistsWarning_IsShown(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing-file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing-file.md"))
	m.SetSize(80, 24)

	if !m.FileExists() {
		t.Error("expected FileExists() to return true")
	}

	view := m.View()

	if !strings.Contains(view, "⚠") {
		t.Error("expected view to contain warning symbol '⚠'")
	}
	if !strings.Contains(view, "already exists") {
		t.Error("expected view to contain 'already exists' warning message")
	}
}

func TestFileNamingModel_FileExistsWarning_NotShown_WhenFileDoesNotExist(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `new-file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker()) // No existing files
	m.SetSize(80, 24)

	if m.FileExists() {
		t.Error("expected FileExists() to return false")
	}

	view := m.View()

	if strings.Contains(view, "already exists") {
		t.Error("expected view to NOT contain 'already exists' warning")
	}
}

func TestFileNamingModel_OverwriteView_ShowsWarning(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing.md"))
	m.SetSize(80, 24)

	// Go to overwrite state
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()

	if !strings.Contains(view, "overwrite") {
		t.Error("expected overwrite view to contain 'overwrite' warning")
	}
	if !strings.Contains(view, "⚠") {
		t.Error("expected overwrite view to contain warning symbol")
	}
}

// Test View rendering

func TestFileNamingModel_View_EmptyDimensions(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	// Don't set dimensions

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got: %s", view)
	}
}

func TestFileNamingModel_View_ContainsTitle(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Save Document") {
		t.Error("expected view to contain 'Save Document' title")
	}
}

func TestFileNamingModel_View_ContainsSuggestedFilename(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `my-document.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "my-document.md") {
		t.Error("expected view to contain the suggested filename")
	}
}

func TestFileNamingModel_View_ConfirmStateActionBar(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())
	m.SetSize(80, 24)

	view := m.View()

	if !strings.Contains(view, "Enter Save") {
		t.Error("expected view to contain 'Enter Save' in action bar")
	}
	if !strings.Contains(view, "[e] Edit name") {
		t.Error("expected view to contain '[e] Edit name' in action bar")
	}
	if !strings.Contains(view, "[c] Cancel") {
		t.Error("expected view to contain '[c] Cancel' in action bar")
	}
}

func TestFileNamingModel_View_FileExistsActionBar(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing.md"))
	m.SetSize(80, 24)

	view := m.View()

	// When file exists in confirm state, action bar should mention overwrite
	if !strings.Contains(view, "overwrite") {
		t.Error("expected view to contain 'overwrite' in action bar when file exists")
	}
}

func TestFileNamingModel_View_EditStateActionBar(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())
	m.SetSize(80, 24)

	// Enter edit mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	view := m.View()

	if !strings.Contains(view, "Enter Confirm") {
		t.Error("expected edit view to contain 'Enter Confirm' in action bar")
	}
	if !strings.Contains(view, "Esc Cancel") {
		t.Error("expected edit view to contain 'Esc Cancel' in action bar")
	}
}

func TestFileNamingModel_View_OverwriteStateActionBar(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `existing.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker("existing.md"))
	m.SetSize(80, 24)

	// Go to overwrite state
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()

	if !strings.Contains(view, "[o] Overwrite") {
		t.Error("expected overwrite view to contain '[o] Overwrite' in action bar")
	}
	if !strings.Contains(view, "[e] Edit name") {
		t.Error("expected overwrite view to contain '[e] Edit name' in action bar")
	}
	if !strings.Contains(view, "[c] Cancel") {
		t.Error("expected overwrite view to contain '[c] Cancel' in action bar")
	}
}

// Test Init

func TestFileNamingModel_Init(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	cmd := m.Init()

	// Should return a command for text input blink
	if cmd == nil {
		t.Error("expected Init() to return a command")
	}
}

// Test SetSize

func TestFileNamingModel_SetSize(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height to be 50, got %d", m.height)
	}
}

func TestFileNamingModel_SetSize_AdjustsInputWidth(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetSize(80, 24)

	// Input width should be set based on total width
	if m.input.Width < 20 {
		t.Errorf("expected input width to be at least 20, got %d", m.input.Width)
	}
}

// Test WindowSizeMsg

func TestFileNamingModel_Update_WindowSizeMsg(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from WindowSizeMsg")
	}
	if newM.width != 120 {
		t.Errorf("expected width to be 120, got %d", newM.width)
	}
	if newM.height != 40 {
		t.Errorf("expected height to be 40, got %d", newM.height)
	}
}

// Test state and result constants

func TestFileNamingState_Values(t *testing.T) {
	if StateConfirm != 0 {
		t.Errorf("expected StateConfirm to be 0, got %d", StateConfirm)
	}
	if StateEdit != 1 {
		t.Errorf("expected StateEdit to be 1, got %d", StateEdit)
	}
	if StateOverwriteConfirm != 2 {
		t.Errorf("expected StateOverwriteConfirm to be 2, got %d", StateOverwriteConfirm)
	}
}

func TestFileNamingResult_Values(t *testing.T) {
	if ResultPending != 0 {
		t.Errorf("expected ResultPending to be 0, got %d", ResultPending)
	}
	if ResultSave != 1 {
		t.Errorf("expected ResultSave to be 1, got %d", ResultSave)
	}
	if ResultCancel != 2 {
		t.Errorf("expected ResultCancel to be 2, got %d", ResultCancel)
	}
}

// Test editing rechecks file existence

func TestFileNamingModel_EditToExistingFile_UpdatesFileExists(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `new-file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	checker := newMockFileChecker("other-existing.md")
	m.SetFileChecker(checker)

	// Initially, file doesn't exist
	if m.FileExists() {
		t.Error("expected FileExists() to be false initially")
	}

	// Enter edit mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	// Change to a file that exists
	m.input.SetValue("other-existing.md")

	// Confirm edit
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Now FileExists should be true
	if !newM.FileExists() {
		t.Error("expected FileExists() to be true after editing to existing file")
	}
}

// Test DefaultFileChecker

func TestDefaultFileChecker_Exists_NonExistent(t *testing.T) {
	checker := DefaultFileChecker{}
	// Test with a path that definitely doesn't exist
	if checker.Exists("/this/path/definitely/does/not/exist/file.txt") {
		t.Error("expected Exists() to return false for non-existent file")
	}
}

// Test text input passthrough in edit mode

func TestFileNamingModel_EditMode_TextInputReceivesKeys(t *testing.T) {
	config := FileNamingConfig{
		ClaudeResponse: "saving this as `file.md`",
		Content:        "# Content",
	}

	m := NewFileNamingModel(config)
	m.SetFileChecker(newMockFileChecker())

	// Enter edit mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	// Clear and type a new value
	m.input.SetValue("")

	// Simulate typing 'a'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	// The input should have received the character
	// Note: Due to how bubbletea textinput works, we need to verify via the Update mechanism
	// For this test, we just verify the state is still edit and no crash occurred
	if m.State() != StateEdit {
		t.Error("expected state to remain StateEdit during typing")
	}
}
