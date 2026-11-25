# Phase 1: Phase 1: Pre-kickoff sharp benchmarks and nflverse L5 fallback in eval replay

## Done

### Task 1: Export `odds.ParseLinesSnapshotJSON` in `internal/ingest/odds/lines.go` and `analyzer.HasSharpBenchmark` in `internal/analyzer/grade.go`, reconstruct `Game.Sharp` from pre-kickoff archived `odds:lines` and `odds:props:{gameID}` (with `BodySHA256` body memoization and pre-kickoff roster/prior-week nflverse player resolution) in `internal/eval/replay.go`, track `DataCoverage.WithSharp`, and add unit tests verifying sharp rule firing and strict pre-kickoff isolation.

- Exported `odds.ParseLinesSnapshotJSON` in `internal/ingest/odds/lines.go` (sharing `parseEvents` in `internal/ingest/odds/events.go`) to decode event metadata and priced game lines in one pass.
- Exported `analyzer.HasSharpBenchmark` in `internal/analyzer/grade.go` to report whether `Game.Sharp` has a benchmark for a leg's `SharpKey`.
- Added `DataCoverage.WithSharp` in `internal/eval/types.go` and reconstructed `Game.Sharp` in `internal/eval/replay.go` from pre-kickoff archived `odds:lines` (memoized per week by `BodySHA256`) and `odds:props:{gameID}` (with `db.BatchPlayerResolver` built from pre-kickoff archived rosters and prior-week `nflverse_player_weeks`).
- Added unit tests in `internal/ingest/odds/archive_parse_test.go`, `internal/analyzer/sharp_test.go`, and `internal/eval/replay_test.go` verifying `ParseLinesSnapshotJSON`, `HasSharpBenchmark`, sharp rule firing (`sharp_line_edge`, `sharp_price_edge`), `WithSharp` coverage, and strict pre-kickoff and prior-week isolation.

### Task 2: Add strictly prior-week `nflverse_player_weeks` L5 fallback for `PlayerStats.Games` (and position backfill) in `internal/eval/replay.go` when a player lacks a pre-kickoff archived ESPN game log, track `DataCoverage.WithL5Fallback`, and add unit tests verifying L5 hit-rate/median-trap firing and zero same-week/future leakage.

- Added `DataCoverage.WithL5Fallback` in `internal/eval/types.go` to track legs backed by prior-week nflverse game logs.
- Extended `loadPriorNFLverse` and `loadArchivedPlayerStats` in `internal/eval/replay.go` (with `buildNFLversePlayerGames`) to populate `PlayerStats.Games` (`PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`) most-recent-first from strictly prior-week `nflverse_player_weeks` rows when no pre-kickoff archived ESPN game log exists, and to backfill `PlayerStats.Position` from the latest prior-week nflverse row when pre-kickoff rosters omit the player.
- Updated `accumulateWeekCoverage` in `internal/eval/replay.go` to count `WithL5Fallback` only when a player has no pre-kickoff ESPN game log.
- Added `TestLoadDatasetNFLverseL5Fallback` in `internal/eval/replay_test.go` verifying `hit_rate` and `median_trap` rule firing, pre-kickoff ESPN game log precedence, position backfill, `WithL5Fallback` coverage accounting, and zero same-week or future leakage.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`loadPriorNFLverse` in `internal/eval/replay.go` already loads `NFLversePlayerWeeksForSeason` for prior-week player resolution; the nflverse L5 fallback task can reuse or extend that per-week query to populate `PlayerStats.Games` when no pre-kickoff `espn:gamelog:{id}` exists.

### Task 2

Market-wide replay (`ModeMarket`) can reuse `loadPriorNFLverse` and `loadArchivedPlayerStats` for both slip-based and market-wide player legs; `WithL5Fallback` is ready to be surfaced in `cmd/eval` and `/analyzer/results`.

## Files touched

- `internal/analyzer/grade.go`
- `internal/analyzer/sharp_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`
- `internal/eval/types.go`
- `internal/ingest/odds/archive_parse_test.go`
- `internal/ingest/odds/events.go`
- `internal/ingest/odds/lines.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 500ms, lint 4.7s, test 5.9s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
