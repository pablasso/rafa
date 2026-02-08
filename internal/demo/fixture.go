package demo

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/pablasso/rafa/internal/plan"
)

const fixtureVersionV1 = 1

//go:embed fixtures/default.v1.json fixtures/create.default.v1.json
var embeddedFixtures embed.FS

type fixtureV1 struct {
	Version int `json:"version"`
	Plan    struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Tasks []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"tasks"`
	} `json:"plan"`
	Attempts []TaskAttempt `json:"attempts"`
}

// LoadDefaultDataset loads the embedded demo fixture.
func LoadDefaultDataset() (*Dataset, error) {
	data, err := embeddedFixtures.ReadFile("fixtures/default.v1.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded demo fixture: %w", err)
	}

	var fx fixtureV1
	if err := json.Unmarshal(data, &fx); err != nil {
		return nil, fmt.Errorf("parse embedded demo fixture: %w", err)
	}
	if fx.Version != fixtureVersionV1 {
		return nil, fmt.Errorf("unsupported demo fixture version %d", fx.Version)
	}

	p := &plan.Plan{
		ID:   fx.Plan.ID,
		Name: fx.Plan.Name,
	}
	for _, t := range fx.Plan.Tasks {
		p.Tasks = append(p.Tasks, plan.Task{
			ID:     t.ID,
			Title:  t.Title,
			Status: plan.TaskStatusPending,
		})
	}

	return &Dataset{
		Plan:     p,
		Attempts: fx.Attempts,
	}, nil
}
