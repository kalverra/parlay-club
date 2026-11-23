# Phase 2: Phase 2: Add single-pass `EvaluateSlip` and switch web callers to it

## Done

### Task 3: Add `SlipEvaluation`, `EvaluateSlip`, and `EvaluateSlipWithRubric` in `internal/analyzer/grade.go` (with `ScoreSlipWithRubric` delegating to it), update `internal/web/analyzer.go` (`analyzerView`) to call `EvaluateSlip` once, and add tests covering single-pass warning and grade parity.

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

### Task 3

- Phase 3 can now extract shared single-leg predicates and grade factor rule constants across `rules.go`, `grade.go`, and `docs.go`.

## Files touched

- `internal/analyzer/grade.go`
- `internal/analyzer/rubric_test.go`
- `internal/web/analyzer.go`

## Run log

- Mode: 
- Checkpoints: task-3
- Verify: verify passed: generate 800ms, lint 7.8s, test 8.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
