package display

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Status represents the current execution status.
type Status int

const (
	StatusIdle Status = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

func (s Status) String() string {
	switch s {
	case StatusIdle:
		return "Idle"
	case StatusRunning:
		return "Running"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	case StatusCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// State holds the current display state.
type State struct {
	TaskNum     int
	TotalTasks  int
	TaskTitle   string
	TaskID      string
	Attempt     int
	MaxAttempts int
	Status      Status
	StartTime   time.Time
}

// Display manages the terminal status line.
type Display struct {
	mu       sync.Mutex
	writer   io.Writer
	state    State
	ticker   *time.Ticker
	done     chan struct{}
	wg       sync.WaitGroup // Ensures goroutine exits before Stop() returns
	active   bool
	lastLine string
}

// New creates a new Display writing to the given writer.
func New(w io.Writer) *Display {
	return &Display{
		writer: w,
		done:   make(chan struct{}),
	}
}

// Start begins the display update loop.
func (d *Display) Start() {
	d.mu.Lock()
	if d.active {
		d.mu.Unlock()
		return
	}
	d.active = true
	d.state.StartTime = time.Now()
	d.ticker = time.NewTicker(time.Second)
	d.wg.Add(1)
	d.mu.Unlock()

	go d.updateLoop()
}

// Stop halts the display update loop and clears the status line.
// Blocks until the update goroutine has exited to prevent race conditions.
func (d *Display) Stop() {
	d.mu.Lock()
	if !d.active {
		d.mu.Unlock()
		return
	}
	d.active = false
	d.mu.Unlock()

	d.ticker.Stop()
	close(d.done)
	d.wg.Wait() // Wait for goroutine to exit before clearing
	d.clearLine()
}

// UpdateTask updates the current task information.
func (d *Display) UpdateTask(taskNum, totalTasks int, taskID, taskTitle string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state.TaskNum = taskNum
	d.state.TotalTasks = totalTasks
	d.state.TaskID = taskID
	d.state.TaskTitle = taskTitle
}

// UpdateAttempt updates the current attempt number.
func (d *Display) UpdateAttempt(attempt, maxAttempts int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state.Attempt = attempt
	d.state.MaxAttempts = maxAttempts
}

// UpdateStatus updates the execution status.
func (d *Display) UpdateStatus(status Status) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state.Status = status
}

// updateLoop periodically renders the status line.
func (d *Display) updateLoop() {
	defer d.wg.Done()
	d.render()
	for {
		select {
		case <-d.ticker.C:
			d.render()
		case <-d.done:
			return
		}
	}
}

// render draws the current status line.
func (d *Display) render() {
	d.mu.Lock()
	state := d.state
	lastLine := d.lastLine
	d.mu.Unlock()

	elapsed := time.Since(state.StartTime)
	line := d.formatLine(state, elapsed)

	// Only update if changed (reduces flicker)
	if line == lastLine {
		return
	}

	d.mu.Lock()
	d.lastLine = line
	d.mu.Unlock()

	// Move to start of line, clear it, write new content
	fmt.Fprintf(d.writer, "\r\033[K%s", line)
}

// formatLine creates the status line string.
func (d *Display) formatLine(state State, elapsed time.Duration) string {
	if state.TotalTasks == 0 {
		return ""
	}

	// Truncate title if too long
	title := state.TaskTitle
	if len(title) > 40 {
		title = title[:37] + "..."
	}

	timeStr := formatDuration(elapsed)

	return fmt.Sprintf("Task %d/%d: %s │ Attempt %d/%d │ ⏱ %s │ %s",
		state.TaskNum,
		state.TotalTasks,
		title,
		state.Attempt,
		state.MaxAttempts,
		timeStr,
		state.Status)
}

// clearLine clears the status line.
func (d *Display) clearLine() {
	fmt.Fprintf(d.writer, "\r\033[K")
}

// PrintAbove prints a message above the status line.
// Use this for important messages that shouldn't be overwritten.
func (d *Display) PrintAbove(format string, args ...interface{}) {
	d.clearLine()
	fmt.Fprintf(d.writer, format+"\n", args...)
	d.render()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
