# Technical Design: TUI Demo Mode

## Overview

Add a `rafa demo` command that launches the TUI with simulated execution, allowing rapid iteration on TUI changes without requiring real Claude Code execution. The demo mode streams realistic fake output and supports multiple scenarios (all pass, mixed, failures, retries).

**Related PRD**: [docs/prds/rafa-core.md](../prds/rafa-core.md)

## Goals

- Enable rapid TUI development iteration without Claude Code
- Simulate realistic execution output that matches real Claude Code format
- Support multiple scenarios to test different UI states (success, failure, retry)
- Allow speed control for quick testing vs realistic demos
- No Claude authentication required

## Non-Goals

- Replacing actual integration tests
- Simulating all possible Claude Code edge cases
- Network simulation or API mocking

## Architecture

The existing codebase already supports dependency injection via the `executor.Runner` interface and `Executor.WithRunner()` method. Demo mode leverages this by injecting a `DemoRunner` that simulates execution.

```
┌─────────────────────────────────────────────────────────────────┐
│                         cmd/rafa/main.go                        │
│                    rafa demo [--scenario=X] [--speed=Y]         │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                       internal/tui/app.go                       │
│                    Run(WithDemoMode(...))                       │
└─────────────────────────────────────────────────────────────────┘
                                  │
              ┌───────────────────┴───────────────────┐
              ▼                                       ▼
┌─────────────────────────┐             ┌─────────────────────────┐
│   Normal Mode           │             │   Demo Mode             │
│   - ClaudeRunner        │             │   - DemoRunner          │
│   - Load from disk      │             │   - In-memory demo plan │
│   - Requires auth       │             │   - No auth required    │
│   - Git operations      │             │   - Skip git ops        │
└─────────────────────────┘             └─────────────────────────┘
              │                                       │
              └───────────────┬───────────────────────┘
                              ▼
                ┌─────────────────────────┐
                │   executor.Runner       │
                │   (interface)           │
                │   - ClaudeRunner        │
                │   - DemoRunner          │
                └─────────────────────────┘
```

### Leveraging Existing Infrastructure

The `executor.Runner` interface (defined in `internal/executor/executor.go:18`):

```go
type Runner interface {
    Run(ctx context.Context, task *plan.Task, planContext string,
        attempt, maxAttempts int, output OutputWriter) error
}
```

The `Executor` already supports injection via `WithRunner()` (`internal/executor/executor.go:53`):

```go
func (e *Executor) WithRunner(r Runner) *Executor {
    e.runner = r
    return e
}
```

Demo mode simply provides a `DemoRunner` implementing this interface.

## Technical Details

### Command Interface

```bash
# Basic demo - all tasks succeed
rafa demo

# Scenario flags
rafa demo --scenario=success    # All tasks pass (default)
rafa demo --scenario=mixed      # Some pass, some fail, some retry
rafa demo --scenario=fail       # All tasks fail after max retries
rafa demo --scenario=retry      # Tasks fail then succeed on retry

# Speed control
rafa demo --speed=fast          # 500ms per task (quick iteration)
rafa demo --speed=normal        # 2s per task (default)
rafa demo --speed=slow          # 5s per task (realistic demo)

# Combined
rafa demo --scenario=mixed --speed=fast
```

### New Package: internal/demo/

#### demo.go - Demo Configuration and Plan Generation

