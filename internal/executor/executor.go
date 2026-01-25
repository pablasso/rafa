package executor

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pablasso/rafa/internal/display"
	"github.com/pablasso/rafa/internal/git"
	"github.com/pablasso/rafa/internal/plan"
)

// MaxAttempts is the maximum number of times to retry a failed task.
const MaxAttempts = 5

// Runner defines the interface for executing tasks.
type Runner interface {
	Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output OutputWriter) error
}

// Executor orchestrates the execution of plan tasks.
type Executor struct {
	planDir    string
	repoRoot   string
	plan       *plan.Plan
	logger     *plan.ProgressLogger
	runner     Runner
	lock       *plan.PlanLock
	startTime  time.Time
	allowDirty bool
	saveHook   func() // Optional hook called after each plan save (for testing)
	display    *display.Display
}

// New creates a new Executor for the given plan directory and plan.
func New(planDir string, p *plan.Plan) *Executor {
	// Derive repo root from planDir (.rafa/plans/<id>-<name>/ -> repo root)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(planDir)))

	return &Executor{
		planDir:  planDir,
		repoRoot: repoRoot,
		plan:     p,
		logger:   plan.NewProgressLogger(planDir),
		runner:   NewClaudeRunner(),
		lock:     plan.NewPlanLock(planDir),
	}
}

// WithRunner sets a custom runner (useful for testing).
func (e *Executor) WithRunner(r Runner) *Executor {
	e.runner = r
	return e
}

// WithAllowDirty sets whether to allow running with a dirty workspace.
func (e *Executor) WithAllowDirty(allow bool) *Executor {
	e.allowDirty = allow
	return e
}

// WithSaveHook sets an optional hook called after each plan save (for testing).
func (e *Executor) WithSaveHook(hook func()) *Executor {
	e.saveHook = hook
	return e
}

// WithDisplay sets the display for status updates.
func (e *Executor) WithDisplay(d *display.Display) *Executor {
	e.display = d
	return e
}

// notifySave calls the save hook if one is configured.
func (e *Executor) notifySave() {
	if e.saveHook != nil {
		e.saveHook()
	}
}

