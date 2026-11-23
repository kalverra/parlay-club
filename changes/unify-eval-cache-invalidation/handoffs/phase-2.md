# Phase 2: Phase 2: Default eval cache at web construction and remove handler fallback

## Done

### Task 3: Default `deps.EvalCache = eval.NewCache()` when nil in `internal/web` constructor and remove the duplicate `eval.LoadDataset` / `eval.EvaluateAll` fallback branch in `internal/web/eval_results.go`, verifying nil-cache construction in `internal/web/eval_results_test.go`.

- Defaulted `deps.EvalCache = eval.NewCache()` when nil in `NewWithDeps` (`internal/web/routes.go`).
- Removed the duplicate `eval.LoadDataset` / `eval.EvaluateAll` fallback branch in `evalResults` (`internal/web/eval_results.go`), routing all `/analyzer/results` requests through `h.deps.EvalCache.GetOrCompute`.
- Added `TestEvalResultsDefaultsNilEvalCache` in `internal/web/eval_results_test.go` to verify `NewWithDeps` handles omitted `EvalCache`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Files touched

- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `internal/web/routes.go`

## Run log

- Mode: 
- Checkpoints: task-3
- Verify: verify passed: generate 400ms, lint reused, test 9.7s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
