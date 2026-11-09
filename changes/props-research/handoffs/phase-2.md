# Phase 2: On-demand prop refresh and ESPN research syncs

## Done

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

## Deviations

- In `SyncGameResearch`, if a game already kicked off, props are skipped as expected; `SyncGameResearch` explicitly refreshes stale home and away rosters so team rosters remain available post-kickoff on the research view.

## Open issues

- None.

## Next-phase notes

- Phase 3 implements the `median_trap` analyzer rule in `internal/analyzer` and store-to-analyzer wiring in `internal/web`:
  - Add `median_trap` rule in `internal/analyzer` with threshold tests (`line > L5 median`, `line <= median`, `< 3` games, non-yardage markets, unmatched players, and the `line < mean` sentence).
  - Wire stored `player_gamelogs` into `internal/web/analyzer.go` so slips evaluate against real player stats.
  - Update `docs/reference/analyzer-rules.md` and `docs/concepts/analyzer.md`, and remove the built `median_trap` section from `plan/analyzer.md`.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/refresh/refresh.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`
- `internal/refresh/sources.go`
- `plan/data-sources.md`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
