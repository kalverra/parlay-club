# Phase 2: Phase 2: Schedule-aware recurring slip auto-creation and weekend slate scope

## Done

### Task 4: Add `SlateScopeWeekend` (`"weekend"`, matching Sat/Sun/Mon ET) in `internal/db/slips.go` (plus migration `0016_weekend_slate_scope.sql` if schema constraints restrict `slate_scope`) and expose it in recurring schedule forms in `internal/web/schedules.go` and `web/templates/week.html`, with tests.

- Added `SlateScopeWeekend` (`"weekend"`, labeled `"Weekend Slate"`, matching Saturday, Sunday, and Monday in Eastern time) to `SlateScope.Valid`, `SlateScope.Label`, and `SlateScope.Matches` in `internal/db/slips.go`.
- Verified in `internal/db/migrations/0009_multi_slip_events.sql` and `internal/db/migrations/0001_init.sql` that neither `slips.slate_scope` nor `group_slip_schedules.slate_scope` carries a `CHECK` constraint, so no `0016_weekend_slate_scope.sql` migration was required.
- Exposed `SlateScopeWeekend` in `defaultSlateScopeOptions` (`internal/web/slips.go`), `WeekPage.SlateScopes` (`internal/web/home.go`), the recurring schedule form (`web/templates/week.html`), and the one-off slip create form (`web/templates/slip.html`).
- Added unit and handler tests for `SlateScopeWeekend` across `internal/db/slips_test.go`, `internal/db/schedules_test.go`, `internal/web/schedules_test.go`, `internal/web/home_test.go`, and `internal/web/slips_test.go`.

### Task 5: Update `slips.EnsureWeekSlips` in `internal/slips/slips.go` (and callers in `internal/web/home.go`) to load week games and skip auto-creating recurring slips when the stored week has no upcoming games matching the schedule's scope, with tests covering playoff weeks and completed weeks.

- Extended `slips.Store` in `internal/slips/slips.go` with `GamesForWeek` and added `now time.Time` to `slips.EnsureWeekSlips`.
- Updated `slips.EnsureWeekSlips` to load week games when any active schedules lack slips and skip auto-creating slips when stored games exist (`len(games) > 0`) but none matching the schedule's `SlateScope` are upcoming (`g.Status != db.GameFinal && now.Before(g.Kickoff)`), while preserving existing slips and cold-DB creation when `len(games) == 0`.
- Updated `groupWeek` in `internal/web/home.go` and `TestMigrationUpgradePathEnsureWeekSlipsSingleSlip` in `internal/db/migrate_test.go` to pass `now` to `slips.EnsureWeekSlips`.
- Added unit and integration tests covering playoff weekends (`thursday` skipped, `sunday_main` and `weekend` created, Saturday-only playoff week), completed weeks (no dead slips created, final games with future kickoff ignored, existing slips preserved), `SlateScopeWeekend` in `FilterGamesByScope` and `NextStep`, and `GET /groups/{id}/week` handler behavior in `internal/slips/slips_test.go`, `internal/slips/next_test.go`, and `internal/web/home_test.go`.

### Task 6: Docs: update `docs/concepts/slips.md` and `docs/concepts/groups.md` for `weekend` slate scope and schedule-aware `EnsureWeekSlips` skipping.

- Updated `docs/concepts/slips.md` to document the `weekend` (`Weekend Slate`, Saturday/Sunday/Monday ET) slate scope across the overview, hub scope badges, create-slip form options, slate scope table, and multi-slip week hub, plus schedule-aware `slips.EnsureWeekSlips` skipping.
- Updated `docs/concepts/groups.md` to document `weekend` (`Weekend Slate`) in recurring slip schedules and how `EnsureWeekSlips` loads `GamesForWeek` and skips missing schedules without upcoming non-final matching games when stored games exist.
- Updated `docs/concepts/groups.md` front matter (removing legacy `sources` and `last_verified` fields) and added `## Overview` with a Mermaid diagram so `good-vibes docs lint` passes with zero errors.
- Regenerated `docs/llms.txt` via `good-vibes docs index` to include `Groups`.

## Deviations

### Task 4

- Did not modify `internal/web/schedules.go` because schedule creation and updates already delegate `slate_scope` validation directly to `db.SlateScope.Valid()`, while the recurring schedule form data is built in `internal/web/home.go` (`groupWeek`) via `defaultSlateScopeOptions` from `internal/web/slips.go`.
- Also included `SlateScopeWeekend` in `defaultSlateScopeOptions` (`internal/web/slips.go`) and `web/templates/slip.html` so one-off slip creation and recurring schedule forms share the same list of valid slate scopes.

### Task 5

- `hasUpcomingMatchingGame` in `internal/slips/slips.go` excludes `db.GameFinal` games alongside `!now.Before(g.Kickoff)` so finished games with synthetic future kickoff timestamps are not treated as upcoming, matching `internal/refresh/rules.go`.
- Also updated `TestMigrationUpgradePathEnsureWeekSlipsSingleSlip` in `internal/db/migrate_test.go` for the added `now time.Time` parameter on `slips.EnsureWeekSlips`.

### Task 6

- Removed unsupported `sources` and `last_verified` front-matter fields and converted `## Summary` to `## Overview` with a Mermaid diagram in `docs/concepts/groups.md` (and regenerated `docs/llms.txt`) because `good-vibes docs lint` treats legacy front-matter schema errors as hard failures on modified docs.

## Open issues

### Task 4

- None.

### Task 5

- None.

### Task 6

- None.

## Next-phase notes

### Task 4

- Task 5 can use `db.SlateScopeWeekend` directly when updating `slips.EnsureWeekSlips` in `internal/slips/slips.go` and `internal/web/home.go` to skip schedules without upcoming matching games.
- Task 6 should document `SlateScopeWeekend` (`"weekend"` / `"Weekend Slate"`) in `docs/concepts/slips.md` and `docs/concepts/groups.md`.

### Task 5

- Task 6 can update `docs/concepts/slips.md` and `docs/concepts/groups.md` to document `SlateScopeWeekend` (`"weekend"` / `"Weekend Slate"`) and schedule-aware `slips.EnsureWeekSlips` skipping when no upcoming games match a schedule's scope.

### Task 6

- Phase 2 is complete. Any remaining references to legacy slate scopes in `docs/reference/schema.md` or `docs/reference/routes.md` were outside this task's scope.

## Files touched

- `docs/concepts/groups.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `internal/db/migrate_test.go`
- `internal/db/schedules_test.go`
- `internal/db/slips.go`
- `internal/db/slips_test.go`
- `internal/slips/next_test.go`
- `internal/slips/slips.go`
- `internal/slips/slips_test.go`
- `internal/web/home.go`
- `internal/web/home_test.go`
- `internal/web/schedules_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/templates/slip.html`
- `web/templates/week.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: test 1.1s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
