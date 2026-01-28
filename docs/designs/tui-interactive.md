# Technical Design: Interactive TUI for Rafa

## Overview

Add an interactive Terminal User Interface (TUI) to Rafa that allows users to create plans from design documents and monitor execution progress - all without leaving the terminal. The TUI becomes the default entry point when running `rafa` with no arguments, while existing CLI subcommands remain available for scripting and automation.

**Related PRD**: [docs/prds/rafa-core.md](../prds/rafa-core.md)

## Goals

- Provide an intuitive TUI for creating plans and monitoring execution
- Show real-time execution progress with split-view (status + output)
- Allow file browsing to select design documents for plan creation
- Maintain backwards compatibility with existing CLI commands
- Use Bubble Tea for modern, composable TUI architecture

## Non-Goals

- Replacing CLI commands (they remain for scripting/CI)
- Editing plans within the TUI (users edit JSON directly if needed)
- Parallel plan execution
- Remote/networked plan management

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         cmd/rafa/main.go                        │
│                    (detects args, routes to TUI or CLI)         │
└─────────────────────────────────────────────────────────────────┘
                                  │
              ┌───────────────────┴───────────────────┐
              ▼                                       ▼
┌─────────────────────────┐             ┌─────────────────────────┐
│   internal/tui/         │             │   internal/cli/         │
│   (Bubble Tea app)      │             │   (existing commands)   │
└─────────────────────────┘             └─────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        TUI Components                           │
├─────────────────────────────────────────────────────────────────┤
│  internal/tui/                                                  │
│  ├── app.go           # Main Bubble Tea model, orchestrates views│
│  ├── views/                                                     │
│  │   ├── home.go      # Landing screen with options             │
│  │   ├── filepicker.go# Browse/select design files              │
│  │   ├── create.go    # Plan creation progress                  │
│  │   ├── planlist.go  # List existing plans (for run selection) │
│  │   └── run.go       # Execution monitor (split view)          │
│  ├── components/                                                │
│  │   ├── statusbar.go # Bottom status/help bar                  │
│  │   ├── progress.go  # Task progress indicator                 │
│  │   └── output.go    # Scrollable output viewport              │
│  └── styles.go        # Lip Gloss styling                       │
└─────────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Existing Internal Packages                  │
├─────────────────────────────────────────────────────────────────┤
│  internal/ai/        # Claude CLI for plan creation             │
│  internal/executor/  # Task execution (needs adaptation)        │
│  internal/plan/      # Plan storage and state                   │
│  internal/display/   # (deprecated in TUI mode)                 │
└─────────────────────────────────────────────────────────────────┘
```

## Technical Details

### Entry Point Changes

**cmd/rafa/main.go** will detect whether to launch TUI or CLI:

```go
func main() {
    // If no args (or just flags like --help), launch TUI
    // If subcommand provided, route to CLI
    if len(os.Args) == 1 {
        tui.Run()
    } else {
        cli.Execute()
    }
}
```

### New Package: internal/tui/

#### app.go - Main Application Model

```go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

type View int

const (
    ViewHome View = iota
    ViewFilePicker
    ViewCreating
    ViewPlanList
    ViewRunning
)

type Model struct {
    currentView  View
    width        int
    height       int

    // Sub-models for each view
    home         homeModel
    filePicker   filePickerModel
    creating     creatingModel
    planList     planListModel
    running      runningModel

    // Shared state
    repoRoot     string
    rafaDir      string
    err          error
}

