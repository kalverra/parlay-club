# Phase 3: Phase 3: Calibration metrics, rubric diff, offline CLI, and docs

## Done

### Task 4: Implement `Evaluate` and `Compare` (leg/slip calibration by band, lift over implied odds, flat ROI, per-factor attribution, and migration matrix) in `internal/eval/metrics.go` with table-driven tests in `internal/eval/metrics_test.go`.

- Fixed the module import path in `internal/eval/types.go` from `github.com/parlay-club/parlay-club` to `github.com/kalverra/parlay-club`.
- Implemented `Evaluate` in `internal/eval/metrics.go` to compute leg and slip calibration by band (`A`–`F`, `ungraded`), implied probability, lift, flat ROI, Brier score, per-factor attribution, and monotonicity checks while excluding pushes and voids from decided denominators.
- Implemented `Compare` in `internal/eval/metrics.go` to compute per-band leg and slip deltas and the leg band migration matrix between baseline and candidate reports.
- Added table-driven tests in `internal/eval/metrics_test.go` covering empty datasets, leg/slip calibration, lift, flat ROI, factor attribution, push/void exclusion, monotonicity detection, and rubric comparison/migration.

### Task 5: Add the offline CLI in `cmd/eval/main.go` supporting single-rubric, `--baseline`/`--candidate`, `--candidate-file`, week/season filters, and `text`/`json` output, with smoke tests in `cmd/eval/main_test.go`.

- Blocked before implementation: verified `internal/eval/replay.go` (`LoadDataset`, `ReplayFilter`) and the pure raw-JSON parsers in `internal/ingest/espn` and `internal/ingest/odds` are missing from the worktree.

### Task 6: Docs: update `docs/concepts/analyzer.md` and `docs/guides/local-dev.md` to document versioned rubrics and how to run offline grading evals against a local or Litestream-restored SQLite database, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` to document versioned rubrics (`Rubric`, `RubricV1`, `ActiveRubric`, `ScoreLegWithRubric`, `ScoreSlipWithRubric`) and leak-free offline grading evals.
- Updated `docs/guides/local-dev.md` with an "Offline grading evals" section covering local SQLite runs, rubric comparison flags (`--baseline`, `--candidate`, `--candidate-file`), season/week/format flags, and Litestream restore workflows.
- Ran `good-vibes docs lint`, confirming zero errors or warnings on `docs/concepts/analyzer.md` and zero new warnings on `docs/guides/local-dev.md`.

## Deviations

None.

## Open issues

### Task 5

- Phase 2 (Tasks 2 and 3) recorded `Files touched: None` in `changes/grading-evals/handoffs/phase-2.md`, so `eval.LoadDataset` and `eval.ReplayFilter` do not exist. `cmd/eval/main.go` and `cmd/eval/main_test.go` cannot load a SQLite database until Phase 2 is implemented.

### Task 6

- `cmd/eval/main.go` and `internal/eval/replay.go` are not yet present in the worktree (see Task 5 notes); the CLI examples in `docs/guides/local-dev.md` match the design contract and will work once Phase 2 and Task 5 land.

## Next-phase notes

### Task 4

- `Evaluate` and `Compare` are ready for the offline CLI in `cmd/eval/main.go`.
- `Report` records per-leg outcomes internally so `Compare(baseline, candidate)` can compute the migration matrix directly from two `Report` values.

### Task 5

- Re-run Phase 2 Tasks 2 and 3 (`ParseGameLogJSON`, `ParseRosterJSON`, `ParseLinesJSON`, `ParsePropsJSON`, and `internal/eval/replay.go`) before re-running Task 5.

### Task 6

- None.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/guides/local-dev.md`
- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/eval/types.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 3
