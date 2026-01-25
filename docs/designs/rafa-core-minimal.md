# Technical Design: Rafa Core - Minimal Dogfooding Version

## Overview

Implement the minimal task execution loop for Rafa to enable dogfooding. This design focuses on the core loop that runs n tasks sequentially, with retry logic and state persistence.

**PRD**: [docs/prds/rafa-core.md](../prds/rafa-core.md)

## Goals

- Run `rafa plan run <name>` to execute a plan's tasks sequentially
- Retry failed tasks up to 10 times with fresh Claude Code sessions
- Persist plan state to plan.json after each task state change
- Log progress events to progress.log
- Resume interrupted runs from first pending task

## Non-Goals

- TUI with live updates (use simple console output for now)
- `rafa init` / `rafa deinit` commands (user creates `.rafa/plans/` manually)
- AGENTS.md suggestions post-run
- Human input required detection
- Output.log capture (defer to later)
- Headless/CI mode

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Layer                                │
│  internal/cli/plan/run.go                                   │
│  - Parse args, find plan folder, load plan                  │
│  - Call executor.Run()                                      │
│  - Handle Ctrl+C gracefully                                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Executor Layer                             │
│  internal/executor/executor.go                              │
│  - Orchestrate task loop                                    │
│  - For each pending task: run → check result → update state │
│  - Retry on failure (up to MaxAttempts)                     │
│  - Stop on success or max attempts exhausted                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Runner Layer                              │
│  internal/executor/runner.go                                │
│  - Execute single task via Claude CLI                       │
│  - Build prompt with task context                           │
│  - Parse exit status (success/failure)                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    State Layer                               │
│  internal/plan/state.go                                     │
│  - Load plan from JSON                                      │
│  - Update task/plan status                                  │
│  - Save plan to JSON (atomic write)                         │
│                                                             │
│  internal/plan/progress.go                                  │
│  - Append events to progress.log                            │
└─────────────────────────────────────────────────────────────┘
```

## Technical Details

### New Files

| File | Purpose |
|------|---------|
| `internal/cli/plan/run.go` | `plan run` command implementation |
| `internal/cli/plan/run_test.go` | Unit tests for run command |
| `internal/executor/executor.go` | Task loop orchestration |
| `internal/executor/executor_test.go` | Unit tests for executor |
| `internal/executor/runner.go` | Single task execution via Claude |
| `internal/executor/runner_test.go` | Unit tests for runner |
| `internal/plan/state.go` | Plan loading and saving |
| `internal/plan/state_test.go` | Unit tests for state management |
| `internal/plan/progress.go` | Progress event logging |
| `internal/plan/progress_test.go` | Unit tests for progress logging |
| `internal/plan/lock.go` | Plan lock file management |
| `internal/plan/lock_test.go` | Unit tests for lock management |

### CLI: `plan run` Command

**File**: `internal/cli/plan/run.go`

```go
// Command definition
var runCmd = &cobra.Command{
    Use:   "run <name>",
    Short: "Run a plan (resumes from first pending task)",
    Args:  cobra.ExactArgs(1),
    RunE:  runPlan,
}

