## Done

- Extended `internal/ingest/espn/gamelog.go` with `GameLogEntry` and `GameLog.Entries` decoded from the ESPN `events` map in lockstep with `Markets`.
- Added `internal/db/migrations/0015_research_context.sql` with `player_gamelogs.games_json`, the 12 additive `nflverse_player_weeks` opponent/position/stat columns, and `idx_nflverse_player_weeks_opp_pos`, updating migration tests to schema version 15.
- Updated `internal/db/players.go` (`PlayerGameLog`, `UpsertPlayerGameLog`, `scanPlayerGameLog`) and `internal/refresh/research.go` (`syncGameLog`) to persist and read `games_json`.
- Added unit tests against saved ESPN fixtures (`gamelog.json`, `gamelog_chase.json`, `gamelog_burrow.json`, `gamelog_mcpherson.json`), `db.TestPlayerGameLogs`, `db.TestMigration0015ResearchContext`, and `refresh.TestSyncPlayerResearchGameLog`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `nflverse_player_weeks` columns (`opponent`, `player_name`, `position`, `pass_att`, `pass_yds`, `pass_tds`, `rush_yds`, `rush_tds`, `receptions`, `rec_yds`, `rec_tds`, `kicker_pts`) and `idx_nflverse_player_weeks_opp_pos` are now in place for Task 2's nflverse ingest and `internal/db/nflverse.go` updates.
