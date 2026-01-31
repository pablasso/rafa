package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ErrSessionExpired indicates the session cannot be resumed.
var ErrSessionExpired = errors.New("session expired or not found")

// ConversationConfig holds settings for a conversation session.
type ConversationConfig struct {
	SessionID     string // Claude session ID for --resume
	InitialPrompt string // First message to send
	SkillName     string // Skill name for context (used by TUI layer, not by invoke)
}

// StreamEvent represents a parsed event from Claude's stream-json output.
type StreamEvent struct {
	Type           string // "init", "text", "tool_use", "tool_result", "error", "done"
	Text           string // For text events
	ToolName       string // For tool_use events
	ToolTarget     string // File or resource being accessed
	SessionID      string // Available from init, assistant, and result events
	InputTokens    int64  // Token usage (from result event)
	OutputTokens   int64
	CostUSD        float64 // Total cost (from result event)
	SessionExpired bool    // True if this error is due to session expiration
}

// Conversation manages a multi-turn conversation with Claude CLI.
// Each message is a separate `claude -p` invocation with --resume.
type Conversation struct {
	sessionID string
	ctx       context.Context
	cancel    context.CancelFunc

	// Current invocation state
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

// StartConversation begins a new conversation with the initial prompt.
// Returns after Claude finishes the first response.
func StartConversation(ctx context.Context, config ConversationConfig) (*Conversation, <-chan StreamEvent, error) {
	if !IsClaudeAvailable() {
		return nil, nil, errors.New("Claude Code CLI not found. Install it: https://claude.ai/code")
	}

	ctx, cancel := context.WithCancel(ctx)

	conv := &Conversation{
		sessionID: config.SessionID,
		ctx:       ctx,
		cancel:    cancel,
	}

	events, err := conv.invoke(config.InitialPrompt)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return conv, events, nil
}

// SendMessage sends a follow-up message in the conversation.
// Uses --resume with the session ID from previous responses.
// Returns a channel of events for this response.
func (c *Conversation) SendMessage(message string) (<-chan StreamEvent, error) {
	if c.sessionID == "" {
		return nil, fmt.Errorf("no session ID available for resume")
	}
	return c.invoke(message)
}

// invoke runs a single claude -p invocation and returns the event stream.
func (c *Conversation) invoke(prompt string) (<-chan StreamEvent, error) {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--dangerously-skip-permissions",
	}

	if c.sessionID != "" {
		args = append(args, "--resume", c.sessionID)
	}

	c.mu.Lock()
	c.cmd = CommandContext(c.ctx, "claude", args...)
	c.cmd.Stderr = os.Stderr

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.stdout = stdout

	if err := c.cmd.Start(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	events := make(chan StreamEvent, 100)

	go func() {
		defer close(events)
		scanner := bufio.NewScanner(stdout)
		// Increase buffer size for large JSON lines
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			// Check for session expiration error
			if isSessionExpiredError(line) {
				events <- StreamEvent{Type: "error", Text: "session expired", SessionExpired: true}
				break
			}

			event := parseStreamEvent(line)
			if event.Type != "" {
				// Capture session ID for future --resume calls
				if event.SessionID != "" {
					c.mu.Lock()
					c.sessionID = event.SessionID
					c.mu.Unlock()
				}
				events <- event
			}
		}

		// Check for scanner errors (e.g., line too long)
		if err := scanner.Err(); err != nil {
			events <- StreamEvent{Type: "error", Text: fmt.Sprintf("stream read error: %v", err)}
		}

		// Wait for command to finish
		c.mu.Lock()
		if c.cmd != nil {
			c.cmd.Wait()
		}
		c.mu.Unlock()
	}()

	return events, nil
}

// SessionID returns the current session ID for persistence.
func (c *Conversation) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

// Stop terminates the current invocation.
func (c *Conversation) Stop() error {
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// isSessionExpiredError checks if the output indicates an expired session.
func isSessionExpiredError(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "session not found") ||
		strings.Contains(lower, "session expired") ||
		strings.Contains(lower, "invalid session")
}

// IsSessionExpiredEvent checks if a StreamEvent indicates session expiration.
func IsSessionExpiredEvent(event StreamEvent) bool {
	return event.SessionExpired || (event.Type == "error" && isSessionExpiredError(event.Text))
}

