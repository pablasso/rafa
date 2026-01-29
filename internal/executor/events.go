package executor

import (
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

// ExecutorEvents receives callbacks during plan execution.
// Implement this interface in the TUI to receive updates.
type ExecutorEvents interface {
	// OnTaskStart is called when a task begins execution
	OnTaskStart(taskNum, total int, task *plan.Task, attempt int)

	// OnTaskComplete is called when a task succeeds
	OnTaskComplete(task *plan.Task)

	// OnTaskFailed is called when a task attempt fails
	OnTaskFailed(task *plan.Task, attempt int, err error)

	// OnOutput is called for each line of output from the AI
	OnOutput(line string)

	// OnPlanComplete is called when all tasks finish successfully
	OnPlanComplete(succeeded, total int, duration time.Duration)

	// OnPlanFailed is called when a task exhausts retries
	OnPlanFailed(task *plan.Task, reason string)
}
