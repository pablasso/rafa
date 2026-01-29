package demo

import (
	"strings"
	"testing"

	"github.com/pablasso/rafa/internal/plan"
)

func TestGenerateOutput_ContainsImplementationProgressSection(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
			"Criterion 2",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	if !strings.Contains(joined, "## Implementation Progress") {
		t.Error("Output should contain '## Implementation Progress' section")
	}
}

func TestGenerateOutput_ContainsAcceptanceCriteriaVerificationSection(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
			"Criterion 2",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	if !strings.Contains(joined, "## Acceptance Criteria Verification") {
		t.Error("Output should contain '## Acceptance Criteria Verification' section")
	}
}

func TestGenerateOutput_RetryAttemptMessage(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	// Test attempt 1 - should NOT have retry message
	output1 := runner.generateOutput(task, 1)
	joined1 := strings.Join(output1, "\n")
	if strings.Contains(joined1, "Retrying task") {
		t.Error("Attempt 1 should NOT contain retry message")
	}

	// Test attempt 2 - should have retry message
	output2 := runner.generateOutput(task, 2)
	joined2 := strings.Join(output2, "\n")
	if !strings.Contains(joined2, "Retrying task (attempt 2)") {
		t.Error("Attempt 2 should contain 'Retrying task (attempt 2)' message")
	}
	if !strings.Contains(joined2, "Analyzing previous failure") {
		t.Error("Retry attempt should contain 'Analyzing previous failure' context")
	}

	// Test attempt 3 - should have retry message with attempt 3
	output3 := runner.generateOutput(task, 3)
	joined3 := strings.Join(output3, "\n")
	if !strings.Contains(joined3, "Retrying task (attempt 3)") {
		t.Error("Attempt 3 should contain 'Retrying task (attempt 3)' message")
	}
}

func TestGenerateOutput_PassingCriteriaShowCheckmark(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"First criterion",
			"Second criterion",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	// All criteria should pass with checkmarks
	if !strings.Contains(joined, "1. ✅ First criterion") {
		t.Error("Passing criterion should have ✅ checkmark")
	}
	if !strings.Contains(joined, "2. ✅ Second criterion") {
		t.Error("Passing criterion should have ✅ checkmark")
	}
}

func TestGenerateOutput_FailingCriteriaShowXMark(t *testing.T) {
	config := NewConfig(ScenarioFail, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"First criterion",
			"Second criterion",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	// First criterion should pass
	if !strings.Contains(joined, "1. ✅ First criterion") {
		t.Error("First criterion should pass with ✅ checkmark")
	}
	// Last criterion should fail with X mark
	if !strings.Contains(joined, "2. ❌ Second criterion - **FAILED**") {
		t.Error("Failing criterion should have ❌ mark and **FAILED** suffix")
	}
}

func TestGenerateOutput_SuccessIncludesSuggestedCommitMessage(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	if !strings.Contains(joined, "SUGGESTED_COMMIT_MESSAGE:") {
		t.Error("Successful output should contain SUGGESTED_COMMIT_MESSAGE")
	}
	if !strings.Contains(joined, "All acceptance criteria verified") {
		t.Error("Successful output should indicate all criteria verified")
	}
}

func TestGenerateOutput_FailureDoesNotIncludeSuggestedCommitMessage(t *testing.T) {
	config := NewConfig(ScenarioFail, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	if strings.Contains(joined, "SUGGESTED_COMMIT_MESSAGE:") {
		t.Error("Failed output should NOT contain SUGGESTED_COMMIT_MESSAGE")
	}
	if !strings.Contains(joined, "Some acceptance criteria were not met") {
		t.Error("Failed output should indicate criteria not met")
	}
	if !strings.Contains(joined, "Task requires retry") {
		t.Error("Failed output should suggest retry")
	}
}

func TestGenerateWorkLines_AllTasksHaveTemplates(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	taskIDs := []string{"t01", "t02", "t03", "t04", "t05"}

	for _, id := range taskIDs {
		task := &plan.Task{ID: id}
		lines := runner.generateWorkLines(task)

		if len(lines) == 0 {
			t.Errorf("Task %s should have work template lines", id)
		}

		// Verify it's not the fallback (which has exactly 3 lines)
		if len(lines) == 3 && lines[0] == "Analyzing requirements..." {
			t.Errorf("Task %s should have specific template, not fallback", id)
		}
	}
}

func TestGenerateWorkLines_T01Content(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "t01"}
	lines := runner.generateWorkLines(task)
	joined := strings.Join(lines, "\n")

	expectedContent := []string{
		"Creating directory structure",
		"internal/feature/",
		"config.yaml",
		"make build",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(joined, expected) {
			t.Errorf("t01 work lines should contain %q", expected)
		}
	}
}

func TestGenerateWorkLines_T02Content(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "t02"}
	lines := runner.generateWorkLines(task)
	joined := strings.Join(lines, "\n")

	expectedContent := []string{
		"Defining data models",
		"type Feature struct",
		"interface definitions",
		"make test",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(joined, expected) {
			t.Errorf("t02 work lines should contain %q", expected)
		}
	}
}

