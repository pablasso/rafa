# PRD: Rafa - Task Loop Runner for AI Agents

## Problem

AI coding agents are powerful but unreliable for multi-step tasks. They lose context, declare victory prematurely, and require constant human supervision. Geoffrey Huntley's "Ralph Wiggum" technique solves this with a simple bash loop that retries until success—but it's manual, hard to monitor, and lacks structure.

Developers need a way to:
- Define a sequence of tasks with clear acceptance criteria
- Run agents in a loop until each task genuinely succeeds
- Monitor progress without babysitting
- Trust the system to retry on failure and stop when done

## Users

**Primary**: Developers using Claude Code who want to automate multi-task implementations.

**Needs**:
- Convert existing technical designs or PRDs into executable task plans
- Run tasks sequentially with automatic retry on failure
- See progress at a glance without watching full agent output
- Resume or audit runs via logs and git history
- Keep history of all plans and runs

## User Journey

1. **Install Rafa**: User runs `curl -fsSL https://rafa.dev/install.sh | sh` (or similar) to install the binary
2. **Initialize repo**: User runs `rafa init` in their repo root to create `.rafa/` folder
3. **Create plan**: User runs `rafa plan create <design.md>` to generate a JSON plan from their technical design or PRD
4. **Start run**: User runs `rafa plan run <name>` to begin executing tasks
5. **Monitor**: TUI displays current task, progress, and elapsed time. User can walk away.
6. **Completion**: Rafa finishes all tasks (or stops at max attempts). User reviews git history and progress file.
7. **Iterate**: User can run another plan, or resume a failed plan after fixing issues

## Requirements

### Installation & Setup

- [ ] Install via curl script: `curl -fsSL https://rafa.dev/install.sh | sh` (or similar one-liner)
- [ ] `rafa init` - Initialize Rafa in a repository (creates `.rafa/` folder)
- [ ] `rafa deinit` - Remove Rafa from a repository (removes `.rafa/` folder)
  - Shows what will be deleted (plan count, total size)
  - Requires confirmation before proceeding
- [ ] Rafa stores all metadata in `.rafa/` folder at repo root

### Plan Management

- [ ] `rafa plan create <file>` - Convert a technical design or PRD (markdown) into a JSON plan file
  - Uses AI to extract discrete tasks from the document
  - Each task sized to use ~50-60% of agent context
  - Each task must have clear, verifiable acceptance criteria
  - Creates a self-contained plan folder: `.rafa/plans/<id>-<plan-name>/`
  - On success: displays summary (plan ID, name, task count) and instructs user to run `rafa plan run <name>`
- [ ] Each plan folder contains all related files:
  ```
  .rafa/plans/xK9pQ2-feature-auth/
    plan.json       # The plan definition
    progress.log    # Event log for this plan
    output.log      # Full agent output for debugging
  ```
- [ ] Store unlimited plan folders (history preserved, not meant for parallel execution)

### Plan File Structure

JSON format inspired by Anthropic's long-running agents article. Uses short IDs (e.g., `xK9pQ2`) instead of UUIDs for readability:

```json
{
  "id": "xK9pQ2",
  "name": "feature-auth",
  "description": "Brief description of what this plan accomplishes",
  "sourceFile": "path/to/original/design.md",
  "createdAt": "ISO-8601 timestamp",
  "status": "not_started | in_progress | completed | failed",
  "tasks": [
    {
      "id": "t01",
      "title": "Short task title",
      "description": "Detailed description of what needs to be done",
      "acceptanceCriteria": [
        "Criterion 1 - must be verifiable",
        "Criterion 2 - ideally runnable (e.g., 'npm test passes')",
        "Criterion 3 - specific and measurable"
      ],
      "status": "pending | in_progress | completed | failed",
      "attempts": 0
    }
  ]
}
```

### Execution

- [ ] `rafa plan run <name>` - Start or resume executing a plan
  - Automatically resumes from the first pending task (skips completed tasks)
  - No manual task selection; always continues in sequence
- [ ] One agent, one task, one loop at a time (never parallel)
- [ ] Each task is executed by a fresh agent (new Claude Code session)
- [ ] Agent is instructed to:
  - Work on the single assigned task
  - Verify all acceptance criteria are met
  - Commit changes with a descriptive message upon success
  - Exit when done (success or failure)
- [ ] On task completion: mark task complete, move to next task
- [ ] On task failure: spin up a new fresh agent to retry (complete context handoff - new agent reads current state from files/git)
- [ ] Maximum 10 attempts per task (10 fresh agents) before stopping
  - After 10 failed attempts, task is marked as "failed" and plan execution stops
  - This is "determined failure" - the task could not be completed despite multiple fresh attempts
- [ ] If task requires human input: log `human_input_required` event, TUI displays task ID and title, instructs user to complete manually and mark done in `plan.json`
- [ ] Tasks run sequentially in order defined in the plan
- [ ] Trust agent self-verification of acceptance criteria (no external supervisor)

### Progress Tracking

- [ ] Progress file per plan (`progress.log` in the plan folder)
- [ ] Detailed agent output per plan (`output.log` in the plan folder)
- [ ] Events tracked in progress file:

| Event | Data |
|-------|------|
| `plan_started` | timestamp, plan_id |
| `task_started` | timestamp, task_id, attempt_number |
| `task_completed` | timestamp, task_id |
| `task_failed` | timestamp, task_id, attempt_number |
| `plan_completed` | timestamp, total_time, summary (X/Y tasks succeeded) |
| `plan_cancelled` | timestamp, last_task_id |
| `human_input_required` | timestamp, task_id, reason |

### TUI Display

- [ ] Current task number / total tasks (e.g., "Task 3/12")
- [ ] Current task ID and title
- [ ] Current attempt number for the task
- [ ] Total elapsed time
- [ ] Overall status indicator

### User Controls

- [ ] Cancel mid-run (Ctrl+C or quit command)
- [ ] On cancel: log `plan_cancelled` event, leave code and plan state as-is (no cleanup)
- [ ] User cannot manually mark tasks done via TUI (can edit JSON directly if needed)

### AGENTS.md Integration

- [ ] At end of a completed run, suggest additions to AGENTS.md based on observed patterns
- [ ] Never auto-modify AGENTS.md (suggestions only, user decides)

### CLI Help

```
$ rafa -h
Rafa - Task loop runner for AI coding agents

Usage:
  rafa <command> [arguments]

Commands:
  init      Initialize Rafa in the current repository
  deinit    Remove Rafa from the current repository
  plan      Manage and execute plans

Run "rafa <command> -h" for more information about a command.
```

```
$ rafa plan -h
Manage and execute plans

Usage:
  rafa plan <command> [arguments]

Commands:
  create <file>   Create a plan from a technical design or PRD
  run <name>      Run a plan (resumes from first pending task)
```

### Constraints

- [ ] Claude Code only (no other AI coding tools)
- [ ] Invokes Claude Code via CLI (`claude -p "..." --dangerously-skip-permissions`)

## User Experience

| State | What the user sees |
|-------|-------------------|
| Claude Code missing | "Error: Claude Code CLI not found. Install it first: https://claude.ai/code" |
| Claude Code not authenticated | "Error: Claude Code not authenticated. Run `claude auth` first." |
| Not initialized | "Run `rafa init` to initialize this repository." |
| Deinit confirmation | "This will delete .rafa/ (3 plans, 12MB). Continue? [y/N]" |
| No plans | "No plans found. Run `rafa plan create <design.md>` to create one." |
| Create success | "Plan created: xK9pQ2-feature-auth (8 tasks). Run `rafa plan run feature-auth` to start." |
| Plan loaded | Plan summary: name, description, task count, status |
| Resuming | "Resuming from task 3/8..." (skips completed tasks) |
| Running | TUI with progress: "Task 3/8: Implement user auth [Attempt 1] - 00:12:34" |
| Task retry | "Task 3/8: Implement user auth [Attempt 2] - Spinning up fresh agent..." |
| Human input needed | "Task t03 'Implement user auth' requires human input. Complete manually and set status to 'completed' in plan.json." |
| Max attempts | "Task 3/8 failed after 10 attempts. Human intervention required." |
| Completed | "Plan complete: 8/8 tasks succeeded in 01:23:45. See suggested AGENTS.md additions." |
| Cancelled | "Run cancelled. Progress saved. Resume with `rafa plan run <name>`." |

## Scope

**In scope:**
- Binary installation via curl script
- Repository initialization and de-initialization (`rafa init`, `rafa deinit`)
- Converting markdown designs to JSON plans via AI
- Sequential task execution with retry logic
- Automatic resume from first pending task
- Progress tracking and logging (per-plan, self-contained)
- Simple TUI for monitoring
- AGENTS.md suggestions post-run

**Out of scope:**
- Parallel task execution
- External task verification/supervision
- Support for AI tools other than Claude Code
- Configurable model or token limits
- Auto-modification of any files (except plan status)
- Creating or validating technical designs (user responsibility)
- Headless/CI mode (TUI only for v1)

## Success Metrics

- User can go from technical design to running plan in under 5 minutes
- Plans complete without human intervention (when tasks are well-defined)
- Failed runs provide enough context (logs, progress file, git history) to debug issues
- Users trust Rafa enough to start a run and walk away

## Dependencies

- Claude Code CLI installed and authenticated
- Git repository (for commit tracking)
- User has created a technical design or PRD document

## Open Questions

- None at this time. Remaining questions are technical design decisions.

## Future Enhancements

- **`rafa plan list`**: List plans (default: unfinished only, `--all` flag for complete history)
- **`rafa plan logs <name>`**: Tail the agent output log for a plan
- **`rafa plan status <name>`**: Debug view of plan execution progress
- **Headless mode**: Run without TUI for CI/CD integration (stream logs to stdout, exit codes for success/failure)
- **Specification validator**: Tool to analyze if acceptance criteria are strong enough before running
- **`rafa doctor`**: Check if recommended harnesses are set up (tests, linters, typechecks, browser MCPs)
- **Auto-upgrade warnings**: Notify user when new Rafa version is available
- **External verification**: Optional supervisor to independently verify acceptance criteria
- **Task dependency graph**: Allow tasks to declare dependencies (still sequential, but smarter ordering)
- **Max attempts per task**: Configurable (currently fixed at 10)
- **Context limit detection**: Detect when agent hits context limit and auto-restart
