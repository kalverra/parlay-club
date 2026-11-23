# Phase 1: Phase 1: Multi-rubric suite, weekly progression, and benchmark CLI

## Done

### Task 1: Add `BuiltInRubrics()` to `internal/analyzer/rubric.go`, add `WeekSummary`, `Report.Weekly`, `SuiteReport`, `EvaluateAll`, and `LoadDatasetJSON` in `internal/eval/`, and add unit tests in `internal/eval/metrics_test.go` and `internal/eval/replay_test.go`.

- Added `BuiltInRubrics()` to `internal/analyzer/rubric.go` returning a defensive copy of registered rubrics in registry order, with test coverage in `internal/analyzer/rubric_test.go`.
- Added `WeekSummary`, `Report.Weekly`, `SuiteReport`, and `EvaluateAll` in `internal/eval/metrics.go`, sorting weekly summaries chronologically by `(Season, WeekType, WeekNum)`.
- Added `LoadDatasetJSON` in `internal/eval/replay.go` with `Coverage.TotalLegs` fallback when omitted in JSON.
- Added `TestEvaluateWeekly` and `TestEvaluateAll` in `internal/eval/metrics_test.go`, plus `TestLoadDatasetJSON` in `internal/eval/replay_test.go`.

### Task 2: Add the deterministic synthetic fixture `internal/eval/testdata/benchmark_dataset.json`, extend `cmd/eval/main.go` with `--dataset`, `--all`, and `--format markdown` output, and cover all new CLI modes and error cases in `cmd/eval/main_test.go`.

- Added deterministic synthetic fixture `internal/eval/testdata/benchmark_dataset.json` (4 weeks, 24 slips, 72 legs spanning spreads, totals, moneylines, and player props across all A-F bands with weather, injury, nfelo, nflverse, stack, and conflict signals).
- Extended `cmd/eval/main.go` with `--dataset`, `--all`, and `--format markdown` output covering single-rubric, suite, and comparison reports.
- Added CLI tests in `cmd/eval/main_test.go` covering `--dataset`, `--all`, `--format markdown`, and error cases (`--db` vs `--dataset` mutual exclusivity, `--all` with compare flags, unsupported `--format`, and missing dataset).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`EvaluateAll` returns a `SuiteReport` with `Diffs` empty when only `RubricV1` is registered and populated when additional rubrics are passed; `cmd/eval` and `eval.Cache` can consume it directly.

### Task 2

`cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown` is ready to be wired into `.github/workflows/ci.yml`.

## Files touched

- `cmd/eval/main.go`
- `cmd/eval/main_test.go`
- `internal/analyzer/rubric.go`
- `internal/analyzer/rubric_test.go`
- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`
- `internal/eval/testdata/benchmark_dataset.json`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 500ms, lint 1.9s, test 1.1s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
