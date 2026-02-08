package demo

import (
	"context"
	"fmt"
	"time"

	"github.com/pablasso/rafa/internal/ai"
)

// CreateDataset holds replay data for the create-plan demo surface.
type CreateDataset struct {
	SourceFile string
	Events     []ai.StreamEvent
}

// CreateReplayConfig controls pacing for create-plan replay events.
type CreateReplayConfig struct {
	Preset     Preset
	EventDelay time.Duration
}

var (
	createQuickTarget  = 20 * time.Second
	createMediumTarget = 90 * time.Second
	createSlowTarget   = 5 * time.Minute

	createMinDelay = 5 * time.Millisecond
	createMaxDelay = 3 * time.Second
)

// NewCreateReplayConfig computes replay pacing based on preset and dataset size.
func NewCreateReplayConfig(preset Preset, dataset *CreateDataset) (CreateReplayConfig, error) {
	if dataset == nil {
		return CreateReplayConfig{}, fmt.Errorf("nil create dataset")
	}

	target, err := createTargetDurationForPreset(preset)
	if err != nil {
		return CreateReplayConfig{}, err
	}

	eventCount := len(dataset.Events)
	if eventCount == 0 {
		return CreateReplayConfig{
			Preset:     preset,
			EventDelay: createMinDelay,
		}, nil
	}

	delay := time.Duration(int64(target) / int64(eventCount))
	return CreateReplayConfig{
		Preset:     preset,
		EventDelay: clampDuration(delay, createMinDelay, createMaxDelay),
	}, nil
}

func createTargetDurationForPreset(preset Preset) (time.Duration, error) {
	switch preset {
	case PresetQuick:
		return createQuickTarget, nil
	case PresetMedium:
		return createMediumTarget, nil
	case PresetSlow:
		return createSlowTarget, nil
	default:
		return 0, fmt.Errorf("unknown demo preset %q", preset)
	}
}

// CreateReplayStarter replays fixture events instead of launching Claude.
type CreateReplayStarter struct {
	dataset *CreateDataset
	config  CreateReplayConfig
}

// NewCreateReplayStarter creates a conversation starter for create-plan demo replay.
func NewCreateReplayStarter(dataset *CreateDataset, config CreateReplayConfig) *CreateReplayStarter {
	return &CreateReplayStarter{
		dataset: dataset,
		config:  config,
	}
}

// Start implements the plan-create ConversationStarter interface.
func (s *CreateReplayStarter) Start(ctx context.Context, _ ai.ConversationConfig) (*ai.Conversation, <-chan ai.StreamEvent, error) {
	if s == nil || s.dataset == nil {
		return nil, nil, fmt.Errorf("nil create replay starter dataset")
	}

	events := make(chan ai.StreamEvent, 100)
	replayEvents := make([]ai.StreamEvent, len(s.dataset.Events))
	copy(replayEvents, s.dataset.Events)
	delay := s.config.EventDelay
	if delay <= 0 {
		delay = createMinDelay
	}

	go func() {
		defer close(events)

		for i := range replayEvents {
			select {
			case <-ctx.Done():
				return
			case events <- replayEvents[i]:
			}

			if i < len(replayEvents)-1 && !waitForReplay(ctx, delay) {
				return
			}
		}
	}()

	return nil, events, nil
}

func waitForReplay(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// FallbackCreateDataset provides a tiny create-plan replay when fixture loading fails.
func FallbackCreateDataset() *CreateDataset {
	return &CreateDataset{
		SourceFile: "docs/designs/demo-mode-reborn.md",
		Events: []ai.StreamEvent{
			{Type: "init", SessionID: "demo-create-fallback"},
			{Type: "tool_use", ToolName: "Read", ToolTarget: "docs/designs/demo-mode-reborn.md"},
			{Type: "tool_result"},
			{Type: "text", Text: "Analyzing the design and extracting implementation tasks.\n\nPLAN_APPROVED_JSON:\n{\n  \"name\": \"demo-create-plan\",\n  \"description\": \"Fallback extracted plan\",\n  \"tasks\": [\n    {\n      \"title\": \"Identify scope\",\n      \"description\": \"Summarize the implementation scope from the design.\",\n      \"acceptanceCriteria\": [\"Scope summary is complete\"]\n    },\n    {\n      \"title\": \"Draft implementation tasks\",\n      \"description\": \"Break scope into sequenced tasks.\",\n      \"acceptanceCriteria\": [\"Tasks are ordered and actionable\"]\n    }\n  ]\n}\n"},
			{Type: "done", SessionID: "demo-create-fallback"},
		},
	}
}
