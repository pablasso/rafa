package cli

import (
	"fmt"

	"github.com/pablasso/rafa/internal/cli/plan"
	"github.com/pablasso/rafa/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "rafa",
	Short:   "Task loop runner for AI coding agents",
	Long:    `Rafa runs AI coding agents in a loop until each task succeeds. One agent, one task, one loop at a time.`,
	Version: version.Version,
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(deinitCmd)
	rootCmd.AddCommand(plan.PlanCmd)
	rootCmd.AddCommand(demoCmd)

	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"rafa version %s\ncommit: %s\nbuilt: %s\n",
		version.Version,
		version.CommitSHA,
		version.BuildDate,
	))
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
