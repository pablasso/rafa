package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	deinitForce bool
)

var deinitCmd = &cobra.Command{
	Use:   "deinit",
	Short: "Remove Rafa from the current repository",
	Long:  "Removes the .rafa/ folder and all plans. This action cannot be undone.",
	RunE:  runDeinit,
}

func init() {
	deinitCmd.Flags().BoolVarP(&deinitForce, "force", "f", false, "Skip confirmation prompt")
}

func runDeinit(cmd *cobra.Command, args []string) error {
	// Check if initialized
	info, err := os.Stat(rafaDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("rafa is not initialized in this repository")
	}
	if err != nil {
		return fmt.Errorf("failed to check .rafa directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf(".rafa exists but is not a directory")
	}

	// Calculate what will be deleted
	planCount, totalSize, err := calculateDirStats(rafaDir)
	if err != nil {
		return fmt.Errorf("failed to analyze .rafa/: %w", err)
	}

	// Show confirmation unless --force
	if !deinitForce {
		fmt.Printf("This will delete .rafa/ (%d plans, %s). Continue? [y/N] ", planCount, formatSize(totalSize))

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Remove the directory
	if err := os.RemoveAll(rafaDir); err != nil {
		return fmt.Errorf("failed to remove .rafa/: %w", err)
	}

	// Remove lock file pattern from .gitignore
	if err := removeFromGitignore(gitignoreEntry); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	fmt.Println("Rafa has been removed from this repository.")
	return nil
}

func calculateDirStats(dir string) (planCount int, totalSize int64, err error) {
	plansDir := filepath.Join(dir, "plans")
	entries, readErr := os.ReadDir(plansDir)
	if readErr == nil {
		planCount = len(entries)
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
	)
	switch {
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
