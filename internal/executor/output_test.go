package executor

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOutputCapture_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write to stdout
	testMessage := "Hello from stdout\n"
	n, err := oc.Stdout().Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Write to Stdout() error: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Write returned %d, expected %d", n, len(testMessage))
	}

	// Close to ensure data is written to file
	oc.Close()

	// Read the log file
	logPath := filepath.Join(tmpDir, outputLogFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Hello from stdout") {
		t.Errorf("log file should contain 'Hello from stdout', got: %s", string(content))
	}
}

func TestOutputCapture_WritesToStdout(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// The multiOut writer writes to both os.Stdout and the log file.
	// We verify the MultiWriter works by checking the log file receives output.
	// (os.Stdout output cannot be captured without replacing the file descriptor)
	testMessage := "Test output message\n"
	oc.Stdout().Write([]byte(testMessage))
	oc.Close()

	// Read back from log file to verify multiwriter wrote to it
	logPath := filepath.Join(tmpDir, outputLogFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test output message") {
		t.Errorf("log file should contain test message, got: %s", string(content))
	}

	// Note: we also verify that Stderr returns a non-nil writer
	// in TestOutputCapture_WriterInterfaces below
}

func TestOutputCapture_TaskHeaders(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write task header
	// Truncate to second precision since RFC3339 loses nanoseconds
	beforeHeader := time.Now().Truncate(time.Second)
	oc.WriteTaskHeader("t01", 1)
	afterHeader := time.Now().Add(time.Second).Truncate(time.Second)

	// Write some content
	oc.Stdout().Write([]byte("Task output here\n"))

	// Write task footer (success)
	oc.WriteTaskFooter("t01", true)

	// Write another task header/footer (failure)
	oc.WriteTaskHeader("t02", 2)
	oc.WriteTaskFooter("t02", false)

	oc.Close()

	// Read the log file
	logPath := filepath.Join(tmpDir, outputLogFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Verify header format
	if !strings.Contains(contentStr, "=== Task t01, Attempt 1 ===") {
		t.Errorf("log should contain task header, got: %s", contentStr)
	}

	// Verify timestamp is RFC3339 and within expected range
	if !strings.Contains(contentStr, "Started:") {
		t.Error("log should contain 'Started:' timestamp")
	}

	// The timestamp should be parseable as RFC3339
	// Find the line with "Started:" and extract the timestamp
	lines := strings.Split(contentStr, "\n")
	foundValidTimestamp := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Started:") {
			timestampStr := strings.TrimPrefix(line, "Started: ")
			parsedTime, parseErr := time.Parse(time.RFC3339, timestampStr)
			if parseErr != nil {
				t.Errorf("timestamp should be RFC3339 format, got: %s, error: %v", timestampStr, parseErr)
			} else {
				if parsedTime.Before(beforeHeader) || parsedTime.After(afterHeader) {
					t.Errorf("timestamp %v should be between %v and %v", parsedTime, beforeHeader, afterHeader)
				}
				foundValidTimestamp = true
			}
			break
		}
	}
	if !foundValidTimestamp {
		t.Error("should find a valid timestamp in the log")
	}

	// Verify footer format for success
	if !strings.Contains(contentStr, "=== Task t01: SUCCESS ===") {
		t.Errorf("log should contain SUCCESS footer, got: %s", contentStr)
	}

	// Verify footer format for failure
	if !strings.Contains(contentStr, "=== Task t02: FAILED ===") {
		t.Errorf("log should contain FAILED footer, got: %s", contentStr)
	}

	// Verify second task header
	if !strings.Contains(contentStr, "=== Task t02, Attempt 2 ===") {
		t.Errorf("log should contain second task header, got: %s", contentStr)
	}
}

func TestOutputCapture_AppendsToExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, outputLogFileName)

	// Create initial content
	initialContent := "Previous run content\n"
	if err := os.WriteFile(logPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial content: %v", err)
	}

	// Create OutputCapture (should append)
	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	oc.WriteTaskHeader("t01", 1)
	oc.Close()

	// Read the log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)

	// Verify original content is preserved
	if !strings.Contains(contentStr, "Previous run content") {
		t.Errorf("log should preserve previous content, got: %s", contentStr)
	}

	// Verify new content was appended
	if !strings.Contains(contentStr, "=== Task t01, Attempt 1 ===") {
		t.Errorf("log should contain new task header, got: %s", contentStr)
	}
}

