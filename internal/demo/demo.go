package demo

import (
	"fmt"
	"strings"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// Scenario defines the demo execution behavior.
type Scenario string

const (
	ScenarioSuccess Scenario = "success" // All tasks pass
	ScenarioMixed   Scenario = "mixed"   // Some pass, some fail, some retry
	ScenarioFail    Scenario = "fail"    // All tasks fail
	ScenarioRetry   Scenario = "retry"   // Tasks fail then succeed
)

// Speed controls the execution pace.
type Speed string

const (
	SpeedFast     Speed = "fast"     // 500ms, 5 tasks (~2.5s)
	SpeedNormal   Speed = "normal"   // 10s, 18 tasks (~3 min)
	SpeedSlow     Speed = "slow"     // 30s, 60 tasks (~30 min)
	SpeedMarathon Speed = "marathon" // 1m, 120 tasks (~2 hrs)
	SpeedExtended Speed = "extended" // 2m, 360 tasks (~12 hrs)
)

// Config holds demo mode configuration.
type Config struct {
	Scenario  Scenario
	Speed     Speed
	TaskDelay time.Duration // Computed from Speed or overridden
	TaskCount int           // Computed from Speed or overridden
}

// NewConfig creates a new demo configuration with computed task delay and count.
func NewConfig(scenario Scenario, speed Speed) *Config {
	c := &Config{Scenario: scenario, Speed: speed}
	switch speed {
	case SpeedFast:
		c.TaskDelay = 500 * time.Millisecond
		c.TaskCount = 5
	case SpeedNormal:
		c.TaskDelay = 10 * time.Second
		c.TaskCount = 18
	case SpeedSlow:
		c.TaskDelay = 30 * time.Second
		c.TaskCount = 60
	case SpeedMarathon:
		c.TaskDelay = 1 * time.Minute
		c.TaskCount = 120
	case SpeedExtended:
		c.TaskDelay = 2 * time.Minute
		c.TaskCount = 360
	default:
		c.TaskDelay = 10 * time.Second
		c.TaskCount = 18
	}
	return c
}

// NewConfigWithOptions creates a demo configuration with optional overrides.
// If taskDelay is 0, the speed preset's delay is used.
// If taskCount is 0, the speed preset's task count is used.
func NewConfigWithOptions(scenario Scenario, speed Speed, taskDelay time.Duration, taskCount int) *Config {
	c := NewConfig(scenario, speed)
	if taskDelay > 0 {
		c.TaskDelay = taskDelay
	}
	if taskCount > 0 {
		c.TaskCount = taskCount
	}
	return c
}

// CreateDemoPlan returns an in-memory plan for demo purposes with 5 tasks.
func CreateDemoPlan() *plan.Plan {
	return CreateDemoPlanWithTaskCount(5)
}

// Module names for generating varied tasks
var moduleNames = []string{
	"user", "auth", "billing", "inventory", "reporting",
	"analytics", "notifications", "search", "payments", "shipping",
	"catalog", "admin",
}

// Task patterns for generating varied tasks
var taskPatterns = []struct {
	suffix      string
	description string
	criteria    []string
}{
	{
		suffix:      "setup",
		description: "Set up project structure and configuration",
		criteria:    []string{"Directory structure exists", "Config files created", "make build succeeds"},
	},
	{
		suffix:      "data models",
		description: "Define data structures and interfaces",
		criteria:    []string{"Model structs defined", "Interfaces documented", "Unit tests pass"},
	},
	{
		suffix:      "business logic",
		description: "Implement business logic with validation",
		criteria:    []string{"Core functions implemented", "Input validation added", "Error handling complete"},
	},
	{
		suffix:      "API endpoints",
		description: "Build REST API endpoints",
		criteria:    []string{"Endpoints registered", "Request/response handling works", "Integration tests pass"},
	},
	{
		suffix:      "documentation",
		description: "Add usage documentation and examples",
		criteria:    []string{"README updated", "API docs generated", "Examples provided"},
	},
	{
		suffix:      "caching",
		description: "Implement caching layer for performance",
		criteria:    []string{"Cache strategy defined", "Cache invalidation works", "Performance improved"},
	},
	{
		suffix:      "monitoring",
		description: "Add metrics and monitoring",
		criteria:    []string{"Metrics exported", "Dashboards configured", "Alerts set up"},
	},
	{
		suffix:      "migrations",
		description: "Create database migrations",
		criteria:    []string{"Migration files created", "Up/down migrations work", "Data integrity maintained"},
	},
}

// CreateDemoPlanWithTaskCount returns an in-memory plan with the specified number of tasks.
// Tasks cycle through module names and task patterns for variety.
func CreateDemoPlanWithTaskCount(n int) *plan.Plan {
	tasks := make([]plan.Task, n)

	for i := 0; i < n; i++ {
		module := moduleNames[i%len(moduleNames)]
		pattern := taskPatterns[i%len(taskPatterns)]

		tasks[i] = plan.Task{
			ID:                 fmt.Sprintf("t%02d", i+1),
			Title:              fmt.Sprintf("%s %s %s", strings.Title(module), pattern.suffix, ""),
			Description:        fmt.Sprintf("%s for %s module", pattern.description, module),
			AcceptanceCriteria: pattern.criteria,
			Status:             plan.TaskStatusPending,
		}
		// Clean up title (remove trailing space)
		tasks[i].Title = strings.TrimSpace(tasks[i].Title)
	}

	return &plan.Plan{
		ID:          "demo-001",
		Name:        "demo-feature",
		Description: "A sample feature implementation for TUI demonstration",
		SourceFile:  "docs/designs/demo-feature.md",
		Status:      plan.PlanStatusNotStarted,
		Tasks:       tasks,
	}
}
