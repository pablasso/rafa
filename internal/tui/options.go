package tui

import "github.com/pablasso/rafa/internal/demo"

// Options configures TUI startup behavior.
type Options struct {
	Demo *DemoOptions
}

// DemoOptions configure demo mode when starting the TUI.
type DemoOptions struct {
	Mode     demo.Mode
	Preset   demo.Preset
	Scenario demo.Scenario
}
