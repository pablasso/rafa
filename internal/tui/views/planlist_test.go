package views

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/msgs"
)

// createTestPlan creates a plan.json file in the specified directory.
func createTestPlan(t *testing.T, plansDir, planID, planName, status string, tasks []plan.Task) {
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
}

func writeLiveLockFile(t *testing.T, lockFile string) {
	t.Helper()
	if err := os.WriteFile(lockFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}
}

func TestNewPlanListModel_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	if err := os.MkdirAll(filepath.Join(rafaDir, "plans"), 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	m := NewPlanListModel(rafaDir)

	if len(m.Plans()) != 0 {
		t.Errorf("expected 0 plans, got %d", len(m.Plans()))
	}
	if m.Cursor() != 0 {
		t.Errorf("expected cursor to be 0, got %d", m.Cursor())
	}
	if m.RafaDir() != rafaDir {
		t.Errorf("expected rafaDir to be %s, got %s", rafaDir, m.RafaDir())
	}
}

func TestNewPlanListModel_NonExistentDirectory(t *testing.T) {
	m := NewPlanListModel("/nonexistent/.rafa")

	if len(m.Plans()) != 0 {
		t.Errorf("expected 0 plans for nonexistent dir, got %d", len(m.Plans()))
	}
}

func TestNewPlanListModel_LoadsPlans(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create test plans
	createTestPlan(t, plansDir, "xK9pQ2", "feature-auth", plan.PlanStatusNotStarted, []plan.Task{
		{ID: "t01", Status: plan.TaskStatusPending},
		{ID: "t02", Status: plan.TaskStatusPending},
		{ID: "t03", Status: plan.TaskStatusPending},
		{ID: "t04", Status: plan.TaskStatusPending},
	})

	createTestPlan(t, plansDir, "mN3jL7", "refactor-db", plan.PlanStatusCompleted, []plan.Task{
		{ID: "t01", Status: plan.TaskStatusCompleted},
		{ID: "t02", Status: plan.TaskStatusCompleted},
	})

	createTestPlan(t, plansDir, "pR8wK1", "add-tests", plan.PlanStatusInProgress, []plan.Task{
		{ID: "t01", Status: plan.TaskStatusCompleted},
		{ID: "t02", Status: plan.TaskStatusCompleted},
		{ID: "t03", Status: plan.TaskStatusPending},
	})

	m := NewPlanListModel(rafaDir)

	if len(m.Plans()) != 3 {
		t.Errorf("expected 3 plans, got %d", len(m.Plans()))
	}

	// Find the in-progress plan and check completed count
	for _, p := range m.Plans() {
		if p.ID == "pR8wK1" {
			if p.Completed != 2 {
				t.Errorf("expected 2 completed tasks for in-progress plan, got %d", p.Completed)
			}
			if p.TaskCount != 3 {
				t.Errorf("expected 3 total tasks, got %d", p.TaskCount)
			}
		}
	}
}