func Run() error {
    p := tea.NewProgram(
        initialModel(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )
    _, err := p.Run()
    return err
}
```

### View Specifications

#### 1. Home View (`views/home.go`)

The landing screen when TUI launches.

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                           R A F A                               │
│                    Task Loop Runner for AI                      │
│                                                                 │
│                                                                 │
│                    [c] Create new plan                          │
│                    [r] Run existing plan                        │
│                    [q] Quit                                     │
│                                                                 │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  ↑↓ Navigate  •  Enter Select  •  q Quit                        │
└─────────────────────────────────────────────────────────────────┘
```

**Behavior**:
- Check if `.rafa/` exists; if not, show "Run `rafa init` first" message
- Keyboard: `c` or select "Create" → FilePicker view
- Keyboard: `r` or select "Run" → PlanList view
- Keyboard: `q` → quit

#### 2. File Picker View (`views/filepicker.go`)

Browse filesystem to select a design document.

```
┌─────────────────────────────────────────────────────────────────┐
│  Select Design Document                                         │
├─────────────────────────────────────────────────────────────────┤
│  📁 ..                                                          │
│  📁 docs/                                                       │
│  📁 src/                                                        │
│  📄 README.md                                                   │
│  📄 design.md                              ← highlighted        │
│  📄 prd.md                                                      │
│                                                                 │
│                                                                 │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  ↑↓ Navigate  •  Enter Select  •  Esc Back  •  / Filter         │
└─────────────────────────────────────────────────────────────────┘
```

**Behavior**:
- Use `github.com/charmbracelet/bubbles/filepicker`
- Start in repo root
- Filter to show `.md` files by default (configurable)
- On file select → transition to Creating view
- Esc → back to Home

#### 3. Creating View (`views/create.go`)

Shows progress while AI extracts tasks from design.

```
┌─────────────────────────────────────────────────────────────────┐
│  Creating Plan                                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Source: docs/designs/feature-auth.md                           │
│                                                                 │
│  ⣾ Extracting tasks from design...                              │
│                                                                 │
│                                                                 │
│                                                                 │
│                                                                 │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  Please wait...  •  Ctrl+C Cancel                               │
└─────────────────────────────────────────────────────────────────┘
```

On completion:
```
┌─────────────────────────────────────────────────────────────────┐
│  Plan Created                                                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ✓ Plan created: xK9pQ2-feature-auth                            │
│                                                                 │
│  Tasks:                                                         │
│    1. Set up authentication middleware                          │
│    2. Implement user login endpoint                             │
│    3. Add session management                                    │
│    4. Write integration tests                                   │
│                                                                 │
│  [r] Run this plan now    [h] Return to home                    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  r Run plan  •  h Home  •  q Quit                               │
└─────────────────────────────────────────────────────────────────┘
```

**Behavior**:
- Spinner while `ai.ExtractTasks()` runs in background goroutine
- Send results via Bubble Tea message
- On success: show summary, offer to run immediately
- On error: show error, allow retry or go back

#### 4. Plan List View (`views/planlist.go`)

Select an existing plan to run.

```
┌─────────────────────────────────────────────────────────────────┐
│  Select Plan to Run                                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ● xK9pQ2-feature-auth       4 tasks   not_started             │
│  ○ mN3jL7-refactor-db        6 tasks   completed                │
│  ○ pR8wK1-add-tests          3 tasks   in_progress (2/3)        │
│                                                                 │
│                                                                 │
│                                                                 │
│                                                                 │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  ↑↓ Navigate  •  Enter Run  •  Esc Back                         │
└─────────────────────────────────────────────────────────────────┘
```

**Behavior**:
- List all plans from `.rafa/plans/`
- Show status, task count, progress
- Enter on selection → transition to Running view
- Highlight runnable plans (not_started or in_progress)

#### 5. Running View (`views/run.go`)

Split-view execution monitor - the most complex view.

```
┌─────────────────────────────────────────────────────────────────┐
│  Running: xK9pQ2-feature-auth                                   │
├───────────────────────────────────┬─────────────────────────────┤
│  Progress                         │  Output                     │
│                                   │                             │
│  Task 2/4: Implement login        │  Analyzing codebase...      │
│  Attempt: 1/5                     │  Found auth/ directory      │
│  Elapsed: 00:03:42                │  Creating login handler...  │
│                                   │  Writing tests...           │
│  ■■■■□□□□ 50%                     │  > npm test                 │
│                                   │  PASS src/auth/login.test   │
│  Tasks:                           │  All tests passed           │
│  ✓ 1. Set up middleware           │                             │
│  ⣾ 2. Implement login      ←      │  Committing changes...      │
│  ○ 3. Add session mgmt            │                             │
│  ○ 4. Write integration tests     │                             │
│                                   │                             │
├───────────────────────────────────┴─────────────────────────────┤
│  Running...  •  Ctrl+C Cancel                                   │
└─────────────────────────────────────────────────────────────────┘
```

**Layout**:
- Left panel (40%): Progress information
  - Current task number and title
  - Attempt counter
  - Elapsed time
  - Progress bar
  - Task list with status indicators
- Right panel (60%): Scrollable output viewport
  - Real-time output from Claude CLI
  - Auto-scrolls to bottom
  - Can scroll up to review (pauses auto-scroll)

**Status Indicators**:
- `✓` completed
- `⣾` in progress (spinner)
- `○` pending
- `✗` failed

**Behavior**:
- Execute plan using adapted `executor.Executor`
- Stream output to viewport via channel
- Update progress on task state changes
- Ctrl+C → graceful cancellation, return to home with status
- On completion → show summary, return to home

### Executor Adaptation

The existing `internal/executor/` needs changes to work with TUI:

```go
// New interface for TUI integration
type ExecutorEvents interface {
    OnTaskStart(taskNum, total int, task *plan.Task, attempt int)
    OnTaskComplete(task *plan.Task)
    OnTaskFailed(task *plan.Task, attempt int, err error)
    OnOutput(line string)
    OnPlanComplete(succeeded, total int, duration time.Duration)
    OnPlanFailed(task *plan.Task, reason string)
}

// Executor gains event callback support
type Executor struct {
    // ... existing fields ...
    events ExecutorEvents  // nil for CLI mode (uses display.Display)
}

func (e *Executor) WithEvents(events ExecutorEvents) *Executor {
    e.events = events
    return e
}
```

The TUI's running view implements `ExecutorEvents` and translates events to Bubble Tea messages.

### Output Streaming

For real-time output in the TUI:

```go
// internal/executor/output.go additions

type StreamingOutput struct {
    logFile    *os.File
    eventsChan chan string  // For TUI consumption
}

func (s *StreamingOutput) Write(p []byte) (n int, err error) {
    // Write to log file
    n, err = s.logFile.Write(p)

    // Send to TUI if channel exists
    if s.eventsChan != nil {
        // Non-blocking send, drop if buffer full
        select {
        case s.eventsChan <- string(p):
        default:
        }
    }
    return
}
```

### Dependencies

Add to `go.mod`:

```
github.com/charmbracelet/bubbletea v1.x
github.com/charmbracelet/bubbles v0.x   // For filepicker, spinner, viewport
github.com/charmbracelet/lipgloss v1.x  // For styling
```

### Key Components

#### StatusBar (`components/statusbar.go`)

Consistent bottom bar across all views showing contextual help.

```go
type StatusBar struct {
    items []string  // e.g., ["↑↓ Navigate", "Enter Select", "q Quit"]
}
```

#### Progress (`components/progress.go`)

Reusable progress bar component.

```go
type Progress struct {
    current int
    total   int
    width   int
}
```

#### Output Viewport (`components/output.go`)

Wrapper around `bubbles/viewport` with auto-scroll behavior.

```go
type OutputViewport struct {
    viewport viewport.Model
    autoScroll bool
    lines    []string
}
```

## Edge Cases

| Case | How it's handled |
|------|------------------|
| `.rafa/` doesn't exist | Home view shows message to run `rafa init` first |
| No plans exist | Plan list shows "No plans. Create one first." with shortcut |
| Plan is locked (running elsewhere) | Show error, don't allow selection |
| Terminal too small | Show minimum size warning, degrade gracefully |
| Claude CLI not found | Show error on first action that needs it |
| Output buffer overflow | Ring buffer with configurable size, old lines dropped |
| Ctrl+C during creation | Cancel extraction, return to file picker |
| Ctrl+C during execution | Graceful stop, save state, show summary |
| Network timeout during extraction | Show error with retry option |

## Security

- No new security concerns; TUI uses same underlying execution as CLI
- File picker is constrained to repo root by default
- No remote connections or external data transmission

## Performance

- Output streaming uses buffered channel (1000 lines) to prevent blocking
- Viewport renders only visible lines
- File picker lazy-loads directory contents
- Plan list caches results, refreshes on focus

## Testing

### Unit Tests
- Each view model has tests for state transitions
- Component tests for progress bar, status bar rendering
- Mock executor events for running view tests

### Integration Tests
- Full TUI flow with mock AI responses
- Keyboard navigation through all views
- Window resize handling

### Manual Testing
- Test on various terminal sizes
- Test with tmux/screen
- Test keyboard shortcuts don't conflict with common terminal bindings

## Rollout

1. **Phase 1**: Implement TUI framework and Home view
2. **Phase 2**: Add FilePicker and Creating views (plan creation flow)
3. **Phase 3**: Add PlanList and Running views (execution flow)
4. **Phase 4**: Polish, edge cases, testing
5. **Release**: TUI becomes default, CLI remains available

No feature flags needed - TUI and CLI can coexist from the start.

## Trade-offs

### Bubble Tea vs tview
**Chose Bubble Tea because**:
- Elm-inspired architecture fits well with Go
- Excellent documentation and active community
- Composable components (bubbles library)
- Better handling of async operations
- Modern, clean API

**tview would have been better for**:
- Complex multi-panel layouts (though Bubble Tea handles this fine)
- If we needed traditional widgets like forms, tables

### Split View vs Tabs
**Chose split view because**:
- Users can see progress AND output simultaneously
- No context switching during execution
- Matches common IDE/terminal patterns

**Tabs would have been better for**:
- Very narrow terminals
- If we had more than 2 distinct panels

### Adapting Executor vs New TUI-Specific Executor
**Chose adaptation because**:
- Single source of truth for execution logic
- CLI and TUI stay in sync automatically
- Less code duplication

**New executor would have been better for**:
- If TUI needed fundamentally different execution model
- If CLI execution logic was too entangled with display

## Open Questions

None at this time.
