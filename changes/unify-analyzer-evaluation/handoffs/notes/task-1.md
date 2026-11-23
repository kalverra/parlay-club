# Task 1 Notes

## Done

- Defined `ruleHitRate`, `ruleRisingUsage`, `ruleTeammateOutBump`, `ruleFavorableMatchup`, `ruleNfeloCushion`, `ruleParlayDrag`, and `ruleChalkDrag` constants in `internal/analyzer/rules.go` and replaced all raw string literals across `grade.go` and `docs.go`.
- Extracted shared single-leg predicates in `internal/analyzer/rules.go`: `evalMedianTrap`, `evalWindHazard`, `isFakeWeatherLeg`, `evalSpreadHook`, `isChalkLeg`, and `chalkPayoutAndBust`.
- Updated `rules.go` and `grade.go` to call the shared predicates while preserving existing warning messages, grade factor details, and thresholds.
- Verified exact parity with `go test -race ./internal/analyzer/...` and `golangci-lint run ./internal/analyzer/...` (both pass with 0 issues).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 2 can add `GradeRubricFor(r Rubric) RubricDoc` in `internal/analyzer/docs.go` and wrap `GradeRubric()`. Note that `docs.go` uses `RubricDoc` (rather than `GradeRubricDoc` as written in the design doc).

## Commit

Unify analyzer rule constants and leg predicates

Centralizing grade factor rule IDs and single-leg threshold checks removes duplication between warning rules and leg grading so thresholds cannot drift apart. Existing analyzer tests and linter checks pass with identical behavior.
