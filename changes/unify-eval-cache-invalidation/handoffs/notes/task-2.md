# Task 2: Wire one shared slips.Service in cmd/server

## Done

- Constructed `slipService` before `refresh.New` in `cmd/server/sandbox.go` (`newSandboxRuntime`), passed it via `refresh.Deps.Slips`, and stored that same instance on `sandboxRuntime.slips`.
- Extracted `newProdServices` in `cmd/server/main.go` to construct `slipService`, attach `SetOnGradeChange(evalCache.InvalidateGroup)`, and pass `Slips: slipService` into `refresh.New` before returning both services.
- Added `TestProdServicesShareSlipsInstanceAndInvalidateEvalCache` in `cmd/server/main_test.go` and `TestSandboxRuntimeSharesSlipsInstanceAndInvalidatesEvalCache` in `cmd/server/sandbox_test.go` verifying auto-grading invalidates `evalCache`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 can now default `deps.EvalCache` in `internal/web` and remove the `EvalCache == nil` fallback branch in `internal/web/eval_results.go`.

## Commit

Share one slips service across server and sandbox

Previously, server and sandbox startup omitted refresh.Deps.Slips and constructed a second slips.Service for HTTP handlers, leaving auto-grading without the evalCache invalidation hook. Constructing one shared slips.Service before refresh.New ensures auto-grading and manual grading share the same callback and keep evalCache fresh.
