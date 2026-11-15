## Done

- Added migration `internal/db/migrations/0012_nflverse_signals.sql` creating `nflverse_player_ids`, `nflverse_team_weeks`, and `nflverse_player_weeks`, and updated `internal/db/migrate_test.go` and `internal/db/migrate_internal_test.go` to schema version 12.
- Added `internal/db/nflverse.go` with idempotent upserts (`UpsertNFLversePlayerIDs`, `UpsertNFLverseTeamWeeks`, `UpsertNFLversePlayerWeeks`) and season/player queries (`NFLversePlayerIDByESPN`, `NFLverseTeamWeeksForSeason`, `NFLversePlayerWeeksForPlayer`, `NFLversePlayerWeeksForSeason`), plus unit tests in `internal/db/nflverse_test.go`.
- Added the `nflverse:season:{year}` sync source in `internal/refresh/nflverse.go`, `internal/refresh/rules.go`, and `internal/refresh/refresh.go` (6h window on Tue/Wed/game days, 24h otherwise, skipped once all games are final and last sync is > 12h after the latest kickoff), joining `Players`, `TeamStats`, `PlayerStats`, and `SnapCounts` onto ESPN IDs.
- Wired `nflverse.NewClient` into `cmd/server/main.go` and `cmd/server/sandbox.go`, added the 4 real 2026 CSV fixtures to `internal/sandbox/registry.go`, added the new tables to `internal/sandbox/reset.go`, and updated `internal/sandbox/sandbox_test.go` and `cmd/server/sandbox_test.go`.
- Added unit, staleness, failure-backoff, and end-to-end sandbox-fixture refresh tests in `internal/refresh/nflverse_test.go`; `good-vibes verify lint` and `good-vibes verify test` both passed.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `db.Store` exposes `NFLverseTeamWeeksForSeason(ctx, season, seasonType)` (ordered `week_num ASC, team ASC`), `NFLversePlayerWeeksForPlayer(ctx, espnID, season, seasonType)` (ordered `week_num DESC`), and `NFLversePlayerWeeksForSeason(ctx, season, seasonType)` (ordered `week_num DESC, espn_id ASC`) for `internal/web` to aggregate `TeamDefenseProfile` and `PlayerUsage` in Phase 2/3.
- `refresh.NFLverseSource(season)` writes the `sync_state` key `nflverse:season:{year}` so `internal/web` can enforce freshness checks (e.g., nil when missing or stale) just like `nfelo:projections`.
