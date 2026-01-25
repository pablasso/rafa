package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablasso/rafa/internal/plan"
)

func TestAnalyzeRetries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plan with tasks
	p := &plan.Plan{
		ID: "test-plan",
		Tasks: []plan.Task{
			{ID: "t01", Title: "First task"},
			{ID: "t02", Title: "Second task"},
		},
	}

	analyzer := NewAnalyzer(tmpDir, p)

	// Test events where t01 required 3 attempts
	events := []plan.ProgressEvent{
		{Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(1)}},
		{Event: plan.EventTaskFailed, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(1)}},
		{Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(2)}},
		{Event: plan.EventTaskFailed, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(2)}},
		{Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(3)}},
		{Event: plan.EventTaskCompleted, Data: map[string]interface{}{"task_id": "t01"}},
		{Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t02", "attempt": float64(1)}},
		{Event: plan.EventTaskCompleted, Data: map[string]interface{}{"task_id": "t02"}},
	}

	suggestions := analyzer.analyzeRetries(events)

	// Should have 1 suggestion for t01 (required >1 attempt)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}

	s := suggestions[0]
	if s.Category != "Common Issues" {
		t.Errorf("expected category 'Common Issues', got '%s'", s.Category)
	}

	if !strings.Contains(s.Title, "First task") {
		t.Errorf("expected title to contain 'First task', got '%s'", s.Title)
	}

	if !strings.Contains(s.Title, "3 attempts") {
		t.Errorf("expected title to contain '3 attempts', got '%s'", s.Title)
	}
}

func TestAnalyzeRetries_NoRetries(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{
		ID: "test-plan",
		Tasks: []plan.Task{
			{ID: "t01", Title: "First task"},
		},
	}

	analyzer := NewAnalyzer(tmpDir, p)

	// All tasks completed on first attempt
	events := []plan.ProgressEvent{
		{Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(1)}},
		{Event: plan.EventTaskCompleted, Data: map[string]interface{}{"task_id": "t01"}},
	}

	suggestions := analyzer.analyzeRetries(events)

	if len(suggestions) != 0 {
		t.Fatalf("expected 0 suggestions for single-attempt tasks, got %d", len(suggestions))
	}
}

func TestAnalyzeFailurePatterns_Tests(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{ID: "test-plan"}
	analyzer := NewAnalyzer(tmpDir, p)

	// Multiple test failure lines (>2)
	lines := []string{
		"Running tests...",
		"FAIL test_something",
		"test failed: expected 1, got 2",
		"another TEST FAILure here",
		"all done",
	}

	suggestions := analyzer.analyzeFailurePatterns(lines)

	// Should have 1 suggestion for test failures
	var testSuggestion *Suggestion
	for i, s := range suggestions {
		if s.Category == "Testing" {
			testSuggestion = &suggestions[i]
			break
		}
	}

	if testSuggestion == nil {
		t.Fatal("expected a Testing category suggestion")
	}

	if !strings.Contains(testSuggestion.Title, "test failures") {
		t.Errorf("expected title about test failures, got '%s'", testSuggestion.Title)
	}

	if testSuggestion.Example == "" {
		t.Error("expected an example for test failures")
	}
}

func TestAnalyzeFailurePatterns_Tests_NotEnough(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{ID: "test-plan"}
	analyzer := NewAnalyzer(tmpDir, p)

	// Only 2 test failures (threshold is >2)
	lines := []string{
		"test failed once",
		"test failed twice",
	}

	suggestions := analyzer.analyzeFailurePatterns(lines)

	for _, s := range suggestions {
		if s.Category == "Testing" {
			t.Fatal("should not generate testing suggestion for only 2 failures")
		}
	}
}

func TestAnalyzeFailurePatterns_Formatting(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{ID: "test-plan"}
	analyzer := NewAnalyzer(tmpDir, p)

	// Formatting issue lines
	lines := []string{
		"Running formatter...",
		"fmt check failed",
		"all done",
	}

	suggestions := analyzer.analyzeFailurePatterns(lines)

	var fmtSuggestion *Suggestion
	for i, s := range suggestions {
		if s.Category == "Formatting" {
			fmtSuggestion = &suggestions[i]
			break
		}
	}

	if fmtSuggestion == nil {
		t.Fatal("expected a Formatting category suggestion")
	}

	if !strings.Contains(fmtSuggestion.Title, "Formatting issues") {
		t.Errorf("expected title about formatting issues, got '%s'", fmtSuggestion.Title)
	}
}

