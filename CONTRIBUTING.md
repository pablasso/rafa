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
rafa demo [--scenario=<SCENARIO>] [--speed=<SPEED>]
```

**Scenarios:**

| Scenario | Behavior |
|----------|----------|
| success  | All tasks pass (default) |
| mixed    | Some pass, some fail, some retry |
| fail     | All tasks fail |
| retry    | Tasks fail twice, succeed on 3rd attempt |

**Speeds:**

| Speed  | Delay per task | Use case |
|--------|----------------|----------|
| fast   | 500ms | Quick iteration |
| normal | 2s | Default viewing |
| slow   | 5s | Demos/presentations |

Demo mode is useful for:
- Iterating on TUI changes without API calls
- Testing UI states (success, failure, retry)
- Demonstrating Rafa to others

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
