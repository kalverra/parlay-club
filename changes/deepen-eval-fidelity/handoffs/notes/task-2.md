# Task 2

## Done

- Verified `rubricFactorLabels` in `internal/eval/metrics.go` uses `analyzer.GradeRubricFor(rubric)` so factor labels match the evaluated rubric.
- Added `SlipFactors []FactorStats` (`json:"slip_factors,omitempty"`) to `eval.Report` and accumulated `slipGrade.Factors` on decided slips (`parlay.Won` / `parlay.Lost`) using shared `recordFactors` and `finalizeFactorStats` helpers.
- Keyed `legOutcome` by `(SlipID, LegID)` with a `(SlipID, legIdx)` fallback when `LegID == 0`, and updated `computeMigrations` to join outcomes by identity key rather than slice index.
- Added `CompareRubrics(dataset ReplayDataset, baseline, candidate analyzer.Rubric) (Report, Report, DiffReport)` to `internal/eval/metrics.go`.
- Added unit tests in `internal/eval/metrics_test.go` covering `SlipFactors` attribution on decided vs. push slips, `CompareRubrics`, and reordered, subset, empty, and zero-ID migration comparisons.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `eval.Report.SlipFactors` and `eval.CompareRubrics` are ready for `cmd/eval` and `internal/web/eval_results.go` to wire up in Phase 3.

## Commit

Track slip factors and key leg migrations by ID

Eval reports previously discarded slip-level factor attribution and zipped leg outcomes by slice index during rubric comparisons. Accumulating SlipFactors on decided slips and joining leg outcomes by stable slip and leg identity prevents silent migration corruption and exposes slip-level drivers alongside leg factors.
