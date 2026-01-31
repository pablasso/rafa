# PRD: Rafa Workflow Orchestration

## Problem

Rafa currently handles the execution phase of AI-assisted development—running plans with retry logic and progress tracking. But the workflow leading up to execution is manual: developers create PRDs and design docs outside Rafa, then feed them into `rafa plan create`.

This creates friction:
- Context is lost between phases (PRD → Design → Plan)
- No structured review process before moving forward
- Users must manually invoke Claude Code for each document type
- No way to resume interrupted conversations

Developers need an integrated workflow where Rafa guides them from problem definition through implementation, with built-in review gates and resumable sessions.

## Users

**Primary**: Developers using Claude Code who want structured AI assistance for the full development lifecycle.

**Needs**:
- Guided PRD creation with AI-driven questions and drafting
- Technical design creation (from PRD or fresh context)
- Conversational plan creation with optional additional instructions
- Built-in review rounds at each phase
- Ability to pause and resume any phase
- Clear phase transitions without forced progression
- Visibility into what Claude is doing during conversations

## User Journey

1. **Initialize**: User runs `rafa init` to set up `.rafa/` and install skills
2. **Create PRD** (optional): User starts `rafa prd`, has a conversation with Claude using the `/prd` skill, reviews auto-generated review feedback, approves when satisfied
3. **Create Design**: User starts `rafa design`, optionally references a PRD, converses with Claude using `/technical-design`, reviews auto-generated feedback, approves
4. **Create Plan**: User starts `rafa plan create`, selects a design doc, optionally provides additional instructions, converses with Claude to refine the plan
5. **Execute Plan**: User runs `rafa plan run`, monitors progress with cost/token tracking, reviews completed implementation
6. **Iterate**: User can revisit any phase, resume sessions, or start fresh

## Requirements

### Skills Integration

- [ ] `rafa init` downloads skills from `github.com/pablasso/skills` (always latest) and installs to `.claude/skills/` in the project
- [ ] If skills repo is unavailable, `rafa init` fails with an error (do not partially initialize)
- [ ] Skills installed: `prd`, `prd-review`, `technical-design`, `technical-design-review`, `code-review`
- [ ] Skills are invoked by instructing Claude to use them (e.g., "Use the /prd skill")
- [ ] Skills are NOT bundled in the Rafa repository—fetched fresh during init

### Session Management

- [ ] Track sessions in `.rafa/sessions/` (gitignored)
- [ ] Each session stored as JSON: `{sessionId, phase, documentPath, status, createdAt, updatedAt}`
- [ ] Always use `--resume <session-id>` to continue conversations
- [ ] Session naming: `<phase>-<document-name>.json` (e.g., `prd-user-auth.json`)
- [ ] If Claude session has expired, inform user and offer to start fresh (detection mechanism TBD in design)
- [ ] Multiple concurrent drafting sessions allowed (e.g., two design docs in progress)
- [ ] Plan execution remains single-instance (existing locking mechanism)

### Claude CLI Integration

- [ ] Use `claude -p <prompt> --output-format stream-json --include-partial-messages --dangerously-skip-permissions`
- [ ] Use `--resume <session-id>` for continuing conversations
- [ ] Parse stream-json events to populate activity timeline:
  - `tool_use` events → show tool name and target (e.g., "Reading auth.go")
  - `tool_use_result` events → show completion (e.g., "done 2s")
  - Subagent spawns/completions (Task tool usage)
- [ ] Stream text responses in real-time to main pane
- [ ] Extract cost and token usage from stream-json output (for plan execution display)

### TUI Layout - Conversational Mode

For PRD, Design, and Plan creation phases:

```
┌─────────────────────────────────────────────────────────────────┐
│ Rafa - Creating PRD                                             │
├──────────────┬──────────────────────────────────────────────────┤
│              │                                                  │
│  Activity    │  [Claude's streamed response appears here]       │
│  ─────       │                                                  │
│  ├─ Started  │  I'll help you create a PRD. Let me ask some     │
│  │  session  │  clarifying questions first...                   │
│  ├─ Reading  │                                                  │
│  │  prds/..  │  What problem are you trying to solve?           │
│  ├─ Spawned  │                                                  │
│  │  Explore  │                                                  │
│  │  subagent │                                                  │
│  ├─ Subagent │                                                  │
│  │  done 2s  │                                                  │
│  └─ Writing  │                                                  │
│     draft... │                                                  │
│              │                                                  │
│              │                                                  │
│              │                                                  │
│              │                                                  │
├──────────────┴──────────────────────────────────────────────────┤
│ > [Multi-line input field]                                      │
│                                                    Ctrl+Enter   │
├─────────────────────────────────────────────────────────────────┤
│ [a] Approve   [c] Cancel          (type to revise)              │
└─────────────────────────────────────────────────────────────────┘
```

**Left pane - Activity timeline**:
- Scrolling log of Claude's actions, parsed from stream-json events
- Shows: tool uses (Read, Write, Edit, Glob, Grep, etc.), subagent spawns/completions, phase transitions
- Scrolls vertically when content exceeds pane height (keeps newest visible)
- New events append to bottom with tree-style formatting

