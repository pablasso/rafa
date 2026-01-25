# Technical Design: Executor-Owned Commits

## Overview

Task completion metadata is left uncommitted when plans finish or fail. The current design has agents commit their work, then the executor updates metadata, relying on the **next agent** to commit those metadata changes. This breaks when:

1. **Last task**: No subsequent agent exists to commit final metadata
2. **Failed subsequent task**: If task N+1 fails, task N's completion metadata is never committed

**Root Cause**: Split responsibility - agents commit implementation, executor updates metadata, next agent commits metadata.

**Solution**: Single responsibility - executor owns ALL commits. Agents implement and validate, but do not commit.

### Current Flow (Fragile)

```
Agent 1 commits implementation → Executor updates metadata (dirty) →
Agent 2 commits (includes prev metadata) → Executor updates metadata (dirty) →
... Last task → No one commits final metadata
```

### Proposed Flow (Robust)

```
Agent 1 implements (no commit) → Executor updates metadata + commits ALL →
Agent 2 implements (no commit) → Executor updates metadata + commits ALL →
... Last task → Executor commits (same as every other task)
```

## Goals

- Executor owns all commits (implementation + metadata together)
- Same number of commits as today (1 per successful task + 1 for plan completion)
- Metadata always accurately reflects execution state
- Workspace clean after each successful task

## Non-Goals

- Changing the number of commits
- Committing on task failure (user may want to inspect state)
- Forcing agents to never use git (just nudge them not to commit)

## Architecture

