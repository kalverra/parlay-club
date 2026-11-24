# Task 3

## Done

- Added `Sharp map[SharpKey]SharpQuote` to `analyzer.Game` in `internal/analyzer/analyzer.go` and raised `RubricV1.LinePriceMax` from `10` to `15` in `internal/analyzer/rubric.go`.
- Added `sharp_line_edge`, `sharp_line_fade`, `sharp_price_edge`, and `sharp_price_fade` rule IDs in `internal/analyzer/rules.go` and implemented `scoreSharpBenchmark` inside `scoreLineAndPrice` in `internal/analyzer/grade.go`.
- Registered all four sharp factors in `GradeRubricFor` (`internal/analyzer/docs.go`).
- Added `TestScoreLegSharpBenchmarks` in `internal/analyzer/analyzer_test.go` covering fail-soft behavior, line and price edge/fade thresholds across Tier 1/2 and Tier 3 consensus, player prop thresholds and name normalization, `LinePriceMax` clamping, `GradeRubric()` registration, and friendly tone checks.

## Deviations

- `LinePriceMax` lives on `Rubric` (`RubricV1.LinePriceMax` in `internal/analyzer/rubric.go`) rather than as a package constant in `grade.go`, so `RubricV1.LinePriceMax` was raised from `10` to `15`.

## Open issues

- None.

## Next-phase notes

- `LoadSlipSignals` can now populate `Game.Sharp` directly from `BuildSharpBenchmarks` output; `ScoreLeg` and `GradeRubric()` already handle `Game.Sharp` and fail soft when it is nil.

## Commit

Add sharp line and price grading factors to analyzer

ScoreLeg now compares leg lines and prices against tiered sharp benchmarks when Game.Sharp is populated, giving friends credit for beating sharp numbers and flagging steep juice in plain language.
