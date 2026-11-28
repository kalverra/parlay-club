# Task 4

## Done

- Added `MarketExplanationMetrics` and `MarketExplanation` on `eval.Report` in `internal/eval/metrics.go` to compute MAE, RMSE, Bernoulli KL divergence, Pearson correlation `r`, and within-5pp rate for `RawModelProb` vs. `MarketProb`, skipping legs without a de-vigged sharp benchmark or model probability.
- Added `MarketBeatingMetrics` and `MarketBeating` on `eval.Report` to compare `RawModelLogLoss`, `BlendedLogLoss`, and `MarketLogLoss`, compute `LogLossSkill`, `BrierSkill`, and `FlatROI`, and evaluate `AlphaGatePassed` (`SampleCount >= 250 && LogLossSkill > 0 && FlatROI > 0`).
- Added `slipOutcome` tracking to `evaluateWithEngine` and `SuiteReport.SlipGrades(engineName string)` with the same `ActiveRubric` and `Reports[0]` fallback behavior as `LegGrades`.
- Added unit tests in `internal/eval/metrics_test.go` covering zero-error matching, missing-benchmark skipping, alpha-gate sample/skill/ROI thresholds, and `SuiteReport.SlipGrades`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Report.MarketExplanation` and `Report.MarketBeating` are populated by `Evaluate` and `EvaluateEngine` and ready for `cmd/eval` CLI output and `internal/web/eval_results.go` rendering in Task 5.
- `SuiteReport.SlipGrades` is ready for `/record` and `/slips/{id}` settled-slip Process vs. Outcome badges in Phase 3.

## Commit

Add dual-track eval metrics and SlipGrades lookup

Split internal/eval reporting into Explaining Vegas (raw model vs. sharp market) and Beating Vegas (skill scores, ROI, and alpha gate) scorecards, and expose SuiteReport.SlipGrades for settled-slip process badges.