func TestAnalyzeFailurePatterns_Dependencies(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{ID: "test-plan"}
	analyzer := NewAnalyzer(tmpDir, p)

	// Module not found error
	lines := []string{
		"go: cannot find module providing package github.com/example/missing",
		"all done",
	}

	suggestions := analyzer.analyzeFailurePatterns(lines)

	var depSuggestion *Suggestion
	for i, s := range suggestions {
		if s.Category == "Dependencies" {
			depSuggestion = &suggestions[i]
			break
		}
	}

	if depSuggestion == nil {
		t.Fatal("expected a Dependencies category suggestion")
	}

	if !strings.Contains(depSuggestion.Title, "dependency") {
		t.Errorf("expected title about dependency issues, got '%s'", depSuggestion.Title)
	}
}

func TestAnalyzeSuccessPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a plan with tasks that reference verification commands
	p := &plan.Plan{
		ID: "test-plan",
		Tasks: []plan.Task{
			{
				ID:                 "t01",
				Title:              "First task",
				AcceptanceCriteria: []string{"make test passes", "code compiles"},
			},
			{
				ID:                 "t02",
				Title:              "Second task",
				AcceptanceCriteria: []string{"make test passes", "make fmt passes"},
			},
			{
				ID:                 "t03",
				Title:              "Third task",
				AcceptanceCriteria: []string{"go build succeeds"},
			},
		},
	}

	analyzer := NewAnalyzer(tmpDir, p)

	// Plan completed event
	events := []plan.ProgressEvent{
		{Event: plan.EventPlanCompleted, Data: map[string]interface{}{}},
	}

	suggestions := analyzer.analyzeSuccessPatterns(events, nil)

	// Should suggest documenting "make test" (appears in 2 tasks)
	var makeTestSuggestion *Suggestion
	for i, s := range suggestions {
		if strings.Contains(s.Title, "make test") {
			makeTestSuggestion = &suggestions[i]
			break
		}
	}

	if makeTestSuggestion == nil {
		t.Fatal("expected a suggestion for 'make test'")
	}

	if makeTestSuggestion.Category != "Verification" {
		t.Errorf("expected category 'Verification', got '%s'", makeTestSuggestion.Category)
	}
}

func TestAnalyzeSuccessPatterns_NoPlanCompleted(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{
		ID: "test-plan",
		Tasks: []plan.Task{
			{
				ID:                 "t01",
				Title:              "First task",
				AcceptanceCriteria: []string{"make test passes"},
			},
			{
				ID:                 "t02",
				Title:              "Second task",
				AcceptanceCriteria: []string{"make test passes"},
			},
		},
	}

	analyzer := NewAnalyzer(tmpDir, p)

	// No plan_completed event
	events := []plan.ProgressEvent{
		{Event: plan.EventTaskStarted, Data: map[string]interface{}{}},
	}

	suggestions := analyzer.analyzeSuccessPatterns(events, nil)

	if len(suggestions) != 0 {
		t.Fatalf("expected 0 suggestions without plan completion, got %d", len(suggestions))
	}
}

func TestDeduplicate(t *testing.T) {
	suggestions := []Suggestion{
		{Category: "Testing", Title: "Test failures", Description: "First"},
		{Category: "Testing", Title: "Test failures", Description: "Duplicate"},
		{Category: "Formatting", Title: "Format issues", Description: "Unique"},
		{Category: "Testing", Title: "Different title", Description: "Also unique"},
	}

	result := deduplicate(suggestions)

	if len(result) != 3 {
		t.Fatalf("expected 3 unique suggestions, got %d", len(result))
	}

	// Verify first one wins
	for _, s := range result {
		if s.Category == "Testing" && s.Title == "Test failures" {
			if s.Description != "First" {
				t.Errorf("expected first occurrence to be kept, got '%s'", s.Description)
			}
		}
	}
}

func TestDeduplicate_Empty(t *testing.T) {
	result := deduplicate(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d", len(result))
	}
}

func TestFormatSuggestions(t *testing.T) {
	suggestions := []Suggestion{
		{
			Category:    "Testing",
			Title:       "Test failures observed",
			Description: "Run tests before committing",
			Example:     "make test",
		},
		{
			Category:    "Formatting",
			Title:       "Format issues",
			Description: "Format code",
		},
		{
			Category:    "Testing",
			Title:       "Another test suggestion",
			Description: "More testing info",
		},
	}

	output := FormatSuggestions(suggestions)

	// Check header
	if !strings.Contains(output, "SUGGESTED AGENTS.md ADDITIONS") {
		t.Error("output should contain header")
	}

	// Check category headers
	if !strings.Contains(output, "## Testing") {
		t.Error("output should contain Testing category header")
	}
	if !strings.Contains(output, "## Formatting") {
		t.Error("output should contain Formatting category header")
	}

	// Check suggestions are included
	if !strings.Contains(output, "Test failures observed") {
		t.Error("output should contain suggestion title")
	}
	if !strings.Contains(output, "Run tests before committing") {
		t.Error("output should contain suggestion description")
	}

	// Check example is included
	if !strings.Contains(output, "make test") {
		t.Error("output should contain example")
	}

	// Check footer
	if !strings.Contains(output, "Review these suggestions") {
		t.Error("output should contain footer")
	}
}

