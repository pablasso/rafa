package ai

import "os/exec"

// CommandContext is the function used to create exec.Cmd instances.
// It can be replaced in tests to mock command execution.
var CommandContext = exec.CommandContext

// LookPath is the function used to check for executable availability.
// It can be replaced in tests to mock command availability.
var LookPath = exec.LookPath

// IsClaudeAvailable checks if the claude command exists in PATH.
func IsClaudeAvailable() bool {
	_, err := LookPath("claude")
	return err == nil
}
