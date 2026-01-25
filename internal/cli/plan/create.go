package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pablasso/rafa/internal/ai"
	"github.com/pablasso/rafa/internal/plan"
	"github.com/pablasso/rafa/internal/util"
	"github.com/spf13/cobra"
)

var (
	createName   string
	createDryRun bool
)

// CreateOptions holds the options for the create command.
type CreateOptions struct {
	FilePath string
	Name     string
	DryRun   bool
}

var createCmd = &cobra.Command{
	Use:   "create <file>",
	Short: "Create a plan from a technical design or PRD",
	Long:  `Create a plan by converting a technical design or PRD markdown file into an executable JSON plan with discrete tasks and acceptance criteria.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := CreateOptions{
			FilePath: args[0],
			Name:     createName,
			DryRun:   createDryRun,
		}

		// 1. Validate inputs
		if err := validateInputs(opts); err != nil {
			return err
		}

		// 2. Read design file
		content, err := os.ReadFile(opts.FilePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		fmt.Printf("Creating plan from: %s\n", opts.FilePath)
		fmt.Println("Extracting tasks...")

		// 3. Extract tasks via Claude CLI
		ctx := context.Background()
		extracted, err := ai.ExtractTasks(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to extract tasks: %w", err)
		}

		// 4. Assemble the plan
		p, err := assemblePlan(opts, extracted)
		if err != nil {
			return err
		}

		// 5. Handle dry-run vs actual creation
		if opts.DryRun {
			printDryRunPreview(p)
			return nil
		}

		// 6. Create plan folder
		if err := plan.CreatePlanFolder(p); err != nil {
			return err
		}

		// 7. Print success message
		printSuccess(p)
		return nil
	},
}

func init() {
	createCmd.Flags().StringVar(&createName, "name", "", "Name for the plan")
	createCmd.Flags().BoolVar(&createDryRun, "dry-run", false, "Preview changes without applying them")
}

// validateInputs checks that all inputs are valid before proceeding.
func validateInputs(opts CreateOptions) error {
	// Check .rafa/ exists (skip for dry-run)
	if !opts.DryRun {
		if _, err := findRepoRoot(); err != nil {
			return fmt.Errorf("rafa not initialized. Run `rafa init` first")
		}
	}

	// Verify file exists
	info, err := os.Stat(opts.FilePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", opts.FilePath)
	}
	if err != nil {
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Verify file has .md extension
	if !strings.HasSuffix(strings.ToLower(opts.FilePath), ".md") {
		return fmt.Errorf("file must be markdown (.md): %s", opts.FilePath)
	}

	// Verify file is not empty
	if info.Size() == 0 {
		return fmt.Errorf("design document is empty: %s", opts.FilePath)
	}

	return nil
}

// findRepoRoot walks up directories looking for .rafa/ folder.
// Returns the directory containing .rafa/.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		rafaPath := filepath.Join(dir, ".rafa")
		if info, err := os.Stat(rafaPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", fmt.Errorf(".rafa directory not found")
		}
		dir = parent
	}
}

// normalizeSourcePath converts an absolute path to relative from repo root.
// Falls back to the original path if conversion fails.
func normalizeSourcePath(filePath string) string {
	// Make the path absolute first
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filePath
	}

	// Find repo root
	repoRoot, err := findRepoRoot()
	if err != nil {
		return filePath
	}

	// Try to make it relative
	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return filePath
	}

	return relPath
}

// determinePlanBaseName selects the base plan name based on priority:
// --name flag > AI-extracted name > filename without extension.
// The result is normalized to kebab-case but not yet checked for collisions.
func determinePlanBaseName(opts CreateOptions, extracted *plan.TaskExtractionResult) string {
	if opts.Name != "" {
		return util.ToKebabCase(opts.Name)
	}

	if extracted.Name != "" {
		return util.ToKebabCase(extracted.Name)
	}

	// Use filename without extension
	base := filepath.Base(opts.FilePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return util.ToKebabCase(name)
}

// assemblePlan creates a Plan from the extraction result.
func assemblePlan(opts CreateOptions, extracted *plan.TaskExtractionResult) (*plan.Plan, error) {
	// Generate plan ID
	id, err := util.GenerateShortID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan ID: %w", err)
	}

	// Resolve plan name
	baseName := determinePlanBaseName(opts, extracted)
	name, err := plan.ResolvePlanName(baseName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plan name: %w", err)
	}

	// Convert ExtractedTask to Task with sequential IDs
	tasks := make([]plan.Task, len(extracted.Tasks))
	for i, et := range extracted.Tasks {
		tasks[i] = plan.Task{
			ID:                 util.GenerateTaskID(i),
			Title:              et.Title,
			Description:        et.Description,
			AcceptanceCriteria: et.AcceptanceCriteria,
			Status:             plan.TaskStatusPending,
			Attempts:           0,
		}
	}

	// Normalize source path
	sourcePath := normalizeSourcePath(opts.FilePath)

	return &plan.Plan{
		ID:          id,
		Name:        name,
		Description: extracted.Description,
		SourceFile:  sourcePath,
		CreatedAt:   time.Now(),
		Status:      plan.PlanStatusNotStarted,
		Tasks:       tasks,
	}, nil
}

// printDryRunPreview displays the plan preview without saving.
func printDryRunPreview(p *plan.Plan) {
	fmt.Println()
	fmt.Println("Plan preview (dry run - nothing saved):")
	fmt.Println()
	fmt.Printf("  Name: %s\n", p.Name)
	fmt.Printf("  Source: %s\n", p.SourceFile)
	fmt.Printf("  Tasks: %d\n", len(p.Tasks))
	fmt.Println()

	for _, task := range p.Tasks {
		fmt.Printf("  %s: %s\n", task.ID, task.Title)
		for _, ac := range task.AcceptanceCriteria {
			fmt.Printf("       - %s\n", ac)
		}
	}

	fmt.Println()
	fmt.Println("To create this plan, run without --dry-run.")
}

// printSuccess displays the success message after plan creation.
func printSuccess(p *plan.Plan) {
	fmt.Println()
	fmt.Printf("Plan created: %s-%s\n", p.ID, p.Name)
	fmt.Println()
	fmt.Printf("  %d tasks extracted:\n", len(p.Tasks))
	fmt.Println()

	for _, task := range p.Tasks {
		fmt.Printf("  %s: %s\n", task.ID, task.Title)
	}

	fmt.Println()
	fmt.Printf("Run `rafa plan run %s` to start execution.\n", p.Name)
}
