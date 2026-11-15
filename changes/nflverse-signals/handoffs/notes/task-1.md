# Task 1: `internal/ingest/nflverse` CSV clients

## Done

- Built `internal/ingest/nflverse` (`client.go`, `teams.go`, `players.go`, `team_stats.go`, `player_stats.go`, `snap_counts.go`) with header-based CSV parsing, `User-Agent: parlay-club`, 30s default timeout, and injected `*http.Client`.
- Implemented `Players`, `TeamStats`, `PlayerStats`, and `SnapCounts` returning plain structs (`Player`, `TeamWeek`, `PlayerWeek`, `SnapCount`) with ESPN team-code normalization (`LA`→`LAR`, `WAS`→`WSH`, plus historical `OAK`→`LV`, `SD`→`LAC`, `STL`→`LAR`, `JAC`→`JAX`) and opponent-row joining in `TeamStats`.
- Saved real 2026 `nflverse-data` CSV fixtures in `internal/ingest/nflverse/testdata/` (`stats_team_week_2026.csv`, `stats_player_week_2026.csv`, `snap_counts_2026.csv`, `players.csv`, and `README.md`).
- Added unit and fixture tests in `internal/ingest/nflverse/nflverse_test.go` covering all 4 endpoints, all 32 ESPN team codes, reordered columns, postseason game types, missing columns, blank optional cells, and bad numeric errors.

## Deviations

- `snap_counts_{season}.csv` names the season-type column `game_type` (with values `REG`, `WC`, `DIV`, `CON`, `SB`) rather than `season_type`; `SnapCounts` reads `game_type` and maps both `REG`/`POST`/`WC`/`DIV`/`CON`/`SB`/`PRE` to ESPN integer `SeasonType` (`1`/`2`/`3`).
- In `stats_player_week_{season}.csv`, position-specific EPA/share cells (such as `receiving_epa` for QBs) are blank rather than `0`, and one row in week 9 has a blank `player_id`; `PlayerStats` drops rows missing identity cells (`player_id`, `team`, `season`, `week`, `season_type`) and defaults blank/`NA` optional stat cells to `0`.

## Open issues

- None. All 2026 `nflverse` CSVs (`players.csv` 7.0 MiB, `stats_player_week_2026.csv` 4.0 MiB, `snap_counts_2026.csv` 1.1 MiB, `stats_team_week_2026.csv` 109 KiB) are currently under `archive.MaxBodySize` (8 MiB).

## Next-phase notes

- `nflverse.SeasonTypeRegular` (`2`) and `nflverse.SeasonTypePostseason` (`3`) match `weeks.season_type` directly for `0012_nflverse_signals.sql` and `internal/refresh`.
- `TeamStats` already joins each game's opponent row onto `OppPassingEPA`, `OppRushingEPA`, `OppAttempts`, `OppCarries`, `OppPassingYards`, and `OppRushingYards`, so `internal/refresh` can compute `def_pass_epa_allowed`, `def_rush_epa_allowed`, `plays_per_game`, and `pass_rate` directly from `TeamWeek`.
