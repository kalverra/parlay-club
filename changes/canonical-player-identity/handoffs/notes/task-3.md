# Task 3: Migration `0013` and canonical player backfill

## Done

- Added [0013_canonical_players.sql](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrations/0013_canonical_players.sql) and wired a version-13 transaction hook in [migrate.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrate.go) so `db.Migrate` runs the Go backfill atomically during migration.
- Implemented `BackfillCanonicalPlayers` / `backfillCanonicalPlayersTx` in [canonical_backfill.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/canonical_backfill.go) to refresh `roster_players.norm_name` with accent folding, deduplicate colliding `odds` rows (preferring linked and richer link/sid/event_sid rows, deleting losers before updating winners), unify unmatched casing per game, backfill `legs.player_id`/`team`/`player_name`, and upsert auto-discovered `player_matches`.
- Updated `latestSchemaVersion` to `13` in [migrate_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrate_test.go) and [migrate_internal_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrate_internal_test.go), and added `TestMigration0013CanonicalPlayers` covering colliding prod-shaped `"Ollie Gordon"` / `"Ollie Gordon II"` / `"Ollie Gordon II (MIA)"` rows, unmatched casing variants, accented past-week `legs`, and idempotency.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Migration `0013` and `db.BackfillCanonicalPlayers` are in place; existing `odds` and `legs` rows are canonicalized on `db.Migrate`.
