# Task 5: Schedule-aware `slips.EnsureWeekSlips`

## Done

- Extended `slips.Store` in `internal/slips/slips.go` with `GamesForWeek` and added `now time.Time` to `slips.EnsureWeekSlips`.
- Updated `slips.EnsureWeekSlips` to load week games when any active schedules lack slips and skip auto-creating slips when stored games exist (`len(games) > 0`) but none matching the schedule's `SlateScope` are upcoming (`g.Status != db.GameFinal && now.Before(g.Kickoff)`), while preserving existing slips and cold-DB creation when `len(games) == 0`.
- Updated `groupWeek` in `internal/web/home.go` and `TestMigrationUpgradePathEnsureWeekSlipsSingleSlip` in `internal/db/migrate_test.go` to pass `now` to `slips.EnsureWeekSlips`.
- Added unit and integration tests covering playoff weekends (`thursday` skipped, `sunday_main` and `weekend` created, Saturday-only playoff week), completed weeks (no dead slips created, final games with future kickoff ignored, existing slips preserved), `SlateScopeWeekend` in `FilterGamesByScope` and `NextStep`, and `GET /groups/{id}/week` handler behavior in `internal/slips/slips_test.go`, `internal/slips/next_test.go`, and `internal/web/home_test.go`.

## Deviations

- `hasUpcomingMatchingGame` in `internal/slips/slips.go` excludes `db.GameFinal` games alongside `!now.Before(g.Kickoff)` so finished games with synthetic future kickoff timestamps are not treated as upcoming, matching `internal/refresh/rules.go`.
- Also updated `TestMigrationUpgradePathEnsureWeekSlipsSingleSlip` in `internal/db/migrate_test.go` for the added `now time.Time` parameter on `slips.EnsureWeekSlips`.

## Open issues

- None.

## Next-phase notes

- Task 6 can update `docs/concepts/slips.md` and `docs/concepts/groups.md` to document `SlateScopeWeekend` (`"weekend"` / `"Weekend Slate"`) and schedule-aware `slips.EnsureWeekSlips` skipping when no upcoming games match a schedule's scope.

## Commit

Skip recurring slips without upcoming matching games

Playoff weeks and completed weeks have no upcoming Thursday or post-Monday games, so auto-creating slips for every active recurring schedule leaves empty or dead slips on the week hub. Loading stored week games in EnsureWeekSlips lets groups keep recurring schedules active year-round without generating unusable slips.
