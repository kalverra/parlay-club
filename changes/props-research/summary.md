---
slug: props-research
run: 20261109-204047-props-research
outcome: done
generated: 2026-11-10T16:38:36Z
---

# Run summary: props-research

## Outcome

- Status: done
- Branch: props-research
- Final verdict: agy@gemini-4-argon-medium approved (issues: 0)

## Totals

- Tasks: 9 of 9
- Phases: 4
- Checkpoints: 10
- Duration: 1h59m
- Cost: $—
- Input tokens: 6385554
- Output tokens: 69060
- Cache read tokens: 6091823

## Phases

### Phase 1: ESPN rosters, game logs, injuries, and player matching

Tasks:

- [x] Save real ESPN `roster`, `gamelog`, and `summary` fixtures under `internal/ingest/espn/testdata/`, and add `Client.Roster`, `Client.GameLog`, `Client.SummaryInjuries`, and `ParseInjuries` in `internal/ingest/espn` with fixture unit tests and `TestLive*` tests that skip under `-short`
- [x] Add migration `0007_props_research.sql` (`roster_players` and `player_gamelogs`), Store methods for rosters, game logs, `games.injuries_json`, and `player_matches`, plus `NormalizePlayerName`, `ResolvePlayerMatch`, and `SaveManualPlayerMatch` with unit tests for suffixes, punctuation, manual overrides, and same-name players across teams

Phase totals: 2 checkpoints, 25m22s, $—, 0 input / 0 output / 0 cache tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

- Ingest clients and parsers in `internal/ingest/espn`:
  - `Client.Roster` fetches `GET /teams/{teamID}/roster` and flattens position groups into `RosterPlayer` records.
  - `Client.SummaryInjuries` fetches `GET /summary?event={id}` for raw injuries JSON. `ParseInjuries` extracts per-team injury lists filtered to key offensive and defensive positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`).
  - `Client.GameLog` fetches athlete game logs via `webBaseURL`. It maps stat column headers dynamically, counts games played, and extracts series for `PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, and `KICKER_PTS`.
  - Added test fixtures (`roster.json`, `summary.json`, `gamelog.json`, `gamelog_kicker.json`), fixture unit tests, and live tests skipping under `-short`.
- Database storage and migration in `internal/db`:
  - Added migration `0007_props_research.sql` creating `roster_players` and `player_gamelogs` tables.
  - Added `Store` methods for roster players (`UpsertRosterPlayers`, `RosterPlayerByID`, `RosterForTeam`, `RosterForTeams`), player gamelogs (`UpsertPlayerGameLog`, `PlayerGameLogByID`, `PlayerGameLogsByIDs`), and game injuries (`SetGameInjuries`).
- Player matching logic in `internal/db`:
  - `NormalizePlayerName` cleans punctuation, collapses whitespace, and drops generational suffixes (`jr`, `sr`, `ii`, `iii`, `iv`, `v`).
  - `ResolvePlayerMatch` checks manual matches, cached matches, and roster records across both matchup teams. Unambiguous hits auto-cache with `manual = 0`. Ambiguous or absent matches return unmatched without guessing.
  - `SaveManualPlayerMatch` records manual overrides (`manual = 1`), removes conflicting non-manual opponent matches in the same game, and backfills `team` and `player_id` across matching `odds` and `legs`.
- Living docs updated in `docs/reference/data-sources.md` and `docs/reference/schema.md`.

### Phase 2: On-demand prop refresh and ESPN research syncs

Tasks:

- [x] Add `SyncGameProps`, `SyncGameResearch`, and `SyncPlayerResearch` (plus background gamelog sync for matched slip legs) in `internal/refresh` for `odds:props:{game}` (2 h window, 150-credit Parlay API floor, The Odds API `EventProps`/`MergeProps` enrichment and fallback, skipped after kickoff), `espn:roster:{team}` (24 h), `espn:gamelog:{id}` (12 h), and `espn:summary:{game}` (2 h, non-final), with fake-clock tests proving that opening the same game twice within the cache window spends credits once
- [x] Docs: update `docs/reference/data-sources.md`, `docs/concepts/data-refresh.md`, and `docs/reference/schema.md` with ESPN rosters, game logs, summary injuries, `odds:props:{game}`, and player matching, and delete those built sections from `plan/data-sources.md`

Phase totals: 2 checkpoints, 22m10s, $—, 0 input / 0 output / 0 cache tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