**Main pane**: Streamed Claude response

**Bottom**: Multi-line input field (Ctrl+Enter / Cmd+Enter to submit)
- Max height: 5 lines, scrolls internally if content exceeds
- Disabled while Claude is working (user must wait for response to complete)

**Action bar**: Approve / Cancel options (typing in input field acts as revision)

### TUI Layout - Plan Execution Mode

Extends current running view with activity timeline and cost/token tracking:

```
┌─────────────────────────────────────────────────────────────────┐
│ Rafa - Running Plan: user-auth                                  │
├──────────────┬──────────────────────────────────────────────────┤
│              │                                                  │
│  Task 3/8    │  [Claude's execution output streams here]        │
│  Attempt 1/5 │                                                  │
│  00:12:34    │  Let me implement the login endpoint...          │
│              │                                                  │
│  Activity    │                                                  │
│  ─────       │                                                  │
│  ├─ Reading  │                                                  │
│  │  auth.go  │                                                  │
│  ├─ Spawned  │                                                  │
│  │  Explore  │                                                  │
│  ├─ Subagent │                                                  │
│  │  done 2s  │                                                  │
│  └─ Editing  │                                                  │
│     login.go │                                                  │
│              │                                                  │
│  Usage       │                                                  │
│  ─────       │                                                  │
│  Task: 12.4k │                                                  │
│  Plan: 89.2k │                                                  │
│  Cost: $0.89 │                                                  │
│              │                                                  │
│  Tasks       │                                                  │
│  ─────       │                                                  │
│  ✓ t01 t02   │                                                  │
│  ▶ t03       │                                                  │
│  ○ t04 t05   │                                                  │
│              │                                                  │
├──────────────┴──────────────────────────────────────────────────┤
│ Ctrl+C to cancel                                                │
└─────────────────────────────────────────────────────────────────┘
```

**Left pane sections**:
- **Header**: Current task (N/M), attempt number, elapsed time
- **Activity timeline**: Same as conversational mode - scrolling log of tool uses, subagent activity (resets per task). Scrolls when content exceeds available space.
- **Usage**: Token count (task + plan cumulative) and estimated cost
- **Tasks**: Compact task list with status indicators (✓ complete, ▶ current, ○ pending)

### TUI Home Screen

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│   ╭───────────────────────────────────────────────────────╮     │
│   │                                                       │     │
│   │   Rafa                                                │     │
│   │   AI Workflow Orchestrator                            │     │
│   │                                                       │     │
│   ╰───────────────────────────────────────────────────────╯     │
│                                                                 │
│   Define                                                        │
│   ─────                                                         │
│   [p] Create PRD           Define the problem and requirements  │
│   [d] Create Design Doc    Plan the technical approach          │
│                                                                 │
│   Execute                                                       │
│   ───────                                                       │
│   [c] Create Plan          Break design into executable tasks   │
│   [r] Run Plan             Execute tasks with AI agents         │
│                                                                 │
│   [q] Quit                                                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### PRD Creation Flow

- [ ] `rafa prd` or TUI option starts PRD creation
- [ ] `rafa prd --name <name>` starts with a specific name
- [ ] `rafa prd resume` or `rafa prd resume <name>` resumes existing session
- [ ] Rafa instructs Claude: "Use the /prd skill to help the user create a PRD"
- [ ] User converses with Claude through Rafa TUI
- [ ] After Claude finishes drafting, auto-trigger: "Now use /prd-review to review what you created"
- [ ] Agent decides which review findings are worth acting on vs acceptable trade-offs
- [ ] User sees review feedback and any changes made, can approve, revise, or cancel
- [ ] On approve: Claude suggests filename based on content, save to `docs/prds/<name>.md`
- [ ] If filename exists, warn user and ask for alternative
- [ ] Show completion message with next steps

### Design Doc Creation Flow

- [ ] `rafa design` or TUI option starts design creation
- [ ] `rafa design --from <prd-path>` starts from existing PRD
- [ ] `rafa design --name <name>` starts with specific name
- [ ] `rafa design resume` or `rafa design resume <name>` resumes session
- [ ] If no `--from` provided, ask: "Do you have an existing PRD?" → file picker or fresh start
- [ ] Rafa instructs Claude: "Use the /technical-design skill" (with PRD context if provided)
- [ ] User converses with Claude
- [ ] After drafting, auto-trigger: "Now use /technical-design-review to review"
- [ ] Agent decides which review findings are worth acting on
- [ ] User approves, revises, or cancels
- [ ] On approve: save to `docs/designs/<name>.md`
- [ ] Show completion message with next steps

### Plan Creation Flow

- [ ] `rafa plan create` or TUI option starts plan creation
- [ ] If no design docs exist in `docs/designs/`, show error and suggest creating one first
- [ ] Prompt user to select a design doc from `docs/designs/`
- [ ] Optional: user can provide initial instructions/context
- [ ] Rafa instructs Claude to extract tasks from design doc (current behavior)
- [ ] Make this conversational: user can refine, add constraints, adjust scope
- [ ] On approve: save plan to `.rafa/plans/<id>-<name>/`
- [ ] Show completion message