func TestNewPlanListModel_StartsCursorOnFirstRunnablePlan(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create a locked in-progress plan.
	createTestPlan(t, plansDir, "locked", "alpha", plan.PlanStatusInProgress, nil)
	lockedPlanDir := filepath.Join(plansDir, "locked-alpha")
	lockFile := filepath.Join(lockedPlanDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	// Create an unlocked not-started plan.
	createTestPlan(t, plansDir, "open", "beta", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)
	if m.Cursor() != 0 {
		t.Fatalf("expected cursor at first runnable plan, got %d", m.Cursor())
	}
	if len(m.Plans()) < 1 {
		t.Fatal("expected at least one plan")
	}
	if m.Plans()[m.Cursor()].Locked {
		t.Fatal("expected initial cursor to land on an unlocked plan")
	}
}

func TestNewPlanListModel_GroupsAndSortsPlans(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "ip2", "zzz-progress", plan.PlanStatusInProgress, nil)
	createTestPlan(t, plansDir, "ip1", "aaa-progress", plan.PlanStatusInProgress, nil)
	createTestPlan(t, plansDir, "fd1", "mmm-failed", plan.PlanStatusFailed, nil)
	createTestPlan(t, plansDir, "ns1", "bbb-not-started", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "uk1", "ccc-unknown", "custom_state", nil)

	// Locked completed should be in locked section due lock precedence.
	createTestPlan(t, plansDir, "lc1", "locked-completed", plan.PlanStatusCompleted, nil)
	lockedPlanDir := filepath.Join(plansDir, "lc1-locked-completed")
	lockFile := filepath.Join(lockedPlanDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	// Unlocked completed should stay in completed section.
	createTestPlan(t, plansDir, "cp1", "done", plan.PlanStatusCompleted, nil)

	m := NewPlanListModel(rafaDir)
	got := m.Plans()
	wantIDs := []string{"ip1", "ip2", "fd1", "ns1", "uk1", "lc1", "cp1"}

	if len(got) != len(wantIDs) {
		t.Fatalf("expected %d plans, got %d", len(wantIDs), len(got))
	}

	for i, want := range wantIDs {
		if got[i].ID != want {
			t.Fatalf("expected position %d to be %q, got %q", i, want, got[i].ID)
		}
	}

	if !got[5].Locked {
		t.Fatal("expected locked-completed plan to be in locked section with Locked=true")
	}
	if got[6].Status != plan.PlanStatusCompleted || got[6].Locked {
		t.Fatal("expected final plan to be unlocked completed")
	}
}

func TestPlanListModel_Init(t *testing.T) {
	m := NewPlanListModel("")
	cmd := m.Init()

	if cmd != nil {
		t.Error("expected Init() to return nil")
	}
}

func TestPlanListModel_Update_WindowSizeMsg(t *testing.T) {
	m := NewPlanListModel("")
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}

	newM, cmd := m.Update(msg)

	if cmd != nil {
		t.Error("expected no command from WindowSizeMsg")
	}
	if newM.width != 80 {
		t.Errorf("expected width to be 80, got %d", newM.width)
	}
	if newM.height != 24 {
		t.Errorf("expected height to be 24, got %d", newM.height)
	}
}

func TestPlanListModel_Update_EmptyState_EscReturnsHome(t *testing.T) {
	m := NewPlanListModel("/nonexistent/.rafa")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected command from Esc in empty state")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestPlanListModel_Update_EmptyState_CReturnsFilePicker(t *testing.T) {
	m := NewPlanListModel("/nonexistent/.rafa")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if cmd == nil {
		t.Fatal("expected command from 'c' in empty state")
	}

	msg := cmd()
	fpMsg, ok := msg.(msgs.GoToFilePickerMsg)
	if !ok {
		t.Errorf("expected msgs.GoToFilePickerMsg, got %T", msg)
	}
	if !fpMsg.ForPlanCreation {
		t.Error("expected ForPlanCreation to be true")
	}
}

func TestPlanListModel_Update_EmptyState_CtrlCQuits(t *testing.T) {
	m := NewPlanListModel("/nonexistent/.rafa")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("expected command from Ctrl+C")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestPlanListModel_Update_NavigateDown(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "plan1", "test1", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan2", "test2", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan3", "test3", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)

	// Navigate down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after down, got %d", newM.cursor)
	}

	// Navigate down again
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to be 2 after second down, got %d", newM.cursor)
	}

	// Try to navigate past the end
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newM.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", newM.cursor)
	}
}

func TestPlanListModel_Update_NavigateUp(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "plan1", "test1", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan2", "test2", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan3", "test3", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)
	m.cursor = 2 // Start at bottom

	// Navigate up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after up, got %d", newM.cursor)
	}

	// Navigate up again
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to be 0 after second up, got %d", newM.cursor)
	}

	// Try to navigate past the beginning
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", newM.cursor)
	}
}