// Run executes all pending tasks in the plan.
// It acquires a lock, processes tasks sequentially, and handles retries.
func (e *Executor) Run(ctx context.Context) error {
	// Acquire lock
	if err := e.lock.Acquire(); err != nil {
		return err
	}
	defer e.lock.Release()

	// Check workspace cleanliness before starting (excluding our lock file)
	if !e.allowDirty {
		status, err := git.GetStatus(e.repoRoot)
		if err != nil {
			return fmt.Errorf("failed to check git status: %w", err)
		}
		// Filter out our lock file from the dirty files list
		dirtyFiles := e.filterOutLockFile(status.Files)
		if len(dirtyFiles) > 0 {
			return e.workspaceDirtyError(dirtyFiles)
		}
	}

	// Check if all tasks are already completed
	if e.plan.AllTasksCompleted() {
		fmt.Println("All tasks already completed.")
		return nil
	}

	// Find first pending task
	firstIdx := e.plan.FirstPendingTask()
	if firstIdx == -1 {
		fmt.Println("No pending tasks found.")
		return nil
	}

	// If re-running a failed plan, reset attempts on the blocking task
	if e.plan.Status == plan.PlanStatusFailed {
		task := &e.plan.Tasks[firstIdx]
		if task.Attempts >= MaxAttempts {
			task.Attempts = 0
			task.Status = plan.TaskStatusPending
		}
	}

	// Update plan status if not started
	if e.plan.Status == plan.PlanStatusNotStarted {
		e.plan.Status = plan.PlanStatusInProgress
		if err := plan.SavePlan(e.planDir, e.plan); err != nil {
			return fmt.Errorf("failed to save plan: %w", err)
		}
		e.notifySave()
	}

	// Log plan started and record start time
	e.startTime = time.Now()
	if err := e.logger.PlanStarted(e.plan.ID); err != nil {
		return fmt.Errorf("failed to log plan started: %w", err)
	}

	// Build plan context once
	planContext := e.buildPlanContext()

	// Create output capture for logging
	output, err := NewOutputCapture(e.planDir)
	if err != nil {
		// Output capture is non-critical, log warning and continue
		fmt.Printf("Warning: failed to create output capture: %v\n", err)
		output = nil
	}
	if output != nil {
		defer output.Close()
	}

	// Execute tasks from first pending
	for i := firstIdx; i < len(e.plan.Tasks); i++ {
		task := &e.plan.Tasks[i]

		// Skip completed tasks (for resume scenarios)
		if task.Status == plan.TaskStatusCompleted {
			continue
		}

		err := e.executeTask(ctx, task, i, planContext, output)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled - reset task to pending
				task.Status = plan.TaskStatusPending
				if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
					fmt.Printf("Warning: failed to save plan after cancel: %v\n", saveErr)
				} else {
					e.notifySave()
				}
				e.logger.PlanCancelled(task.ID)
				if e.display != nil {
					e.display.UpdateStatus(display.StatusCancelled)
				}
				return nil
			}

			// Max attempts reached - mark plan as failed
			e.plan.Status = plan.PlanStatusFailed
			if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
				fmt.Printf("Warning: failed to save plan after failure: %v\n", saveErr)
			} else {
				e.notifySave()
			}
			e.logger.PlanFailed(task.ID, task.Attempts)
			return fmt.Errorf("task %s failed after %d attempts", task.ID, task.Attempts)
		}
	}

	// All tasks completed
	e.plan.Status = plan.PlanStatusCompleted
	if err := plan.SavePlan(e.planDir, e.plan); err != nil {
		return fmt.Errorf("failed to save plan: %w", err)
	}
	e.notifySave()

	duration := time.Since(e.startTime)
	e.logger.PlanCompleted(len(e.plan.Tasks), e.countCompleted(), duration)

	// Commit any remaining metadata (plan completion status)
	// CommitAll returns nil when there's nothing to commit (e.g., agent already committed)
	// We only warn on error since the agent might have already committed everything.
	if !e.allowDirty {
		msg := fmt.Sprintf("[rafa] Complete plan: %s (%d tasks)", e.plan.Name, len(e.plan.Tasks))
		if err := git.CommitAll(e.repoRoot, msg); err != nil {
			fmt.Printf("Warning: failed to commit plan completion: %v\n", err)
		}
	}

	fmt.Printf("\nPlan completed! (%s)\n", e.formatDuration(duration))
	return nil
}

