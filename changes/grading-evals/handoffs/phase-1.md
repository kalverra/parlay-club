# Phase 1: Phase 1: Versioned rubrics in `internal/analyzer`

## Done

### Task 1: Introduce `Rubric`, `RubricV1`, `ActiveRubric`, `RubricByName`, `ScoreLegWithRubric`, and `ScoreSlipWithRubric` in `internal/analyzer/rubric.go`, refactor `grade.go` and `docs.go` to read from `Rubric`, and add equivalence and override tests in `internal/analyzer/rubric_test.go`.

The worker wrote no notes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Files touched

- `internal/analyzer/docs.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/rubric.go`
- `internal/analyzer/rubric_test.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: generate 500ms, lint changed: internal/analyzer/docs.go, internal/analyzer/grade.go, internal/analyzer/rubric_test.go, test 9.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
