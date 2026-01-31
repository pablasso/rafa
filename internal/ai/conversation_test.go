package ai

import (
	"testing"
)

func TestParseStreamEvent_Init(t *testing.T) {
	// Test init event returning SessionID
	input := `{"type":"system","subtype":"init","session_id":"9f36fe2b-469c-4f0e-8df3-fc8ff1b2e297","tools":[],"mcp_servers":[]}`

	event := parseStreamEvent(input)

	if event.Type != "init" {
		t.Errorf("expected Type='init', got %q", event.Type)
	}
	if event.SessionID != "9f36fe2b-469c-4f0e-8df3-fc8ff1b2e297" {
		t.Errorf("expected SessionID='9f36fe2b-469c-4f0e-8df3-fc8ff1b2e297', got %q", event.SessionID)
	}
}

func TestParseStreamEvent_TextDelta(t *testing.T) {
	// Test text_delta returning text content
	input := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello, world!"}}}`

	event := parseStreamEvent(input)

	if event.Type != "text" {
		t.Errorf("expected Type='text', got %q", event.Type)
	}
	if event.Text != "Hello, world!" {
		t.Errorf("expected Text='Hello, world!', got %q", event.Text)
	}
}

func TestParseStreamEvent_ToolUse(t *testing.T) {
	// Test tool_use returning ToolName and ToolTarget
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/path/to/file.go"}}]},"session_id":"abc-123"}`

	event := parseStreamEvent(input)

	if event.Type != "tool_use" {
		t.Errorf("expected Type='tool_use', got %q", event.Type)
	}
	if event.ToolName != "Read" {
		t.Errorf("expected ToolName='Read', got %q", event.ToolName)
	}
	if event.ToolTarget != "/path/to/file.go" {
		t.Errorf("expected ToolTarget='/path/to/file.go', got %q", event.ToolTarget)
	}
	if event.SessionID != "abc-123" {
		t.Errorf("expected SessionID='abc-123', got %q", event.SessionID)
	}
}

func TestParseStreamEvent_TaskToolExtractsDescription(t *testing.T) {
	// Test Task tool extracting subagent description
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_2","name":"Task","input":{"description":"Explore codebase structure","prompt":"Find all Go files","subagent_type":"Explore"}}]}}`

	event := parseStreamEvent(input)

	if event.Type != "tool_use" {
		t.Errorf("expected Type='tool_use', got %q", event.Type)
	}
	if event.ToolName != "Task" {
		t.Errorf("expected ToolName='Task', got %q", event.ToolName)
	}
	if event.ToolTarget != "Explore codebase structure" {
		t.Errorf("expected ToolTarget='Explore codebase structure', got %q", event.ToolTarget)
	}
}

func TestParseStreamEvent_ToolResult(t *testing.T) {
	// Test tool_result event
	input := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"file contents here"}]}}`

	event := parseStreamEvent(input)

	if event.Type != "tool_result" {
		t.Errorf("expected Type='tool_result', got %q", event.Type)
	}
}

func TestParseStreamEvent_ResultExtractsUsageAndCost(t *testing.T) {
	// Test result event extracting usage and cost
	input := `{"type":"result","session_id":"xyz-789","total_cost_usd":0.01772025,"usage":{"input_tokens":3000,"output_tokens":500,"cache_read_input_tokens":18098}}`

	event := parseStreamEvent(input)

	if event.Type != "done" {
		t.Errorf("expected Type='done', got %q", event.Type)
	}
	if event.SessionID != "xyz-789" {
		t.Errorf("expected SessionID='xyz-789', got %q", event.SessionID)
	}
	if event.CostUSD != 0.01772025 {
		t.Errorf("expected CostUSD=0.01772025, got %f", event.CostUSD)
	}
	if event.InputTokens != 3000 {
		t.Errorf("expected InputTokens=3000, got %d", event.InputTokens)
	}
	if event.OutputTokens != 500 {
		t.Errorf("expected OutputTokens=500, got %d", event.OutputTokens)
	}
}

func TestParseStreamEvent_ResultWithError(t *testing.T) {
	// Test result event with is_error:true returning error type
	input := `{"type":"result","is_error":true,"error":"Something went wrong"}`

	event := parseStreamEvent(input)

	if event.Type != "error" {
		t.Errorf("expected Type='error', got %q", event.Type)
	}
}

func TestParseStreamEvent_MalformedJSON(t *testing.T) {
	// Test malformed JSON returns empty StreamEvent (no panic)
	inputs := []string{
		`not json at all`,
		`{"type":`,
		`{broken: json}`,
		``,
		`null`,
	}

	for _, input := range inputs {
		event := parseStreamEvent(input)

		if event.Type != "" {
			t.Errorf("expected empty Type for malformed input %q, got %q", input, event.Type)
		}
	}
}

func TestParseStreamEvent_UnknownType(t *testing.T) {
	// Test unknown type returns empty StreamEvent
	input := `{"type":"unknown_event_type","data":"something"}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for unknown type, got %q", event.Type)
	}
}

