## Done

- Added `ReplayMode` (`ModeSlips`, `ModeMarket`) to `ReplayFilter` and `MarketLegs` (`market_legs`) to `ReplayedWeek` in `internal/eval/types.go` and `internal/eval/replay.go`.
- Updated `Evaluate` and `legOutcomeKey` in `internal/eval/metrics.go` to score `MarketLegs` with `analyzer.ScoreLegWithRubric`, compute leg calibration, lift, flat ROI, Brier score, factor attribution, and cross-week rubric migrations while leaving `SlipBands` and `SlipFactors` empty.
- Wired `ModeMarket` into `LoadDataset` in `internal/eval/replay.go` using `BuildMarketCandidates` and `NewMarketGrader`, reconstructing pre-kickoff game signals, sharp benchmarks, and archived/nflverse L5 player stats without loading user slips, and updated `LoadDatasetJSON` to count `MarketLegs`.
- Added unit and end-to-end tests (`TestEvaluateMarketLegsAndMigrations` in `internal/eval/metrics_test.go` and `TestLoadDatasetMarketModeEndToEnd` in `internal/eval/replay_test.go`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`eval.LoadDataset` now supports `ReplayFilter{Mode: eval.ModeMarket}` and `eval.Evaluate` / `eval.EvaluateAll` handle `MarketLegs` with empty `SlipBands` and `SlipFactors`. Phase 3 can wire `ModeMarket` into `eval.Cache`, `cmd/eval`, and `GET /analyzer/results?source=market`.

## Commit

Add market-wide replay mode and leg evaluation

Replaying all deduplicated pre-kickoff market candidates alongside user slips lets rubric calibration, lift, flat ROI, Brier score, and band migrations be measured across the full NFL slate without mixing synthetic single-leg markets into parlay-level slip metrics.
