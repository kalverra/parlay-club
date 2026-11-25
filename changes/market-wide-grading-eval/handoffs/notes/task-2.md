# Task 2

## Done

- Added `DataCoverage.WithL5Fallback` in `internal/eval/types.go` to track legs backed by prior-week nflverse game logs.
- Extended `loadPriorNFLverse` and `loadArchivedPlayerStats` in `internal/eval/replay.go` (with `buildNFLversePlayerGames`) to populate `PlayerStats.Games` (`PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`) most-recent-first from strictly prior-week `nflverse_player_weeks` rows when no pre-kickoff archived ESPN game log exists, and to backfill `PlayerStats.Position` from the latest prior-week nflverse row when pre-kickoff rosters omit the player.
- Updated `accumulateWeekCoverage` in `internal/eval/replay.go` to count `WithL5Fallback` only when a player has no pre-kickoff ESPN game log.
- Added `TestLoadDatasetNFLverseL5Fallback` in `internal/eval/replay_test.go` verifying `hit_rate` and `median_trap` rule firing, pre-kickoff ESPN game log precedence, position backfill, `WithL5Fallback` coverage accounting, and zero same-week or future leakage.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Market-wide replay (`ModeMarket`) can reuse `loadPriorNFLverse` and `loadArchivedPlayerStats` for both slip-based and market-wide player legs; `WithL5Fallback` is ready to be surfaced in `cmd/eval` and `/analyzer/results`.

## Commit

Add nflverse L5 fallback to eval replay

Historical replay previously left L5 hit-rate and median-trap rules silent whenever a player lacked a pre-kickoff archived ESPN game log. Backfilling `PlayerStats.Games` and `Position` from strictly prior-week nflverse rows restores those signals without introducing same-week or future leakage.
