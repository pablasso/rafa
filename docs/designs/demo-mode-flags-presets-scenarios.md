# Technical Design: Demo Mode Flags, Presets, and Scenarios

## Overview

Demo mode was reintroduced in `e415dbf824b8c4cae3e62c717e94364a6b720b56` as a TUI-only replay of a real run, using `.rafa/plans/.../plan.json` + `output.log` for authenticity.

This design evolves demo mode so it is:

- **Opt-in**: Demo mode is launched explicitly via a flag (e.g. `rafa --demo`) and **auto-starts playback**.
- **Configurable**: Playback can be configured via **preset** flags (with a default) to target a quick ~1 minute run, a ~30 minute run, or an hours-long run.
- **Future-proof**: Demo data remains “real”, but the demo runtime no longer reads from a repository’s `.rafa/` directory.
- **Scenario-driven**: Demo mode supports multiple **result scenarios** (default: all tasks succeed), plus a few variants with flakiness and failures.

Related docs:

- `docs/designs/demo-mode-reborn.md` (current implementation baseline)
- `docs/designs/tui-demo-mode.md` (prior design with scenarios/speed ideas)

## Goals

- Launch demo playback only when Rafa is started with `--demo`.
- Remove the Demo Mode menu item from the TUI (demo is started by flag, not via the UI).
- Add demo presets selectable by flag (with a default):
  - **Quick**: ~1 minute total playback
  - **Medium**: ~30 minutes total playback
  - **Slow**: hours-long playback
- Keep demo output/tool/usage data rooted in a **real run**, but decouple from `.rafa/` at runtime.
- Add demo scenarios for task results:
  - Default: all tasks succeed
  - Additional: at least two variants introducing retries/flakiness and hard failures

## Non-Goals

- Reintroducing full CLI subcommands in this release (we only add flags to the TUI entrypoint).
- Perfectly matching wall-clock durations (targets are approximate and dataset-dependent).
- Simulating every possible executor edge case; scenarios focus on core UI states (success/retry/failure).

## Architecture

High level:

```
cmd/rafa/main.go
  └─ parse flags (demo enabled / preset / scenario)
     └─ tui.Run(tui.Options{Demo: ...})
         └─ If demo enabled:
              start in running view and auto-start playback
                demo.LoadDataset(...)  (embedded fixture)
                demo.ApplyScenario(...)
                demo.NewPlayback(...).Run(...)
```

Key changes from current:

- `cmd/rafa/main.go` must accept flags (currently it errors on any args).
- `tui.Run()` should take options (or read from an injected config) to control:
  - whether demo mode is enabled
  - which preset/scenario to use when demo starts
- `internal/demo` should load a **shipped fixture** (embedded or installed alongside the binary), rather than reaching into `.rafa/`.

## Technical Details

### CLI / Flags

Add a minimal flag surface to the TUI entrypoint:

- `--demo` (bool): starts demo playback (auto-start)
- `--demo-preset` (string): `quick|medium|slow` (default: `medium`)
- `--demo-scenario` (string): `success|flaky|fail` (default: `success`)

Notes:

- If any `--demo-*` flag is provided without `--demo`, exit with a clear error.
- Positional args remain unsupported (keep the “TUI-only” stance while allowing flags).
- `--help` should show usage including demo flags.

### TUI Behavior (No Demo Menu)

Design: there is no Demo Mode menu item in the TUI.

When `--demo` is provided:

- TUI starts directly in the running view in demo mode.
- Playback auto-starts without user interaction.

When `--demo` is not provided:

- TUI behaves normally (Create Plan / Run Plan / Quit) with no demo entrypoint.

### Demo Presets (Duration Targets)

We want presets that map to **approximate total playback time**, not just “fast/slow”.

Proposed model:

- Base dataset is a fixed set of tasks + events (see “Demo Dataset”).
- Presets define:
  - `TargetDuration` (e.g. 1m / 30m / 2h)
  - `MaxTasks` and `MaxEventsPerTask` caps (optional; usually consistent across presets)
  - A **pacing strategy** that derives `LineDelay` and `TaskDelay`

Implementation approach (preferred):

1. Build the demo “play list” (tasks to play + events to stream) after applying the scenario.
2. Estimate total event count: `Nevents` and task count `Ntasks`.
3. Given baseline delays (current defaults), compute an estimated baseline time:
   - `Tbase ≈ Nevents*baseLineDelay + Ntasks*baseTaskDelay`
4. Compute a scale factor: `s = TargetDuration / Tbase`
5. Apply scaling with sensible bounds:
   - `LineDelay = clamp(baseLineDelay*s, minLineDelay, maxLineDelay)`
   - `TaskDelay = clamp(baseTaskDelay*s, minTaskDelay, maxTaskDelay)`

This ensures “quick/medium/slow” hit the target even if we tweak the dataset, and avoids hardcoding “30 minutes” into raw delays.

