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

func TestOutputCapture_WithEventsChan_StreamsOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a buffered channel for events
	eventsChan := make(chan string, 10)

	oc, err := NewOutputCaptureWithEvents(tmpDir, eventsChan)
	if err != nil {
		t.Fatalf("NewOutputCaptureWithEvents() error: %v", err)
	}

	// Write to stdout - with JSON parsing enabled, plain text lines
	// are passed through with the newline stripped (line buffering)
	testMessage := "Hello from stdout\n"
	oc.Stdout().Write([]byte(testMessage))

	oc.Close()

	// Check the channel received the message (newline stripped due to line buffering)
	select {
	case msg := <-eventsChan:
		expected := "Hello from stdout"
		if msg != expected {
			t.Errorf("eventsChan received %q, want %q", msg, expected)
		}
	default:
		t.Error("eventsChan should have received a message")
	}
}

func TestOutputCapture_WithEventsChan_NonBlockingWrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a channel with no buffer - would block if we try to write
	eventsChan := make(chan string)

	oc, err := NewOutputCaptureWithEvents(tmpDir, eventsChan)
	if err != nil {
		t.Fatalf("NewOutputCaptureWithEvents() error: %v", err)
	}

	// Write should succeed even though no one is reading the channel
	// (non-blocking, should drop the message)
	testMessage := "This should not block\n"
	n, err := oc.Stdout().Write([]byte(testMessage))

	if err != nil {
		t.Errorf("Write should not error, got: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Write returned %d, expected %d", n, len(testMessage))
	}

	oc.Close()

	// Channel should be empty (message was dropped)
	select {
	case msg := <-eventsChan:
		t.Errorf("eventsChan should be empty, got: %q", msg)
	default:
		// Expected - channel is empty
	}
}

func TestOutputCapture_WithEventsChan_StderrAlsoStreams(t *testing.T) {
	tmpDir := t.TempDir()

	eventsChan := make(chan string, 10)

	oc, err := NewOutputCaptureWithEvents(tmpDir, eventsChan)
	if err != nil {
		t.Fatalf("NewOutputCaptureWithEvents() error: %v", err)
	}

	// Write to stderr
	testMessage := "Error message\n"
	oc.Stderr().Write([]byte(testMessage))

	oc.Close()

	// Check the channel received the message
	select {
	case msg := <-eventsChan:
		if msg != testMessage {
			t.Errorf("eventsChan received %q, want %q", msg, testMessage)
		}
	default:
		t.Error("eventsChan should have received stderr message")
	}
}

func TestOutputCapture_WithNilEventsChan_WorksNormally(t *testing.T) {
	tmpDir := t.TempDir()

	// Pass nil for eventsChan - should work like regular OutputCapture
	oc, err := NewOutputCaptureWithEvents(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewOutputCaptureWithEvents() error: %v", err)
	}

	// Write should succeed
	testMessage := "Normal output\n"
	n, err := oc.Stdout().Write([]byte(testMessage))

	if err != nil {
		t.Errorf("Write should not error, got: %v", err)
	}
	if n != len(testMessage) {
		t.Errorf("Write returned %d, expected %d", n, len(testMessage))
	}

	oc.Close()

	// Verify EventsChan returns nil
	if oc.EventsChan() != nil {
		t.Error("EventsChan() should return nil when not set")
	}
}

func TestOutputCapture_EventsChan_ReturnsChannel(t *testing.T) {
	tmpDir := t.TempDir()

	eventsChan := make(chan string, 10)

	oc, err := NewOutputCaptureWithEvents(tmpDir, eventsChan)
	if err != nil {
		t.Fatalf("NewOutputCaptureWithEvents() error: %v", err)
	}
	defer oc.Close()

	// Verify EventsChan returns the channel
	if oc.EventsChan() != eventsChan {
		t.Error("EventsChan() should return the provided channel")
	}
}

func TestOutputCapture_WithEventsChan_WritesToLogFile(t *testing.T) {
	tmpDir := t.TempDir()

	eventsChan := make(chan string, 10)

	oc, err := NewOutputCaptureWithEvents(tmpDir, eventsChan)
	if err != nil {
		t.Fatalf("NewOutputCaptureWithEvents() error: %v", err)
	}

	testMessage := "Test output message\n"
	oc.Stdout().Write([]byte(testMessage))

	oc.Close()

	// Verify message was written to log file (not just the channel)
	logPath := tmpDir + "/" + outputLogFileName
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test output message") {
		t.Errorf("log file should contain test message, got: %s", string(content))
	}
}

