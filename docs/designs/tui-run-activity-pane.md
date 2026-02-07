# Technical Design: Scrollable Panes with Scrollbars (Run View)

## Overview

In the plan execution **Run** TUI view (`internal/tui/views/run.go`), the **Activity** timeline is currently rendered as a section inside the left progress panel, and the **Output** and **Tasks** areas don’t expose scroll position (no scrollbar). This makes it hard to:

- Visually scan Activity as a distinct surface (it competes with Usage + Tasks for vertical space).
- Inspect older Activity entries (it only shows the most recent lines).
- Understand scroll position (no scrollbar).

This change introduces consistent, scrollable panes with visible scrollbars in the Run view:

- **Activity** becomes its own bordered pane in the left column, positioned **below Progress**, aligned with the main Output pane.
- **Activity is plan-wide** (not per-task) and includes clear separators between tasks/attempts.
- **Activity**, **Output**, and the **Tasks list** all:
  - Scroll on demand (user-controlled).
  - Auto-scroll only when the user is at the end (bottom) and content is growing/advancing.
  - Display a visible scrollbar.
- Mouse wheel scrolling works anywhere a pane is scrollable, when the terminal permits it.

## Goals

- Make Activity a dedicated bordered pane in the Run view.
- Keep Activity history for the whole plan and add separators on task/attempt transitions.
- Provide manual scrolling for Activity history (plan-wide, not just current task).
- Provide “stick-to-bottom” auto-scroll semantics when at bottom; pause auto-scroll when the user scrolls up.
- Render scrollbars for Activity, Output, and Tasks list that reflect scroll position and content length.
- Keep Output pane keyboard behavior compatible (default focus remains Output; existing scroll keys still work).
- Enable mouse wheel scrolling for all scrollable panes (Activity/Tasks/Output).

## Non-Goals

- Changing what events are considered “Activity” (sources of `ToolUseMsg` / `ToolResultMsg` remain unchanged).
- Redesigning the **Create Plan** view (it already has a dedicated Activity pane).
- Adding horizontal scrolling.
- Adding task selection, filtering, or search in the Tasks list (scroll-only for now).

## Architecture

### New Layout (Run View)

Current:

```
┌─────────────────────────────┬──────────────────────────────────┐
│ Progress (includes Activity) │ Output                            │
└─────────────────────────────┴──────────────────────────────────┘
```

Proposed:

```
┌─────────────────────────────┬──────────────────────────────────┐
│ Progress                     │ Output                            │
│ (header + usage + tasks)     │ (scroll + bar)                    │
├─────────────────────────────┤                                  │
│ Activity                     │                                  │
│ (plan-wide, scroll + bar)    │                                  │
└─────────────────────────────┴──────────────────────────────────┘
```

Within the **Progress** pane, the **Tasks** section becomes its own scrollable sub-region with a scrollbar (so the header + usage remain visible).

### Key Components

- **Run view layout**: `internal/tui/views/run.go`
  - Split the current left panel into two bordered panes stacked vertically:
    - `renderProgressPane(...)` (top; header + usage + scrollable Tasks)
    - `renderActivityPane(...)` (bottom; plan-wide activity timeline with separators)
  - Render Output as the right column with a larger width share and a scrollbar.

- **Scrollable panes & scrollbar rendering**: `internal/tui/components/`
  - Add a small, reusable scrollbar helper (used by Activity/Output/Tasks).
  - Introduce a generic `ScrollViewport` (or specific viewports) that:
    - Wraps `bubbles/viewport.Model` for vertical scrolling.
    - Tracks `autoScroll` state consistently across panes.
    - Renders a visible vertical scrollbar.
  - Update `components.OutputViewport` to render a scrollbar (using the shared helper).

## Technical Details

### Pane Sizing & Alignment

The Run view will render **two columns**, with Output taking the larger share of the width:

- `leftWidth`: fixed ratio (e.g., 30% of total) with a minimum width (e.g., 24 cols).
- `outputWidth`: remaining width (prefer keeping ~70% as the default), with a minimum width (e.g., 36 cols).

The **left column** is split vertically into two bordered rectangles:

