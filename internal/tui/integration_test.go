package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/msgs"
	"github.com/pablasso/rafa/internal/tui/views"
)

// setupTestEnv creates a test environment with .rafa/ structure.
// Returns the rafaDir path and a cleanup function.
func setupTestEnv(t *testing.T) (repoRoot, rafaDir string) {
	t.Helper()

	tmpDir := t.TempDir()
	repoRoot = tmpDir

	// Create .git directory to simulate repo root
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	// Create .rafa directory structure
	rafaDir = filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create .rafa/plans dir: %v", err)
	}

	return repoRoot, rafaDir
}

// setupTestEnvWithDesignDoc creates a test environment with a design document.
func setupTestEnvWithDesignDoc(t *testing.T, designContent string) (repoRoot, rafaDir, designPath string) {
	t.Helper()

	repoRoot, rafaDir = setupTestEnv(t)

	// Create docs/designs directory
	designsDir := filepath.Join(repoRoot, "docs", "designs")
	if err := os.MkdirAll(designsDir, 0755); err != nil {
		t.Fatalf("failed to create docs/designs dir: %v", err)
	}

	// Create design document
	designPath = filepath.Join(designsDir, "feature.md")
	if err := os.WriteFile(designPath, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to write design.md: %v", err)
	}

	return repoRoot, rafaDir, designPath
}

// createTestPlan creates a plan.json file in the specified directory.
func createTestPlan(t *testing.T, plansDir, planID, planName, status string, tasks []plan.Task) string {
	t.Helper()

	folderName := planID + "-" + planName
	folderPath := filepath.Join(plansDir, folderName)
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		t.Fatalf("failed to create plan folder: %v", err)
	}

	p := plan.Plan{
		ID:     planID,
		Name:   planName,
		Status: status,
		Tasks:  tasks,
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal plan: %v", err)
	}

	planPath := filepath.Join(folderPath, "plan.json")
	if err := os.WriteFile(planPath, data, 0644); err != nil {
		t.Fatalf("failed to write plan.json: %v", err)
	}

	return folderPath
}

// createLockedPlan creates a plan with a .lock file to simulate it running elsewhere.
func createLockedPlan(t *testing.T, plansDir, planID, planName string, tasks []plan.Task) string {
	t.Helper()

	folderPath := createTestPlan(t, plansDir, planID, planName, plan.PlanStatusInProgress, tasks)

	lockPath := filepath.Join(folderPath, ".lock")
	if err := os.WriteFile(lockPath, []byte("locked"), 0644); err != nil {
		t.Fatalf("failed to create .lock file: %v", err)
	}

	return folderPath
}

// createTestModel creates a Model for testing with the given rafaDir.
func createTestModel(t *testing.T, repoRoot, rafaDir string) Model {
	t.Helper()

	m := Model{
		currentView: ViewHome,
		repoRoot:    repoRoot,
		rafaDir:     rafaDir,
	}
	m.home = views.NewHomeModel(rafaDir)
	m.home.SetSize(80, 24)

	return m
}

// mockAI represents a mock AI implementation for testing.
type mockAI struct {
	tasks []string
	name  string
	err   error
}

// setupMockAI sets up mock functions for plan creation.
// Returns a function to restore the original functions.
func setupMockAI(t *testing.T, mock mockAI) func() {
	t.Helper()

	originalExtractTasks := views.GetExtractTasksFunc()
	originalCreatePlanFolder := views.GetCreatePlanFolderFunc()
	originalCheckClaudeCLI := views.GetCheckClaudeCLIFunc()

	// Mock CLI check to always succeed
	views.SetCheckClaudeCLIFunc(func() error {
		return nil
	})

	// Mock task extraction
	views.SetExtractTasksFunc(func(ctx context.Context, content string) (*plan.TaskExtractionResult, error) {
		if mock.err != nil {
			return nil, mock.err
		}

		extractedTasks := make([]plan.ExtractedTask, len(mock.tasks))
		for i, title := range mock.tasks {
			extractedTasks[i] = plan.ExtractedTask{
				Title:              title,
				Description:        "Test task description",
				AcceptanceCriteria: []string{"Test criterion"},
			}
		}

		return &plan.TaskExtractionResult{
			Name:        mock.name,
			Description: "Test plan description",
			Tasks:       extractedTasks,
		}, nil
	})

	// Mock plan folder creation to do nothing
	views.SetCreatePlanFolderFunc(func(p *plan.Plan) error {
		return nil
	})

	return func() {
		views.SetExtractTasksFunc(originalExtractTasks)
		views.SetCreatePlanFolderFunc(originalCreatePlanFolder)
		views.SetCheckClaudeCLIFunc(originalCheckClaudeCLI)
	}
}