func TestFormatStreamLine_TextDelta(t *testing.T) {
	// Test extracting text from stream_event with text_delta
	jsonLine := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello world"}}}`

	result := FormatStreamLine(jsonLine)
	expected := "Hello world"
	if result != expected {
		t.Errorf("formatStreamLine() = %q, want %q", result, expected)
	}
}

func TestFormatStreamLine_ToolUse(t *testing.T) {
	// Tool markers are now handled by activity hooks, not output text.
	jsonLine := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash"}]}}`

	result := FormatStreamLine(jsonLine)
	if result != "" {
		t.Errorf("formatStreamLine() = %q, want empty string", result)
	}
}

func TestFormatStreamLine_SystemEvent(t *testing.T) {
	// System events should be ignored
	jsonLine := `{"type":"system","subtype":"init","session_id":"abc123"}`

	result := FormatStreamLine(jsonLine)
	if result != "" {
		t.Errorf("formatStreamLine() for system event = %q, want empty string", result)
	}
}

func TestFormatStreamLine_ResultEvent(t *testing.T) {
	// Result events should be ignored
	jsonLine := `{"type":"result","subtype":"success","result":"done"}`

	result := FormatStreamLine(jsonLine)
	if result != "" {
		t.Errorf("formatStreamLine() for result event = %q, want empty string", result)
	}
}

func TestFormatStreamLine_PlainText(t *testing.T) {
	// Non-JSON input should pass through unchanged
	plainText := "This is plain text output"

	result := FormatStreamLine(plainText)
	if result != plainText {
		t.Errorf("formatStreamLine() = %q, want %q", result, plainText)
	}
}

func TestFormatStreamLine_EmptyLine(t *testing.T) {
	result := FormatStreamLine("")
	if result != "" {
		t.Errorf("formatStreamLine() for empty = %q, want empty string", result)
	}

	result = FormatStreamLine("   ")
	if result != "" {
		t.Errorf("formatStreamLine() for whitespace = %q, want empty string", result)
	}
}

func TestStreamingWriter_LineBuffering(t *testing.T) {
	// Test that partial writes are buffered until newline
	eventsChan := make(chan string, 10)

	sw := &streamingWriter{
		underlying: io.Discard,
		eventsChan: eventsChan,
		isStderr:   false,
	}

	// Write partial line
	sw.Write([]byte("Hello "))
	sw.Write([]byte("world"))

	// No message yet (no newline)
	select {
	case msg := <-eventsChan:
		t.Errorf("should not receive message before newline, got: %q", msg)
	default:
		// Expected
	}

	// Write newline - should now receive the complete line
	sw.Write([]byte("\n"))

	select {
	case msg := <-eventsChan:
		expected := "Hello world"
		if msg != expected {
			t.Errorf("received %q, want %q", msg, expected)
		}
	default:
		t.Error("should have received message after newline")
	}
}

func TestStreamingWriter_JSONParsing(t *testing.T) {
	eventsChan := make(chan string, 10)

	sw := &streamingWriter{
		underlying: io.Discard,
		eventsChan: eventsChan,
		isStderr:   false,
	}

	// Write a text delta followed by a boundary event so buffered content flushes.
	jsonLine := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Streamed text"}}}` + "\n"
	boundary := `{"type":"result","subtype":"success"}` + "\n"
	sw.Write([]byte(jsonLine))
	sw.Write([]byte(boundary))

	select {
	case msg := <-eventsChan:
		expected := "Streamed text"
		if msg != expected {
			t.Errorf("received %q, want %q", msg, expected)
		}
	default:
		t.Error("should have received parsed text from JSON")
	}
}

func TestStreamingWriter_JSONParsing_CoalescesTextDeltas(t *testing.T) {
	eventsChan := make(chan string, 10)

	sw := &streamingWriter{
		underlying: io.Discard,
		eventsChan: eventsChan,
		isStderr:   false,
	}

	first := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"I'll start"}}}` + "\n"
	second := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":" by reading files"}}}` + "\n"
	stop := `{"type":"stream_event","event":{"type":"message_stop"}}` + "\n"

	sw.Write([]byte(first))
	sw.Write([]byte(second))
	sw.Write([]byte(stop))

	select {
	case msg := <-eventsChan:
		expected := "I'll start by reading files"
		if msg != expected {
			t.Errorf("received %q, want %q", msg, expected)
		}
	default:
		t.Fatal("should have received coalesced text")
	}

	select {
	case extra := <-eventsChan:
		t.Fatalf("expected a single coalesced message, got extra: %q", extra)
	default:
	}
}

