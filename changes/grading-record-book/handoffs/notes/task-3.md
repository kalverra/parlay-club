# Task 3

## Done

- Added `PendingFinalLeg`, `Store.PendingLegsOnFinalGames`, and `Store.SetLegResultIfPending` (`WHERE id = ? AND result = 'PENDING'`) in `internal/db/legs.go` with unit tests in `internal/db/legs_test.go`.
- Added `slips.AutoGraderName` (`"auto"`), `slips.AutoGrade`, and `slips.Service.AutoGradeLegs` in `internal/slips/rules.go`, recomputing slip results/statuses via `gradeSlip`, writing system activity (`user_id = NULL`), and publishing `slip:{id}` only for slips with updated legs, with unit tests in `internal/slips/grade_test.go`.
- Added `GradeSource` (`"grade"`), `BoxScoreClient`, `SlipGrader`, and `grade.go` in `internal/refresh` with game-only score optimization (skipping `BoxScore` when a final game has no pending player-prop legs), per-game 5-minute error backoff, and singleflight + archive-keyed `BoxScore` fetches.
- Added integration tests in `internal/refresh/grade_test.go` covering game-only optimization, player-prop grading and zero-waste re-runs, manual override durability across re-runs, and per-game `BoxScore` error backoff.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `refresh.New` automatically wires `BoxScore` from `Deps.Scoreboard` (which is `*espn.Client` in `cmd/server/main.go` and `cmd/server/sandbox.go`) and defaults `Deps.Slips` to `slips.New(deps.Store, deps.Publisher, now)` when omitted, so callers do not need extra wiring unless passing a custom mock.
- Failed `BoxScore` fetches back off per-game under `SummarySource(gameID)` for `failureBackoff` (`5m`) without blocking other `FINAL` games from grading.