Preset defaults (initial targets):

- `quick`: `TargetDuration=1m`
- `medium`: `TargetDuration=30m`
- `slow`: `TargetDuration=2h` (or `3h` if we want “hours” to be more obvious)

Recommended task caps (to keep quick runs short and long runs varied):

| Preset | TargetDuration | MaxTasks |
|--------|----------------|----------|
| `quick` | 1 minute | 5 |
| `medium` | 30 minutes | 10 |
| `slow` | 2 hours | all tasks in fixture |

Suggested bounds (initial values):

- `LineDelay`: clamp to `[1ms, 2s]`
- `TaskDelay`: clamp to `[0s, 60s]`

### Demo Scenarios (Task Results)

Current: playback uses dataset attempt success and stops on first failed task.

Design: add scenario-driven attempt outcomes, independent from the raw captured run.

Scenario set:

- `success` (default): all tasks succeed on attempt 1
- `flaky`: a small subset of tasks fail once, then succeed on retry
  - Example: tasks 2 and 4 fail attempt 1, succeed attempt 2
- `fail`: a task fails repeatedly and the plan ends as failed
  - Example: task 3 fails up to `executor.MaxAttempts` and stops

How to implement:

- Introduce a scenario “plan” that, for each task in the demo run, specifies:
  - how many attempts to simulate
  - which attempt succeeds (if any)
- Playback emits the same message types the executor path uses today:
  - `TaskStartedMsg` for each attempt
  - stream events for that attempt
  - `TaskCompletedMsg` or `TaskFailedMsg`
  - `PlanDoneMsg` with `Success` and a consistent summary

Event data for retries/failures:

- Default: reuse the fixture’s “normal” attempt stream per task (prefer a successful attempt if the source run had retries).
- For “failed” attempts in flaky/fail scenarios:
  - Stream a subset of events then inject an error output line (e.g. `Error: demo injected failure`)
  - Emit `TaskFailedMsg{Attempt: n}`.
- For retries:
  - Re-run the same base events (or a slightly different subset) on attempt 2+.

This keeps the demo data real, while allowing deterministic scenario coverage for UI states.

Determinism rule (so runs/tests are repeatable):

- Choose targets by **index within the playback task list** (1-based): `fail` targets #3, `flaky` targets #2 and #4.
- If the preset’s `MaxTasks` would exclude a target index, pick the last available task index instead.

### Demo Dataset (Real, Shipped, Not `.rafa/`)

Current: `internal/demo/data.go` reads from:

- `.rafa/plans/KG8JBy-rafa-workflow-orchestration/plan.json`
- `.rafa/plans/KG8JBy-rafa-workflow-orchestration/output.log` (currently ~41MB)

Design requirement: do not read from `.rafa/` at runtime.

Proposed approach:

1. Add a **curated fixture** in-repo, derived from a real `.rafa/plans/...` run:
   - Store a **pre-parsed `Dataset` JSON** (recommended) containing:
     - plan task metadata (IDs/titles/order)
     - per-task attempt event streams (output/tool/usage)
   - Keep it small (target: <1–2MB) by:
     - limiting to a subset of tasks
     - limiting to early, representative events per task
     - optionally truncating extremely large tool results
     - coalescing many small text deltas into larger output chunks (e.g. flush on newline or ~512–1024 bytes)
2. Ship the fixture with the binary using `go:embed` (preferred), or as install-time data next to the binary.
3. `demo.LoadDefaultDataset()` loads from the embedded fixture rather than filesystem `.rafa/`.

Why “pre-parsed Dataset JSON”:

- Today’s `.rafa/.../output.log` is large and includes lots of information we don’t need for demo playback.
- A Dataset fixture can include only what the demo player needs (already extracted output fragments + tool/usage events), which is smaller and avoids depending on the exact `output.log` format.

Fixture format (v1):

- File: embedded JSON (optionally gzip-compressed), e.g. `internal/demo/fixtures/default.v1.json`.
- Top-level includes a version for forward compatibility.
- The fixture uses a *minimal* plan/task representation required for the running view (IDs, titles, ordering).

Example shape:

```json
{
  "version": 1,
  "plan": {
    "id": "DEMO",
    "name": "demo",
    "tasks": [{"id":"t01","title":"..." }]
  },
  "attempts": [{
    "taskID": "t01",
    "attempt": 1,
    "success": true,
    "events": [
      {"type":"output","text":"..."},
      {"type":"tool_use","toolName":"Read","toolTarget":"README.md"},
      {"type":"tool_result"},
      {"type":"usage","inputTokens":123,"outputTokens":456,"costUSD":0.01}
    ]
  }]
}
```

Loader behavior:

- Reject unknown `version` with a clear error and fall back to `FallbackDataset()`.
- Treat missing or malformed fixtures as non-fatal (demo still starts with fallback + warning).

Fixture generation:

- Add a small generator (e.g. `scripts/gen_demo_fixture.go` or `internal/demo/fixtures/gen.go`) that:
  - reads from a chosen `.rafa/plans/<id>-<name>/` directory (developer-only)
  - parses output into demo `Event`s using existing parsing utilities
  - writes the curated fixture file into the repo
- Document how/when to refresh the fixture if we want a newer “real run”.

Initial fixture source:

- Use `.rafa/plans/KG8JBy-rafa-workflow-orchestration/` as the fixture source (most realistic run), but only as generator input.
- The generator should prefer the **latest successful attempt** per task when multiple attempts exist in the source log.
- The `KG8JBy` source log includes at least one attempt with a missing `=== Task ...: SUCCESS ===` footer (likely due to Rafa being terminated between Claude completion and footer write); treat these as successful by inferring from the JSON `result` event (`subtype` / `is_error`) when available.

### Backward Compatibility

- Normal plan execution behavior remains unchanged.
- Demo is only available via `--demo` and has no runtime dependency on `.rafa/` (fixtures are embedded).
- Demo mode becomes an opt-in “power user” feature via flags.

## Security

- Demo mode should not touch git, locks, or `.rafa/` persistence.
- Embedded fixtures must not include secrets (tokens, private file contents). The generator should support basic redaction/truncation if needed.
- Default redaction policy (so we can ship fixtures safely):
  - Normalize absolute paths (e.g. `/Users/<name>/...`) to repo-relative paths when possible, otherwise replace the home prefix with `<HOME>/`.
  - Truncate unusually long output/tool-target strings to a sane limit (e.g. 2–4KB) to avoid accidental large embeds.

## Edge Cases

| Case | Handling |
|------|----------|
| `rafa --demo-preset=slow` without `--demo` | Exit with helpful error |
| Invalid preset/scenario values | Exit with helpful error + list valid values |
| Fixture missing/corrupt | Fall back to in-memory `FallbackDataset()` and show a warning banner |
| Extremely small terminals | Existing “terminal too small” view still applies |

## Performance

- Ensure fixture loading is fast and does not allocate excessively.
- Avoid embedding very large raw logs; prefer trimmed/pre-parsed fixtures.
- Scenario transformations should be O(tasks + events).

## Testing

- Unit:
  - flag parsing: valid/invalid combinations
  - preset pacing: scaling hits approximate target duration bounds
  - scenario planner: produces correct attempt sequences and final plan result
- Integration (optional):
  - golden-style test that runs demo playback with a small fixture and asserts message ordering (TaskStarted → events → completion).

## Rollout

- Ship behind `--demo` only.
- Optionally announce in docs/changelog for contributors who want to demo the TUI.
- After implementation, update `CONTRIBUTING.md` to reflect the flag-driven auto-start flow (and removal of the home menu item).

## Rollback / Recovery

- Remove flag parsing and revert demo mode to current behavior (or remove demo mode entirely) without affecting normal plan execution.

## Risks & Mitigations

- Risk: embedding a big dataset bloats the binary.
  - Mitigation: curate/trim fixture; consider gzip-compressed embedded assets.
- Risk: “real run” data contains sensitive content.
  - Mitigation: fixture generator supports truncation/redaction; review fixture changes in PRs.
- Risk: target-duration presets feel inconsistent when dataset changes.
  - Mitigation: scale delays based on computed event counts; add bounds.

## Implementation Plan

1. Flags + options plumbing
   - Parse flags in `cmd/rafa/main.go` and pass into `tui.Run(opts)`.
2. Demo auto-start plumbing
   - Start in running view + auto-start playback when demo is enabled.
3. Remove demo menu entrypoint
   - Remove demo item/shortcut from the home view.
4. Demo dataset fixture
   - Add fixture file(s) + `go:embed` loader.
   - Add generator script and documentation.
5. Presets
   - Implement `PresetQuick/Medium/Slow` with target durations and delay scaling.
6. Scenarios
   - Implement `success|flaky|fail` outcome planning + playback attempt sequencing.
7. Tests
   - Add new scenario/preset/flag parsing tests.

## Trade-offs

- Embed fixture vs read from repo `.rafa/`:
  - Embed is future-proof and works anywhere; `.rafa/` is fragile and repo-dependent.
- Multiple fixtures vs scenario transformations:
  - Multiple fixtures increase maintenance; transformations keep one real base dataset but add some synthetic behavior.
- Target-duration scaling vs hardcoded delays:
  - Scaling is resilient to dataset changes; hardcoded delays are simpler but brittle.

## Open Questions

None.

## Ready To Implement

Ready to implement with the current decisions:

- Default preset: `medium`
- Auto-start via `--demo` (no demo menu item)
- Scenario set: `success|flaky|fail`
- Fixture format: embedded, pre-parsed `Dataset` JSON generated from `KG8JBy-rafa-workflow-orchestration`
