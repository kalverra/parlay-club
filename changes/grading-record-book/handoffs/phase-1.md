# Phase 1: Phase 1: ESPN BoxScore parser and pure auto-grader

## Done

### Task 1: Add `Client.BoxScore` (`internal/ingest/espn`) parsing `boxscore.players[]` by key name and handling DNP vs empty box scores, alongside fixture tests using `summary_week9_featured.json` and `summary_empty.json`

- Added `BoxScore`, `PlayerBoxStats`, `PassingStats`, `RushingStats`, `ReceivingStats`, `KickingStats`, and `(*Client).BoxScore(ctx, eventID)` in `internal/ingest/espn/boxscore.go`, parsing `boxscore.players[]` by `keys[]` name and merging categories per player.
- Updated `parseMadeAttempts` in `internal/ingest/espn/gamelog.go` to split on both `/` (`2/2`) and `-` (`2-2`), and added `FieldGoalsMade*3 + ExtraPointsMade` fallback when `totalKickingPoints` is omitted.
- Added fixture and unit tests in `internal/ingest/espn/boxscore_test.go` covering `summary_week9_featured.json`, `summary_empty.json`, `summary.json`, reordered keys, kicking fallback, `OtherTDs`, and HTTP errors.

### Task 2: Create `internal/grader` implementing all 13 markets in `plan/product.md` (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, props, `ANYTIME_TD`, `KICKER_PTS`, `OTHER`) with table-driven tests for wins, losses, pushes, DNPs, and missing stats

- Created `internal/grader/grader.go` with `LegInput`, `GameScore`, `IsPlayerMarket`, and `GradeLeg` implementing all 13 markets from `plan/product.md` (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, `PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`, `OTHER`) with no DB or HTTP dependencies.
- Implemented player lookup by `PlayerID` first, then normalized `PlayerName` (stripping punctuation and generational suffixes, narrowed by `Team` when set), distinguishing `HasStats == false` (`NEEDS_GRADING`) from absent players in populated box scores (`VOID` / DNP) and active players with `0` in unlogged categories.
- Added table-driven unit tests in `internal/grader/grader_test.go` covering wins, losses, pushes, DNPs, missing stats, kicking fallback (`FieldGoalsMade*3 + ExtraPointsMade`), `ANYTIME_TD` (`YES`/`NO`, excluding `PassingTDs`), and `IsPlayerMarket`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `BoxScore.HasStats` is `true` when `len(boxscore.players) > 0` and `false` for empty or pregame summaries; `grader` should return `NEEDS_GRADING` when `HasStats == false` and `VOID` (DNP) when `HasStats == true` and the player is absent from `BoxScore.Players`.
- Players who appear in any stat group (including `defensive`, `fumbles`, or `punting`) are present in `BoxScore.Players` with `0` for all unlogged categories.

### Task 2

- `grader.IsPlayerMarket(market)` can be used by `internal/refresh` to skip `ESPN.BoxScore` fetches on `FINAL` games whose `PENDING` legs are all game/team markets (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, `OTHER`).
- `GameScore.Final` must be set to `true` (e.g. `game.Status == db.GameFinal`); non-final `GameScore` values return `parlay.Pending`.

## Files touched

- `internal/grader/grader.go`
- `internal/grader/grader_test.go`
- `internal/ingest/espn/boxscore.go`
- `internal/ingest/espn/boxscore_test.go`
- `internal/ingest/espn/gamelog.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
