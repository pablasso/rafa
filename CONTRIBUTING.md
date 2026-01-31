# Contributing to Rafa

## Development Setup

Prerequisites:
- Go 1.21+
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
- `make install` - Install to PATH
- `make release-dry-run` - Test release locally

### Hot Reload

For live TUI development with auto-restart on file changes:

```bash
# One-time setup
sudo apt install inotify-tools  # Linux
brew install fswatch            # macOS

# Terminal 1: Run TUI in a loop
make dev       # or: make dev-demo

# Terminal 2: Watch and rebuild
make watch     # Linux
make watch-mac # macOS
```

Edit any `.go` file and save - the TUI restarts automatically with your changes.

To exit, close the terminal tab/pane.

### Code Formatting

Run `make fmt` before committing. CI checks formatting.

## Demo Mode

For TUI development without Claude authentication:

```bash
rafa demo [--scenario=<SCENARIO>] [--speed=<SPEED>] [--tasks=N] [--task-delay=DURATION]
```

**Scenarios:**

| Scenario | Behavior |
|----------|----------|
| success  | All tasks pass (default) |
| mixed    | Some pass, some fail, some retry |
| fail     | All tasks fail |
| retry    | Tasks fail twice, succeed on 3rd attempt |

**Speed Presets:**

| Speed    | Delay | Tasks | Duration | Use case |
|----------|-------|-------|----------|----------|
| fast     | 500ms | 5     | ~2.5s    | Quick dev iteration |
| normal   | 10s   | 18    | ~3 min   | Default demos |
| slow     | 30s   | 60    | ~30 min  | Presentations |
| marathon | 1m    | 120   | ~2 hrs   | Long demos |
| extended | 2m    | 360   | ~12 hrs  | All-day |

**Override Flags:**

- `--tasks=N` - Override task count from speed preset
- `--task-delay=DURATION` - Override delay (e.g., "30s", "1m", "2m30s")

**Examples:**

```bash
rafa demo                              # normal: 18 tasks, 10s each
rafa demo --speed=marathon             # 120 tasks, 1m each (~2 hrs)
rafa demo --speed=marathon --tasks=60  # 60 tasks, 1m each (~1 hr)
rafa demo --task-delay=3m              # 18 tasks, 3m each
```

**Development with demo flags:**

```bash
make dev-demo DEMO_ARGS="--speed=marathon"
make dev-demo DEMO_ARGS="--speed=fast --tasks=10"
```

Demo mode is useful for:
- Iterating on TUI changes without API calls
- Testing UI states (success, failure, retry)
- Demonstrating Rafa to others
- Long-running demos for presentations or displays

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

### Recovering from a Crash

Remove the lock file and resume the plan:

```bash
rm .rafa/plans/*<plan-name>/run.lock
rafa plan run <plan-name>
```

The plan will resume from the first pending task.

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
