package plan

import "time"

// Plan represents a collection of tasks extracted from a source document.
type Plan struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SourceFile  string    `json:"sourceFile"`
	CreatedAt   time.Time `json:"createdAt"`
	Status      string    `json:"status"`
	Tasks       []Task    `json:"tasks"`
}

// Plan status constants
const (
	PlanStatusNotStarted = "not_started"
	PlanStatusInProgress = "in_progress"
	PlanStatusCompleted  = "completed"
	PlanStatusFailed     = "failed"
)
