# Phase 2: Phase 2: Dual-track evals — Explaining Vegas vs. Beating Vegas

## Done

### Task 4: Add `MarketExplanationMetrics` (MAE, RMSE, KL divergence, Pearson r, within-5pp rate on `RawModelProb` vs. `MarketProb`) and `MarketBeatingMetrics` (raw vs. blended vs. market log loss, skill scores, ROI, and `AlphaGatePassed`) plus `SuiteReport.SlipGrades` in `internal/eval`, with unit tests verifying zero-error matching, missing-benchmark skipping, and alpha-gate thresholds.

- Added `MarketExplanationMetrics` and `MarketExplanation` on `eval.Report` in `internal/eval/metrics.go` to compute MAE, RMSE, Bernoulli KL divergence, Pearson correlation `r`, and within-5pp rate for `RawModelProb` vs. `MarketProb`, skipping legs without a de-vigged sharp benchmark or model probability.
- Added `MarketBeatingMetrics` and `MarketBeating` on `eval.Report` to compare `RawModelLogLoss`, `BlendedLogLoss`, and `MarketLogLoss`, compute `LogLossSkill`, `BrierSkill`, and `FlatROI`, and evaluate `AlphaGatePassed` (`SampleCount >= 250 && LogLossSkill > 0 && FlatROI > 0`).
- Added `slipOutcome` tracking to `evaluateWithEngine` and `SuiteReport.SlipGrades(engineName string)` with the same `ActiveRubric` and `Reports[0]` fallback behavior as `LegGrades`.
- Added unit tests in `internal/eval/metrics_test.go` covering zero-error matching, missing-benchmark skipping, alpha-gate sample/skill/ROI thresholds, and `SuiteReport.SlipGrades`.

### Task 5: Surface both "1. Explaining Vegas (Match First)" and "2. Beating Vegas (Alpha Gate)" scorecards in `cmd/eval` output and on `/docs/evals` (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`), with handler and render tests.

- Added "1. Explaining Vegas (Match First)" (`writeMarketExplanationText`, `writeMarketExplanationMarkdown`) and "2. Beating Vegas (Alpha Gate)" (`writeMarketBeatingText`, `writeMarketBeatingMarkdown`) scorecards to `cmd/eval/main.go` text and markdown outputs (JSON already serializes `Report.MarketExplanation` and `Report.MarketBeating`).
- Added `EvalMarketExplanationView`, `EvalMarketBeatingView`, `buildEvalMarketExplanation`, and `buildEvalMarketBeating` to `internal/web/eval_results.go`.
- Rendered both dual-track scorecards in a phone-first responsive grid in `web/templates/analyzer_results.html` with `Alpha Gate: PASS` / `Alpha Gate: FAIL` status badges and skill-color indicators.
- Added CLI output tests in `cmd/eval/main_test.go` (`TestWriteReportDualTrackScorecards`) and handler/render tests in `internal/web/eval_results_test.go` covering `/docs/evals` and `analyzer-results` rendering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

- `Report.MarketExplanation` and `Report.MarketBeating` are populated by `Evaluate` and `EvaluateEngine` and ready for `cmd/eval` CLI output and `internal/web/eval_results.go` rendering in Task 5.
- `SuiteReport.SlipGrades` is ready for `/record` and `/slips/{id}` settled-slip Process vs. Outcome badges in Phase 3.

### Task 5

- Phase 2 dual-track evals are complete; Phase 3 can proceed with theme color token updates, 3-layer progressive disclosure UI, and settled-slip Process vs. Outcome badges.

## Files touched

- `cmd/eval/main.go`
- `cmd/eval/main_test.go`
- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `web/static/app.css`
- `web/templates/analyzer_results.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: generate changed: web/static/app.css, lint reused, test 10.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
