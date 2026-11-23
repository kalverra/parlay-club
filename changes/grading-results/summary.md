---
slug: grading-results
run: 20261123-203849-grading-results
outcome: done
generated: 2026-11-23T21:01:32Z
---

# Run summary: grading-results

## Outcome

- Status: done
- Branch: grading-results
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 3
- Checkpoints: 7
- Duration: 19m15s
- Cost: $—
- Fresh input tokens: 1023567
- Cache read tokens: 10527313
- Cache write tokens: 0
- Output tokens: 183981

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 11734861 | $— |

## Phases

### Phase 1: Multi-rubric suite, weekly progression, and benchmark CLI

Tasks:

- [x] Add `BuiltInRubrics()` to `internal/analyzer/rubric.go`, add `WeekSummary`, `Report.Weekly`, `SuiteReport`, `EvaluateAll`, and `LoadDatasetJSON` in `internal/eval/`, and add unit tests in `internal/eval/metrics_test.go` and `internal/eval/replay_test.go`.
- [x] Add the deterministic synthetic fixture `internal/eval/testdata/benchmark_dataset.json`, extend `cmd/eval/main.go` with `--dataset`, `--all`, and `--format markdown` output, and cover all new CLI modes and error cases in `cmd/eval/main_test.go`.

Phase totals: 2 checkpoints, 5m41s, $—, 2880311 input / 51908 output / 2608263 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `BuiltInRubrics()` to `internal/analyzer/rubric.go`, add `WeekSummary`, `Report.Weekly`, `SuiteReport`, `EvaluateAll`, and `LoadDatasetJSON` in `internal/eval/`, and add unit tests in `internal/eval/metrics_test.go` and `internal/eval/replay_test.go`.

- Added `BuiltInRubrics()` to `internal/analyzer/rubric.go` returning a defensive copy of registered rubrics in registry order, with test coverage in `internal/analyzer/rubric_test.go`.
- Added `WeekSummary`, `Report.Weekly`, `SuiteReport`, and `EvaluateAll` in `internal/eval/metrics.go`, sorting weekly summaries chronologically by `(Season, WeekType, WeekNum)`.
- Added `LoadDatasetJSON` in `internal/eval/replay.go` with `Coverage.TotalLegs` fallback when omitted in JSON.
- Added `TestEvaluateWeekly` and `TestEvaluateAll` in `internal/eval/metrics_test.go`, plus `TestLoadDatasetJSON` in `internal/eval/replay_test.go`.

### Task 2: Add the deterministic synthetic fixture `internal/eval/testdata/benchmark_dataset.json`, extend `cmd/eval/main.go` with `--dataset`, `--all`, and `--format markdown` output, and cover all new CLI modes and error cases in `cmd/eval/main_test.go`.

- Added deterministic synthetic fixture `internal/eval/testdata/benchmark_dataset.json` (4 weeks, 24 slips, 72 legs spanning spreads, totals, moneylines, and player props across all A-F bands with weather, injury, nfelo, nflverse, stack, and conflict signals).
- Extended `cmd/eval/main.go` with `--dataset`, `--all`, and `--format markdown` output covering single-rubric, suite, and comparison reports.
- Added CLI tests in `cmd/eval/main_test.go` covering `--dataset`, `--all`, `--format markdown`, and error cases (`--db` vs `--dataset` mutual exclusivity, `--all` with compare flags, unsupported `--format`, and missing dataset).

### Phase 2: In-memory eval cache, settlement invalidation, and Grading Results page

Tasks:

- [x] Implement `eval.Cache` in `internal/eval/cache.go` with concurrency/invalidation tests in `internal/eval/cache_test.go`, add `SetOnGradeChange` to `internal/slips/rules.go` (invoked on `SetLegResult` and `AutoGradeLegs` commits) with tests in `internal/slips/grade_test.go`, and wire both in `cmd/server/main.go` and `cmd/server/sandbox.go`.
- [x] Add `GET /analyzer/results` in `internal/web/eval_results.go` and `internal/web/slips.go`, create `web/templates/analyzer_results.html`, link it from `web/templates/analyzer_guide.html`, register the template in `internal/web/render.go` and `internal/web/render_test.go`, run `go generate ./...`, and add handler tests in `internal/web/eval_results_test.go` covering auth, group isolation, empty state, season/rubric filters, and cache invalidation after settlement.

