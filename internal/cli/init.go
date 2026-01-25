package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Rafa in the current repository",
	Long:  "Creates a .rafa/ folder to store plans and execution data.",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check prerequisites first
	if err := checkPrerequisites(); err != nil {
		return err
	}

	// Check if already initialized
	if IsInitialized() {
		return fmt.Errorf("rafa is already initialized in this repository")
	}

	// Create .rafa directory structure
	dirs := []string{
		rafaDir,
		filepath.Join(rafaDir, "plans"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	fmt.Println("Initialized Rafa in", rafaDir)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Create a technical design or PRD document")
	fmt.Println("  2. Run: rafa plan create <design.md>")
	return nil
}
