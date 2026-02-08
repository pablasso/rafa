# Technical Design: Unsaved Create-Plan Demo Mode

## Overview

Add a create-focused demo surface that replays realistic plan-creation activity in the TUI without writing plan files. This keeps demo sessions focused on the Create Plan UI and avoids mutating `.rafa/plans/`.

## Goals

- Add `--demo-mode=create` while keeping `--demo` and existing run demo behavior.
- Replay realistic create-plan stream events (tool activity + response text).
- Parse `PLAN_APPROVED_JSON` through the same parser used in production.
- End in a completed create view with explicit “demo only / not saved” messaging.

## Non-Goals

- Persisting plans during create demo.
- Auto-transitioning create demo into run demo.
- Changing production create flow behavior (real create still saves, then returns to Home when the user confirms).

## CLI and Options

- New flag: `--demo-mode=run|create` (default `run`)
- `--demo-scenario` remains run-only and is rejected with `--demo-mode=create`
- `internal/tui/options.go` adds `DemoOptions.Mode`

## TUI Behavior

### Run Demo (`--demo-mode=run`)

Unchanged behavior.

### Create Demo (`--demo-mode=create`)

- Starts directly in `ViewPlanCreate`
- Uses replayed fixture events via a demo `ConversationStarter`
- On valid `PLAN_APPROVED_JSON`, sets completed state with no file writes
- Shows action bar: `[DEMO]`, replay/home/quit controls

## Data and Fixtures

- Embedded fixture: `internal/demo/fixtures/create.default.v1.json`
- Loader: `LoadDefaultCreateDataset()`
- Fallback: `FallbackCreateDataset()`
- Replay starter: `CreateReplayStarter`
- Pacing: `NewCreateReplayConfig()` based on demo preset

## Fixture Generation Rule

Create fixture generation must fail if the selected source design doc already has a plan:

- Check `.rafa/plans/*/plan.json` `sourceFile` values
- Require `source-doc` not already referenced

Generator command:

```bash
go run ./scripts/gen_demo_create_fixture.go \
  --source-doc docs/designs/plan-create-command.md \
  --stream-log /path/to/create-stream.jsonl
```

## Testing

- CLI parsing tests for mode defaults/validation.
- Create view tests for unsaved completion and replay.
- App startup test for `--demo-mode=create`.
- Guard tests for “source doc already planned” enforcement.
