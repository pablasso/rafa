package util

import (
	"regexp"
	"testing"
)

func TestGenerateShortID(t *testing.T) {
	t.Run("length is always 6", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id, err := GenerateShortID()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(id) != 6 {
				t.Errorf("expected length 6, got %d for id %q", len(id), id)
			}
		}
	})

	t.Run("contains only alphanumeric characters", func(t *testing.T) {
		pattern := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
		for i := 0; i < 100; i++ {
			id, err := GenerateShortID()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !pattern.MatchString(id) {
				t.Errorf("id %q contains non-alphanumeric characters", id)
			}
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id, err := GenerateShortID()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if seen[id] {
				t.Errorf("duplicate id generated: %q", id)
			}
			seen[id] = true
		}
	})
}

func TestGenerateTaskID(t *testing.T) {
	tests := []struct {
		index    int
		expected string
	}{
		{0, "t01"},
		{1, "t02"},
		{9, "t10"},
		{98, "t99"},
		{99, "t100"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := GenerateTaskID(tc.index)
			if result != tc.expected {
				t.Errorf("GenerateTaskID(%d) = %q, want %q", tc.index, result, tc.expected)
			}
		})
	}
}

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"hello_world", "hello-world"},
		{"Hello---World", "hello-world"},
		{"  Hello  ", "hello"},
		{"Feature: Auth!", "feature-auth"},
		{"", ""},
		{"already-kebab", "already-kebab"},
		{"MixedCase_And Spaces", "mixedcase-and-spaces"},
		{"123 Numbers 456", "123-numbers-456"},
		{"---leading-trailing---", "leading-trailing"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := toKebabCase(tc.input)
			if result != tc.expected {
				t.Errorf("toKebabCase(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
