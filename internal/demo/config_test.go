package demo

import (
	"fmt"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

func TestNewConfig_ScalesToPresetTargets(t *testing.T) {
	base := makeDataset(10, 1000)

	tests := []struct {
		name      string
		preset    Preset
		maxTasks  int
		target    time.Duration
		tolerance time.Duration
	}{
		{
			name:      "quick",
			preset:    PresetQuick,
			maxTasks:  5,
			target:    time.Minute,
			tolerance: 3 * time.Second,
		},
		{
			name:      "medium",
			preset:    PresetMedium,
			maxTasks:  10,
			target:    30 * time.Minute,
			tolerance: 10 * time.Second,
		},
		{
			name:      "slow",
			preset:    PresetSlow,
			maxTasks:  0,
			target:    2 * time.Hour,
			tolerance: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := ApplyScenario(base, ScenarioSuccess, tt.maxTasks)
			if err != nil {
				t.Fatalf("ApplyScenario: %v", err)
			}

			cfg, err := NewConfig(tt.preset, ds)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}

			_, eventCount := estimateCounts(ds, cfg.MaxEventsPerTask)
			estimated := time.Duration(eventCount)*cfg.LineDelay + time.Duration(cfg.MaxTasks)*cfg.TaskDelay
			if delta := absDuration(estimated - tt.target); delta > tt.tolerance {
				t.Fatalf("estimated=%s target=%s delta=%s (LineDelay=%s TaskDelay=%s events=%d tasks=%d)",
					estimated, tt.target, delta, cfg.LineDelay, cfg.TaskDelay, eventCount, cfg.MaxTasks)
			}
		})
	}
}

func makeDataset(taskCount int, eventsPerAttempt int) *Dataset {
	p := &plan.Plan{
		ID:   "DEMO",
		Name: "demo",
	}
	var attempts []TaskAttempt

	for i := 1; i <= taskCount; i++ {
		taskID := fmt.Sprintf("t%02d", i)
		p.Tasks = append(p.Tasks, plan.Task{
			ID:     taskID,
			Title:  fmt.Sprintf("Task %d", i),
			Status: plan.TaskStatusPending,
		})

		events := make([]Event, 0, eventsPerAttempt)
		for j := 0; j < eventsPerAttempt; j++ {
			events = append(events, Event{Type: EventOutput, Text: "x"})
		}
		events = append(events, Event{Type: EventUsage, InputTokens: 1, OutputTokens: 1, CostUSD: 0.0001})

		attempts = append(attempts, TaskAttempt{
			TaskID:  taskID,
			Attempt: 1,
			Success: true,
			Events:  events,
		})
	}

	return &Dataset{
		Plan:     p,
		Attempts: attempts,
	}
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
