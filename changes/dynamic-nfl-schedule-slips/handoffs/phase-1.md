# Phase 1: Phase 1: Schedule-derived game-day rules and per-date pre-fill

## Done

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

## Deviations

### Task 1

- `upcomingGamesOnDate` in `internal/refresh/rules.go` excludes `db.GameFinal` games alongside `!now.Before(game.Kickoff)` so already-final games with synthetic future kickoff timestamps (such as `TestKickSyncsSummariesAndUniqueRostersWithoutOddsLines`) are not treated as upcoming.
- Updated `internal/refresh/refresh.go`, `internal/refresh/sources.go`, and `internal/refresh/research.go` call sites and constants (`oddsGameDayAge`, `oddsEveAge`, `prefillStartHour`, `gameDayPrefillWindow`, `gameDayPrefillStale`, and passing `games` to `scoreboardStale`, `rosterStale`, and `gameLogStale`) so the package compiles and passes tests ahead of Task 2.

### Task 2

- `upcomingWeekGames` in `internal/refresh/refresh.go` filters out `db.GameFinal` alongside `!now.Before(game.Kickoff)` to stay consistent with `upcomingGamesOnDate` from Task 1.

### Task 3

- None.

## Open issues

### Task 1

- `maybeRunSundayPrefill`, `SundayPrefillSource` (`"prefill:sunday"`), and `RunPrefill` week-wide prop warming in `internal/refresh/refresh.go` remain unchanged for Task 2.

### Task 2

- `docs/concepts/data-refresh.md`, `AGENTS.md`, and other reference/decision docs still mention `prefill:sunday` / Sunday-only pre-fill and await Task 3.

### Task 3

- Reference, guide, and runbook docs outside this task's scope (`docs/reference/data-sources.md`, `docs/reference/routes.md`, `docs/guides/gcp-setup.md`, and `docs/runbooks/deploy.md`) still contain legacy `prefill:sunday` / Sunday-only pre-fill wording.

## Next-phase notes

### Task 1

- Task 2 can build directly on `gameDayPrefillWindow`, `gameDayPrefillStale`, and `upcomingGamesOnDate` when updating `maybeRunSundayPrefill` to `maybeRunGameDayPrefill`, switching `SundayPrefillSource` to `GameDayPrefillSource = "prefill:gameday"`, and scoping `RunPrefill` prop warming to `upcomingGamesOnDate(games, now)`.

### Task 2

- Task 3 can update `docs/concepts/data-refresh.md` and `AGENTS.md` to document `GameDayPrefillSource = "prefill:gameday"`, `maybeRunGameDayPrefill`, and date-scoped `RunPrefill` prop warming.

### Task 3

- Phase 2 only touches slip schedules (`Weekend` slate scope and schedule-aware `EnsureWeekSlips` in `docs/concepts/slips.md` and `docs/concepts/groups.md`), which is independent of `internal/refresh` pre-fill docs.

## Files touched

- `AGENTS.md`
- `docs/concepts/data-refresh.md`
- `docs/llms.txt`
- `internal/refresh/prefill_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/research.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`
- `internal/refresh/sources.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 2.8s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 2
