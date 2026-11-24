# Phase 3: Phase 3: Render dotted O/U charts, opposing defense L5 chart, and matchup context

## Done

### Task 6: Update `internal/web/research.go` to resolve per-market O/U lines on tab switches, build proportional `BarChartView` data (including `LinePct`, hit rates, L5 min/max, player L5 opponent + overall/position defense badges, and opposing defense L5 bars), load this week's matchup context (opponent, cached spread/total, weather/indoor, player injury status), and assemble position-tailored usage tiles.

- Updated `resolveMarketAndLine` and `buildMarketOptions` in `internal/web/research.go` to resolve per-market O/U lines via `Store.LatestPropForPlayerMarket` and omit `&line=` from market tab URLs.
- Added shared `BarChartView`, `DefRankBadge`, `MatchupContextView`, and `UsageTileView` models, with proportional bar/O/U line percentages (`LinePct`, `HeightPct`), L5/season hit rates, L5 min–max, player L5 opponent + defense rank badges, and opposing defense L5 bars.
- Loaded this week's matchup context (`MatchupContextView`: opponent, cached spread/total, indoor/weather, and `InjuryStatus`) and built position-tailored `UsageTiles` for QB, RB, WR/TE, and other/K positions.
- Added `TestResearchPlayerMarketTabResolvesOwnLine` in `internal/web/research_test.go` and `TestBuildPlayerResearchViewDetails` / `TestBuildUsageTilesByPosition` in `internal/web/research_unit_test.go`.

### Task 7: Update `web/templates/partials.html` (`research-player`) with the phone-first matchup strip, hit-rate/range tiles, position-tailored usage row, horizontal dotted O/U reference line on both Last-5 charts, opponent and defense-rank badges on player bars, and the opposing-defense Last-5 chart; run `go generate ./...` and add comprehensive tests in `internal/web/research_test.go`.

- Updated `web/templates/partials.html` (`research-player`) with the header injury badge, phone-first matchup strip (`OpponentLabel`, `Spread`, `GameTotal`, `Weather`), 6-tile hit-rate/range summary grid (`L5 Hit`, `Season Hit`, `L5 Range`), and position-tailored `UsageTiles`.
- Updated both the player Last-5 chart (`PlayerChart`) and opposing defense Last-5 chart (`OppDefenseChart`) to share a proportional `h-40` plot area with an exact horizontal dotted O/U reference line (`bottom: {{.LinePct}}%`, `O/U {{.LineStr}}`).
- Added per-bar opponent labels and color-tinted overall/position defense-rank badges (`OverallDef`, `PosDef`) on player bars, and rendered `OppDefenseChart` with overall/position rank pills and opposing player/team/week labels.
- Ran `go generate ./...` to rebuild `web/static/app.css`, updated `TestResearchPlayerSurfacesNFLverseMetricsAndHTMX` for RB-tailored usage tiles, and added `TestResearchPlayerRichChartsAndMatchupContext` in `internal/web/research_test.go`.

## Deviations

### Task 6

- Kept legacy `PlayerResearchView` fields (`Bars`, `SnapPct`, `TargetShare`, `WOPR`, `OppDefLabel`, `OppDefRank`) populated alongside `PlayerChart` and `UsageTiles` so existing `web/templates/partials.html` and template tests keep passing until the template task updates them.

## Open issues

None.

## Next-phase notes

### Task 6

- `web/templates/partials.html` (`research-player`) can now render `PlayerChart` (`HasLine`, `LinePct`, `LineStr`, `Bars` with `OpponentLabel`, `OverallDef`, `PosDef`), `OppDefenseChart` (`OppDefenseTitle`, `OppOverallBadge`, `OppPosBadge`), `Matchup` + `InjuryStatus`, `L5HitRate`/`SeasonHitRate`/`L5MinMax`, and `UsageTiles`.

### Task 7

- All Phase 3 template and handler pieces are in place for `research-player`.

## Files touched

- `internal/web/research.go`
- `internal/web/research_test.go`
- `internal/web/research_unit_test.go`
- `web/static/app.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-6, task-7
- Verify: verify passed: generate 400ms, lint changed: internal/web/research_test.go, test 17.9s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
