package executor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const outputLogFileName = "output.log"

const streamChunkFlushBytes = 768

// AssistantBoundaryChunk is an internal stream marker sent over eventsChan to
// indicate an assistant turn boundary. It is not written to output.log.
// Keeping this marker on the same channel as text preserves ordering.
const AssistantBoundaryChunk = "\x00RAFA_ASSISTANT_BOUNDARY\x00"

// OutputWriter provides writers for capturing command output.
// This interface is used by Runner to allow for different output strategies.
type OutputWriter interface {
	Stdout() io.Writer
	Stderr() io.Writer
}

// StreamHooks provides callbacks for structured stream events.
// All callbacks are optional.
type StreamHooks struct {
	OnToolUse           func(toolID, parentToolID, toolName, toolTarget string)
	OnToolResult        func(toolID string)
	OnUsage             func(inputTokens, outputTokens int64, costUSD float64)
	OnAssistantBoundary func()
}

func (h StreamHooks) hasCallbacks() bool {
	return h.OnToolUse != nil ||
		h.OnToolResult != nil ||
		h.OnUsage != nil ||
		h.OnAssistantBoundary != nil
}

// OutputCapture manages output to both terminal and log file.
type OutputCapture struct {
	logFile    *os.File
	multiOut   io.Writer
	multiErr   io.Writer
	eventsChan chan string // For TUI consumption; nil when not streaming
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
	return NewOutputCaptureWithEventsAndHooks(planDir, eventsChan, StreamHooks{})
}

// NewOutputCaptureWithEventsAndHooks creates an output capture with optional
// event streaming and structured stream callbacks.
func NewOutputCaptureWithEventsAndHooks(planDir string, eventsChan chan string, hooks StreamHooks) (*OutputCapture, error) {
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
	if eventsChan != nil || hooks.hasCallbacks() {
		stdoutUnderlying := io.Writer(f)
		stderrUnderlying := io.Writer(f)
		if eventsChan == nil {
			stdoutUnderlying = io.MultiWriter(os.Stdout, f)
			stderrUnderlying = io.MultiWriter(os.Stderr, f)
		}

		streamingOut := &streamingWriter{
			underlying: stdoutUnderlying,
			eventsChan: eventsChan,
			hooks:      hooks,
			isStderr:   false,
		}
		oc.multiOut = streamingOut

		if eventsChan != nil {
			streamingErr := &streamingWriter{
				underlying: stderrUnderlying,
				eventsChan: eventsChan,
				isStderr:   true, // Pass through raw stderr for error messages
			}
			oc.multiErr = streamingErr
		} else {
			oc.multiErr = stderrUnderlying
		}
	} else {
		oc.multiOut = io.MultiWriter(os.Stdout, f)
		oc.multiErr = io.MultiWriter(os.Stderr, f)
	}

	return oc, nil
}

// streamingWriter wraps a writer and sends output to a channel for TUI streaming.
// It buffers partial lines and parses JSON stream events to extract displayable text.
type streamingWriter struct {
	underlying io.Writer
	eventsChan chan string
	lineBuf    strings.Builder // Buffer for partial lines
	outputBuf  strings.Builder // Buffer for coalescing tiny text deltas
	hooks      StreamHooks
	isStderr   bool // If true, pass through raw (no JSON parsing)
}

// Write writes to the underlying writer and sends parsed output to eventsChan.
// For stdout, it parses JSON stream events and extracts displayable text.
// For stderr, it passes through raw text for error messages.
func (s *streamingWriter) Write(p []byte) (n int, err error) {
	// Always write raw data to underlying writer (log file)
	n, err = s.underlying.Write(p)

	// If no stream consumers are active, no extra processing is needed.
	if s.eventsChan == nil && !s.hooks.hasCallbacks() {
		return
	}

	// For stderr, pass through raw text (actual errors)
	if s.isStderr {
		select {
		case s.eventsChan <- string(p):
		default:
			// Drop if buffer full, don't block execution
		}
		return
	}

	// For stdout, buffer and process complete lines for JSON parsing
	s.lineBuf.Write(p)
	content := s.lineBuf.String()

	for {
		idx := strings.Index(content, "\n")
		if idx == -1 {
			break
		}
		line := content[:idx]
		content = content[idx+1:]

		// Parse JSON and extract displayable text plus structured events.
		parsed := parseStreamLineDetails(line)
		if parsed.ToolUse != nil {
			s.emitToolUse(parsed.ToolUse.ID, parsed.ToolUse.ParentToolID, parsed.ToolUse.Name, parsed.ToolUse.Target)
		}
		if parsed.ToolResult != nil {
			s.emitToolResult(parsed.ToolResult.ToolID)
		}
		if parsed.Usage != nil {
			s.emitUsage(parsed.Usage.InputTokens, parsed.Usage.OutputTokens, parsed.Usage.CostUSD)
		}
		if parsed.Text != "" {
			s.outputBuf.WriteString(parsed.Text)
			if strings.Contains(parsed.Text, "\n") || s.outputBuf.Len() >= streamChunkFlushBytes {
				s.flushOutput()
			}
		}
		if parsed.Flush {
			s.flushOutput()
		}
		// Emit assistant boundary only after pending text is flushed so
		// run-view separators are never inserted mid-sentence.
		if parsed.AssistantBoundary {
			s.emit(AssistantBoundaryChunk)
			s.emitAssistantBoundary()
		}
	}

	s.lineBuf.Reset()
	s.lineBuf.WriteString(content)
	return
}

