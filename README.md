# Rafa

A task loop runner for AI coding agents. Implements Geoffrey Huntley's [Ralph Wiggum](https://ghuntley.com/ralph/) technique and Anthropic's [recommendations for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) with structure and monitoring.

## What it does

Rafa helps you implement a technical design by running AI agents in a loop until each task succeeds. You write the design, Rafa handles the execution. One agent, one task at a time.

## Philosophy

You own the design. Rafa owns the execution.

Fresh context on every task. Fresh context on every retry. Agents commit after each completed task. Progress is tracked. You can walk away.

Users only review after plans are implemented.

## Status

**v0.3.1** - Experimental. TUI version, dogfooding.

## Prerequisites

- Git (repository must be initialized)
- [Claude Code](https://claude.ai/code) installed and authenticated

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh
```

To upgrade, run the same command again.

### Building from Source

```bash
git clone https://github.com/pablasso/rafa.git
cd rafa
make build
# Optionally install to PATH
make install
```

## Getting Started

### Launch the TUI

```bash
rafa
```

Note: Rafa is TUI-only in this release. Run `rafa` with no arguments.

The interactive interface guides you through creating and running plans.
Rafa will create `.rafa/` automatically when you save your first plan.

## Usage

### Interactive TUI (Default)

Run `rafa` with no arguments to launch the interactive interface:

```bash
rafa
```

The TUI allows you to:
- Create new plans from design documents
- Run existing plans with real-time progress
- Monitor task execution with split-view output

**Navigation:**
- Arrow keys or `j`/`k` to navigate
- `Enter` to select
- `Esc` to go back
- `Ctrl+C` to cancel or quit

### Demo Mode

Demo mode is opt-in and launched with `--demo`.

Run demo (default demo mode):

```bash
rafa --demo
# equivalent to:
rafa --demo --demo-mode=run
```

Create-plan demo (unsaved, create view only):

```bash
rafa --demo --demo-mode=create
```

Create-plan demo behavior:
- Replays realistic create-plan activity in the Create Plan view
- Extracts and validates `PLAN_APPROVED_JSON` from replayed output
- Does **not** write `.rafa/plans/.../plan.json`
- Does **not** auto-transition to running view

Optional flags:
- `--demo-preset=quick|medium|slow` (run and create demo pacing)
- `--demo-scenario=success|flaky|fail` (run demo only; not valid with `--demo-mode=create`)

### Creating a Plan

Select **Create Plan** in the TUI, pick a design document, and follow the prompts.
After creation succeeds, Rafa stays on a success screen. Press `Enter` to return Home, then use **Run Plan** whenever you want to execute it.

Plans are stored in `.rafa/plans/<id>-<name>/` with:

- `plan.json` - Plan state and task definitions
- `progress.log` - Event log (JSON lines)
- `output.log` - Agent output (not yet captured)

### Running a Plan

Select **Run Plan** in the TUI, pick a plan, and Rafa will execute tasks sequentially. Each task runs in a fresh Claude Code session with the task context, description, and acceptance criteria.

**Behavior:**

- Starts from the first pending task (skips completed ones)
- Retries failed tasks up to 5 times with fresh agent sessions
- Saves state after each task status change
- Handles Ctrl+C gracefully (resets current task to pending)

### Resuming a Plan

Select the same plan again from **Run Plan**. Rafa automatically resumes from the first incomplete task. If a task previously failed (hit max attempts), it resets to pending and continues retrying.

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

## Uninstall

Remove the binary:

```bash
rm $(which rafa)
```

Remove Rafa data from a specific repository:

```bash
rm -rf .rafa/
```

## Not Yet Implemented

- Output capture to `output.log`
- AGENTS.md suggestions post-run
- Human input detection
- Headless/CI mode

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and release process.

## License

[MIT](LICENSE)
