# Task 3: Add single-pass `EvaluateSlip` and switch web callers to it

## Done

- Added `SlipEvaluation`, `EvaluateSlip`, and `EvaluateSlipWithRubric` in `internal/analyzer/grade.go`.
- Updated `ScoreSlipWithRubric` in `internal/analyzer/grade.go` to delegate to `EvaluateSlipWithRubric(...).Grade`.
- Updated `analyzerView` in `internal/web/analyzer.go` to call `analyzer.EvaluateSlip` once instead of calling `AnalyzeSlip` and `ScoreSlip` separately.
- Added `TestEvaluateSlipParity` in `internal/analyzer/rubric_test.go` covering empty/ungradable slips, multi-leg slips with stacks/conflicts/weather/hooks/median traps/chalk, and custom rubrics.
- Verified with `golangci-lint run ./internal/analyzer/... ./internal/web/...` (0 issues) and `go test -short ./internal/analyzer/... ./internal/web/...` (pass).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 3 can now extract shared single-leg predicates and grade factor rule constants across `rules.go`, `grade.go`, and `docs.go`.

## Commit

Add single-pass EvaluateSlip and switch web analyzerView

EvaluateSlipWithRubric runs AnalyzeSlip once and computes leg and slip grades from the same pass, eliminating the duplicate analysis calls in ScoreSlipWithRubric and analyzerView.
