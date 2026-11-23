# Task 3

## Done

- Defaulted `deps.EvalCache = eval.NewCache()` when nil in `NewWithDeps` (`internal/web/routes.go`).
- Removed the duplicate `eval.LoadDataset` / `eval.EvaluateAll` fallback branch in `evalResults` (`internal/web/eval_results.go`), routing all `/analyzer/results` requests through `h.deps.EvalCache.GetOrCompute`.
- Added `TestEvalResultsDefaultsNilEvalCache` in `internal/web/eval_results_test.go` to verify `NewWithDeps` handles omitted `EvalCache`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Default web EvalCache when nil and drop fallback

Ensure the web package always owns an eval cache at construction time so request handlers do not duplicate dataset loading and evaluation logic. This keeps all grading results reads on a single cache path.
