package plan

import (
	"errors"
	"fmt"
)

// TaskExtractionResult represents the structured response from AI task extraction.
type TaskExtractionResult struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Tasks       []ExtractedTask `json:"tasks"`
}

// ExtractedTask represents a single task extracted by the AI.
type ExtractedTask struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

// Validate checks that the extraction result contains valid data.
func (r *TaskExtractionResult) Validate() error {
	// Name is optional (falls back to filename)
	if len(r.Tasks) == 0 {
		return errors.New("no tasks extracted")
	}
	for i, task := range r.Tasks {
		if task.Title == "" {
			return fmt.Errorf("task %d missing title", i+1)
		}
		if len(task.AcceptanceCriteria) == 0 {
			return fmt.Errorf("task %d (%s) missing acceptance criteria", i+1, task.Title)
		}
	}
	return nil
}
