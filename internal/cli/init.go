package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablasso/rafa/internal/skills"
	"github.com/spf13/cobra"
)

const (
	gitignoreLockEntry     = ".rafa/**/*.lock"
	gitignoreSessionsEntry = ".rafa/sessions/"
	gitignorePath          = ".gitignore"
	claudeSkillsDir        = ".claude/skills"
)

// SkillsInstaller is an interface for installing skills, allowing mocking in tests.
type SkillsInstaller interface {
	Install() error
	Uninstall() error
}

// skillsInstallerFactory creates a skills installer for a given target directory.
// This is a package-level variable that can be overridden in tests.
var skillsInstallerFactory = func(targetDir string) SkillsInstaller {
	return skills.NewInstaller(targetDir)
}

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

	// Track what's been created for cleanup on failure
	var installer SkillsInstaller
	success := false
	defer func() {
		if !success {
			// Clean up all partial state on failure
			os.RemoveAll(rafaDir)
			if installer != nil {
				installer.Uninstall()
			}
			removeFromGitignore(gitignoreLockEntry)
			removeFromGitignore(gitignoreSessionsEntry)
		}
	}()

	// Create .rafa directory structure
	dirs := []string{
		rafaDir,
		filepath.Join(rafaDir, "plans"),
		filepath.Join(rafaDir, "sessions"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Install skills from GitHub
	fmt.Print("Installing skills from github.com/pablasso/skills... ")
	installer = skillsInstallerFactory(claudeSkillsDir)
	if err := installer.Install(); err != nil {
		fmt.Println("failed")
		return fmt.Errorf("failed to install skills: %w", err)
	}
	fmt.Println("done")

	// Add gitignore entries
	if err := addToGitignore(gitignoreLockEntry); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}
	if err := addToGitignore(gitignoreSessionsEntry); err != nil {
		return fmt.Errorf("failed to update .gitignore: %w", err)
	}

	// Mark success to prevent cleanup
	success = true

	fmt.Println("Initialized Rafa in", rafaDir)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run: rafa prd             # Create a PRD")
	fmt.Println("  2. Run: rafa design          # Create a technical design")
	fmt.Println("  3. Run: rafa plan create     # Create an execution plan")
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
