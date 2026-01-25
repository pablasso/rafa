package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// DefaultExtractionTimeout is the maximum time allowed for task extraction.
const DefaultExtractionTimeout = 5 * time.Minute

// IsClaudeAvailable checks if the claude command exists in PATH.
func IsClaudeAvailable() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// ExtractTasks uses Claude CLI to extract tasks from a design document.
// The context controls cancellation and timeout. If the context has no deadline,
// DefaultExtractionTimeout is applied.
func ExtractTasks(ctx context.Context, designContent string) (*plan.TaskExtractionResult, error) {
	if !IsClaudeAvailable() {
		return nil, errors.New("Claude Code CLI not found. Install it: https://claude.ai/code")
	}

	// Apply default timeout if context has no deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultExtractionTimeout)
		defer cancel()
	}

	prompt := buildExtractionPrompt(designContent)

	// Execute claude CLI with the prompt.
	// --dangerously-skip-permissions is required for non-interactive use. This is safe here
	// because we only use the -p flag with a controlled prompt (no file access or tool use).
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--output-format", "json", "--dangerously-skip-permissions")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.New("task extraction timed out")
		}
		if ctx.Err() == context.Canceled {
			return nil, errors.New("task extraction was cancelled")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("claude command failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute claude command: %w", err)
	}

	// Extract JSON from response
	jsonData, err := extractJSON(output)
	if err != nil {
		return nil, fmt.Errorf("failed to extract JSON from claude response: %w", err)
	}

	// Parse the JSON response
	var result plan.TaskExtractionResult
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse claude response: %w", err)
	}

	// Validate the result
	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("invalid extraction result: %w", err)
	}

	return &result, nil
}

// buildExtractionPrompt creates the prompt for task extraction from a design document.
func buildExtractionPrompt(designContent string) string {
	return fmt.Sprintf(`You are a technical project planner. Analyze this design document and extract discrete implementation tasks.

DESIGN DOCUMENT:
%s

OUTPUT REQUIREMENTS:
Return a JSON object with this exact structure:
{
  "name": "kebab-case-name-from-document",
  "description": "One sentence describing the overall goal",
  "tasks": [
    {
      "title": "Short imperative title (e.g., 'Implement user login endpoint')",
      "description": "Detailed description of what needs to be done. Include relevant context from the design.",
      "acceptanceCriteria": [
        "Specific, verifiable criterion (e.g., 'npm test passes')",
        "Another measurable criterion",
        "Prefer runnable checks over prose"
      ]
    }
  ]
}

TASK GUIDELINES:
- Each task should use roughly 50-60%% of an AI agent's context window
- Tasks must be completable in sequence (later tasks can depend on earlier ones)
- Acceptance criteria must be verifiable by the agent itself
- Prefer criteria that can be verified with commands (tests, type checks, lint)
- Include 2-5 acceptance criteria per task
- Order tasks by implementation dependency

Return ONLY the JSON, no markdown formatting or explanation.`, designContent)
}

// extractJSON defensively extracts a JSON object from potentially noisy output.
func extractJSON(data []byte) ([]byte, error) {
	// First, try if the entire data is valid JSON
	if json.Valid(data) {
		return data, nil
	}

	// Find first '{' and last '}' to extract JSON object
	str := string(data)
	start := -1
	end := -1

	for i, c := range str {
		if c == '{' {
			start = i
			break
		}
	}

	for i := len(str) - 1; i >= 0; i-- {
		if str[i] == '}' {
			end = i
			break
		}
	}

	if start == -1 || end == -1 || start >= end {
		return nil, errors.New("no JSON object found in response")
	}

	extracted := []byte(str[start : end+1])
	if !json.Valid(extracted) {
		return nil, errors.New("extracted content is not valid JSON")
	}

	return extracted, nil
}
