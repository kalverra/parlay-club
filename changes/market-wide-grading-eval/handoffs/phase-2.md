# Phase 2: Phase 2: Market-wide candidate builder, deterministic grader, and evaluator

## Done

### Task 3: Export `espn.ParseBoxScoreJSON` in `internal/ingest/espn/boxscore.go` and `Store.LatestAPIResponse` in `internal/db/archive_read.go`, then implement the market-wide candidate builder (`internal/eval/market.go`) using `board.Build` median deduplication and deterministic offline grader (`internal/eval/market_grade.go`) over final scores, completed archived ESPN box scores, and same-week `nflverse_player_weeks` with unit tests covering all markets and missing-data guards.

- Exported `espn.ParseBoxScoreJSON` in `internal/ingest/espn/boxscore.go` and `Store.LatestAPIResponse` in `internal/db/archive_read.go`, with unit tests in `internal/ingest/espn/boxscore_test.go` and `internal/db/archive_test.go`.
- Implemented `BuildMarketCandidates` in `internal/eval/market.go` using `board.Build` (`homeBook == ""`, `now == time.Time{}`) for median line and median price deduplication, dropping non-`FINAL` games, `OTHER`/unsupported markets, zero/invalid odds, and missing lines on line-based markets.
- Implemented `MarketGrader` (`NewMarketGrader`, `GradeLeg`, `GradeCandidates`) in `internal/eval/market_grade.go` grading game lines and `TEAM_TOTAL` from final scores, and player props from `game.BoxScoreJSON`, completed archived `espn:summary:{gameID}` responses, or same-week `nflverse_player_weeks` fallback (dropping unmatched players and `Pending`/`NeedsGrading` results).
- Added unit tests in `internal/eval/market_test.go` covering multi-book median deduplication, all markets, ESPN box score DNP voids vs. nflverse unmatched drops, and missing-data/cross-week guards.

### Task 4: Add `ReplayMode` (`ModeSlips` / `ModeMarket`) to `ReplayFilter`, `MarketLegs` to `ReplayedWeek`, and market-leg scoring and rubric migration matching in `internal/eval/metrics.go` and `internal/eval/replay.go`, with end-to-end replay and comparison tests verifying calibration, lift, flat ROI, Brier score, factor attribution, and empty slip sections.

- Added `ReplayMode` (`ModeSlips`, `ModeMarket`) to `ReplayFilter` and `MarketLegs` (`market_legs`) to `ReplayedWeek` in `internal/eval/types.go` and `internal/eval/replay.go`.
- Updated `Evaluate` and `legOutcomeKey` in `internal/eval/metrics.go` to score `MarketLegs` with `analyzer.ScoreLegWithRubric`, compute leg calibration, lift, flat ROI, Brier score, factor attribution, and cross-week rubric migrations while leaving `SlipBands` and `SlipFactors` empty.
- Wired `ModeMarket` into `LoadDataset` in `internal/eval/replay.go` using `BuildMarketCandidates` and `NewMarketGrader`, reconstructing pre-kickoff game signals, sharp benchmarks, and archived/nflverse L5 player stats without loading user slips, and updated `LoadDatasetJSON` to count `MarketLegs`.
- Added unit and end-to-end tests (`TestEvaluateMarketLegsAndMigrations` in `internal/eval/metrics_test.go` and `TestLoadDatasetMarketModeEndToEnd` in `internal/eval/replay_test.go`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

Wire `BuildMarketCandidates` and `NewMarketGrader` / `GradeCandidates` into `ModeMarket` replay (`internal/eval/replay.go`, `internal/eval/types.go`, and `internal/eval/metrics.go`).

### Task 4

`eval.LoadDataset` now supports `ReplayFilter{Mode: eval.ModeMarket}` and `eval.Evaluate` / `eval.EvaluateAll` handle `MarketLegs` with empty `SlipBands` and `SlipFactors`. Phase 3 can wire `ModeMarket` into `eval.Cache`, `cmd/eval`, and `GET /analyzer/results?source=market`.

## Files touched

- `internal/db/archive_read.go`
- `internal/db/archive_test.go`
- `internal/eval/market.go`
- `internal/eval/market_grade.go`
- `internal/eval/market_test.go`
- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`
- `internal/eval/types.go`
- `internal/ingest/espn/boxscore.go`
- `internal/ingest/espn/boxscore_test.go`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: generate 500ms, lint reused, changed: internal/eval/replay_test.go, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
