# Phase 2: Phase 2: Pure archive parsers and leak-free historical replayer

## Done

### Task 2: Expose pure raw-JSON parsers (`ParseGameLogJSON`, `ParseRosterJSON` in `internal/ingest/espn`, and `ParseLinesJSON`, `ParsePropsJSON` in `internal/ingest/odds`) reusing existing decoders, with unit tests against fixture payloads.

The worker wrote no notes.

### Task 3: Implement `ReplayFilter`, `ReplayDataset`, `DataCoverage`, and `LoadDataset` in `internal/eval/replay.go`, plus leakage-guard and partial-archive fail-soft tests in `internal/eval/replay_test.go`.

The worker wrote no notes.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Files touched

None.

## Run log

- Mode: 
- Checkpoints: task-2, task-3
- Verify: verify passed: generate reused, lint reused, changed: internal/analyzer/docs.go, internal/analyzer/grade.go, internal/analyzer/rubric_test.go, test 500ms; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
