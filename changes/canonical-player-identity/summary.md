---
slug: canonical-player-identity
run: 20261117-014019-canonical-player-identity
outcome: done
generated: 2026-11-17T02:05:56Z
---

# Run summary: canonical-player-identity

## Outcome

- Status: done
- Branch: canonical-player-identity
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 2
- Checkpoints: 5
- Duration: 17m30s
- Cost: $—
- Fresh input tokens: 170962
- Cache read tokens: 1697794
- Cache write tokens: 0
- Output tokens: 18460

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 1887216 | $— |

## Phases

### Phase 1: Unify name normalization and `(TEAM)` stripping

Tasks:

- [x] Add `StripPlayerTag` and accent folding to `NormalizePlayerName` in `internal/db/players.go`, and delegate `internal/board.normalize` to it, with unit tests in `internal/db/players_test.go` and `internal/board/board_test.go`.

Phase totals: 1 checkpoints, 2m21s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `StripPlayerTag` and accent folding to `NormalizePlayerName` in `internal/db/players.go`, and delegate `internal/board.normalize` to it, with unit tests in `internal/db/players_test.go` and `internal/board/board_test.go`.

- Added `StripPlayerTag(name string) (cleaned, teamHint string)` in `internal/db/players.go` to strip trailing parenthetical tags and extract 2-4 letter uppercase team hints.
- Updated `NormalizePlayerName` in `internal/db/players.go` to strip trailing parenthetical tags via `StripPlayerTag` and fold common Latin accents via `plainRune` before punctuation and suffix removal.
- Delegated `internal/board.normalize` directly to `db.NormalizePlayerName` and removed the duplicate `foldAccents` and `plainRune` helpers from `internal/board/board.go`.
- Added unit tests for `StripPlayerTag` and accented/tagged names in `internal/db/players_test.go`, and board normalization/grouping tests in `internal/board/board_test.go`.

### Phase 2: Canonicalize prop ingest and backfill stored rows

Tasks:

- [x] Batch-load `roster_players` and `player_matches` once per game in `internal/refresh/research.go` (`mapPropsToOdds`), rewrite each row's `PlayerName` and `Team` to the canonical player (unifying unmatched casing per game and preferring linked rows on same-book collisions), and test with `"Ollie Gordon"` / `"Ollie Gordon II"` / `"Ollie Gordon II (MIA)"` fixtures.
- [x] Add a schema/data backfill (migration `0013` + Go helper) to deduplicate existing `odds` and backfill `legs.player_id`/`team`, with a migration test on colliding prod-shaped rows.
- [x] Docs: update `docs/reference/data-sources.md` and `docs/reference/schema.md` to document canonical player normalization and `(TEAM)` tag stripping.

Phase totals: 4 checkpoints, 15m9s, $—, 1868756 input / 18460 output / 1697794 cache read / 0 cache write tokens

Checkpoints: task-2, task-3, task-4, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
