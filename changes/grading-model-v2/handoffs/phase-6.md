# Phase 6: Phase 6: Calibration eval metrics and side-by-side v1/v2 comparison

## Done

### Task 15: Extend `internal/eval` (`metrics.go`, `replay.go`, `cache.go`) and `cmd/eval` to evaluate all registered `GradingEngine` implementations side-by-side and emit `CalibrationMetrics` (Log Loss, Brier, classwise ECE, ≥80% bin spread, market baseline, SGP joint Log Loss vs. naive independence, and reliability bins) in text, Markdown, and JSON, with unit tests on synthetic and SGP replay fixtures.

- Extended `internal/eval/metrics.go` with `ReliabilityBin`, `CalibrationMetrics`, `Report.Calibration`, and engine-level evaluation (`EvaluateEngine`, `EvaluateAllEngines`, `CompareEngines`), computing Log Loss, Brier Score, 10-bin Classwise ECE, ≥80% reliability bin spread, market baseline (from `LegGrade.MarketProb`, pre-kickoff `Sharp`, or implied decimal odds), and SGP copula joint Log Loss vs. naive independence (`Prod(p_i)`).
- Updated `internal/eval/replay.go` (`attachPreKickoffOddsSignals`) to populate `analyzer.Game.MarketScript` via `analyzer.BuildMarketScript` from pre-kickoff archived `odds:lines` in both `ModeSlips` and `ModeMarket`.
- Switched `internal/eval/cache.go` (`GetOrComputeMode`) to call `EvaluateAllEngines(dataset, analyzer.BuiltInEngines())` so cached `SuiteReport` results evaluate `v1` and `v2` side-by-side.
- Extended `cmd/eval/main.go` so `--all`, `--rubric`, `--baseline`, and `--candidate` resolve built-in `GradingEngine` implementations as well as rubrics, and formatted `CalibrationMetrics` and side-by-side engine comparison tables in text, Markdown, and JSON.
- Added unit tests in `internal/eval/metrics_test.go`, `internal/eval/replay_test.go`, `internal/eval/cache_test.go`, and `cmd/eval/main_test.go` covering synthetic 10-bin calibration, SGP vs. naive independence replay, `MarketScript` leak-free replay, cache invalidation, and CLI output formats.

### Task 16: Update `internal/web/eval_results.go` and `web/templates/analyzer_results.html` to display v1 vs. v2 engine selection, calibration summary tiles, reliability table, and SGP correlation lift cleanly at phone width, with handler and rendering tests.

- Updated `internal/web/eval_results.go` (`EvalResultsPage`, `EvalEngineOption`, `EvalEngineComparisonView`, `EvalCalibrationView`, `EvalReliabilityBinView`) to support `?engine=` (with backwards-compatible `?rubric=` fallback), map `report.Calibration` metrics, format 10-bin reliability rows, compute SGP correlation lift (`NaiveIndepLogLoss - SGPLogLoss`), and build side-by-side `v1` vs. `v2` comparison summaries.
- Updated `web/templates/analyzer_results.html` with the `Engine:` switcher (`v1` / `v2`), Probability Calibration summary tiles (Log Loss, Brier Score, Classwise ECE, ≥80% Bin Spread, Market Log Loss, Market Brier), SGP Correlation Lift panel, phone-first Reliability Table (`sm:hidden` compact bin rows + `sm:block` table), and side-by-side Engine Comparison cards/table.
- Added handler and rendering tests in `internal/web/eval_results_test.go` (`TestEvalResultsEngineSelectionAndCalibration`, `TestAnalyzerResultsTemplateRendersCalibrationAndPhoneWidthLayout`) covering `v1` vs. `v2` selection, `?rubric=` fallback, market-mode SGP hiding, calibration tiles, reliability bins, SGP correlation lift, and phone-width markup.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 15

- Task 16 (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`) already receives both `v1` and `v2` reports from `eval.Cache` (`suite.Reports`) and can bind `report.Calibration` (`LogLoss`, `BrierScore`, `ClasswiseECE`, `PopulatedBins`, `TotalBins`, `BinSpreadOK`, `MarketLogLoss`, `MarketBrier`, `SGPLogLoss`, `NaiveIndepLogLoss`, `SGPCount`, `Reliability`) directly into the calibration summary tiles, reliability table, and SGP lift view.

### Task 16

- Phase 7 (`internal/web/analyzer.go`, `web/templates/partials.html`, `web/templates/analyzer_guide.html`, and `docs/reference/routes.md`) should document `?engine=` alongside `?rubric=` on `GET /analyzer/results` when updating living docs after v2 promotion.

## Files touched

- `cmd/eval/main.go`
- `cmd/eval/main_test.go`
- `internal/eval/cache.go`
- `internal/eval/cache_test.go`
- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`
- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `web/templates/analyzer_results.html`

## Run log

- Mode: 
- Checkpoints: task-15, task-16
- Verify: verify passed: generate 1.1s, lint reused, changed: internal/web/eval_results_test.go, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
