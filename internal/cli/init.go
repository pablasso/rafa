package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	gitignoreEntry = ".rafa/**/*.lock"
	gitignorePath  = ".gitignore"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Rafa in the current repository",
	Long:  "Creates a .rafa/ folder to store plans and execution data.",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check prerequisites with progress feedback
	if err := checkPrerequisitesWithProgress(); err != nil {
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

	// Add lock file pattern to .gitignore
	if err := addToGitignore(gitignoreEntry); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	fmt.Println("Initialized Rafa in", rafaDir)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Create a technical design or PRD document")
	fmt.Println("  2. Run: rafa plan create <design.md>")
	return nil
}

// addToGitignore adds an entry to .gitignore if not already present.
func addToGitignore(entry string) error {
	// Read existing content
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if entry already exists
	if len(content) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == entry {
				return nil // Already present
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}

	// Build new content
	var newContent string
	if len(content) > 0 {
		newContent = string(content)
		if content[len(content)-1] != '\n' {
			newContent += "\n"
		}
	}
	newContent += entry + "\n"

	return os.WriteFile(gitignorePath, []byte(newContent), 0644)
}

// removeFromGitignore removes an entry from .gitignore.
func removeFromGitignore(entry string) error {
	content, err := os.ReadFile(gitignorePath)
	if os.IsNotExist(err) {
		return nil // Nothing to remove
	}
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != entry {
			newLines = append(newLines, line)
		}
	}

	// Don't modify .gitignore if removing the entry would leave it empty
	newContent := strings.Join(newLines, "\n")
	if strings.TrimSpace(newContent) == "" {
		return nil
	}

	return os.WriteFile(gitignorePath, []byte(newContent), 0644)
}
