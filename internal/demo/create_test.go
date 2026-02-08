package demo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/ai"
)

func TestLoadDefaultCreateDataset(t *testing.T) {
	ds, err := LoadDefaultCreateDataset()
	if err != nil {
		t.Fatalf("LoadDefaultCreateDataset() error = %v", err)
	}
	if ds.SourceFile == "" {
		t.Fatalf("expected source file")
	}
	if len(ds.Events) == 0 {
		t.Fatalf("expected fixture events")
	}

	var hasMarker bool
	for _, ev := range ds.Events {
		if ev.Type == "text" && strings.Contains(ev.Text, "PLAN_APPROVED_JSON:") {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		t.Fatalf("expected fixture to contain PLAN_APPROVED_JSON marker")
	}
}

func TestNewCreateReplayConfigByPreset(t *testing.T) {
	ds := &CreateDataset{
		SourceFile: "docs/designs/example.md",
		Events: []ai.StreamEvent{
			{Type: "init"},
			{Type: "text", Text: "x"},
			{Type: "done"},
		},
	}

	quick, err := NewCreateReplayConfig(PresetQuick, ds)
	if err != nil {
		t.Fatalf("quick config error = %v", err)
	}
	medium, err := NewCreateReplayConfig(PresetMedium, ds)
	if err != nil {
		t.Fatalf("medium config error = %v", err)
	}
	slow, err := NewCreateReplayConfig(PresetSlow, ds)
	if err != nil {
		t.Fatalf("slow config error = %v", err)
	}

	if quick.EventDelay <= 0 || medium.EventDelay <= 0 || slow.EventDelay <= 0 {
		t.Fatalf("expected positive delays, got quick=%s medium=%s slow=%s", quick.EventDelay, medium.EventDelay, slow.EventDelay)
	}
	if !(quick.EventDelay <= medium.EventDelay && medium.EventDelay <= slow.EventDelay) {
		t.Fatalf("expected quick<=medium<=slow delays, got quick=%s medium=%s slow=%s", quick.EventDelay, medium.EventDelay, slow.EventDelay)
	}
}

func TestCreateReplayStarter_Start(t *testing.T) {
	ds := &CreateDataset{
		SourceFile: "docs/designs/example.md",
		Events: []ai.StreamEvent{
			{Type: "init", SessionID: "a"},
			{Type: "text", Text: "hello"},
			{Type: "done", SessionID: "a"},
		},
	}
	starter := NewCreateReplayStarter(ds, CreateReplayConfig{
		Preset:     PresetQuick,
		EventDelay: 1 * time.Millisecond,
	})

	_, events, err := starter.Start(context.Background(), ai.ConversationConfig{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	var got []ai.StreamEvent
	for ev := range events {
		got = append(got, ev)
	}
	if len(got) != len(ds.Events) {
		t.Fatalf("expected %d events, got %d", len(ds.Events), len(got))
	}
	for i := range got {
		if got[i].Type != ds.Events[i].Type {
			t.Fatalf("event %d type = %q, want %q", i, got[i].Type, ds.Events[i].Type)
		}
	}
}

func TestFallbackCreateDatasetHasMarker(t *testing.T) {
	ds := FallbackCreateDataset()
	var hasMarker bool
	for _, ev := range ds.Events {
		if ev.Type == "text" && strings.Contains(ev.Text, "PLAN_APPROVED_JSON:") {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		t.Fatalf("expected fallback create dataset to include PLAN_APPROVED_JSON marker")
	}
}
