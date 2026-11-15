# Phase 1: Phase 1: Ingest, SQLite storage, refresh wiring, and sandbox fixtures

## Done

### Task 1: Build `internal/ingest/nflverse` CSV clients (`Players`, `TeamStats`, `PlayerStats`, `SnapCounts`) with ESPN team-code mapping and header-based parsing, plus tests against real saved CSV fixtures.

- Built `internal/ingest/nflverse` (`client.go`, `teams.go`, `players.go`, `team_stats.go`, `player_stats.go`, `snap_counts.go`) with header-based CSV parsing, `User-Agent: parlay-club`, 30s default timeout, and injected `*http.Client`.
- Implemented `Players`, `TeamStats`, `PlayerStats`, and `SnapCounts` returning plain structs (`Player`, `TeamWeek`, `PlayerWeek`, `SnapCount`) with ESPN team-code normalization (`LA`→`LAR`, `WAS`→`WSH`, plus historical `OAK`→`LV`, `SD`→`LAC`, `STL`→`LAR`, `JAC`→`JAX`) and opponent-row joining in `TeamStats`.
- Saved real 2026 `nflverse-data` CSV fixtures in `internal/ingest/nflverse/testdata/` (`stats_team_week_2026.csv`, `stats_player_week_2026.csv`, `snap_counts_2026.csv`, `players.csv`, and `README.md`).
- Added unit and fixture tests in `internal/ingest/nflverse/nflverse_test.go` covering all 4 endpoints, all 32 ESPN team codes, reordered columns, postseason game types, missing columns, blank optional cells, and bad numeric errors.

### Task 3: Docs: update `docs/reference/data-sources.md`, `docs/reference/schema.md`, `docs/concepts/data-refresh.md`, and `docs/reference/sandbox-replay.md` for `nflverse` sources and tables.

- Updated `docs/reference/data-sources.md` with the `nflverse` client (`Players`, `TeamStats`, `PlayerStats`, `SnapCounts`), 30s default timeout, header-based parsing, season-type and ESPN team-code normalization, and 8 MiB archive behavior.
- Updated `docs/reference/schema.md` for `0012_nflverse_signals.sql` (schema version 12), adding `nflverse_player_ids`, `nflverse_team_weeks`, and `nflverse_player_weeks` to the summary, migrations, tables, DDL, and global table conventions.
- Updated `docs/concepts/data-refresh.md` for `nflverse:season:{year}` in `freeSourcesFor`, its staleness/skip rules (6h Tue/Wed/game days, 24h otherwise; skipped when all games are final and last sync > 12h after latest kickoff), and post-commit `week:{weekID}` publishing.
- Updated `docs/reference/sandbox-replay.md` with the 4 `nflverse` week 9 fixtures (104 total across 4 stages: 61 captured, 43 derived), reset table list, startup sync, and test coverage, and ran `good-vibes docs lint` and `good-vibes docs index` (`docs/llms.txt`).

## Deviations

### Task 1

- `snap_counts_{season}.csv` names the season-type column `game_type` (with values `REG`, `WC`, `DIV`, `CON`, `SB`) rather than `season_type`; `SnapCounts` reads `game_type` and maps both `REG`/`POST`/`WC`/`DIV`/`CON`/`SB`/`PRE` to ESPN integer `SeasonType` (`1`/`2`/`3`).
- In `stats_player_week_{season}.csv`, position-specific EPA/share cells (such as `receiving_epa` for QBs) are blank rather than `0`, and one row in week 9 has a blank `player_id`; `PlayerStats` drops rows missing identity cells (`player_id`, `team`, `season`, `week`, `season_type`) and defaults blank/`NA` optional stat cells to `0`.

## Open issues

### Task 1

- None. All 2026 `nflverse` CSVs (`players.csv` 7.0 MiB, `stats_player_week_2026.csv` 4.0 MiB, `snap_counts_2026.csv` 1.1 MiB, `stats_team_week_2026.csv` 109 KiB) are currently under `archive.MaxBodySize` (8 MiB).

## Next-phase notes

### Task 1

- `nflverse.SeasonTypeRegular` (`2`) and `nflverse.SeasonTypePostseason` (`3`) match `weeks.season_type` directly for `0012_nflverse_signals.sql` and `internal/refresh`.
- `TeamStats` already joins each game's opponent row onto `OppPassingEPA`, `OppRushingEPA`, `OppAttempts`, `OppCarries`, `OppPassingYards`, and `OppRushingYards`, so `internal/refresh` can compute `def_pass_epa_allowed`, `def_rush_epa_allowed`, `plays_per_game`, and `pass_rate` directly from `TeamWeek`.

### Task 3

- Phase 2 (`internal/analyzer` and `internal/web/analyzer.go`) can wire `db.NFLverseTeamWeeksForSeason` and `db.NFLversePlayerWeeksForSeason` / `NFLversePlayerWeeksForPlayer` into `TeamDefenseProfile` and `PlayerUsage`, add the 4 new rules, `ScoreLeg`/`ScoreSlip`, and `GET /analyzer/guide`.

## Files touched

- `cmd/server/main.go`
- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/data-refresh.md`
- `docs/llms.txt`
- `docs/reference/data-sources.md`
- `docs/reference/sandbox-replay.md`
- `docs/reference/schema.md`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0012_nflverse_signals.sql`
- `internal/db/nflverse.go`
- `internal/db/nflverse_test.go`
- `internal/ingest/nflverse/client.go`
- `internal/ingest/nflverse/nflverse_test.go`
- `internal/ingest/nflverse/player_stats.go`
- `internal/ingest/nflverse/players.go`
- `internal/ingest/nflverse/snap_counts.go`
- `internal/ingest/nflverse/team_stats.go`
- `internal/ingest/nflverse/teams.go`
- `internal/ingest/nflverse/testdata/README.md`
- `internal/ingest/nflverse/testdata/players.csv`
- `internal/ingest/nflverse/testdata/snap_counts_2026.csv`
- `internal/ingest/nflverse/testdata/stats_player_week_2026.csv`
- `internal/ingest/nflverse/testdata/stats_team_week_2026.csv`
- `internal/refresh/nflverse.go`
- `internal/refresh/nflverse_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/rules.go`
- `internal/sandbox/registry.go`
- `internal/sandbox/reset.go`
- `internal/sandbox/sandbox_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-3
- Verify: verify passed: lint reused, test reused
- Rounds used: 1 of 2; verify failures: 0
