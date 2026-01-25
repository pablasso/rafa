package plan

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProgressLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.Log("test_event", map[string]interface{}{
		"key": "value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists and contains valid JSON
	logPath := filepath.Join(tmpDir, progressLogFileName)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var event ProgressEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if event.Event != "test_event" {
		t.Errorf("event mismatch: got %s, want test_event", event.Event)
	}

	if event.Data["key"] != "value" {
		t.Errorf("data mismatch: got %v, want value", event.Data["key"])
	}

	if event.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestProgressLogger_MultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)

	// Log multiple events
	events := []string{"event1", "event2", "event3"}
	for _, evt := range events {
		err := logger.Log(evt, nil)
		if err != nil {
			t.Fatalf("unexpected error logging %s: %v", evt, err)
		}
	}

	// Verify all events are in the file
	logPath := filepath.Join(tmpDir, progressLogFileName)
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var readEvents []string
	for scanner.Scan() {
		var event ProgressEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
		readEvents = append(readEvents, event.Event)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	if len(readEvents) != len(events) {
		t.Fatalf("event count mismatch: got %d, want %d", len(readEvents), len(events))
	}

	for i, evt := range events {
		if readEvents[i] != evt {
			t.Errorf("event %d mismatch: got %s, want %s", i, readEvents[i], evt)
		}
	}
}

func TestProgressLogger_PlanStarted(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.PlanStarted("test-plan-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventPlanStarted {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventPlanStarted)
	}

	if event.Data["plan_id"] != "test-plan-123" {
		t.Errorf("plan_id mismatch: got %v, want test-plan-123", event.Data["plan_id"])
	}
}

func TestProgressLogger_TaskStarted(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.TaskStarted("task-456", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventTaskStarted {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventTaskStarted)
	}

	if event.Data["task_id"] != "task-456" {
		t.Errorf("task_id mismatch: got %v, want task-456", event.Data["task_id"])
	}

	// JSON numbers are float64
	if attempt, ok := event.Data["attempt"].(float64); !ok || int(attempt) != 2 {
		t.Errorf("attempt mismatch: got %v, want 2", event.Data["attempt"])
	}
}

func TestProgressLogger_TaskCompleted(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.TaskCompleted("task-789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventTaskCompleted {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventTaskCompleted)
	}

	if event.Data["task_id"] != "task-789" {
		t.Errorf("task_id mismatch: got %v, want task-789", event.Data["task_id"])
	}
}

func TestProgressLogger_TaskFailed(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.TaskFailed("task-failed-1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventTaskFailed {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventTaskFailed)
	}

	if event.Data["task_id"] != "task-failed-1" {
		t.Errorf("task_id mismatch: got %v, want task-failed-1", event.Data["task_id"])
	}

	if attempt, ok := event.Data["attempt"].(float64); !ok || int(attempt) != 3 {
		t.Errorf("attempt mismatch: got %v, want 3", event.Data["attempt"])
	}
}

func TestProgressLogger_PlanCompleted(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	duration := 5 * time.Second
	err := logger.PlanCompleted(10, 8, duration)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventPlanCompleted {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventPlanCompleted)
	}

	if total, ok := event.Data["total_tasks"].(float64); !ok || int(total) != 10 {
		t.Errorf("total_tasks mismatch: got %v, want 10", event.Data["total_tasks"])
	}

	if succeeded, ok := event.Data["succeeded_tasks"].(float64); !ok || int(succeeded) != 8 {
		t.Errorf("succeeded_tasks mismatch: got %v, want 8", event.Data["succeeded_tasks"])
	}

	if durationMs, ok := event.Data["duration_ms"].(float64); !ok || int64(durationMs) != 5000 {
		t.Errorf("duration_ms mismatch: got %v, want 5000", event.Data["duration_ms"])
	}
}

func TestProgressLogger_PlanCancelled(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.PlanCancelled("last-task-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventPlanCancelled {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventPlanCancelled)
	}

	if event.Data["last_task_id"] != "last-task-id" {
		t.Errorf("last_task_id mismatch: got %v, want last-task-id", event.Data["last_task_id"])
	}
}

func TestProgressLogger_PlanFailed(t *testing.T) {
	tmpDir := t.TempDir()

	logger := NewProgressLogger(tmpDir)
	err := logger.PlanFailed("failed-task", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := readLastEvent(t, tmpDir)

	if event.Event != EventPlanFailed {
		t.Errorf("event mismatch: got %s, want %s", event.Event, EventPlanFailed)
	}

	if event.Data["task_id"] != "failed-task" {
		t.Errorf("task_id mismatch: got %v, want failed-task", event.Data["task_id"])
	}

	if attempts, ok := event.Data["attempts"].(float64); !ok || int(attempts) != 5 {
		t.Errorf("attempts mismatch: got %v, want 5", event.Data["attempts"])
	}
}

// readLastEvent reads and parses the last event from the progress log.
func readLastEvent(t *testing.T, tmpDir string) ProgressEvent {
	t.Helper()

	logPath := filepath.Join(tmpDir, progressLogFileName)
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	var lastLine string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	var event ProgressEvent
	if err := json.Unmarshal([]byte(lastLine), &event); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	return event
}
