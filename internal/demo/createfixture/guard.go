package createfixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizeSourceFile normalizes a source file path to repo-relative slash form.
func NormalizeSourceFile(repoRoot, sourceFile string) (string, error) {
	if sourceFile == "" {
		return "", fmt.Errorf("source file is required")
	}
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	sourceAbs, err := filepath.Abs(sourceFile)
	if err != nil {
		return "", fmt.Errorf("resolve source file: %w", err)
	}

	rel, err := filepath.Rel(rootAbs, sourceAbs)
	if err != nil {
		return "", fmt.Errorf("compute source file relative path: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

// SourceFileAlreadyPlanned checks whether any existing plan references sourceFile.
func SourceFileAlreadyPlanned(repoRoot, sourceFile string) (bool, error) {
	normalized, err := NormalizeSourceFile(repoRoot, sourceFile)
	if err != nil {
		return false, err
	}

	plansDir := filepath.Join(repoRoot, ".rafa", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read plans directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		planPath := filepath.Join(plansDir, entry.Name(), "plan.json")
		data, err := os.ReadFile(planPath)
		if err != nil {
			continue
		}
		var payload struct {
			SourceFile string `json:"sourceFile"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			continue
		}
		if payload.SourceFile == "" {
			continue
		}
		planSource := filepath.ToSlash(strings.TrimSpace(payload.SourceFile))
		if planSource == normalized {
			return true, nil
		}
	}

	return false, nil
}

// EnsureSourceFileHasNoPlan returns an error when sourceFile is already referenced by any plan.
func EnsureSourceFileHasNoPlan(repoRoot, sourceFile string) error {
	normalized, err := NormalizeSourceFile(repoRoot, sourceFile)
	if err != nil {
		return err
	}
	exists, err := SourceFileAlreadyPlanned(repoRoot, sourceFile)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("source design document %q already has a plan; choose a design doc without an existing plan", normalized)
	}
	return nil
}
