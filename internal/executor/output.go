package executor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const outputLogFileName = "output.log"

// OutputCapture manages output to both terminal and log file.
type OutputCapture struct {
	logFile  *os.File
	multiOut io.Writer
	multiErr io.Writer
}

// NewOutputCapture creates an output capture for the given plan directory.
// Opens output.log in append mode to preserve history across runs.
func NewOutputCapture(planDir string) (*OutputCapture, error) {
	logPath := filepath.Join(planDir, outputLogFileName)

	// Open in append mode - preserves history when re-running failed plans
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	oc := &OutputCapture{
		logFile: f,
	}

	// Create multi-writers for stdout and stderr
	oc.multiOut = io.MultiWriter(os.Stdout, f)
	oc.multiErr = io.MultiWriter(os.Stderr, f)

	return oc, nil
}

// Stdout returns the writer for stdout.
func (oc *OutputCapture) Stdout() io.Writer {
	return oc.multiOut
}

// Stderr returns the writer for stderr.
func (oc *OutputCapture) Stderr() io.Writer {
	return oc.multiErr
}

// Close closes the log file.
func (oc *OutputCapture) Close() error {
	return oc.logFile.Close()
}

// WriteTaskHeader writes a header line to the log for a new task attempt.
func (oc *OutputCapture) WriteTaskHeader(taskID string, attempt int) {
	header := fmt.Sprintf("\n=== Task %s, Attempt %d ===\n", taskID, attempt)
	oc.logFile.WriteString(header)
	oc.logFile.WriteString(fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC3339)))
}

// WriteTaskFooter writes a footer line to the log after task completion.
func (oc *OutputCapture) WriteTaskFooter(taskID string, success bool) {
	result := "SUCCESS"
	if !success {
		result = "FAILED"
	}
	footer := fmt.Sprintf("\n=== Task %s: %s ===\n\n", taskID, result)
	oc.logFile.WriteString(footer)
}
