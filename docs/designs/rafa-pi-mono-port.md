# Technical Design: Rafa Port to Pi-Mono

## Overview

Port Rafa from Go/Bubble Tea to TypeScript using the pi-mono framework. Rafa remains a standalone TUI application that orchestrates the full development workflow: PRD creation, technical design, plan extraction, and task execution. The actual coding work continues to be performed by Claude Code CLI (`claude -p`).

**PRDs**: [rafa-core.md](../prds/rafa-core.md), [rafa-workflow.md](../prds/rafa-workflow.md)

## Goals

- Rewrite Rafa in TypeScript using pi-mono's TUI and patterns
- Maintain all existing functionality (PRD, design, plan, run workflows)
- Continue using Claude Code CLI for agent execution
- Leverage pi-mono's session management patterns for conversation persistence
- Improve code maintainability by using a proven framework

## Non-Goals

- Direct LLM API access (we keep using `claude -p`)
- Becoming an extension to pi-coding-agent
- Changing the core Rafa workflow or UX
- Supporting LLM providers other than Claude

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Rafa TUI                                │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐       │
│  │  HomeView     │  │  ConvoView    │  │  RunView      │       │
│  │  (menu)       │  │  (prd/design) │  │  (execution)  │       │
│  └───────────────┘  └───────────────┘  └───────────────┘       │
├─────────────────────────────────────────────────────────────────┤
│                      Core Services                              │
│  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐       │
│  │  PlanManager  │  │  SessionMgr   │  │  ClaudeRunner │       │
│  │  (plans/)     │  │  (sessions/)  │  │  (claude -p)  │       │
│  └───────────────┘  └───────────────┘  └───────────────┘       │
├─────────────────────────────────────────────────────────────────┤
│                      Pi-Mono Packages                           │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  @mariozechner/pi-tui                                     │ │
│  │  TUI, Editor, Markdown, SelectList, Box, Container, etc.  │ │
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Key Differences from Current Go Implementation

| Aspect | Current (Go) | New (TypeScript) |
|--------|--------------|------------------|
| Language | Go | TypeScript |
| TUI Framework | Bubble Tea | pi-tui |
| Build Output | Single binary | Node.js CLI (or bun compile) |
| State Format | JSON files | JSONL (pi-mono pattern) |
| Dependencies | ~10 Go modules | pi-tui + Node stdlib |

## Technical Details

### Package Structure

```
rafa/
├── src/
│   ├── index.ts              # Entry point, CLI routing
│   ├── tui/
│   │   ├── app.ts            # Main TUI container, view routing
│   │   ├── views/
│   │   │   ├── home.ts       # Home menu view
│   │   │   ├── conversation.ts # PRD/Design conversation view
│   │   │   ├── run.ts        # Plan execution view
│   │   │   ├── plan-list.ts  # Plan selection view
│   │   │   └── file-picker.ts # Design doc picker
│   │   └── components/
│   │       ├── activity-log.ts   # Tool use timeline
│   │       ├── task-progress.ts  # Task list with status
│   │       └── output-stream.ts  # Claude output display
│   ├── core/
│   │   ├── plan.ts           # Plan types and operations
│   │   ├── task.ts           # Task types
│   │   ├── session.ts        # Conversation session management
│   │   ├── claude-runner.ts  # Claude CLI invocation
│   │   ├── stream-parser.ts  # Parse stream-json output
│   │   └── progress.ts       # Progress event logging
│   ├── storage/
│   │   ├── plans.ts          # Plan CRUD operations
│   │   ├── sessions.ts       # Session persistence
│   │   └── settings.ts       # User settings
│   └── utils/
│       ├── git.ts            # Git operations
│       ├── lock.ts           # File locking
│       └── id.ts             # Short ID generation
├── package.json
└── tsconfig.json
```

### Key Components

#### 1. TUI Application (`src/tui/app.ts`)

