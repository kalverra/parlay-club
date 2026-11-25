# Task 17

## Done

- Extended `GradeView` in `internal/web/analyzer.go` with `Uncertain`, `CorrSummary`, `Pillars` (`PillarView` with tonal badge/bar classes), and up to 3 plain-English `Why` bullets ranked from the strongest pillar drivers, plus exported `LegGradeView`/`SlipGradeView` helpers and optional `Dependencies.GradingEngine` injection in `internal/web/routes.go`, `internal/web/slips.go`, and `internal/web/pick.go` (including highlight legs and parlays).
- Extracted a shared `grade-breakdown` template in `web/templates/partials.html` used across slip panel legs, slip panel banner, pick tray legs, pick tray banner, and highlight leg/parlay cards, rendering the four-pillar meters, `"High uncertainty"` badge, `CorrSummary`, and `"Why"` bullets cleanly within 375px phone width while falling back to v1 factor lists when `Pillars` is empty.
- Updated `AnalyzerGuidePage` and `web/templates/analyzer_guide.html` to document the v2 four-pillar scoring model, Power de-vigging, Gaussian copula script multiplier, SGP correlation tax, `"High uncertainty"` badge, and v2 leg/slip weights while keeping A–F bands, warning rules, and v1 factor tables intact.
- Regenerated `web/static/app.css` via `go generate ./...` and added template rendering and HTTP handler tests (`TestGradeBreakdownsRenderFourPillarsAndV1FallbackTemplates` and `TestSlipPanelTrayAndGuideWithEngineV2Handler`) in `internal/web/analyzer_test.go`.

## Deviations

- Added an optional `GradingEngine` field to `web.Dependencies` (wiring it in `slips.go`, `pick.go`, and `analyzer.go`) and updated `highlightsView`/`legHighlightView` in `slips.go` to use `SlipGradeView`/`LegGradeView` so v2 pillars flow into highlight cards and handler tests can exercise `EngineV2` in parallel without mutating global `analyzer.ActiveEngine`.

## Open issues

- When Task 18 promotes `analyzer.ActiveEngine = analyzer.EngineV2`, `TestAnalyzerGuideRendersAllRulesAndAnchors` (`Active engine: v1`) and `TestSlipPanelTrayAndGuideRenderSharpFactors` (which expects v1 `Sharp line edge` / `+8` factor rows on panel and tray) in `internal/web/analyzer_test.go` will need updating for v2 outputs.

## Next-phase notes

- `web_test` already passes with `EngineV2` wired via `Dependencies.GradingEngine`; once `eval-prod` clears gates and `ActiveEngine` switches to `EngineV2`, check `TestAnalyzerGuideRendersAllRulesAndAnchors` and `TestSlipPanelTrayAndGuideRenderSharpFactors` alongside any other `ActiveEngine == EngineV1` assertions.

## Commit

Render v2 four-pillar grades and guide on web UI

Adds the four-pillar breakdown, plain-English Why bullets, script multiplier note, and High uncertainty badge across leg, slip, tray, and highlight popovers at 375px phone width while preserving v1 factor fallback and updating the in-app grading guide.
