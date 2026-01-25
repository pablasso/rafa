# Technical Design: TUI, Output Capture & AGENTS.md Suggestions

## Overview

Complete the remaining PRD features to bring Rafa to feature parity with the v1 specification. This design covers three features that enhance the execution experience and post-run analysis.

**PRD Reference**: [docs/prds/rafa-core.md](../prds/rafa-core.md)

## Goals

- Provide a live TUI during plan execution showing progress, elapsed time, and status
- Capture full agent output to `output.log` for debugging while still streaming to terminal
- Suggest AGENTS.md additions after successful plan completion based on observed patterns

## Non-Goals

- Full-screen TUI application (keep it simple - status line only)
- Auto-modification of AGENTS.md (suggestions only, user decides)
- AI-powered failure analysis (use simple heuristics for now)
- Color themes or TUI customization
- Human input detection (the 5-retry mechanism already handles this - after max attempts, user knows intervention is needed)

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CLI Layer                                    │
│  internal/cli/plan/run.go                                           │
│  - Creates TUI display                                              │
│  - Passes display to executor                                       │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Executor Layer                                 │
│  internal/executor/executor.go                                      │
│  - Updates TUI on state changes                                     │
│  - Passes output writer to runner                                   │
│  - Triggers post-run analysis                                       │
└─────────────────────────────────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
┌───────────────────────────┐   ┌───────────────────────────────────┐
│      Runner Layer         │   │        Display Layer              │
│  internal/executor/       │   │  internal/display/display.go      │
│  runner.go                │   │  - Status line rendering          │
│  - Writes to MultiWriter  │   │  - Elapsed time ticker            │
│  - Logs to output.log     │   │  - Progress formatting            │
└───────────────────────────┘   └───────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Analysis Layer                                  │
│  internal/analysis/suggestions.go                                   │
│  - Reads progress.log and output.log                                │
│  - Identifies patterns (retries, common failures)                   │
│  - Generates AGENTS.md suggestions                                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Technical Details

### New Files

| File | Purpose |
|------|---------|
| `internal/display/display.go` | TUI status line rendering and updates |
| `internal/display/display_test.go` | Tests for display formatting |
| `internal/executor/output.go` | Output capture to terminal and log file |
| `internal/executor/output_test.go` | Tests for output handling |
| `internal/analysis/suggestions.go` | AGENTS.md suggestion generation |
| `internal/analysis/suggestions_test.go` | Tests for suggestion generation |

### Modified Files

| File | Changes |
|------|---------|
| `internal/executor/executor.go` | Integrate display, output capture, post-run analysis |
| `internal/executor/runner.go` | Accept output writer instead of using os.Stdout directly |
| `internal/cli/plan/run.go` | Create and manage display lifecycle |

---

## Feature 1: TUI Display

### Design Approach

Use a simple status line approach rather than a full TUI framework. The status line is rendered at the bottom of the terminal and updated in place using ANSI escape codes. Agent output streams normally above the status line.

```
[Claude Code output streams here normally...]
...
─────────────────────────────────────────────────────────────────────
Task 3/8: Implement user auth │ Attempt 2/10 │ ⏱ 00:12:34 │ Running
```

### Display Component

**File**: `internal/display/display.go`

```go
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
    TaskNum      int
    TotalTasks   int
    TaskTitle    string
    TaskID       string
    Attempt      int
    MaxAttempts  int
    Status       Status
    StartTime    time.Time
}

// Display manages the terminal status line.
type Display struct {
    mu        sync.Mutex
    writer    io.Writer
    state     State
    ticker    *time.Ticker
    done      chan struct{}
    wg        sync.WaitGroup // Ensures goroutine exits before Stop() returns
    active    bool
    lastLine  string
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
    d.mu.Unlock()

    elapsed := time.Since(state.StartTime)
    line := d.formatLine(state, elapsed)

    // Only update if changed (reduces flicker)
    if line == d.lastLine {
        return
    }
    d.lastLine = line

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
```

### Integration with Executor

Modify `executor.go` to accept and use the display:

```go
// In Executor struct
type Executor struct {
    // ... existing fields
    display *display.Display
}

// WithDisplay sets the display for status updates.
func (e *Executor) WithDisplay(d *display.Display) *Executor {
    e.display = d
    return e
}

// In executeTask, update display state
func (e *Executor) executeTask(ctx context.Context, task *plan.Task, idx int, planContext string) error {
    if e.display != nil {
        e.display.UpdateTask(idx+1, len(e.plan.Tasks), task.ID, task.Title)
        e.display.UpdateStatus(display.StatusRunning)
    }

    for task.Attempts < MaxAttempts {
        task.Attempts++

        if e.display != nil {
            e.display.UpdateAttempt(task.Attempts, MaxAttempts)
        }
        // ... rest of execution logic
    }
}
```

