package demo

import (
	"fmt"
	"strings"
)

// Mode controls which TUI surface demo mode starts on.
type Mode string

const (
	ModeRun    Mode = "run"
	ModeCreate Mode = "create"
)

// ParseMode validates and normalizes a demo mode value.
func ParseMode(value string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case ModeRun, ModeCreate:
		return Mode(strings.ToLower(strings.TrimSpace(value))), nil
	default:
		return "", fmt.Errorf("invalid demo mode %q (valid: run, create)", value)
	}
}
