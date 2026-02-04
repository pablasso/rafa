# Technical Design: Demo Mode Reborn (TUI Only)

## Overview

Reintroduce demo mode as a **TUI-only** experience that replays a real plan run.
The demo uses the recorded execution in `.rafa/plans/KG8JBy-rafa-workflow-orchestration/`
so the output, tool usage, and token/cost data reflect actual Claude runs. No CLI
entrypoints are added.

## Goals

- Provide a **TUI menu item** to start demo mode.
- Replay **real plan data** (plan.json + output.log) for authenticity.
- Simulate activity timeline updates (tool use + tool results) and token/cost usage.
- Keep demo mode isolated from git checks, locks, and plan persistence.

## Non-Goals

- No CLI command (TUI-only).
- No changes to executor behavior for real runs.
- No external configuration system (environment variables only if needed later).

## Data Source

Demo mode loads:

- `plan.json` for task metadata (titles, IDs, ordering).
- `output.log` for realistic Claude stream output, tool usage, and token usage.

Both are loaded from:

```
.rafa/plans/KG8JBy-rafa-workflow-orchestration/
```

If the data is missing or invalid, demo mode falls back to a small in-memory
dataset to avoid breaking the TUI.

## Playback Model

Demo mode replays a subset of the captured execution:

- **Task start** → `TaskStartedMsg`
- **Tool use** → `ToolUseMsg`
- **Tool result** → `ToolResultMsg`
- **Token usage** → `UsageMsg`
- **Output text** → streamed to the output viewport
- **Task completion** → `TaskCompletedMsg`
- **Plan completion** → `PlanDoneMsg`

The output stream is derived from the exact JSON lines in `output.log`. Parsing
reuses the same stream-line formatting as real runs to keep the output view
consistent.

## Pacing Defaults

Defaults are tuned for TUI demos:

- `LineDelay`: 12ms between output/events
- `TaskDelay`: 400ms between tasks
- `MaxTasks`: 5 (subset of the plan to keep demo time reasonable)
- `MaxEventsPerTask`: 450 (avoids excessively long task playback)

These defaults can be adjusted in code if needed for longer showcases.

## UI Integration

Home menu adds **Demo Mode** (`d`) under Execute:

- Start demo playback immediately in the running view.
- Running view displays `[DEMO]` in the status bar.

## Risks & Mitigations

- **Large output log lines** can exceed scanner limits.
  - Increase scanner buffer size during parsing.
- **Missing demo data** would break the feature.
  - Provide a fallback dataset.
- **Very long tasks** could make the demo sluggish.
  - Limit playback to a subset of tasks and events.

## Testing

- Run `make fmt`, `make build`, `make test`.
- Manual sanity: launch TUI, press `d`, ensure output, activity, and usage appear.
