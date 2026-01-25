package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCmd_MissingArg(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Args = cobra.ExactArgs(1)

	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("expected error for missing plan name, got nil")
	}
}

func TestRunPlan_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// No .rafa/ directory

	cmd := &cobra.Command{}
	err := runPlan(cmd, []string{"foo"})

	if err == nil {
		t.Error("expected error for missing .rafa/, got nil")
	}
	if err != nil && err.Error() != "rafa not initialized. Run `rafa init` first" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunPlan_PlanNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create .rafa/plans/ but no plans
	os.MkdirAll(filepath.Join(tmpDir, ".rafa", "plans"), 0755)

	cmd := &cobra.Command{}
	err := runPlan(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for nonexistent plan, got nil")
	}
	if err != nil && err.Error() != "plan not found: nonexistent" {
		t.Errorf("unexpected error message: %v", err)
	}
}