Phase totals: 2 checkpoints, 8m38s, $—, 4222044 input / 69596 output / 3905504 cache read / 0 cache write tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 3: CI benchmark summary, local snapshot task, and living docs

Tasks:

- [x] Add the offline benchmark eval summary step to `.github/workflows/ci.yml` (publishing to `$GITHUB_STEP_SUMMARY`) and verify with `actionlint`, leaving `lefthook.yml` `pre-push` unchanged.
- [x] Add `scripts/eval-prod.sh` and `[tasks.eval-prod]` in `mise.toml` for one-command read-only Litestream snapshot restore and rubric comparison with automatic temp-file cleanup.
- [x] Docs: update `docs/concepts/analyzer.md`, `docs/reference/routes.md`, and `docs/guides/local-dev.md` to document `GET /analyzer/results`, in-memory eval caching, the CI benchmark summary, and `mise run eval-prod`, then run `good-vibes docs lint`.

Phase totals: 3 checkpoints, 4m17s, $—, 3436508 input / 50290 output / 3096796 cache read / 0 cache write tokens

Checkpoints: task-5, task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 5: Add the offline benchmark eval summary step to `.github/workflows/ci.yml` (publishing to `$GITHUB_STEP_SUMMARY`) and verify with `actionlint`, leaving `lefthook.yml` `pre-push` unchanged.

- Added the `Grading eval benchmark summary` step to `.github/workflows/ci.yml` after `Test` to run `go run ./cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown >> "$GITHUB_STEP_SUMMARY"`.
- Verified `.github/workflows/ci.yml` with `actionlint` and tested the `cmd/eval` markdown command against `internal/eval/testdata/benchmark_dataset.json`.
- Left `lefthook.yml` `pre-push` unchanged.

### Task 6: Add `scripts/eval-prod.sh` and `[tasks.eval-prod]` in `mise.toml` for one-command read-only Litestream snapshot restore and rubric comparison with automatic temp-file cleanup.

- Added executable `scripts/eval-prod.sh` to validate `litestream` and `LITESTREAM_BUCKET`/`LITESTREAM_REPLICA_URL`, restore a snapshot into a temporary directory under `./data/` using `deploy/litestream.yml`, clean up the temp directory via `trap` on `EXIT`/`INT`/`TERM`, and run `go run ./cmd/eval --db "$tmpdb" "$@"` (defaulting to `--all`).
- Added `[tasks.eval-prod]` in `mise.toml` invoking `./scripts/eval-prod.sh`.
- Verified `sh -n scripts/eval-prod.sh`, error exits when `litestream` or replica env vars are missing, temp-dir cleanup after a failed mock `litestream restore`, and `mise tasks ls` registration.

### Task 7: Docs: update `docs/concepts/analyzer.md`, `docs/reference/routes.md`, and `docs/guides/local-dev.md` to document `GET /analyzer/results`, in-memory eval caching, the CI benchmark summary, and `mise run eval-prod`, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` (overview diagram, versioned rubrics and CI benchmark summary, and `GET /analyzer/results` + `eval.Cache` invalidation).
- Updated `docs/reference/routes.md` with `GET /analyzer/results` in the Shell and tabs table and a dedicated `### Analyzer results` section covering auth/group scoping, `?season=`, `?rubric=`, `?from=`, and `eval.Cache` invalidation.
- Updated `docs/guides/local-dev.md` under `Offline grading evals` to document `--dataset`, `--all`, `--format markdown`, the CI benchmark summary command, and `mise run eval-prod` (`scripts/eval-prod.sh`), and removed backticks on six non-existent runtime paths so `good-vibes docs lint` passes.
- Ran `good-vibes docs lint`: `0 error(s), 59 warning(s)`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
