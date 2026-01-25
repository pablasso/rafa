package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/testutil"
)

func TestClaudeRunner_BuildPrompt(t *testing.T) {
	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:          "t01",
		Title:       "Implement feature X",
		Description: "Create the main feature functionality",
		AcceptanceCriteria: []string{
			"Tests pass",
			"Linting passes",
		},
	}
	planContext := "This is the plan context."

	prompt := runner.buildPrompt(task, planContext, 1, 3)

	// Verify prompt includes task ID
	if !strings.Contains(prompt, "t01") {
		t.Error("prompt should include task ID")
	}

	// Verify prompt includes task title
	if !strings.Contains(prompt, "Implement feature X") {
		t.Error("prompt should include task title")
	}

	// Verify prompt includes task description
	if !strings.Contains(prompt, "Create the main feature functionality") {
		t.Error("prompt should include task description")
	}

	// Verify prompt includes plan context
	if !strings.Contains(prompt, planContext) {
		t.Error("prompt should include plan context")
	}

	// Verify prompt includes attempt info
	if !strings.Contains(prompt, "Attempt") {
		t.Error("prompt should include attempt information")
	}
}

func TestClaudeRunner_PromptIncludesAllCriteria(t *testing.T) {
	runner := NewClaudeRunner()
	criteria := []string{
		"Tests pass",
		"Linting passes",
		"Documentation updated",
		"No warnings",
	}
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: criteria,
	}

	prompt := runner.buildPrompt(task, "", 1, 1)

	for _, criterion := range criteria {
		if !strings.Contains(prompt, criterion) {
			t.Errorf("prompt should include criterion %q", criterion)
		}
	}
}

func TestClaudeRunner_PromptIncludesAttemptNumber(t *testing.T) {
	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	tests := []struct {
		attempt     int
		maxAttempts int
		want        string
	}{
		{1, 3, "Attempt**: 1 of 3"},
		{2, 3, "Attempt**: 2 of 3"},
		{3, 3, "Attempt**: 3 of 3"},
		{1, 5, "Attempt**: 1 of 5"},
	}

	for _, tt := range tests {
		prompt := runner.buildPrompt(task, "", tt.attempt, tt.maxAttempts)
		if !strings.Contains(prompt, tt.want) {
			t.Errorf("attempt %d of %d: prompt should include %q", tt.attempt, tt.maxAttempts, tt.want)
		}
	}
}

func TestClaudeRunner_PromptIncludesRetryNote(t *testing.T) {
	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	// Attempt 2 should include retry note
	prompt := runner.buildPrompt(task, "", 2, 3)
	if !strings.Contains(prompt, "Previous attempts") {
		t.Error("prompt for attempt > 1 should include retry note")
	}

	// Attempt 3 should also include retry note
	prompt = runner.buildPrompt(task, "", 3, 3)
	if !strings.Contains(prompt, "Previous attempts") {
		t.Error("prompt for attempt > 1 should include retry note")
	}
}

func TestClaudeRunner_PromptNoRetryNoteOnFirstAttempt(t *testing.T) {
	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	prompt := runner.buildPrompt(task, "", 1, 3)
	if strings.Contains(prompt, "Previous attempts") {
		t.Error("prompt for first attempt should not include retry note")
	}
}

func TestClaudeRunner_Run_Success(t *testing.T) {
	// Save original CommandContext
	originalCommandContext := ai.CommandContext
	defer func() {
		ai.CommandContext = originalCommandContext
	}()

	// Mock Claude CLI with successful output
	ai.CommandContext = testutil.MockCommandFunc("Task completed successfully")

	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	err := runner.Run(context.Background(), task, "context", 1, 3, nil)
	if err != nil {
		t.Errorf("Run() returned error: %v", err)
	}
}

func TestClaudeRunner_Run_Failure(t *testing.T) {
	// Save original CommandContext
	originalCommandContext := ai.CommandContext
	defer func() {
		ai.CommandContext = originalCommandContext
	}()

	// Mock Claude CLI with failure
	ai.CommandContext = testutil.MockCommandFuncFail(1)

	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	err := runner.Run(context.Background(), task, "context", 1, 3, nil)
	if err == nil {
		t.Error("Run() should return error on non-zero exit")
	}
	if !strings.Contains(err.Error(), "claude exited with error") {
		t.Errorf("error should indicate claude exit error, got: %v", err)
	}
}

func TestClaudeRunner_Run_Cancellation(t *testing.T) {
	// Save original CommandContext
	originalCommandContext := ai.CommandContext
	defer func() {
		ai.CommandContext = originalCommandContext
	}()

	// Mock Claude CLI with sleep
	ai.CommandContext = testutil.MockCommandFuncSleep("10")

	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := runner.Run(ctx, task, "context", 1, 3, nil)
	if err == nil {
		t.Error("Run() should return error on cancellation")
	}
	if err != context.Canceled {
		t.Errorf("Run() should return context.Canceled, got: %v", err)
	}
}

func TestNewClaudeRunner(t *testing.T) {
	runner := NewClaudeRunner()
	if runner == nil {
		t.Error("NewClaudeRunner() should return non-nil runner")
	}
}

func TestClaudeRunner_PromptIncludesDoNotCommitInstruction(t *testing.T) {
	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	prompt := runner.buildPrompt(task, "", 1, 3)

	// Verify prompt includes "DO NOT commit" instruction
	if !strings.Contains(prompt, "DO NOT commit") {
		t.Error("prompt should include 'DO NOT commit' instruction")
	}

	// Verify prompt includes instruction for suggested commit message
	if !strings.Contains(prompt, "SUGGESTED_COMMIT_MESSAGE") {
		t.Error("prompt should include 'SUGGESTED_COMMIT_MESSAGE' instruction")
	}

	// Verify prompt includes note about orchestrator handling commit
	if !strings.Contains(prompt, "orchestrator will commit") {
		t.Error("prompt should include note about orchestrator handling commit")
	}

	// Verify prompt includes note about leaving changes uncommitted
	if !strings.Contains(prompt, "Leave changes uncommitted") {
		t.Error("prompt should include note about leaving changes uncommitted")
	}
}

func TestClaudeRunner_PromptRetryNoteIncludesUncommittedChanges(t *testing.T) {
	runner := NewClaudeRunner()
	task := &plan.Task{
		ID:                 "t01",
		Title:              "Test task",
		Description:        "Test description",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	// Test retry attempt includes note about uncommitted changes
	prompt := runner.buildPrompt(task, "", 2, 3)

	if !strings.Contains(prompt, "uncommitted changes from previous attempts") {
		t.Error("retry prompt should include note about reviewing uncommitted changes from previous attempts")
	}

	if !strings.Contains(prompt, "git status") {
		t.Error("retry prompt should include suggestion to use git status")
	}
}