// sendKey simulates sending a key press to the model.
func sendKey(t *testing.T, m *Model, key string) tea.Cmd {
	t.Helper()

	var keyMsg tea.KeyMsg
	switch key {
	case "up":
		keyMsg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		keyMsg = tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		keyMsg = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		if len(key) == 1 {
			keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		} else {
			t.Fatalf("unknown key: %s", key)
		}
	}

	newModel, cmd := m.Update(keyMsg)
	*m = newModel.(Model)
	return cmd
}

// sendWindowSize simulates a window resize event.
func sendWindowSize(t *testing.T, m *Model, width, height int) tea.Cmd {
	t.Helper()

	msg := tea.WindowSizeMsg{Width: width, Height: height}
	newModel, cmd := m.Update(msg)
	*m = newModel.(Model)
	return cmd
}

// processCmd processes a command and returns the resulting message.
func processCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// TestCreatePlanFlow tests the complete create flow:
// Home → FilePicker → Creating → success → Running → Home
func TestCreatePlanFlow(t *testing.T) {
	_, rafaDir, designPath := setupTestEnvWithDesignDoc(t, `# Test Feature
This is a test design document.

## Tasks
1. First task
2. Second task
`)

	// Setup mock AI
	restore := setupMockAI(t, mockAI{
		tasks: []string{"First task", "Second task"},
		name:  "test-feature",
	})
	defer restore()

	// Create model starting at Home
	m := createTestModel(t, filepath.Dir(rafaDir), rafaDir)
	sendWindowSize(t, &m, 80, 24)

	// Verify starting at Home view
	if m.currentView != ViewHome {
		t.Fatalf("expected to start at ViewHome, got %d", m.currentView)
	}

	// Press 'c' to go to FilePicker
	cmd := sendKey(t, &m, "c")
	if cmd == nil {
		t.Fatal("expected command from 'c' key")
	}

	// Process the command to get the transition message
	msg := processCmd(cmd)
	if _, ok := msg.(msgs.GoToFilePickerMsg); !ok {
		t.Fatalf("expected GoToFilePickerMsg, got %T", msg)
	}

	// Simulate view transition
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Verify now at FilePicker view
	if m.currentView != ViewFilePicker {
		t.Fatalf("expected ViewFilePicker, got %d", m.currentView)
	}
	sendWindowSize(t, &m, 80, 24)

	// Simulate file selection by sending FileSelectedMsg directly
	// (The actual filepicker navigation is tested in filepicker_test.go)
	fileMsg := msgs.FileSelectedMsg{Path: designPath}
	newModel, _ = m.Update(fileMsg)
	m = newModel.(Model)

	// Verify now at PlanCreate view
	if m.currentView != ViewPlanCreate {
		t.Fatalf("expected ViewPlanCreate, got %d", m.currentView)
	}
	sendWindowSize(t, &m, 80, 24)

	// Simulate successful plan save (using the new message type)
	planSavedMsg := views.PlanCreateSavedMsg{
		PlanID: "abc123-test-feature",
		Tasks:  []string{"First task", "Second task"},
	}
	newModel, _ = m.Update(planSavedMsg)
	m = newModel.(Model)

	// View should still be PlanCreate but in completed state
	if m.currentView != ViewPlanCreate {
		t.Fatalf("expected ViewPlanCreate after success, got %d", m.currentView)
	}

	// Verify success view renders correctly
	view := m.View()
	if !strings.Contains(view, "Plan saved") {
		t.Error("expected view to contain 'Plan saved'")
	}

	// Press 'r' to run the plan
	cmd = sendKey(t, &m, "r")
	if cmd == nil {
		t.Fatal("expected command from 'r' key in success state")
	}

	// Process the RunPlanMsg
	msg = processCmd(cmd)
	runMsg, ok := msg.(msgs.RunPlanMsg)
	if !ok {
		t.Fatalf("expected RunPlanMsg, got %T", msg)
	}
	if runMsg.PlanID != "abc123-test-feature" {
		t.Errorf("expected plan ID abc123-test-feature, got %s", runMsg.PlanID)
	}
}

