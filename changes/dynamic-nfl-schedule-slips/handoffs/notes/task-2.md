# Task 2

## Done

- Replaced `SundayPrefillSource` (`"prefill:sunday"`) with `GameDayPrefillSource` (`"prefill:gameday"`) and renamed `maybeRunSundayPrefill` to `maybeRunGameDayPrefill` in `internal/refresh/refresh.go` to gate automatic `Kick()` pre-fill once per Eastern game date.
- Scoped `RunPrefill` Parlay-only bulk prop warming in `internal/refresh/refresh.go` to `upcomingGamesOnDate(games, now)` (excluding `db.GameFinal` games), falling back to `upcomingWeekGames(games, now)` when `upcomingGamesOnDate` is empty so manual `POST /internal/prefill` calls on non-game days retain week-wide coverage.
- Updated existing pre-fill tests in `internal/refresh/prefill_test.go` to assert `GameDayPrefillSource` (`"prefill:gameday"`).
- Added integration tests in `internal/refresh/prefill_test.go` (`TestKickMultiDayHolidayPrefillIdempotencyAndCreditConservation`, `TestKickIrregularGameDaysPrefill`, `TestRunPrefillNonGameDayFallsBackToAllUpcomingWeekGames`, and `TestKickGameDayPrefillHaltsPropsBelowCreditFloor`) verifying Thanksgiving/Black Friday/Sunday multi-day idempotency, Playoff Saturday and Christmas Wednesday pre-fill, credit conservation, fallback behavior, and credit-floor halting.

## Deviations

- `upcomingWeekGames` in `internal/refresh/refresh.go` filters out `db.GameFinal` alongside `!now.Before(game.Kickoff)` to stay consistent with `upcomingGamesOnDate` from Task 1.

## Open issues

- `docs/concepts/data-refresh.md`, `AGENTS.md`, and other reference/decision docs still mention `prefill:sunday` / Sunday-only pre-fill and await Task 3.

## Next-phase notes

- Task 3 can update `docs/concepts/data-refresh.md` and `AGENTS.md` to document `GameDayPrefillSource = "prefill:gameday"`, `maybeRunGameDayPrefill`, and date-scoped `RunPrefill` prop warming.

## Commit

Trigger game-day prefill once per ET game date

Holiday weeks and playoff weekends feature games on non-Sunday dates, so Sunday-only pre-fill missed those slates while week-wide prop warming would multiply Parlay API credit spend across multi-day weeks. Gating pre-fill once per Eastern game date and scoping automatic bulk props to that date's upcoming games covers every NFL game day without duplicating weekly credit usage.
