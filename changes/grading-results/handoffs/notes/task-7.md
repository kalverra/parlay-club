## Done

- Updated `docs/concepts/analyzer.md` (overview diagram, versioned rubrics and CI benchmark summary, and `GET /analyzer/results` + `eval.Cache` invalidation).
- Updated `docs/reference/routes.md` with `GET /analyzer/results` in the Shell and tabs table and a dedicated `### Analyzer results` section covering auth/group scoping, `?season=`, `?rubric=`, `?from=`, and `eval.Cache` invalidation.
- Updated `docs/guides/local-dev.md` under `Offline grading evals` to document `--dataset`, `--all`, `--format markdown`, the CI benchmark summary command, and `mise run eval-prod` (`scripts/eval-prod.sh`), and removed backticks on six non-existent runtime paths so `good-vibes docs lint` passes.
- Ran `good-vibes docs lint`: `0 error(s), 59 warning(s)`.

## Deviations

Removed backticks from six pre-existing runtime path references (`./data/parlay.db`, `./data/sunday-sandbox.db`, `./tmp/server`) in `docs/guides/local-dev.md` because `good-vibes docs lint` promotes `names-path` warnings to errors on modified docs.

## Open issues

None.

## Next-phase notes

All Phase 3 tasks are complete and `good-vibes docs lint` passes with 0 errors.

## Commit

Document grading results, eval caching, CI benchmark, and eval-prod

Updates the analyzer concept, routes reference, and local-dev guide so the living docs reflect GET /analyzer/results, group-scoped in-memory eval caching, the CI fixture benchmark summary, and the one-command mise run eval-prod workflow.
