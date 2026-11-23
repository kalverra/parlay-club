## Done

- Added `Store.LegsForSlips` in `internal/db/legs.go` to batch-load legs for multiple slip IDs in one query ordered by `(slip_id, id)`.
- Added `SlipWithLegs` and `Store.SlipsWithLegsForWeek` in `internal/db/slips.go` to load a week's slips (oldest slip ID first) and their legs with optional `groupID` (`0` = all) and `SlipStatus` (`""` = all) filtering.
- Added SQLite tests `TestLegsForSlipsBatchesAndPreservesOrder` in `internal/db/legs_test.go` and `TestSlipsWithLegsForWeekFiltersByGroupAndStatus` in `internal/db/slips_test.go`.

## Deviations

`SlipsWithLegsForWeek` takes `status SlipStatus` instead of `string` and validates non-empty values with `SlipStatus.Valid()`, matching `SetSlipStatus` and `SetSlipGrade` in `internal/db/slips.go`.

## Open issues

None.

## Next-phase notes

`internal/eval/replay.go` (`loadWeekSlips`) can now replace its per-slip `SlipIDsForWeek` + `SlipByID` + `LegsForSlip` loop with `store.SlipsWithLegsForWeek(ctx, weekID, groupID, db.SlipGraded)`.

## Commit

Add batched week slip-and-leg reads to internal/db

Replay currently loads a week's slips by issuing N+1 SlipByID and LegsForSlip queries. Batching week slip and leg reads into SlipsWithLegsForWeek and LegsForSlips lets callers filter by group and status in two queries without per-slip round trips.
