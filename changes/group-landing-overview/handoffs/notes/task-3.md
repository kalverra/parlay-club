# Task 3

## Done

- Updated `groupWeek` in `internal/web/home.go` (with `overviewRecordSeason`) to always assemble and render `GroupOverviewPage` via `buildGroupOverviewPage`, removing the single-slip redirect, inline create-slip branch, and obsolete `WeekPage`/`WeekSlipCard` types.
- Redesigned `web/templates/week.html` as a phone-first group overview with the group header (`+ New slip`), conditional active/upcoming spotlight cards or quiet-state CTA (`+ Start a slip`), 3-metric group record snapshot linking to `/record`, recent slips list, recurring schedules with HTMX controls and `/slips/new?mode=recurring` link, and members strip linking to `/groups`.
- Updated `internal/web/home_test.go` to cover single-slip spotlight, multi-slip spotlight deadline ordering, auto-created scheduled slips, quiet-state with and without an upcoming week, recent slips/record snapshot rendering, and non-member 404 enforcement.
- Verified with `go test ./internal/web/...` (passed) and `golangci-lint run ./internal/web/...` (0 issues).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`GET /groups/{id}/week` now consistently renders `GroupOverviewPage` across all states (`0`, `1`, or `2+` slips); remaining work in the change can focus on any follow-up verification or docs updates.