- `progressHeight` (top): header + usage + scrollable Tasks sub-region
- `activityHeight` (bottom): plan-wide Activity timeline (scrollable)

Suggested height allocation:

- `progressHeight`: ~35–45% of available height, clamped to a minimum (e.g., 10 lines) so header+usage remain readable.
- `activityHeight`: remaining height, clamped to a minimum (e.g., 6 lines) so scrolling is useful.

If the terminal is too narrow to satisfy `leftWidth` + `outputWidth` minimums, fall back to a **single-column** layout:

1. Output (top, largest height share)
2. Progress (middle)
3. Activity (bottom)

Width accounting for scrollbars:

- Any pane that renders a 1-column scrollbar must subtract 1 from the underlying viewport/content width (so wrapped text and truncation don’t collide with the scrollbar).
- In the Progress pane, only the Tasks sub-region renders a scrollbar (the header + usage remain static).

### Scroll Models

Add a small generic scrollable viewport in `internal/tui/components/` and reuse it for Activity and Tasks. Keep `components.OutputViewport` as the specialized text viewport (wrapping + ring buffer) but update it to render a scrollbar using the same helper.

**Important**: `internal/tui/components` must not depend on `internal/tui/views` (the views already import components), so component APIs should operate on plain strings (`[]string` lines or `string` content), not view-layer structs.

Suggested `ScrollViewport` API (used for Activity + Tasks):

```go
type ScrollViewport struct {
  // wraps viewport.Model, has autoScroll, ring buffer, width/height, etc.
}

func NewScrollViewport(width, height, maxLines int) ScrollViewport
func (s *ScrollViewport) SetSize(width, height int)
func (s *ScrollViewport) SetLines(lines []string)
func (s *ScrollViewport) Update(msg tea.Msg) (ScrollViewport, tea.Cmd)
func (s ScrollViewport) View() string // includes scrollbar

// Optional helpers for view-layer ergonomics:
func (s ScrollViewport) AtBottom() bool
func (s *ScrollViewport) SetAutoScroll(enabled bool)
func (s *ScrollViewport) EnsureVisible(lineIndex int, center bool)
```

#### Activity (Plan-Wide + Separators)

- Keep `RunningModel.activities` for the whole plan (do not clear on task start).
- On each `TaskStartedMsg`:
  - Reset per-task usage counters (e.g., `taskTokens`) as today (but do not clear Activity).
  - Append a separator entry before tool activity for that attempt, for example:
  - `── Task 3/10: Implement session management (Attempt 2/5) ──`
- Cap the in-memory Activity history to a fixed number of entries (e.g., 2,000) to prevent unbounded growth in long runs.
- Rendering responsibility stays in the Run view:
  - Convert `[]RunActivityEntry` (including separators) into `[]string` lines.
  - Include done/spinner indicator for tool entries.
  - Truncate to viewport width (minus scrollbar column).
  - Call `activityView.SetLines(lines)` and preserve scroll state unless auto-scroll is active.

#### Output (Scrollbar + Existing Auto-Scroll)

- Update `components.OutputViewport` to render a scrollbar alongside the viewport content.
- Keep existing behavior:
  - Wrapped text/ring buffer behavior remains unchanged.
  - Auto-scroll pauses when the user scrolls up and resumes when they return to bottom.
- Mouse wheel scrolling is supported by passing `tea.MouseMsg` through to the underlying `viewport.Update`.

#### Tasks List (Scrollable + Scrollbar)

- Add a dedicated `tasksView` in `RunningModel` to render the full task list as lines with a scrollbar.
- Keep the Progress pane header + Usage static; allocate remaining height to the Tasks viewport.
- Auto-scroll semantics for Tasks are interpreted as “auto-follow the current task”:
  - Default: enabled (keeps the active task visible with minimal movement).
  - User scroll interaction disables auto-follow until re-enabled (End/G, or a dedicated “jump to current” key if added).

### Auto-Scroll Semantics

Consistent semantics across scrollable regions:

- Start with `autoScroll=true` for each scrollable region.
- Any user-initiated upward scrolling disables auto-scroll for that region.
- Returning to the “end” re-enables auto-scroll for that region:
  - Activity/Output: “end” means bottom of the content.
  - Tasks: “end” means “follow current task” (the plan’s active task advances).

This preserves “scroll on demand” while still keeping the default view live as new output/activity arrives and as the plan progresses.

### Scrollbar Rendering

Use a shared helper to render a 1-column scrollbar (Activity, Output, and Tasks viewport):

- Track: `│` (subtle style)
- Thumb: `█` (or `┃`) positioned based on scroll percent / visible fraction

Suggested thumb sizing/positioning:

- `contentLines := total number of rendered lines`
- `viewLines := viewport height`
- `maxYOffset := max(0, contentLines-viewLines)`
- `thumbSize := max(1, int(float64(viewLines)*float64(viewLines)/float64(contentLines)))` (proportional to visible fraction)
- `thumbMaxTop := viewLines - thumbSize`
- `thumbTop := 0` when `maxYOffset==0`, else `int(float64(yOffset)/float64(maxYOffset) * float64(thumbMaxTop))`

### Focus, Keyboard, and Mouse (Scrolling “On Demand”)

There are three scrollable regions in the Run view:

- Tasks list (top-left, within the Progress pane)
- Activity pane (bottom-left)
- Output pane (right)

Add a focus state to `RunningModel`:

```go
type focusPane int
const (
  focusOutput focusPane = iota
  focusActivity
  focusTasks
)
```

Keyboard:

- `tab`: cycle focus across scrollable regions.
- Default focus remains Output (preserves existing behavior).
- Scroll keys (`up/down`, `pgup/pgdown`, `home/end`, `g/G`, `ctrl+u/ctrl+d`) apply to the focused region.

Mouse wheel:

- When the terminal permits it, mouse wheel events should scroll the region under the cursor.
- If the wheel event is over a specific region, route the event to that region and set focus accordingly.
- Bubble Tea is already configured with mouse support in `internal/tui/app.go` (`tea.WithMouseCellMotion()`); the Run view only needs to route `tea.MouseMsg` to the correct viewport.

Implementation note (hit-testing):

- Track pane bounding boxes (Activity/Progress/Output) and the Tasks sub-region bounding box (within Progress) in screen coordinates.
- On `tea.WindowSizeMsg` (and any layout recalculation), recompute these bounds so `tea.MouseMsg` events can be routed reliably even with borders/padding.

UX affordance:

- Highlight the focused top-level pane border (Activity/Progress/Output) by changing `BoxStyle.BorderForeground(...)` to the same color used by `styles.SelectedStyle` (or introduce an exported focused border color/style in `internal/tui/styles`).
- When `focusTasks` is active, highlight the Progress pane border (since the Tasks viewport is inside it).
- Add status bar hints (e.g., `Tab Focus`, `↑↓ Scroll`, `Focus: Output|Activity|Tasks`) while running.

### Rendering Progress Pane (Tasks Scrollbar)

Refactor the existing left panel into:

- `renderActivityPane(...)` (new)
- `renderProgressPane(...)` (existing content minus Activity)

In `renderProgressPane(...)`:

- Keep the header (task N/M, attempt, elapsed) static.
- Keep the Usage section static.
- Allocate the remaining vertical space to a scrollable Tasks viewport with a scrollbar.

### Interaction With Run States

- **Running**: Activity, Tasks, and Output are all scrollable and show scrollbars.
- **Done/Cancelled**: keep panes scrollable (useful for inspection) while status bar options remain `Enter Home` / `q Quit`.

## Security

- No new external I/O or permissions changes.
- No sensitive data changes; Activity remains a display-only view of existing event metadata.

## Edge Cases

