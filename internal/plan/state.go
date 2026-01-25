package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindPlanFolder finds a plan folder by name suffix in .rafa/plans/.
// Returns the full path to the plan folder.
func FindPlanFolder(name string) (string, error) {
	plansPath := filepath.Join(rafaDir, plansDir)

	entries, err := os.ReadDir(plansPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no plans found. Run 'rafa plan create <design.md>' first")
		}
		return "", fmt.Errorf("failed to read plans directory: %w", err)
	}

	var matches []string
	suffix := "-" + name

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), suffix) {
			matches = append(matches, entry.Name())
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("plan not found: %s", name)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("multiple plans match '%s': %v", name, matches)
	}

	return filepath.Join(plansPath, matches[0]), nil
}

// LoadPlan reads and parses plan.json from a plan directory.
func LoadPlan(planDir string) (*Plan, error) {
	planPath := filepath.Join(planDir, "plan.json")

	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan.json: %w", err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan.json: %w", err)
	}

	return &plan, nil
}

// SavePlan atomically writes plan.json to the plan directory.
// Uses a temp file + rename to ensure atomic writes.
func SavePlan(planDir string, p *Plan) error {
	planPath := filepath.Join(planDir, "plan.json")
	tmpPath := fmt.Sprintf("%s.tmp.%d", planPath, os.Getpid())

	// Marshal with 2-space indent to match CreatePlanFolder
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Write to temp file
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename temp file to plan.json (atomic operation)
	if err := os.Rename(tmpPath, planPath); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// FirstPendingTask finds the first non-completed task.
// If a failed task is found, its status is reset to pending (attempts are preserved).
// Returns the task index, or -1 if all tasks are completed.
func (p *Plan) FirstPendingTask() int {
	for i := range p.Tasks {
		switch p.Tasks[i].Status {
		case TaskStatusPending, TaskStatusInProgress:
			return i
		case TaskStatusFailed:
			// Reset failed task to pending, preserving attempts
			p.Tasks[i].Status = TaskStatusPending
			return i
		}
	}
	return -1
}

// AllTasksCompleted returns true if all tasks have status completed.
func (p *Plan) AllTasksCompleted() bool {
	for i := range p.Tasks {
		if p.Tasks[i].Status != TaskStatusCompleted {
			return false
		}
	}
	return true
}