func TestParseStreamEvent_SystemNonInit(t *testing.T) {
	// Test system event that is not init returns empty
	input := `{"type":"system","subtype":"other","data":"something"}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for non-init system event, got %q", event.Type)
	}
}

func TestParseStreamEvent_StreamEventContentBlockStart(t *testing.T) {
	// Test stream_event with content_block_start for tool_use
	input := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool_1","name":"Grep"}}}`

	event := parseStreamEvent(input)

	if event.Type != "tool_use" {
		t.Errorf("expected Type='tool_use', got %q", event.Type)
	}
	if event.ToolName != "Grep" {
		t.Errorf("expected ToolName='Grep', got %q", event.ToolName)
	}
}

func TestParseStreamEvent_AssistantNoToolUse(t *testing.T) {
	// Test assistant message without tool_use returns empty
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Just some text"}]}}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for text-only assistant message, got %q", event.Type)
	}
}

func TestParseStreamEvent_UserNoToolResult(t *testing.T) {
	// Test user message without tool_result returns empty
	input := `{"type":"user","message":{"content":[{"type":"text","text":"User input"}]}}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for text-only user message, got %q", event.Type)
	}
}

func TestExtractToolTarget_ReadWriteEdit(t *testing.T) {
	// Test file_path extraction for Read, Write, Edit tools
	tools := []string{"Read", "Write", "Edit"}

	for _, toolName := range tools {
		input := map[string]interface{}{
			"file_path": "/Users/test/code/main.go",
		}

		target := extractToolTarget(toolName, input)

		if target != "/Users/test/code/main.go" {
			t.Errorf("expected target='/Users/test/code/main.go' for %s, got %q", toolName, target)
		}
	}
}

func TestExtractToolTarget_GlobGrep(t *testing.T) {
	// Test pattern extraction for Glob and Grep tools
	tools := []string{"Glob", "Grep"}

	for _, toolName := range tools {
		input := map[string]interface{}{
			"pattern": "**/*.go",
		}

		target := extractToolTarget(toolName, input)

		if target != "**/*.go" {
			t.Errorf("expected target='**/*.go' for %s, got %q", toolName, target)
		}
	}
}

func TestExtractToolTarget_Task(t *testing.T) {
	// Test description extraction for Task tool
	input := map[string]interface{}{
		"description":   "Search for authentication code",
		"prompt":        "Find all files related to authentication",
		"subagent_type": "Explore",
	}

	target := extractToolTarget("Task", input)

	if target != "Search for authentication code" {
		t.Errorf("expected target='Search for authentication code', got %q", target)
	}
}

func TestExtractToolTarget_Bash(t *testing.T) {
	// Test command extraction for Bash tool
	input := map[string]interface{}{
		"command": "git status",
	}

	target := extractToolTarget("Bash", input)

	if target != "git status" {
		t.Errorf("expected target='git status', got %q", target)
	}
}

func TestExtractToolTarget_WebFetch(t *testing.T) {
	// Test url extraction for WebFetch tool
	input := map[string]interface{}{
		"url":    "https://example.com/api",
		"prompt": "Get the data",
	}

	target := extractToolTarget("WebFetch", input)

	if target != "https://example.com/api" {
		t.Errorf("expected target='https://example.com/api', got %q", target)
	}
}

func TestExtractToolTarget_UnknownTool(t *testing.T) {
	// Test unknown tool returns empty string
	input := map[string]interface{}{
		"something": "value",
	}

	target := extractToolTarget("UnknownTool", input)

	if target != "" {
		t.Errorf("expected empty target for unknown tool, got %q", target)
	}
}

func TestExtractToolTarget_MissingField(t *testing.T) {
	// Test missing field returns empty string
	input := map[string]interface{}{
		"other_field": "value",
	}

	target := extractToolTarget("Read", input)

	if target != "" {
		t.Errorf("expected empty target for missing file_path, got %q", target)
	}
}

func TestExtractToolTarget_NilInput(t *testing.T) {
	// Test nil input doesn't panic
	target := extractToolTarget("Read", nil)

	if target != "" {
		t.Errorf("expected empty target for nil input, got %q", target)
	}
}

func TestExtractToolTarget_EmptyInput(t *testing.T) {
	// Test empty input returns empty string
	input := map[string]interface{}{}

	target := extractToolTarget("Read", input)

	if target != "" {
		t.Errorf("expected empty target for empty input, got %q", target)
	}
}

func TestIsSessionExpiredError(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`{"error": "session not found"}`, true},
		{`{"error": "Session Not Found"}`, true},
		{`{"error": "session expired"}`, true},
		{`{"error": "Session Expired"}`, true},
		{`{"error": "invalid session"}`, true},
		{`{"error": "Invalid Session"}`, true},
		{`{"error": "some other error"}`, false},
		{`{"type": "result", "data": "success"}`, false},
		{``, false},
	}

	for _, tt := range tests {
		result := isSessionExpiredError(tt.input)
		if result != tt.expected {
			t.Errorf("isSessionExpiredError(%q) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseStreamEvent_ResultWithZeroUsage(t *testing.T) {
	// Test result event with zero or missing usage values
	input := `{"type":"result","session_id":"test-123","total_cost_usd":0.0,"usage":{}}`

	event := parseStreamEvent(input)

	if event.Type != "done" {
		t.Errorf("expected Type='done', got %q", event.Type)
	}
	if event.InputTokens != 0 {
		t.Errorf("expected InputTokens=0, got %d", event.InputTokens)
	}
	if event.OutputTokens != 0 {
		t.Errorf("expected OutputTokens=0, got %d", event.OutputTokens)
	}
	if event.CostUSD != 0.0 {
		t.Errorf("expected CostUSD=0.0, got %f", event.CostUSD)
	}
}

func TestParseStreamEvent_ResultWithoutUsage(t *testing.T) {
	// Test result event without usage field
	input := `{"type":"result","session_id":"test-456","total_cost_usd":0.05}`

	event := parseStreamEvent(input)

	if event.Type != "done" {
		t.Errorf("expected Type='done', got %q", event.Type)
	}
	if event.SessionID != "test-456" {
		t.Errorf("expected SessionID='test-456', got %q", event.SessionID)
	}
	if event.InputTokens != 0 {
		t.Errorf("expected InputTokens=0, got %d", event.InputTokens)
	}
	if event.OutputTokens != 0 {
		t.Errorf("expected OutputTokens=0, got %d", event.OutputTokens)
	}
}

func TestParseStreamEvent_EmptyStreamEvent(t *testing.T) {
	// Test stream_event with no event field
	input := `{"type":"stream_event"}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for empty stream_event, got %q", event.Type)
	}
}

