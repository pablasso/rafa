package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	rafaDir  = ".rafa"
	plansDir = "plans"
)

// ResolvePlanName checks for name collisions in the plans directory and returns
// a unique name. If the baseName is not taken, it returns as-is. If taken, it
// appends -2, -3, etc. until a unique name is found.
func ResolvePlanName(baseName string) (string, error) {
	plansPath := filepath.Join(rafaDir, plansDir)

	entries, err := os.ReadDir(plansPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet, so no collisions possible
			return baseName, nil
		}
		return "", fmt.Errorf("failed to read plans directory: %w", err)
	}

	// Build a set of existing names (extracted from folder names)
	existingNames := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Folder format is <id>-<name>, so we split on first hyphen
		parts := strings.SplitN(entry.Name(), "-", 2)
		if len(parts) == 2 {
			existingNames[parts[1]] = true
		}
	}

	// If baseName is not taken, return it
	if !existingNames[baseName] {
		return baseName, nil
	}

	// Find a unique suffix
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", baseName, suffix)
		if !existingNames[candidate] {
			return candidate, nil
		}
	}
}

// CreatePlanFolder creates the plan folder structure with plan.json and log files.
// The folder is created at .rafa/plans/<id>-<name>/
func CreatePlanFolder(plan *Plan) error {
	folderName := fmt.Sprintf("%s-%s", plan.ID, plan.Name)
	folderPath := filepath.Join(rafaDir, plansDir, folderName)

	// Create directory structure
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return fmt.Errorf("failed to create plan folder: %w", err)
	}

	// Write plan.json with pretty formatting
	planData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	planPath := filepath.Join(folderPath, "plan.json")
	if err := os.WriteFile(planPath, planData, 0644); err != nil {
		return fmt.Errorf("failed to write plan.json: %w", err)
	}

	// Create empty log files
	progressLogPath := filepath.Join(folderPath, "progress.log")
	if err := os.WriteFile(progressLogPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create progress.log: %w", err)
	}

	outputLogPath := filepath.Join(folderPath, "output.log")
	if err := os.WriteFile(outputLogPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create output.log: %w", err)
	}

	return nil
}
