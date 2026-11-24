---
slug: one-off-slip-flow
run: 20261124-162519-one-off-slip-flow
outcome: done
generated: 2026-11-24T16:38:16Z
---

# Run summary: one-off-slip-flow

## Outcome

- Status: done
- Branch: one-off-slip-flow
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-low approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 2
- Checkpoints: 5
- Duration: 11m24s
- Cost: $—
- Fresh input tokens: 402054
- Cache read tokens: 3228652
- Cache write tokens: 0
- Output tokens: 58528

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 3689234 | $— |

## Phases

### Phase 1: Scope label, slip hub entry point, and single switcher

Tasks:

- [x] Rename `db.SlateScopeThursday.Label()` to `"Thursday Slate"` in `internal/db/slips.go`, update static option labels in `web/templates/slip.html` and `web/templates/week.html`, and update corresponding tests in `internal/db/slips_test.go`, `internal/web/home_test.go`, and `internal/web/slips_test.go`.
- [x] Add `WeekID` and `ScopeBadge` to `SlipView` (`internal/web/slips.go`), display `ScopeBadge` and a `+ New slip` link (`/slips/new?week=...`) in `web/templates/partials.html` for both single-slip and multi-slip weeks, remove the redundant top-bar `#slip-switcher` (`internal/web/page.go`, `internal/web/slips.go`, `web/templates/layout.html`, `web/static/app.js`), and update `internal/web/layout_test.go` and `internal/web/slips_test.go`.

Phase totals: 2 checkpoints, 4m6s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Rename `db.SlateScopeThursday.Label()` to `"Thursday Slate"` in `internal/db/slips.go`, update static option labels in `web/templates/slip.html` and `web/templates/week.html`, and update corresponding tests in `internal/db/slips_test.go`, `internal/web/home_test.go`, and `internal/web/slips_test.go`.

- Updated `db.SlateScopeThursday.Label()` in `internal/db/slips.go` from `"Thursday Night"` to `"Thursday Slate"`.
- Updated the static `<option value="thursday">` text in `web/templates/slip.html` and `web/templates/week.html` to `"Thursday Slate"`, and updated the schedule name placeholder in `web/templates/week.html` to `"e.g. Thursday Slip"`.
- Updated `TestSlateScopeLabel` in `internal/db/slips_test.go`, `TestCreateSlipFormSlateScopeSelection` in `internal/web/slips_test.go`, and added `"Thursday Slate"` scope badge assertions to `TestGroupWeekRendersMultipleSlips` and `TestGroupWeekAutoCreatesMultipleScheduledSlipsAndRendersView` in `internal/web/home_test.go`.

### Task 2: Add `WeekID` and `ScopeBadge` to `SlipView` (`internal/web/slips.go`), display `ScopeBadge` and a `+ New slip` link (`/slips/new?week=...`) in `web/templates/partials.html` for both single-slip and multi-slip weeks, remove the redundant top-bar `#slip-switcher` (`internal/web/page.go`, `internal/web/slips.go`, `web/templates/layout.html`, `web/static/app.js`), and update `internal/web/layout_test.go` and `internal/web/slips_test.go`.

- Added `WeekID` and `ScopeBadge` to `SlipView` in `internal/web/slips.go`, populated from `slip.WeekID` and `scopeBadge(slip.SlateScope)` in `slipView`.
- Updated `web/templates/partials.html` to render `ScopeBadge` beside the slip title and a `+ New slip` link (`/slips/new?week=...`) in both single-slip weeks (metadata row) and multi-slip weeks (at the end of `[data-sibling-switcher]`).
- Removed the redundant top-bar `#slip-switcher` from `internal/web/page.go` (`SlipOption`, `Page.Slips`), `internal/web/slips.go` (`slipOptions`), `web/templates/layout.html`, and `web/static/app.js`, and regenerated `web/static/app.css`.
- Updated `internal/web/layout_test.go` and `internal/web/slips_test.go` to verify `#slip-switcher` is absent and `ScopeBadge` and `+ New slip` render in both single-slip and multi-slip views.

### Phase 2: Create-slip form UX, Groups home summary, and docs