func TestParseStreamEvent_StreamEventUnknownEventType(t *testing.T) {
	// Test stream_event with unknown event type
	input := `{"type":"stream_event","event":{"type":"message_start"}}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for unknown stream_event type, got %q", event.Type)
	}
}

func TestParseStreamEvent_TextDeltaNotTextType(t *testing.T) {
	// Test content_block_delta that isn't text_delta
	input := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for non-text delta, got %q", event.Type)
	}
}

func TestParseStreamEvent_ContentBlockStartNotToolUse(t *testing.T) {
	// Test content_block_start that isn't tool_use
	input := `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`

	event := parseStreamEvent(input)

	if event.Type != "" {
		t.Errorf("expected empty Type for non-tool_use content_block_start, got %q", event.Type)
	}
}

func TestParseStreamEvent_LargeTokenCounts(t *testing.T) {
	// Test result event with large token counts (int64)
	input := `{"type":"result","session_id":"large-test","total_cost_usd":150.25,"usage":{"input_tokens":5000000,"output_tokens":1000000}}`

	event := parseStreamEvent(input)

	if event.Type != "done" {
		t.Errorf("expected Type='done', got %q", event.Type)
	}
	if event.InputTokens != 5000000 {
		t.Errorf("expected InputTokens=5000000, got %d", event.InputTokens)
	}
	if event.OutputTokens != 1000000 {
		t.Errorf("expected OutputTokens=1000000, got %d", event.OutputTokens)
	}
	if event.CostUSD != 150.25 {
		t.Errorf("expected CostUSD=150.25, got %f", event.CostUSD)
	}
}
