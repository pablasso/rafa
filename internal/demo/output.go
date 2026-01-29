package demo

import (
	"fmt"
	"strings"

	"github.com/pablasso/rafa/internal/plan"
)

// generateOutput creates realistic Claude Code output for a task.
// The output format matches real Claude Code execution logs.
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
// Each task ID (t01-t05) has a custom template that simulates
// realistic work being performed for that type of task.
// Dynamic tasks (t06+) generate output based on title keywords.
func (d *DemoRunner) generateWorkLines(task *plan.Task) []string {
	// Task-specific realistic output for core tasks
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

	// Dynamic output generation for tasks t06+
	return d.generateDynamicWorkLines(task)
}

// generateDynamicWorkLines creates output based on task title keywords.
func (d *DemoRunner) generateDynamicWorkLines(task *plan.Task) []string {
	title := strings.ToLower(task.Title)

	// Pattern-based output templates
	if strings.Contains(title, "setup") {
		return []string{
			"Initializing project structure...",
			"  - Creating directories",
			"  - Setting up configuration",
			"",
			"Running initial build check...",
			"Build successful.",
		}
	}

	if strings.Contains(title, "data model") || strings.Contains(title, "models") {
		return []string{
			"Defining data structures...",
			"",
			"```go",
			"type Entity struct {",
			"    ID        string `json:\"id\"`",
			"    CreatedAt time.Time",
			"}",
			"```",
			"",
			"Writing model tests...",
			"Running `make test`... All tests pass",
		}
	}

	if strings.Contains(title, "business logic") || strings.Contains(title, "logic") {
		return []string{
			"Implementing core logic...",
			"",
			"Added functions:",
			"  - Process(): handles main workflow",
			"  - Validate(): input validation",
			"",
			"Writing unit tests...",
			"Running `make test`... All tests pass",
		}
	}

	if strings.Contains(title, "api") || strings.Contains(title, "endpoint") {
		return []string{
			"Registering routes...",
			"",
			"| Method | Path     | Handler    |",
			"|--------|----------|------------|",
			"| GET    | /api/... | List       |",
			"| POST   | /api/... | Create     |",
			"",
			"Running integration tests...",
			"All endpoints verified.",
		}
	}

	if strings.Contains(title, "documentation") || strings.Contains(title, "docs") {
		return []string{
			"Updating documentation...",
			"",
			"  - README.md updated",
			"  - API docs generated",
			"  - Examples added",
			"",
			"Documentation complete.",
		}
	}

	if strings.Contains(title, "caching") || strings.Contains(title, "cache") {
		return []string{
			"Implementing caching layer...",
			"",
			"Cache configuration:",
			"  - TTL: 5 minutes",
			"  - Max entries: 1000",
			"  - Eviction: LRU",
			"",
			"Running cache tests...",
			"Cache layer verified.",
		}
	}

	if strings.Contains(title, "monitoring") || strings.Contains(title, "metrics") {
		return []string{
			"Setting up monitoring...",
			"",
			"Metrics added:",
			"  - request_count",
			"  - request_duration",
			"  - error_rate",
			"",
			"Configuring dashboards...",
			"Monitoring setup complete.",
		}
	}

	if strings.Contains(title, "migration") {
		return []string{
			"Creating database migrations...",
			"",
			"Migration files:",
			"  - 001_create_tables.up.sql",
			"  - 001_create_tables.down.sql",
			"",
			"Running migration tests...",
			"Migrations verified.",
		}
	}

	// Generic fallback with variety based on task ID
	return []string{
		"Analyzing requirements...",
		fmt.Sprintf("Implementing %s...", task.Title),
		"",
		"Running verification...",
		"Task complete.",
	}
}

// generateCommitMessage returns a suggested commit message for the task.
// Each task ID (t01-t05) has a specific commit message, with dynamic
// generation for other tasks based on title.
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

	// Generate commit message from title
	title := strings.ToLower(task.Title)
	switch {
	case strings.Contains(title, "setup"):
		return fmt.Sprintf("Set up %s", task.Title)
	case strings.Contains(title, "data model"):
		return fmt.Sprintf("Implement %s", task.Title)
	case strings.Contains(title, "api") || strings.Contains(title, "endpoint"):
		return fmt.Sprintf("Add %s", task.Title)
	case strings.Contains(title, "documentation"):
		return fmt.Sprintf("Document %s", task.Title)
	case strings.Contains(title, "caching"):
		return fmt.Sprintf("Add %s", task.Title)
	case strings.Contains(title, "monitoring"):
		return fmt.Sprintf("Configure %s", task.Title)
	case strings.Contains(title, "migration"):
		return fmt.Sprintf("Create %s", task.Title)
	default:
		return fmt.Sprintf("Complete %s", strings.ToLower(task.Title))
	}
}