func runPlan(cmd *cobra.Command, args []string) error {
    planName := args[0]

    // 1. Check .rafa/ exists
    if _, err := os.Stat(".rafa"); os.IsNotExist(err) {
        return fmt.Errorf("rafa not initialized. Run `rafa init` first")
    }

    // 2. Find plan folder by name suffix
    planDir, err := plan.FindPlanFolder(planName)
    if err != nil {
        return err
    }

    // 3. Load plan from JSON
    p, err := plan.LoadPlan(planDir)
    if err != nil {
        return err
    }

    // 4. Check Claude CLI availability
    if !ai.IsClaudeAvailable() {
        return fmt.Errorf("Claude Code CLI not found. Install it: https://claude.ai/code")
    }

    // 5. Create executor and run
    ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    exec := executor.New(planDir, p)
    return exec.Run(ctx)
}
```

**Registration** in `internal/cli/plan/plan.go`:
```go
func init() {
    PlanCmd.AddCommand(runCmd)
}
```

### Plan Folder Discovery

**File**: `internal/plan/state.go`

```go
// FindPlanFolder finds a plan folder by name suffix.
// Looks for folders matching pattern "*-<name>" in .rafa/plans/
func FindPlanFolder(name string) (string, error) {
    plansDir := ".rafa/plans"
    entries, err := os.ReadDir(plansDir)
    if err != nil {
        if os.IsNotExist(err) {
            return "", fmt.Errorf("no plans found. Run `rafa plan create <design.md>` first")
        }
        return "", err
    }

    suffix := "-" + name
    var matches []string
    for _, entry := range entries {
        if entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
            matches = append(matches, filepath.Join(plansDir, entry.Name()))
        }
    }

    switch len(matches) {
    case 0:
        return "", fmt.Errorf("plan not found: %s", name)
    case 1:
        return matches[0], nil
    default:
        return "", fmt.Errorf("multiple plans match '%s': %v", name, matches)
    }
}
```

### Plan State Management

**File**: `internal/plan/state.go`

```go
// LoadPlan reads plan.json from the given plan directory.
func LoadPlan(planDir string) (*Plan, error) {
    path := filepath.Join(planDir, "plan.json")
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read plan: %w", err)
    }

    var p Plan
    if err := json.Unmarshal(data, &p); err != nil {
        return nil, fmt.Errorf("failed to parse plan: %w", err)
    }
    return &p, nil
}

// SavePlan writes plan.json to the given plan directory.
// Uses atomic write (write to temp, then rename) to prevent corruption.
func SavePlan(planDir string, p *Plan) error {
    path := filepath.Join(planDir, "plan.json")
    data, err := json.MarshalIndent(p, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal plan: %w", err)
    }

    // Atomic write: write to unique temp file, then rename
    // Use PID to avoid collisions if multiple processes somehow run
    tmpPath := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write plan: %w", err)
    }
    if err := os.Rename(tmpPath, path); err != nil {
        os.Remove(tmpPath) // Clean up on failure
        return fmt.Errorf("failed to save plan: %w", err)
    }
    return nil
}

// FirstPendingTask returns the index of the first non-completed task, or -1 if none.
// Also resets failed tasks to pending (with attempts preserved) so they can be retried.
// This allows re-running a failed plan without manual JSON editing.
func (p *Plan) FirstPendingTask() int {
    for i := range p.Tasks {
        t := &p.Tasks[i]
        switch t.Status {
        case TaskStatusPending, TaskStatusInProgress:
            return i
        case TaskStatusFailed:
            // Reset failed task to pending so it can be retried
            // Attempts counter is preserved so we know this is a retry
            t.Status = TaskStatusPending
            return i
        }
    }
    return -1
}

// AllTasksCompleted returns true if all tasks have completed status.
func (p *Plan) AllTasksCompleted() bool {
    for _, t := range p.Tasks {
        if t.Status != TaskStatusCompleted {
            return false
        }
    }
    return true
}
```

### Plan Lock File

**File**: `internal/plan/lock.go`

Prevents concurrent runs of the same plan. Uses a simple lock file with PID.

```go
const lockFileName = "run.lock"

// PlanLock manages a lock file to prevent concurrent plan execution.
type PlanLock struct {
    path string
}

// NewPlanLock creates a lock manager for the given plan directory.
func NewPlanLock(planDir string) *PlanLock {
    return &PlanLock{
        path: filepath.Join(planDir, lockFileName),
    }
}

