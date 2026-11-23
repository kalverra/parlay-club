# Phase 1: Phase 1: Unify single-leg predicates, factor rule constants, and explicit-rubric docs

## Done

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

## Deviations

### Task 2

- Used `RubricDoc` as the return type of `GradeRubricFor(r Rubric)` rather than `GradeRubricDoc` as written in the design doc, matching the existing `RubricDoc` type in `internal/analyzer/docs.go`. Updated the Approach section of `changes/unify-analyzer-evaluation/design.md` to match.

## Open issues

None.

## Next-phase notes

### Task 1

- Task 2 can add `GradeRubricFor(r Rubric) RubricDoc` in `internal/analyzer/docs.go` and wrap `GradeRubric()`. Note that `docs.go` uses `RubricDoc` (rather than `GradeRubricDoc` as written in the design doc).

### Task 2

- Phase 2 can now call `analyzer.GradeRubricFor(rubric)` when inspecting factor labels and weights for non-active candidate rubrics.

## Files touched

- `internal/analyzer/docs.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/nflverse_grade_test.go`
- `internal/analyzer/rules.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 700ms, lint 5s, test 8.3s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