func TestPlanListModel_Update_VimNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "plan1", "test1", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan2", "test2", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)

	// Navigate down with j
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newM.cursor != 1 {
		t.Errorf("expected cursor to be 1 after 'j', got %d", newM.cursor)
	}

	// Navigate up with k
	newM, _ = newM.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newM.cursor != 0 {
		t.Errorf("expected cursor to be 0 after 'k', got %d", newM.cursor)
	}
}

func TestPlanListModel_Update_EnterReturnsRunPlanMsg(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "xK9pQ2", "feature-auth", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("expected command from Enter")
	}

	msg := cmd()
	runMsg, ok := msg.(msgs.RunPlanMsg)
	if !ok {
		t.Fatalf("expected msgs.RunPlanMsg, got %T", msg)
	}
	// PlanID should be in "shortID-name" format
	if runMsg.PlanID != "xK9pQ2-feature-auth" {
		t.Errorf("expected PlanID to be 'xK9pQ2-feature-auth', got %s", runMsg.PlanID)
	}
}

func TestPlanListModel_Update_EscReturnsHome(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "plan1", "test1", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected command from Esc")
	}

	msg := cmd()
	if _, ok := msg.(msgs.GoToHomeMsg); !ok {
		t.Errorf("expected msgs.GoToHomeMsg, got %T", msg)
	}
}

func TestPlanListModel_View_EmptyDimensions(t *testing.T) {
	m := NewPlanListModel("")

	view := m.View()

	if view != "" {
		t.Errorf("expected empty view when dimensions are 0, got: %s", view)
	}
}

func TestPlanListModel_View_EmptyState(t *testing.T) {
	m := NewPlanListModel("/nonexistent/.rafa")
	m.SetSize(80, 24)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "Select Plan to Run") {
		t.Error("expected view to contain 'Select Plan to Run'")
	}

	// Check for empty state message
	if !strings.Contains(view, "No plans found.") {
		t.Error("expected view to contain 'No plans found.'")
	}

	// Check for hint
	if !strings.Contains(view, "Press 'c' to create a new plan") {
		t.Error("expected view to contain create plan hint")
	}

	// Check status bar
	if !strings.Contains(view, "c Create plan") {
		t.Error("expected view to contain 'c Create plan' in status bar")
	}
	if !strings.Contains(view, "Esc Back") {
		t.Error("expected view to contain 'Esc Back' in status bar")
	}
}

func TestPlanListModel_View_WithPlans(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "xK9pQ2", "feature-auth", plan.PlanStatusNotStarted, []plan.Task{
		{ID: "t01", Status: plan.TaskStatusPending},
		{ID: "t02", Status: plan.TaskStatusPending},
		{ID: "t03", Status: plan.TaskStatusPending},
		{ID: "t04", Status: plan.TaskStatusPending},
	})

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	view := m.View()

	// Check for title
	if !strings.Contains(view, "Select Plan to Run") {
		t.Error("expected view to contain 'Select Plan to Run'")
	}

	// Check for plan ID
	if !strings.Contains(view, "xK9pQ2-feature-auth") {
		t.Error("expected view to contain 'xK9pQ2-feature-auth'")
	}
	if !strings.Contains(view, "Ready to Run (1)") {
		t.Error("expected view to contain Ready to Run section heading")
	}

	// Check for task count
	if !strings.Contains(view, "4 tasks") {
		t.Error("expected view to contain '4 tasks'")
	}

	// Check for status
	if !strings.Contains(view, "not_started") {
		t.Error("expected view to contain 'not_started'")
	}

	// Check for selection indicator (selected)
	if !strings.Contains(view, "●") {
		t.Error("expected view to contain '●' for selected item")
	}

	// Check status bar
	if !strings.Contains(view, "↑↓ Navigate") {
		t.Error("expected view to contain '↑↓ Navigate' in status bar")
	}
	if !strings.Contains(view, "Enter Run") {
		t.Error("expected view to contain 'Enter Run' in status bar")
	}
	if !strings.Contains(view, "Esc Back") {
		t.Error("expected view to contain 'Esc Back' in status bar")
	}
}

