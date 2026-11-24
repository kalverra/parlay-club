# Phase 2: Phase 2: Create-slip form UX, Groups home summary, and docs

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Files touched

- `docs/concepts/slips.md`
- `internal/web/home.go`
- `internal/web/home_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/templates/slip.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: test 600ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
