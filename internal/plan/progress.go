package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const progressLogFileName = "progress.log"

// Event type constants for progress logging.
const (
	EventPlanStarted   = "plan_started"
	EventPlanCompleted = "plan_completed"
	EventPlanCancelled = "plan_cancelled"
	EventPlanFailed    = "plan_failed"
	EventTaskStarted   = "task_started"
	EventTaskCompleted = "task_completed"
	EventTaskFailed    = "task_failed"
)

// ProgressEvent represents a single progress log entry.
type ProgressEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ProgressLogger writes progress events to a JSON Lines file.
type ProgressLogger struct {
	path string
}

// NewProgressLogger creates a new progress logger for the given plan directory.
func NewProgressLogger(planDir string) *ProgressLogger {
	return &ProgressLogger{
		path: filepath.Join(planDir, progressLogFileName),
	}
}

// Log appends a progress event to the log file.
func (p *ProgressLogger) Log(event string, data map[string]interface{}) error {
	entry := ProgressEvent{
		Timestamp: time.Now(),
		Event:     event,
		Data:      data,
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	jsonBytes = append(jsonBytes, '\n')

	f, err := os.OpenFile(p.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(jsonBytes)
	return err
}

// PlanStarted logs a plan_started event.
func (p *ProgressLogger) PlanStarted(planID string) error {
	return p.Log(EventPlanStarted, map[string]interface{}{
		"plan_id": planID,
	})
}

// TaskStarted logs a task_started event.
func (p *ProgressLogger) TaskStarted(taskID string, attempt int) error {
	return p.Log(EventTaskStarted, map[string]interface{}{
		"task_id": taskID,
		"attempt": attempt,
	})
}

// TaskCompleted logs a task_completed event.
func (p *ProgressLogger) TaskCompleted(taskID string) error {
	return p.Log(EventTaskCompleted, map[string]interface{}{
		"task_id": taskID,
	})
}

// TaskFailed logs a task_failed event.
func (p *ProgressLogger) TaskFailed(taskID string, attempt int) error {
	return p.Log(EventTaskFailed, map[string]interface{}{
		"task_id": taskID,
		"attempt": attempt,
	})
}

// PlanCompleted logs a plan_completed event with summary statistics.
func (p *ProgressLogger) PlanCompleted(totalTasks, succeededTasks int, duration time.Duration) error {
	return p.Log(EventPlanCompleted, map[string]interface{}{
		"total_tasks":     totalTasks,
		"succeeded_tasks": succeededTasks,
		"duration_ms":     duration.Milliseconds(),
	})
}

// PlanCancelled logs a plan_cancelled event.
func (p *ProgressLogger) PlanCancelled(lastTaskID string) error {
	return p.Log(EventPlanCancelled, map[string]interface{}{
		"last_task_id": lastTaskID,
	})
}

// PlanFailed logs a plan_failed event.
func (p *ProgressLogger) PlanFailed(taskID string, attempts int) error {
	return p.Log(EventPlanFailed, map[string]interface{}{
		"task_id":  taskID,
		"attempts": attempts,
	})
}
