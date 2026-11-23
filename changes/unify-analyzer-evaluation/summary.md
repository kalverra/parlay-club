---
slug: unify-analyzer-evaluation
run: 20261123-213808-unify-analyzer-evaluation
outcome: done
generated: 2026-11-23T21:52:06Z
---

# Run summary: unify-analyzer-evaluation

## Outcome

- Status: done
- Branch: unify-analyzer-evaluation
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 3 of 3
- Phases: 2
- Checkpoints: 4
- Duration: 9m33s
- Cost: $—
- Fresh input tokens: 230806
- Cache read tokens: 1196589
- Cache write tokens: 0
- Output tokens: 24488

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 1451883 | $— |

## Phases

### Phase 1: Unify single-leg predicates, factor rule constants, and explicit-rubric docs

Tasks:

- [x] Define constants for all grade factor rule IDs in `internal/analyzer`, extract shared single-leg predicates (`medianTrap`, weather hazard, spread hook, heavy chalk) used by both `rules.go` and `grade.go`, and verify exact parity against existing analyzer tests.
- [x] Add `GradeRubricFor(r Rubric) GradeRubricDoc` in `internal/analyzer/docs.go` with `GradeRubric()` delegating to `ActiveRubric`, and add unit tests confirming non-active rubrics render their own weights and labels without touching global state.

Phase totals: 2 checkpoints, 4m8s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Define constants for all grade factor rule IDs in `internal/analyzer`, extract shared single-leg predicates (`medianTrap`, weather hazard, spread hook, heavy chalk) used by both `rules.go` and `grade.go`, and verify exact parity against existing analyzer tests.

- Defined `ruleHitRate`, `ruleRisingUsage`, `ruleTeammateOutBump`, `ruleFavorableMatchup`, `ruleNfeloCushion`, `ruleParlayDrag`, and `ruleChalkDrag` constants in `internal/analyzer/rules.go` and replaced all raw string literals across `grade.go` and `docs.go`.
- Extracted shared single-leg predicates in `internal/analyzer/rules.go`: `evalMedianTrap`, `evalWindHazard`, `isFakeWeatherLeg`, `evalSpreadHook`, `isChalkLeg`, and `chalkPayoutAndBust`.
- Updated `rules.go` and `grade.go` to call the shared predicates while preserving existing warning messages, grade factor details, and thresholds.
- Verified exact parity with `go test -race ./internal/analyzer/...` and `golangci-lint run ./internal/analyzer/...` (both pass with 0 issues).

### Task 2: Add `GradeRubricFor(r Rubric) GradeRubricDoc` in `internal/analyzer/docs.go` with `GradeRubric()` delegating to `ActiveRubric`, and add unit tests confirming non-active rubrics render their own weights and labels without touching global state.

- Added `GradeRubricFor(r Rubric) RubricDoc` in `internal/analyzer/docs.go` to render grading bands, leg factors, and slip factors from the supplied `Rubric` without reading `ActiveRubric`.
- Updated `GradeRubric()` in `internal/analyzer/docs.go` to delegate directly to `GradeRubricFor(ActiveRubric)`.
- Added `TestGradeRubricForNonActiveRubric` in `internal/analyzer/nflverse_grade_test.go` verifying custom rubric weights, band cutoffs, labels, and descriptions render accurately without mutating `ActiveRubric`.
- Verified with `go test -race ./internal/analyzer/...` (pass) and `golangci-lint run ./internal/analyzer/...` (0 issues).

### Phase 2: Add single-pass `EvaluateSlip` and switch web callers to it

Tasks:

- [x] Add `SlipEvaluation`, `EvaluateSlip`, and `EvaluateSlipWithRubric` in `internal/analyzer/grade.go` (with `ScoreSlipWithRubric` delegating to it), update `internal/web/analyzer.go` (`analyzerView`) to call `EvaluateSlip` once, and add tests covering single-pass warning and grade parity.

Phase totals: 2 checkpoints, 4m44s, $—, 931680 input / 16956 output / 810048 cache read / 0 cache write tokens

Checkpoints: task-3, final Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `SlipEvaluation`, `EvaluateSlip`, and `EvaluateSlipWithRubric` in `internal/analyzer/grade.go` (with `ScoreSlipWithRubric` delegating to it), update `internal/web/analyzer.go` (`analyzerView`) to call `EvaluateSlip` once, and add tests covering single-pass warning and grade parity.

- Added `SlipEvaluation`, `EvaluateSlip`, and `EvaluateSlipWithRubric` in `internal/analyzer/grade.go`.
- Updated `ScoreSlipWithRubric` in `internal/analyzer/grade.go` to delegate to `EvaluateSlipWithRubric(...).Grade`.
- Updated `analyzerView` in `internal/web/analyzer.go` to call `analyzer.EvaluateSlip` once instead of calling `AnalyzeSlip` and `ScoreSlip` separately.
- Added `TestEvaluateSlipParity` in `internal/analyzer/rubric_test.go` covering empty/ungradable slips, multi-leg slips with stacks/conflicts/weather/hooks/median traps/chalk, and custom rubrics.
- Verified with `golangci-lint run ./internal/analyzer/... ./internal/web/...` (0 issues) and `go test -short ./internal/analyzer/... ./internal/web/...` (pass).

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
