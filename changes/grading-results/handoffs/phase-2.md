# Phase 2: Phase 2: In-memory eval cache, settlement invalidation, and Grading Results page

## Done

### Task 3: Implement `eval.Cache` in `internal/eval/cache.go` with concurrency/invalidation tests in `internal/eval/cache_test.go`, add `SetOnGradeChange` to `internal/slips/rules.go` (invoked on `SetLegResult` and `AutoGradeLegs` commits) with tests in `internal/slips/grade_test.go`, and wire both in `cmd/server/main.go` and `cmd/server/sandbox.go`.

- Implemented `eval.Cache` in `internal/eval/cache.go` with `(groupID, season)`-keyed `SuiteReport` memoization, generation-guarded cache writes, `InvalidateGroup`, and `InvalidateAll`.
- Added concurrency and group-isolation tests in `internal/eval/cache_test.go`.
- Added `Service.SetOnGradeChange` to `internal/slips/rules.go`, invoking the callback after commits in `SetLegResult` and for each distinct updated group in `AutoGradeLegs`, with unit tests in `internal/slips/grade_test.go`.
- Wired `eval.Cache` in `cmd/server/main.go` (connecting `slipService.SetOnGradeChange(evalCache.InvalidateGroup)`) and `cmd/server/sandbox.go` (calling `evalCache.InvalidateAll()` on sandbox stage transitions and resets).

### Task 4: Add `GET /analyzer/results` in `internal/web/eval_results.go` and `internal/web/slips.go`, create `web/templates/analyzer_results.html`, link it from `web/templates/analyzer_guide.html`, register the template in `internal/web/render.go` and `internal/web/render_test.go`, run `go generate ./...`, and add handler tests in `internal/web/eval_results_test.go` covering auth, group isolation, empty state, season/rubric filters, and cache invalidation after settlement.

- Added `EvalCache *eval.Cache` to `web.Dependencies` in `internal/web/routes.go` and wired `evalCache` into `web.NewWithDeps` in `cmd/server/main.go`.
- Implemented `GET /analyzer/results` in `internal/web/eval_results.go` and mounted it on `slipHandlers.Routes` in `internal/web/slips.go` with `?season=` (`all`/`0`/year, `400` on invalid), `?rubric=`, safe `?from=` handling, and `eval.Cache` lookup.
- Created `web/templates/analyzer_results.html` with phone-first cards and tables for leg and slip A–F calibration, rubric comparison, week-over-week progression, factor attribution, pre-kickoff data coverage, and empty state, and linked to it from `web/templates/analyzer_guide.html`.
- Registered `"analyzer-results": "templates/analyzer_results.html"` in `internal/web/render.go` and `internal/web/render_test.go`, and regenerated `web/static/app.css` via `go generate ./...`.
- Added handler tests in `internal/web/eval_results_test.go` covering auth redirects, empty state, invalid `?season=` (`400`), unsafe `?from=`, guide link, group isolation, season/rubric filters, and cache invalidation after `PUT /legs/{id}/result`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 3

`cmd/server/main.go` constructs `evalCache` and wires `slipService.SetOnGradeChange(evalCache.InvalidateGroup)`; Task 4 can pass `EvalCache: evalCache` into `web.Dependencies` once `EvalCache` is added there.

### Task 4

Task 5 can add the CI benchmark summary step in `.github/workflows/ci.yml`, `[tasks.eval-prod]` in `mise.toml`, `scripts/eval-prod.sh`, and any living docs updates.

## Files touched

- `cmd/server/main.go`
- `cmd/server/sandbox.go`
- `internal/eval/cache.go`
- `internal/eval/cache_test.go`
- `internal/slips/grade_test.go`
- `internal/slips/rules.go`
- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `internal/web/render.go`
- `internal/web/render_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/templates/analyzer_guide.html`
- `web/templates/analyzer_results.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: generate reused, lint 5.5s, test 7.6s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