- Staleness rules and source keys in `internal/refresh`:
  - Added `OddsPropsSource(gameID)` (`odds:props:{game}`): 2-hour cache window, skipped after kickoff.
  - Added `RosterSource(teamID)` (`espn:roster:{team}`): 24-hour cache window.
  - Added `SummarySource(gameID)` (`espn:summary:{game}`): 2-hour cache window, skipped once game is final.
  - Added `GameLogSource(athleteID)` (`espn:gamelog:{id}`): 12-hour cache window.
- On-demand prop sync and fallback in `internal/refresh/research.go`:
  - Syncs home and away rosters before matching prop players.
  - Checks Parlay API floor (`OddsPropsFloor = 150`).
  - Merges or falls back to The Odds API `EventProps` based on enrichment floor (100) and fallback floor (25).
  - Resolves prop subject teams for `TEAM_TOTAL` and player props via `ResolvePlayerMatch`.
  - Upserts `db.Odds` rows and records sync state.
- Service entry points and background syncs in `internal/refresh/refresh.go`:
  - `SyncGameProps`: synchronous 10-second timeout fetch for game prop odds.
  - `SyncGameResearch`: synchronous 10-second timeout fetch for game summary injuries, props, and rosters.
  - `SyncPlayerResearch`: synchronous 10-second timeout fetch for athlete game logs.
  - Background gamelog sync in `Kick()`: inspects active week's slip legs and syncs stale game logs for any matched player.
- Tests with fake clocks and stubbed clients in `internal/refresh/research_test.go` and `internal/refresh/rules_test.go`:
  - Proved opening the same game twice within the 2-hour window spends Parlay API credits only once.
  - Verified props are skipped after kickoff.
  - Verified Parlay API credit floor, The Odds API fallback, and enrichment behavior.
  - Verified concurrent singleflight deduplication on props sync.
  - Verified `Kick()` background gamelog sync for matched slip legs.
- Living documentation and plan cleanup:
  - Updated `docs/concepts/data-refresh.md`, `docs/reference/data-sources.md`, and `docs/reference/schema.md`.
  - Pruned built sections from `plan/data-sources.md`.

### Phase 3: `median_trap` analyzer rule

Tasks:

- [x] Add the `median_trap` rule in `internal/analyzer` with table-driven threshold tests (`line > L5 median`, `line <= median`, `< 3` games, unmatched player, and the `line < mean` sentence) and wire stored `player_gamelogs` into `internal/web/analyzer.go` with handler tests
- [x] Docs: update `docs/reference/analyzer-rules.md` and `docs/concepts/analyzer.md` with `median_trap` and live player stats, and remove the built `median_trap` section from `plan/analyzer.md`

Phase totals: 2 checkpoints, 13m48s, $—, 0 input / 0 output / 0 cache tokens

Checkpoints: task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

