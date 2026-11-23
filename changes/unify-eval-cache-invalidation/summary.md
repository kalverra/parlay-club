---
slug: unify-eval-cache-invalidation
run: 20261123-212728-unify-eval-cache-invalidation
outcome: done
generated: 2026-11-23T21:37:05Z
---

# Run summary: unify-eval-cache-invalidation

## Outcome

- Status: done
- Branch: unify-eval-cache-invalidation
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-low approved (issues: 0)

## Totals

- Tasks: 3 of 3
- Phases: 2
- Checkpoints: 3
- Duration: 7m7s
- Cost: $—
- Fresh input tokens: 0
- Cache read tokens: 0
- Cache write tokens: 0
- Output tokens: 0

## Phases

### Phase 1: Require shared slips module in refresh and wire production + sandbox startup

Tasks:

- [x] Remove the silent `slips.New` fallback from `refresh.New` in `internal/refresh/refresh.go`, guard `s.slips == nil` in `internal/refresh/grade.go`, and update `internal/refresh` tests to pass `Deps.Slips` explicitly alongside a new end-to-end test verifying `AutoGradeLegs` invalidates `eval.Cache` via `SetOnGradeChange`.
- [x] Construct one shared `slips.Service` before `refresh.New` in `cmd/server/main.go` and `cmd/server/sandbox.go` and pass it into `refresh.Deps.Slips`, with a test verifying sandbox and server wiring share one instance so auto-grading invalidates `evalCache`.

Phase totals: 2 checkpoints, 4m59s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Remove the silent `slips.New` fallback from `refresh.New` in `internal/refresh/refresh.go`, guard `s.slips == nil` in `internal/refresh/grade.go`, and update `internal/refresh` tests to pass `Deps.Slips` explicitly alongside a new end-to-end test verifying `AutoGradeLegs` invalidates `eval.Cache` via `SetOnGradeChange`.

- Removed the silent `slips.New` fallback from `refresh.New` in `internal/refresh/refresh.go` so `s.slips` is always `deps.Slips` directly.
- Confirmed the existing `if s.slips == nil { return false, nil }` guard in `internal/refresh/grade.go` (`syncGrade`) and added `TestGradeSourceNilSlipsGuard` in `internal/refresh/grade_test.go`.
- Updated `TestGradeSourceGameOnlyOptimization`, `TestGradeSourcePlayerPropsAndZeroWasteReruns`, and `TestGradeSourceBoxScoreErrorBackoff` in `internal/refresh/grade_test.go` to pass `Deps.Slips` explicitly.
- Added `TestGradeSourceInvalidatesEvalCache` in `internal/refresh/grade_test.go` to verify `AutoGradeLegs` invalidates `eval.Cache` via `SetOnGradeChange(evalCache.InvalidateGroup)`.

### Task 2: Construct one shared `slips.Service` before `refresh.New` in `cmd/server/main.go` and `cmd/server/sandbox.go` and pass it into `refresh.Deps.Slips`, with a test verifying sandbox and server wiring share one instance so auto-grading invalidates `evalCache`.

- Constructed `slipService` before `refresh.New` in `cmd/server/sandbox.go` (`newSandboxRuntime`), passed it via `refresh.Deps.Slips`, and stored that same instance on `sandboxRuntime.slips`.
- Extracted `newProdServices` in `cmd/server/main.go` to construct `slipService`, attach `SetOnGradeChange(evalCache.InvalidateGroup)`, and pass `Slips: slipService` into `refresh.New` before returning both services.
- Added `TestProdServicesShareSlipsInstanceAndInvalidateEvalCache` in `cmd/server/main_test.go` and `TestSandboxRuntimeSharesSlipsInstanceAndInvalidatesEvalCache` in `cmd/server/sandbox_test.go` verifying auto-grading invalidates `evalCache`.

### Phase 2: Default eval cache at web construction and remove handler fallback

Tasks:

- [x] Default `deps.EvalCache = eval.NewCache()` when nil in `internal/web` constructor and remove the duplicate `eval.LoadDataset` / `eval.EvaluateAll` fallback branch in `internal/web/eval_results.go`, verifying nil-cache construction in `internal/web/eval_results_test.go`.

Phase totals: 1 checkpoints, 1m32s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Default `deps.EvalCache = eval.NewCache()` when nil in `internal/web` constructor and remove the duplicate `eval.LoadDataset` / `eval.EvaluateAll` fallback branch in `internal/web/eval_results.go`, verifying nil-cache construction in `internal/web/eval_results_test.go`.

- Defaulted `deps.EvalCache = eval.NewCache()` when nil in `NewWithDeps` (`internal/web/routes.go`).
- Removed the duplicate `eval.LoadDataset` / `eval.EvaluateAll` fallback branch in `evalResults` (`internal/web/eval_results.go`), routing all `/analyzer/results` requests through `h.deps.EvalCache.GetOrCompute`.
- Added `TestEvalResultsDefaultsNilEvalCache` in `internal/web/eval_results_test.go` to verify `NewWithDeps` handles omitted `EvalCache`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
