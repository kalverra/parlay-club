# Task 1

## Done

- Removed the silent `slips.New` fallback from `refresh.New` in `internal/refresh/refresh.go` so `s.slips` is always `deps.Slips` directly.
- Confirmed the existing `if s.slips == nil { return false, nil }` guard in `internal/refresh/grade.go` (`syncGrade`) and added `TestGradeSourceNilSlipsGuard` in `internal/refresh/grade_test.go`.
- Updated `TestGradeSourceGameOnlyOptimization`, `TestGradeSourcePlayerPropsAndZeroWasteReruns`, and `TestGradeSourceBoxScoreErrorBackoff` in `internal/refresh/grade_test.go` to pass `Deps.Slips` explicitly.
- Added `TestGradeSourceInvalidatesEvalCache` in `internal/refresh/grade_test.go` to verify `AutoGradeLegs` invalidates `eval.Cache` via `SetOnGradeChange(evalCache.InvalidateGroup)`.

## Deviations

None. (`internal/refresh/grade.go` already had the `s.slips == nil` guard in `syncGrade`, so no edit to `grade.go` was needed.)

## Open issues

- `cmd/server/main.go` and `cmd/server/sandbox.go` do not yet pass `Deps.Slips` to `refresh.New`, so auto-grading is a no-op at runtime until Task 2 wires the shared `slips.Service`.

## Next-phase notes

- In Task 2, construct `slips.Service` first in `cmd/server/main.go` and `cmd/server/sandbox.go`, attach `SetOnGradeChange(evalCache.InvalidateGroup)`, and pass that same instance into `refresh.Deps.Slips`.

## Commit

Require explicit slips service in refresh

Omitting Deps.Slips in refresh.New previously constructed a private slips.Service without the eval cache invalidation callback, causing auto-graded legs to leave stale results in eval.Cache. Removing the silent fallback ensures callers share a single configured slips.Service and makes missing wiring explicit.
