package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/testutil"
	"github.com/spf13/cobra"
)

// validMockResponse is a valid JSON response that mimics Claude CLI output.
const validMockResponse = `{
  "name": "test-feature",
  "description": "A test feature implementation",
  "tasks": [
    {
      "title": "Implement feature X",
      "description": "Create the main feature",
      "acceptanceCriteria": ["Tests pass", "Linting passes"]
    },
    {
      "title": "Add documentation",
      "description": "Document the feature",
      "acceptanceCriteria": ["README updated"]
    }
  ]
}`

// setupIntegrationTest saves and restores global state (CommandContext, flags).
// Returns the temp directory path.
func setupIntegrationTest(t *testing.T, mockOutput string) string {
	t.Helper()

	// Save original CommandContext and reset global flags on cleanup
	originalCommandContext := ai.CommandContext
	t.Cleanup(func() {
		ai.CommandContext = originalCommandContext
		createName = ""
		createDryRun = false
	})

	// Mock Claude CLI
	ai.CommandContext = testutil.MockCommandFunc(mockOutput)

	// Setup temp directory and working directory
	return testutil.SetupTestDir(t)
}

// executeCreateCommand runs the create command with the given options.
// This directly invokes the RunE function to avoid Cobra's argument parsing.
func executeCreateCommand(t *testing.T, filePath string, name string, dryRun bool) error {
	t.Helper()

	// Set global flags
	createName = name
	createDryRun = dryRun

	// Create a minimal command to pass to RunE
	cmd := &cobra.Command{}
	args := []string{filePath}

	return createCmd.RunE(cmd, args)
}

func TestCreateCommandE2E(t *testing.T) {
	tmpDir := setupIntegrationTest(t, validMockResponse)

	// Create .rafa/plans/ directory structure
	plansDir := filepath.Join(tmpDir, ".rafa", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create .rafa/plans: %v", err)
	}

	// Create a design document
	designContent := "# Test Design\n\nThis is a test design document."
	designPath := filepath.Join(tmpDir, "design.md")
	if err := os.WriteFile(designPath, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design.md: %v", err)
	}

	// Execute the create command
	err := executeCreateCommand(t, designPath, "", false)
	if err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	// Verify plan folder was created
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		t.Fatalf("failed to read plans directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 plan folder, got %d", len(entries))
	}

	planFolder := entries[0]
	if !planFolder.IsDir() {
		t.Error("expected plan folder to be a directory")
	}

	// Verify folder name contains "test-feature"
	if !strings.Contains(planFolder.Name(), "test-feature") {
		t.Errorf("expected folder name to contain 'test-feature', got %q", planFolder.Name())
	}

	// Verify plan.json exists and is valid
	planJSONPath := filepath.Join(plansDir, planFolder.Name(), "plan.json")
	planData, err := os.ReadFile(planJSONPath)
	if err != nil {
		t.Fatalf("failed to read plan.json: %v", err)
	}

	var p plan.Plan
	if err := json.Unmarshal(planData, &p); err != nil {
		t.Fatalf("failed to parse plan.json: %v", err)
	}

	// Verify plan contents
	if p.Name != "test-feature" {
		t.Errorf("expected plan name 'test-feature', got %q", p.Name)
	}
	if p.Description != "A test feature implementation" {
		t.Errorf("unexpected description: %q", p.Description)
	}
	if len(p.Tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(p.Tasks))
	}
	if p.Status != plan.PlanStatusNotStarted {
		t.Errorf("expected status 'not_started', got %q", p.Status)
	}

	// Verify tasks have correct IDs
	if p.Tasks[0].ID != "t01" {
		t.Errorf("expected first task ID 't01', got %q", p.Tasks[0].ID)
	}
	if p.Tasks[1].ID != "t02" {
		t.Errorf("expected second task ID 't02', got %q", p.Tasks[1].ID)
	}

	// Verify log files exist
	progressLogPath := filepath.Join(plansDir, planFolder.Name(), "progress.log")
	if _, err := os.Stat(progressLogPath); os.IsNotExist(err) {
		t.Error("progress.log not created")
	}

	outputLogPath := filepath.Join(plansDir, planFolder.Name(), "output.log")
	if _, err := os.Stat(outputLogPath); os.IsNotExist(err) {
		t.Error("output.log not created")
	}
}

