package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindPlanFolder_Found(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create .rafa/plans/ with a plan folder
	plansPath := filepath.Join(rafaDir, plansDir)
	os.MkdirAll(filepath.Join(plansPath, "abc123-my-plan"), 0755)

	result, err := FindPlanFolder("my-plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(plansPath, "abc123-my-plan")
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestFindPlanFolder_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create .rafa/plans/ with a different plan
	plansPath := filepath.Join(rafaDir, plansDir)
	os.MkdirAll(filepath.Join(plansPath, "abc123-other-plan"), 0755)

	_, err := FindPlanFolder("my-plan")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "plan not found: my-plan") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFindPlanFolder_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create .rafa/plans/ with multiple matching plans
	plansPath := filepath.Join(rafaDir, plansDir)
	os.MkdirAll(filepath.Join(plansPath, "abc123-my-plan"), 0755)
	os.MkdirAll(filepath.Join(plansPath, "def456-my-plan"), 0755)

	_, err := FindPlanFolder("my-plan")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple plans match") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFindPlanFolder_NoPlansDir(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Don't create .rafa/plans/

	_, err := FindPlanFolder("my-plan")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no plans found") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "Create a plan") {
		t.Errorf("error should mention how to create a plan: %v", err)
	}
}

func TestLoadPlan_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid plan.json
	plan := &Plan{
		ID:          "test123",
		Name:        "test-plan",
		Description: "A test plan",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
		Status:      PlanStatusInProgress,
		Tasks: []Task{
			{
				ID:                 "task-1",
				Title:              "First Task",
				Description:        "Do something",
				AcceptanceCriteria: []string{"It works"},
				Status:             TaskStatusPending,
				Attempts:           0,
			},
		},
	}

	data, _ := json.MarshalIndent(plan, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "plan.json"), data, 0644)

	loaded, err := LoadPlan(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.ID != plan.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, plan.ID)
	}
	if loaded.Name != plan.Name {
		t.Errorf("Name mismatch: got %q, want %q", loaded.Name, plan.Name)
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("Tasks count mismatch: got %d, want 1", len(loaded.Tasks))
	}
	if loaded.Tasks[0].Title != "First Task" {
		t.Errorf("Task title mismatch: got %q, want %q", loaded.Tasks[0].Title, "First Task")
	}
}

func TestLoadPlan_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid JSON
	os.WriteFile(filepath.Join(tmpDir, "plan.json"), []byte("{invalid json}"), 0644)

	_, err := LoadPlan(tmpDir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse plan.json") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadPlan_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't create plan.json

	_, err := LoadPlan(tmpDir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read plan.json") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSavePlan_Success(t *testing.T) {
	tmpDir := t.TempDir()

	plan := &Plan{
		ID:          "save123",
		Name:        "save-test",
		Description: "Testing save",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
		Status:      PlanStatusInProgress,
		Tasks: []Task{
			{
				ID:          "task-1",
				Title:       "Task One",
				Description: "Do task one",
				Status:      TaskStatusCompleted,
				Attempts:    1,
			},
		},
	}

	err := SavePlan(tmpDir, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(filepath.Join(tmpDir, "plan.json"))
	if err != nil {
		t.Fatalf("failed to read plan.json: %v", err)
	}

	var loaded Plan
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse plan.json: %v", err)
	}

	if loaded.ID != plan.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, plan.ID)
	}
	if loaded.Tasks[0].Status != TaskStatusCompleted {
		t.Errorf("Task status mismatch: got %q, want %q", loaded.Tasks[0].Status, TaskStatusCompleted)
	}

	// Verify pretty-printed (2-space indent)
	content := string(data)
	if !strings.Contains(content, "\n  \"") {
		t.Error("plan.json should use 2-space indentation")
	}
}

