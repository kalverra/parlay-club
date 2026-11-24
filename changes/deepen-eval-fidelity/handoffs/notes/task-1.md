# Task 1

## Done

- Updated `loadWeekSlips` in `internal/eval/replay.go` to retain all pre-kickoff legs (including `PENDING` and `NEEDS_GRADING`) on `SlipGraded` slips while computing `CombinedOdds` only from counting settled legs (`isSettledResult(dl.Result) && dl.Result.Counts()`).
- Updated `Evaluate` in `internal/eval/metrics.go` to pass all pre-kickoff legs to `analyzer.ScoreSlipWithRubric` while skipping unsettled legs during leg-band, leg-factor, and `legOutcomes` accumulation.
- Updated `slipDecimalOdds` in `internal/eval/metrics.go` to match `loadWeekSlips` by only including counting settled legs when deriving fallback slip decimal odds.
- Added `TestLoadDatasetEarlyBustedSlip` in `internal/eval/replay_test.go` and `TestEvaluateEarlyBustedSlip` in `internal/eval/metrics_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Evaluate` now skips unsettled legs before appending to `legOutcomes`, so Phase 2/3 identity-keyed `legOutcome` work only needs to key settled legs.

## Commit

Preserve unsettled legs on early-busted slips in eval

Early-busted graded slips previously dropped pending legs during replay,
distorting parlay drag, stack, and conflict scoring in offline evaluation.
Keeping all pre-kickoff legs for slip scoring while filtering unsettled legs
out of leg-level metrics restores parity with live pre-kickoff grading.
