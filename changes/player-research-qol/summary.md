---
slug: player-research-qol
run: 20261124-144040-player-research-qol
outcome: done
generated: 2026-11-24T15:18:17Z
---

# Run summary: player-research-qol

## Outcome

- Status: done
- Branch: player-research-qol
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 3
- Checkpoints: 8
- Duration: 31m13s
- Cost: $—
- Fresh input tokens: 1736199
- Cache read tokens: 20354612
- Cache write tokens: 0
- Output tokens: 296689

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 22387500 | $— |

## Phases

### Phase 1: Persist ESPN game-log opponents and nflverse weekly matchup stats

Tasks:

- [x] Extend `internal/ingest/espn/gamelog.go` to decode the `events` map into `GameLog.Entries` (aligned with `Markets`), add migration `internal/db/migrations/0015_research_context.sql` with `player_gamelogs.games_json` and the new `nflverse_player_weeks` columns and opponent/position index, and update `internal/db/players.go` and `internal/refresh/research.go` to store `games_json`, with unit tests against saved ESPN fixtures.
- [x] Extend `internal/ingest/nflverse/player_stats.go`, `internal/db/nflverse.go`, and `internal/refresh/nflverse.go` to parse, upsert, and query weekly opponent, player name, position, passing/rushing/receiving/kicking stats (`NFLversePlayerWeeksAgainstOpponent`, `NFLversePlayerWeeksForPositionTypes`), with fixture and store round-trip tests.
- [x] Docs: update `docs/reference/schema.md` and `docs/reference/data-sources.md` for migration `0015_research_context.sql`, `player_gamelogs.games_json`, and the expanded `nflverse_player_weeks` columns.

Phase totals: 3 checkpoints, 9m1s, $—, 5407174 input / 82670 output / 4959390 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Extend `internal/ingest/espn/gamelog.go` to decode the `events` map into `GameLog.Entries` (aligned with `Markets`), add migration `internal/db/migrations/0015_research_context.sql` with `player_gamelogs.games_json` and the new `nflverse_player_weeks` columns and opponent/position index, and update `internal/db/players.go` and `internal/refresh/research.go` to store `games_json`, with unit tests against saved ESPN fixtures.

- Extended `internal/ingest/espn/gamelog.go` with `GameLogEntry` and `GameLog.Entries` decoded from the ESPN `events` map in lockstep with `Markets`.
- Added `internal/db/migrations/0015_research_context.sql` with `player_gamelogs.games_json`, the 12 additive `nflverse_player_weeks` opponent/position/stat columns, and `idx_nflverse_player_weeks_opp_pos`, updating migration tests to schema version 15.
- Updated `internal/db/players.go` (`PlayerGameLog`, `UpsertPlayerGameLog`, `scanPlayerGameLog`) and `internal/refresh/research.go` (`syncGameLog`) to persist and read `games_json`.
- Added unit tests against saved ESPN fixtures (`gamelog.json`, `gamelog_chase.json`, `gamelog_burrow.json`, `gamelog_mcpherson.json`), `db.TestPlayerGameLogs`, `db.TestMigration0015ResearchContext`, and `refresh.TestSyncPlayerResearchGameLog`.

### Task 2: Extend `internal/ingest/nflverse/player_stats.go`, `internal/db/nflverse.go`, and `internal/refresh/nflverse.go` to parse, upsert, and query weekly opponent, player name, position, passing/rushing/receiving/kicking stats (`NFLversePlayerWeeksAgainstOpponent`, `NFLversePlayerWeeksForPositionTypes`), with fixture and store round-trip tests.

- Extended `internal/ingest/nflverse/player_stats.go` (`PlayerWeek`, `playerStatsColumns`, `parsePlayerStatsRow`, `normalizePosition`) to parse `opponent_team` (mapped through `mapTeam`), `player_display_name` (falling back to `player_name`), `position` (normalizing `FB`/`HB` to `RB`), `attempts`, `fg_made`, and `pat_made`.
- Updated `internal/db/nflverse.go` (`NFLversePlayerWeek`, `nflversePlayerWeekColumns`, `UpsertNFLversePlayerWeeks`, `scanNFLversePlayerWeeks`) to persist and scan the 12 new opponent, player name, position, and passing/rushing/receiving/kicking stat columns while keeping unchanged rows' `updated_at` intact.
- Added `Store.NFLversePlayerWeeksAgainstOpponent` and `Store.NFLversePlayerWeeksForPositionTypes` in `internal/db/nflverse.go` with position normalization and multi-season-type ordering.
- Updated `internal/refresh/nflverse.go` (`buildNFLversePlayerWeeks`) to populate the new `db.NFLversePlayerWeek` fields (including `KickerPts = 3*FGMade + PATMade`) during nflverse sync.
- Updated and added fixture, store round-trip, and refresh sync tests in `internal/ingest/nflverse/nflverse_test.go`, `internal/db/nflverse_test.go`, and `internal/refresh/nflverse_test.go`.

### Task 3: Docs: update `docs/reference/schema.md` and `docs/reference/data-sources.md` for migration `0015_research_context.sql`, `player_gamelogs.games_json`, and the expanded `nflverse_player_weeks` columns.

