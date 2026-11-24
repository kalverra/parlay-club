# Task 7

## Done

- Updated `web/templates/partials.html` (`research-player`) with the header injury badge, phone-first matchup strip (`OpponentLabel`, `Spread`, `GameTotal`, `Weather`), 6-tile hit-rate/range summary grid (`L5 Hit`, `Season Hit`, `L5 Range`), and position-tailored `UsageTiles`.
- Updated both the player Last-5 chart (`PlayerChart`) and opposing defense Last-5 chart (`OppDefenseChart`) to share a proportional `h-40` plot area with an exact horizontal dotted O/U reference line (`bottom: {{.LinePct}}%`, `O/U {{.LineStr}}`).
- Added per-bar opponent labels and color-tinted overall/position defense-rank badges (`OverallDef`, `PosDef`) on player bars, and rendered `OppDefenseChart` with overall/position rank pills and opposing player/team/week labels.
- Ran `go generate ./...` to rebuild `web/static/app.css`, updated `TestResearchPlayerSurfacesNFLverseMetricsAndHTMX` for RB-tailored usage tiles, and added `TestResearchPlayerRichChartsAndMatchupContext` in `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- All Phase 3 template and handler pieces are in place for `research-player`.

## Commit

Render dotted O/U charts, opp defense L5, and matchup strip

Update the research-player template so phone-width views show this week's matchup context, hit rates, position-tailored usage tiles, dotted O/U lines on both charts, and defense-rank badges. Regenerate Tailwind CSS and add HTML rendering tests covering the new chart and matchup elements.