The executor becomes the single owner of all git commits during plan execution.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Executor                                     │
│  internal/executor/executor.go                                      │
│                                                                     │
│  executeTask() flow:                                                │
│  1. Mark task in_progress, log task_started                         │
│  2. Run agent (implements task, does NOT commit)                    │
│  3. Agent returns success + suggested commit message                │
│  4. Mark task completed, log task_completed                         │
│  5. Commit ALL changes (implementation + metadata)                  │
│                                                                     │
│  Run() flow after all tasks:                                        │
│  1. Mark plan completed, log plan_completed                         │
│  2. Commit metadata (if any uncommitted)                            │
└─────────────────────────────────────────────────────────────────────┘
```

### Commit Responsibility (New)

| What | Who |
|------|-----|
| Implementation code | Agent creates, **Executor commits** |
| All metadata (start + completion) | Executor updates, **Executor commits** |
| Commit message | Agent suggests, Executor uses (or defaults to task title) |

### Commit Frequency

Same as today:
- N commits for N successful tasks (implementation + metadata together)
- Plan completion metadata is included in last task's commit (or separate if needed)

## Technical Details

### 1. New Function in `internal/git/git.go`

```go
// CommitAll stages all changes and commits with the given message.
// Returns nil if there are no changes to commit.
func CommitAll(dir string, message string) error {
    // Stage all changes
    addCmd := exec.Command("git", "add", "-A")
    if dir != "" {
        addCmd.Dir = dir
    }
    if err := addCmd.Run(); err != nil {
        return fmt.Errorf("failed to stage files: %w", err)
    }

    // Check if there are staged changes
    diffCmd := exec.Command("git", "diff", "--cached", "--quiet")
    if dir != "" {
        diffCmd.Dir = dir
    }
    if err := diffCmd.Run(); err == nil {
        // No staged changes (exit code 0 means no diff)
        return nil
    }

    // Commit
    commitCmd := exec.Command("git", "commit", "-m", message)
    if dir != "" {
        commitCmd.Dir = dir
    }
    if err := commitCmd.Run(); err != nil {
        return fmt.Errorf("failed to commit: %w", err)
    }

    return nil
}
```

### 2. Modified Agent Prompt in `runner.go`

Update `buildPrompt()` to tell agents NOT to commit:

```go
func (r *ClaudeRunner) buildPrompt(task *plan.Task, planContext string, attempt, maxAttempts int) string {
    // ... existing context and task info ...

    sb.WriteString("## Instructions\n")
    sb.WriteString("1. Implement the task as described\n")
    sb.WriteString("2. Verify ALL acceptance criteria are met\n")
    sb.WriteString("3. If you have a code review skill available (e.g., `/code-review`), use it\n")
    sb.WriteString("4. **DO NOT commit your changes** - the orchestrator will handle the commit\n")
    sb.WriteString("5. When done, output your suggested commit message in this format:\n")
    sb.WriteString("   SUGGESTED_COMMIT_MESSAGE: <your descriptive commit message>\n\n")

    sb.WriteString("IMPORTANT: Leave changes uncommitted. The orchestrator will commit after validating.\n")

    return sb.String()
}
```

For retry attempts, update the retry note:

```go
if attempt > 1 {
    sb.WriteString("**Note**: Previous attempts failed. ")
    sb.WriteString("If there are uncommitted changes from previous attempts, ")
    sb.WriteString("review them and continue from where the previous attempt left off.\n\n")
}
```

### 3. Modified `executeTask()` in `executor.go`

Remove the "workspace must be clean" check after agent returns. Instead, commit all changes:

```go
func (e *Executor) executeTask(ctx context.Context, task *plan.Task, idx int, planContext string, output *OutputCapture) error {
    // ... existing setup ...

    for task.Attempts < MaxAttempts {
        // ... existing attempt setup ...

        // Run the task
        err := e.runner.Run(ctx, task, planContext, task.Attempts, MaxAttempts, output)
        if err != nil {
            // Task failed - don't commit, allow retry with dirty workspace
            e.logger.TaskFailed(task.ID, task.Attempts)
            // ... existing retry logic ...
            continue
        }

        // Task succeeded - update metadata and commit everything
        task.Status = plan.TaskStatusCompleted
        if saveErr := plan.SavePlan(e.planDir, e.plan); saveErr != nil {
            return fmt.Errorf("failed to save plan: %w", saveErr)
        }
        e.notifySave()

        if logErr := e.logger.TaskCompleted(task.ID); logErr != nil {
            return fmt.Errorf("failed to log task completed: %w", logErr)
        }

        // Commit all changes (implementation + metadata)
        if !e.allowDirty {
            commitMsg := e.getCommitMessage(task, output)
            if err := git.CommitAll(e.repoRoot, commitMsg); err != nil {
                return fmt.Errorf("failed to commit: %w", err)
            }

            // Verify workspace is clean after commit (catches git hooks, etc.)
            if status, err := git.GetStatus(e.repoRoot); err == nil && !status.Clean {
                return fmt.Errorf("workspace not clean after commit: %v", status.Files)
            }
        }

        if output != nil {
            output.WriteTaskFooter(task.ID, true)
        }
        return nil
    }

    return fmt.Errorf("max attempts reached")
}
```

### 4. New Helper Method for Commit Messages

```go
// getCommitMessage extracts agent's suggested message or uses default.
func (e *Executor) getCommitMessage(task *plan.Task, output *OutputCapture) string {
    // Try to extract from agent output if available
    if output != nil {
        if suggested := output.ExtractCommitMessage(); suggested != "" {
            return suggested
        }
    }

    // Default to task-based message with [rafa] prefix for easy filtering
    return fmt.Sprintf("[rafa] Complete task %s: %s", task.ID, task.Title)
}
```

### 5. Add to `OutputCapture` in `output.go`

```go
// ExtractCommitMessage looks for SUGGESTED_COMMIT_MESSAGE in the captured output.
// Searches only the last 100 lines for efficiency with large log files.
func (oc *OutputCapture) ExtractCommitMessage() string {
    content, err := os.ReadFile(oc.logFile.Name())
    if err != nil {
        return ""
    }

    lines := strings.Split(string(content), "\n")

    // Only search last 100 lines for efficiency
    start := len(lines) - 100
    if start < 0 {
        start = 0
    }

    for i := len(lines) - 1; i >= start; i-- {
        if strings.HasPrefix(lines[i], "SUGGESTED_COMMIT_MESSAGE:") {
            return strings.TrimSpace(strings.TrimPrefix(lines[i], "SUGGESTED_COMMIT_MESSAGE:"))
        }
    }
    return ""
}
```

### 6. Modified `Run()` for Plan Completion

The plan completion metadata is committed as part of the last task's commit. But if the last task's agent already committed (edge case), we need a final commit:

```go
// All tasks completed
e.plan.Status = plan.PlanStatusCompleted
if err := plan.SavePlan(e.planDir, e.plan); err != nil {
    return fmt.Errorf("failed to save plan: %w", err)
}
e.notifySave()

duration := time.Since(e.startTime)
e.logger.PlanCompleted(len(e.plan.Tasks), e.countCompleted(), duration)

// Commit any remaining metadata (handles edge case where agent committed)
if !e.allowDirty {
    msg := fmt.Sprintf("[rafa] Complete plan: %s (%d tasks)", e.plan.Name, len(e.plan.Tasks))
    if err := git.CommitAll(e.repoRoot, msg); err != nil {
        // Only warn - might be "nothing to commit" which is fine
        fmt.Printf("Note: %v\n", err)
    }
}