- Updated `docs/reference/schema.md` for migration `0015_research_context.sql` (schema version 15), adding `player_gamelogs.games_json` and the 12 new `nflverse_player_weeks` columns plus `idx_nflverse_player_weeks_opp_pos` to the summary, migrations list, table overview, DDL, and conventions.
- Updated `docs/reference/data-sources.md` to document `espn.GameLog.Entries` (decoded from `events` and stored in `player_gamelogs.games_json`) and the expanded `nflverse.PlayerStats` columns (`player_display_name`, `position` normalization, `opponent_team`, `attempts`, `fg_made`, `pat_made`, and `kicker_pts`).

### Phase 2: Compute defense ranks, opponent L5 history, and market-specific O/U lookup

Tasks:

- [x] Add `DefOverallEPAAllowed` and `DefOverallEPARank` to `analyzer.TeamDefenseProfile`, implement `BuildPositionDefenseRanks` and `BuildOppDefenseLast5` in `internal/analyzer/signals.go`, and expand `LoadPlayerResearchSignals` with defense rank maps, opposing defense L5 games, and position-relevant L3 usage averages, with unit tests covering edge cases (`<5` games, ties, `FB`→`RB`, stale nflverse).
- [x] Add `Store.LatestPropForPlayerMarket` in `internal/db/odds.go` so player research resolves the current week's O/U line for a specific market without bleeding lines across market tabs, with unit tests in `internal/db/players_test.go`.

Phase totals: 2 checkpoints, 5m50s, $—, 3809673 input / 73963 output / 3447549 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Add `DefOverallEPAAllowed` and `DefOverallEPARank` to `analyzer.TeamDefenseProfile`, implement `BuildPositionDefenseRanks` and `BuildOppDefenseLast5` in `internal/analyzer/signals.go`, and expand `LoadPlayerResearchSignals` with defense rank maps, opposing defense L5 games, and position-relevant L3 usage averages, with unit tests covering edge cases (`<5` games, ties, `FB`→`RB`, stale nflverse).

- Added `DefOverallEPAAllowed` and `DefOverallEPARank` to `analyzer.TeamDefenseProfile` and computed/ranked them in `BuildTeamDefenseProfiles` (`internal/analyzer/analyzer.go`, `internal/analyzer/signals.go`).
- Implemented `BuildPositionDefenseRanks` and `BuildOppDefenseLast5` (with `OppDefenseGame`) in `internal/analyzer/signals.go`, supporting position normalization (`FB`/`HB` → `RB`), primary-market defaults, tie-breaking, `<5` games, and zero-value games.
- Expanded `PlayerResearchSignals` and `LoadPlayerResearchSignals` with `TeamDefProfiles`, `PosDefRanks`, `OppOverallDefRank`, `OppPosDefRank`, `OppDefenseL5`, and position-relevant L3 usage averages (`CarriesPerGame`, `TargetsPerGame`, `PassAttPerGame`, `AirYardsShare`, `RZOppsPerGame`, `EPAPerGame`).
- Added unit tests in `internal/analyzer/signals_test.go` covering overall defense ranks, position defense ranks, opposing defense L5 history, `<5` games, ties, `FB`→`RB` normalization, and stale nflverse fallback.

### Task 5: Add `Store.LatestPropForPlayerMarket` in `internal/db/odds.go` so player research resolves the current week's O/U line for a specific market without bleeding lines across market tabs, with unit tests in `internal/db/players_test.go`.

- Added `Store.LatestPropForPlayerMarket(ctx, athleteID, playerName, team, market)` in `internal/db/odds.go` to return the newest prop odds row with a non-nil line filtered by `market_type` when `market` is non-empty.
- Delegated `Store.LatestPropForPlayer` to `LatestPropForPlayerMarket(..., "")` so candidate resolution and query logic stay in one place.
- Added `TestLatestPropForPlayerMarket` in `internal/db/players_test.go` covering empty-market fallback, market-specific filtering without cross-market bleed, newest-row selection, `player_matches` lookup, nil-line exclusion, and team filtering.

### Phase 3: Render dotted O/U charts, opposing defense L5 chart, and matchup context

Tasks:

- [x] Update `internal/web/research.go` to resolve per-market O/U lines on tab switches, build proportional `BarChartView` data (including `LinePct`, hit rates, L5 min/max, player L5 opponent + overall/position defense badges, and opposing defense L5 bars), load this week's matchup context (opponent, cached spread/total, weather/indoor, player injury status), and assemble position-tailored usage tiles.
- [x] Update `web/templates/partials.html` (`research-player`) with the phone-first matchup strip, hit-rate/range tiles, position-tailored usage row, horizontal dotted O/U reference line on both Last-5 charts, opponent and defense-rank badges on player bars, and the opposing-defense Last-5 chart; run `go generate ./...` and add comprehensive tests in `internal/web/research_test.go`.

Phase totals: 3 checkpoints, 15m40s, $—, 10209076 input / 128704 output / 9504357 cache read / 0 cache write tokens

Checkpoints: task-6, task-7, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 1
- Final verify: passed
- Open issues: none
