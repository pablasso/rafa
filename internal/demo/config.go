package demo

import "time"

// Speed controls playback pacing.
type Speed string

const (
	SpeedFast   Speed = "fast"
	SpeedNormal Speed = "normal"
	SpeedSlow   Speed = "slow"
)

// Config controls demo playback behavior.
type Config struct {
	Speed            Speed
	LineDelay        time.Duration
	TaskDelay        time.Duration
	MaxTasks         int
	MaxEventsPerTask int
}

// DefaultConfig returns a fast, TUI-friendly demo configuration.
func DefaultConfig() Config {
	return Config{
		Speed:            SpeedFast,
		LineDelay:        12 * time.Millisecond,
		TaskDelay:        400 * time.Millisecond,
		MaxTasks:         5,
		MaxEventsPerTask: 450,
	}
}

// WithDefaults fills zero-value config fields with defaults.
func (c Config) WithDefaults() Config {
	defaults := DefaultConfig()
	if c.Speed == "" {
		c.Speed = defaults.Speed
	}
	if c.LineDelay == 0 {
		c.LineDelay = defaults.LineDelay
	}
	if c.TaskDelay == 0 {
		c.TaskDelay = defaults.TaskDelay
	}
	if c.MaxTasks == 0 {
		c.MaxTasks = defaults.MaxTasks
	}
	if c.MaxEventsPerTask == 0 {
		c.MaxEventsPerTask = defaults.MaxEventsPerTask
	}
	return c
}