| Case | Handling |
|------|----------|
| Very small terminal width | Fall back to a single-column layout (Output, then Progress, then Activity) with all scrollbars enabled; ensure no negative widths. |
| Very small terminal height | Allow panes/sub-regions to shrink; ensure scrollbars render within remaining height. |
| Activity/Output/Tasks content shorter than viewport | Scrollbar renders an “empty” track (no thumb) or a minimal thumb; viewport remains stable. |
| Plan-wide Activity grows large | Cap in-memory Activity entries; keep viewport ring buffer bounded. |
| Rapid output/activity updates | Ring buffers cap memory; auto-scroll only when the user is at the end for that region. |
| Spinner animation in last activity | Re-render only the last activity line when possible; preserve YOffset unless auto-scroll is enabled. |
| Mouse wheel unsupported by terminal | Keyboard scrolling still works; mouse events are ignored or treated as no-ops. |

## Performance

- Rendering is lightweight (plain text + 1-column scrollbar) for Activity/Tasks/Output.
- Cap Activity and Output history with ring buffers (and cap Activity entries in-memory) to prevent unbounded growth.
- Avoid per-frame heavy recomputation:
  - Only rebuild Activity lines on new entries / done transitions / spinner tick.
  - Only rebuild Tasks lines on task status/current task changes.
  - Only rewrap Output when width changes (existing `OutputViewport` behavior).

## Testing

- **Unit tests** for `components.ScrollViewport` (Activity/Tasks):
  - Auto-scroll toggles on scroll keys and mouse wheel.
  - Updating lines while `autoScroll=false` preserves YOffset.
  - `EnsureVisible(...)` behavior (keeps current task visible when enabled).
  - Scrollbar thumb position for top/middle/bottom and short-content cases.
- **Unit tests** for `components.OutputViewport` scrollbar integration:
  - Scrollbar appears and does not break wrapping/rewrap behavior when width changes.
  - Auto-scroll semantics remain unchanged.
- **Run view tests** (`internal/tui/views/run_test.go`):
  - View contains a dedicated “Activity” pane and includes task/attempt separators.
  - Focus cycles across Output/Activity/Tasks and routes scroll keys to the correct viewport.
  - Mouse wheel events (with coordinates) scroll the pane under the cursor (or fall back to focus).
  - Narrow-width fallback produces the single-column layout.

## Rollout

- Ship as a UI-only refactor.
- No flags required; risk is localized to TUI rendering and shared viewport components.
- If `OutputViewport` scrollbar is enabled globally, verify other usages (e.g., PlanCreate “Response” pane) still render correctly; otherwise gate the scrollbar behind an option.
- Keep a temporary constant to force the legacy Run layout (single left progress panel with embedded Activity section) and/or disable scrollbars for quick rollback during early testing.

## Rollback / Recovery

- Revert to previous Run view layout by removing:
  - Activity pane rendering
  - focus + mouse routing logic
  - `ScrollViewport` usage for Activity/Tasks
  - Output scrollbar integration (if scoped to Run view)
  - and restoring Activity as a section inside the Progress pane.

## Risks & Mitigations

- Risk: Scroll key conflicts between panes.
  - Mitigation: explicit focus + `tab` cycle; status bar hint; default focus remains Output.
- Risk: Mouse wheel hit-testing is brittle with borders/padding.
  - Mitigation: route by region when coordinates are clearly inside; otherwise fall back to focused region; set focus on successful hit.
- Risk: Spinner animation causes viewport jitter.
  - Mitigation: update only the last activity line; preserve YOffset unless auto-scroll is enabled.
- Risk: Output scrollbar changes effective wrapping width.
  - Mitigation: subtract scrollbar width from content width; add tests for wrap/rewrap behavior.
- Risk: Plan-wide Activity consumes memory on long runs.
  - Mitigation: cap in-memory entries and viewport buffer size.
- Risk: Tasks “auto-follow current task” feels jumpy.
  - Mitigation: disable auto-follow on user scroll; provide an explicit re-enable action (End/G or “jump to current” key) and a visible focus/auto indicator.
- Risk: Layout becomes cramped on narrow terminals.
  - Mitigation: enforce minimum widths and fall back to the single-column layout.

## Trade-offs

- **Focus-based scrolling** vs **dedicated key chords** for pane scrolling:
  - Focus is more discoverable and scalable to future panes, but adds state + visual affordances.
- **Mouse-wheel-under-cursor** vs **mouse-wheel-on-focus**:
  - Under-cursor feels natural but requires careful region hit-testing; focus-based is simpler but less intuitive.
