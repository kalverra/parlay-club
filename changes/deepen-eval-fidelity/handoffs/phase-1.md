# Phase 1: Phase 1: Preserve full pre-kickoff slips in replay and filter unsettled legs during leg aggregation

## Done

### Task 1: Update `loadWeekSlips` in `internal/eval/replay.go` to keep all pre-kickoff legs on `SlipGraded` slips (computing `CombinedOdds` only from counting settled legs), and update `Evaluate` in `internal/eval/metrics.go` to score the full slip while restricting leg-band, leg-factor, and outcome accumulation to settled legs, with replay and metrics tests for early-busted slips.

- Updated `loadWeekSlips` in `internal/eval/replay.go` to retain all pre-kickoff legs (including `PENDING` and `NEEDS_GRADING`) on `SlipGraded` slips while computing `CombinedOdds` only from counting settled legs (`isSettledResult(dl.Result) && dl.Result.Counts()`).
- Updated `Evaluate` in `internal/eval/metrics.go` to pass all pre-kickoff legs to `analyzer.ScoreSlipWithRubric` while skipping unsettled legs during leg-band, leg-factor, and `legOutcomes` accumulation.
- Updated `slipDecimalOdds` in `internal/eval/metrics.go` to match `loadWeekSlips` by only including counting settled legs when deriving fallback slip decimal odds.
- Added `TestLoadDatasetEarlyBustedSlip` in `internal/eval/replay_test.go` and `TestEvaluateEarlyBustedSlip` in `internal/eval/metrics_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `Evaluate` now skips unsettled legs before appending to `legOutcomes`, so Phase 2/3 identity-keyed `legOutcome` work only needs to key settled legs.

## Files touched

- `internal/eval/metrics.go`
- `internal/eval/metrics_test.go`
- `internal/eval/replay.go`
- `internal/eval/replay_test.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: generate 600ms, lint 5.8s, test 9.7s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