func TestCreateCommandDryRun(t *testing.T) {
	tmpDir := setupIntegrationTest(t, validMockResponse)

	// Create a design document (NO .rafa/ directory)
	designContent := "# Test Design\n\nThis is a test design document."
	designPath := filepath.Join(tmpDir, "design.md")
	if err := os.WriteFile(designPath, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design.md: %v", err)
	}

	// Execute the create command with --dry-run
	err := executeCreateCommand(t, designPath, "", true)
	if err != nil {
		t.Fatalf("create command with --dry-run failed: %v", err)
	}

	// Verify NO .rafa/ directory was created
	rafaPath := filepath.Join(tmpDir, ".rafa")
	if _, err := os.Stat(rafaPath); !os.IsNotExist(err) {
		t.Error("dry-run should not create .rafa/ directory")
	}
}

func TestCreateCommandWithNameFlag(t *testing.T) {
	tmpDir := setupIntegrationTest(t, validMockResponse)

	// Create .rafa/plans/ directory structure
	plansDir := filepath.Join(tmpDir, ".rafa", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create .rafa/plans: %v", err)
	}

	// Create a design document
	designContent := "# Test Design\n\nThis is a test design document."
	designPath := filepath.Join(tmpDir, "design.md")
	if err := os.WriteFile(designPath, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design.md: %v", err)
	}

	// Execute the create command with --name flag
	err := executeCreateCommand(t, designPath, "custom-plan-name", false)
	if err != nil {
		t.Fatalf("create command failed: %v", err)
	}

	// Verify plan folder uses custom name
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		t.Fatalf("failed to read plans directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 plan folder, got %d", len(entries))
	}

	planFolder := entries[0]
	if !strings.Contains(planFolder.Name(), "custom-plan-name") {
		t.Errorf("expected folder name to contain 'custom-plan-name', got %q", planFolder.Name())
	}

	// Verify plan.json has custom name
	planJSONPath := filepath.Join(plansDir, planFolder.Name(), "plan.json")
	planData, err := os.ReadFile(planJSONPath)
	if err != nil {
		t.Fatalf("failed to read plan.json: %v", err)
	}

	var p plan.Plan
	if err := json.Unmarshal(planData, &p); err != nil {
		t.Fatalf("failed to parse plan.json: %v", err)
	}

	if p.Name != "custom-plan-name" {
		t.Errorf("expected plan name 'custom-plan-name', got %q", p.Name)
	}
}

func TestCreateCommandErrors(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(tmpDir string) string // returns file path
		mockOutput  string
		wantErrText string
	}{
		{
			name: "missing file",
			setup: func(tmpDir string) string {
				os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)
				return filepath.Join(tmpDir, "nonexistent.md")
			},
			mockOutput:  validMockResponse,
			wantErrText: "file not found",
		},
		{
			name: "invalid JSON response",
			setup: func(tmpDir string) string {
				os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)
				designPath := filepath.Join(tmpDir, "design.md")
				os.WriteFile(designPath, []byte("# Design"), 0644)
				return designPath
			},
			mockOutput:  "not valid json",
			wantErrText: "no JSON object found",
		},
		{
			name: "no tasks in response",
			setup: func(tmpDir string) string {
				os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)
				designPath := filepath.Join(tmpDir, "design.md")
				os.WriteFile(designPath, []byte("# Design"), 0644)
				return designPath
			},
			mockOutput:  `{"name": "empty", "description": "No tasks", "tasks": []}`,
			wantErrText: "no tasks extracted",
		},
		{
			name: "not initialized (no .rafa/)",
			setup: func(tmpDir string) string {
				designPath := filepath.Join(tmpDir, "design.md")
				os.WriteFile(designPath, []byte("# Design"), 0644)
				return designPath
			},
			mockOutput:  validMockResponse,
			wantErrText: "rafa not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupIntegrationTest(t, tt.mockOutput)

			// Run setup and get file path
			filePath := tt.setup(tmpDir)

			// Execute the create command
			err := executeCreateCommand(t, filePath, "", false)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErrText)
			}
		})
	}
}
