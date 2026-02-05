package demo

import (
	"fmt"
	"strings"
	"time"
)

// Preset controls demo playback pacing via a target duration.
type Preset string

const (
	PresetQuick  Preset = "quick"
	PresetMedium Preset = "medium"
	PresetSlow   Preset = "slow"
)

func ParsePreset(value string) (Preset, error) {
	switch Preset(strings.ToLower(strings.TrimSpace(value))) {
	case PresetQuick, PresetMedium, PresetSlow:
		return Preset(strings.ToLower(strings.TrimSpace(value))), nil
	default:
		return "", fmt.Errorf("invalid demo preset %q (valid: quick, medium, slow)", value)
	}
}

type presetSettings struct {
	TargetDuration   time.Duration
	MaxTasks         int
	MaxEventsPerTask int
}

func settingsForPreset(preset Preset) (presetSettings, error) {
	switch preset {
	case PresetQuick:
		return presetSettings{
			TargetDuration:   time.Minute,
			MaxTasks:         5,
			MaxEventsPerTask: 450,
		}, nil
	case PresetMedium:
		return presetSettings{
			TargetDuration:   30 * time.Minute,
			MaxTasks:         10,
			MaxEventsPerTask: 450,
		}, nil
	case PresetSlow:
		return presetSettings{
			TargetDuration:   2 * time.Hour,
			MaxTasks:         0, // all tasks in the dataset
			MaxEventsPerTask: 450,
		}, nil
	default:
		return presetSettings{}, fmt.Errorf("unknown demo preset %q", preset)
	}
}

func MaxTasksForPreset(preset Preset) (int, error) {
	settings, err := settingsForPreset(preset)
	if err != nil {
		return 0, err
	}
	return settings.MaxTasks, nil
}

// Config controls demo playback behavior.
type Config struct {
	Preset           Preset
	LineDelay        time.Duration
	TaskDelay        time.Duration
	MaxTasks         int
	MaxEventsPerTask int
}

var (
	baseLineDelay = 12 * time.Millisecond
	baseTaskDelay = 400 * time.Millisecond

	minLineDelay = 1 * time.Millisecond
	maxLineDelay = 2 * time.Second

	minTaskDelay = 0 * time.Second
	maxTaskDelay = 60 * time.Second
)

// NewConfig computes a demo playback config for a preset, scaled to the dataset.
//
// The dataset should already reflect the selected scenario and task limit.
func NewConfig(preset Preset, dataset *Dataset) (Config, error) {
	settings, err := settingsForPreset(preset)
	if err != nil {
		return Config{}, err
	}
	if dataset == nil || dataset.Plan == nil {
		return Config{}, fmt.Errorf("nil dataset")
	}

	taskCount, eventCount := estimateCounts(dataset, settings.MaxEventsPerTask)
	if taskCount == 0 || eventCount == 0 {
		return Config{
			Preset:           preset,
			LineDelay:        clampDuration(baseLineDelay, minLineDelay, maxLineDelay),
			TaskDelay:        clampDuration(baseTaskDelay, minTaskDelay, maxTaskDelay),
			MaxTasks:         taskCount,
			MaxEventsPerTask: settings.MaxEventsPerTask,
		}, nil
	}

	base := time.Duration(eventCount)*baseLineDelay + time.Duration(taskCount)*baseTaskDelay
	if base <= 0 {
		base = baseLineDelay
	}
	scale := float64(settings.TargetDuration) / float64(base)
	lineDelay := time.Duration(float64(baseLineDelay) * scale)
	taskDelay := time.Duration(float64(baseTaskDelay) * scale)

	return Config{
		Preset:           preset,
		LineDelay:        clampDuration(lineDelay, minLineDelay, maxLineDelay),
		TaskDelay:        clampDuration(taskDelay, minTaskDelay, maxTaskDelay),
		MaxTasks:         taskCount,
		MaxEventsPerTask: settings.MaxEventsPerTask,
	}, nil
}

func clampDuration(value, min, max time.Duration) time.Duration {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func estimateCounts(dataset *Dataset, maxEventsPerTask int) (tasks int, events int) {
	if dataset == nil || dataset.Plan == nil {
		return 0, 0
	}
	if maxEventsPerTask <= 0 {
		maxEventsPerTask = 450
	}

	attemptsByTask := groupAttempts(dataset.Attempts)
	for _, task := range dataset.Plan.Tasks {
		tasks++
		taskAttempts := attemptsByTask[task.ID]
		if len(taskAttempts) == 0 {
			events++
			continue
		}
		for _, attempt := range taskAttempts {
			events += minInt(len(attempt.Events), maxEventsPerTask)
		}
	}
	return tasks, events
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
