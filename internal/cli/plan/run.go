package plan

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/analysis"
	"github.com/pablasso/rafa/internal/display"
	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/spf13/cobra"
)

var runAllowDirty bool

func init() {
	runCmd.Flags().BoolVar(&runAllowDirty, "allow-dirty", false, "Allow running with uncommitted changes (not recommended)")
}

var runCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a plan (resumes from first pending task)",
	Long:  `Execute tasks from a previously created plan. Resumes from the first pending task if interrupted.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPlan,
}

func runPlan(cmd *cobra.Command, args []string) error {
	planName := args[0]

	// 1. Check .rafa/ exists
	if _, err := findRepoRoot(); err != nil {
		return fmt.Errorf("rafa not initialized. Run `rafa init` first")
	}

	// 2. Find plan folder by name suffix
	planDir, err := plan.FindPlanFolder(planName)
	if err != nil {
		return err
	}

	// 3. Load plan from JSON
	p, err := plan.LoadPlan(planDir)
	if err != nil {
		return err
	}

	// 4. Check Claude CLI availability
	if !ai.IsClaudeAvailable() {
		return fmt.Errorf("Claude Code CLI not found. Install it: https://claude.ai/code")
	}

	// 5. Create display for status line
	disp := display.New(os.Stdout)
	disp.Start()

	// 6. Create executor and run with signal handling
	ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	exec := executor.New(planDir, p).
		WithAllowDirty(runAllowDirty).
		WithDisplay(disp)

	err = exec.Run(ctx)

	// Always stop display to ensure cleanup
	disp.Stop()

	if err != nil {
		return err
	}

	// 7. Run post-completion analysis if plan completed successfully
	// Reload plan to check final status (executor updates it)
	p, reloadErr := plan.LoadPlan(planDir)
	if reloadErr != nil {
		// Log error but don't fail - analysis is non-critical
		log.Printf("Warning: failed to reload plan for analysis: %v", reloadErr)
		return nil
	}

	if p.Status == plan.PlanStatusCompleted {
		analyzer := analysis.NewAnalyzer(planDir, p)
		suggestions, analyzeErr := analyzer.Analyze()
		if analyzeErr != nil {
			// Log error but don't fail - analysis is non-critical
			log.Printf("Warning: failed to analyze plan: %v", analyzeErr)
			return nil
		}

		if len(suggestions) > 0 {
			fmt.Print(analysis.FormatSuggestions(suggestions))
		}
	}

	return nil
}
