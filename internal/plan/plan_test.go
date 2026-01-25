package plan

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPlanSerialization(t *testing.T) {
	original := Plan{
		ID:          "plan-123",
		Name:        "Test Plan",
		Description: "A test plan description",
		SourceFile:  "/path/to/source.md",
		CreatedAt:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Status:      PlanStatusInProgress,
		Tasks: []Task{
			{
				ID:                 "task-1",
				Title:              "First Task",
				Description:        "Do the first thing",
				AcceptanceCriteria: []string{"Criterion 1", "Criterion 2"},
				Status:             TaskStatusPending,
				Attempts:           0,
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal plan: %v", err)
	}

	// Unmarshal back
	var restored Plan
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal plan: %v", err)
	}

	// Verify fields
	if restored.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", restored.Name, original.Name)
	}
	if restored.Description != original.Description {
		t.Errorf("Description mismatch: got %q, want %q", restored.Description, original.Description)
	}
	if restored.SourceFile != original.SourceFile {
		t.Errorf("SourceFile mismatch: got %q, want %q", restored.SourceFile, original.SourceFile)
	}
	if !restored.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", restored.CreatedAt, original.CreatedAt)
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: got %q, want %q", restored.Status, original.Status)
	}
	if len(restored.Tasks) != len(original.Tasks) {
		t.Errorf("Tasks length mismatch: got %d, want %d", len(restored.Tasks), len(original.Tasks))
	}
}

func TestTaskSerialization(t *testing.T) {
	original := Task{
		ID:                 "task-456",
		Title:              "Test Task",
		Description:        "A test task description",
		AcceptanceCriteria: []string{"AC 1", "AC 2", "AC 3"},
		Status:             TaskStatusCompleted,
		Attempts:           2,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal task: %v", err)
	}

	// Unmarshal back
	var restored Task
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal task: %v", err)
	}

	// Verify fields
	if restored.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Title != original.Title {
		t.Errorf("Title mismatch: got %q, want %q", restored.Title, original.Title)
	}
	if restored.Description != original.Description {
		t.Errorf("Description mismatch: got %q, want %q", restored.Description, original.Description)
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: got %q, want %q", restored.Status, original.Status)
	}
	if restored.Attempts != original.Attempts {
		t.Errorf("Attempts mismatch: got %d, want %d", restored.Attempts, original.Attempts)
	}
	if len(restored.AcceptanceCriteria) != len(original.AcceptanceCriteria) {
		t.Errorf("AcceptanceCriteria length mismatch: got %d, want %d",
			len(restored.AcceptanceCriteria), len(original.AcceptanceCriteria))
	}
	for i, ac := range original.AcceptanceCriteria {
		if restored.AcceptanceCriteria[i] != ac {
			t.Errorf("AcceptanceCriteria[%d] mismatch: got %q, want %q",
				i, restored.AcceptanceCriteria[i], ac)
		}
	}
}

func TestTimestampFormat(t *testing.T) {
	plan := Plan{
		ID:        "plan-123",
		CreatedAt: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
		Status:    PlanStatusNotStarted,
		Tasks:     []Task{},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("failed to marshal plan: %v", err)
	}

	// Check that the timestamp is in RFC3339/ISO-8601 format
	jsonStr := string(data)
	expectedTimestamp := `"createdAt":"2024-01-15T10:30:45Z"`
	if !strings.Contains(jsonStr, expectedTimestamp) {
		t.Errorf("timestamp not in expected ISO-8601 format\ngot: %s\nwant to contain: %s",
			jsonStr, expectedTimestamp)
	}
}

func TestTaskExtractionResultValidate(t *testing.T) {
	tests := []struct {
		name    string
		result  TaskExtractionResult
		wantErr string
	}{
		{
			name: "valid result passes",
			result: TaskExtractionResult{
				Name:        "Test Plan",
				Description: "A description",
				Tasks: []ExtractedTask{
					{
						Title:              "Task 1",
						Description:        "Do something",
						AcceptanceCriteria: []string{"It works"},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "empty tasks returns error",
			result: TaskExtractionResult{
				Name:        "Test Plan",
				Description: "A description",
				Tasks:       []ExtractedTask{},
			},
			wantErr: "no tasks extracted",
		},
		{
			name: "nil tasks returns error",
			result: TaskExtractionResult{
				Name:        "Test Plan",
				Description: "A description",
				Tasks:       nil,
			},
			wantErr: "no tasks extracted",
		},
		{
			name: "task without title returns error",
			result: TaskExtractionResult{
				Name: "Test Plan",
				Tasks: []ExtractedTask{
					{
						Title:              "",
						Description:        "Do something",
						AcceptanceCriteria: []string{"It works"},
					},
				},
			},
			wantErr: "task 1 missing title",
		},
		{
			name: "task without acceptance criteria returns error",
			result: TaskExtractionResult{
				Name: "Test Plan",
				Tasks: []ExtractedTask{
					{
						Title:              "Task 1",
						Description:        "Do something",
						AcceptanceCriteria: []string{},
					},
				},
			},
			wantErr: "task 1 (Task 1) missing acceptance criteria",
		},
		{
			name: "empty name is allowed",
			result: TaskExtractionResult{
				Name:        "",
				Description: "A description",
				Tasks: []ExtractedTask{
					{
						Title:              "Task 1",
						Description:        "Do something",
						AcceptanceCriteria: []string{"It works"},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "second task validation error has correct index",
			result: TaskExtractionResult{
				Name: "Test Plan",
				Tasks: []ExtractedTask{
					{
						Title:              "Task 1",
						Description:        "Do something",
						AcceptanceCriteria: []string{"It works"},
					},
					{
						Title:              "Task 2",
						Description:        "Do something else",
						AcceptanceCriteria: nil,
					},
				},
			},
			wantErr: "task 2 (Task 2) missing acceptance criteria",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}