- Implemented `median_trap` analyzer rule in `internal/analyzer`:
  - Evaluates `OVER` yardage legs (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`) with a matched player (`PlayerID != ""`) and at least 3 games in the season log.
  - Computes the median of the player's last 5 games and the season mean across all games.
  - Triggers a `Warn` alert when `*leg.Line > median`. Formats message: `"{Player}'s last-5 median is {median} yards, so {line} is a stretch."`
  - When `*leg.Line < mean`, appends `" His {mean}-yard average is propped up by one big game."`
- Table-driven unit tests in `internal/analyzer/analyzer_test.go`:
  - Covered line above median, line at or below median, fewer than 3 games, unmatched player, `UNDER` side, and non-yardage markets (`RECEPTIONS`, `PASS_TDS`).
  - Verified presence and absence of the propped-up average sentence based on `line < mean`.
- Stored game log wiring in `internal/web`:
  - Added `playerStatsForLegs` to fetch stored `player_gamelogs` for all distinct matched player IDs on a slip and unmarshal their stat series.
  - Wired player stats into `analyzerView` in `internal/web/slips.go` and `internal/web/analyzer.go`.
  - Added handler tests `TestSlipPanelBannerMedianTrap` and `TestSlipPanelBannerMedianTrapQuietCases` in `internal/web/analyzer_test.go`.
- Documentation updates:
  - Updated `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to document `median_trap`, required stats inputs, and quiet conditions.
  - Pruned the built `median_trap` section from `plan/analyzer.md`.

### Phase 4: Prop quick-add, player autocomplete, and Research tab

Tasks:

- [x] Add `GET /games/{id}/props` for on-demand prop quick-add pills on expanded game cards, player `<datalist>` autocomplete in the leg form, and automatic `player_id`/`team` resolution when saving a leg in `internal/web`, with handler tests
- [x] Build `GET /research`, `GET /research/games/{id}`, `GET /research/players/{espnID}`, and `POST /research/matches` (last-5 hit bars with median and mean marked, season games played, position-filtered key injuries, manual match fixes, and links from game cards and slip legs), with handler and template tests
- [x] Docs: update `docs/concepts/slips.md` and `docs/reference/routes.md` with prop quick-add, player autocomplete, and the research routes, and delete the built Research tab and route rows from `plan/product.md` and `plan/architecture.md`

Phase totals: 4 checkpoints, 58m16s, $—, 6385554 input / 69060 output / 6091823 cache tokens

Checkpoints: task-7, task-8, task-9, final Review fixes: 0 Integration checks: 0

#### Done

- Expanded game card prop quick-add (`GET /games/{id}/props`):
  - Fetches game prop odds on demand via `SyncGameProps` using HTMX `hx-trigger="toggle once"`.
  - Displays quick-add pills for team totals (`TEAM_TOTAL`) and player props (passing, rushing, receiving, anytime TD, kicker points) grouped by player.
  - Links game cards to `/research?game_id={id}` and player prop headers to `/research?player_id={espnID}`.
  - Quick-add button posts `POST /slips/{id}/legs` with `quick=1` and selection metadata prefilled, returning the "Added {selection}" confirmation.
- Leg form player autocomplete and leg resolution:
  - Populates player `<datalist>` (`WeekPlayerNames`) on `GET /slips/{id}/legs/new` and `GET /legs/{id}/edit` with all distinct athlete names across the week's team rosters and prop lines.
  - In `legFromForm`, automatically resolves `player_id` (ESPN athlete ID) and `team` via `ResolvePlayerMatch` when a player market leg is submitted.
  - Adds "Research &rarr;" link to leg rows on the slip view when a leg has an associated player ID or game ID.
- Research tab and partials in `internal/web/research.go` and `web/templates/research.html`:
  - `GET /research`: Full page with week game dropdown and player autocomplete selector, supporting deep links via `?game_id=`, `?player_id=`, `?leg_id=`, `?market=`, and `?line=`. Supports HTMX partial delegation targeting `#game-research` or `#player-research`.
  - `GET /research/games/{id}`: On-demand sync of game injuries, rosters, and props (`SyncGameResearch`). Renders key injuries per team filtered to positions (`QB`, `RB`, `WR`, `TE`, `LT`, `OT`, `C`, `CB`, `S`), sportsbook prop player matching status, and manual match resolution form.
  - `GET /research/players/{espnID}`: On-demand sync of player gamelog (`SyncPlayerResearch`). Renders last-5 hit bars (green over line, red under), L5 median and season mean callouts, season games played, and interactive market switcher tabs.
  - `POST /research/matches`: Saves manual match (`manual = 1`) via `SaveManualPlayerMatch`, clears conflicting non-manual matches for that game, backfills odds and leg rows, and re-renders the updated game research partial.
- Navigation and responsive layout:
  - Added Research tab to the mobile bottom navigation bar and desktop top navigation.
  - At `lg` width, responsive layout links and displays research panels cleanly beside the slip.
- Tests:
  - Handler tests in `internal/web/research_test.go` (`TestResearchPage`, `TestResearchGameHandler`, `TestResearchPlayerHandler`, `TestResearchMatchesHandler`, `TestResearchLinksOnSlipLegs`).
  - Game props handler test in `internal/web/games_test.go` (`TestGamePropsHandler`).
  - Autocomplete datalist test in `internal/web/slips_test.go` (`TestLegFormPlayerAutocompleteDatalist`).
  - Layout test in `internal/web/layout_test.go` covering navigation links.
- Documentation:
  - Updated `docs/concepts/slips.md` with prop quick-add, autocomplete, and research deep links.
  - Updated `docs/reference/routes.md` documenting `GET /games/{id}/props`, `GET /research`, `GET /research/games/{id}`, `GET /research/players/{espnID}`, and `POST /research/matches`.
  - Regenerated `docs/llms.txt`.
  - Pruned completed research tab and route entries from `plan/product.md` and `plan/architecture.md`.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
