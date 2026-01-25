package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// MaxAttempts is the maximum number of times to retry a failed task.
const MaxAttempts = 10

// Runner defines the interface for executing tasks.
type Runner interface {
	Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int) error
}

// Executor orchestrates the execution of plan tasks.
type Executor struct {
	planDir   string
	plan      *plan.Plan
	logger    *plan.ProgressLogger
	runner    Runner
	lock      *plan.PlanLock
	startTime time.Time
}

// New creates a new Executor for the given plan directory and plan.
func New(planDir string, p *plan.Plan) *Executor {
	return &Executor{
		planDir: planDir,
		plan:    p,
		logger:  plan.NewProgressLogger(planDir),
		runner:  NewClaudeRunner(),
		lock:    plan.NewPlanLock(planDir),
	}
}

// WithRunner sets a custom runner (useful for testing).
func (e *Executor) WithRunner(r Runner) *Executor {
	e.runner = r
	return e
}

// Run executes all pending tasks in the plan.
// It acquires a lock, processes tasks sequentially, and handles retries.
func (e *Executor) Run(ctx context.Context) error {
	// Acquire lock
	if err := e.lock.Acquire(); err != nil {
		return err
	}
	defer e.lock.Release()

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

	// Update plan status if not started
	if e.plan.Status == plan.PlanStatusNotStarted {
		e.plan.Status = plan.PlanStatusInProgress
		if err := plan.SavePlan(e.planDir, e.plan); err != nil {
			return fmt.Errorf("failed to save plan: %w", err)
		}
	}

	// Log plan started and record start time
	e.startTime = time.Now()
	if err := e.logger.PlanStarted(e.plan.ID); err != nil {
		return fmt.Errorf("failed to log plan started: %w", err)
	}

	// Build plan context once
	planContext := e.buildPlanContext()

	// Execute tasks from first pending
	for i := firstIdx; i < len(e.plan.Tasks); i++ {
		task := &e.plan.Tasks[i]

		// Skip completed tasks (for resume scenarios)
		if task.Status == plan.TaskStatusCompleted {
			continue
		}

		err := e.executeTask(ctx, task, i, planContext)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled - reset task to pending
				task.Status = plan.TaskStatusPending
				if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
					fmt.Printf("Warning: failed to save plan after cancel: %v\n", saveErr)
				}
				e.logger.PlanCancelled(task.ID)
				return nil
			}

			// Max attempts reached - mark plan as failed
			e.plan.Status = plan.PlanStatusFailed
			if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
				fmt.Printf("Warning: failed to save plan after failure: %v\n", saveErr)
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

	duration := time.Since(e.startTime)
	e.logger.PlanCompleted(len(e.plan.Tasks), e.countCompleted(), duration)

	fmt.Printf("\nPlan completed! (%s)\n", e.formatDuration(duration))
	return nil
}

// executeTask runs a single task with retry logic.
func (e *Executor) executeTask(ctx context.Context, task *plan.Task, idx int, planContext string) error {
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

		// Print progress
		fmt.Printf("\nTask %d/%d: %s [Attempt %d/%d]\n",
			idx+1, len(e.plan.Tasks), task.Title, task.Attempts, MaxAttempts)

		// Log task started
		if err := e.logger.TaskStarted(task.ID, task.Attempts); err != nil {
			return fmt.Errorf("failed to log task started: %w", err)
		}

		// Run the task
		err := e.runner.Run(ctx, task, planContext, task.Attempts, MaxAttempts)
		if err == nil {
			// Task succeeded
			task.Status = plan.TaskStatusCompleted
			if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
				return fmt.Errorf("failed to save plan: %w", saveErr)
			}
			if logErr := e.logger.TaskCompleted(task.ID); logErr != nil {
				return fmt.Errorf("failed to log task completed: %w", logErr)
			}
			return nil
		}

		// Task failed
		if logErr := e.logger.TaskFailed(task.ID, task.Attempts); logErr != nil {
			fmt.Printf("Warning: failed to log task failed: %v\n", logErr)
		}
		fmt.Printf("Task failed: %v\n", err)

		// Check if max attempts reached
		if task.Attempts >= MaxAttempts {
			task.Status = plan.TaskStatusFailed
			if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
				fmt.Printf("Warning: failed to save plan: %v\n", saveErr)
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
