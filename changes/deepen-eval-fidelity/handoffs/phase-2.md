# Phase 2: Phase 2: Track slip-level factor attribution and harden rubric comparison

## Done

### Task 2: Switch `rubricFactorLabels` in `internal/eval/metrics.go` to `analyzer.GradeRubricFor(rubric)`, add `SlipFactors []FactorStats` to `eval.Report` accumulated from `slipGrade.Factors` on decided slips, and key `legOutcome` by `(SlipID, LegID)` (with `(SlipID, index)` fallback for zero-ID fixtures) alongside `CompareRubrics`, with unit tests covering slip factors and mismatched/reordered comparisons.

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

### Task 2

- `eval.Report.SlipFactors` and `eval.CompareRubrics` are ready for `cmd/eval` and `internal/web/eval_results.go` to wire up in Phase 3.

## Files touched

- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`

## Run log

- Mode: 
- Checkpoints: task-2
- Verify: verify passed: generate 500ms, lint reused, changed: internal/eval/metrics.go, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