func TestSavePlan_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial plan.json
	initialPlan := &Plan{
		ID:     "initial",
		Name:   "initial-plan",
		Status: PlanStatusNotStarted,
		Tasks:  []Task{},
	}
	initialData, _ := json.MarshalIndent(initialPlan, "", "  ")
	planPath := filepath.Join(tmpDir, "plan.json")
	os.WriteFile(planPath, initialData, 0644)

	// Save updated plan
	updatedPlan := &Plan{
		ID:     "updated",
		Name:   "updated-plan",
		Status: PlanStatusInProgress,
		Tasks:  []Task{},
	}

	err := SavePlan(tmpDir, updatedPlan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no temp file remains
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") {
			t.Errorf("temp file should be cleaned up: %s", entry.Name())
		}
	}

	// Verify content was updated
	loaded, err := LoadPlan(tmpDir)
	if err != nil {
		t.Fatalf("failed to load plan: %v", err)
	}
	if loaded.ID != "updated" {
		t.Errorf("plan not updated: got ID %q, want %q", loaded.ID, "updated")
	}
}

func TestFirstPendingTask_FindsPending(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Status: TaskStatusCompleted},
			{ID: "2", Status: TaskStatusPending},
			{ID: "3", Status: TaskStatusPending},
		},
	}

	idx := plan.FirstPendingTask()
	if idx != 1 {
		t.Errorf("got index %d, want 1", idx)
	}
}

func TestFirstPendingTask_FindsInProgress(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Status: TaskStatusCompleted},
			{ID: "2", Status: TaskStatusInProgress},
			{ID: "3", Status: TaskStatusPending},
		},
	}

	idx := plan.FirstPendingTask()
	if idx != 1 {
		t.Errorf("got index %d, want 1", idx)
	}
}

func TestFirstPendingTask_FindsFailed(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Status: TaskStatusCompleted},
			{ID: "2", Status: TaskStatusFailed, Attempts: 2},
			{ID: "3", Status: TaskStatusPending},
		},
	}

	idx := plan.FirstPendingTask()
	if idx != 1 {
		t.Errorf("got index %d, want 1", idx)
	}

	// Verify status was reset to pending
	if plan.Tasks[1].Status != TaskStatusPending {
		t.Errorf("failed task status not reset: got %q, want %q", plan.Tasks[1].Status, TaskStatusPending)
	}
}

func TestFirstPendingTask_FailedPreservesAttempts(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Status: TaskStatusFailed, Attempts: 3},
		},
	}

	idx := plan.FirstPendingTask()
	if idx != 0 {
		t.Errorf("got index %d, want 0", idx)
	}

	// Verify attempts were preserved
	if plan.Tasks[0].Attempts != 3 {
		t.Errorf("attempts not preserved: got %d, want 3", plan.Tasks[0].Attempts)
	}
}

func TestFirstPendingTask_NoneRemaining(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Status: TaskStatusCompleted},
			{ID: "2", Status: TaskStatusCompleted},
		},
	}

	idx := plan.FirstPendingTask()
	if idx != -1 {
		t.Errorf("got index %d, want -1", idx)
	}
}

func TestAllTasksCompleted_True(t *testing.T) {
	plan := &Plan{
		Tasks: []Task{
			{ID: "1", Status: TaskStatusCompleted},
			{ID: "2", Status: TaskStatusCompleted},
			{ID: "3", Status: TaskStatusCompleted},
		},
	}

	if !plan.AllTasksCompleted() {
		t.Error("expected AllTasksCompleted to return true")
	}
}

func TestAllTasksCompleted_False(t *testing.T) {
	testCases := []struct {
		name  string
		tasks []Task
	}{
		{
			name: "has pending",
			tasks: []Task{
				{ID: "1", Status: TaskStatusCompleted},
				{ID: "2", Status: TaskStatusPending},
			},
		},
		{
			name: "has in_progress",
			tasks: []Task{
				{ID: "1", Status: TaskStatusCompleted},
				{ID: "2", Status: TaskStatusInProgress},
			},
		},
		{
			name: "has failed",
			tasks: []Task{
				{ID: "1", Status: TaskStatusCompleted},
				{ID: "2", Status: TaskStatusFailed},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := &Plan{Tasks: tc.tasks}
			if plan.AllTasksCompleted() {
				t.Error("expected AllTasksCompleted to return false")
			}
		})
	}
}

func TestAllTasksCompleted_EmptyTasks(t *testing.T) {
	plan := &Plan{Tasks: []Task{}}

	// Empty task list should return true (vacuously true)
	if !plan.AllTasksCompleted() {
		t.Error("expected AllTasksCompleted to return true for empty tasks")
	}
}
