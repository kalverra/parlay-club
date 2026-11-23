# Phase 1: Phase 1: Require shared slips module in refresh and wire production + sandbox startup

## Done

### Task 1: Remove the silent `slips.New` fallback from `refresh.New` in `internal/refresh/refresh.go`, guard `s.slips == nil` in `internal/refresh/grade.go`, and update `internal/refresh` tests to pass `Deps.Slips` explicitly alongside a new end-to-end test verifying `AutoGradeLegs` invalidates `eval.Cache` via `SetOnGradeChange`.

- Removed the silent `slips.New` fallback from `refresh.New` in `internal/refresh/refresh.go` so `s.slips` is always `deps.Slips` directly.
- Confirmed the existing `if s.slips == nil { return false, nil }` guard in `internal/refresh/grade.go` (`syncGrade`) and added `TestGradeSourceNilSlipsGuard` in `internal/refresh/grade_test.go`.
- Updated `TestGradeSourceGameOnlyOptimization`, `TestGradeSourcePlayerPropsAndZeroWasteReruns`, and `TestGradeSourceBoxScoreErrorBackoff` in `internal/refresh/grade_test.go` to pass `Deps.Slips` explicitly.
- Added `TestGradeSourceInvalidatesEvalCache` in `internal/refresh/grade_test.go` to verify `AutoGradeLegs` invalidates `eval.Cache` via `SetOnGradeChange(evalCache.InvalidateGroup)`.

### Task 2: Construct one shared `slips.Service` before `refresh.New` in `cmd/server/main.go` and `cmd/server/sandbox.go` and pass it into `refresh.Deps.Slips`, with a test verifying sandbox and server wiring share one instance so auto-grading invalidates `evalCache`.

- Constructed `slipService` before `refresh.New` in `cmd/server/sandbox.go` (`newSandboxRuntime`), passed it via `refresh.Deps.Slips`, and stored that same instance on `sandboxRuntime.slips`.
- Extracted `newProdServices` in `cmd/server/main.go` to construct `slipService`, attach `SetOnGradeChange(evalCache.InvalidateGroup)`, and pass `Slips: slipService` into `refresh.New` before returning both services.
- Added `TestProdServicesShareSlipsInstanceAndInvalidateEvalCache` in `cmd/server/main_test.go` and `TestSandboxRuntimeSharesSlipsInstanceAndInvalidatesEvalCache` in `cmd/server/sandbox_test.go` verifying auto-grading invalidates `evalCache`.

## Deviations

### Task 1

None. (`internal/refresh/grade.go` already had the `s.slips == nil` guard in `syncGrade`, so no edit to `grade.go` was needed.)

## Open issues

### Task 1

- `cmd/server/main.go` and `cmd/server/sandbox.go` do not yet pass `Deps.Slips` to `refresh.New`, so auto-grading is a no-op at runtime until Task 2 wires the shared `slips.Service`.

## Next-phase notes

### Task 1

- In Task 2, construct `slips.Service` first in `cmd/server/main.go` and `cmd/server/sandbox.go`, attach `SetOnGradeChange(evalCache.InvalidateGroup)`, and pass that same instance into `refresh.Deps.Slips`.

### Task 2

- Phase 2 can now default `deps.EvalCache` in `internal/web` and remove the `EvalCache == nil` fallback branch in `internal/web/eval_results.go`.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `internal/refresh/grade_test.go`
- `internal/refresh/refresh.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: lint 2.1s, test 8.8s; skipped: generate (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