```go
package demo

import (
    "time"

    "github.com/pablasso/rafa/internal/plan"
)

type Scenario string

const (
    ScenarioSuccess Scenario = "success"  // All tasks pass
    ScenarioMixed   Scenario = "mixed"    // Some pass, some fail, some retry
    ScenarioFail    Scenario = "fail"     // All tasks fail
    ScenarioRetry   Scenario = "retry"    // Tasks fail then succeed
)

type Speed string

const (
    SpeedFast   Speed = "fast"   // 500ms per task
    SpeedNormal Speed = "normal" // 2s per task
    SpeedSlow   Speed = "slow"   // 5s per task
)

type Config struct {
    Scenario  Scenario
    Speed     Speed
    TaskDelay time.Duration // Computed from Speed
}

func NewConfig(scenario Scenario, speed Speed) *Config {
    c := &Config{Scenario: scenario, Speed: speed}
    switch speed {
    case SpeedFast:
        c.TaskDelay = 500 * time.Millisecond
    case SpeedSlow:
        c.TaskDelay = 5 * time.Second
    default:
        c.TaskDelay = 2 * time.Second
    }
    return c
}

// CreateDemoPlan returns an in-memory plan for demo purposes
func CreateDemoPlan() *plan.Plan {
    return &plan.Plan{
        ID:          "demo-001",
        Name:        "demo-feature",
        Description: "A sample feature implementation for TUI demonstration",
        SourceFile:  "docs/designs/demo-feature.md",
        Status:      plan.StatusNotStarted,
        Tasks: []plan.Task{
            {
                ID:          "t01",
                Title:       "Set up project structure",
                Description: "Create the base directory structure and configuration files",
                AcceptanceCriteria: []string{
                    "Directory structure exists",
                    "Config files created",
                    "make build succeeds",
                },
                Status: plan.StatusPending,
            },
            {
                ID:          "t02",
                Title:       "Implement core data models",
                Description: "Define the primary data structures and interfaces",
                AcceptanceCriteria: []string{
                    "Model structs defined",
                    "Interfaces documented",
                    "Unit tests pass",
                },
                Status: plan.StatusPending,
            },
            {
                ID:          "t03",
                Title:       "Add business logic layer",
                Description: "Implement the main business logic with validation",
                AcceptanceCriteria: []string{
                    "Core functions implemented",
                    "Input validation added",
                    "Error handling complete",
                    "Unit tests cover edge cases",
                },
                Status: plan.StatusPending,
            },
            {
                ID:          "t04",
                Title:       "Create API endpoints",
                Description: "Build REST API endpoints for the feature",
                AcceptanceCriteria: []string{
                    "Endpoints registered",
                    "Request/response handling works",
                    "Integration tests pass",
                },
                Status: plan.StatusPending,
            },
            {
                ID:          "t05",
                Title:       "Write documentation",
                Description: "Add usage documentation and examples",
                AcceptanceCriteria: []string{
                    "README updated",
                    "API docs generated",
                    "Examples provided",
                },
                Status: plan.StatusPending,
            },
        },
    }
}
```

#### runner.go - Demo Runner Implementation

The `DemoRunner` implements the existing `executor.Runner` interface:

```go
package demo

import (
    "context"
    "errors"
    "fmt"
    "math/rand"
    "time"

    "github.com/pablasso/rafa/internal/executor"
    "github.com/pablasso/rafa/internal/plan"
)

// DemoRunner implements executor.Runner with simulated execution.
type DemoRunner struct {
    config *Config
    rng    *rand.Rand
}

// Compile-time interface verification
var _ executor.Runner = (*DemoRunner)(nil)

func NewDemoRunner(config *Config) *DemoRunner {
    return &DemoRunner{
        config: config,
        rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// Run implements executor.Runner interface.
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
        // Task t03 always fails, t02 needs retry, others succeed
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
```

#### output.go - Realistic Output Generation

Based on actual Claude Code output format observed in `.rafa/plans/*/output.log`:

```go
package demo

import (
    "fmt"
    "strings"

    "github.com/pablasso/rafa/internal/plan"
)

// generateOutput creates realistic Claude Code output for a task
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
```

### Integration with TUI

#### Modified internal/tui/app.go

Add demo mode support to the existing TUI:

```go
type Model struct {
    // ... existing fields (currentView, width, height, sub-models, etc.)

    // Demo mode fields
    demoMode   bool
    demoConfig *demo.Config
}

type Option func(*Model)

func WithDemoMode(config *demo.Config) Option {
    return func(m *Model) {
        m.demoMode = true
        m.demoConfig = config
    }
}

// Run starts the TUI application with optional configuration.
func Run(opts ...Option) error {
    m := initialModel()
    for _, opt := range opts {
        opt(&m)
    }

    p := tea.NewProgram(
        m,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )
    _, err := p.Run()
    return err
}
```

#### New file: cmd/demo.go

```go
package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/pablasso/rafa/internal/demo"
    "github.com/pablasso/rafa/internal/tui"
)

var demoCmd = &cobra.Command{
    Use:   "demo",
    Short: "Launch TUI in demo mode with simulated execution",
    Long: `Launch the TUI with simulated plan execution for testing and demonstration.

No Claude authentication required. Useful for:
  - Iterating on TUI changes quickly
  - Demonstrating rafa to others
  - Testing different UI states

Scenarios:
  success  All tasks pass (default)
  mixed    Some pass, some fail, some need retry
  fail     All tasks fail after max retries
  retry    Tasks fail initially, succeed on retry

