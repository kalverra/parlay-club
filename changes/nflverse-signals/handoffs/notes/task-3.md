## Done

- Updated `docs/reference/data-sources.md` with the `nflverse` client (`Players`, `TeamStats`, `PlayerStats`, `SnapCounts`), 30s default timeout, header-based parsing, season-type and ESPN team-code normalization, and 8 MiB archive behavior.
- Updated `docs/reference/schema.md` for `0012_nflverse_signals.sql` (schema version 12), adding `nflverse_player_ids`, `nflverse_team_weeks`, and `nflverse_player_weeks` to the summary, migrations, tables, DDL, and global table conventions.
- Updated `docs/concepts/data-refresh.md` for `nflverse:season:{year}` in `freeSourcesFor`, its staleness/skip rules (6h Tue/Wed/game days, 24h otherwise; skipped when all games are final and last sync > 12h after latest kickoff), and post-commit `week:{weekID}` publishing.
- Updated `docs/reference/sandbox-replay.md` with the 4 `nflverse` week 9 fixtures (104 total across 4 stages: 61 captured, 43 derived), reset table list, startup sync, and test coverage, and ran `good-vibes docs lint` and `good-vibes docs index` (`docs/llms.txt`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 (`internal/analyzer` and `internal/web/analyzer.go`) can wire `db.NFLverseTeamWeeksForSeason` and `db.NFLversePlayerWeeksForSeason` / `NFLversePlayerWeeksForPlayer` into `TeamDefenseProfile` and `PlayerUsage`, add the 4 new rules, `ScoreLeg`/`ScoreSlip`, and `GET /analyzer/guide`.
