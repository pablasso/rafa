package cli

import (
	"github.com/pablasso/rafa/internal/cli/plan"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "rafa",
	Short:   "Task loop runner for AI coding agents",
	Long:    `Rafa runs AI coding agents in a loop until each task succeeds. One agent, one task, one loop at a time.`,
	Version: "0.1.0",
}

func init() {
	rootCmd.AddCommand(plan.PlanCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