// Acquire attempts to acquire the lock. Returns an error if already locked.
func (l *PlanLock) Acquire() error {
    // Try to create lock file atomically with O_EXCL (fails if file exists)
    f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
    if err == nil {
        // Successfully created lock file - write our PID
        defer f.Close()
        _, err = f.WriteString(fmt.Sprintf("%d", os.Getpid()))
        return err
    }

    // Lock file exists - check if the process is still running
    if !os.IsExist(err) {
        return err // Some other error
    }

    data, readErr := os.ReadFile(l.path)
    if readErr != nil {
        return fmt.Errorf("lock file exists but cannot be read: %w", readErr)
    }

    var pid int
    if _, err := fmt.Sscanf(string(data), "%d", &pid); err == nil {
        if processExists(pid) {
            return fmt.Errorf("plan is already running (pid %d). If this is stale, delete %s", pid, l.path)
        }
    }

    // Stale lock file - remove it and retry once
    if err := os.Remove(l.path); err != nil {
        return fmt.Errorf("cannot remove stale lock file: %w", err)
    }

    // Retry atomic creation
    f, err = os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("cannot acquire lock: %w", err)
    }
    defer f.Close()
    _, err = f.WriteString(fmt.Sprintf("%d", os.Getpid()))
    return err
}

// Release removes the lock file.
func (l *PlanLock) Release() error {
    return os.Remove(l.path)
}

// processExists checks if a process with the given PID is running.
func processExists(pid int) bool {
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    // On Unix, FindProcess always succeeds. Send signal 0 to check if process exists.
    err = process.Signal(syscall.Signal(0))
    return err == nil
}
```

### Progress Event Logging

**File**: `internal/plan/progress.go`

```go
// Event types
const (
    EventPlanStarted    = "plan_started"
    EventPlanCompleted  = "plan_completed"
    EventPlanCancelled  = "plan_cancelled"
    EventPlanFailed     = "plan_failed"
    EventTaskStarted    = "task_started"
    EventTaskCompleted  = "task_completed"
    EventTaskFailed     = "task_failed"
)

// ProgressEvent represents a single event in the progress log.
type ProgressEvent struct {
    Timestamp time.Time              `json:"timestamp"`
    Event     string                 `json:"event"`
    Data      map[string]interface{} `json:"data,omitempty"`
}

// ProgressLogger writes events to progress.log
type ProgressLogger struct {
    path string
}

// NewProgressLogger creates a logger for the given plan directory.
func NewProgressLogger(planDir string) *ProgressLogger {
    return &ProgressLogger{
        path: filepath.Join(planDir, "progress.log"),
    }
}

// Log appends an event to the progress log.
func (l *ProgressLogger) Log(event string, data map[string]interface{}) error {
    e := ProgressEvent{
        Timestamp: time.Now(),
        Event:     event,
        Data:      data,
    }

    line, err := json.Marshal(e)
    if err != nil {
        return err
    }

    f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = f.WriteString(string(line) + "\n")
    return err
}

// Convenience methods
func (l *ProgressLogger) PlanStarted(planID string) error {
    return l.Log(EventPlanStarted, map[string]interface{}{"plan_id": planID})
}

func (l *ProgressLogger) TaskStarted(taskID string, attempt int) error {
    return l.Log(EventTaskStarted, map[string]interface{}{
        "task_id": taskID,
        "attempt": attempt,
    })
}

func (l *ProgressLogger) TaskCompleted(taskID string) error {
    return l.Log(EventTaskCompleted, map[string]interface{}{"task_id": taskID})
}

func (l *ProgressLogger) TaskFailed(taskID string, attempt int) error {
    return l.Log(EventTaskFailed, map[string]interface{}{
        "task_id": taskID,
        "attempt": attempt,
    })
}

func (l *ProgressLogger) PlanCompleted(totalTasks, succeededTasks int, duration time.Duration) error {
    return l.Log(EventPlanCompleted, map[string]interface{}{
        "total_tasks":     totalTasks,
        "succeeded_tasks": succeededTasks,
        "duration_sec":    duration.Seconds(),
    })
}

func (l *ProgressLogger) PlanCancelled(lastTaskID string) error {
    return l.Log(EventPlanCancelled, map[string]interface{}{"last_task_id": lastTaskID})
}

func (l *ProgressLogger) PlanFailed(taskID string, attempts int) error {
    return l.Log(EventPlanFailed, map[string]interface{}{
        "task_id":  taskID,
        "attempts": attempts,
    })
}
```

### Executor

**File**: `internal/executor/executor.go`

```go
const MaxAttempts = 10

