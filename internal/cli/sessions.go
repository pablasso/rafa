package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/pablasso/rafa/internal/session"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List all active sessions",
	Long:  `List all active conversation sessions for PRDs, designs, and plans.`,
	RunE:  runSessions,
}

func runSessions(cmd *cobra.Command, args []string) error {
	// Check if rafa is initialized
	if err := RequireInitialized(); err != nil {
		return err
	}

	// Load sessions from storage
	sessionsDir := filepath.Join(rafaDir, "sessions")
	storage := session.NewStorage(sessionsDir)

	sessions, err := storage.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active sessions.")
		return nil
	}

	// Create tabwriter for formatted output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PHASE\tNAME\tSTATUS\tUPDATED")

	for _, sess := range sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			sess.Phase,
			sess.Name,
			sess.Status,
			formatAge(sess.UpdatedAt),
		)
	}

	return w.Flush()
}

// formatAge returns a human-readable relative time string.
func formatAge(t time.Time) string {
	now := time.Now()
	duration := now.Sub(t)

	if duration < time.Minute {
		return "just now"
	}

	minutes := int(duration.Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%dm ago", minutes)
	}

	hours := int(duration.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}

	days := hours / 24
	return fmt.Sprintf("%dd ago", days)
}