---

## Feature 2: Output Capture

### Design Approach

Create an `OutputCapture` type that wraps output handling. It uses `io.MultiWriter` to write to both the terminal and `output.log`.

**File**: `internal/executor/output.go`

```go
package executor

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "time"
)

const outputLogFileName = "output.log"

// OutputCapture manages output to both terminal and log file.
type OutputCapture struct {
    logFile  *os.File
    multiOut io.Writer
    multiErr io.Writer
}

// NewOutputCapture creates an output capture for the given plan directory.
// Opens output.log in append mode to preserve history across runs.
func NewOutputCapture(planDir string) (*OutputCapture, error) {
    logPath := filepath.Join(planDir, outputLogFileName)

    // Open in append mode - preserves history when re-running failed plans
    f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }

    oc := &OutputCapture{
        logFile: f,
    }

    // Create multi-writers for stdout and stderr
    oc.multiOut = io.MultiWriter(os.Stdout, f)
    oc.multiErr = io.MultiWriter(os.Stderr, f)

    return oc, nil
}

// Stdout returns the writer for stdout.
func (oc *OutputCapture) Stdout() io.Writer {
    return oc.multiOut
}

// Stderr returns the writer for stderr.
func (oc *OutputCapture) Stderr() io.Writer {
    return oc.multiErr
}

// Close closes the log file.
func (oc *OutputCapture) Close() error {
    return oc.logFile.Close()
}

// WriteTaskHeader writes a header line to the log for a new task attempt.
func (oc *OutputCapture) WriteTaskHeader(taskID string, attempt int) {
    header := fmt.Sprintf("\n=== Task %s, Attempt %d ===\n", taskID, attempt)
    oc.logFile.WriteString(header)
    oc.logFile.WriteString(fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC3339)))
}

// WriteTaskFooter writes a footer line to the log after task completion.
func (oc *OutputCapture) WriteTaskFooter(taskID string, success bool) {
    result := "SUCCESS"
    if !success {
        result = "FAILED"
    }
    footer := fmt.Sprintf("\n=== Task %s: %s ===\n\n", taskID, result)
    oc.logFile.WriteString(footer)
}
```

### Runner Integration

Modify `runner.go` to accept an output capture:

```go
// Run executes a single task via Claude Code CLI.
func (r *ClaudeRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output *OutputCapture) error {
    prompt := r.buildPrompt(task, planContext, attempt, maxAttempts)

    cmd := ai.CommandContext(ctx, "claude",
        "-p", prompt,
        "--dangerously-skip-permissions",
    )

    if output != nil {
        cmd.Stdout = output.Stdout()
        cmd.Stderr = output.Stderr()
    } else {
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
    }

    // ... rest of execution
}
```

### MockRunner Update

Update `internal/testutil/mock.go` to match the new interface:

```go
// MockRunner implements executor.Runner for testing.
type MockRunner struct {
    Responses []error
    CallCount int
    Calls     []MockRunnerCall
}

type MockRunnerCall struct {
    Task        *plan.Task
    PlanContext string
    Attempt     int
    MaxAttempts int
    Output      *executor.OutputCapture
}

func (m *MockRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int, output *executor.OutputCapture) error {
    m.Calls = append(m.Calls, MockRunnerCall{
        Task:        task,
        PlanContext: planContext,
        Attempt:     attempt,
        MaxAttempts: maxAttempts,
        Output:      output,
    })
    if m.CallCount >= len(m.Responses) {
        return nil
    }
    err := m.Responses[m.CallCount]
    m.CallCount++
    return err
}
```

---

## Feature 3: AGENTS.md Suggestions

### Design Approach

After a successful plan completion, analyze the execution to generate suggestions for AGENTS.md. Focus on actionable patterns:

1. **Common failure patterns**: Commands that frequently fail, tests that are flaky
2. **Successful patterns**: Things that worked well (e.g., always run `make fmt`)
3. **Task-specific learnings**: Context that would help future agents

**File**: `internal/analysis/suggestions.go`

