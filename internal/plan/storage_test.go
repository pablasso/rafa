package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolvePlanName(t *testing.T) {
	t.Run("new name returns unchanged", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/plans/ directory
		os.MkdirAll(filepath.Join(rafaDir, plansDir), 0755)

		result, err := ResolvePlanName("my-project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "my-project" {
			t.Errorf("got %q, want %q", result, "my-project")
		}
	})

	t.Run("existing name returns name-2", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/plans/ with existing plan
		plansPath := filepath.Join(rafaDir, plansDir)
		os.MkdirAll(filepath.Join(plansPath, "abc123-my-project"), 0755)

		result, err := ResolvePlanName("my-project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "my-project-2" {
			t.Errorf("got %q, want %q", result, "my-project-2")
		}
	})

	t.Run("existing name and name-2 returns name-3", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/plans/ with existing plans
		plansPath := filepath.Join(rafaDir, plansDir)
		os.MkdirAll(filepath.Join(plansPath, "abc123-my-project"), 0755)
		os.MkdirAll(filepath.Join(plansPath, "def456-my-project-2"), 0755)

		result, err := ResolvePlanName("my-project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "my-project-3" {
			t.Errorf("got %q, want %q", result, "my-project-3")
		}
	})

	t.Run("empty plans directory works", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create empty .rafa/plans/ directory
		os.MkdirAll(filepath.Join(rafaDir, plansDir), 0755)

		result, err := ResolvePlanName("new-plan")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "new-plan" {
			t.Errorf("got %q, want %q", result, "new-plan")
		}
	})

	t.Run("nonexistent plans directory works", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Don't create the directory - it shouldn't exist

		result, err := ResolvePlanName("first-plan")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "first-plan" {
			t.Errorf("got %q, want %q", result, "first-plan")
		}
	})

	t.Run("ignores files in plans directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		// Create .rafa/plans/ with a file (not a directory) that matches the pattern
		plansPath := filepath.Join(rafaDir, plansDir)
		os.MkdirAll(plansPath, 0755)
		os.WriteFile(filepath.Join(plansPath, "abc123-my-project"), []byte("not a dir"), 0644)

		result, err := ResolvePlanName("my-project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "my-project" {
			t.Errorf("got %q, want %q", result, "my-project")
		}
	})
}

func TestCreatePlanFolder(t *testing.T) {
	t.Run("creates directory structure", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		plan := &Plan{
			ID:          "abc123",
			Name:        "test-plan",
			Description: "A test plan",
			SourceFile:  "/path/to/source.md",
			CreatedAt:   time.Now(),
			Status:      PlanStatusNotStarted,
			Tasks:       []Task{},
		}

		err := CreatePlanFolder(plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check directory exists
		folderPath := filepath.Join(rafaDir, plansDir, "abc123-test-plan")
		info, err := os.Stat(folderPath)
		if err != nil {
			t.Fatalf("folder not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}
	})

	t.Run("writes valid plan.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		plan := &Plan{
			ID:          "xyz789",
			Name:        "json-test",
			Description: "Testing JSON output",
			SourceFile:  "/source.md",
			CreatedAt:   time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			Status:      PlanStatusNotStarted,
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

		err := CreatePlanFolder(plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read and parse plan.json
		planPath := filepath.Join(rafaDir, plansDir, "xyz789-json-test", "plan.json")
		data, err := os.ReadFile(planPath)
		if err != nil {
			t.Fatalf("failed to read plan.json: %v", err)
		}

		var restored Plan
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("failed to parse plan.json: %v", err)
		}

		// Verify key fields
		if restored.ID != plan.ID {
			t.Errorf("ID mismatch: got %q, want %q", restored.ID, plan.ID)
		}
		if restored.Name != plan.Name {
			t.Errorf("Name mismatch: got %q, want %q", restored.Name, plan.Name)
		}
		if len(restored.Tasks) != len(plan.Tasks) {
			t.Errorf("Tasks count mismatch: got %d, want %d", len(restored.Tasks), len(plan.Tasks))
		}
	})

	t.Run("creates empty log files", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		plan := &Plan{
			ID:        "log123",
			Name:      "log-test",
			Status:    PlanStatusNotStarted,
			CreatedAt: time.Now(),
			Tasks:     []Task{},
		}

		err := CreatePlanFolder(plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		folderPath := filepath.Join(rafaDir, plansDir, "log123-log-test")

		// Check progress.log exists and is empty
		progressData, err := os.ReadFile(filepath.Join(folderPath, "progress.log"))
		if err != nil {
			t.Fatalf("failed to read progress.log: %v", err)
		}
		if len(progressData) != 0 {
			t.Errorf("progress.log should be empty, got %d bytes", len(progressData))
		}

		// Check output.log exists and is empty
		outputData, err := os.ReadFile(filepath.Join(folderPath, "output.log"))
		if err != nil {
			t.Fatalf("failed to read output.log: %v", err)
		}
		if len(outputData) != 0 {
			t.Errorf("output.log should be empty, got %d bytes", len(outputData))
		}
	})

	t.Run("plan.json is pretty printed", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(originalWd)

		plan := &Plan{
			ID:        "pretty123",
			Name:      "pretty-test",
			Status:    PlanStatusNotStarted,
			CreatedAt: time.Now(),
			Tasks:     []Task{},
		}

		err := CreatePlanFolder(plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		planPath := filepath.Join(rafaDir, plansDir, "pretty123-pretty-test", "plan.json")
		data, err := os.ReadFile(planPath)
		if err != nil {
			t.Fatalf("failed to read plan.json: %v", err)
		}

		// Check for indentation (2 spaces)
		content := string(data)
		if len(content) < 10 {
			t.Fatal("plan.json content too short")
		}
		// A pretty-printed JSON should have newlines
		if content[0] != '{' || content[len(content)-1] != '}' {
			t.Error("plan.json should start with { and end with }")
		}
		// Check for 2-space indentation
		if !strings.Contains(content, "\n  \"") {
			t.Error("plan.json should use 2-space indentation")
		}
	})
}
