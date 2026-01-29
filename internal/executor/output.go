package executor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const outputLogFileName = "output.log"

// OutputWriter provides writers for capturing command output.
// This interface is used by Runner to allow for different output strategies.
type OutputWriter interface {
	Stdout() io.Writer
	Stderr() io.Writer
}

// OutputCapture manages output to both terminal and log file.
type OutputCapture struct {
	logFile    *os.File
	multiOut   io.Writer
	multiErr   io.Writer
	eventsChan chan string // For TUI consumption, nil for CLI
}

// NewOutputCapture creates an output capture for the given plan directory.
// Opens output.log in append mode to preserve history across runs.
func NewOutputCapture(planDir string) (*OutputCapture, error) {
	return NewOutputCaptureWithEvents(planDir, nil)
}

// NewOutputCaptureWithEvents creates an output capture with optional event streaming.
// When eventsChan is non-nil, output is streamed to the channel for TUI integration.
// The channel should be buffered to avoid blocking; if the buffer is full, data is dropped.
func NewOutputCaptureWithEvents(planDir string, eventsChan chan string) (*OutputCapture, error) {
	logPath := filepath.Join(planDir, outputLogFileName)

	// Open in append mode - preserves history when re-running failed plans
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	oc := &OutputCapture{
		logFile:    f,
		eventsChan: eventsChan,
	}

	// Create multi-writers for stdout and stderr
	// When eventsChan is set, use streamingWriter for TUI integration
	// In TUI mode, we only write to the log file and stream to the channel
	// (not to stdout/stderr, which would corrupt the TUI display)
	if eventsChan != nil {
		streamingOut := &streamingWriter{
			underlying: f, // Only write to log file in TUI mode
			eventsChan: eventsChan,
		}
		streamingErr := &streamingWriter{
			underlying: f, // Only write to log file in TUI mode
			eventsChan: eventsChan,
		}
		oc.multiOut = streamingOut
		oc.multiErr = streamingErr
	} else {
		oc.multiOut = io.MultiWriter(os.Stdout, f)
		oc.multiErr = io.MultiWriter(os.Stderr, f)
	}

	return oc, nil
}

// streamingWriter wraps a writer and sends output to a channel for TUI streaming.
type streamingWriter struct {
	underlying io.Writer
	eventsChan chan string
}

// Write writes to the underlying writer and sends to eventsChan with non-blocking select.
func (s *streamingWriter) Write(p []byte) (n int, err error) {
	// Write to underlying writer (log file + terminal)
	n, err = s.underlying.Write(p)

	// Send to TUI if channel exists (non-blocking)
	if s.eventsChan != nil {
		select {
		case s.eventsChan <- string(p):
		default:
			// Drop if buffer full, don't block execution
		}
	}
	return
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

// EventsChan returns the events channel for TUI streaming, or nil for CLI mode.
func (oc *OutputCapture) EventsChan() chan string {
	return oc.eventsChan
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

const (
	commitMessagePrefix = "SUGGESTED_COMMIT_MESSAGE:"
	maxLinesToSearch    = 100
)

// ExtractCommitMessage searches the captured output for a suggested commit message.
// It looks for a line starting with 'SUGGESTED_COMMIT_MESSAGE:' in the last 100 lines
// of output for efficiency. Returns the trimmed message after the prefix, or empty
// string if no message is found or on read error. If multiple messages exist within
// the last 100 lines, returns the most recent one. Callers should ensure the log
// file is synced (via Sync() or close/reopen) before calling if recent writes need
// to be included.
func (oc *OutputCapture) ExtractCommitMessage() string {
	// Get the log file path from the open file
	logPath := oc.logFile.Name()

	// Open for reading (the file is opened write-only, so we need a separate read handle)
	f, err := os.Open(logPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	// Read all lines into a slice
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if scanner.Err() != nil {
		return ""
	}

	// Determine start index for searching (last 100 lines)
	start := 0
	if len(lines) > maxLinesToSearch {
		start = len(lines) - maxLinesToSearch
	}

	// Search from most recent lines first (reverse order)
	for i := len(lines) - 1; i >= start; i-- {
		line := lines[i]
		if strings.HasPrefix(line, commitMessagePrefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, commitMessagePrefix))
		}
	}

	return ""
}