// Executor orchestrates the task execution loop.
type Executor struct {
    planDir  string
    plan     *plan.Plan
    logger   *plan.ProgressLogger
    runner   Runner
    lock     *plan.PlanLock
}

// Runner interface for executing single tasks (allows mocking in tests)
type Runner interface {
    Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int) error
}

// New creates a new Executor.
func New(planDir string, p *plan.Plan) *Executor {
    return &Executor{
        planDir: planDir,
        plan:    p,
        logger:  plan.NewProgressLogger(planDir),
        runner:  NewClaudeRunner(),
        lock:    plan.NewPlanLock(planDir),
    }
}

// Run executes the plan's tasks sequentially.
func (e *Executor) Run(ctx context.Context) error {
    // Acquire lock to prevent concurrent runs
    if err := e.lock.Acquire(); err != nil {
        return err
    }
    defer e.lock.Release()

    startTime := time.Now()

    // Find first pending task
    taskIdx := e.plan.FirstPendingTask()
    if taskIdx == -1 {
        if e.plan.AllTasksCompleted() {
            fmt.Println("All tasks already completed.")
            return nil
        }
        fmt.Println("No pending tasks found.")
        return nil
    }

    // Log plan started (only if not already in progress)
    if e.plan.Status == plan.PlanStatusNotStarted {
        e.plan.Status = plan.PlanStatusInProgress
        if err := plan.SavePlan(e.planDir, e.plan); err != nil {
            return err
        }
        e.logger.PlanStarted(e.plan.ID)
    }

    totalTasks := len(e.plan.Tasks)
    fmt.Printf("Resuming from task %d/%d...\n", taskIdx+1, totalTasks)

    // Build plan context for runner
    planContext := e.buildPlanContext()

    // Execute tasks sequentially
    for i := taskIdx; i < totalTasks; i++ {
        task := &e.plan.Tasks[i]

        if err := e.executeTask(ctx, task, i+1, totalTasks, planContext); err != nil {
            if ctx.Err() != nil {
                // Cancelled by user - reset task to pending so it resumes correctly
                task.Status = plan.TaskStatusPending
                plan.SavePlan(e.planDir, e.plan)
                e.logger.PlanCancelled(task.ID)
                fmt.Println("\nRun cancelled. Progress saved. Resume with `rafa plan run " + e.plan.Name + "`.")
                return nil
            }
            // Task failed after max attempts
            e.plan.Status = plan.PlanStatusFailed
            plan.SavePlan(e.planDir, e.plan)
            e.logger.PlanFailed(task.ID, task.Attempts)
            return err
        }
    }

    // All tasks completed
    e.plan.Status = plan.PlanStatusCompleted
    if err := plan.SavePlan(e.planDir, e.plan); err != nil {
        return err
    }

    duration := time.Since(startTime)
    succeeded := e.countCompleted()
    e.logger.PlanCompleted(totalTasks, succeeded, duration)

    fmt.Printf("\nPlan complete: %d/%d tasks succeeded in %s.\n",
        succeeded, totalTasks, formatDuration(duration))

    return nil
}

// executeTask runs a single task with retry logic.
func (e *Executor) executeTask(ctx context.Context, task *plan.Task, num, total int, planContext string) error {
    for task.Attempts < MaxAttempts {
        // Check for cancellation
        if ctx.Err() != nil {
            return ctx.Err()
        }

        task.Attempts++
        task.Status = plan.TaskStatusInProgress
        if err := plan.SavePlan(e.planDir, e.plan); err != nil {
            return err
        }

        fmt.Printf("\nTask %d/%d: %s [Attempt %d/%d]\n",
            num, total, task.Title, task.Attempts, MaxAttempts)

        e.logger.TaskStarted(task.ID, task.Attempts)

        // Execute task
        err := e.runner.Run(ctx, task, planContext, task.Attempts, MaxAttempts)

        if err == nil {
            // Success
            task.Status = plan.TaskStatusCompleted
            if err := plan.SavePlan(e.planDir, e.plan); err != nil {
                return err
            }
            e.logger.TaskCompleted(task.ID)
            fmt.Printf("Task %d/%d completed.\n", num, total)
            return nil
        }

        // Failure
        e.logger.TaskFailed(task.ID, task.Attempts)
        fmt.Printf("Task %d/%d failed (attempt %d/%d): %v\n",
            num, total, task.Attempts, MaxAttempts, err)

        if task.Attempts >= MaxAttempts {
            task.Status = plan.TaskStatusFailed
            plan.SavePlan(e.planDir, e.plan)
            return fmt.Errorf("task %s failed after %d attempts", task.ID, MaxAttempts)
        }

        fmt.Println("Spinning up fresh agent for retry...")
    }

    return nil
}

