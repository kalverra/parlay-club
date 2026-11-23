# Task 5: Add the offline CLI in `cmd/eval/main.go` supporting single-rubric, `--baseline`/`--candidate`, `--candidate-file`, week/season filters, and `text`/`json` output, with smoke tests in `cmd/eval/main_test.go`.

## Done

- Blocked before implementation: verified `internal/eval/replay.go` (`LoadDataset`, `ReplayFilter`) and the pure raw-JSON parsers in `internal/ingest/espn` and `internal/ingest/odds` are missing from the worktree.

## Deviations

None.

## Open issues

- Phase 2 (Tasks 2 and 3) recorded `Files touched: None` in `changes/grading-evals/handoffs/phase-2.md`, so `eval.LoadDataset` and `eval.ReplayFilter` do not exist. `cmd/eval/main.go` and `cmd/eval/main_test.go` cannot load a SQLite database until Phase 2 is implemented.

## Next-phase notes

- Re-run Phase 2 Tasks 2 and 3 (`ParseGameLogJSON`, `ParseRosterJSON`, `ParseLinesJSON`, `ParsePropsJSON`, and `internal/eval/replay.go`) before re-running Task 5.