fmt.Printf("\nPlan completed! (%s)\n", e.formatDuration(duration))
return nil
```

### Files to Modify

| File | Changes |
|------|---------|
| `internal/git/git.go` | Add `CommitAll` function |
| `internal/git/git_test.go` | Add tests for `CommitAll` |
| `internal/executor/runner.go` | Update prompt: don't commit, suggest message |
| `internal/executor/executor.go` | Remove clean check, add commit after task success |
| `internal/executor/output.go` | Add `ExtractCommitMessage` method |
| `internal/executor/executor_test.go` | Update tests for new commit flow |

## Edge Cases

| Case | How it's handled |
|------|------------------|
| No changes to commit | `CommitAll` returns nil (git diff --cached --quiet succeeds) |
| `--allow-dirty` flag used | Skip ALL commits (user explicitly opted out of clean workspace) |
| Agent accidentally commits | Accept it - executor commits remaining metadata only |
| Agent doesn't suggest message | Use default: `[rafa] Complete task <id>: <title>` |
| Task fails | Don't commit, leave workspace dirty for retry/inspection |
| Plan fails (max attempts) | Don't commit failure state - user can inspect locally |
| Plan cancelled (Ctrl+C) | Don't commit - user can inspect/resume later |
| Agent leaves workspace dirty on failure | Keep dirty, next attempt sees it and can continue |
| Workspace dirty after commit (git hooks) | Verify and error if not clean |
| `git add -A` fails | Return error, task stays incomplete (fail fast) |
| `git commit` fails (pre-commit hook) | Return error, task stays incomplete (fail fast) |
| Commit message has special chars | Git handles escaping; no special handling needed |

## Security

- No new external commands (uses existing `git` binary)
- Uses `git add -A` to stage all changes - same as current agent behavior
  - Relies on `.gitignore` to exclude sensitive files (`.env`, credentials, etc.)
  - If `.gitignore` is properly configured, this is safe
- Respects `--allow-dirty` flag for users who don't want automatic commits
- Agent still validates work before executor commits (acceptance criteria check)

## Performance

- Same number of commits as today (1 per successful task)
- Commit happens after task success (~50ms overhead)
- Negligible compared to task execution time (minutes per task)
- No increase in git operations vs. current approach

## Testing

### Unit Tests

| Test | Description |
|------|-------------|
| `TestCommitAll_CommitsChanges` | Successfully stages and commits all changes |
| `TestCommitAll_NoChanges` | Returns nil when nothing to commit |
| `TestCommitAll_InvalidDir` | Returns error for invalid directory |
| `TestExtractCommitMessage_Found` | Extracts message from output |
| `TestExtractCommitMessage_NotFound` | Returns empty when no message |
| `TestBuildPrompt_NoCommitInstruction` | Prompt includes "DO NOT commit" |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestRun_ExecutorCommitsAfterTask` | Executor commits implementation + metadata |
| `TestRun_UsesAgentSuggestedMessage` | Commit uses agent's suggested message |
| `TestRun_DefaultMessageWhenNoSuggestion` | Falls back to task title |
| `TestRun_TaskFailure_NothingCommitted` | Failed task leaves workspace dirty |
| `TestRun_AgentAccidentallyCommits_Handled` | Executor handles pre-committed work |
| `TestRun_AllowDirty_NoCommits` | With --allow-dirty, no commits |
| `TestRun_WorkspaceCleanAfterSuccess` | After successful plan, workspace is clean |

## Trade-offs

### Executor Commits vs. Agent Commits

**Chosen**: Executor owns all commits

**Why**:
- Single point of responsibility
- Implementation + metadata committed atomically
- Same commit count as before (no increase)
- No reliance on "next agent" pattern
- Handles last-task and failed-subsequent-task cases automatically

**Considered**: Keep agent commits, add executor commits for metadata
- Rejected: Doubles commit count (N agent + N executor)

### Agent Commit Message Suggestion

**Chosen**: Agent suggests, executor uses if available, else default

**Why**:
- Agent has best context for describing implementation
- Graceful fallback if agent doesn't suggest
- Non-blocking: agent forgetting to suggest doesn't break anything

**Format**: `SUGGESTED_COMMIT_MESSAGE: <message>`
- Easy to parse
- Explicit (not ambiguous with other output)
- Agent can include multi-line by using conventional format

### Handling Accidental Agent Commits

**Chosen**: Accept it, commit remaining changes (metadata only)

**Why**:
- Graceful degradation
- Nudge agents not to commit, but don't enforce
- If agent commits, workspace may still have metadata changes
- `git commit` with no changes is a no-op (returns "nothing to commit")

### On Task Failure: Commit or Not?

**Chosen**: Don't commit on failure

**Why**:
- User may want to inspect workspace state
- Next retry attempt sees previous changes and can continue
- Consistent with current behavior
- Clean separation: success = commit, failure = leave dirty

### Commit Message Format

**Chosen**: `[rafa] Complete task <id>: <title>`

**Why**:
- `[rafa]` prefix makes commits easy to filter in git log (`git log --grep='\[rafa\]'`)
- Task ID enables correlation with plan metadata
- Title provides human-readable context

### Workspace Verification After Commit

**Chosen**: Verify workspace is clean after commit, error if not

**Why**:
- Catches edge cases like git hooks modifying files
- Ensures consistent state for next task
- Fails fast rather than accumulating dirty state

### Git Operation Failures

**Chosen**: Fail fast - return error, task stays incomplete

**Why**:
- Clear failure mode (no partial/inconsistent state)
- User can fix git issues and resume plan
- Preserves workspace state for debugging
- Alternatives (retry, ignore) could mask real problems

**Scenarios**:
- `git add -A` fails (permissions, disk full) → task incomplete, can retry
- `git commit` fails (pre-commit hook rejects) → task incomplete, agent sees rejection on retry

## Open Questions

None - the solution is straightforward.
