package cli

import (
	"fmt"
	"os"

	"github.com/pablasso/rafa/internal/demo"
	"github.com/pablasso/rafa/internal/tui"
	"github.com/spf13/cobra"
)

var (
	demoScenario string
	demoSpeed    string
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Launch TUI in demo mode with simulated execution",
	Long: `Launch the TUI with simulated plan execution for testing and demonstration.

No Claude authentication required. Useful for:
  - Iterating on TUI changes quickly
  - Demonstrating rafa to others
  - Testing different UI states

Scenarios:
  success  All tasks pass (default)
  mixed    Some pass, some fail, some need retry
  fail     All tasks fail after max retries
  retry    Tasks fail initially, succeed on retry

Speeds:
  fast     500ms per task (quick iteration)
  normal   2s per task (default)
  slow     5s per task (realistic demo)`,
	Run: runDemo,
}

func init() {
	demoCmd.Flags().StringVar(&demoScenario, "scenario", "success",
		"Demo scenario: success, mixed, fail, retry")
	demoCmd.Flags().StringVar(&demoSpeed, "speed", "normal",
		"Execution speed: fast, normal, slow")
}

func runDemo(cmd *cobra.Command, args []string) {
	scenario := demo.Scenario(demoScenario)
	speed := demo.Speed(demoSpeed)

	// Validate scenario
	switch scenario {
	case demo.ScenarioSuccess, demo.ScenarioMixed,
		demo.ScenarioFail, demo.ScenarioRetry:
		// valid
	default:
		fmt.Fprintf(os.Stderr, "Invalid scenario: %s\n", demoScenario)
		os.Exit(1)
	}

	// Validate speed
	switch speed {
	case demo.SpeedFast, demo.SpeedNormal, demo.SpeedSlow:
		// valid
	default:
		fmt.Fprintf(os.Stderr, "Invalid speed: %s\n", demoSpeed)
		os.Exit(1)
	}

	config := demo.NewConfig(scenario, speed)

	if err := tui.Run(tui.WithDemoMode(config)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