func TestGenerateWorkLines_T03Content(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "t03"}
	lines := runner.generateWorkLines(task)
	joined := strings.Join(lines, "\n")

	expectedContent := []string{
		"business logic",
		"validation functions",
		"error handling",
		"comprehensive tests",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(joined, expected) {
			t.Errorf("t03 work lines should contain %q", expected)
		}
	}
}

func TestGenerateWorkLines_T04Content(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "t04"}
	lines := runner.generateWorkLines(task)
	joined := strings.Join(lines, "\n")

	expectedContent := []string{
		"API routes",
		"/api/features",
		"GET",
		"POST",
		"integration tests",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(joined, expected) {
			t.Errorf("t04 work lines should contain %q", expected)
		}
	}
}

func TestGenerateWorkLines_T05Content(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "t05"}
	lines := runner.generateWorkLines(task)
	joined := strings.Join(lines, "\n")

	expectedContent := []string{
		"README.md",
		"API documentation",
		"usage examples",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(joined, expected) {
			t.Errorf("t05 work lines should contain %q", expected)
		}
	}
}

func TestGenerateWorkLines_UnknownTaskFallback(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "unknown-task", Title: "Some unknown task"}
	lines := runner.generateWorkLines(task)

	// Dynamic tasks generate 5 lines for generic fallback
	if len(lines) != 5 {
		t.Errorf("Unknown task should return 5 fallback lines, got %d", len(lines))
	}

	// First and last lines should match generic fallback pattern
	if lines[0] != "Analyzing requirements..." {
		t.Errorf("Fallback line 0: got %q, want %q", lines[0], "Analyzing requirements...")
	}
	if lines[4] != "Task complete." {
		t.Errorf("Fallback line 4: got %q, want %q", lines[4], "Task complete.")
	}
}

func TestGenerateCommitMessage_AllTasksHaveMessages(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	expectedMessages := map[string]string{
		"t01": "Set up project structure with configuration files",
		"t02": "Implement core data models and interfaces",
		"t03": "Add business logic layer with validation",
		"t04": "Create REST API endpoints with handlers",
		"t05": "Add documentation and usage examples",
	}

	for id, expected := range expectedMessages {
		task := &plan.Task{ID: id, Title: "Some title"}
		got := runner.generateCommitMessage(task)
		if got != expected {
			t.Errorf("generateCommitMessage(%s): got %q, want %q", id, got, expected)
		}
	}
}

func TestGenerateCommitMessage_UnknownTaskFallback(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{ID: "unknown", Title: "My Custom Task"}
	got := runner.generateCommitMessage(task)
	expected := "Complete my custom task"

	if got != expected {
		t.Errorf("generateCommitMessage(unknown): got %q, want %q", got, expected)
	}
}

func TestGenerateOutput_ContainsWorkingOnTitle(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "My Special Task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	if !strings.Contains(joined, "Working on: My Special Task") {
		t.Error("Output should contain 'Working on: <task title>'")
	}
}

func TestGenerateOutput_SectionsInCorrectOrder(t *testing.T) {
	config := NewConfig(ScenarioSuccess, SpeedFast)
	runner := NewDemoRunner(config)

	task := &plan.Task{
		ID:    "t01",
		Title: "Test task",
		AcceptanceCriteria: []string{
			"Criterion 1",
		},
	}

	output := runner.generateOutput(task, 1)
	joined := strings.Join(output, "\n")

	// Check that sections appear in the correct order
	progressIdx := strings.Index(joined, "## Implementation Progress")
	criteriaIdx := strings.Index(joined, "## Acceptance Criteria Verification")
	commitIdx := strings.Index(joined, "SUGGESTED_COMMIT_MESSAGE:")

	if progressIdx == -1 || criteriaIdx == -1 || commitIdx == -1 {
		t.Fatal("Missing expected sections in output")
	}

	if progressIdx >= criteriaIdx {
		t.Error("Implementation Progress should come before Acceptance Criteria Verification")
	}
	if criteriaIdx >= commitIdx {
		t.Error("Acceptance Criteria Verification should come before SUGGESTED_COMMIT_MESSAGE")
	}
}