```go
package analysis

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/pablasso/rafa/internal/plan"
)

// Suggestion represents a suggested AGENTS.md addition.
type Suggestion struct {
    Category    string // e.g., "Testing", "Formatting", "Common Issues"
    Title       string
    Description string
    Example     string // Optional code/command example
}

// Analyzer generates AGENTS.md suggestions from plan execution data.
type Analyzer struct {
    planDir string
    plan    *plan.Plan
}

// NewAnalyzer creates a new analyzer for the given plan.
func NewAnalyzer(planDir string, p *plan.Plan) *Analyzer {
    return &Analyzer{
        planDir: planDir,
        plan:    p,
    }
}

// Analyze examines the plan execution and generates suggestions.
func (a *Analyzer) Analyze() ([]Suggestion, error) {
    events, err := a.loadProgressEvents()
    if err != nil {
        return nil, err
    }

    outputLines, err := a.loadOutputLog()
    if err != nil {
        // Output log is optional
        outputLines = nil
    }

    var suggestions []Suggestion

    // Analyze retry patterns
    retrySuggestions := a.analyzeRetries(events)
    suggestions = append(suggestions, retrySuggestions...)

    // Analyze failure patterns in output
    if outputLines != nil {
        failureSuggestions := a.analyzeFailurePatterns(outputLines)
        suggestions = append(suggestions, failureSuggestions...)
    }

    // Analyze successful patterns
    successSuggestions := a.analyzeSuccessPatterns(events, outputLines)
    suggestions = append(suggestions, successSuggestions...)

    // Deduplicate similar suggestions
    suggestions = a.deduplicate(suggestions)

    return suggestions, nil
}

// loadProgressEvents reads and parses the progress log.
func (a *Analyzer) loadProgressEvents() ([]plan.ProgressEvent, error) {
    path := filepath.Join(a.planDir, "progress.log")
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var events []plan.ProgressEvent
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        var event plan.ProgressEvent
        if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
            continue // Skip malformed lines
        }
        events = append(events, event)
    }

    return events, scanner.Err()
}

// loadOutputLog reads the output log.
func (a *Analyzer) loadOutputLog() ([]string, error) {
    path := filepath.Join(a.planDir, "output.log")
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var lines []string
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }

    return lines, scanner.Err()
}

// analyzeRetries looks for tasks that required multiple attempts.
func (a *Analyzer) analyzeRetries(events []plan.ProgressEvent) []Suggestion {
    taskAttempts := make(map[string]int)

    for _, event := range events {
        if event.Event == plan.EventTaskStarted {
            taskID, _ := event.Data["task_id"].(string)
            attempt, _ := event.Data["attempt"].(float64)
            if int(attempt) > taskAttempts[taskID] {
                taskAttempts[taskID] = int(attempt)
            }
        }
    }

    var suggestions []Suggestion

    // Find tasks with >1 attempt
    for taskID, attempts := range taskAttempts {
        if attempts > 1 {
            task := a.findTask(taskID)
            if task != nil {
                suggestions = append(suggestions, Suggestion{
                    Category:    "Common Issues",
                    Title:       fmt.Sprintf("Task '%s' required %d attempts", task.Title, attempts),
                    Description: "Consider adding more specific acceptance criteria or breaking this task into smaller pieces.",
                })
            }
        }
    }

    return suggestions
}

// analyzeFailurePatterns looks for common failure patterns in output.
func (a *Analyzer) analyzeFailurePatterns(lines []string) []Suggestion {
    var suggestions []Suggestion

    // Pattern: test failures
    testFailures := 0
    for _, line := range lines {
        lower := strings.ToLower(line)
        if strings.Contains(lower, "fail") && strings.Contains(lower, "test") {
            testFailures++
        }
    }
    if testFailures > 2 {
        suggestions = append(suggestions, Suggestion{
            Category:    "Testing",
            Title:       "Multiple test failures observed",
            Description: "Agents encountered test failures during execution. Consider documenting test requirements.",
            Example:     "## Testing\n\nRun tests before committing:\n\n```bash\nmake test\n```",
        })
    }

    // Pattern: formatting issues
    fmtIssues := 0
    for _, line := range lines {
        lower := strings.ToLower(line)
        if strings.Contains(lower, "fmt") || strings.Contains(lower, "format") || strings.Contains(lower, "lint") {
            if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "differ") {
                fmtIssues++
            }
        }
    }
    if fmtIssues > 0 {
        suggestions = append(suggestions, Suggestion{
            Category:    "Formatting",
            Title:       "Formatting issues detected",
            Description: "Code formatting checks failed. Document the formatting command.",
            Example:     "## Formatting\n\nAlways run the formatter after modifying code:\n\n```bash\nmake fmt\n```",
        })
    }

    // Pattern: import/dependency issues
    for _, line := range lines {
        lower := strings.ToLower(line)
        if strings.Contains(lower, "cannot find module") || strings.Contains(lower, "module not found") {
            suggestions = append(suggestions, Suggestion{
                Category:    "Dependencies",
                Title:       "Module/dependency issues",
                Description: "Dependency resolution issues occurred. Document how to install dependencies.",
            })
            break
        }
    }

    return suggestions
}

// analyzeSuccessPatterns looks for patterns in successful execution.
func (a *Analyzer) analyzeSuccessPatterns(events []plan.ProgressEvent, outputLines []string) []Suggestion {
    var suggestions []Suggestion

    // If plan completed successfully with tasks that had verification commands
    planCompleted := false
    for _, event := range events {
        if event.Event == plan.EventPlanCompleted {
            planCompleted = true
            break
        }
    }

    if planCompleted && len(a.plan.Tasks) > 0 {
        // Look for common verification patterns in acceptance criteria
        verifyCommands := make(map[string]int)
        for _, task := range a.plan.Tasks {
            for _, criterion := range task.AcceptanceCriteria {
                lower := strings.ToLower(criterion)
                if strings.Contains(lower, "make test") {
                    verifyCommands["make test"]++
                }
                if strings.Contains(lower, "make fmt") {
                    verifyCommands["make fmt"]++
                }
                if strings.Contains(lower, "go build") {
                    verifyCommands["go build"]++
                }
            }
        }

        // Suggest documenting frequently used commands
        for cmd, count := range verifyCommands {
            if count >= 2 {
                suggestions = append(suggestions, Suggestion{
                    Category:    "Verification",
                    Title:       fmt.Sprintf("'%s' used in %d tasks", cmd, count),
                    Description: "This command was used for verification across multiple tasks. Document it prominently.",
                })
            }
        }
    }

    return suggestions
}

// findTask finds a task by ID.
func (a *Analyzer) findTask(taskID string) *plan.Task {
    for i := range a.plan.Tasks {
        if a.plan.Tasks[i].ID == taskID {
            return &a.plan.Tasks[i]
        }
    }
    return nil
}

// deduplicate removes similar suggestions.
func (a *Analyzer) deduplicate(suggestions []Suggestion) []Suggestion {
    seen := make(map[string]bool)
    var result []Suggestion

    for _, s := range suggestions {
        key := s.Category + ":" + s.Title
        if !seen[key] {
            seen[key] = true
            result = append(result, s)
        }
    }

    return result
}

// FormatSuggestions formats suggestions for display.
func FormatSuggestions(suggestions []Suggestion) string {
    if len(suggestions) == 0 {
        return ""
    }

    var sb strings.Builder
    sb.WriteString("\n")
    sb.WriteString("════════════════════════════════════════════════════════════════\n")
    sb.WriteString("  SUGGESTED AGENTS.md ADDITIONS\n")
    sb.WriteString("════════════════════════════════════════════════════════════════\n")
    sb.WriteString("\n")
    sb.WriteString("Based on this plan execution, consider adding to AGENTS.md:\n\n")

    // Group by category
    byCategory := make(map[string][]Suggestion)
    for _, s := range suggestions {
        byCategory[s.Category] = append(byCategory[s.Category], s)
    }

    // Sort categories for consistent output
    var categories []string
    for cat := range byCategory {
        categories = append(categories, cat)
    }
    sort.Strings(categories)

    for _, cat := range categories {
        sb.WriteString(fmt.Sprintf("## %s\n\n", cat))
        for _, s := range byCategory[cat] {
            sb.WriteString(fmt.Sprintf("- %s\n", s.Title))
            sb.WriteString(fmt.Sprintf("  %s\n", s.Description))
            if s.Example != "" {
                sb.WriteString(fmt.Sprintf("\n  Suggested content:\n"))
                for _, line := range strings.Split(s.Example, "\n") {
                    sb.WriteString(fmt.Sprintf("  %s\n", line))
                }
            }
            sb.WriteString("\n")
        }
    }

    sb.WriteString("════════════════════════════════════════════════════════════════\n")
    sb.WriteString("Note: Review these suggestions and add relevant ones to AGENTS.md\n")

    return sb.String()
}
```