// parseStreamEvent converts a JSON line to a StreamEvent.
// Stream-json format (verified against Claude CLI v2.1.27):
// - {"type":"system","subtype":"init","session_id":"uuid",...} - session start
// - {"type":"stream_event","event":{...}} - streaming content
// - {"type":"assistant","message":{...},"session_id":"uuid"} - assistant responses
// - {"type":"user","message":{...}} - tool results
// - {"type":"result","session_id":"uuid","total_cost_usd":0.01,...} - completion
func parseStreamEvent(line string) StreamEvent {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return StreamEvent{}
	}

	eventType, _ := raw["type"].(string)

	switch eventType {
	case "system":
		return parseSystemEvent(raw)

	case "stream_event":
		return parseStreamEventNested(raw)

	case "assistant":
		return parseAssistantMessage(raw)

	case "user":
		return parseUserMessage(raw)

	case "result":
		return parseResultEvent(raw)
	}

	return StreamEvent{}
}

// parseSystemEvent handles system events like init.
func parseSystemEvent(raw map[string]interface{}) StreamEvent {
	subtype, _ := raw["subtype"].(string)
	if subtype == "init" {
		sessionID, _ := raw["session_id"].(string)
		return StreamEvent{Type: "init", SessionID: sessionID}
	}
	return StreamEvent{}
}

// parseStreamEventNested handles nested stream_event structures.
func parseStreamEventNested(raw map[string]interface{}) StreamEvent {
	event, ok := raw["event"].(map[string]interface{})
	if !ok {
		return StreamEvent{}
	}

	eventType, _ := event["type"].(string)

	switch eventType {
	case "content_block_delta":
		delta, _ := event["delta"].(map[string]interface{})
		if deltaType, _ := delta["type"].(string); deltaType == "text_delta" {
			text, _ := delta["text"].(string)
			return StreamEvent{Type: "text", Text: text}
		}
	case "content_block_start":
		// Check for tool_use
		block, _ := event["content_block"].(map[string]interface{})
		if blockType, _ := block["type"].(string); blockType == "tool_use" {
			name, _ := block["name"].(string)
			return StreamEvent{Type: "tool_use", ToolName: name}
		}
	}

	return StreamEvent{}
}

// parseAssistantMessage extracts tool uses from complete messages.
func parseAssistantMessage(raw map[string]interface{}) StreamEvent {
	message, ok := raw["message"].(map[string]interface{})
	if !ok {
		return StreamEvent{}
	}

	// Extract session ID if present
	sessionID, _ := raw["session_id"].(string)

	content, ok := message["content"].([]interface{})
	if !ok {
		return StreamEvent{}
	}

	for _, c := range content {
		block, _ := c.(map[string]interface{})
		if blockType, _ := block["type"].(string); blockType == "tool_use" {
			name, _ := block["name"].(string)
			// Extract target from input if available
			input, _ := block["input"].(map[string]interface{})
			target := extractToolTarget(name, input)
			return StreamEvent{Type: "tool_use", ToolName: name, ToolTarget: target, SessionID: sessionID}
		}
	}

	return StreamEvent{}
}

// parseUserMessage extracts tool results from user messages.
func parseUserMessage(raw map[string]interface{}) StreamEvent {
	message, ok := raw["message"].(map[string]interface{})
	if !ok {
		return StreamEvent{}
	}

	content, ok := message["content"].([]interface{})
	if !ok {
		return StreamEvent{}
	}

	for _, c := range content {
		block, _ := c.(map[string]interface{})
		if blockType, _ := block["type"].(string); blockType == "tool_result" {
			return StreamEvent{Type: "tool_result"}
		}
	}

	return StreamEvent{}
}

// parseResultEvent handles the final result event with usage and cost.
func parseResultEvent(raw map[string]interface{}) StreamEvent {
	// Check for error
	if isError, _ := raw["is_error"].(bool); isError {
		return StreamEvent{Type: "error"}
	}

	sessionID, _ := raw["session_id"].(string)
	costUSD, _ := raw["total_cost_usd"].(float64)

	var inputTokens, outputTokens int64
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		if v, ok := usage["input_tokens"].(float64); ok {
			inputTokens = int64(v)
		}
		if v, ok := usage["output_tokens"].(float64); ok {
			outputTokens = int64(v)
		}
	}

	return StreamEvent{
		Type:         "done",
		SessionID:    sessionID,
		CostUSD:      costUSD,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}
}

// extractToolTarget gets the relevant target from tool input.
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