func TestPlanListModel_View_HidesEmptySections(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "n1", "only-ready", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)
	view := m.View()

	if !strings.Contains(view, "Ready to Run (1)") {
		t.Error("expected Ready to Run section")
	}
	if strings.Contains(view, "Running Elsewhere (") {
		t.Error("did not expect Running Elsewhere section when empty")
	}
	if strings.Contains(view, "Completed (") {
		t.Error("did not expect Completed section when empty")
	}
}

func TestPlanListModel_View_CompletedSectionVisibleAndSelectable(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "n1", "ready", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "c1", "done", plan.PlanStatusCompleted, nil)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)
	view := m.View()

	if !strings.Contains(view, "Completed (1)") {
		t.Fatal("expected Completed section heading")
	}

	// Move selection from ready to completed and ensure Enter still runs.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected run command when selecting completed plan")
	}
	msg := cmd()
	runMsg, ok := msg.(msgs.RunPlanMsg)
	if !ok {
		t.Fatalf("expected msgs.RunPlanMsg, got %T", msg)
	}
	if runMsg.PlanID != "c1-done" {
		t.Fatalf("expected completed plan run id c1-done, got %s", runMsg.PlanID)
	}
}

func TestPlanListModel_View_InProgressStatus(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "pR8wK1", "add-tests", plan.PlanStatusInProgress, []plan.Task{
		{ID: "t01", Status: plan.TaskStatusCompleted},
		{ID: "t02", Status: plan.TaskStatusCompleted},
		{ID: "t03", Status: plan.TaskStatusPending},
	})

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	view := m.View()

	// Check for in_progress with fraction
	if !strings.Contains(view, "in_progress (2/3)") {
		t.Error("expected view to contain 'in_progress (2/3)'")
	}
}

func TestPlanListModel_View_UnselectedIndicator(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "plan1", "test1", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan2", "test2", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	view := m.View()

	// Check for both indicators (one selected, one not)
	if !strings.Contains(view, "●") {
		t.Error("expected view to contain '●' for selected item")
	}
	if !strings.Contains(view, "○") {
		t.Error("expected view to contain '○' for unselected item")
	}
}

func TestPlanListModel_View_SingleTask(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "single", "task", plan.PlanStatusNotStarted, []plan.Task{
		{ID: "t01", Status: plan.TaskStatusPending},
	})

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	view := m.View()

	// Check for singular "task" instead of "tasks"
	if !strings.Contains(view, "1 task") {
		t.Error("expected view to contain '1 task' (singular)")
	}
	if strings.Contains(view, "1 tasks") {
		t.Error("expected view NOT to contain '1 tasks'")
	}
}

func TestPlanListModel_SetSize(t *testing.T) {
	m := NewPlanListModel("")
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("expected width to be 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height to be 50, got %d", m.height)
	}
}

func TestPlanListModel_Selection_ChangesOnNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	createTestPlan(t, plansDir, "plan1", "test1", plan.PlanStatusNotStarted, nil)
	createTestPlan(t, plansDir, "plan2", "test2", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	// Initial view - cursor at 0
	view1 := m.View()

	// Navigate down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	view2 := m.View()

	// Views should be different (different selection styling)
	if view1 == view2 {
		t.Error("expected views to differ after navigation")
	}
}