### User Controls During Conversation

- [ ] **Approve** (`a`): Mark phase complete, save document, show next steps
- [ ] **Cancel** (`c`): Exit phase, leave files as-is, return to home
- [ ] **Revise**: Type in input field and submit (Ctrl+Enter / Cmd+Enter) to send feedback to Claude
- [ ] Input field disabled while Claude is working (must wait for response to complete)
- [ ] **Back to menu**: After any phase completion, option to return home

### Phase Completion

After each phase is approved:

```
✓ PRD saved to docs/prds/user-auth.md

Next steps:
  • Run 'rafa design --from docs/prds/user-auth.md' to create a technical design
  • Or press [d] to start a design doc
  • Press [m] to return to menu

Session saved. You can resume anytime with 'rafa prd resume user-auth'
```

- [ ] Never auto-progress to next phase
- [ ] Always show how to proceed when ready
- [ ] Allow return to main menu

### File Naming

- [ ] Claude suggests filename based on document content
- [ ] Default pattern if no suggestion: `<phase>-<timestamp>.md`
- [ ] If suggested name exists, warn and ask for alternative
- [ ] Design docs can inherit PRD name when created from one (user can override)

### CLI Commands

```
rafa init                      # Initialize .rafa/, install skills
rafa deinit                    # Remove .rafa/ (with confirmation)

rafa prd [--name <name>]       # Start PRD creation
rafa prd resume [<name>]       # Resume PRD session

rafa design [--name <name>] [--from <prd>]   # Start design doc
rafa design resume [<name>]                   # Resume design session

rafa plan create               # Create plan from design doc
rafa plan run [<name>]         # Run a plan
rafa plan list                 # List plans
rafa plan resume [<name>]      # Resume plan creation session

rafa sessions                  # List active sessions
```

### State Storage

```
.rafa/
  sessions/           # Gitignored
    prd-user-auth.json
    design-user-auth.json
    plan-create-user-auth.json
  plans/
    xK9pQ2-user-auth/
      plan.json
      progress.log
      output.log

.claude/
  skills/             # Installed by rafa init from pablasso/skills
```

Session JSON structure:
```json
{
  "sessionId": "claude-session-id-here",
  "phase": "prd|design|plan-create",
  "name": "user-auth",
  "documentPath": "docs/prds/user-auth.md",
  "status": "in_progress|completed|cancelled",
  "createdAt": "ISO-8601",
  "updatedAt": "ISO-8601",
  "fromDocument": "docs/prds/user-auth.md"  // For design docs created from PRD
}
```

### Constraints

- [ ] Claude Code CLI only (no direct API calls)
- [ ] `--dangerously-skip-permissions` for all invocations
- [ ] Skills fetched from `github.com/pablasso/skills` during `rafa init`, installed to `.claude/skills/`
- [ ] Sessions directory (`.rafa/sessions/`) gitignored
- [ ] TUI is primary interface, CLI commands as alternative

## User Experience

| State | What the user sees |
|-------|-------------------|
| No skills installed | "Skills not found. Run 'rafa init' to install." |
| No designs for plan | "No design docs found in docs/designs/. Create a design first with 'rafa design'." |
| Starting PRD | "Starting PRD creation. Claude will guide you through the process." |
| Claude working | Activity timeline updates with tool uses and subagent activity |
| Claude streaming | Response appears in real-time in main pane |
| Waiting for input | Action bar highlights, input field focused |
| Auto-review | "Running automatic review..." then review feedback streams |
| Approve | Saves immediately, shows "✓ PRD saved to docs/prds/user-auth.md" + next steps |
| Cancel | "Cancelled. Files left as-is. Session saved for later." |
| Resume | "Resuming session: prd-user-auth. Last updated 2 hours ago." |
| Session expired | "Session expired. Start a new session with 'rafa prd'." |
| File exists | "docs/prds/user-auth.md already exists. Choose a different name?" |
| Plan running | Progress, tokens, cost in left pane; output streaming in main |

## Scope

**In scope:**
- PRD creation with /prd skill and auto-review
- Design doc creation with /technical-design skill and auto-review
- Conversational plan creation with optional user instructions
- Session persistence and resume via --resume
- TUI for all conversational phases
- Cost/token tracking during plan execution
- Skills fetched from `github.com/pablasso/skills` via `rafa init`
- CLI commands as alternative to TUI

**Out of scope:**
- Direct API calls (CLI only)
- Parallel phase execution
- Auto-progression between phases
- External integrations (Jira, Linear, etc.)
- Collaborative/multi-user sessions
- Version control for PRDs/designs (just files in git)

## Success Metrics

- User can go from idea to running implementation with Rafa guiding each phase
- Built-in reviews catch issues before moving to next phase
- Sessions can be paused and resumed without losing context
- Users trust the workflow enough to create production PRDs and designs through Rafa

## Dependencies

- Claude Code CLI installed and authenticated
- Git repository (for file storage and plan commits)
- Existing Rafa plan execution infrastructure

## Open Questions

- None at this time. Remaining questions are technical design decisions.