### Executor Integration

Call the analyzer after successful plan completion:

```go
// In executor.go, after plan completion
func (e *Executor) Run(ctx context.Context) error {
    // ... existing execution logic ...

    // All tasks completed
    e.plan.Status = plan.PlanStatusCompleted
    // ... save plan ...

    // Generate AGENTS.md suggestions
    analyzer := analysis.NewAnalyzer(e.planDir, e.plan)
    suggestions, err := analyzer.Analyze()
    if err == nil && len(suggestions) > 0 {
        fmt.Print(analysis.FormatSuggestions(suggestions))
    }

    return nil
}
```

---

## Edge Cases

| Case | How it's handled |
|------|------------------|
| Terminal doesn't support ANSI | ANSI codes will appear in output; modern terminals universally support ANSI. For CI/headless, users can redirect output to file. |
| output.log write fails | Log error but continue execution (output capture is non-critical) |
| Re-running a failed plan | output.log appends - preserves history of all attempts for debugging |
| Very long task titles | Truncate to 40 chars with "..." |
| Empty progress.log | Return no suggestions |
| Plan fails before completion | Skip AGENTS.md analysis (only run on success) |
| Ctrl+C during display | Display.Stop() waits for goroutine, then clears status line cleanly |

---

## Security

- **No new external dependencies**: Uses standard library only for TUI
- **Output.log permissions**: Standard 0644 file permissions
- **No secrets in logs**: Output is already streamed to terminal; log file has same content
- **Suggestions are read-only**: Analysis only reads existing files, never modifies AGENTS.md

