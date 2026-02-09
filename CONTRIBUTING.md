# Contributing to Rafa

This guide is for contributors and maintainers. For installation and day-to-day usage, see the README.

## Development Setup

Prerequisites:
- Go 1.25.6+
- Git
- Make

Building from source:

```bash
git clone https://github.com/pablasso/rafa.git
cd rafa
make build
```

## Development Workflow

### Make Commands

- `make test` - Run tests
- `make fmt` - Format code
- `make check-fmt` - Check formatting
- `make build` - Build binary
- `make release-dry-run` - Test release locally

### Hot Reload

For live TUI development with auto-restart on file changes:

```bash
# One-time setup
sudo apt install inotify-tools  # Linux
brew install fswatch            # macOS

# Terminal 1: Run TUI in a loop
make dev

# Terminal 2: Watch and rebuild
make watch     # Linux
make watch-mac # macOS
```

Edit any `.go` file and save - the TUI restarts automatically with your changes.

To exit, close the terminal tab/pane.

### Demo Mode (TUI)

Demo mode is opt-in via `--demo`. It is useful to make TUI changes without having to create or run real plans.

1. Build or run the dev loop:
   ```bash
   make build
   ./bin/rafa --demo
   ```
   You can also use `make dev` for hot reload.
2. Demo starts automatically and shows `[DEMO]` in the status bar.

Modes:

- `--demo-mode=run` (default): replay running view execution
- `--demo-mode=create`: replay create-plan view extraction (demo-only, unsaved)

Optional flags:

- `--demo-preset=quick|medium|slow` (default: `medium`)
- `--demo-scenario=success|flaky|fail` (run mode only)

Fixtures:

- Run demo fixture: `internal/demo/fixtures/default.v1.json`
- Create demo fixture: `internal/demo/fixtures/create.default.v1.json`

If fixture loading fails, Rafa falls back to in-memory demo data and shows a warning in the UI.

Refresh run fixture from a real run (developer-only):

```bash
go run ./scripts/gen_demo_fixture.go
```

Refresh create fixture from a captured create-plan stream (developer-only):

```bash
go run ./scripts/gen_demo_create_fixture.go \
  --source-doc docs/designs/plan-create-command.md \
  --stream-log /path/to/create-stream.jsonl
```

The create fixture generator enforces a realism rule: `--source-doc` must not already be referenced as `sourceFile` in any `.rafa/plans/*/plan.json`.

## Plan Monitoring & Troubleshooting

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
3. Manually inspect the task payload:

```bash
cat .rafa/plans/*-<name>/plan.json | jq '.tasks[0]'
```

### Lock Issues

If Rafa reports "plan is already running" but nothing is running:

```bash
rm .rafa/plans/*-<name>/run.lock
```

## Debugging Crashes

Rafa has crash diagnostics to help debug unexpected crashes.

### Crash Log Locations

| Crash Type | Log Location |
|------------|--------------|
| Panic during plan execution | `.rafa/plans/<plan-id>/crash.log` |
| Signal crash (SIGSEGV, SIGBUS, SIGABRT) | `~/.rafa/crash.log` |

### Diagnosing a Crashed Plan

If a plan crashes mid-execution:

1. **Check for a crash.log in the plan directory:**
   ```bash
   cat .rafa/plans/*<plan-name>/crash.log
   ```

2. **Check the global crash log:**
   ```bash
   cat ~/.rafa/crash.log
   ```

3. **Check for stale lock files** (indicates unclean exit):
   ```bash
   ls .rafa/plans/*<plan-name>/run.lock
   ```

4. **Review the output log** for the last successful operation:
   ```bash
   tail -100 .rafa/plans/*<plan-name>/output.log
   ```

### What the Crash Logs Contain

- **Timestamp**: When the crash occurred
- **Panic/Signal**: What triggered the crash
- **Stack trace**: Full Go stack trace for debugging

### Common Crash Scenarios

| Symptom | Likely Cause | Debug Steps |
|---------|--------------|-------------|
| `run.lock` exists, no crash.log | Process killed externally (OOM, sleep, etc.) | Check system logs: `log show --predicate 'eventMessage contains "rafa"' --last 1h` |
| crash.log with panic | Bug in rafa code | File an issue with the stack trace |
| crash.log with SIGSEGV | Memory corruption or CGO issue | Check for race conditions with `go test -race ./...` |

### Debugging Silent Crashes

If a crash leaves no crash.log (process killed externally or panic in unprotected goroutine), use Go's stack trace controls:

**Capture panic traces from TUI mode:**
```bash
GOTRACEBACK=crash ./bin/rafa 2>/tmp/rafa-crash.log
```

Navigate the TUI normally. If it crashes, check `/tmp/rafa-crash.log` for the stack trace.

**GOTRACEBACK values:**
| Value | Behavior |
|-------|----------|
| `none` | No stack trace |
| `single` | Current goroutine only (default) |
| `all` | All goroutines |
| `crash` | All goroutines + core dump |

**If the log file is empty after crash:**
The process was killed by an external signal, not a Go panic. Use system tools:
```bash
# macOS - trace system calls
sudo dtruss -f ./bin/rafa 2>&1 | tee /tmp/rafa-dtruss.log

# Check exit code
./bin/rafa; echo "Exit code: $?"
```

**Dump goroutine stacks without crashing (send to running process):**
```bash
kill -SIGQUIT $(pgrep rafa)
```

### Recovering from a Crash

Remove the lock file and resume the plan:

```bash
rm .rafa/plans/*<plan-name>/run.lock
```

Then launch `rafa` and choose **Run Plan**. The plan will resume from the first pending task.

## Testing

Run the test suite:

```bash
make test
```

## Releasing

Tag-based releases via GitHub Actions:

```bash
make release VERSION=v0.x.x
```

This runs tests, creates the tag, and pushes it. GitHub Actions builds and publishes automatically.

The changelog is auto-generated from commit messages.
