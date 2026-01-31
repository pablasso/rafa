package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pablasso/rafa/internal/session"
	"github.com/pablasso/rafa/internal/tui"
	"github.com/spf13/cobra"
)

var (
	designName string
	designFrom string
)

var designCmd = &cobra.Command{
	Use:   "design",
	Short: "Create a technical design document",
	Long:  `Start an interactive session to create a technical design with AI guidance.`,
	RunE:  runDesign,
}

var designResumeCmd = &cobra.Command{
	Use:   "resume [name]",
	Short: "Resume an existing design session",
	Long:  `Resume an in-progress design session. If no name is provided, resumes the most recent session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDesignResume,
}

func init() {
	designCmd.Flags().StringVar(&designName, "name", "", "Name for the design (optional)")
	designCmd.Flags().StringVar(&designFrom, "from", "", "Path to source PRD document (optional)")
	designCmd.AddCommand(designResumeCmd)
}

func runDesign(cmd *cobra.Command, args []string) error {
	// Check if rafa is initialized
	if err := RequireInitialized(); err != nil {
		return err
	}

	// Check if skills are installed
	if !skillsInstalled() {
		return fmt.Errorf("skills not installed. Run 'rafa init' to install them.")
	}

	// Validate --from path if provided
	if designFrom != "" {
		if _, err := os.Stat(designFrom); os.IsNotExist(err) {
			return fmt.Errorf("source PRD path does not exist: %s", designFrom)
		}
	}

	// Launch TUI in conversation mode
	return tui.RunWithConversation(tui.ConversationOpts{
		Phase:   session.PhaseDesign,
		Name:    designName,
		FromDoc: designFrom,
	})
}

func runDesignResume(cmd *cobra.Command, args []string) error {
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
		sess, err = storage.Load(session.PhaseDesign, args[0])
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no design session named %q found. Start a new design with: rafa design", args[0])
			}
			return fmt.Errorf("failed to load session: %w", err)
		}
	} else {
		// Load most recent session for this phase
		sess, err = storage.LoadByPhase(session.PhaseDesign)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no design sessions found. Start a new design with: rafa design")
			}
			return fmt.Errorf("failed to load session: %w", err)
		}
	}

	// Check if session is already completed
	if sess.Status == session.StatusCompleted {
		return fmt.Errorf("session %q is already completed. Start a new design with: rafa design --name <new-name>", sess.Name)
	}

	// Check if session is cancelled
	if sess.Status == session.StatusCancelled {
		return fmt.Errorf("session %q was cancelled. Start a new design with: rafa design --name <new-name>", sess.Name)
	}

	// Launch TUI with session resume
	return tui.RunWithConversation(tui.ConversationOpts{
		Phase:      session.PhaseDesign,
		Name:       sess.Name,
		ResumeFrom: sess,
	})
}