// TestRunExistingPlanFlow tests the run existing plan flow:
// Home → PlanList → Running → Home
func TestRunExistingPlanFlow(t *testing.T) {
	repoRoot, rafaDir := setupTestEnv(t)
	plansDir := filepath.Join(rafaDir, "plans")

	// Create an existing plan
	createTestPlan(t, plansDir, "xyz789", "existing-plan", plan.PlanStatusNotStarted, []plan.Task{
		{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
	})

	// Create model starting at Home
	m := createTestModel(t, repoRoot, rafaDir)
	sendWindowSize(t, &m, 80, 24)

	// Verify starting at Home view
	if m.currentView != ViewHome {
		t.Fatalf("expected to start at ViewHome, got %d", m.currentView)
	}

	// Press 'r' to go to PlanList
	cmd := sendKey(t, &m, "r")
	if cmd == nil {
		t.Fatal("expected command from 'r' key")
	}

	// Process the command to get the transition message
	msg := processCmd(cmd)
	if _, ok := msg.(msgs.GoToPlanListMsg); !ok {
		t.Fatalf("expected GoToPlanListMsg, got %T", msg)
	}

	// Simulate view transition
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Verify now at PlanList view
	if m.currentView != ViewPlanList {
		t.Fatalf("expected ViewPlanList, got %d", m.currentView)
	}
	sendWindowSize(t, &m, 80, 24)

	// Verify the plan is visible in the view
	view := m.View()
	if !strings.Contains(view, "xyz789-existing-plan") {
		t.Error("expected view to contain plan ID 'xyz789-existing-plan'")
	}

	// Press Enter to run the selected plan
	cmd = sendKey(t, &m, "enter")
	if cmd == nil {
		t.Fatal("expected command from Enter key")
	}

	// Process the RunPlanMsg
	msg = processCmd(cmd)
	runMsg, ok := msg.(msgs.RunPlanMsg)
	if !ok {
		t.Fatalf("expected RunPlanMsg, got %T", msg)
	}
	if runMsg.PlanID != "xyz789-existing-plan" {
		t.Errorf("expected plan ID xyz789-existing-plan, got %s", runMsg.PlanID)
	}

	// Process run plan transition - this actually loads and starts Running view
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	// Verify now at Running view
	if m.currentView != ViewRunning {
		t.Fatalf("expected ViewRunning, got %d", m.currentView)
	}
	sendWindowSize(t, &m, 100, 40)

	// Simulate execution done
	doneMsg := msgs.ExecutionDoneMsg{Success: true}
	newModel, _ = m.Update(doneMsg)
	m = newModel.(Model)

	// Verify returned to Home view
	if m.currentView != ViewHome {
		t.Fatalf("expected ViewHome after execution done, got %d", m.currentView)
	}
}

// TestKeyboardNavigation tests arrow key navigation and Enter/Escape in all views.
func TestKeyboardNavigation(t *testing.T) {
	t.Run("Home arrow navigation", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Initial cursor should be at 0
		if m.home.Cursor() != 0 {
			t.Fatalf("expected cursor at 0, got %d", m.home.Cursor())
		}

		// Navigate down through all 5 items (Define: p, d; Execute: c, r; Quit: q)
		sendKey(t, &m, "down")
		if m.home.Cursor() != 1 {
			t.Errorf("expected cursor at 1 after down, got %d", m.home.Cursor())
		}

		sendKey(t, &m, "down")
		if m.home.Cursor() != 2 {
			t.Errorf("expected cursor at 2 after second down, got %d", m.home.Cursor())
		}

		sendKey(t, &m, "down")
		if m.home.Cursor() != 3 {
			t.Errorf("expected cursor at 3 after third down, got %d", m.home.Cursor())
		}

		sendKey(t, &m, "down")
		if m.home.Cursor() != 4 {
			t.Errorf("expected cursor at 4 after fourth down, got %d", m.home.Cursor())
		}

		// Navigate past end (should stay at 4 - Quit)
		sendKey(t, &m, "down")
		if m.home.Cursor() != 4 {
			t.Errorf("expected cursor to stay at 4, got %d", m.home.Cursor())
		}

		// Navigate up through all items
		sendKey(t, &m, "up")
		if m.home.Cursor() != 3 {
			t.Errorf("expected cursor at 3 after up, got %d", m.home.Cursor())
		}

		sendKey(t, &m, "up")
		if m.home.Cursor() != 2 {
			t.Errorf("expected cursor at 2 after second up, got %d", m.home.Cursor())
		}

		sendKey(t, &m, "up")
		if m.home.Cursor() != 1 {
			t.Errorf("expected cursor at 1 after third up, got %d", m.home.Cursor())
		}

		sendKey(t, &m, "up")
		if m.home.Cursor() != 0 {
			t.Errorf("expected cursor at 0 after fourth up, got %d", m.home.Cursor())
		}

		// Navigate past beginning (should stay)
		sendKey(t, &m, "up")
		if m.home.Cursor() != 0 {
			t.Errorf("expected cursor to stay at 0, got %d", m.home.Cursor())
		}
	})

	t.Run("Home Enter activates selection", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Press Enter on first item (Create PRD) - sends GoToConversationMsg
		cmd := sendKey(t, &m, "enter")
		if cmd == nil {
			t.Fatal("expected command from Enter")
		}

		msg := processCmd(cmd)
		if _, ok := msg.(msgs.GoToConversationMsg); !ok {
			t.Errorf("expected GoToConversationMsg, got %T", msg)
		}
	})

	t.Run("FilePicker Escape goes back", func(t *testing.T) {
		// Need design docs for "c" key (Create Plan) to go to FilePicker
		repoRoot, rafaDir, _ := setupTestEnvWithDesignDoc(t, "# Test Design\n")
		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to FilePicker (via Create Plan which requires design docs)
		cmd := sendKey(t, &m, "c")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)

		if m.currentView != ViewFilePicker {
			t.Fatalf("expected ViewFilePicker, got %d", m.currentView)
		}
		sendWindowSize(t, &m, 80, 24)

		// Press Escape to go back
		cmd = sendKey(t, &m, "esc")
		if cmd == nil {
			t.Fatal("expected command from Escape")
		}

		msg = processCmd(cmd)
		if _, ok := msg.(msgs.GoToHomeMsg); !ok {
			t.Errorf("expected GoToHomeMsg, got %T", msg)
		}
	})

	t.Run("PlanList arrow navigation", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		plansDir := filepath.Join(rafaDir, "plans")

		createTestPlan(t, plansDir, "plan1", "test-a", plan.PlanStatusNotStarted, nil)
		createTestPlan(t, plansDir, "plan2", "test-b", plan.PlanStatusNotStarted, nil)
		createTestPlan(t, plansDir, "plan3", "test-c", plan.PlanStatusNotStarted, nil)

		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to PlanList
		cmd := sendKey(t, &m, "r")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)

		if m.currentView != ViewPlanList {
			t.Fatalf("expected ViewPlanList, got %d", m.currentView)
		}
		sendWindowSize(t, &m, 80, 24)

		// Initial cursor at 0
		if m.planList.Cursor() != 0 {
			t.Fatalf("expected cursor at 0, got %d", m.planList.Cursor())
		}

		// Navigate down
		sendKey(t, &m, "down")
		if m.planList.Cursor() != 1 {
			t.Errorf("expected cursor at 1 after down, got %d", m.planList.Cursor())
		}

		// Navigate down again
		sendKey(t, &m, "down")
		if m.planList.Cursor() != 2 {
			t.Errorf("expected cursor at 2 after second down, got %d", m.planList.Cursor())
		}
	})

	t.Run("PlanList Escape goes back", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		plansDir := filepath.Join(rafaDir, "plans")
		createTestPlan(t, plansDir, "plan1", "test", plan.PlanStatusNotStarted, nil)

		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to PlanList
		cmd := sendKey(t, &m, "r")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		// Press Escape to go back
		cmd = sendKey(t, &m, "esc")
		if cmd == nil {
			t.Fatal("expected command from Escape")
		}

		msg = processCmd(cmd)
		if _, ok := msg.(msgs.GoToHomeMsg); !ok {
			t.Errorf("expected GoToHomeMsg, got %T", msg)
		}
	})

	t.Run("Home q quits", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		cmd := sendKey(t, &m, "q")
		if cmd == nil {
			t.Fatal("expected command from 'q'")
		}

		msg := processCmd(cmd)
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})
}

