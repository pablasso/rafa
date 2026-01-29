package demo

import (
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
	SpeedFast   Speed = "fast"   // 500ms per task
	SpeedNormal Speed = "normal" // 2s per task
	SpeedSlow   Speed = "slow"   // 5s per task
)

// Config holds demo mode configuration.
type Config struct {
	Scenario  Scenario
	Speed     Speed
	TaskDelay time.Duration // Computed from Speed
}

// NewConfig creates a new demo configuration with computed task delay.
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

// CreateDemoPlan returns an in-memory plan for demo purposes.
func CreateDemoPlan() *plan.Plan {
	return &plan.Plan{
		ID:          "demo-001",
		Name:        "demo-feature",
		Description: "A sample feature implementation for TUI demonstration",
		SourceFile:  "docs/designs/demo-feature.md",
		Status:      plan.PlanStatusNotStarted,
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
				Status: plan.TaskStatusPending,
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
				Status: plan.TaskStatusPending,
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
				Status: plan.TaskStatusPending,
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
				Status: plan.TaskStatusPending,
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
				Status: plan.TaskStatusPending,
			},
		},
	}
}