- **Generic `ScrollViewport`** vs per-pane bespoke implementations:
  - Generic reduces duplication (Activity/Tasks) and centralizes scrollbar logic; bespoke may be simpler for one-offs.
- Using `bubbles/viewport` vs a custom scroll implementation:
  - `viewport` reduces custom code but requires careful content updates to preserve scroll state; custom scroll gives more control but adds maintenance.

## Open Questions

- Should Output’s scrollbar be enabled everywhere `OutputViewport` is used, or only in the Run view?
  - Recommendation: make it an option and enable it in Run first to reduce blast radius.
- For Tasks auto-follow: should the current task be centered, top-aligned, or just “kept visible” with minimal movement?
  - Recommendation: “kept visible” (only scroll when the current task leaves the viewport) to reduce jumpiness.
- Should Activity separators be inserted on every attempt or only when the task number changes?
  - Recommendation: insert on every `TaskStartedMsg` (includes attempt), since retries are meaningful context.

## Ready To Implement

Yes — decisions are captured:

- Activity is plan-wide with task/attempt separators.
- Activity, Output, and Tasks list get scrollbars + consistent auto-scroll semantics.
- Mouse wheel scroll works for all scrollable regions when supported by the terminal.

## Implementation Tasks

- [ ] Add a shared scrollbar helper in `internal/tui/components/` (track + thumb rendering).
- [ ] Add `internal/tui/components/scrollviewport.go` implementing `ScrollViewport` (viewport + auto-scroll + scrollbar + ring buffer + `EnsureVisible`).
- [ ] Add unit tests for `ScrollViewport` (auto-scroll, scroll preservation, scrollbar math, `EnsureVisible`).
- [ ] Update `internal/tui/components/output.go` to render a scrollbar and subtract scrollbar width from wrapping/viewport sizing; update `internal/tui/components/output_test.go` accordingly.
- [ ] Refactor `internal/tui/views/run.go`:
  - [ ] Split `renderLeftPanel` into `renderActivityPane` + `renderProgressPane`.
  - [ ] Update the Run view layout:
    - [ ] Render the left column as a vertical stack (Progress on top, Activity below).
    - [ ] Render Output as the right column with a larger width share.
  - [ ] Add `activityView` + `tasksView` scroll models and wire sizing into window/layout updates.
  - [ ] Make Activity plan-wide:
    - [ ] Stop clearing Activity on `TaskStartedMsg`.
    - [ ] Separate per-task usage reset from Activity history (replace `clearActivities()` with a `resetTaskUsage()`-style helper).
    - [ ] Append a task/attempt separator on each `TaskStartedMsg`.
    - [ ] Cap Activity history in-memory.
  - [ ] Make the Tasks list scrollable with scrollbar and “auto-follow current task” behavior.
  - [ ] Add focus state + `tab` cycling across Output/Activity/Tasks; route scroll keys to the focused region.
  - [ ] Route mouse wheel events to the region under the cursor (and set focus), falling back to focused region when ambiguous.
  - [ ] Update layout sizing logic and keep the narrow-width fallback (single-column layout).
- [ ] Add focused-pane border styling in `internal/tui/styles/styles.go` (or derive from `styles.SelectedStyle`) and update the Run view to use it.
- [ ] Update `internal/tui/views/run_test.go`:
  - [ ] Assert Activity separators exist and Activity is not cleared between tasks.
  - [ ] Assert focus cycling and scroll routing across Output/Activity/Tasks.
  - [ ] Assert mouse wheel routing under cursor works (or falls back to focus).
  - [ ] Assert narrow-width fallback renders the single-column layout.
- [ ] Manually validate in a real terminal:
  - [ ] Activity scrolls (keys + mouse), shows scrollbar, and auto-scrolls only at end.
  - [ ] Output scrolls (keys + mouse), shows scrollbar, and preserves existing auto-scroll behavior.
  - [ ] Tasks list scrolls (keys + mouse), shows scrollbar, and follows current task unless user scrolls away.
  - [ ] Layout remains readable at common terminal sizes (e.g., 100x30, 140x40).
