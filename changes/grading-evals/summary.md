---
slug: grading-evals
run: 20261123-192703-grading-evals
outcome: done
generated: 2026-11-23T19:59:26Z
---

# Run summary: grading-evals

## Outcome

- Status: done
- Branch: grading-evals
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 6 of 6
- Phases: 3
- Checkpoints: 7
- Duration: 19m47s
- Cost: $—
- Fresh input tokens: 950070
- Cache read tokens: 6053050
- Cache write tokens: 0
- Output tokens: 136037

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 7139157 | $— |

## Phases

### Phase 1: Versioned rubrics in `internal/analyzer`

Tasks:

- [x] Introduce `Rubric`, `RubricV1`, `ActiveRubric`, `RubricByName`, `ScoreLegWithRubric`, and `ScoreSlipWithRubric` in `internal/analyzer/rubric.go`, refactor `grade.go` and `docs.go` to read from `Rubric`, and add equivalence and override tests in `internal/analyzer/rubric_test.go`.

Phase totals: 1 checkpoints, 3m29s, $—, 1092359 input / 25606 output / 999232 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Introduce `Rubric`, `RubricV1`, `ActiveRubric`, `RubricByName`, `ScoreLegWithRubric`, and `ScoreSlipWithRubric` in `internal/analyzer/rubric.go`, refactor `grade.go` and `docs.go` to read from `Rubric`, and add equivalence and override tests in `internal/analyzer/rubric_test.go`.

The worker wrote no notes.

### Phase 2: Pure archive parsers and leak-free historical replayer

Tasks:

- [x] Expose pure raw-JSON parsers (`ParseGameLogJSON`, `ParseRosterJSON` in `internal/ingest/espn`, and `ParseLinesJSON`, `ParsePropsJSON` in `internal/ingest/odds`) reusing existing decoders, with unit tests against fixture payloads.
- [x] Implement `ReplayFilter`, `ReplayDataset`, `DataCoverage`, and `LoadDataset` in `internal/eval/replay.go`, plus leakage-guard and partial-archive fail-soft tests in `internal/eval/replay_test.go`.

Phase totals: 2 checkpoints, 3m50s, $—, 3294389 input / 51735 output / 2966889 cache read / 0 cache write tokens

Checkpoints: task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 2: Expose pure raw-JSON parsers (`ParseGameLogJSON`, `ParseRosterJSON` in `internal/ingest/espn`, and `ParseLinesJSON`, `ParsePropsJSON` in `internal/ingest/odds`) reusing existing decoders, with unit tests against fixture payloads.

The worker wrote no notes.

### Task 3: Implement `ReplayFilter`, `ReplayDataset`, `DataCoverage`, and `LoadDataset` in `internal/eval/replay.go`, plus leakage-guard and partial-archive fail-soft tests in `internal/eval/replay_test.go`.

The worker wrote no notes.

### Phase 3: Calibration metrics, rubric diff, offline CLI, and docs

Tasks:

- [x] Implement `Evaluate` and `Compare` (leg/slip calibration by band, lift over implied odds, flat ROI, per-factor attribution, and migration matrix) in `internal/eval/metrics.go` with table-driven tests in `internal/eval/metrics_test.go`.
- [x] Add the offline CLI in `cmd/eval/main.go` supporting single-rubric, `--baseline`/`--candidate`, `--candidate-file`, week/season filters, and `text`/`json` output, with smoke tests in `cmd/eval/main_test.go`.
- [x] Docs: update `docs/concepts/analyzer.md` and `docs/guides/local-dev.md` to document versioned rubrics and how to run offline grading evals against a local or Litestream-restored SQLite database, then run `good-vibes docs lint`.

Phase totals: 4 checkpoints, 11m42s, $—, 2012446 input / 51057 output / 1622338 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