func (s *streamingWriter) emit(text string) {
	if text == "" || s.eventsChan == nil {
		return
	}
	select {
	case s.eventsChan <- text:
	default:
		// Drop if buffer full, don't block execution
	}
}

func (s *streamingWriter) flushOutput() {
	if s.outputBuf.Len() == 0 {
		return
	}
	s.emit(s.outputBuf.String())
	s.outputBuf.Reset()
}

func (s *streamingWriter) emitToolUse(toolID, parentToolID, toolName, toolTarget string) {
	if s.hooks.OnToolUse != nil {
		s.hooks.OnToolUse(toolID, parentToolID, toolName, toolTarget)
	}
}

func (s *streamingWriter) emitToolResult(toolID string) {
	if s.hooks.OnToolResult != nil {
		s.hooks.OnToolResult(toolID)
	}
}

func (s *streamingWriter) emitUsage(inputTokens, outputTokens int64, costUSD float64) {
	if s.hooks.OnUsage != nil {
		s.hooks.OnUsage(inputTokens, outputTokens, costUSD)
	}
}

func (s *streamingWriter) emitAssistantBoundary() {
	if s.hooks.OnAssistantBoundary != nil {
		s.hooks.OnAssistantBoundary()
	}
}

type parsedStreamLine struct {
	Text              string
	Flush             bool
	ToolUse           *toolUseEvent
	ToolResult        *toolResultEvent
	Usage             *usageEvent
	AssistantBoundary bool
}

type toolUseEvent struct {
	ID           string
	ParentToolID string
	Name         string
	Target       string
}

type toolResultEvent struct {
	ToolID string
}