```typescript
import { TUI, Container, Key } from "@mariozechner/pi-tui";

type ViewState = "home" | "conversation" | "run" | "plan-list" | "file-picker";

class RafaApp {
  private tui: TUI;
  private currentView: ViewState = "home";
  private views: Map<ViewState, Component>;

  constructor() {
    this.tui = new TUI();
    this.views = new Map([
      ["home", new HomeView(this)],
      ["conversation", new ConversationView(this)],
      ["run", new RunView(this)],
      // ...
    ]);
  }

  navigate(view: ViewState, context?: unknown) {
    this.currentView = view;
    this.views.get(view)?.activate(context);
    this.tui.invalidate();
  }

  async run() {
    this.tui.start();
    // Input loop handled by pi-tui
  }
}
```

#### 2. Claude Runner (`src/core/claude-runner.ts`)

Invokes Claude Code CLI and parses stream-json output. Two modes:

**Task Execution** (fresh session each time):
```typescript
async function runTask(options: TaskRunOptions): Promise<void> {
  const args = [
    "-p", options.prompt,
    "--output-format", "stream-json",
    "--dangerously-skip-permissions",
    "--verbose",
    "--include-partial-messages",
  ];

  const proc = spawn("claude", args);

  for await (const line of readline(proc.stdout)) {
    const event = JSON.parse(line);
    options.onEvent(parseClaudeEvent(event));
  }

  if (proc.exitCode !== 0) {
    throw new Error(`Claude exited with code ${proc.exitCode}`);
  }
}
```

**Conversation Mode** (resumable sessions for PRD/Design):
```typescript
async function runConversation(options: ConversationRunOptions): Promise<string> {
  const args = [
    "-p", options.prompt,
    "--output-format", "stream-json",
    "--dangerously-skip-permissions",
    "--verbose",
  ];

  if (options.sessionId) {
    args.push("--resume", options.sessionId);
  }

  const proc = spawn("claude", args);
  let newSessionId: string | null = null;

  for await (const line of readline(proc.stdout)) {
    const event = JSON.parse(line);
    // Session ID appears in init/system events - capture it during parsing
    if (event.type === "system" && event.session_id) {
      newSessionId = event.session_id;
    }
    options.onEvent(parseClaudeEvent(event));
  }

  if (!newSessionId) {
    throw new Error("No session ID found in Claude output");
  }
  return newSessionId;
}
```

**Error handling**: Any Claude error (exit code != 0, unexpected output) is treated as a failure. For task execution, this triggers a retry. For conversations, we surface the error and let user decide to retry or start fresh.

#### 3. Conversation View (`src/tui/views/conversation.ts`)

Handles PRD and Design creation with activity timeline:

```typescript
import { Container, Editor, Markdown, Box } from "@mariozechner/pi-tui";

class ConversationView implements Component {
  private activityLog: ActivityLogComponent;
  private outputPane: Markdown;
  private inputEditor: Editor;
  private phase: "prd" | "design";

  constructor(app: RafaApp, phase: "prd" | "design") {
    this.phase = phase;
    this.activityLog = new ActivityLogComponent();
    this.outputPane = new Markdown();
    this.inputEditor = new Editor({
      placeholder: "Type to revise, or press 'a' to approve...",
      maxHeight: 5,
    });
  }

  render(width: number): string[] {
    // Left pane (25%): activity timeline
    // Right pane (75%): Claude's response
    // Bottom: multi-line input
    // Footer: [a] Approve [c] Cancel
  }

  handleInput(key: Key): boolean {
    if (key.matches("a") && !this.inputEditor.hasFocus()) {
      this.approve();
      return true;
    }
    if (key.matches("c")) {
      this.cancel();
      return true;
    }
    return this.inputEditor.handleInput(key);
  }

  private async sendMessage(text: string) {
    // Add to activity log
    // Run claude with --resume
    // Stream response to outputPane
  }
}
```

#### 4. Run View (`src/tui/views/run.ts`)

Plan execution with real-time progress:

```typescript
class RunView implements Component {
  private plan: Plan;
  private currentTask: Task;
  private activityLog: ActivityLogComponent;
  private taskProgress: TaskProgressComponent;
  private outputStream: OutputStreamComponent;

  async executePlan(plan: Plan) {
    this.plan = plan;

    // Find first pending task (for resume support)
    const firstPendingIdx = plan.tasks.findIndex(t => t.status === "pending");
    if (firstPendingIdx === -1) return; // All tasks completed

    for (let i = firstPendingIdx; i < plan.tasks.length; i++) {
      const task = plan.tasks[i];
      if (task.status === "completed") continue; // Skip completed (shouldn't happen but defensive)

      this.currentTask = task;

      for (let attempt = task.attempts + 1; attempt <= MAX_ATTEMPTS; attempt++) {
        task.attempts = attempt;
        task.status = "in_progress";
        this.activityLog.clear();
        await this.savePlan(); // Persist status change

        const success = await this.executeTask(task, attempt);

        if (success) {
          task.status = "completed";
          await this.savePlan();
          await this.commitChanges(task);
          break;
        }

        if (attempt === MAX_ATTEMPTS) {
          task.status = "failed";
          plan.status = "failed";
          await this.savePlan();
          return;
        }
      }
    }

    plan.status = "completed";
    await this.savePlan();
  }

  private async executeTask(task: Task, attempt: number): Promise<boolean> {
    const prompt = buildTaskPrompt(this.plan, task, attempt);

    await runClaude({
      prompt,
      onEvent: (event) => {
        this.activityLog.addEvent(event);
        if (event.type === "text") {
          this.outputStream.append(event.data);
        }
        this.tui.invalidate();
      },
    });

    // Success determined by Claude's exit and self-verification
    return true; // or false based on result
  }
}
```

#### 5. Activity Log Component

Displays tool usage timeline parsed from stream-json:

```typescript
class ActivityLogComponent implements Component {
  private events: ActivityEvent[] = [];

  addEvent(claudeEvent: ClaudeEvent) {
    if (claudeEvent.type === "tool_use") {
      this.events.push({
        icon: "├─",
        label: claudeEvent.data.name,
        detail: claudeEvent.data.target,
        status: "running",
      });
    } else if (claudeEvent.type === "tool_result") {
      // Update last event of same tool
      const last = this.findLastOfTool(claudeEvent.data.name);
      if (last) {
        last.status = "done";
        last.duration = claudeEvent.data.duration;
      }
    }
  }

  render(width: number): string[] {
    // Tree-style activity display
    // ├─ Reading auth.go
    // ├─ Spawned Explore
    // │  └─ done 2s
    // └─ Editing login.go
  }
}
```

### Data Model

#### Plan Format (unchanged)

```typescript
interface Plan {
  id: string;           // 6-char random ID
  name: string;         // kebab-case
  description: string;
  sourceFile: string;
  createdAt: string;    // ISO-8601
  status: "not_started" | "in_progress" | "completed" | "failed";
  tasks: Task[];
}

interface Task {
  id: string;           // t01, t02, etc.
  title: string;
  description: string;
  acceptanceCriteria: string[];
  status: "pending" | "in_progress" | "completed" | "failed";
  attempts: number;
}
```

#### Session Format (new, JSONL)

```jsonl
{"type":"session","id":"abc123","phase":"prd","createdAt":"..."}
{"type":"user","id":"m1","parentId":null,"content":"I want to build..."}
{"type":"assistant","id":"m2","parentId":"m1","content":"Let me ask..."}
{"type":"tool_use","id":"m3","parentId":"m2","tool":"Read","target":"..."}
```

This matches pi-mono's session format, enabling potential future interop.

### Storage Layout

```
.rafa/
├── plans/
│   └── xK9pQ2-feature-auth/
│       ├── plan.json
│       ├── progress.jsonl    # Changed from .log
│       └── output.log
├── sessions/                 # Gitignored
│   ├── prd-user-auth.jsonl
│   └── design-user-auth.jsonl
└── settings.json
```

### CLI Commands

```bash
rafa                    # Launch TUI (home screen)
rafa prd [--name NAME]  # Start PRD creation
rafa design [--from PRD] [--name NAME]  # Start design creation
rafa plan create [FILE] # Create plan from design
rafa plan run [NAME]    # Execute plan
rafa plan list          # List plans
```