---

## Performance

- **Display updates**: Once per second (timer-based, not busy-loop)
- **Output capture**: Uses io.MultiWriter (single write call writes to both destinations)
- **Analysis**: Runs once at end, reads files sequentially

---

## Testing

### Unit Tests

**`internal/display/display_test.go`**:
| Test | Description |
|------|-------------|
| `TestFormatDuration` | Formats durations as MM:SS or HH:MM:SS |
| `TestFormatLine` | Creates correct status line format |
| `TestFormatLine_LongTitle` | Truncates long titles with ellipsis |
| `TestUpdateTask` | Updates task state correctly |
| `TestUpdateAttempt` | Updates attempt state correctly |

**`internal/executor/output_test.go`**:
| Test | Description |
|------|-------------|
| `TestOutputCapture_WritesToFile` | Writes to output.log |
| `TestOutputCapture_WritesToStdout` | Also writes to stdout |
| `TestOutputCapture_TaskHeaders` | Writes task headers and footers |

**`internal/analysis/suggestions_test.go`**:
| Test | Description |
|------|-------------|
| `TestAnalyzeRetries` | Identifies tasks with multiple attempts |
| `TestAnalyzeFailurePatterns_Tests` | Detects test failure patterns |
| `TestAnalyzeFailurePatterns_Formatting` | Detects formatting issues |
| `TestFormatSuggestions` | Formats suggestions correctly |
| `TestDeduplicate` | Removes duplicate suggestions |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestRunWithDisplay` | Full execution with display updates |
| `TestRunWithOutputCapture` | Verifies output.log contents |
| `TestSuggestionsAfterCompletion` | Suggestions generated after successful run |

---

## Trade-offs

### TUI Library vs. Simple ANSI

**Chosen**: Simple ANSI escape codes

**Why**:
- No new dependencies
- Sufficient for status line updates
- Full TUI frameworks (bubbletea) add complexity for minimal benefit
- Easier to test (just string formatting)

**Considered**: bubbletea for full TUI
- Rejected: Overkill for a status line; adds significant dependency

### Human Input Detection

**Chosen**: Not implementing - rely on existing retry mechanism

**Why**:
- The 10-retry limit already serves as "give up and ask human"
- Phrase detection is crude and could cause false positives
- Output.log captures everything needed for debugging
- User intervenes either way after max attempts
- Simpler implementation, less code to maintain

**Considered**: Detect phrases like "credentials required" to stop early
- Rejected: Too flaky, marginal benefit (saves ~10-30 min of retries at best)

### AGENTS.md Suggestions: Automatic vs. Manual

**Chosen**: Display suggestions, user manually adds

**Why**:
- PRD explicitly states "suggestions only, user decides"
- Avoids accidentally adding incorrect/irrelevant content
- User maintains control over their documentation

### Output.log: Append vs. Truncate

**Chosen**: Append mode

**Why**:
- Preserves full history when re-running failed plans
- Useful for debugging intermittent failures or comparing attempts
- Task headers in the log clearly delineate different runs
- Users can manually truncate if needed (`> output.log` or delete file)

---

## Decisions

1. **Display update frequency**: 1 second - sufficient for elapsed time without excessive CPU
2. **Status line position**: Bottom of output - natural position, doesn't interfere with streaming
3. **Suggestion categories**: Hardcoded - keeps analysis simple and predictable
4. **ANSI assumption**: Assume ANSI support - all modern terminals support ANSI escape codes
5. **Output.log append mode**: Preserves history across runs for debugging
6. **No human input detection**: Rely on retry mechanism - simpler and more reliable