// TestWindowResize tests that layout adapts to size changes.
func TestWindowResize(t *testing.T) {
	t.Run("Layout adapts to size changes", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		m := createTestModel(t, repoRoot, rafaDir)

		// Set initial size
		sendWindowSize(t, &m, 80, 24)

		view1 := m.View()
		if view1 == "" {
			t.Error("expected non-empty view at 80x24")
		}

		// Resize to larger
		sendWindowSize(t, &m, 120, 40)

		view2 := m.View()
		if view2 == "" {
			t.Error("expected non-empty view at 120x40")
		}

		// Views should be different due to different centering
		if view1 == view2 {
			t.Error("expected views to differ for different sizes")
		}

		// Resize to smaller
		sendWindowSize(t, &m, 60, 15)

		view3 := m.View()
		if view3 == "" {
			t.Error("expected non-empty view at 60x15")
		}
	})

	t.Run("Minimum size warning appears", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		m := createTestModel(t, repoRoot, rafaDir)

		// Set size below minimum
		sendWindowSize(t, &m, MinTerminalWidth-1, MinTerminalHeight)

		view := m.View()
		if !strings.Contains(view, "Terminal too small") {
			t.Error("expected 'Terminal too small' warning for width below minimum")
		}
		if !strings.Contains(view, "Minimum:") {
			t.Error("expected minimum dimensions in warning")
		}
		if !strings.Contains(view, "Current:") {
			t.Error("expected current dimensions in warning")
		}

		// Test height below minimum
		sendWindowSize(t, &m, MinTerminalWidth, MinTerminalHeight-1)

		view = m.View()
		if !strings.Contains(view, "Terminal too small") {
			t.Error("expected 'Terminal too small' warning for height below minimum")
		}

		// Test both below minimum
		sendWindowSize(t, &m, MinTerminalWidth-10, MinTerminalHeight-5)

		view = m.View()
		if !strings.Contains(view, "Terminal too small") {
			t.Error("expected 'Terminal too small' warning for both below minimum")
		}

		// Test exactly at minimum - should NOT show warning
		sendWindowSize(t, &m, MinTerminalWidth, MinTerminalHeight)

		view = m.View()
		if strings.Contains(view, "Terminal too small") {
			t.Error("should NOT show warning at exactly minimum size")
		}
	})

	t.Run("All views respond to resize", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		plansDir := filepath.Join(rafaDir, "plans")
		createTestPlan(t, plansDir, "plan1", "test", plan.PlanStatusNotStarted, []plan.Task{
			{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
		})

		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Test FilePicker view resize
		cmd := sendKey(t, &m, "c")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 100, 40)

		// Should not panic or produce empty view
		view := m.View()
		if m.currentView == ViewFilePicker && view == "" {
			t.Error("expected non-empty view for FilePicker after resize")
		}

		// Go back home
		cmd = sendKey(t, &m, "esc")
		msg = processCmd(cmd)
		newModel, _ = m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		// Test PlanList view resize
		cmd = sendKey(t, &m, "r")
		msg = processCmd(cmd)
		newModel, _ = m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 100, 40)

		view = m.View()
		if m.currentView == ViewPlanList && view == "" {
			t.Error("expected non-empty view for PlanList after resize")
		}
	})
}

