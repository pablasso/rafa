package demo

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/tui/views"
)

// Playback drives demo events into the TUI.
type Playback struct {
	Config  Config
	Dataset *Dataset
}

// NewPlayback creates a demo playback from a dataset and config.
func NewPlayback(dataset *Dataset, config Config) *Playback {
	return &Playback{
		Config:  config,
		Dataset: dataset,
	}
}

// Run streams demo events to the TUI program and output channel.
func (p *Playback) Run(ctx context.Context, program *tea.Program, output chan<- string) {
	if p.Dataset == nil || p.Dataset.Plan == nil || program == nil || output == nil {
		return
	}
	defer close(output)

	start := time.Now()
	taskLimit := p.Config.MaxTasks
	if taskLimit <= 0 || taskLimit > len(p.Dataset.Plan.Tasks) {
		taskLimit = len(p.Dataset.Plan.Tasks)
	}

	attemptsByTask := groupAttempts(p.Dataset.Attempts)
	success := true
	completed := 0

	for idx := 0; idx < taskLimit; idx++ {
		task := p.Dataset.Plan.Tasks[idx]
		attempts := attemptsByTask[task.ID]

		if len(attempts) == 0 {
			attempts = []TaskAttempt{
				{
					TaskID:  task.ID,
					Attempt: 1,
					Success: true,
					Events: []Event{
						{Type: EventOutput, Text: fmt.Sprintf("Starting task %s...", task.Title)},
					},
				},
			}
		}

		sort.Slice(attempts, func(i, j int) bool {
			return attempts[i].Attempt < attempts[j].Attempt
		})

		taskSucceeded := false
		for _, attempt := range attempts {
			program.Send(views.TaskStartedMsg{
				TaskNum: idx + 1,
				Total:   taskLimit,
				TaskID:  task.ID,
				Title:   task.Title,
				Attempt: attempt.Attempt,
			})

			p.streamEvents(ctx, program, output, attempt.Events)

			if attempt.Success {
				program.Send(views.TaskCompletedMsg{TaskID: task.ID})
				completed++
				taskSucceeded = true
				break
			}

			program.Send(views.TaskFailedMsg{
				TaskID:  task.ID,
				Attempt: attempt.Attempt,
				Err:     fmt.Errorf("demo failure"),
			})

			if !p.wait(ctx, p.Config.LineDelay*10) {
				return
			}
		}

		if !taskSucceeded {
			success = false
			break
		}

		if idx < taskLimit-1 {
			if !p.wait(ctx, p.Config.TaskDelay) {
				return
			}
		}
	}

	duration := time.Since(start)
	program.Send(views.PlanDoneMsg{
		Success:   success,
		Message:   fmt.Sprintf("Demo stopped on task %d", completed+1),
		Succeeded: completed,
		Total:     taskLimit,
		Duration:  duration,
	})
}

func (p *Playback) streamEvents(ctx context.Context, program *tea.Program, output chan<- string, events []Event) {
	limit := len(events)
	if p.Config.MaxEventsPerTask > 0 && limit > p.Config.MaxEventsPerTask {
		limit = p.Config.MaxEventsPerTask
	}

	for i := 0; i < limit; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		event := events[i]
		switch event.Type {
		case EventOutput:
			select {
			case output <- event.Text:
			default:
			}
		case EventToolUse:
			program.Send(views.ToolUseMsg{
				ToolName:   event.ToolName,
				ToolTarget: event.ToolTarget,
			})
		case EventToolResult:
			program.Send(views.ToolResultMsg{})
		case EventUsage:
			program.Send(views.UsageMsg{
				InputTokens:  event.InputTokens,
				OutputTokens: event.OutputTokens,
				CostUSD:      event.CostUSD,
			})
		}

		if !p.wait(ctx, p.Config.LineDelay) {
			return
		}
	}
}

func (p *Playback) wait(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func groupAttempts(attempts []TaskAttempt) map[string][]TaskAttempt {
	result := make(map[string][]TaskAttempt)
	for _, attempt := range attempts {
		result[attempt.TaskID] = append(result[attempt.TaskID], attempt)
	}
	return result
}

func bestAttempt(attempts []TaskAttempt) *TaskAttempt {
	if len(attempts) == 0 {
		return nil
	}

	// Prefer the latest successful attempt.
	bestIndex := -1
	bestAttemptNum := -1
	for i := range attempts {
		if attempts[i].Success && attempts[i].Attempt > bestAttemptNum {
			bestIndex = i
			bestAttemptNum = attempts[i].Attempt
		}
	}
	if bestIndex >= 0 {
		return &attempts[bestIndex]
	}

	// Otherwise prefer attempt 1.
	for i := range attempts {
		if attempts[i].Attempt == 1 {
			return &attempts[i]
		}
	}
	return &attempts[0]
}

// FallbackDataset provides a small in-memory dataset when real data is missing.
func FallbackDataset() *Dataset {
	p := &plan.Plan{
		ID:          "DEMO",
		Name:        "demo",
		Description: "Fallback demo plan",
		Tasks: []plan.Task{
			{ID: "t01", Title: "Load demo data", Status: plan.TaskStatusPending},
			{ID: "t02", Title: "Replay output stream", Status: plan.TaskStatusPending},
			{ID: "t03", Title: "Show completion state", Status: plan.TaskStatusPending},
		},
	}

	events := []Event{
		{Type: EventOutput, Text: "Starting demo playback..."},
		{Type: EventOutput, Text: "Streaming example output..."},
		{Type: EventOutput, Text: "All done."},
	}

	return &Dataset{
		Plan: p,
		Attempts: []TaskAttempt{
			{TaskID: "t01", Attempt: 1, Success: true, Events: events},
			{TaskID: "t02", Attempt: 1, Success: true, Events: events},
			{TaskID: "t03", Attempt: 1, Success: true, Events: events},
		},
	}
}
