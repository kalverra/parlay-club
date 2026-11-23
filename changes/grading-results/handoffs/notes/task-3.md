# Task 3 Notes

## Done

- Implemented `eval.Cache` in `internal/eval/cache.go` with `(groupID, season)`-keyed `SuiteReport` memoization, generation-guarded cache writes, `InvalidateGroup`, and `InvalidateAll`.
- Added concurrency and group-isolation tests in `internal/eval/cache_test.go`.
- Added `Service.SetOnGradeChange` to `internal/slips/rules.go`, invoking the callback after commits in `SetLegResult` and for each distinct updated group in `AutoGradeLegs`, with unit tests in `internal/slips/grade_test.go`.
- Wired `eval.Cache` in `cmd/server/main.go` (connecting `slipService.SetOnGradeChange(evalCache.InvalidateGroup)`) and `cmd/server/sandbox.go` (calling `evalCache.InvalidateAll()` on sandbox stage transitions and resets).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`cmd/server/main.go` constructs `evalCache` and wires `slipService.SetOnGradeChange(evalCache.InvalidateGroup)`; Task 4 can pass `EvalCache: evalCache` into `web.Dependencies` once `EvalCache` is added there.

## Commit

Add in-memory eval cache and settlement invalidation

Group-scoped grading results need fast repeated reads without persisting derived snapshots in SQLite or serving stale calibration after a leg settles. `eval.Cache` memoizes `SuiteReport` values per `(groupID, season)` in memory and clears affected groups via `slips.Service.SetOnGradeChange` and sandbox stage resets.
