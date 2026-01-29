package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/pablasso/rafa/internal/demo"
	"github.com/pablasso/rafa/internal/tui"
	"github.com/spf13/cobra"
)

var (
	demoScenario  string
	demoSpeed     string
	demoTaskDelay string
	demoTaskCount int
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

Speed Presets (sets both delay and task count):
  fast       500ms delay,  5 tasks   (~2.5s total)
  normal     10s delay,   18 tasks   (~3 min total, default)
  slow       30s delay,   60 tasks   (~30 min total)
  marathon    1m delay,  120 tasks   (~2 hrs total)
  extended    2m delay,  360 tasks   (~12 hrs total)

Override Flags:
  --tasks       Override task count from speed preset
  --task-delay  Override delay from speed preset (e.g., "30s", "1m", "2m30s")

Examples:
  rafa demo                              # normal: 18 tasks, 10s each
  rafa demo --speed=marathon             # 120 tasks, 1m each (~2 hrs)
  rafa demo --speed=marathon --tasks=60  # 60 tasks, 1m each (~1 hr)
  rafa demo --task-delay=3m              # 18 tasks, 3m each`,
	Run: runDemo,
}

func init() {
	demoCmd.Flags().StringVar(&demoScenario, "scenario", "success",
		"Demo scenario: success, mixed, fail, retry")
	demoCmd.Flags().StringVar(&demoSpeed, "speed", "normal",
		"Speed preset: fast, normal, slow, marathon, extended")
	demoCmd.Flags().StringVar(&demoTaskDelay, "task-delay", "",
		"Override delay per task (e.g., '30s', '1m', '2m30s')")
	demoCmd.Flags().IntVar(&demoTaskCount, "tasks", 0,
		"Override number of tasks (0 = use speed preset)")
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
	case demo.SpeedFast, demo.SpeedNormal, demo.SpeedSlow,
		demo.SpeedMarathon, demo.SpeedExtended:
		// valid
	default:
		fmt.Fprintf(os.Stderr, "Invalid speed: %s\n", demoSpeed)
		os.Exit(1)
	}

	// Parse task delay override
	var taskDelay time.Duration
	if demoTaskDelay != "" {
		var err error
		taskDelay, err = time.ParseDuration(demoTaskDelay)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid task-delay: %s\n", demoTaskDelay)
			os.Exit(1)
		}
	}

	config := demo.NewConfigWithOptions(scenario, speed, taskDelay, demoTaskCount)

	if err := tui.Run(tui.WithDemoMode(config)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
