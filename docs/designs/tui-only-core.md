# Technical Design: TUI-Only Core (Plan Create + Run)

## Overview

We are hard-removing everything outside the TUI flows for creating plans from design docs and running those plans. The goal is a lean TUI-only product that supports:
- **Create Plan** (from `docs/designs/*.md`)
- **Run Plan** (from `.rafa/plans/*/plan.json`)

This design assumes the scope described in `docs/prds/rafa-core.md`, with the workflow orchestration PRD deferred (rolled back). If you want a different PRD referenced, let me know.

## Goals

- Remove all CLI entrypoints and related packages/tests.
- Remove PRD/design conversation flows and session/skills scaffolding.
- Remove demo mode and legacy plan creation view.
- Keep only TUI plan creation + execution flows, with current plan/executor behavior intact.
- Update only **README** and **CONTRIBUTING** to reflect the new TUI-only scope.

## Non-Goals

- No changes to plan format, executor semantics, retry logic, or logging formats.
- No new features (no new flows, no new UI, no new config).
- No updates to other docs besides README and CONTRIBUTING.

## Architecture

**Before (simplified)**
- `cmd/rafa` -> TUI (default) **and** CLI (cobra)
- TUI views: home, file picker, plan create (conversational), plan list, run, plus legacy create, conversation, demo
- CLI flows for init/prd/design/plan/run/demo
- Skills/session/analysis packages for workflow orchestration

**After**
- `cmd/rafa` -> **TUI-only**
- TUI views: home, file picker, plan create (conversational), plan list, run
- AI usage: `internal/ai/conversation.go` (plan creation) and `internal/ai/CommandContext` (plan execution runner)
- Core engine remains: `internal/plan`, `internal/executor`, `internal/git`

## Technical Details

### Removed Surface Area

**Packages to delete**
- `internal/cli/*` (all cobra commands + tests)
- `internal/session/*`
- `internal/skills/*`
- `internal/demo/*`
- `internal/display/*`
- `internal/analysis/*`

**TUI views to delete**
- `internal/tui/views/create.go` (legacy plan create) + tests
- `internal/tui/views/conversation.go` + tests
- `internal/tui/views/filenaming.go` + tests

**AI functions to delete**
- `internal/ai/ExtractTasks` (`internal/ai/claude.go`) and related tests
  - Keep `internal/ai/CommandContext` and `internal/ai/conversation.go`

### Kept Flows

**Plan Create (Conversational)**
- `internal/tui/views/plancreate.go`
- `internal/ai/conversation.go`
- `internal/plan/*` + `internal/util/*`

**Plan Run**
- `internal/tui/views/run.go`
- `internal/executor/*`
- `internal/git/*`

### Internal Refactors Required

- Remove `ViewCreating`, `ViewConversation`, demo mode wiring from `internal/tui/app.go` and tests.
- Move or inline `ActivityEntry` type currently in `conversation.go` (used by `plancreate.go`).
- Remove executor display integration (`internal/display`) from `internal/executor/executor.go`.
- Remove demo mode support from `internal/tui/app.go` and `internal/tui/views/run.go`.
- Delete CLI references from `go.mod` if they become unused (e.g., cobra).

## Edge Cases

| Case | How it’s handled |
|------|-------------------|
| No design docs | Home → Create Plan returns error message (existing behavior) |
| No plans | Plan list shows empty state and offers Create Plan |
| Dirty git workspace | Executor error (unchanged behavior) |
| Claude CLI missing | Plan create/run should surface error (unchanged) |

## Performance

No expected changes. We are removing unused code paths only.

## Testing

- Update or delete tests tied to removed packages/views.
- Run `make fmt`, `make build`, `make test`.

## Rollout

- Standard release process; no feature flags.

## Rollback / Recovery

- Revert the change set if we need legacy flows back.

## Risks & Mitigations

- **Risk:** Removing shared types (e.g., `ActivityEntry`) breaks plan create.
  - **Mitigation:** Move the type into `plancreate.go` or a small shared helper file in `internal/tui/views`.
- **Risk:** Removing CLI and demo leaves unused imports or broken tests.
  - **Mitigation:** Systematically delete packages + fix references + update tests in one sweep.
- **Risk:** Executor display removal affects behavior.
  - **Mitigation:** Keep executor events/output capture (used by TUI), remove only display-specific code paths.

## Implementation Plan

1. Remove CLI packages and tests (`internal/cli/*`) and update `go.mod` if needed.
2. Remove workflow packages: `internal/session`, `internal/skills`, `internal/analysis`, `internal/demo`, `internal/display` (plus tests).
3. Delete unused TUI views (`create.go`, `conversation.go`, `filenaming.go`) and update `internal/tui/app.go` routing and tests.
4. Remove `ai.ExtractTasks` and associated tests; keep `ai.CommandContext` + `ai.Conversation`.
5. Clean executor display hooks; keep events/output capture for TUI.
6. Update README + CONTRIBUTING to reflect TUI-only scope and removed CLI.
7. Run `make fmt`, `make build`, `make test`.

## Trade-offs

- **Pros:** Smaller surface area, fewer dependencies, clearer product story.
- **Cons:** No CLI or demo mode; loss of legacy workflows in codebase.

## Open Questions

- None. Scope is intentionally narrow per requirements.

## Ready To Implement

Yes — implementation can proceed now.
