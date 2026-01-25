package ai

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    string
		wantErr bool
	}{
		{
			name:    "clean JSON",
			input:   []byte(`{"name":"test","tasks":[]}`),
			want:    `{"name":"test","tasks":[]}`,
			wantErr: false,
		},
		{
			name:    "JSON with leading text",
			input:   []byte(`Here is the result: {"name":"test","tasks":[]}`),
			want:    `{"name":"test","tasks":[]}`,
			wantErr: false,
		},
		{
			name:    "JSON with trailing text",
			input:   []byte(`{"name":"test","tasks":[]} Hope this helps!`),
			want:    `{"name":"test","tasks":[]}`,
			wantErr: false,
		},
		{
			name:    "JSON with leading and trailing text",
			input:   []byte(`Here you go: {"name":"test","tasks":[]} Let me know if you need anything else.`),
			want:    `{"name":"test","tasks":[]}`,
			wantErr: false,
		},
		{
			name:    "markdown-wrapped JSON",
			input:   []byte("```json\n" + `{"name":"test","tasks":[]}` + "\n```"),
			want:    `{"name":"test","tasks":[]}`,
			wantErr: false,
		},
		{
			name:    "nested JSON object",
			input:   []byte(`{"name":"test","tasks":[{"title":"Task 1","description":"Do something","acceptanceCriteria":["test passes"]}]}`),
			want:    `{"name":"test","tasks":[{"title":"Task 1","description":"Do something","acceptanceCriteria":["test passes"]}]}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   []byte(`{"name":"test"`),
			wantErr: true,
		},
		{
			name:    "no JSON",
			input:   []byte(`This is just plain text with no JSON`),
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   []byte{},
			wantErr: true,
		},
		{
			name:    "only braces without valid JSON",
			input:   []byte(`{invalid json content}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("extractJSON() = %s, want %s", string(got), tt.want)
			}
		})
	}
}

func TestBuildExtractionPrompt(t *testing.T) {
	designContent := "# Test Design\n\nThis is a test design document."
	prompt := buildExtractionPrompt(designContent)

	// Verify prompt includes design content
	if !strings.Contains(prompt, designContent) {
		t.Error("prompt should include design content")
	}

	// Verify prompt specifies JSON output
	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should specify JSON output")
	}

	// Verify prompt includes task guidelines
	if !strings.Contains(prompt, "TASK GUIDELINES") {
		t.Error("prompt should include task guidelines")
	}

	// Verify prompt includes the required JSON structure fields
	requiredFields := []string{"name", "description", "tasks", "title", "acceptanceCriteria"}
	for _, field := range requiredFields {
		if !strings.Contains(prompt, field) {
			t.Errorf("prompt should include field %q", field)
		}
	}

	// Verify prompt includes context window guidance
	if !strings.Contains(prompt, "50-60%") {
		t.Error("prompt should include context window sizing guidance")
	}
}

func TestIsClaudeAvailable(t *testing.T) {
	// Just verify it runs without panic
	// The actual result depends on whether claude is installed
	_ = IsClaudeAvailable()
}