func TestPlanListModel_LockedPlan_Detection(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create a plan without lock
	createTestPlan(t, plansDir, "unlocked", "test1", plan.PlanStatusNotStarted, nil)

	// Create a plan with lock
	createTestPlan(t, plansDir, "locked", "test2", plan.PlanStatusInProgress, nil)
	lockedPlanDir := filepath.Join(plansDir, "locked-test2")
	lockFile := filepath.Join(lockedPlanDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	m := NewPlanListModel(rafaDir)

	if len(m.Plans()) != 2 {
		t.Errorf("expected 2 plans, got %d", len(m.Plans()))
	}

	// Find the locked plan and verify it's marked as locked
	var foundLocked, foundUnlocked bool
	for _, p := range m.Plans() {
		if p.ID == "locked" {
			foundLocked = true
			if !p.Locked {
				t.Error("expected plan 'locked' to have Locked=true")
			}
		}
		if p.ID == "unlocked" {
			foundUnlocked = true
			if p.Locked {
				t.Error("expected plan 'unlocked' to have Locked=false")
			}
		}
	}
	if !foundLocked {
		t.Error("locked plan not found")
	}
	if !foundUnlocked {
		t.Error("unlocked plan not found")
	}
}

func TestPlanListModel_LockedPlan_CannotSelect(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create a locked plan
	createTestPlan(t, plansDir, "locked", "test", plan.PlanStatusInProgress, nil)
	lockedPlanDir := filepath.Join(plansDir, "locked-test")
	lockFile := filepath.Join(lockedPlanDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	// Try to select the locked plan with Enter
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should not return a RunPlanMsg command
	if cmd != nil {
		t.Error("expected no command when selecting locked plan")
	}

	// Should set error message
	if newM.LockedErrMsg() == "" {
		t.Error("expected error message when selecting locked plan")
	}
	if !strings.Contains(newM.LockedErrMsg(), "running elsewhere") {
		t.Errorf("expected error message to mention 'running elsewhere', got: %s", newM.LockedErrMsg())
	}
}

func TestPlanListModel_LockedPlan_ShowsLockIndicator(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create a locked plan
	createTestPlan(t, plansDir, "locked", "test", plan.PlanStatusInProgress, nil)
	lockedPlanDir := filepath.Join(plansDir, "locked-test")
	lockFile := filepath.Join(lockedPlanDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	view := m.View()

	// Should show "(locked)" in the status
	if !strings.Contains(view, "locked") {
		t.Error("expected view to contain 'locked' indicator")
	}
}

func TestPlanListModel_LockedPlan_ErrMsgClearedOnNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	rafaDir := filepath.Join(tmpDir, ".rafa")
	plansDir := filepath.Join(rafaDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create two plans, first one locked
	createTestPlan(t, plansDir, "locked", "test1", plan.PlanStatusInProgress, nil)
	lockedPlanDir := filepath.Join(plansDir, "locked-test1")
	lockFile := filepath.Join(lockedPlanDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	createTestPlan(t, plansDir, "unlocked", "test2", plan.PlanStatusNotStarted, nil)

	m := NewPlanListModel(rafaDir)
	m.SetSize(80, 24)

	// Move to the locked plan (cursor starts on first runnable plan).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Try to select the locked plan
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.LockedErrMsg() == "" {
		t.Fatal("expected error message after selecting locked plan")
	}

	// Navigate up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Error message should be cleared
	if m.LockedErrMsg() != "" {
		t.Error("expected error message to be cleared after navigation")
	}
}

func TestIsLocked(t *testing.T) {
	tmpDir := t.TempDir()

	// Test without lock file
	if isLocked(tmpDir) {
		t.Error("expected isLocked to return false for directory without run.lock")
	}

	// Create lock file
	lockFile := filepath.Join(tmpDir, "run.lock")
	writeLiveLockFile(t, lockFile)

	// Test with lock file
	if !isLocked(tmpDir) {
		t.Error("expected isLocked to return true for directory with run.lock")
	}
}

func TestIsLocked_RemovesStaleLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockFile := filepath.Join(tmpDir, "run.lock")
	if err := os.WriteFile(lockFile, []byte("99999999"), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	if isLocked(tmpDir) {
		t.Fatal("expected stale lock to be treated as unlocked")
	}

	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Fatal("expected stale lock file to be removed")
	}
}

func TestIsLocked_RemovesInvalidLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockFile := filepath.Join(tmpDir, "run.lock")
	if err := os.WriteFile(lockFile, []byte("invalid"), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	if isLocked(tmpDir) {
		t.Fatal("expected invalid lock to be treated as unlocked")
	}

	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Fatal("expected invalid lock file to be removed")
	}
}
