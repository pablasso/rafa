package display

import (
	"bytes"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "00:00",
		},
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			expected: "00:45",
		},
		{
			name:     "minutes and seconds",
			duration: 5*time.Minute + 30*time.Second,
			expected: "05:30",
		},
		{
			name:     "59 minutes 59 seconds",
			duration: 59*time.Minute + 59*time.Second,
			expected: "59:59",
		},
		{
			name:     "one hour",
			duration: 1 * time.Hour,
			expected: "01:00:00",
		},
		{
			name:     "hours minutes seconds",
			duration: 2*time.Hour + 34*time.Minute + 56*time.Second,
			expected: "02:34:56",
		},
		{
			name:     "large duration",
			duration: 12*time.Hour + 5*time.Minute + 3*time.Second,
			expected: "12:05:03",
		},
		{
			name:     "rounds to nearest second",
			duration: 5*time.Minute + 30*time.Second + 500*time.Millisecond,
			expected: "05:31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatLine(t *testing.T) {
	d := New(&bytes.Buffer{})

	tests := []struct {
		name     string
		state    State
		elapsed  time.Duration
		expected string
	}{
		{
			name: "basic format",
			state: State{
				TaskNum:     1,
				TotalTasks:  5,
				TaskTitle:   "Implement login",
				TaskID:      "t01",
				Attempt:     1,
				MaxAttempts: 5,
				Status:      StatusRunning,
			},
			elapsed:  1*time.Minute + 30*time.Second,
			expected: "Task 1/5: Implement login │ Attempt 1/5 │ ⏱ 01:30 │ Running",
		},
		{
			name: "zero total tasks returns empty",
			state: State{
				TotalTasks: 0,
			},
			elapsed:  0,
			expected: "",
		},
		{
			name: "completed status",
			state: State{
				TaskNum:     3,
				TotalTasks:  8,
				TaskTitle:   "Write tests",
				TaskID:      "t03",
				Attempt:     2,
				MaxAttempts: 10,
				Status:      StatusCompleted,
			},
			elapsed:  5*time.Minute + 45*time.Second,
			expected: "Task 3/8: Write tests │ Attempt 2/10 │ ⏱ 05:45 │ Completed",
		},
		{
			name: "failed status",
			state: State{
				TaskNum:     2,
				TotalTasks:  4,
				TaskTitle:   "Deploy app",
				TaskID:      "t02",
				Attempt:     5,
				MaxAttempts: 5,
				Status:      StatusFailed,
			},
			elapsed:  10 * time.Minute,
			expected: "Task 2/4: Deploy app │ Attempt 5/5 │ ⏱ 10:00 │ Failed",
		},
		{
			name: "with hours",
			state: State{
				TaskNum:     1,
				TotalTasks:  1,
				TaskTitle:   "Long task",
				TaskID:      "t01",
				Attempt:     1,
				MaxAttempts: 5,
				Status:      StatusRunning,
			},
			elapsed:  1*time.Hour + 15*time.Minute + 30*time.Second,
			expected: "Task 1/1: Long task │ Attempt 1/5 │ ⏱ 01:15:30 │ Running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.formatLine(tt.state, tt.elapsed)
			if result != tt.expected {
				t.Errorf("formatLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatLine_LongTitle(t *testing.T) {
	d := New(&bytes.Buffer{})

	tests := []struct {
		name           string
		title          string
		expectedInLine string
	}{
		{
			name:           "exactly 40 chars",
			title:          "1234567890123456789012345678901234567890",
			expectedInLine: "1234567890123456789012345678901234567890",
		},
		{
			name:           "41 chars truncated",
			title:          "12345678901234567890123456789012345678901",
			expectedInLine: "1234567890123456789012345678901234567...",
		},
		{
			name:           "very long title truncated",
			title:          "This is a very long task title that should definitely be truncated with ellipsis",
			expectedInLine: "This is a very long task title that s...",
		},
		{
			name:           "short title unchanged",
			title:          "Short title",
			expectedInLine: "Short title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := State{
				TaskNum:     1,
				TotalTasks:  5,
				TaskTitle:   tt.title,
				TaskID:      "t01",
				Attempt:     1,
				MaxAttempts: 5,
				Status:      StatusRunning,
			}
			result := d.formatLine(state, 1*time.Minute)

			// Verify the title appears correctly in the output
			expectedPrefix := "Task 1/5: " + tt.expectedInLine + " │"
			if len(result) < len(expectedPrefix) || result[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("formatLine() with title %q:\ngot:  %q\nwant prefix: %q", tt.title, result, expectedPrefix)
			}
		})
	}
}

func TestUpdateTask(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	// Initial state should be zero values
	if d.state.TaskNum != 0 || d.state.TotalTasks != 0 || d.state.TaskID != "" || d.state.TaskTitle != "" {
		t.Error("Initial state should have zero values")
	}

	// Update task
	d.UpdateTask(3, 8, "t03", "Implement user auth")

	// Verify state was updated
	if d.state.TaskNum != 3 {
		t.Errorf("TaskNum = %d, want 3", d.state.TaskNum)
	}
	if d.state.TotalTasks != 8 {
		t.Errorf("TotalTasks = %d, want 8", d.state.TotalTasks)
	}
	if d.state.TaskID != "t03" {
		t.Errorf("TaskID = %q, want %q", d.state.TaskID, "t03")
	}
	if d.state.TaskTitle != "Implement user auth" {
		t.Errorf("TaskTitle = %q, want %q", d.state.TaskTitle, "Implement user auth")
	}
}

func TestUpdateAttempt(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	// Initial state should be zero values
	if d.state.Attempt != 0 || d.state.MaxAttempts != 0 {
		t.Error("Initial state should have zero values")
	}

	// Update attempt
	d.UpdateAttempt(2, 10)

	// Verify state was updated
	if d.state.Attempt != 2 {
		t.Errorf("Attempt = %d, want 2", d.state.Attempt)
	}
	if d.state.MaxAttempts != 10 {
		t.Errorf("MaxAttempts = %d, want 10", d.state.MaxAttempts)
	}

	// Update again
	d.UpdateAttempt(5, 10)

	if d.state.Attempt != 5 {
		t.Errorf("Attempt = %d, want 5", d.state.Attempt)
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusIdle, "Idle"},
		{StatusRunning, "Running"},
		{StatusCompleted, "Completed"},
		{StatusFailed, "Failed"},
		{StatusCancelled, "Cancelled"},
		{Status(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Status(%d).String() = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	// Initial state should be StatusIdle (zero value)
	if d.state.Status != StatusIdle {
		t.Errorf("Initial status = %v, want StatusIdle", d.state.Status)
	}

	// Update to running
	d.UpdateStatus(StatusRunning)
	if d.state.Status != StatusRunning {
		t.Errorf("Status = %v, want StatusRunning", d.state.Status)
	}

	// Update to completed
	d.UpdateStatus(StatusCompleted)
	if d.state.Status != StatusCompleted {
		t.Errorf("Status = %v, want StatusCompleted", d.state.Status)
	}
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	if d == nil {
		t.Fatal("New() returned nil")
	}
	if d.writer != &buf {
		t.Error("writer not set correctly")
	}
	if d.done == nil {
		t.Error("done channel not initialized")
	}
	if d.active {
		t.Error("should not be active initially")
	}
}

func TestStartStop(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	// Should not be active initially
	if d.active {
		t.Error("should not be active before Start()")
	}

	// Start the display
	d.Start()

	// Give the goroutine time to start
	time.Sleep(50 * time.Millisecond)

	d.mu.Lock()
	active := d.active
	d.mu.Unlock()

	if !active {
		t.Error("should be active after Start()")
	}

	// Stop the display
	d.Stop()

	d.mu.Lock()
	active = d.active
	d.mu.Unlock()

	if active {
		t.Error("should not be active after Stop()")
	}
}

func TestStartIdempotent(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	// Start multiple times should be safe
	d.Start()
	d.Start()
	d.Start()

	time.Sleep(50 * time.Millisecond)

	d.Stop()

	// Should be stopped
	d.mu.Lock()
	active := d.active
	d.mu.Unlock()

	if active {
		t.Error("should not be active after Stop()")
	}
}

func TestStopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	d := New(&buf)

	// Stop without start should be safe
	d.Stop()

	// Start then stop multiple times
	d.done = make(chan struct{}) // Reset done channel
	d.Start()
	time.Sleep(50 * time.Millisecond)
	d.Stop()
	d.Stop()
	d.Stop()

	// Should remain stopped
	d.mu.Lock()
	active := d.active
	d.mu.Unlock()

	if active {
		t.Error("should not be active after multiple Stop() calls")
	}
}
