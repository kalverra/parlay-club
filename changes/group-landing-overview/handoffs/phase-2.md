# Phase 2: Phase 2: Group landing page template and route migration

## Done

### Task 3: Update `groupWeek` in `internal/web/home.go` and redesign `web/templates/week.html` to always render the group overview with the conditional active/upcoming slip spotlight, quiet-state CTA, record snapshot, recent slips, recurring schedules (linking to `/slips/new?mode=recurring`), and members strip; update `internal/web/home_test.go` for active, multi-slip, quiet, and non-member cases.

- Updated `groupWeek` in `internal/web/home.go` (with `overviewRecordSeason`) to always assemble and render `GroupOverviewPage` via `buildGroupOverviewPage`, removing the single-slip redirect, inline create-slip branch, and obsolete `WeekPage`/`WeekSlipCard` types.
- Redesigned `web/templates/week.html` as a phone-first group overview with the group header (`+ New slip`), conditional active/upcoming spotlight cards or quiet-state CTA (`+ Start a slip`), 3-metric group record snapshot linking to `/record`, recent slips list, recurring schedules with HTMX controls and `/slips/new?mode=recurring` link, and members strip linking to `/groups`.
- Updated `internal/web/home_test.go` to cover single-slip spotlight, multi-slip spotlight deadline ordering, auto-created scheduled slips, quiet-state with and without an upcoming week, recent slips/record snapshot rendering, and non-member 404 enforcement.
- Verified with `go test ./internal/web/...` (passed) and `golangci-lint run ./internal/web/...` (0 issues).

### Task 4: Docs: update `docs/concepts/groups.md` and `docs/concepts/slips.md` to describe the per-group overview landing page and spotlight behavior.

- Updated `docs/concepts/groups.md` (`## The active group` and `## Screens`) to document that `/groups/{id}/week` always renders the per-group overview landing page with spotlight slips or the quiet state, record snapshot, recent slips, recurring schedules, and members strip.
- Updated `docs/concepts/slips.md` (`## Creating a slip` and `## Several slips`) to document `UpcomingWeeks`/`LatestWeek` resolution, `isSpotlightSlip` eligibility and urgency/deadline sorting, the quiet-state hero, and the overview sections below it.
- Verified with `good-vibes docs lint` (0 errors).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

`GET /groups/{id}/week` now consistently renders `GroupOverviewPage` across all states (`0`, `1`, or `2+` slips); remaining work in the change can focus on any follow-up verification or docs updates.

### Task 4

Documentation in `docs/concepts/groups.md` and `docs/concepts/slips.md` now matches the per-group overview landing page implemented on `GET /groups/{id}/week`.

## Files touched

- `docs/concepts/groups.md`
- `docs/concepts/slips.md`
- `internal/web/home.go`
- `internal/web/home_test.go`
- `web/static/app.css`
- `web/templates/week.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
