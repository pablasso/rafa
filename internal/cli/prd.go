package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablasso/rafa/internal/session"
	"github.com/pablasso/rafa/internal/skills"
	"github.com/pablasso/rafa/internal/tui"
	"github.com/spf13/cobra"
)

var prdName string

var prdCmd = &cobra.Command{
	Use:   "prd",
	Short: "Create a PRD (Product Requirements Document)",
	Long:  `Start an interactive session to create a PRD with AI guidance.`,
	RunE:  runPrd,
}

var prdResumeCmd = &cobra.Command{
	Use:   "resume [name]",
	Short: "Resume an existing PRD session",
	Long:  `Resume an in-progress PRD session. If no name is provided, resumes the most recent session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPrdResume,
}

func init() {
	prdCmd.Flags().StringVar(&prdName, "name", "", "Name for the PRD (optional)")
	prdCmd.AddCommand(prdResumeCmd)
}

// skillsInstalled checks if the required skills are installed.
func skillsInstalled() bool {
	installer := skills.NewInstaller(claudeSkillsDir)
	return installer.IsInstalled()
}

func runPrd(cmd *cobra.Command, args []string) error {
	// Check if rafa is initialized
	if err := RequireInitialized(); err != nil {
		return err
	}

	// Check if skills are installed
	if !skillsInstalled() {
		return fmt.Errorf("skills not installed. Run 'rafa init' to install them.")
	}

	// Launch TUI in conversation mode
	return tui.RunWithConversation(tui.ConversationOpts{
		Phase: session.PhasePRD,
		Name:  prdName,
	})
}

func runPrdResume(cmd *cobra.Command, args []string) error {
	// Check if rafa is initialized
	if err := RequireInitialized(); err != nil {
		return err
	}

	// Check if skills are installed
	if !skillsInstalled() {
		return fmt.Errorf("skills not installed. Run 'rafa init' to install them.")
	}

	// Load session from storage
	sessionsDir := filepath.Join(rafaDir, "sessions")
	storage := session.NewStorage(sessionsDir)

	var sess *session.Session
	var err error

	if len(args) > 0 {
		// Load specific named session
		sess, err = storage.Load(session.PhasePRD, args[0])
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no PRD session named %q found. Start a new PRD with: rafa prd", args[0])
			}
			return fmt.Errorf("failed to load session: %w", err)
		}
	} else {
		// Load most recent session for this phase
		sess, err = storage.LoadByPhase(session.PhasePRD)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no PRD sessions found. Start a new PRD with: rafa prd")
			}
			return fmt.Errorf("failed to load session: %w", err)
		}
	}

	// Check if session is already completed
	if sess.Status == session.StatusCompleted {
		return fmt.Errorf("session %q is already completed. Start a new PRD with: rafa prd --name <new-name>", sess.Name)
	}

	// Check if session is cancelled
	if sess.Status == session.StatusCancelled {
		return fmt.Errorf("session %q was cancelled. Start a new PRD with: rafa prd --name <new-name>", sess.Name)
	}

	// Launch TUI with session resume
	return tui.RunWithConversation(tui.ConversationOpts{
		Phase:      session.PhasePRD,
		Name:       sess.Name,
		ResumeFrom: sess,
	})
}
