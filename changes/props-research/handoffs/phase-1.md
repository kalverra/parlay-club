# Phase 1: ESPN rosters, game logs, injuries, and player matching

## Done

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

## Deviations

- Added `OT` to the allowed key positions in `ParseInjuries` alongside `LT` because ESPN labels offensive tackles as `OT` in game summaries.

## Open issues

- None.

## Next-phase notes

- Phase 2 wires these endpoints and store methods into `internal/refresh`:
  - `OddsPropsSource(gameID)` (`odds:props:{game}`): lazy fetch with a 2-hour cache window, checking Parlay API credit floor (150) and The Odds API fallback/enrichment floors, calling `SyncRoster` before matching props.
  - `SummarySource(gameID)` (`espn:summary:{game}`): 2-hour cache storing raw injuries JSON into `games.injuries_json`.
  - `GameLogSource(athleteID)` (`espn:gamelog:{id}`): 12-hour cache upserting `player_gamelogs`.
  - Service entry points `SyncGameProps`, `SyncGameResearch`, `SyncPlayerResearch`, and background gamelog sync for slip legs in `Kick()`.

## Files touched

- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/games.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0007_props_research.sql`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/db/sync_test.go`
- `internal/ingest/espn/client.go`
- `internal/ingest/espn/gamelog.go`
- `internal/ingest/espn/gamelog_test.go`
- `internal/ingest/espn/live_test.go`
- `internal/ingest/espn/roster.go`
- `internal/ingest/espn/roster_test.go`
- `internal/ingest/espn/summary.go`
- `internal/ingest/espn/summary_test.go`
- `internal/ingest/espn/testdata/README.md`
- `internal/ingest/espn/testdata/gamelog.json`
- `internal/ingest/espn/testdata/gamelog_kicker.json`
- `internal/ingest/espn/testdata/roster.json`
- `internal/ingest/espn/testdata/summary.json`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
