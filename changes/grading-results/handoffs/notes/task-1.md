## Done

- Added `BuiltInRubrics()` to `internal/analyzer/rubric.go` returning a defensive copy of registered rubrics in registry order, with test coverage in `internal/analyzer/rubric_test.go`.
- Added `WeekSummary`, `Report.Weekly`, `SuiteReport`, and `EvaluateAll` in `internal/eval/metrics.go`, sorting weekly summaries chronologically by `(Season, WeekType, WeekNum)`.
- Added `LoadDatasetJSON` in `internal/eval/replay.go` with `Coverage.TotalLegs` fallback when omitted in JSON.
- Added `TestEvaluateWeekly` and `TestEvaluateAll` in `internal/eval/metrics_test.go`, plus `TestLoadDatasetJSON` in `internal/eval/replay_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`EvaluateAll` returns a `SuiteReport` with `Diffs` empty when only `RubricV1` is registered and populated when additional rubrics are passed; `cmd/eval` and `eval.Cache` can consume it directly.
