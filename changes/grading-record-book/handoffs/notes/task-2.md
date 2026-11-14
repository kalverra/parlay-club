# Task 2: `internal/grader` pure auto-grader

## Done

- Created `internal/grader/grader.go` with `LegInput`, `GameScore`, `IsPlayerMarket`, and `GradeLeg` implementing all 13 markets from `plan/product.md` (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, `PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`, `OTHER`) with no DB or HTTP dependencies.
- Implemented player lookup by `PlayerID` first, then normalized `PlayerName` (stripping punctuation and generational suffixes, narrowed by `Team` when set), distinguishing `HasStats == false` (`NEEDS_GRADING`) from absent players in populated box scores (`VOID` / DNP) and active players with `0` in unlogged categories.
- Added table-driven unit tests in `internal/grader/grader_test.go` covering wins, losses, pushes, DNPs, missing stats, kicking fallback (`FieldGoalsMade*3 + ExtraPointsMade`), `ANYTIME_TD` (`YES`/`NO`, excluding `PassingTDs`), and `IsPlayerMarket`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `grader.IsPlayerMarket(market)` can be used by `internal/refresh` to skip `ESPN.BoxScore` fetches on `FINAL` games whose `PENDING` legs are all game/team markets (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, `OTHER`).
- `GameScore.Final` must be set to `true` (e.g. `game.Status == db.GameFinal`); non-final `GameScore` values return `parlay.Pending`.