func TestOutputCapture_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}
	oc.Close()

	logPath := filepath.Join(tmpDir, outputLogFileName)
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}

	// On Unix, check the permissions
	perm := info.Mode().Perm()
	expectedPerm := os.FileMode(0644)
	if perm != expectedPerm {
		t.Errorf("expected permissions %o, got %o", expectedPerm, perm)
	}
}

func TestOutputCapture_StderrWritesToFile(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write to stderr
	testMessage := "Error message on stderr\n"
	n, err := oc.Stderr().Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Write to Stderr() error: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Write returned %d, expected %d", n, len(testMessage))
	}

	oc.Close()

	// Read the log file
	logPath := filepath.Join(tmpDir, outputLogFileName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Error message on stderr") {
		t.Errorf("log file should contain stderr message, got: %s", string(content))
	}
}

func TestNewOutputCapture_InvalidPath(t *testing.T) {
	// Try to create output capture in a non-existent directory
	oc, err := NewOutputCapture("/nonexistent/path/that/should/not/exist")
	if err == nil {
		oc.Close()
		t.Error("expected error for invalid path")
	}
}

func TestOutputCapture_Close_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// First close should succeed
	err = oc.Close()
	if err != nil {
		t.Errorf("first Close() error: %v", err)
	}

	// Second close will return an error (file already closed), which is expected
	// This tests that calling Close multiple times doesn't panic
	_ = oc.Close()
}

func TestOutputCapture_WriterInterfaces(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}
	defer oc.Close()

	// Verify Stdout returns io.Writer
	var stdoutWriter io.Writer = oc.Stdout()
	if stdoutWriter == nil {
		t.Error("Stdout() should return non-nil io.Writer")
	}

	// Verify Stderr returns io.Writer
	var stderrWriter io.Writer = oc.Stderr()
	if stderrWriter == nil {
		t.Error("Stderr() should return non-nil io.Writer")
	}
}

func TestOutputCapture_ExtractCommitMessage_Found(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write some output including a commit message
	oc.Stdout().Write([]byte("Some output\n"))
	oc.Stdout().Write([]byte("More output\n"))
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: Add new feature for user authentication\n"))
	oc.Stdout().Write([]byte("Final output\n"))

	// Need to sync the file before reading
	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	expected := "Add new feature for user authentication"
	if msg != expected {
		t.Errorf("ExtractCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}

func TestOutputCapture_ExtractCommitMessage_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write output without a commit message
	oc.Stdout().Write([]byte("Some output\n"))
	oc.Stdout().Write([]byte("More output\n"))
	oc.Stdout().Write([]byte("No commit message here\n"))

	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	if msg != "" {
		t.Errorf("ExtractCommitMessage() = %q, want empty string", msg)
	}

	oc.Close()
}

func TestOutputCapture_ExtractCommitMessage_EmptyOutput(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Don't write anything
	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	if msg != "" {
		t.Errorf("ExtractCommitMessage() = %q, want empty string", msg)
	}

	oc.Close()
}

func TestOutputCapture_ExtractCommitMessage_OnlyLast100Lines(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write a commit message that will be beyond the last 100 lines
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: Old message that should be ignored\n"))

	// Write 100 more lines to push it out of range
	for i := 0; i < 100; i++ {
		oc.Stdout().Write([]byte("Filler line\n"))
	}

	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	if msg != "" {
		t.Errorf("ExtractCommitMessage() = %q, want empty string (message should be outside last 100 lines)", msg)
	}

	oc.Close()
}

func TestOutputCapture_ExtractCommitMessage_TakesLastMessage(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write multiple commit messages - should return the last one
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: First message\n"))
	oc.Stdout().Write([]byte("Some output\n"))
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: Second message\n"))
	oc.Stdout().Write([]byte("More output\n"))
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE: Third and final message\n"))

	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	expected := "Third and final message"
	if msg != expected {
		t.Errorf("ExtractCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}

func TestOutputCapture_ExtractCommitMessage_TrimsWhitespace(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write a commit message with extra whitespace
	oc.Stdout().Write([]byte("SUGGESTED_COMMIT_MESSAGE:   Message with spaces   \n"))

	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	expected := "Message with spaces"
	if msg != expected {
		t.Errorf("ExtractCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}
