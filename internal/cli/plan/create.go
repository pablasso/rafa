package plan

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	createName   string
	createDryRun bool
)

var createCmd = &cobra.Command{
	Use:   "create <file>",
	Short: "Create a plan from a technical design or PRD",
	Long:  `Create a plan by converting a technical design or PRD markdown file into an executable JSON plan with discrete tasks and acceptance criteria.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("not implemented yet")
		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createName, "name", "", "Name for the meal plan")
	createCmd.Flags().BoolVar(&createDryRun, "dry-run", false, "Preview changes without applying them")
}
