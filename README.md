# Rafa

A task loop runner for AI coding agents. Implements Geoffrey Huntley's [Ralph Wiggum](https://ghuntley.com/ralph/) technique and Anthropic's [recommendations for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) with structure and monitoring.

## What it does

Rafa helps you implement a technical design by running AI agents in a loop until each task succeeds. You write the design, Rafa handles the execution. One agent, one task at a time.

## Philosophy

You own the design. Rafa owns the execution.

Fresh context on every task. Fresh context on every retry. Agents commit after each completed task. Progress is tracked. You can walk away.

Users only review after plans are implemented.

## Status

**v0.1.0** - Experimental. This is a minimal version ready for dogfooding.

## Requirements

- [Claude Code CLI](https://claude.ai/code) installed and authenticated
- Go 1.21+ (for building from source)

## Installation

> **Not yet implemented.** For now, build from source:

```bash
git clone https://github.com/pablasso/rafa.git
cd rafa
go build -o rafa ./cmd/rafa
# Optionally move to PATH
mv rafa /usr/local/bin/
```

## Quick Start

```bash
# 1. Initialize rafa in your project
mkdir -p .rafa/plans

# 2. Create a plan from a technical design
rafa plan create docs/technical-design.md

# 3. Run the plan
rafa plan run my-feature
```

## Usage

### Creating a Plan

```bash
rafa plan create <design.md> [--name <name>] [--dry-run]
```

Takes a markdown file (technical design or PRD) and uses Claude to extract discrete tasks with acceptance criteria.

**Options:**

- `--name` - Override the plan name (default: extracted from document or filename)
- `--dry-run` - Preview the plan without saving

**Example:**

```bash
rafa plan create docs/designs/auth-system.md --name auth-v2
```

**Output:**

```
Creating plan from: docs/designs/auth-system.md
Extracting tasks...

Plan created: abc123-auth-v2

  3 tasks extracted:

  t01: Implement user authentication endpoint
  t02: Add session management
  t03: Create login UI component

Run `rafa plan run auth-v2` to start execution.
```

Plans are stored in `.rafa/plans/<id>-<name>/` with:

- `plan.json` - Plan state and task definitions
- `progress.log` - Event log (JSON lines)
- `output.log` - Agent output (not yet captured)

### Running a Plan

```bash
rafa plan run <name>
```

Executes tasks sequentially. Each task runs in a fresh Claude Code session with the task context, description, and acceptance criteria.

**Behavior:**

- Starts from the first pending task (skips completed ones)
- Retries failed tasks up to 10 times with fresh agent sessions
- Saves state after each task status change
- Handles Ctrl+C gracefully (resets current task to pending)

**Example:**

```bash
rafa plan run auth-v2
```

**Output:**

```
Task 1/3: Implement user authentication endpoint [Attempt 1/10]
[Claude Code output streams here...]
Task 1/3 completed.

Task 2/3: Add session management [Attempt 1/10]
[Claude Code output streams here...]
Task 2/3 failed (attempt 1/10): claude exited with error: exit status 1
Spinning up fresh agent for retry...

Task 2/3: Add session management [Attempt 2/10]
[Claude Code output streams here...]
Task 2/3 completed.

...

Plan complete: 3/3 tasks succeeded in 01:23:45.
```

### Resuming a Plan

Just run the same command again:

```bash
rafa plan run auth-v2
```

Rafa automatically resumes from the first incomplete task. If a task previously failed (hit max attempts), it resets to pending and continues retrying.

### Cancelling a Run

Press `Ctrl+C` during execution. Rafa will:

1. Stop the current task
2. Reset it to pending (so it resumes cleanly)
3. Save state
4. Release the lock

## Monitoring & Debugging

### Check Plan State

```bash
cat .rafa/plans/*-<name>/plan.json | jq
```

Key fields:

- `status` - `not_started`, `in_progress`, `completed`, `failed`
- `tasks[].status` - `pending`, `in_progress`, `completed`, `failed`
- `tasks[].attempts` - Number of attempts made

### Watch Progress Log

```bash
tail -f .rafa/plans/*-<name>/progress.log | jq
```

Events logged:

- `plan_started` - Plan execution began
- `task_started` - Task attempt started (includes attempt number)
- `task_completed` - Task succeeded
- `task_failed` - Task attempt failed
- `plan_completed` - All tasks done
- `plan_cancelled` - User interrupted
- `plan_failed` - Task exhausted max attempts

### Debug a Stuck Task

1. Check the task's acceptance criteria in `plan.json`
2. Review recent events in `progress.log`
3. Manually run the task prompt to debug:

```bash
# Extract the task and run it manually
cat .rafa/plans/*-<name>/plan.json | jq '.tasks[0]'
```

### Lock Issues

If rafa reports "plan is already running" but nothing is running:

```bash
rm .rafa/plans/*-<name>/run.lock
```

## Plan Structure

```
.rafa/
  plans/
    abc123-my-feature/
      plan.json        # Plan state
      progress.log     # Event log (JSON lines)
      output.log       # Agent output (placeholder)
      run.lock         # Lock file (exists during execution)
```

### plan.json

```json
{
  "id": "abc123",
  "name": "my-feature",
  "description": "Implement the new feature",
  "sourceFile": "docs/design.md",
  "createdAt": "2024-01-15T10:00:00Z",
  "status": "in_progress",
  "tasks": [
    {
      "id": "t01",
      "title": "Implement endpoint",
      "description": "Create the REST endpoint...",
      "acceptanceCriteria": ["Tests pass", "Endpoint returns 200"],
      "status": "completed",
      "attempts": 1
    }
  ]
}
```

## Not Yet Implemented

- `rafa init` / `rafa deinit` - Manual `.rafa/plans/` creation for now
- TUI with live updates - Console output only
- Output capture to `output.log`
- AGENTS.md suggestions post-run
- Human input detection
- Headless/CI mode
- Binary releases

## Development

```bash
# Run tests
make test

# Format code
make fmt

# Check formatting
make check-fmt
```

## License

MIT
