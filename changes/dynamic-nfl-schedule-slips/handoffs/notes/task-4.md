# Task 4

## Done

- Added `SlateScopeWeekend` (`"weekend"`, labeled `"Weekend Slate"`, matching Saturday, Sunday, and Monday in Eastern time) to `SlateScope.Valid`, `SlateScope.Label`, and `SlateScope.Matches` in `internal/db/slips.go`.
- Verified in `internal/db/migrations/0009_multi_slip_events.sql` and `internal/db/migrations/0001_init.sql` that neither `slips.slate_scope` nor `group_slip_schedules.slate_scope` carries a `CHECK` constraint, so no `0016_weekend_slate_scope.sql` migration was required.
- Exposed `SlateScopeWeekend` in `defaultSlateScopeOptions` (`internal/web/slips.go`), `WeekPage.SlateScopes` (`internal/web/home.go`), the recurring schedule form (`web/templates/week.html`), and the one-off slip create form (`web/templates/slip.html`).
- Added unit and handler tests for `SlateScopeWeekend` across `internal/db/slips_test.go`, `internal/db/schedules_test.go`, `internal/web/schedules_test.go`, `internal/web/home_test.go`, and `internal/web/slips_test.go`.

## Deviations

- Did not modify `internal/web/schedules.go` because schedule creation and updates already delegate `slate_scope` validation directly to `db.SlateScope.Valid()`, while the recurring schedule form data is built in `internal/web/home.go` (`groupWeek`) via `defaultSlateScopeOptions` from `internal/web/slips.go`.
- Also included `SlateScopeWeekend` in `defaultSlateScopeOptions` (`internal/web/slips.go`) and `web/templates/slip.html` so one-off slip creation and recurring schedule forms share the same list of valid slate scopes.

## Open issues

- None.

## Next-phase notes

- Task 5 can use `db.SlateScopeWeekend` directly when updating `slips.EnsureWeekSlips` in `internal/slips/slips.go` and `internal/web/home.go` to skip schedules without upcoming matching games.
- Task 6 should document `SlateScopeWeekend` (`"weekend"` / `"Weekend Slate"`) in `docs/concepts/slips.md` and `docs/concepts/groups.md`.

## Commit

Add weekend slate scope to slips and schedule forms

Late-season Saturday doubleheaders and NFL playoff weekends span Saturday through Monday without Thursday games, so existing Sunday-only and full-week scopes either excluded Saturday kickoffs or opened midweek games. Adding the weekend scope and exposing it in schedule and slip forms gives groups a dedicated Saturday-through-Monday option.
