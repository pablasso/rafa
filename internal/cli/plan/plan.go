package plan

import (
	"github.com/spf13/cobra"
)

// PlanCmd is the parent command for plan-related subcommands.
var PlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Manage and execute plans",
	Long:  `Commands for creating and executing task plans from technical designs.`,
}

func init() {
	PlanCmd.AddCommand(createCmd)
}