// executeTask runs a single task with retry logic.
func (e *Executor) executeTask(ctx context.Context, task *plan.Task, idx int, planContext string, output *OutputCapture) error {
	// Update display with task info at the start
	if e.display != nil {
		e.display.UpdateTask(idx+1, len(e.plan.Tasks), task.ID, task.Title)
		e.display.UpdateStatus(display.StatusRunning)
	}

	for task.Attempts < MaxAttempts {
		// Check for cancellation before starting
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Increment attempts and set in_progress
		task.Attempts++
		task.Status = plan.TaskStatusInProgress
		if err := plan.SavePlan(e.planDir, e.plan); err != nil {
			return fmt.Errorf("failed to save plan: %w", err)
		}
		e.notifySave()

		// Update display with attempt info
		if e.display != nil {
			e.display.UpdateAttempt(task.Attempts, MaxAttempts)
		}

		// Print progress
		fmt.Printf("\nTask %d/%d: %s [Attempt %d/%d]\n",
			idx+1, len(e.plan.Tasks), task.Title, task.Attempts, MaxAttempts)

		// Log task started
		if err := e.logger.TaskStarted(task.ID, task.Attempts); err != nil {
			return fmt.Errorf("failed to log task started: %w", err)
		}

		// Write task header to output log
		if output != nil {
			output.WriteTaskHeader(task.ID, task.Attempts)
		}

		// Run the task
		err := e.runner.Run(ctx, task, planContext, task.Attempts, MaxAttempts, output)
		if err == nil {
			// Task succeeded - update metadata and commit everything
			task.Status = plan.TaskStatusCompleted
			if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
				return fmt.Errorf("failed to save plan: %w", saveErr)
			}
			e.notifySave()
			if logErr := e.logger.TaskCompleted(task.ID); logErr != nil {
				return fmt.Errorf("failed to log task completed: %w", logErr)
			}

			// Commit all changes (implementation + metadata) unless allowDirty
			if !e.allowDirty {
				commitMsg := e.getCommitMessage(task, output)
				if commitErr := git.CommitAll(e.repoRoot, commitMsg); commitErr != nil {
					return fmt.Errorf("failed to commit: %w", commitErr)
				}

				// Verify workspace is clean after commit (catches git hooks, etc.)
				status, checkErr := git.GetStatus(e.repoRoot)
				if checkErr != nil {
					return fmt.Errorf("failed to check git status after commit: %w", checkErr)
				}
				if !status.Clean {
					return fmt.Errorf("workspace not clean after commit (possibly git hooks modified files): %v", status.Files)
				}
			}

			if output != nil {
				output.WriteTaskFooter(task.ID, true)
			}
			if e.display != nil {
				e.display.UpdateStatus(display.StatusCompleted)
			}
			return nil
		}

		// Task failed
		if logErr := e.logger.TaskFailed(task.ID, task.Attempts); logErr != nil {
			fmt.Printf("Warning: failed to log task failed: %v\n", logErr)
		}
		fmt.Printf("Task failed: %v\n", err)
		if output != nil {
			output.WriteTaskFooter(task.ID, false)
		}

		// Check if max attempts reached
		if task.Attempts >= MaxAttempts {
			task.Status = plan.TaskStatusFailed
			if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
				fmt.Printf("Warning: failed to save plan: %v\n", saveErr)
			} else {
				e.notifySave()
			}
			if e.display != nil {
				e.display.UpdateStatus(display.StatusFailed)
			}
			return fmt.Errorf("max attempts reached")
		}

		// Check for cancellation before retrying
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fmt.Println("Spinning up fresh agent for retry...")
	}

	return fmt.Errorf("max attempts reached")
}

// buildPlanContext returns a context string describing the plan.
func (e *Executor) buildPlanContext() string {
	return fmt.Sprintf("Plan: %s\nDescription: %s\nSource: %s",
		e.plan.Name, e.plan.Description, e.plan.SourceFile)
}

// countCompleted returns the number of completed tasks.
func (e *Executor) countCompleted() int {
	count := 0
	for i := range e.plan.Tasks {
		if e.plan.Tasks[i].Status == plan.TaskStatusCompleted {
			count++
		}
	}
	return count
}

// formatDuration formats a duration as HH:MM:SS or MM:SS.
func (e *Executor) formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// workspaceDirtyError returns a formatted error for dirty workspace.
func (e *Executor) workspaceDirtyError(files []string) error {
	msg := "workspace has uncommitted changes before starting plan\n\nModified files:\n"
	for _, f := range files {
		msg += fmt.Sprintf("  %s\n", f)
	}
	msg += "\nPlease commit or stash your changes before running the plan.\n"
	msg += "Or use --allow-dirty to skip this check (not recommended)."
	return fmt.Errorf("%s", msg)
}

// filterOutLockFile removes the run.lock file from a list of dirty files.
// This is needed because the lock is created before we check workspace cleanliness.
func (e *Executor) filterOutLockFile(files []string) []string {
	lockPath := ".rafa/plans/" + filepath.Base(e.planDir) + "/run.lock"
	var filtered []string
	for _, f := range files {
		if f != lockPath {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// getCommitMessage extracts the agent's suggested commit message from OutputCapture,
// or falls back to a default message format '[rafa] Complete task <id>: <title>'.
// The [rafa] prefix enables easy filtering in git log.
func (e *Executor) getCommitMessage(task *plan.Task, output *OutputCapture) string {
	if output != nil {
		if msg := output.ExtractCommitMessage(); msg != "" {
			return msg
		}
	}
	return fmt.Sprintf("[rafa] Complete task %s: %s", task.ID, task.Title)
}