func TestStreamingWriter_Hooks_EmitToolLifecycleAndUsage(t *testing.T) {
	eventsChan := make(chan string, 10)

	var (
		gotToolName   string
		gotToolTarget string
		gotToolResult bool
		gotInput      int64
		gotOutput     int64
		gotCost       float64
	)

	sw := &streamingWriter{
		underlying: io.Discard,
		eventsChan: eventsChan,
		hooks: StreamHooks{
			OnToolUse: func(toolName, toolTarget string) {
				gotToolName = toolName
				gotToolTarget = toolTarget
			},
			OnToolResult: func() {
				gotToolResult = true
			},
			OnUsage: func(inputTokens, outputTokens int64, costUSD float64) {
				gotInput = inputTokens
				gotOutput = outputTokens
				gotCost = costUSD
			},
		},
	}

	toolUse := `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"tool_use","name":"Read","input":{"file_path":"internal/tui/views/run.go"}}}}` + "\n"
	toolResult := `{"type":"user","message":{"content":[{"type":"tool_result"}]}}` + "\n"
	usage := `{"type":"result","usage":{"input_tokens":123,"output_tokens":45},"total_cost_usd":0.007}` + "\n"

	sw.Write([]byte(toolUse))
	sw.Write([]byte(toolResult))
	sw.Write([]byte(usage))

	if gotToolName != "Read" {
		t.Fatalf("expected tool name Read, got %q", gotToolName)
	}
	if gotToolTarget != "internal/tui/views/run.go" {
		t.Fatalf("expected tool target internal/tui/views/run.go, got %q", gotToolTarget)
	}
	if !gotToolResult {
		t.Fatalf("expected tool result hook to be called")
	}
	if gotInput != 123 || gotOutput != 45 {
		t.Fatalf("expected usage tokens 123/45, got %d/%d", gotInput, gotOutput)
	}
	if gotCost != 0.007 {
		t.Fatalf("expected cost 0.007, got %f", gotCost)
	}
}

func TestStreamingWriter_Hooks_EmitAssistantBoundary(t *testing.T) {
	sw := &streamingWriter{
		underlying: io.Discard,
		hooks: StreamHooks{
			OnAssistantBoundary: func() {
				// marker assertion below
			},
		},
	}

	called := false
	sw.hooks.OnAssistantBoundary = func() {
		called = true
	}

	assistant := `{"type":"assistant","message":{"content":[{"type":"text","text":"Done."}]}}` + "\n"
	if _, err := sw.Write([]byte(assistant)); err != nil {
		t.Fatalf("write error: %v", err)
	}
	if !called {
		t.Fatalf("expected assistant boundary hook to be called")
	}
}

func TestExtractCommitMessage_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write JSON stream events including one with commit message
	oc.Stdout().Write([]byte(`{"type":"system","subtype":"init"}` + "\n"))
	oc.Stdout().Write([]byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Working on the task..."}}}` + "\n"))
	oc.Stdout().Write([]byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"SUGGESTED_COMMIT_MESSAGE: Add JSON streaming support"}}}` + "\n"))
	oc.Stdout().Write([]byte(`{"type":"result","subtype":"success"}` + "\n"))

	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	expected := "Add JSON streaming support"
	if msg != expected {
		t.Errorf("ExtractCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}

func TestExtractCommitMessage_AssistantMessage(t *testing.T) {
	tmpDir := t.TempDir()

	oc, err := NewOutputCapture(tmpDir)
	if err != nil {
		t.Fatalf("NewOutputCapture() error: %v", err)
	}

	// Write assistant message with commit message in content
	oc.Stdout().Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Done!\nSUGGESTED_COMMIT_MESSAGE: Implement feature X\nAll tests pass."}]}}` + "\n"))

	oc.logFile.Sync()

	msg := oc.ExtractCommitMessage()
	expected := "Implement feature X"
	if msg != expected {
		t.Errorf("ExtractCommitMessage() = %q, want %q", msg, expected)
	}

	oc.Close()
}
