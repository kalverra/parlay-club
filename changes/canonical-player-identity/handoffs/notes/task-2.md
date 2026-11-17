## Done

- Added `Store.UpsertPlayerMatches` in `internal/db/players.go` to batch-upsert auto-discovered `player_matches` rows in one transaction while preserving existing `manual = 1` overrides.
- Replaced per-row `ResolvePlayerMatch` calls in `internal/refresh/research.go` (`mapPropsToOdds`) with a per-game `batchPlayerResolver` that preloads `RosterForTeams` and `PlayerMatchesForTeams` once, strips `(TEAM)` tags via `db.StripPlayerTag`, prefers `manual = 1` then `player_matches` then `roster_players` (disambiguating by `teamHint` when it matches home/away), and batch-upserts newly discovered `player_matches`.
- Rewrote matched rows' `PlayerName` and `Team` to the canonical `RosterPlayer.FullName` and resolved team; for unmatched rows, preserved valid `teamHint` and unified casing per game batch by `db.NormalizePlayerName`.
- Deduplicated same-book collisions on `(game_id, book, market_type, team, player_name, side)` in `mapPropsToOdds`, preferring linked rows and then richer link/sid/event_sid rows.
- Added `TestUpsertPlayerMatches` in `internal/db/players_test.go` and `TestSyncPropsCanonicalizesOllieGordonVariants` / `TestMapPropsToOddsUnmatchedCasingAndCollisions` in `internal/refresh/research_test.go` (including `board.Detail` single-card verification).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `mapPropsToOdds` now writes canonical `PlayerName` and `Team` for new prop ingests; the next task still needs to migrate/backfill any pre-existing `odds` and `legs` rows in SQLite.
