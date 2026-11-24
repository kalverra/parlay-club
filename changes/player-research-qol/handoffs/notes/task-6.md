# Task 6

## Done

- Updated `resolveMarketAndLine` and `buildMarketOptions` in `internal/web/research.go` to resolve per-market O/U lines via `Store.LatestPropForPlayerMarket` and omit `&line=` from market tab URLs.
- Added shared `BarChartView`, `DefRankBadge`, `MatchupContextView`, and `UsageTileView` models, with proportional bar/O/U line percentages (`LinePct`, `HeightPct`), L5/season hit rates, L5 min–max, player L5 opponent + defense rank badges, and opposing defense L5 bars.
- Loaded this week's matchup context (`MatchupContextView`: opponent, cached spread/total, indoor/weather, and `InjuryStatus`) and built position-tailored `UsageTiles` for QB, RB, WR/TE, and other/K positions.
- Added `TestResearchPlayerMarketTabResolvesOwnLine` in `internal/web/research_test.go` and `TestBuildPlayerResearchViewDetails` / `TestBuildUsageTilesByPosition` in `internal/web/research_unit_test.go`.

## Deviations

- Kept legacy `PlayerResearchView` fields (`Bars`, `SnapPct`, `TargetShare`, `WOPR`, `OppDefLabel`, `OppDefRank`) populated alongside `PlayerChart` and `UsageTiles` so existing `web/templates/partials.html` and template tests keep passing until the template task updates them.

## Open issues

None.

## Next-phase notes

- `web/templates/partials.html` (`research-player`) can now render `PlayerChart` (`HasLine`, `LinePct`, `LineStr`, `Bars` with `OpponentLabel`, `OverallDef`, `PosDef`), `OppDefenseChart` (`OppDefenseTitle`, `OppOverallBadge`, `OppPosBadge`), `Matchup` + `InjuryStatus`, `L5HitRate`/`SeasonHitRate`/`L5MinMax`, and `UsageTiles`.

## Commit

Build research charts, matchup context, and per-market O/U data

Switching market tabs previously carried the old market's O/U line and rendered fixed WR-centric tiles without opponent context. Resolving each market's own O/U line, computing proportional chart and defense badge data, and assembling matchup and position-tailored tiles prepares the handler for the updated research partial.
