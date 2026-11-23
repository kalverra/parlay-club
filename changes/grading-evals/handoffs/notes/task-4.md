# Task 4: Implement `Evaluate` and `Compare` (leg/slip calibration by band, lift over implied odds, flat ROI, per-factor attribution, and migration matrix) in `internal/eval/metrics.go` with table-driven tests in `internal/eval/metrics_test.go`.

## Done

- Fixed the module import path in `internal/eval/types.go` from `github.com/parlay-club/parlay-club` to `github.com/kalverra/parlay-club`.
- Implemented `Evaluate` in `internal/eval/metrics.go` to compute leg and slip calibration by band (`A`–`F`, `ungraded`), implied probability, lift, flat ROI, Brier score, per-factor attribution, and monotonicity checks while excluding pushes and voids from decided denominators.
- Implemented `Compare` in `internal/eval/metrics.go` to compute per-band leg and slip deltas and the leg band migration matrix between baseline and candidate reports.
- Added table-driven tests in `internal/eval/metrics_test.go` covering empty datasets, leg/slip calibration, lift, flat ROI, factor attribution, push/void exclusion, monotonicity detection, and rubric comparison/migration.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Evaluate` and `Compare` are ready for the offline CLI in `cmd/eval/main.go`.
- `Report` records per-leg outcomes internally so `Compare(baseline, candidate)` can compute the migration matrix directly from two `Report` values.

## Commit

Implement calibration metrics and rubric diff in internal/eval

Add Evaluate and Compare in internal/eval/metrics.go to measure leg and slip calibration, lift over implied odds, flat ROI, per-factor attribution, and band migration across rubrics. Fix the module import path in internal/eval/types.go and add table-driven tests in internal/eval/metrics_test.go so offline rubric evaluation and comparison are verified before building the CLI.