// TestErrorStates tests various error scenarios.
func TestErrorStates(t *testing.T) {
	t.Run("No .rafa directory shows init message", func(t *testing.T) {
		// Create just a temp directory without .rafa
		tmpDir := t.TempDir()

		// Create .git but NOT .rafa
		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create .git dir: %v", err)
		}

		m := Model{
			currentView: ViewHome,
			repoRoot:    tmpDir,
			rafaDir:     filepath.Join(tmpDir, ".rafa"), // Does not exist
		}
		m.home = views.NewHomeModel(m.rafaDir)
		m.home.SetSize(80, 24)
		m.width = 80
		m.height = 24

		view := m.View()

		if !strings.Contains(view, "No .rafa/ directory found") {
			t.Error("expected view to contain 'No .rafa/ directory found'")
		}
		if !strings.Contains(view, "rafa init") {
			t.Error("expected view to contain 'rafa init' instruction")
		}
	})

	t.Run("Empty plan list shows empty state", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		// Don't create any plans

		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to PlanList
		cmd := sendKey(t, &m, "r")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		view := m.View()

		if !strings.Contains(view, "No plans found") {
			t.Error("expected view to contain 'No plans found'")
		}
		if !strings.Contains(view, "Press 'c' to create") {
			t.Error("expected view to contain create instruction")
		}
	})

	t.Run("Extraction error shows error display", func(t *testing.T) {
		_, rafaDir, designPath := setupTestEnvWithDesignDoc(t, "# Test")

		m := createTestModel(t, filepath.Dir(rafaDir), rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to FilePicker and select file
		cmd := sendKey(t, &m, "c")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		// Select file
		fileMsg := msgs.FileSelectedMsg{Path: designPath}
		newModel, _ = m.Update(fileMsg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		// Simulate extraction error (using the new PlanCreate model message)
		errMsg := views.PlanCreateErrorMsg{Err: os.ErrNotExist}
		newModel, _ = m.Update(errMsg)
		m = newModel.(Model)

		view := m.View()

		if !strings.Contains(view, "Error") {
			t.Error("expected view to contain error message")
		}
		if !strings.Contains(view, "Home") {
			t.Error("expected view to contain 'Home' option")
		}
	})

	t.Run("Locked plan shows locked state and prevents selection", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		plansDir := filepath.Join(rafaDir, "plans")

		// Create a locked plan
		createLockedPlan(t, plansDir, "lock1", "locked-plan", []plan.Task{
			{ID: "t01", Title: "Task", Status: plan.TaskStatusPending},
		})

		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to PlanList
		cmd := sendKey(t, &m, "r")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		// View should show locked indicator
		view := m.View()
		if !strings.Contains(view, "locked") {
			t.Error("expected view to indicate plan is locked")
		}

		// Try to select the locked plan
		cmd = sendKey(t, &m, "enter")
		if cmd != nil {
			msg = processCmd(cmd)
			// Should not transition to RunPlanMsg for locked plan
			if _, ok := msg.(msgs.RunPlanMsg); ok {
				t.Error("should not be able to run a locked plan")
			}
		}

		// Should still be at PlanList and show error
		if m.currentView != ViewPlanList {
			t.Error("should remain at PlanList when selecting locked plan")
		}

		view = m.View()
		if !strings.Contains(view, "Plan is running elsewhere") {
			t.Error("expected error message about plan running elsewhere")
		}
	})
}