### Build & Distribution

```json
{
  "name": "rafa",
  "type": "module",
  "bin": { "rafa": "dist/index.js" },
  "scripts": {
    "build": "tsc",
    "build:binary": "bun build --compile ./dist/index.js --outfile dist/rafa"
  },
  "dependencies": {
    "@mariozechner/pi-tui": "^0.51.0"
  }
}
```

**Primary distribution: Curl-installed binary**

```bash
curl -fsSL https://rafa.dev/install.sh | sh
```

The install script:
1. Detects OS and architecture (darwin/linux, arm64/x64)
2. Downloads pre-built Bun binary from GitHub releases
3. Verifies checksum
4. Installs to `~/.local/bin/rafa` (or `/usr/local/bin` with sudo)
5. Adds to PATH if needed

**GitHub Release artifacts**:
- `rafa-darwin-arm64` (macOS Apple Silicon)
- `rafa-darwin-x64` (macOS Intel)
- `rafa-linux-arm64`
- `rafa-linux-x64`
- `checksums.txt`
- `install.sh`

**Secondary**: npm package available for Node.js users who prefer it.

### Skills Installation

Skills are fetched from GitHub during `rafa init` (same as Go version):

```typescript
async function installSkills(): Promise<void> {
  const skillsUrl = "https://github.com/pablasso/skills/archive/refs/heads/main.zip";
  const targetDir = path.join(process.cwd(), ".claude", "skills");

  // Download and extract
  const zip = await fetch(skillsUrl);
  await extract(zip, targetDir);
}
```

Skills installed: `prd`, `prd-review`, `technical-design`, `technical-design-review`, `code-review`

If GitHub is unavailable, `rafa init` fails with a clear error message. Skills are not bundled to ensure users always get the latest versions.

## Edge Cases

| Case | How it's handled |
|------|------------------|
| Claude CLI not installed | Check on startup, show install instructions |
| `--resume` fails (any error) | Surface error, offer to start fresh session |
| Stale lock file | Check if PID exists, clean up if not |
| Network failure during execution | Task marked failed, retry on next attempt |
| Ctrl+C during task | Reset task to pending, save plan state |
| Large output overflow | OutputStreamComponent auto-scrolls, truncates history |
| GitHub unavailable during init | Fail with clear error, user retries later |

## Performance

- **TUI rendering**: pi-tui's differential rendering minimizes redraws
- **Stream parsing**: Line-by-line JSONL parsing, low memory overhead
- **Session files**: Append-only JSONL, fast writes
- **Plan files**: Small JSON, atomic writes via temp+rename

## Testing

| Type | Approach |
|------|----------|
| Unit | Vitest for core logic (plan ops, stream parsing) |
| Component | TUI component rendering tests (headless terminal) |
| Integration | Mock Claude CLI, test full flows |
| E2E | Manual testing with real Claude CLI |

## Migration

### Strategy: Clean Break with Plan Compatibility

This port is a **clean break** for the Rafa binary itself, but maintains **full compatibility** with existing plan data.

### What Migrates Automatically

| Data | Format | Migration |
|------|--------|-----------|
| Plans (`.rafa/plans/*/plan.json`) | JSON | **No change** - TypeScript reads same format |
| Progress logs (`.rafa/plans/*/progress.log`) | JSON Lines | **No change** - already JSONL, just rename to `.jsonl` |
| Output logs (`.rafa/plans/*/output.log`) | Plain text | **No change** - same format |

### What Does NOT Migrate

| Data | Reason |
|------|--------|
| Sessions (`.rafa/sessions/`) | Go version uses single JSON files; TypeScript uses JSONL with tree structure. Sessions are gitignored and ephemeral anyway. |
| Skills (`.claude/skills/`) | Re-fetched on `rafa init`. No migration needed. |

### Migration Flow

When TypeScript Rafa encounters an existing `.rafa/` directory:

1. **Detect version**: Check for `.rafa/version` file (new) or absence (Go version)
2. **Plans**: Read as-is - JSON format unchanged
3. **Progress logs**: Rename `progress.log` → `progress.jsonl` on first access
4. **Sessions**: Ignore old JSON sessions - user starts fresh conversations
5. **Write version marker**: Create `.rafa/version` with `{"version": 2, "runtime": "typescript"}`

```typescript
async function migrateIfNeeded(rafaDir: string): Promise<void> {
  const versionFile = path.join(rafaDir, "version");

  if (!fs.existsSync(versionFile)) {
    // Go version - migrate progress logs
    const plans = await glob(`${rafaDir}/plans/*/progress.log`);
    for (const log of plans) {
      fs.renameSync(log, log.replace(".log", ".jsonl"));
    }

    // Clear old sessions (they're gitignored anyway)
    const sessionsDir = path.join(rafaDir, "sessions");
    if (fs.existsSync(sessionsDir)) {
      fs.rmSync(sessionsDir, { recursive: true });
      fs.mkdirSync(sessionsDir);
    }

    // Mark as migrated
    fs.writeFileSync(versionFile, JSON.stringify({ version: 2, runtime: "typescript" }));
  }
}
```

### Rollback

If a user needs to revert to Go version:
1. Rename `progress.jsonl` back to `progress.log` in each plan
2. Delete `.rafa/version` file
3. Sessions must be recreated (they're ephemeral)

## Implementation References

Before starting implementation, agents should familiarize themselves with these resources:

### Pi-Mono Framework

Clone and explore the pi-mono repository:
```bash
git clone https://github.com/badlogic/pi-mono.git
```

**Key files to study:**

| File | What to learn |
|------|---------------|
| `packages/tui/src/tui.ts` | TUI class, Component interface, render loop |
| `packages/tui/src/components/editor.ts` | Multi-line input with key handling |
| `packages/tui/src/components/markdown.ts` | Streaming markdown rendering |
| `packages/tui/src/components/select-list.ts` | Menu/list selection |
| `packages/tui/src/keys.ts` | Key parsing and matching |
| `packages/coding-agent/src/modes/interactive/` | How a full TUI app is structured |

### Current Go Implementation

Reference the existing Go code for logic that should be preserved:

| File | What to port |
|------|--------------|
| `internal/executor/runner.go` | Task prompt template (lines 54-94) |
| `internal/executor/executor.go` | Execution loop, retry logic, git commits |
| `internal/plan/storage.go` | Plan JSON read/write with atomic saves |
| `internal/plan/lock.go` | PID-based file locking |
| `internal/plan/progress.go` | Progress event logging (JSONL format) |
| `internal/git/git.go` | Git status, add, commit operations |
| `internal/ai/claude.go` | Claude CLI invocation, task extraction |
| `internal/session/session.go` | Session CRUD (adapt to JSONL) |

### Claude Code stream-json Format

The `--output-format stream-json` flag emits JSONL events. **The exact format must be discovered during implementation** by running:

```bash
claude -p "hello" --output-format stream-json --verbose
```

Expected event categories (verify actual field names):
- **System/init events**: Contain session ID for `--resume`
- **Tool events**: Tool invocations and results
- **Content events**: Streaming text output
- **Completion events**: End of response

The Go code in `internal/executor/runner.go` doesn't parse these events - it just pipes output. The TypeScript version needs to parse them for the activity timeline display.

### PRDs for User-Facing Behavior

- `docs/prds/rafa-core.md` - Plan execution, retry logic, TUI display requirements
- `docs/prds/rafa-workflow.md` - PRD/Design conversation flows, session management, TUI layouts

## Rollout

**Project Location**: Create in a new repository (`rafa-ts` or similar). The Go version remains in this repo until the TypeScript port is complete and validated. Once stable, archive the Go repo and rename the TypeScript repo to `rafa`.

### Phase 1: Core Infrastructure

**Task 1: Set up TypeScript project**
- Create new repository with directory structure as defined in Package Structure section
- Initialize package.json with `@mariozechner/pi-tui` dependency
- Configure tsconfig.json for ES modules
- Verify `npm install` and `npm run build` work
- Acceptance: `npx tsc` compiles without errors

**Task 2: Implement minimal TUI with view routing**
- Study `pi-mono/packages/tui/src/tui.ts` for TUI class usage
- Study `pi-mono/packages/coding-agent/src/modes/interactive/` for app structure
- Create `src/tui/app.ts` with TUI instance and view state machine
- Create placeholder views (HomeView, RunView, ConversationView)
- Acceptance: Running `npm start` shows a TUI with navigable views

**Task 3: Port plan and task data structures**
- Copy types from `internal/plan/plan.go` and `internal/plan/task.go`
- Implement plan loading/saving in `src/storage/plans.ts`
- Reference `internal/plan/storage.go` for atomic write pattern (temp file + rename)
- Acceptance: Can load existing `.rafa/plans/*/plan.json` files from Go version

**Task 4: Implement Claude runner with stream-json parsing**
- Create `src/core/claude-runner.ts` with `runTask()` function
- Spawn `claude -p` with flags from the Claude Runner section
- Parse JSONL stdout line-by-line
- Emit typed events (tool_use, tool_result, text, done, error)
- Reference `internal/executor/runner.go` for the prompt template
- Acceptance: Can run a simple prompt and receive parsed events

### Phase 2: Plan Execution

**Task 5: Build RunView with activity log and output stream**
- Study `pi-mono/packages/tui/src/components/markdown.ts` for text rendering
- Create `src/tui/components/activity-log.ts` for tool timeline
- Create `src/tui/components/output-stream.ts` for Claude output
- Wire into RunView with split-pane layout (25% left, 75% right)
- Acceptance: TUI displays streaming Claude output with activity sidebar

**Task 6: Port execution loop with retry logic**
- Reference `internal/executor/executor.go` lines 111-290 for the loop
- Implement task iteration with MAX_ATTEMPTS=5 retries
- Handle Ctrl+C to reset task to pending
- Save plan state after each status change
- Acceptance: Can execute a plan with retries, resume after interruption

**Task 7: Implement git integration**
- Create `src/utils/git.ts` with status, add, commit functions
- Reference `internal/git/git.go` for implementation
- Check workspace clean before execution (unless --allow-dirty)
- Commit after each task with `[rafa]` prefix
- Extract SUGGESTED_COMMIT_MESSAGE from output
- Acceptance: Tasks auto-commit on success, dirty workspace blocks execution

**Task 8: Add progress logging and file locking**
- Create `src/core/progress.ts` for JSONL event logging
- Reference `internal/plan/progress.go` for event types
- Create `src/utils/lock.ts` for PID-based locking
- Reference `internal/plan/lock.go` for stale lock detection
- Acceptance: progress.jsonl records events, concurrent runs blocked

### Phase 3: Conversation Flows

**Task 9: Build ConversationView with Editor**
- Study `pi-mono/packages/tui/src/components/editor.ts` for multi-line input
- Create ConversationView with activity log, output pane, and input editor
- Handle [a] Approve and [c] Cancel hotkeys
- Disable input while Claude is responding
- Acceptance: Can type messages, see responses, approve/cancel

**Task 10: Implement PRD creation flow**
- Reference `docs/prds/rafa-workflow.md` for PRD flow requirements
- Invoke Claude with initial prompt to use /prd skill
- After draft, auto-trigger /prd-review
- On approve, save to `docs/prds/<name>.md`
- Acceptance: `rafa prd` creates a PRD through conversation

**Task 11: Implement Design creation flow**
- Similar to PRD but with /technical-design and /technical-design-review skills
- Support `--from <prd>` to reference existing PRD
- Save to `docs/designs/<name>.md`
- Acceptance: `rafa design` creates a design doc

**Task 12: Add session persistence with JSONL**
- Create `src/storage/sessions.ts` for session CRUD
- Store Claude session ID for `--resume` support
- Implement resume flow: load session, continue conversation
- Handle resume errors by offering fresh start
- Acceptance: Can pause and resume conversations across runs

### Phase 4: Polish

**Task 13: Build home screen with menu**
- Study `pi-mono/packages/tui/src/components/select-list.ts`
- Create HomeView with menu: [p] PRD, [d] Design, [c] Create Plan, [r] Run Plan
- Match layout from `docs/prds/rafa-workflow.md` TUI Home Screen section
- Acceptance: Home screen matches PRD mockup, navigation works

**Task 14: Build file picker for design docs**
- List files from `docs/designs/` directory
- Allow selection with arrow keys and enter
- Used when creating plans without explicit file argument
- Acceptance: `rafa plan create` shows picker when no file specified

**Task 15: Build plan list view**
- List plans from `.rafa/plans/` with status indicators
- Show: name, task count, status (✓ complete, ▶ in progress, ○ pending)
- Allow selection to run or view details
- Acceptance: `rafa plan list` shows all plans with status

**Task 16: Implement plan creation from design docs**
- Reference `internal/ai/claude.go` for task extraction prompt
- Reference `internal/cli/plan/create.go` for the full flow
- Invoke Claude to extract tasks from markdown design doc
- Generate plan ID, validate task structure, save to `.rafa/plans/`
- Support `--name` flag for custom plan name, collision detection
- Acceptance: `rafa plan create docs/designs/foo.md` creates a valid plan

**Task 17: Implement settings and init/deinit**
- `rafa init`: create `.rafa/`, fetch skills from GitHub, create .gitignore
- `rafa deinit`: remove `.rafa/` with confirmation
- Settings file at `.rafa/settings.json`
- Acceptance: Fresh repo can be initialized and de-initialized

### Phase 5: Distribution & Documentation

**Task 18: Set up GitHub releases with Bun binaries**
- Create GitHub Actions workflow for releases
- Build with `bun build --compile` for darwin-arm64, darwin-x64, linux-arm64, linux-x64
- Generate checksums.txt
- Upload artifacts to GitHub release
- Acceptance: Tagged release creates downloadable binaries

**Task 19: Create curl install script**
- Create `install.sh` that detects OS/arch
- Download correct binary from GitHub releases
- Verify checksum, install to ~/.local/bin or /usr/local/bin
- Acceptance: `curl -fsSL .../install.sh | sh` installs rafa

**Task 20: Write README.md**
- Installation instructions (curl, npm)
- Quick start guide
- Command reference
- Link to PRDs for detailed behavior
- Acceptance: New user can install and run first plan from README

**Task 21: Write CONTRIBUTING.md**
- Development setup (clone, npm install, npm run dev)
- Architecture overview with package structure
- How to add new views/components
- Testing approach
- Acceptance: Contributor can set up dev environment from doc

## Trade-offs

### Why not use pi-agent-core directly?

**Considered**: Using pi-agent-core with Anthropic API for task execution.

**Rejected because**:
- Claude Code has all tools, MCP servers, permissions already configured
- Would need to re-implement file editing, bash, search tools
- User's Claude Code customizations (CLAUDE.md, skills) wouldn't apply
- `claude -p` is battle-tested; custom agent loop adds risk

### Why TypeScript over keeping Go?

**Benefits**:
- pi-tui is TypeScript-native, no FFI needed
- Easier to iterate on TUI components
- Node.js ecosystem for parsing, testing
- Bun compile still produces single binary

**Costs**:
- Requires Node.js or Bun runtime (if not compiled)
- Larger binary size than Go
- Different deployment story

### Why keep JSONL sessions?

**Benefits**:
- Append-only writes (fast, crash-safe)
- Matches pi-mono pattern (potential future interop)
- Easy to inspect/debug
- Supports future tree navigation

**Costs**:
- Slightly more complex parsing than single JSON
- Need to rebuild state on load

## Resolved Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Binary distribution | Curl-installed binary (primary) | Single command install, no runtime dependencies |
| Skills installation | Keep fetching from GitHub | Users always get latest versions |
| Session format | JSONL (like pi-mono) | Matches framework patterns, append-only writes |
| Progress file extension | `.jsonl` | Explicit about format, auto-migrated from `.log` |
| Session errors | Treat all as generic failure | Don't try to detect expiration specifically; offer "start fresh" on any error |
