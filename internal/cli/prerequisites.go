package cli

import (
	"fmt"
	"os"
	"os/exec"
)

const rafaDir = ".rafa"

// PrerequisiteError represents a failed prerequisite check with helpful remediation info.
type PrerequisiteError struct {
	Check   string
	Message string
	Help    string
}

func (e *PrerequisiteError) Error() string {
	return fmt.Sprintf("%s: %s\n\n%s", e.Check, e.Message, e.Help)
}

// checkPrerequisites validates the environment before init and plan run commands.
func checkPrerequisites() error {
	// Check if in a git repository
	if err := checkGitRepo(); err != nil {
		return err
	}

	// Check if Claude Code CLI is installed and authenticated
	if err := checkClaudeCode(); err != nil {
		return err
	}

	return nil
}

// checkGitRepo verifies we're in a git repository.
func checkGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return &PrerequisiteError{
			Check:   "Git repository",
			Message: "Not a git repository",
			Help:    "Rafa requires a git repository. Run 'git init' first.",
		}
	}
	return nil
}

// checkClaudeCode verifies Claude Code CLI is installed and authenticated.
func checkClaudeCode() error {
	// Check if claude command exists
	_, err := exec.LookPath("claude")
	if err != nil {
		return &PrerequisiteError{
			Check:   "Claude Code CLI",
			Message: "Claude Code CLI not found",
			Help:    "Install Claude Code: https://claude.ai/code",
		}
	}

	// Check if authenticated (claude auth status returns 0 if authenticated)
	cmd := exec.Command("claude", "auth", "status")
	if err := cmd.Run(); err != nil {
		return &PrerequisiteError{
			Check:   "Claude Code authentication",
			Message: "Claude Code not authenticated",
			Help:    "Run 'claude auth' to authenticate.",
		}
	}

	return nil
}

// IsInitialized checks if rafa is initialized in the current directory.
func IsInitialized() bool {
	info, err := os.Stat(rafaDir)
	return err == nil && info.IsDir()
}

// RequireInitialized returns an error if rafa is not initialized.
func RequireInitialized() error {
	if !IsInitialized() {
		return fmt.Errorf("rafa is not initialized. Run 'rafa init' first.")
	}
	return nil
}
