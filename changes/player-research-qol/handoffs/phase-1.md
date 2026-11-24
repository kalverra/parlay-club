# Phase 1: Phase 1: Persist ESPN game-log opponents and nflverse weekly matchup stats

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `nflverse_player_weeks` columns (`opponent`, `player_name`, `position`, `pass_att`, `pass_yds`, `pass_tds`, `rush_yds`, `rush_tds`, `receptions`, `rec_yds`, `rec_tds`, `kicker_pts`) and `idx_nflverse_player_weeks_opp_pos` are now in place for Task 2's nflverse ingest and `internal/db/nflverse.go` updates.

### Task 2

- `NFLversePlayerWeeksAgainstOpponent` and `NFLversePlayerWeeksForPositionTypes` normalize `FB`/`HB` to `RB` on both write and query, and `buildNFLversePlayerWeeks` computes `KickerPts` as `3*FGMade + PATMade`.

### Task 3

- Schema and data-source reference docs now reflect `0015_research_context.sql`, `player_gamelogs.games_json`, and the expanded `nflverse_player_weeks` columns; Phase 2 can proceed with analyzer defense ranks and `PlayerResearchSignals` updates.

## Files touched

- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0015_research_context.sql`
- `internal/db/nflverse.go`
- `internal/db/nflverse_test.go`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/ingest/espn/gamelog.go`
- `internal/ingest/espn/gamelog_test.go`
- `internal/ingest/nflverse/nflverse_test.go`
- `internal/ingest/nflverse/player_stats.go`
- `internal/refresh/nflverse.go`
- `internal/refresh/nflverse_test.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 600ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