// TestCancellation tests Ctrl+C behavior in different views.
func TestCancellation(t *testing.T) {
	t.Run("Ctrl+C during PlanCreate sets cancelled state", func(t *testing.T) {
		_, rafaDir, designPath := setupTestEnvWithDesignDoc(t, "# Test")

		m := createTestModel(t, filepath.Dir(rafaDir), rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Navigate to PlanCreate view
		cmd := sendKey(t, &m, "c")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		// Select file to go to PlanCreate
		fileMsg := msgs.FileSelectedMsg{Path: designPath}
		newModel, _ = m.Update(fileMsg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		if m.currentView != ViewPlanCreate {
			t.Fatalf("expected ViewPlanCreate, got %d", m.currentView)
		}

		// Press Ctrl+C - in the new flow, this sets cancelled state (not return to FilePicker)
		_ = sendKey(t, &m, "ctrl+c")

		// Verify the model is in cancelled state
		if m.planCreate.State() != views.PlanCreateStateCancelled {
			t.Fatalf("expected PlanCreateStateCancelled, got %d", m.planCreate.State())
		}
	})

	t.Run("Ctrl+C during Running shows summary then can return Home", func(t *testing.T) {
		repoRoot, rafaDir := setupTestEnv(t)
		plansDir := filepath.Join(rafaDir, "plans")

		createTestPlan(t, plansDir, "test1", "running-test", plan.PlanStatusNotStarted, []plan.Task{
			{ID: "t01", Title: "Task One", Status: plan.TaskStatusPending},
			{ID: "t02", Title: "Task Two", Status: plan.TaskStatusPending},
		})

		m := createTestModel(t, repoRoot, rafaDir)
		sendWindowSize(t, &m, 80, 24)

		// Go to Running view
		cmd := sendKey(t, &m, "r")
		msg := processCmd(cmd)
		newModel, _ := m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 80, 24)

		cmd = sendKey(t, &m, "enter")
		msg = processCmd(cmd)
		newModel, _ = m.Update(msg)
		m = newModel.(Model)
		sendWindowSize(t, &m, 100, 40)

		if m.currentView != ViewRunning {
			t.Fatalf("expected ViewRunning, got %d", m.currentView)
		}

		// Press Ctrl+C
		sendKey(t, &m, "ctrl+c")

		// Should show cancelled state
		view := m.View()
		if !strings.Contains(view, "Cancelled") && !strings.Contains(view, "Stopped") {
			t.Error("expected view to show cancelled/stopped state")
		}

		// Should show task summary
		if !strings.Contains(view, "Task Summary") {
			t.Error("expected view to show task summary")
		}

		// Press Enter to return home
		cmd = sendKey(t, &m, "enter")
		if cmd == nil {
			t.Fatal("expected command from Enter in cancelled state")
		}

		msg = processCmd(cmd)
		if _, ok := msg.(msgs.GoToHomeMsg); !ok {
			t.Fatalf("expected GoToHomeMsg, got %T", msg)
		}
	})
}