func (e *Executor) buildPlanContext() string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("Plan: %s\n", e.plan.Name))
    sb.WriteString(fmt.Sprintf("Description: %s\n", e.plan.Description))
    sb.WriteString(fmt.Sprintf("Source: %s\n", e.plan.SourceFile))
    return sb.String()
}

func (e *Executor) countCompleted() int {
    count := 0
    for _, t := range e.plan.Tasks {
        if t.Status == plan.TaskStatusCompleted {
            count++
        }
    }
    return count
}

func formatDuration(d time.Duration) string {
    h := int(d.Hours())
    m := int(d.Minutes()) % 60
    s := int(d.Seconds()) % 60
    if h > 0 {
        return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
    }
    return fmt.Sprintf("%02d:%02d", m, s)
}
```

### Task Runner

**File**: `internal/executor/runner.go`

```go
// ClaudeRunner executes tasks via Claude Code CLI.
type ClaudeRunner struct{}

func NewClaudeRunner() *ClaudeRunner {
    return &ClaudeRunner{}
}

// Run executes a single task via Claude Code CLI.
func (r *ClaudeRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int) error {
    prompt := r.buildPrompt(task, planContext, attempt, maxAttempts)

    cmd := ai.CommandContext(ctx, "claude",
        "-p", prompt,
        "--dangerously-skip-permissions",
    )

    // Stream output to stdout for visibility
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    err := cmd.Run()
    if err != nil {
        // Check if it's a context cancellation
        if ctx.Err() != nil {
            return ctx.Err()
        }
        return fmt.Errorf("claude exited with error: %w", err)
    }

    return nil
}

func (r *ClaudeRunner) buildPrompt(task *plan.Task, planContext string, attempt, maxAttempts int) string {
    var sb strings.Builder

    sb.WriteString("You are executing a task as part of an automated plan.\n\n")
    sb.WriteString("## Context\n")
    sb.WriteString(planContext)
    sb.WriteString("\n")

    sb.WriteString("## Your Task\n")
    sb.WriteString(fmt.Sprintf("**ID**: %s\n", task.ID))
    sb.WriteString(fmt.Sprintf("**Title**: %s\n", task.Title))
    sb.WriteString(fmt.Sprintf("**Attempt**: %d of %d\n", attempt, maxAttempts))
    sb.WriteString(fmt.Sprintf("**Description**: %s\n\n", task.Description))

    // Add retry context if this is not the first attempt
    if attempt > 1 {
        sb.WriteString("**Note**: Previous attempts to complete this task failed. ")
        sb.WriteString("Consider alternative approaches or investigate what went wrong.\n\n")
    }

    sb.WriteString("## Acceptance Criteria\n")
    sb.WriteString("You MUST verify ALL of the following before considering the task complete:\n")
    for i, criterion := range task.AcceptanceCriteria {
        sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, criterion))
    }
    sb.WriteString("\n")

    sb.WriteString("## Instructions\n")
    sb.WriteString("1. Implement the task as described\n")
    sb.WriteString("2. Verify ALL acceptance criteria are met\n")
    sb.WriteString("3. If all criteria pass, commit your changes with a descriptive message\n")
    sb.WriteString("4. Exit when done\n\n")

    sb.WriteString("IMPORTANT: Do not declare success unless ALL acceptance criteria are verifiably met.\n")
    sb.WriteString("If you cannot complete the task or verify the criteria, exit with an error.\n")

    return sb.String()
}
```

### Console Output

For the minimal version, we use simple `fmt.Printf` statements. Example output:

```
$ rafa plan run feature-auth
Resuming from task 3/8...

