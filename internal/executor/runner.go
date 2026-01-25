package executor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
)

// ClaudeRunner executes tasks via Claude Code CLI.
type ClaudeRunner struct{}

// NewClaudeRunner creates a new ClaudeRunner.
func NewClaudeRunner() *ClaudeRunner {
	return &ClaudeRunner{}
}

// Run executes a single task via Claude Code CLI.
func (r *ClaudeRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error {
	prompt := r.buildPrompt(task, planContext, attempt, maxAttempts)

	cmd := ai.CommandContext(ctx, "claude",
		"-p", prompt,
		"--dangerously-skip-permissions",
	)

	// Use OutputWriter if provided, otherwise fall back to os.Stdout/os.Stderr
	if output != nil {
		cmd.Stdout = output.Stdout()
		cmd.Stderr = output.Stderr()
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
}

// buildPrompt constructs the prompt for Claude CLI.
func (r *ClaudeRunner) buildPrompt(task *plan.Task, planContext string, attempt, maxAttempts int) string {
	var sb strings.Builder

	sb.WriteString("You are executing a task as part of an automated plan.\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(planContext)
	sb.WriteString("\n")

	sb.WriteString("## Your Task\n")
	sb.WriteString(fmt.Sprintf("**ID**: %s\n", task.ID))
	sb.WriteString(fmt.Sprintf("**Title**: %s\n", task.Title))
	sb.WriteString(fmt.Sprintf("**Attempt**: %d of %d\n", attempt, maxAttempts))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", task.Description))

	// Add retry note if not first attempt
	if attempt > 1 {
		sb.WriteString("**Note**: Previous attempts to complete this task failed. ")
		sb.WriteString("Consider alternative approaches or investigate what went wrong. ")
		sb.WriteString("If the previous attempt left uncommitted changes, make sure to commit ALL changes ")
		sb.WriteString("(including .rafa/ metadata) and leave the workspace clean.\n\n")
	}

	sb.WriteString("## Acceptance Criteria\n")
	sb.WriteString("You MUST verify ALL of the following before considering the task complete:\n")
	for i, criterion := range task.AcceptanceCriteria {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, criterion))
	}
	sb.WriteString("\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Implement the task as described\n")
	sb.WriteString("2. Verify ALL acceptance criteria are met\n")
	sb.WriteString("3. Before finalizing, perform a code review of your changes. If you have a code review skill available (e.g., `/code-review`), use it to review your implementation and assess what findings are worth addressing vs. acceptable trade-offs\n")
	sb.WriteString("4. Commit ALL changes (implementation code AND `.rafa/` metadata) with a descriptive message\n")
	sb.WriteString("5. Verify workspace is clean with `git status` before exiting\n\n")

	sb.WriteString("IMPORTANT: The workspace MUST be clean when you exit. Do not declare success unless ALL acceptance criteria are met.\n")

	return sb.String()
}
