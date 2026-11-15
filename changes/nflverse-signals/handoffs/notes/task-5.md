# Task 5: Wire `nflverse` DB lookups, clickable grade pills, and `/analyzer/guide`

## Done

- Wired fresh `nflverse` team EPA/pace profiles and player usage lookups (`nflverseForSlip`, `buildTeamDefenseProfiles`, `buildPlayerUsage`) into `internal/web/analyzer.go`, `internal/web/slips.go`, and `internal/web/pick.go` with a 7-day freshness guard (`nflverseFreshFor`) and fail-soft fallback on missing or stale data.
- Added deterministic slip and leg `GradeView`/`GradeFactorView` rendering in `web/templates/partials.html` for both `#slip-panel` and `#pick-tray`, including clickable `<details>` factor breakdowns, `"How grading works"` link, `"See full rubric"` links, and `#rule-<id>` `"Why?"` links on every warning card.
- Added `GET /analyzer/guide` (`analyzerGuide` in `internal/web/analyzer.go`, `web/templates/analyzer_guide.html`, registered in `internal/web/render.go`) backed directly by `analyzer.RuleDocs()` and `analyzer.GradeRubric()` with `#leg-grades`, `#slip-grade`, `#suggestions`, and `#rule-<id>` anchors plus safe `?from=` back-link support.
- Regenerated `web/static/app.css` via `go generate ./...` and added handler render tests in `internal/web/analyzer_test.go` covering `/analyzer/guide` rule/anchor coverage, unsafe `from` rejection, fresh vs. stale `nflverse` wiring, and `#slip-panel` / `#pick-tray` grade pills and links.

## Deviations

- Also wired `nflverseForSlip` and `GradeView` into `internal/web/pick.go` (`dockFrom`) so `#pick-tray` renders the same clickable leg/slip grade pills, inline factor breakdowns, and `#rule-<id>` links as `#slip-panel`.

## Open issues

- None.

## Next-phase notes

- Phase 3 (`internal/board`, `internal/web/pick.go`, `internal/web/research.go`) can reuse `nflverseSource`, `nflverseFreshFor`, and `buildTeamDefenseProfiles` / `buildPlayerUsage` patterns in `internal/web/analyzer.go` when loading `TeamEPA` and `PlayerUsage` for the pick board and research views.