Task 3/8: Implement user authentication [Attempt 1/10]
[Claude Code output streams here...]
Task 3/8 completed.

Task 4/8: Add login endpoint [Attempt 1/10]
[Claude Code output streams here...]
Task 4/8 failed (attempt 1/10): claude exited with error: exit status 1
Spinning up fresh agent for retry...

Task 4/8: Add login endpoint [Attempt 2/10]
[Claude Code output streams here...]
Task 4/8 completed.

...

Plan complete: 8/8 tasks succeeded in 01:23:45.
```

## Testing

### Unit Tests

#### `internal/plan/state_test.go`

| Test | Description |
|------|-------------|
| `TestFindPlanFolder_Found` | Finds plan folder by name suffix |
| `TestFindPlanFolder_NotFound` | Returns error when plan doesn't exist |
| `TestFindPlanFolder_MultipleMatches` | Returns error when multiple folders match |
| `TestFindPlanFolder_NoPlansDir` | Returns helpful error when .rafa/plans doesn't exist |
| `TestLoadPlan_Success` | Loads and parses valid plan.json |
| `TestLoadPlan_InvalidJSON` | Returns error for malformed JSON |
| `TestLoadPlan_FileNotFound` | Returns error when plan.json missing |
| `TestSavePlan_Success` | Saves plan with pretty-printed JSON |
| `TestSavePlan_AtomicWrite` | Verifies atomic write behavior |
| `TestFirstPendingTask_FindsPending` | Returns index of first pending task |
| `TestFirstPendingTask_FindsInProgress` | Returns index of in_progress task |
| `TestFirstPendingTask_FindsFailed` | Returns index of failed task and resets to pending |
| `TestFirstPendingTask_FailedPreservesAttempts` | Resetting failed task preserves attempt count |
| `TestFirstPendingTask_NoneRemaining` | Returns -1 when all completed |
| `TestAllTasksCompleted_True` | Returns true when all completed |
| `TestAllTasksCompleted_False` | Returns false when some pending |

#### `internal/plan/lock_test.go`

| Test | Description |
|------|-------------|
| `TestPlanLock_Acquire_Success` | Acquires lock when not held |
| `TestPlanLock_Acquire_AlreadyLocked` | Returns error when lock held by running process |
| `TestPlanLock_Acquire_StaleLock` | Acquires lock when previous process is dead |
| `TestPlanLock_Acquire_RaceCondition` | Concurrent acquires - only one succeeds |
| `TestPlanLock_Release` | Removes lock file |
| `TestPlanLock_Release_NotHeld` | No error when releasing unheld lock |

#### `internal/plan/progress_test.go`

| Test | Description |
|------|-------------|
| `TestProgressLogger_Log` | Appends JSON event to log file |
| `TestProgressLogger_MultipleEvents` | Appends multiple events in order |
| `TestProgressLogger_PlanStarted` | Logs plan_started with plan_id |
| `TestProgressLogger_TaskStarted` | Logs task_started with task_id and attempt |
| `TestProgressLogger_TaskCompleted` | Logs task_completed with task_id |
| `TestProgressLogger_TaskFailed` | Logs task_failed with task_id and attempt |
| `TestProgressLogger_PlanCompleted` | Logs plan_completed with summary |
| `TestProgressLogger_PlanCancelled` | Logs plan_cancelled with last_task_id |
| `TestProgressLogger_PlanFailed` | Logs plan_failed with task_id and attempts |

#### `internal/executor/executor_test.go`

| Test | Description |
|------|-------------|
| `TestExecutor_AllTasksComplete` | Returns early when all tasks done |
| `TestExecutor_NoPendingTasks` | Handles no pending tasks gracefully |
| `TestExecutor_RunsSingleTask` | Executes one task successfully |
| `TestExecutor_RunsMultipleTasks` | Executes tasks sequentially |
| `TestExecutor_RetriesOnFailure` | Retries failed task up to MaxAttempts |
| `TestExecutor_StopsAfterMaxAttempts` | Stops and marks failed after 10 attempts |
| `TestExecutor_ResumesFromPending` | Skips completed tasks on resume |
| `TestExecutor_CancellationHandled` | Handles Ctrl+C gracefully |
| `TestExecutor_CancellationResetsTaskStatus` | Resets in_progress task to pending on cancel |
| `TestExecutor_AcquiresLock` | Acquires lock before execution |
| `TestExecutor_ReleasesLockOnComplete` | Releases lock after completion |
| `TestExecutor_ReleasesLockOnCancel` | Releases lock on cancellation |
| `TestExecutor_ReleasesLockOnFailure` | Releases lock after max attempts failure |
| `TestExecutor_ConcurrentRunBlocked` | Second run fails when first is running |
| `TestExecutor_SavesStateAfterEachTask` | Persists state after status changes |
| `TestExecutor_LogsProgressEvents` | Logs all expected events |

#### `internal/executor/runner_test.go`

| Test | Description |
|------|-------------|
| `TestClaudeRunner_BuildPrompt` | Builds correct prompt with all fields |
| `TestClaudeRunner_PromptIncludesAllCriteria` | All acceptance criteria in prompt |
| `TestClaudeRunner_PromptIncludesAttemptNumber` | Prompt shows "Attempt X of Y" |
| `TestClaudeRunner_PromptIncludesRetryNote` | Prompt includes retry note when attempt > 1 |
| `TestClaudeRunner_PromptNoRetryNoteOnFirstAttempt` | No retry note on first attempt |
| `TestClaudeRunner_Run_Success` | Returns nil on successful exit |
| `TestClaudeRunner_Run_Failure` | Returns error on non-zero exit |
| `TestClaudeRunner_Run_Cancellation` | Returns context error on cancel |

#### `internal/cli/plan/run_test.go`

| Test | Description |
|------|-------------|
| `TestRunCmd_MissingArg` | Errors when no plan name provided |
| `TestRunCmd_NotInitialized` | Errors when .rafa/ doesn't exist |
| `TestRunCmd_PlanNotFound` | Errors when plan doesn't exist |
| `TestRunCmd_ClaudeNotAvailable` | Errors when Claude CLI missing |

### Integration Tests

**File**: `internal/cli/plan/run_integration_test.go`

| Test | Description |
|------|-------------|
| `TestRunPlan_EndToEnd` | Full flow with mocked Claude runner |
| `TestRunPlan_ResumeInterrupted` | Resume from interrupted state |
| `TestRunPlan_FailureAndRetry` | Fail, retry, succeed flow |
| `TestRunPlan_ResumeFailedPlan` | Resume failed plan (resets failed task to pending) |
| `TestRunPlan_ResumeAfterCancel` | Resume after Ctrl+C (task reset to pending) |
| `TestRunPlan_ConcurrentRunBlocked` | Second run blocked by lock file |

### Test Utilities

Extend `internal/testutil/mock.go`:

```go
// MockRunner implements executor.Runner for testing.
type MockRunner struct {
    Responses []error // Sequence of responses for each Run call
    CallCount int
    Calls     []MockRunnerCall
}

