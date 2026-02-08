package demo

import (
	"encoding/json"
	"fmt"

	"github.com/pablasso/rafa/internal/ai"
)

const createFixtureVersionV1 = 1

type createFixtureV1 struct {
	Version    int               `json:"version"`
	SourceFile string            `json:"sourceFile"`
	Events     []ai.StreamEvent  `json:"events"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// LoadDefaultCreateDataset loads the embedded create-plan demo fixture.
func LoadDefaultCreateDataset() (*CreateDataset, error) {
	data, err := embeddedFixtures.ReadFile("fixtures/create.default.v1.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded create demo fixture: %w", err)
	}

	var fx createFixtureV1
	if err := json.Unmarshal(data, &fx); err != nil {
		return nil, fmt.Errorf("parse embedded create demo fixture: %w", err)
	}
	if fx.Version != createFixtureVersionV1 {
		return nil, fmt.Errorf("unsupported create demo fixture version %d", fx.Version)
	}
	if len(fx.Events) == 0 {
		return nil, fmt.Errorf("create demo fixture has no events")
	}

	sourceFile := fx.SourceFile
	if sourceFile == "" {
		sourceFile = "docs/designs/demo-mode-reborn.md"
	}

	events := make([]ai.StreamEvent, len(fx.Events))
	copy(events, fx.Events)

	return &CreateDataset{
		SourceFile: sourceFile,
		Events:     events,
	}, nil
}