type usageEvent struct {
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

// streamEvent represents the JSON structure from Claude's stream-json output.
type streamEvent struct {
	Type            string `json:"type"`
	Subtype         string `json:"subtype,omitempty"`
	IsError         bool   `json:"is_error,omitempty"`
	ParentToolUseID string `json:"parent_tool_use_id,omitempty"`
	Event           *struct {
		Type         string `json:"type"`
		Delta        *streamDelta
		ContentBlock *streamContentBlock `json:"content_block,omitempty"`
	} `json:"event,omitempty"`
	Message *struct {
		Content []streamContent `json:"content"`
	} `json:"message,omitempty"`
	Usage *struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
}

type streamDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type streamContentBlock struct {
	ID    string                 `json:"id,omitempty"`
	Type  string                 `json:"type"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type streamContent struct {
	ID        string                 `json:"id,omitempty"`
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
}

// FormatStreamLine parses a JSON stream line and extracts displayable text.
// For non-JSON input, it returns the line unchanged.
func FormatStreamLine(line string) string {
	formatted, _ := parseStreamLine(line)
	return formatted
}

// parseStreamLine parses a stream-json line into display text and a flush hint.
// The flush hint is true when buffered text should be emitted (message boundaries,
// result events, or plain-text line boundaries).
func parseStreamLine(line string) (string, bool) {
	parsed := parseStreamLineDetails(line)
	return parsed.Text, parsed.Flush
}

func parseStreamLineDetails(line string) parsedStreamLine {
	line = strings.TrimSpace(line)
	if line == "" {
		return parsedStreamLine{}
	}

	var event streamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		// Not JSON - return as-is
		return parsedStreamLine{Text: line, Flush: true}
	}

	switch event.Type {
	case "stream_event":
		if event.Event != nil {
			switch event.Event.Type {
			case "content_block_delta":
				// Token-level streaming - extract delta text
				if event.Event.Delta != nil && event.Event.Delta.Type == "text_delta" {
					return parsedStreamLine{Text: event.Event.Delta.Text}
				}
			case "content_block_start":
				if event.Event.ContentBlock != nil &&
					event.Event.ContentBlock.Type == "tool_use" &&
					event.Event.ContentBlock.Name != "" {
					name := event.Event.ContentBlock.Name
					return parsedStreamLine{
						ToolUse: &toolUseEvent{
							ID:           event.Event.ContentBlock.ID,
							ParentToolID: event.ParentToolUseID,
							Name:         name,
							Target:       extractToolTarget(name, event.Event.ContentBlock.Input),
						},
					}
				}
			case "content_block_stop", "message_stop":
				return parsedStreamLine{Flush: true}
			}
		}
	case "assistant":
		// Assistant turn boundary - flush any buffered delta text.
		parsed := parsedStreamLine{Flush: true, AssistantBoundary: true}
		if event.Message != nil {
			for _, c := range event.Message.Content {
				if c.Type == "tool_use" && c.Name != "" {
					parsed.ToolUse = &toolUseEvent{
						ID:           c.ID,
						ParentToolID: event.ParentToolUseID,
						Name:         c.Name,
						Target:       extractToolTarget(c.Name, c.Input),
					}
					break
				}
			}
		}
		return parsed
	case "user":
		if event.Message != nil {
			for _, c := range event.Message.Content {
				if c.Type == "tool_result" {
					return parsedStreamLine{
						ToolResult: &toolResultEvent{
							ToolID: c.ToolUseID,
						},
						Flush: true,
					}
				}
			}
		}
	case "result":
		parsed := parsedStreamLine{Flush: true}
		if event.Usage != nil {
			parsed.Usage = &usageEvent{
				InputTokens:  event.Usage.InputTokens,
				OutputTokens: event.Usage.OutputTokens,
				CostUSD:      event.TotalCostUSD,
			}
		}
		return parsed
	}

	// Ignore system/user/meta events.
	return parsedStreamLine{}
}

func extractToolTarget(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "Read", "Write", "Edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Glob", "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "Task":
		if desc, ok := input["description"].(string); ok {
			return desc
		}
	case "Bash":
		if command, ok := input["command"].(string); ok {
			return command
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok {
			return url
		}
	}
	return ""
}

// Stdout returns the writer for stdout.
func (oc *OutputCapture) Stdout() io.Writer {
	return oc.multiOut
}

// Stderr returns the writer for stderr.
func (oc *OutputCapture) Stderr() io.Writer {
	return oc.multiErr
}

// Close closes the log file. Safe to call when no log file is open.
func (oc *OutputCapture) Close() error {
	if oc.logFile != nil {
		return oc.logFile.Close()
	}
	return nil
}

// EventsChan returns the events channel for TUI streaming, or nil if not streaming.
func (oc *OutputCapture) EventsChan() chan string {
	return oc.eventsChan
}

// WriteTaskHeader writes a header line to the log for a new task attempt.
// Safe to call when no log file is open.
func (oc *OutputCapture) WriteTaskHeader(taskID string, attempt int) {
	if oc.logFile == nil {
		return
	}
	header := fmt.Sprintf("\n=== Task %s, Attempt %d ===\n", taskID, attempt)
	oc.logFile.WriteString(header)
	oc.logFile.WriteString(fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC3339)))
}

// WriteTaskFooter writes a footer line to the log after task completion.
// Safe to call when no log file is open.
func (oc *OutputCapture) WriteTaskFooter(taskID string, success bool) {
	if oc.logFile == nil {
		return
	}
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
// It handles both JSON stream format and plain text format.
// For JSON, it searches text_delta events for the commit message prefix.
// Returns the trimmed message after the prefix, or empty string if no message is found.
// Searches the last 100 lines for efficiency. If multiple messages exist, returns the
// most recent one. Callers should ensure the log file is synced before calling.
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

		// Try to extract commit message from this line (handles both JSON and plain text)
		if msg := extractCommitMessageFromLine(line); msg != "" {
			return msg
		}
	}

	return ""
}

// extractCommitMessageFromLine extracts a commit message from a single line.
// Handles both JSON stream format and plain text format.
func extractCommitMessageFromLine(line string) string {
	// First, check for plain text format
	if strings.HasPrefix(line, commitMessagePrefix) {
		return strings.TrimSpace(strings.TrimPrefix(line, commitMessagePrefix))
	}

	// Try to parse as JSON (Claude stream-json format)
	var event streamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return ""
	}

	// Check stream_event with text_delta
	if event.Type == "stream_event" && event.Event != nil {
		if event.Event.Type == "content_block_delta" && event.Event.Delta != nil {
			if event.Event.Delta.Type == "text_delta" {
				text := event.Event.Delta.Text
				if strings.Contains(text, commitMessagePrefix) {
					// Extract the message from the delta text
					idx := strings.Index(text, commitMessagePrefix)
					msg := text[idx+len(commitMessagePrefix):]
					// Handle case where message might span multiple deltas - just get this part
					msg = strings.TrimSpace(msg)
					// Remove trailing newline if present
					msg = strings.TrimSuffix(msg, "\n")
					return msg
				}
			}
		}
	}

	// Check assistant message content
	if event.Type == "assistant" && event.Message != nil {
		for _, c := range event.Message.Content {
			if c.Type == "text" && strings.Contains(c.Text, commitMessagePrefix) {
				idx := strings.Index(c.Text, commitMessagePrefix)
				msg := c.Text[idx+len(commitMessagePrefix):]
				// Take up to the first newline
				if nlIdx := strings.Index(msg, "\n"); nlIdx != -1 {
					msg = msg[:nlIdx]
				}
				return strings.TrimSpace(msg)
			}
		}
	}

	return ""
}
