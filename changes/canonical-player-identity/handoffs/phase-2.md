# Phase 2: Phase 2: Canonicalize prop ingest and backfill stored rows

## Done

### Task 2: Batch-load `roster_players` and `player_matches` once per game in `internal/refresh/research.go` (`mapPropsToOdds`), rewrite each row's `PlayerName` and `Team` to the canonical player (unifying unmatched casing per game and preferring linked rows on same-book collisions), and test with `"Ollie Gordon"` / `"Ollie Gordon II"` / `"Ollie Gordon II (MIA)"` fixtures.

- Added `Store.UpsertPlayerMatches` in `internal/db/players.go` to batch-upsert auto-discovered `player_matches` rows in one transaction while preserving existing `manual = 1` overrides.
- Replaced per-row `ResolvePlayerMatch` calls in `internal/refresh/research.go` (`mapPropsToOdds`) with a per-game `batchPlayerResolver` that preloads `RosterForTeams` and `PlayerMatchesForTeams` once, strips `(TEAM)` tags via `db.StripPlayerTag`, prefers `manual = 1` then `player_matches` then `roster_players` (disambiguating by `teamHint` when it matches home/away), and batch-upserts newly discovered `player_matches`.
- Rewrote matched rows' `PlayerName` and `Team` to the canonical `RosterPlayer.FullName` and resolved team; for unmatched rows, preserved valid `teamHint` and unified casing per game batch by `db.NormalizePlayerName`.
- Deduplicated same-book collisions on `(game_id, book, market_type, team, player_name, side)` in `mapPropsToOdds`, preferring linked rows and then richer link/sid/event_sid rows.
- Added `TestUpsertPlayerMatches` in `internal/db/players_test.go` and `TestSyncPropsCanonicalizesOllieGordonVariants` / `TestMapPropsToOddsUnmatchedCasingAndCollisions` in `internal/refresh/research_test.go` (including `board.Detail` single-card verification).

### Task 3: Add a schema/data backfill (migration `0013` + Go helper) to deduplicate existing `odds` and backfill `legs.player_id`/`team`, with a migration test on colliding prod-shaped rows.

- Added [0013_canonical_players.sql](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrations/0013_canonical_players.sql) and wired a version-13 transaction hook in [migrate.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrate.go) so `db.Migrate` runs the Go backfill atomically during migration.
- Implemented `BackfillCanonicalPlayers` / `backfillCanonicalPlayersTx` in [canonical_backfill.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/canonical_backfill.go) to refresh `roster_players.norm_name` with accent folding, deduplicate colliding `odds` rows (preferring linked and richer link/sid/event_sid rows, deleting losers before updating winners), unify unmatched casing per game, backfill `legs.player_id`/`team`/`player_name`, and upsert auto-discovered `player_matches`.
- Updated `latestSchemaVersion` to `13` in [migrate_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrate_test.go) and [migrate_internal_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/internal/db/migrate_internal_test.go), and added `TestMigration0013CanonicalPlayers` covering colliding prod-shaped `"Ollie Gordon"` / `"Ollie Gordon II"` / `"Ollie Gordon II (MIA)"` rows, unmatched casing variants, accented past-week `legs`, and idempotency.

### Task 4: Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` to document canonical player normalization and `(TEAM)` tag stripping.

- Updated [data-sources.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/docs/reference/data-sources.md) to document `StripPlayerTag` `(TEAM)` tag stripping, accent folding in `NormalizePlayerName`, per-game batch prop resolution (`mapPropsToOdds`, `RosterForTeams`, `PlayerMatchesForTeams`, `UpsertPlayerMatches`), canonical `odds.player_name`/`odds.team` rewriting, unmatched casing unification, and same-book collision deduplication.
- Updated [schema.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/docs/reference/schema.md) to document migration `0013_canonical_players.sql` and its `backfillCanonicalPlayersTx` hook, bump the fresh database version to 13, and document canonical `odds.player_name`/`odds.team` and `roster_players.norm_name` conventions.
- Cleared three pre-existing `names-path` lint errors on [schema.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/canonical-player-identity/docs/reference/schema.md) so `good-vibes docs lint` passes with 0 errors.

## Deviations

### Task 4

Fixed three pre-existing `names-path` references in `docs/reference/schema.md` (`internal/db.Open` and `internal/db/migrations/NNNN_name.sql`) that `good-vibes docs lint` promoted from warnings to errors once the file was modified.

## Open issues

None.

## Next-phase notes

### Task 2

- `mapPropsToOdds` now writes canonical `PlayerName` and `Team` for new prop ingests; the next task still needs to migrate/backfill any pre-existing `odds` and `legs` rows in SQLite.

### Task 3

- Migration `0013` and `db.BackfillCanonicalPlayers` are in place; existing `odds` and `legs` rows are canonicalized on `db.Migrate`.

### Task 4

- Phase 2 documentation is complete and `good-vibes docs lint` passes with 0 errors.

## Files touched

- `docs/reference/data-sources.md`
- `docs/reference/schema.md`
- `internal/db/canonical_backfill.go`
- `internal/db/migrate.go`
- `internal/db/migrate_internal_test.go`
- `internal/db/migrate_test.go`
- `internal/db/migrations/0013_canonical_players.sql`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`

## Run log

- Mode: 
- Checkpoints: task-2, task-3, task-4
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