type MockRunnerCall struct {
    Task        *plan.Task
    PlanContext string
    Attempt     int
    MaxAttempts int
}

func (m *MockRunner) Run(ctx context.Context, task *plan.Task, planContext string, attempt, maxAttempts int) error {
    m.Calls = append(m.Calls, MockRunnerCall{
        Task:        task,
        PlanContext: planContext,
        Attempt:     attempt,
        MaxAttempts: maxAttempts,
    })
    if m.CallCount >= len(m.Responses) {
        return nil
    }
    err := m.Responses[m.CallCount]
    m.CallCount++
    return err
}
```

## Edge Cases

| Case | How it's handled |
|------|------------------|
| `.rafa/` doesn't exist | Error: "rafa not initialized. Run `rafa init` first" |
| `.rafa/plans/` doesn't exist | Error: "no plans found. Run `rafa plan create <design.md>` first" |
| Plan name not found | Error: "plan not found: <name>" |
| Multiple plans match name | Error: "multiple plans match '<name>': [list]" |
| plan.json corrupted | Error: "failed to parse plan: <details>" |
| All tasks already completed | Print "All tasks already completed." and exit |
| Plan previously failed | Reset failed task to pending and resume (attempts preserved) |
| Plan already running | Error: "plan is already running (pid X). If stale, delete run.lock" |
| Stale lock file | Detect dead process, remove lock, proceed |
| Claude CLI not installed | Error: "Claude Code CLI not found..." |
| Ctrl+C or SIGTERM during execution | Reset task to pending, log plan_cancelled, release lock, print resume instructions |
| Task fails 10 times | Mark task failed, plan failed, release lock, stop execution |
| File write fails | Return error, don't corrupt state |

## Performance

- Single-threaded, sequential execution (by design)
- State saved after each task state change (not batched)
- Progress log appended (not rewritten)
- No in-memory buffering of large outputs

## Rollout

1. Implement state management (`internal/plan/state.go`, `progress.go`)
2. Implement runner (`internal/executor/runner.go`)
3. Implement executor (`internal/executor/executor.go`)
4. Implement CLI command (`internal/cli/plan/run.go`)
5. Add tests for each component
6. Manual testing with real plans

## Trade-offs

### Exit code for success/failure detection

**Decision**: Trust Claude's exit code (0 = success, non-zero = failure)

**Alternatives considered**:
- Parse Claude output for explicit success/failure markers
- Have Claude write a status file

**Why this approach**: Simpler, and Claude Code already exits with appropriate codes. If this proves unreliable, we can add output parsing later.

### Streaming vs capturing output

**Decision**: Stream Claude output directly to stdout

**Alternatives considered**:
- Capture to buffer, write to output.log, and display
- Tee to both stdout and file

**Why this approach**: Simplest for dogfooding. Users see what's happening in real-time. We can add output.log capture later.

### Atomic state saves

**Decision**: Write to unique temp file (using PID), then rename

**Why**: Prevents corrupted plan.json if process is killed mid-write. Using PID in temp filename avoids collisions if somehow multiple processes run concurrently.

### Failed plan resumption

**Decision**: Automatically reset failed tasks to pending when re-running a failed plan

**Alternatives considered**:
- Require `--continue` flag to resume failed plans
- Require manual JSON editing to reset task status

**Why this approach**: Better UX for dogfooding. User can simply re-run the command after fixing issues. The attempt counter is preserved so the new agent knows this is a retry.

### Concurrent run protection

**Decision**: Use a lock file (`run.lock`) with PID to prevent concurrent runs of the same plan. Lock is created atomically using `O_EXCL|O_CREATE` flags to prevent race conditions.

**Alternatives considered**:
- No protection (trust user not to run twice)
- File locking with `flock` syscall
- Non-atomic lock creation (read-then-write)

**Why this approach**: Simple, portable, and provides clear error messages. The PID allows detecting stale locks from crashed processes. Using `O_EXCL` ensures atomic creation - if two processes race, only one succeeds. Using `flock` would be more robust but adds complexity and platform-specific code.

### Task status on cancellation

**Decision**: Reset in_progress task back to pending when cancelled (Ctrl+C or SIGTERM)

**Why**: Ensures clean resume. Without this, the task would remain in_progress and `FirstPendingTask()` would find it, but the attempts counter would already be incremented, leading to confusing behavior. SIGTERM is handled alongside SIGINT for robust termination in deployment scenarios.

## Open Questions

None - this is a minimal implementation focused on enabling dogfooding. Refinements will come from usage.
