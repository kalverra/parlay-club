# Phase 1: Phase 1: Scope label, slip hub entry point, and single switcher

## Done

### Task 1: Rename `db.SlateScopeThursday.Label()` to `"Thursday Slate"` in `internal/db/slips.go`, update static option labels in `web/templates/slip.html` and `web/templates/week.html`, and update corresponding tests in `internal/db/slips_test.go`, `internal/web/home_test.go`, and `internal/web/slips_test.go`.

- Updated `db.SlateScopeThursday.Label()` in `internal/db/slips.go` from `"Thursday Night"` to `"Thursday Slate"`.
- Updated the static `<option value="thursday">` text in `web/templates/slip.html` and `web/templates/week.html` to `"Thursday Slate"`, and updated the schedule name placeholder in `web/templates/week.html` to `"e.g. Thursday Slip"`.
- Updated `TestSlateScopeLabel` in `internal/db/slips_test.go`, `TestCreateSlipFormSlateScopeSelection` in `internal/web/slips_test.go`, and added `"Thursday Slate"` scope badge assertions to `TestGroupWeekRendersMultipleSlips` and `TestGroupWeekAutoCreatesMultipleScheduledSlipsAndRendersView` in `internal/web/home_test.go`.

### Task 2: Add `WeekID` and `ScopeBadge` to `SlipView` (`internal/web/slips.go`), display `ScopeBadge` and a `+ New slip` link (`/slips/new?week=...`) in `web/templates/partials.html` for both single-slip and multi-slip weeks, remove the redundant top-bar `#slip-switcher` (`internal/web/page.go`, `internal/web/slips.go`, `web/templates/layout.html`, `web/static/app.js`), and update `internal/web/layout_test.go` and `internal/web/slips_test.go`.

- Added `WeekID` and `ScopeBadge` to `SlipView` in `internal/web/slips.go`, populated from `slip.WeekID` and `scopeBadge(slip.SlateScope)` in `slipView`.
- Updated `web/templates/partials.html` to render `ScopeBadge` beside the slip title and a `+ New slip` link (`/slips/new?week=...`) in both single-slip weeks (metadata row) and multi-slip weeks (at the end of `[data-sibling-switcher]`).
- Removed the redundant top-bar `#slip-switcher` from `internal/web/page.go` (`SlipOption`, `Page.Slips`), `internal/web/slips.go` (`slipOptions`), `web/templates/layout.html`, and `web/static/app.js`, and regenerated `web/static/app.css`.
- Updated `internal/web/layout_test.go` and `internal/web/slips_test.go` to verify `#slip-switcher` is absent and `ScopeBadge` and `+ New slip` render in both single-slip and multi-slip views.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `internal/web/home_test.go` and `internal/web/slips_test.go` still use `"Thursday Night"` as test slip/schedule *names* in existing fixtures; those are user-facing slip names, not scope labels.

### Task 2

- `SlipView.WeekID` and `ScopeBadge` are now available on `/slips/{id}` and `/slips/{id}/panel` partials.

## Files touched

- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/web/home_test.go`
- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/layout.html`
- `web/templates/partials.html`
- `web/templates/slip.html`
- `web/templates/week.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate reused, changed: web/static/app.css, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
