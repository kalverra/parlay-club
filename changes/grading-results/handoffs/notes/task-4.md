# Task 4 Notes

## Done

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

Task 5 can add the CI benchmark summary step in `.github/workflows/ci.yml`, `[tasks.eval-prod]` in `mise.toml`, `scripts/eval-prod.sh`, and any living docs updates.

## Commit

Add in-app Grading Results page at GET /analyzer/results

Expose group-scoped A-F calibration, rubric comparison, weekly progression, factor attribution, and data coverage through the in-memory eval cache so members can inspect grading performance without touching production data or extra APIs.