Speeds:
  fast     500ms per task (quick iteration)
  normal   2s per task (default)
  slow     5s per task (realistic demo)`,
    Run: runDemo,
}

var (
    demoScenario string
    demoSpeed    string
)

func init() {
    rootCmd.AddCommand(demoCmd)
    demoCmd.Flags().StringVar(&demoScenario, "scenario", "success",
        "Demo scenario: success, mixed, fail, retry")
    demoCmd.Flags().StringVar(&demoSpeed, "speed", "normal",
        "Execution speed: fast, normal, slow")
}

func runDemo(cmd *cobra.Command, args []string) {
    scenario := demo.Scenario(demoScenario)
    speed := demo.Speed(demoSpeed)

    // Validate inputs
    switch scenario {
    case demo.ScenarioSuccess, demo.ScenarioMixed,
         demo.ScenarioFail, demo.ScenarioRetry:
        // valid
    default:
        fmt.Fprintf(os.Stderr, "Invalid scenario: %s\n", demoScenario)
        os.Exit(1)
    }

    switch speed {
    case demo.SpeedFast, demo.SpeedNormal, demo.SpeedSlow:
        // valid
    default:
        fmt.Fprintf(os.Stderr, "Invalid speed: %s\n", demoSpeed)
        os.Exit(1)
    }

    config := demo.NewConfig(scenario, speed)

    if err := tui.Run(tui.WithDemoMode(config)); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Demo Mode Behavior

When in demo mode, the TUI will:

1. **Skip authentication check** - No Claude CLI required
2. **Use in-memory demo plan** - `demo.CreateDemoPlan()` instead of loading from disk
3. **Inject DemoRunner** - Via `Executor.WithRunner(demo.NewDemoRunner(config))`
4. **Skip git operations** - No workspace cleanliness checks or commits
5. **Display "[DEMO]" indicator** - In status bar to clarify mode

### UI Changes in Demo Mode

```
┌─────────────────────────────────────────────────────────────────┐
│  RAFA - Plan Executor                              [DEMO MODE]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Demo Feature Plan                                              │
│  ─────────────────                                              │
│  A sample feature implementation for TUI demonstration          │
│                                                                 │
│  Tasks:                                                         │
│  ● t01: Set up project structure                                │
│  ○ t02: Implement core data models                              │
│  ○ t03: Add business logic layer                                │
│  ○ t04: Create API endpoints                                    │
│  ○ t05: Write documentation                                     │
│                                                                 │
│  [Enter] Start execution • [Esc] Home • [q] Quit                │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Tasks

1. **Create internal/demo/ package**
   - `demo.go`: Config, Scenario, Speed types, CreateDemoPlan()
   - `runner.go`: DemoRunner implementing executor.Runner
   - `output.go`: Realistic output generation

2. **Add cmd/demo.go**
   - Cobra command with --scenario and --speed flags
   - Input validation
   - TUI launch with demo config

3. **Modify internal/tui/app.go**
   - Add demoMode and demoConfig fields
   - Add WithDemoMode() option
   - Change Run() signature to accept options

4. **Modify internal/tui/views/home.go**
   - In demo mode, skip to plan view with demo plan
   - Add [DEMO] indicator to status bar

5. **Modify internal/tui/views/run.go**
   - Check for demoMode and inject DemoRunner via WithRunner()
   - Skip git checks when in demo mode

6. **Add unit tests**
   - Test DemoRunner output generation
   - Test scenario behavior (success, fail, retry, mixed)
   - Test speed configurations

## Testing Strategy

```bash
# Manual testing
rafa demo                           # Quick check - all success
rafa demo --scenario=mixed          # See failures and retries
rafa demo --speed=fast              # Rapid iteration

# Automated testing
go test ./internal/demo/...         # Unit tests for demo package
```

## Rollout

### Phase 1: Core Demo Infrastructure
- Create internal/demo/ package
- Implement DemoRunner with basic output
- Add `rafa demo` command with basic wiring

### Phase 2: Scenarios and Speed Controls
- Implement all scenarios (success, mixed, fail, retry)
- Add speed controls (fast, normal, slow)
- Polish output to match real Claude format

### Phase 3: Full TUI Integration
- Wire demo mode into all TUI views
- Add [DEMO] indicator
- Skip auth and git checks in demo mode
- Update help text and documentation
