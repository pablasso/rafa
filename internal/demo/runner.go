package demo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pablasso/rafa/internal/executor"
	"github.com/pablasso/rafa/internal/plan"
)

// Compile-time interface verification
var _ executor.Runner = (*DemoRunner)(nil)

// DemoRunner implements executor.Runner with simulated execution.
type DemoRunner struct {
	config *Config
}

// NewDemoRunner creates a new DemoRunner with the given configuration.
func NewDemoRunner(config *Config) *DemoRunner {
	return &DemoRunner{
		config: config,
	}
}

// Run implements executor.Runner interface.
// It streams realistic output and determines success/failure based on scenario.
func (d *DemoRunner) Run(
	ctx context.Context,
	task *plan.Task,
	planContext string,
	attempt, maxAttempts int,
	output executor.OutputWriter,
) error {
	// Stream realistic output
	d.streamOutput(ctx, task, attempt, output)

	// Determine success/failure based on scenario
	if d.shouldFail(task, attempt) {
		return errors.New("simulated failure: acceptance criteria not met")
	}

	return nil
}

// shouldFail determines if a task should fail based on scenario and attempt.
//
// Scenario behavior:
//   - success: Always passes (returns false)
//   - fail: Always fails (returns true)
//   - retry: Fails first 2 attempts, succeeds on 3rd (returns attempt < 3)
//   - mixed: t02 fails first attempt then succeeds (attempt < 2),
//     t03 always fails (true), others pass (false)
func (d *DemoRunner) shouldFail(task *plan.Task, attempt int) bool {
	switch d.config.Scenario {
	case ScenarioSuccess:
		return false
	case ScenarioFail:
		return true
	case ScenarioRetry:
		// Fail first 2 attempts, succeed on 3rd
		return attempt < 3
	case ScenarioMixed:
		// t02 needs retry, t03 always fails, others pass
		switch task.ID {
		case "t02":
			return attempt < 2
		case "t03":
			return true
		default:
			return false
		}
	}
	return false
}

// streamOutput writes realistic output for a task.
// The line delay is computed as TaskDelay / (number of lines + 1), with a minimum of 30ms.
// Context cancellation stops output streaming immediately.
func (d *DemoRunner) streamOutput(
	ctx context.Context,
	task *plan.Task,
	attempt int,
	output executor.OutputWriter,
) {
	lines := d.generateOutput(task, attempt)

	lineDelay := d.config.TaskDelay / time.Duration(len(lines)+1)
	if lineDelay < 30*time.Millisecond {
		lineDelay = 30 * time.Millisecond
	}

	writer := output.Stdout()
	for _, line := range lines {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Fprintln(writer, line)

		// Use select with timer to respect context cancellation during delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(lineDelay):
		}
	}
}
