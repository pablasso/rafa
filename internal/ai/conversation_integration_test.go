package ai

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// TestConversationResume_PassesResumeFlag tests that conversation resume
// correctly passes the --resume flag to the Claude CLI.
// This test mocks the Claude CLI to capture the arguments.
func TestConversationResume_PassesResumeFlag(t *testing.T) {
	// Save original
	originalCommandContext := CommandContext
	originalLookPath := LookPath

	t.Cleanup(func() {
		CommandContext = originalCommandContext
		LookPath = originalLookPath
	})

	// Track captured arguments
	var capturedArgs [][]string

	// Mock LookPath to pretend claude exists
	LookPath = func(file string) (string, error) {
		return "/usr/bin/claude", nil
	}

	// Mock CommandContext to capture arguments and return immediately
	CommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, args)
		// Return a command that outputs a minimal valid response and exits
		return exec.CommandContext(ctx, "echo", `{"type":"result","session_id":"new-session-123"}`)
	}

	// Start a conversation with an initial prompt (no session ID - new conversation)
	config := ConversationConfig{
		InitialPrompt: "Hello, this is a test",
	}

	conv, events, err := StartConversation(context.Background(), config)
	if err != nil {
		t.Fatalf("StartConversation failed: %v", err)
	}

	// Drain events
	for range events {
	}

	// Verify the initial call did NOT include --resume (new conversation)
	if len(capturedArgs) < 1 {
		t.Fatal("expected at least one call to claude")
	}

	initialArgs := capturedArgs[0]
	for _, arg := range initialArgs {
		if arg == "--resume" {
			t.Error("initial call should not include --resume flag")
		}
	}

	// Verify prompt was passed
	foundPrompt := false
	for i, arg := range initialArgs {
		if arg == "-p" && i+1 < len(initialArgs) && initialArgs[i+1] == "Hello, this is a test" {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Error("initial call should include the prompt")
	}

	// Now send a follow-up message (should use --resume)
	// Update the conversation's session ID (simulating what would happen from parsing the event)
	conv.sessionID = "test-session-456"

	capturedArgs = nil // Reset captured args

	// Mock again for the second call
	CommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append(capturedArgs, args)
		return exec.CommandContext(ctx, "echo", `{"type":"result","session_id":"test-session-456"}`)
	}

	_, err = conv.SendMessage("Follow-up message")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify the follow-up call includes --resume with the session ID
	if len(capturedArgs) < 1 {
		t.Fatal("expected at least one call for follow-up")
	}

	followUpArgs := capturedArgs[0]
	foundResume := false
	foundSessionID := false
	for i, arg := range followUpArgs {
		if arg == "--resume" {
			foundResume = true
			if i+1 < len(followUpArgs) && followUpArgs[i+1] == "test-session-456" {
				foundSessionID = true
			}
		}
	}

	if !foundResume {
		t.Error("follow-up call should include --resume flag")
	}
	if !foundSessionID {
		t.Error("follow-up call should include the session ID after --resume flag")
	}

	// Verify the follow-up prompt was passed
	foundFollowUpPrompt := false
	for i, arg := range followUpArgs {
		if arg == "-p" && i+1 < len(followUpArgs) && followUpArgs[i+1] == "Follow-up message" {
			foundFollowUpPrompt = true
			break
		}
	}
	if !foundFollowUpPrompt {
		t.Error("follow-up call should include the follow-up prompt")
	}
}

// TestConversationResume_WithExistingSession tests starting a conversation
// with an existing session ID (resuming an existing conversation).
func TestConversationResume_WithExistingSession(t *testing.T) {
	// Save original
	originalCommandContext := CommandContext
	originalLookPath := LookPath

	t.Cleanup(func() {
		CommandContext = originalCommandContext
		LookPath = originalLookPath
	})

	var capturedArgs []string

	// Mock LookPath
	LookPath = func(file string) (string, error) {
		return "/usr/bin/claude", nil
	}

	// Mock CommandContext
	CommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", `{"type":"result","session_id":"existing-session-789"}`)
	}

	// Start a conversation with an existing session ID
	config := ConversationConfig{
		SessionID:     "existing-session-789",
		InitialPrompt: "Continue from where we left off",
	}

	_, events, err := StartConversation(context.Background(), config)
	if err != nil {
		t.Fatalf("StartConversation failed: %v", err)
	}

	// Drain events
	for range events {
	}

	// Verify --resume flag is included with the session ID
	foundResume := false
	foundSessionID := false
	for i, arg := range capturedArgs {
		if arg == "--resume" {
			foundResume = true
			if i+1 < len(capturedArgs) && capturedArgs[i+1] == "existing-session-789" {
				foundSessionID = true
			}
		}
	}

	if !foundResume {
		t.Error("call with existing session should include --resume flag")
	}
	if !foundSessionID {
		t.Error("call with existing session should include the session ID after --resume")
	}
}

// TestConversationResume_VerifyAllRequiredFlags tests that all required flags
// are passed to the Claude CLI for a conversation.
func TestConversationResume_VerifyAllRequiredFlags(t *testing.T) {
	// Save original
	originalCommandContext := CommandContext
	originalLookPath := LookPath

	t.Cleanup(func() {
		CommandContext = originalCommandContext
		LookPath = originalLookPath
	})

	var capturedArgs []string

	// Mock LookPath
	LookPath = func(file string) (string, error) {
		return "/usr/bin/claude", nil
	}

	// Mock CommandContext
	CommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", `{"type":"result","session_id":"test"}`)
	}

	config := ConversationConfig{
		SessionID:     "test-session",
		InitialPrompt: "Test prompt",
	}

	_, events, err := StartConversation(context.Background(), config)
	if err != nil {
		t.Fatalf("StartConversation failed: %v", err)
	}

	// Drain events
	for range events {
	}

	// Verify all required flags are present
	requiredFlags := map[string]bool{
		"-p":                             false,
		"--output-format":                false,
		"--verbose":                      false,
		"--include-partial-messages":     false,
		"--dangerously-skip-permissions": false,
		"--resume":                       false,
	}

	argsStr := strings.Join(capturedArgs, " ")

	for flag := range requiredFlags {
		if strings.Contains(argsStr, flag) {
			requiredFlags[flag] = true
		}
	}

	for flag, found := range requiredFlags {
		if !found {
			t.Errorf("missing required flag: %s", flag)
		}
	}

	// Verify --output-format is "stream-json"
	for i, arg := range capturedArgs {
		if arg == "--output-format" && i+1 < len(capturedArgs) {
			if capturedArgs[i+1] != "stream-json" {
				t.Errorf("--output-format should be 'stream-json', got %q", capturedArgs[i+1])
			}
		}
	}
}

// TestConversationResume_NoSessionErrorsOnSend tests that SendMessage
// returns an error when there's no session ID.
func TestConversationResume_NoSessionErrorsOnSend(t *testing.T) {
	conv := &Conversation{
		sessionID: "", // No session ID
	}

	_, err := conv.SendMessage("test message")
	if err == nil {
		t.Error("expected error when sending message without session ID")
	}
	if !strings.Contains(err.Error(), "no session ID") {
		t.Errorf("error should mention 'no session ID', got: %v", err)
	}
}
