# Task 15

## Done

- Extended `internal/eval/metrics.go` with `ReliabilityBin`, `CalibrationMetrics`, `Report.Calibration`, and engine-level evaluation (`EvaluateEngine`, `EvaluateAllEngines`, `CompareEngines`), computing Log Loss, Brier Score, 10-bin Classwise ECE, ≥80% reliability bin spread, market baseline (from `LegGrade.MarketProb`, pre-kickoff `Sharp`, or implied decimal odds), and SGP copula joint Log Loss vs. naive independence (`Prod(p_i)`).
- Updated `internal/eval/replay.go` (`attachPreKickoffOddsSignals`) to populate `analyzer.Game.MarketScript` via `analyzer.BuildMarketScript` from pre-kickoff archived `odds:lines` in both `ModeSlips` and `ModeMarket`.
- Switched `internal/eval/cache.go` (`GetOrComputeMode`) to call `EvaluateAllEngines(dataset, analyzer.BuiltInEngines())` so cached `SuiteReport` results evaluate `v1` and `v2` side-by-side.
- Extended `cmd/eval/main.go` so `--all`, `--rubric`, `--baseline`, and `--candidate` resolve built-in `GradingEngine` implementations as well as rubrics, and formatted `CalibrationMetrics` and side-by-side engine comparison tables in text, Markdown, and JSON.
- Added unit tests in `internal/eval/metrics_test.go`, `internal/eval/replay_test.go`, `internal/eval/cache_test.go`, and `cmd/eval/main_test.go` covering synthetic 10-bin calibration, SGP vs. naive independence replay, `MarketScript` leak-free replay, cache invalidation, and CLI output formats.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 16 (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`) already receives both `v1` and `v2` reports from `eval.Cache` (`suite.Reports`) and can bind `report.Calibration` (`LogLoss`, `BrierScore`, `ClasswiseECE`, `PopulatedBins`, `TotalBins`, `BinSpreadOK`, `MarketLogLoss`, `MarketBrier`, `SGPLogLoss`, `NaiveIndepLogLoss`, `SGPCount`, `Reliability`) directly into the calibration summary tiles, reliability table, and SGP lift view.

## Commit

Add CalibrationMetrics and engine eval to eval/CLI

Evaluate all registered GradingEngine implementations side-by-side in internal/eval, eval.Cache, and cmd/eval with proper scoring rules, reliability bins, market baselines, and SGP copula-vs-independence metrics. This lets offline CLI runs and the upcoming /analyzer/results UI verify EngineV2 calibration and correlation lift against EngineV1 before v2 promotion.