Tasks:

- [x] Add `HasExistingSlips` to `SlipCreateView` (`internal/web/slips.go`), update `web/templates/slip.html` so existing-slip weeks show accurate copy and *"Or add a past week"* collapses inside a `<details>` unless no weeks exist or past-week fields were submitted, and add handler tests in `internal/web/slips_test.go` and `internal/web/home_test.go`.
- [x] Update `home` in `internal/web/home.go` to pick the highest-priority actionable slip summary across all slips in the latest week (actionable step > waiting/export > closed > done, tie-breaking by earliest upcoming deadline) and add multi-slip summary tests in `internal/web/home_test.go`.
- [x] Docs: update `docs/concepts/slips.md` to document `"Thursday Slate"`, the slip hub's scope badge and `+ New slip` entry point, the single sibling pill switcher, the create-slip form's empty vs. existing-week states, and the multi-slip Groups home summary.

Phase totals: 3 checkpoints, 6m38s, $—, 3188912 input / 53041 output / 2846194 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `HasExistingSlips` to `SlipCreateView` (`internal/web/slips.go`), update `web/templates/slip.html` so existing-slip weeks show accurate copy and *"Or add a past week"* collapses inside a `<details>` unless no weeks exist or past-week fields were submitted, and add handler tests in `internal/web/slips_test.go` and `internal/web/home_test.go`.

- Added `HasExistingSlips bool` to `SlipCreateView` in `internal/web/slips.go` and updated `createView` to load `SlipsForWeek` once, set `HasExistingSlips`, and pass the slice to `nextSlipName`.
- Updated `web/templates/slip.html` so weeks with existing slips show *"Add a slip for {{.Page.WeekLabel}}"* and *"Pick a slate, legs per member, and who's in."*, while empty weeks keep the *"Start ... / No slip ... yet"* copy.
- Wrapped the *"Or add a past week"* `<fieldset>` in `web/templates/slip.html` inside a `<details data-past-week>` that opens only when `len(.Create.Weeks) == 0` or `.Create.Season != 0 || .Create.WeekNum != 0`.
- Added `TestCreateFormStates` in `internal/web/slips_test.go` and `<details data-past-week>` state assertions in `internal/web/home_test.go`.

### Task 4: Update `home` in `internal/web/home.go` to pick the highest-priority actionable slip summary across all slips in the latest week (actionable step > waiting/export > closed > done, tie-breaking by earliest upcoming deadline) and add multi-slip summary tests in `internal/web/home_test.go`.

- Replaced `groupStep` in `internal/web/home.go` with `groupWeekSummary`, `betterStep`, and `stepPriority`, loading `GamesForWeek` once in `home` and picking the highest-priority `slips.NextStep` across all slips in the latest week (`StepPick`/`StepLockIn`/`StepJoin` > `StepWaiting`/`StepExport` > `StepClosed` > `StepDone`, tie-breaking by earliest upcoming `Deadline` then `SlipsForWeek` order).
- Added `TestHomeGroupSummaryPrioritizesActionableSlips` in `internal/web/home_test.go` covering graded-vs-open, earliest-deadline tie-breaking, locked-vs-open, and export-vs-closed-vs-done ordering.

### Task 5: Docs: update `docs/concepts/slips.md` to document `"Thursday Slate"`, the slip hub's scope badge and `+ New slip` entry point, the single sibling pill switcher, the create-slip form's empty vs. existing-week states, and the multi-slip Groups home summary.

- Updated `docs/concepts/slips.md` to use the `"Thursday Slate"` scope label across the create form and multi-slip sections.
- Documented the slip hub header's scope badge, `+ New slip` entry point (in the metadata row for single-slip weeks or trailing pill for multi-slip weeks), and single horizontal sibling pill switcher (`[data-sibling-switcher]`).
- Documented the create-slip form's empty-week, existing-slip-week, no-stored-week, and collapsed `<details>` past-week states.
- Documented the multi-slip Groups home summary (`GET /`) priority and earliest-deadline tie-breaking in the `Several slips` section.
- Verified `good-vibes docs lint` reports zero errors and zero warnings for `docs/concepts/slips.md`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