func TestFormatSuggestions_Empty(t *testing.T) {
	output := FormatSuggestions(nil)
	if output != "" {
		t.Errorf("expected empty output for nil suggestions, got '%s'", output)
	}

	output = FormatSuggestions([]Suggestion{})
	if output != "" {
		t.Errorf("expected empty output for empty suggestions, got '%s'", output)
	}
}

func TestFormatSuggestions_CategoryOrder(t *testing.T) {
	// Categories should be sorted alphabetically
	suggestions := []Suggestion{
		{Category: "Zebra", Title: "Z title", Description: "Z desc"},
		{Category: "Alpha", Title: "A title", Description: "A desc"},
		{Category: "Middle", Title: "M title", Description: "M desc"},
	}

	output := FormatSuggestions(suggestions)

	alphaIdx := strings.Index(output, "## Alpha")
	middleIdx := strings.Index(output, "## Middle")
	zebraIdx := strings.Index(output, "## Zebra")

	if alphaIdx > middleIdx || middleIdx > zebraIdx {
		t.Error("categories should be sorted alphabetically")
	}
}

func TestAnalyzer_Analyze_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create progress.log
	progressEvents := []plan.ProgressEvent{
		{Timestamp: time.Now(), Event: plan.EventPlanStarted, Data: map[string]interface{}{"plan_id": "test"}},
		{Timestamp: time.Now(), Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(1)}},
		{Timestamp: time.Now(), Event: plan.EventTaskFailed, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(1)}},
		{Timestamp: time.Now(), Event: plan.EventTaskStarted, Data: map[string]interface{}{"task_id": "t01", "attempt": float64(2)}},
		{Timestamp: time.Now(), Event: plan.EventTaskCompleted, Data: map[string]interface{}{"task_id": "t01"}},
		{Timestamp: time.Now(), Event: plan.EventPlanCompleted, Data: map[string]interface{}{}},
	}

	progressPath := filepath.Join(tmpDir, "progress.log")
	f, err := os.Create(progressPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range progressEvents {
		data, _ := json.Marshal(e)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	// Create output.log with some failures
	outputPath := filepath.Join(tmpDir, "output.log")
	outputLines := []string{
		"Starting execution...",
		"FAIL test_something",
		"test failed again",
		"TEST FAIL more",
		"format check error",
	}
	os.WriteFile(outputPath, []byte(strings.Join(outputLines, "\n")), 0644)

	// Create plan
	p := &plan.Plan{
		ID: "test",
		Tasks: []plan.Task{
			{
				ID:                 "t01",
				Title:              "First task",
				AcceptanceCriteria: []string{"make test passes", "make fmt passes"},
			},
		},
	}

	analyzer := NewAnalyzer(tmpDir, p)
	suggestions, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have multiple suggestions
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion")
	}

	// Verify deduplication worked
	seen := make(map[string]bool)
	for _, s := range suggestions {
		key := s.Category + ":" + s.Title
		if seen[key] {
			t.Errorf("duplicate suggestion found: %s", key)
		}
		seen[key] = true
	}
}

func TestAnalyzer_Analyze_NoProgressLog(t *testing.T) {
	tmpDir := t.TempDir()

	p := &plan.Plan{ID: "test"}
	analyzer := NewAnalyzer(tmpDir, p)

	_, err := analyzer.Analyze()
	if err == nil {
		t.Fatal("expected error when progress.log doesn't exist")
	}
}

func TestAnalyzer_Analyze_NoOutputLog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create progress.log only
	progressEvents := []plan.ProgressEvent{
		{Timestamp: time.Now(), Event: plan.EventPlanStarted, Data: map[string]interface{}{"plan_id": "test"}},
		{Timestamp: time.Now(), Event: plan.EventPlanCompleted, Data: map[string]interface{}{}},
	}

	progressPath := filepath.Join(tmpDir, "progress.log")
	f, err := os.Create(progressPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range progressEvents {
		data, _ := json.Marshal(e)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	p := &plan.Plan{ID: "test", Tasks: []plan.Task{}}
	analyzer := NewAnalyzer(tmpDir, p)

	// Should not error when output.log is missing
	_, err = analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze should not fail when output.log is missing: %v", err)
	}
}
