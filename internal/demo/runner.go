package demo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
)

// Compile-time interface verification
var _ executor.Runner = (*DemoRunner)(nil)

// DemoRunner implements executor.Runner with simulated execution.
type DemoRunner struct {
	config *Config
}

// NewDemoRunner creates a new DemoRunner with the given configuration.
func NewDemoRunner(config *Config) *DemoRunner {
	return &DemoRunner{
		config: config,
	}
}

// Run implements executor.Runner interface.
// It streams realistic output and determines success/failure based on scenario.
func (d *DemoRunner) Run(
	ctx context.Context,
	task *plan.Task,
	planContext string,
	attempt, maxAttempts int,
	output executor.OutputWriter,
) error {
	// Stream realistic output
	d.streamOutput(ctx, task, attempt, output)

	// Determine success/failure based on scenario
	if d.shouldFail(task, attempt) {
		return errors.New("simulated failure: acceptance criteria not met")
	}

	return nil
}

// shouldFail determines if a task should fail based on scenario and attempt.
//
// Scenario behavior:
//   - success: Always passes (returns false)
//   - fail: Always fails (returns true)
//   - retry: Fails first 2 attempts, succeeds on 3rd (returns attempt < 3)
//   - mixed: t02 fails first attempt then succeeds (attempt < 2),
//     t03 always fails (true), others pass (false)
func (d *DemoRunner) shouldFail(task *plan.Task, attempt int) bool {
	switch d.config.Scenario {
	case ScenarioSuccess:
		return false
	case ScenarioFail:
		return true
	case ScenarioRetry:
		// Fail first 2 attempts, succeed on 3rd
		return attempt < 3
	case ScenarioMixed:
		// t02 needs retry, t03 always fails, others pass
		switch task.ID {
		case "t02":
			return attempt < 2
		case "t03":
			return true
		default:
			return false
		}
	}
	return false
}

// streamOutput writes realistic output for a task.
func (d *DemoRunner) streamOutput(
	ctx context.Context,
	task *plan.Task,
	attempt int,
	output executor.OutputWriter,
) {
	lines := d.generateOutput(task, attempt)

	lineDelay := d.config.TaskDelay / time.Duration(len(lines)+1)
	if lineDelay < 30*time.Millisecond {
		lineDelay = 30 * time.Millisecond
	}

	writer := output.Stdout()
	for _, line := range lines {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Fprintln(writer, line)
			time.Sleep(lineDelay)
		}
	}
}

// generateOutput creates realistic Claude Code output for a task.
func (d *DemoRunner) generateOutput(task *plan.Task, attempt int) []string {
	var lines []string

	shouldFail := d.shouldFail(task, attempt)

	// Opening context (matches real Claude output format)
	if attempt > 1 {
		lines = append(lines,
			fmt.Sprintf("Retrying task (attempt %d)...", attempt),
			"",
			"Analyzing previous failure and adjusting approach.",
			"",
		)
	}

	// Work description
	lines = append(lines,
		fmt.Sprintf("Working on: %s", task.Title),
		"",
		"## Implementation Progress",
		"",
	)

	// Simulated work steps based on task
	workLines := d.generateWorkLines(task)
	lines = append(lines, workLines...)

	// Acceptance criteria verification (matches real format)
	lines = append(lines,
		"",
		"## Acceptance Criteria Verification",
		"",
	)

	for i, criterion := range task.AcceptanceCriteria {
		// In fail scenario, last criterion fails
		if shouldFail && i == len(task.AcceptanceCriteria)-1 {
			lines = append(lines, fmt.Sprintf("%d. ❌ %s - **FAILED**", i+1, criterion))
		} else {
			lines = append(lines, fmt.Sprintf("%d. ✅ %s", i+1, criterion))
		}
	}

	lines = append(lines, "")

	if shouldFail {
		lines = append(lines,
			"Some acceptance criteria were not met.",
			"Task requires retry with adjusted approach.",
		)
	} else {
		lines = append(lines,
			"All acceptance criteria verified.",
			"",
			fmt.Sprintf("SUGGESTED_COMMIT_MESSAGE: %s", d.generateCommitMessage(task)),
		)
	}

	return lines
}

// generateWorkLines returns task-specific realistic output.
func (d *DemoRunner) generateWorkLines(task *plan.Task) []string {
	// Task-specific realistic output
	workTemplates := map[string][]string{
		"t01": {
			"Creating directory structure...",
			"  - internal/feature/",
			"  - internal/feature/models/",
			"  - internal/feature/handlers/",
			"",
			"Generating configuration files...",
			"  - config.yaml created",
			"  - .env.example created",
			"",
			"Running `make build`... Success",
		},
		"t02": {
			"Defining data models in `internal/feature/models/`...",
			"",
			"```go",
			"type Feature struct {",
			"    ID          string    `json:\"id\"`",
			"    Name        string    `json:\"name\"`",
			"    Description string    `json:\"description\"`",
			"    CreatedAt   time.Time `json:\"created_at\"`",
			"}",
			"```",
			"",
			"Adding interface definitions...",
			"Writing unit tests in `models_test.go`...",
			"Running `make test`... All tests pass",
		},
		"t03": {
			"Implementing core business logic...",
			"",
			"Added validation functions:",
			"  - ValidateName(): ensures non-empty, max 100 chars",
			"  - ValidateDescription(): sanitizes input",
			"",
			"Added error handling:",
			"  - Custom error types defined",
			"  - Wrapped errors with context",
			"",
			"Writing comprehensive tests...",
			"  - TestValidateName_Empty",
			"  - TestValidateName_TooLong",
			"  - TestValidateDescription_Sanitization",
		},
		"t04": {
			"Registering API routes...",
			"",
			"| Method | Path              | Handler          |",
			"|--------|-------------------|------------------|",
			"| GET    | /api/features     | ListFeatures     |",
			"| GET    | /api/features/:id | GetFeature       |",
			"| POST   | /api/features     | CreateFeature    |",
			"| DELETE | /api/features/:id | DeleteFeature    |",
			"",
			"Implementing request handlers...",
			"Adding integration tests...",
			"Running `make test`... All tests pass",
		},
		"t05": {
			"Updating README.md with feature documentation...",
			"",
			"Generating API documentation...",
			"  - OpenAPI spec created",
			"  - Endpoint descriptions added",
			"",
			"Adding usage examples:",
			"  - Basic usage example",
			"  - Advanced configuration example",
			"",
			"Documentation complete.",
		},
	}

	if lines, ok := workTemplates[task.ID]; ok {
		return lines
	}

	// Generic fallback
	return []string{
		"Analyzing requirements...",
		"Implementing changes...",
		"Running verification...",
	}
}

// generateCommitMessage returns a commit message for the task.
func (d *DemoRunner) generateCommitMessage(task *plan.Task) string {
	messages := map[string]string{
		"t01": "Set up project structure with configuration files",
		"t02": "Implement core data models and interfaces",
		"t03": "Add business logic layer with validation",
		"t04": "Create REST API endpoints with handlers",
		"t05": "Add documentation and usage examples",
	}

	if msg, ok := messages[task.ID]; ok {
		return msg
	}
	return fmt.Sprintf("Complete %s", strings.ToLower(task.Title))
}
