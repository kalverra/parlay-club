# Task 2: Parameterize rubric documentation by explicit `Rubric`

## Done

- Added `GradeRubricFor(r Rubric) RubricDoc` in `internal/analyzer/docs.go` to render grading bands, leg factors, and slip factors from the supplied `Rubric` without reading `ActiveRubric`.
- Updated `GradeRubric()` in `internal/analyzer/docs.go` to delegate directly to `GradeRubricFor(ActiveRubric)`.
- Added `TestGradeRubricForNonActiveRubric` in `internal/analyzer/nflverse_grade_test.go` verifying custom rubric weights, band cutoffs, labels, and descriptions render accurately without mutating `ActiveRubric`.
- Verified with `go test -race ./internal/analyzer/...` (pass) and `golangci-lint run ./internal/analyzer/...` (0 issues).

## Deviations

- Used `RubricDoc` as the return type of `GradeRubricFor(r Rubric)` rather than `GradeRubricDoc` as written in the design doc, matching the existing `RubricDoc` type in `internal/analyzer/docs.go`. Updated the Approach section of `changes/unify-analyzer-evaluation/design.md` to match.

## Open issues

None.

## Next-phase notes

- Phase 2 can now call `analyzer.GradeRubricFor(rubric)` when inspecting factor labels and weights for non-active candidate rubrics.

## Commit

Add GradeRubricFor for explicit rubric documentation

Callers evaluating non-active candidate rubrics need to inspect factor labels and point weights without reading or mutating the package-global ActiveRubric. Parameterizing rubric doc generation by an explicit Rubric keeps GradeRubric() unchanged while supporting isolated evaluation.
