# Task 1

## Done

- Replaced hardcoded weekday checks in `internal/refresh/rules.go` (`gameDay`, `oddsAge`, `sundayPrefillWindow`, `sundayPrefillStale`, `lastKickoffOn`) with scoreboard-derived `isGameDay`, `gamesOnDate`, `upcomingGamesOnDate`, `gameDayPrefillWindow`, `gameDayPrefillStale`, and `sameEasternDate`.
- Updated `scoreboardStale`, `scoreboardAge`, `oddsPickLinesAge`, `nfeloStale`, `nflverseAge`, `oddsPropsAgeFor`, `summaryAgeFor`, `rosterStale`, `gameLogStale`, and `gameDayAge` (plus callers in `internal/refresh/refresh.go`, `internal/refresh/sources.go`, and `internal/refresh/research.go`) to derive game days from stored Eastern kickoff dates, falling back to Thu/Sun/Mon only when `games` is empty.
- Renamed `oddsSundayAge`, `oddsSaturdayAge`, and `sundayPrefillStartHour` to `oddsGameDayAge`, `oddsEveAge`, and `prefillStartHour` in `internal/refresh/refresh.go` and updated `maybeRunSundayPrefill` to use `gameDayPrefillWindow` and `gameDayPrefillStale`.
- Added and updated unit tests in `internal/refresh/rules_test.go` covering Thanksgiving, Black Friday, Christmas Wednesday, playoff Saturdays, non-game days, DST transitions, and cold-DB fallbacks.

## Deviations

- `upcomingGamesOnDate` in `internal/refresh/rules.go` excludes `db.GameFinal` games alongside `!now.Before(game.Kickoff)` so already-final games with synthetic future kickoff timestamps (such as `TestKickSyncsSummariesAndUniqueRostersWithoutOddsLines`) are not treated as upcoming.
- Updated `internal/refresh/refresh.go`, `internal/refresh/sources.go`, and `internal/refresh/research.go` call sites and constants (`oddsGameDayAge`, `oddsEveAge`, `prefillStartHour`, `gameDayPrefillWindow`, `gameDayPrefillStale`, and passing `games` to `scoreboardStale`, `rosterStale`, and `gameLogStale`) so the package compiles and passes tests ahead of Task 2.

## Open issues

- `maybeRunSundayPrefill`, `SundayPrefillSource` (`"prefill:sunday"`), and `RunPrefill` week-wide prop warming in `internal/refresh/refresh.go` remain unchanged for Task 2.

## Next-phase notes

- Task 2 can build directly on `gameDayPrefillWindow`, `gameDayPrefillStale`, and `upcomingGamesOnDate` when updating `maybeRunSundayPrefill` to `maybeRunGameDayPrefill`, switching `SundayPrefillSource` to `GameDayPrefillSource = "prefill:gameday"`, and scoping `RunPrefill` prop warming to `upcomingGamesOnDate(games, now)`.
