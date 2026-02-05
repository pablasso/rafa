package demo

import (
	"fmt"
	"strings"

	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
)

// Scenario controls task outcomes during demo playback.
type Scenario string

const (
	ScenarioSuccess Scenario = "success"
	ScenarioFlaky   Scenario = "flaky"
	ScenarioFail    Scenario = "fail"
)

func ParseScenario(value string) (Scenario, error) {
	switch Scenario(strings.ToLower(strings.TrimSpace(value))) {
	case ScenarioSuccess, ScenarioFlaky, ScenarioFail:
		return Scenario(strings.ToLower(strings.TrimSpace(value))), nil
	default:
		return "", fmt.Errorf("invalid demo scenario %q (valid: success, flaky, fail)", value)
	}
}

// ApplyScenario returns a new dataset shaped for the selected scenario.
//
// The returned dataset is trimmed to the tasks that will actually be played for this scenario:
// - success/flaky: up to maxTasks (or all tasks if maxTasks <= 0)
// - fail: up to the deterministic failing task index
func ApplyScenario(base *Dataset, scenario Scenario, maxTasks int) (*Dataset, error) {
	if base == nil || base.Plan == nil {
		return nil, fmt.Errorf("nil dataset")
	}

	taskLimit := maxTasks
	if taskLimit <= 0 || taskLimit > len(base.Plan.Tasks) {
		taskLimit = len(base.Plan.Tasks)
	}
	if taskLimit == 0 {
		return &Dataset{Plan: &plan.Plan{ID: base.Plan.ID, Name: base.Plan.Name}}, nil
	}

	flakyTargets := []int{2, 4}
	failTarget := 3

	switch scenario {
	case ScenarioSuccess:
		// no-op; keep taskLimit
	case ScenarioFlaky:
		for i := range flakyTargets {
			if flakyTargets[i] > taskLimit {
				flakyTargets[i] = taskLimit
			}
		}
	case ScenarioFail:
		if failTarget > taskLimit {
			failTarget = taskLimit
		}
		taskLimit = failTarget
	default:
		return nil, fmt.Errorf("unknown demo scenario %q", scenario)
	}

	out := &Dataset{
		Plan: &plan.Plan{
			ID:   base.Plan.ID,
			Name: base.Plan.Name,
		},
	}
	out.Plan.Tasks = append(out.Plan.Tasks, cloneTasks(base.Plan.Tasks[:taskLimit])...)

	attemptsByTask := groupAttempts(base.Attempts)
	for idx, task := range out.Plan.Tasks {
		baseAttempt := bestAttempt(attemptsByTask[task.ID])
		if baseAttempt == nil {
			baseAttempt = &TaskAttempt{
				TaskID:  task.ID,
				Attempt: 1,
				Success: true,
				Events: []Event{
					{Type: EventOutput, Text: fmt.Sprintf("Starting task %s...", task.Title)},
				},
			}
		}

		taskIndex := idx + 1 // 1-based

		switch scenario {
		case ScenarioSuccess:
			out.Attempts = append(out.Attempts, TaskAttempt{
				TaskID:  task.ID,
				Attempt: 1,
				Success: true,
				Events:  cloneEvents(baseAttempt.Events),
			})

		case ScenarioFlaky:
			if taskIndex == flakyTargets[0] || taskIndex == flakyTargets[1] {
				out.Attempts = append(out.Attempts,
					TaskAttempt{
						TaskID:  task.ID,
						Attempt: 1,
						Success: false,
						Events:  buildFailedAttemptEvents(baseAttempt.Events),
					},
					TaskAttempt{
						TaskID:  task.ID,
						Attempt: 2,
						Success: true,
						Events:  cloneEvents(baseAttempt.Events),
					},
				)
			} else {
				out.Attempts = append(out.Attempts, TaskAttempt{
					TaskID:  task.ID,
					Attempt: 1,
					Success: true,
					Events:  cloneEvents(baseAttempt.Events),
				})
			}

		case ScenarioFail:
			if taskIndex == failTarget {
				for attempt := 1; attempt <= executor.MaxAttempts; attempt++ {
					out.Attempts = append(out.Attempts, TaskAttempt{
						TaskID:  task.ID,
						Attempt: attempt,
						Success: false,
						Events:  buildFailedAttemptEvents(baseAttempt.Events),
					})
				}
			} else {
				out.Attempts = append(out.Attempts, TaskAttempt{
					TaskID:  task.ID,
					Attempt: 1,
					Success: true,
					Events:  cloneEvents(baseAttempt.Events),
				})
			}
		}
	}

	return out, nil
}

func cloneTasks(tasks []plan.Task) []plan.Task {
	cloned := make([]plan.Task, 0, len(tasks))
	for _, task := range tasks {
		task.Status = plan.TaskStatusPending
		cloned = append(cloned, task)
	}
	return cloned
}

func cloneEvents(events []Event) []Event {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]Event, len(events))
	copy(cloned, events)
	return cloned
}

func buildFailedAttemptEvents(base []Event) []Event {
	var usage *Event
	for i := len(base) - 1; i >= 0; i-- {
		if base[i].Type == EventUsage {
			tmp := base[i]
			usage = &tmp
			break
		}
	}

	limit := len(base)
	if limit > 80 {
		limit = 80
	}
	events := cloneEvents(base[:limit])
	events = append(events, Event{
		Type: EventOutput,
		Text: "\nError: demo injected failure\n",
	})
	if usage != nil {
		events = append(events, *usage)
	}
	return events
}
