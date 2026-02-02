# Rafa

A task loop runner for AI coding agents. Rafa executes tasks extracted from technical designs, running each in a fresh agent session and committing after success.

## What It Does

Rafa helps you implement technical designs by running AI agents in a loop until each task succeeds:

1. You write a technical design document
2. Rafa extracts discrete tasks with acceptance criteria
3. Each task runs in a fresh Claude Code session
4. Progress is tracked, commits happen automatically
5. You review after implementation is complete

**Philosophy**: You own the design. Rafa owns the execution. Fresh context on every task. Fresh context on every retry.

## Prerequisites

- [Claude Code](https://claude.ai/code) installed and authenticated
- Git repository initialized

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh
```

The installer:
- Detects your OS and architecture (macOS/Linux, arm64/x64)
- Downloads the pre-built binary from GitHub releases
- Verifies the checksum
- Installs to `~/.local/bin/rafa`
- Updates your PATH if needed

To install a specific version:
```bash
curl -fsSL https://raw.githubusercontent.com/pablasso/rafa/main/scripts/install.sh | sh -s -- -v v0.1.0
```

To upgrade, run the install command again.

### Install via npm

```bash
npm install -g rafa
```

Requires Node.js 20 or later.

### Build from Source

```bash
git clone https://github.com/pablasso/rafa.git
cd rafa/ts-port
npm install
npm run build
npm link  # Optional: makes 'rafa' available globally
```

## Quick Start

### 1. Initialize Rafa in Your Repository

```bash
cd your-project
rafa init
```

This creates a `.rafa/` directory and installs the skills needed for PRD and design creation.

### 2. Create a Technical Design

Option A: Use Rafa's guided flow
```bash
rafa design
```

Option B: Write your own design document in `docs/designs/your-feature.md`

### 3. Create a Plan from Your Design

```bash
rafa plan create docs/designs/your-feature.md
```

Rafa uses Claude to extract discrete tasks with acceptance criteria from your design document.

### 4. Run the Plan

Launch the TUI and select your plan:
```bash
rafa
```

Or run directly:
```bash
rafa plan run your-feature
```

Watch as Rafa executes each task, retrying failures up to 5 times with fresh agent sessions.

## Command Reference

### TUI Mode

```bash
rafa                    # Launch interactive TUI
```

The TUI provides:
- Home menu for all workflows
- Real-time task execution view
- Activity timeline showing tool usage
- Plan selection and management

**Navigation:**
- Arrow keys or `j`/`k` to navigate
- `Enter` to select
- `Esc` to go back
- `Ctrl+C` to cancel

### CLI Commands

```bash
rafa                           # Launch TUI (home screen)
rafa prd [--name NAME]         # Start PRD creation
rafa design [--from PRD] [--name NAME]  # Start design creation
rafa plan create [FILE]        # Create plan from design
rafa plan run [NAME]           # Execute plan
rafa plan list                 # List plans
rafa init                      # Initialize repository
rafa deinit [--force]          # Remove Rafa from repository
```

### Command Details

#### `rafa init`

Initializes Rafa in the current repository:
- Creates `.rafa/` directory structure
- Fetches skills from GitHub (prd, technical-design, code-review)
- Sets up `.gitignore` entries

#### `rafa prd`

Starts an interactive PRD (Product Requirements Document) creation session:
```bash
rafa prd                    # Interactive naming
rafa prd --name user-auth   # Specify name upfront
```

Uses Claude to guide you through defining requirements, then saves to `docs/prds/<name>.md`.

#### `rafa design`

Starts a technical design creation session:
```bash
rafa design                      # Start fresh
rafa design --from user-auth     # Reference existing PRD
rafa design --name auth-system   # Specify name
```

Saves to `docs/designs/<name>.md`.

#### `rafa plan create`

Creates an execution plan from a design document:
```bash
rafa plan create docs/designs/auth-system.md           # From file
rafa plan create docs/designs/auth-system.md --name v2 # Custom name
rafa plan create                                        # Shows file picker
```

Options:
- `--name` - Override auto-generated plan name
- `--dry-run` - Preview extracted tasks without saving

#### `rafa plan run`

Executes a plan:
```bash
rafa plan run auth-system   # By name
```

Behavior:
- Resumes from first incomplete task
- Retries failed tasks up to 5 times
- Commits after each successful task with `[rafa]` prefix
- Handles `Ctrl+C` gracefully (resets current task to pending)

#### `rafa plan list`

Lists all plans with their status:
```bash
rafa plan list
```

Shows plan name, task count, and status indicators.

## How It Works

### Plan Execution Loop

For each task in your plan:

1. **Fresh Session**: Spawns a new Claude Code session with `--dangerously-skip-permissions`
2. **Task Context**: Provides the task description, acceptance criteria, and plan context
3. **Execution**: Claude implements the task, verifying acceptance criteria
4. **Commit**: On success, auto-commits with message extracted from Claude's output
5. **Retry**: On failure, spins up a fresh session (up to 5 attempts)

### Data Storage

```
.rafa/
├── plans/
│   └── abc123-feature-name/
│       ├── plan.json       # Plan state and tasks
│       ├── progress.jsonl  # Event log
│       └── output.log      # Agent output
├── sessions/               # Conversation sessions (gitignored)
└── settings.json           # User settings
```

### Plan JSON Structure

```json
{
  "id": "abc123",
  "name": "feature-name",
  "description": "Implement the feature",
  "sourceFile": "docs/designs/feature.md",
  "status": "in_progress",
  "tasks": [
    {
      "id": "t01",
      "title": "Create database schema",
      "description": "...",
      "acceptanceCriteria": ["Migration runs", "Tests pass"],
      "status": "completed",
      "attempts": 1
    }
  ]
}
```

## Detailed Documentation

For comprehensive behavior specifications, see the PRDs:

- **[rafa-core.md](docs/prds/rafa-core.md)** - Plan execution, retry logic, task prompts
- **[rafa-workflow.md](docs/prds/rafa-workflow.md)** - PRD/Design creation, session management

## Uninstall

Remove the binary:
```bash
rm $(which rafa)
```

Remove Rafa from a repository:
```bash
rafa deinit
```

Or manually:
```bash
rm -rf .rafa/
```

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup.

## License

[MIT](LICENSE)
