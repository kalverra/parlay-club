---
slug: dynamic-nfl-schedule-slips
run: 20261124-175506-dynamic-nfl-schedule-slips
outcome: done
generated: 2026-11-24T20:59:50Z
---

# Run summary: dynamic-nfl-schedule-slips

## Outcome

- Status: done
- Branch: dynamic-nfl-schedule-slips
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 6 of 6
- Phases: 2
- Checkpoints: 7
- Duration: 31m24s
- Cost: $—
- Fresh input tokens: 2424672
- Cache read tokens: 30431746
- Cache write tokens: 0
- Output tokens: 342933

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-high | 30413053 | $— |
| gemini-4-neon-medium | 2786298 | $— |

## Phases

### Phase 1: Schedule-derived game-day rules and per-date pre-fill

Tasks:

- [x] Replace hardcoded weekday checks in `internal/refresh/rules.go` (`gameDay`, `oddsAge`, `sundayPrefillWindow`, `sundayPrefillStale`) with scoreboard-derived `isGameDay`, `gamesOnDate`, `upcomingGamesOnDate`, `gameDayPrefillWindow`, and `gameDayPrefillStale`, with unit tests covering Thanksgiving, Black Friday, Christmas, playoff Saturdays, and non-game days.
- [x] Update `internal/refresh/refresh.go` (`maybeRunGameDayPrefill`, `RunPrefill`, `GameDayPrefillSource = "prefill:gameday"`) to trigger once per ET game date and scope automatic Parlay-only prop warming to that date's upcoming games, with integration tests verifying credit conservation and multi-day idempotency.
- [x] Docs: update `docs/concepts/data-refresh.md` and the Free-tier guardrails in `AGENTS.md` to document per-game-day pre-fill (`prefill:gameday`) and date-scoped prop warming.

Phase totals: 3 checkpoints, 14m41s, $—, 14849634 input / 177518 output / 14068258 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Replace hardcoded weekday checks in `internal/refresh/rules.go` (`gameDay`, `oddsAge`, `sundayPrefillWindow`, `sundayPrefillStale`) with scoreboard-derived `isGameDay`, `gamesOnDate`, `upcomingGamesOnDate`, `gameDayPrefillWindow`, and `gameDayPrefillStale`, with unit tests covering Thanksgiving, Black Friday, Christmas, playoff Saturdays, and non-game days.

- Replaced hardcoded weekday checks in `internal/refresh/rules.go` (`gameDay`, `oddsAge`, `sundayPrefillWindow`, `sundayPrefillStale`, `lastKickoffOn`) with scoreboard-derived `isGameDay`, `gamesOnDate`, `upcomingGamesOnDate`, `gameDayPrefillWindow`, `gameDayPrefillStale`, and `sameEasternDate`.
- Updated `scoreboardStale`, `scoreboardAge`, `oddsPickLinesAge`, `nfeloStale`, `nflverseAge`, `oddsPropsAgeFor`, `summaryAgeFor`, `rosterStale`, `gameLogStale`, and `gameDayAge` (plus callers in `internal/refresh/refresh.go`, `internal/refresh/sources.go`, and `internal/refresh/research.go`) to derive game days from stored Eastern kickoff dates, falling back to Thu/Sun/Mon only when `games` is empty.
- Renamed `oddsSundayAge`, `oddsSaturdayAge`, and `sundayPrefillStartHour` to `oddsGameDayAge`, `oddsEveAge`, and `prefillStartHour` in `internal/refresh/refresh.go` and updated `maybeRunSundayPrefill` to use `gameDayPrefillWindow` and `gameDayPrefillStale`.
- Added and updated unit tests in `internal/refresh/rules_test.go` covering Thanksgiving, Black Friday, Christmas Wednesday, playoff Saturdays, non-game days, DST transitions, and cold-DB fallbacks.

### Task 2: Update `internal/refresh/refresh.go` (`maybeRunGameDayPrefill`, `RunPrefill`, `GameDayPrefillSource = "prefill:gameday"`) to trigger once per ET game date and scope automatic Parlay-only prop warming to that date's upcoming games, with integration tests verifying credit conservation and multi-day idempotency.

- Replaced `SundayPrefillSource` (`"prefill:sunday"`) with `GameDayPrefillSource` (`"prefill:gameday"`) and renamed `maybeRunSundayPrefill` to `maybeRunGameDayPrefill` in `internal/refresh/refresh.go` to gate automatic `Kick()` pre-fill once per Eastern game date.
- Scoped `RunPrefill` Parlay-only bulk prop warming in `internal/refresh/refresh.go` to `upcomingGamesOnDate(games, now)` (excluding `db.GameFinal` games), falling back to `upcomingWeekGames(games, now)` when `upcomingGamesOnDate` is empty so manual `POST /internal/prefill` calls on non-game days retain week-wide coverage.
- Updated existing pre-fill tests in `internal/refresh/prefill_test.go` to assert `GameDayPrefillSource` (`"prefill:gameday"`).
- Added integration tests in `internal/refresh/prefill_test.go` (`TestKickMultiDayHolidayPrefillIdempotencyAndCreditConservation`, `TestKickIrregularGameDaysPrefill`, `TestRunPrefillNonGameDayFallsBackToAllUpcomingWeekGames`, and `TestKickGameDayPrefillHaltsPropsBelowCreditFloor`) verifying Thanksgiving/Black Friday/Sunday multi-day idempotency, Playoff Saturday and Christmas Wednesday pre-fill, credit conservation, fallback behavior, and credit-floor halting.

### Task 3: Docs: update `docs/concepts/data-refresh.md` and the Free-tier guardrails in `AGENTS.md` to document per-game-day pre-fill (`prefill:gameday`) and date-scoped prop warming.

- Updated `docs/concepts/data-refresh.md` to document `GameDayPrefillSource` (`"prefill:gameday"`), `maybeRunGameDayPrefill`, `gameDayPrefillWindow`, `gameDayPrefillStale`, scoreboard-derived `isGameDay` staleness rules, and `RunPrefill` prop warming scoped to `upcomingGamesOnDate` (with week-wide fallback on non-game-day manual invocations).
- Added `## Overview` (with the existing Mermaid diagram) and a `budget` front-matter override in `docs/concepts/data-refresh.md`, and trimmed `summary` to <= 240 characters so `good-vibes docs lint` passes with zero errors.
- Updated the `## Free-tier guardrails` section in `AGENTS.md` to replace Sunday-only `prefill:sunday` wording with per-game-day `prefill:gameday` pre-fill and date-scoped Parlay-only prop warming.
- Regenerated `docs/llms.txt` via `good-vibes docs index` to reflect the updated `data-refresh` summary.

### Phase 2: Schedule-aware recurring slip auto-creation and weekend slate scope

Tasks:

- [x] Add `SlateScopeWeekend` (`"weekend"`, matching Sat/Sun/Mon ET) in `internal/db/slips.go` (plus migration `0016_weekend_slate_scope.sql` if schema constraints restrict `slate_scope`) and expose it in recurring schedule forms in `internal/web/schedules.go` and `web/templates/week.html`, with tests.
- [x] Update `slips.EnsureWeekSlips` in `internal/slips/slips.go` (and callers in `internal/web/home.go`) to load week games and skip auto-creating recurring slips when the stored week has no upcoming games matching the schedule's scope, with tests covering playoff weeks and completed weeks.
- [x] Docs: update `docs/concepts/slips.md` and `docs/concepts/groups.md` for `weekend` slate scope and schedule-aware `EnsureWeekSlips` skipping.

Phase totals: 4 checkpoints, 15m8s, $—, 16418441 input / 154828 output / 14998233 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
