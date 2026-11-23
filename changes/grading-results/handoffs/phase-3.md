# Phase 3: Phase 3: CI benchmark summary, local snapshot task, and living docs

## Done

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

## Deviations

### Task 7

Removed backticks from six pre-existing runtime path references (`./data/parlay.db`, `./data/sunday-sandbox.db`, `./tmp/server`) in `docs/guides/local-dev.md` because `good-vibes docs lint` promotes `names-path` warnings to errors on modified docs.

## Open issues

None.

## Next-phase notes

### Task 5

The next tasks can add `[tasks.eval-prod]` in `mise.toml`, `scripts/eval-prod.sh`, and the living docs updates.

### Task 6

Task 7 can update the living docs under `docs/` (including `docs/ops/evals.md` or relevant ops/analyzer docs) and run `good-vibes docs lint`.

### Task 7

All Phase 3 tasks are complete and `good-vibes docs lint` passes with 0 errors.

## Files touched

- `.github/workflows/ci.yml`
- `docs/concepts/analyzer.md`
- `docs/guides/local-dev.md`
- `docs/reference/routes.md`
- `mise.toml`
- `scripts/eval-prod.sh`

## Run log

- Mode: 
- Checkpoints: task-5, task-6, task-7
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
