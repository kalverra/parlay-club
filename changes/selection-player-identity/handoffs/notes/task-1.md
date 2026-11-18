# Task 1

## Done

- Extended `db.ResolvedPlayer` in `internal/db/canonical_players.go` with `Position` and `HeadshotURL`, populated from `RosterPlayer` on both cache hits and roster matches.
- Refactored `db.Store.ResolvePlayerMatch` in `internal/db/players.go` to load `RosterForTeams` and `PlayerMatchesForTeams`, delegate to `db.BatchPlayerResolver.ResolveGamePlayer`, and persist `NewMatches()`, honoring `StripPlayerTag` team hints and collision safety.
- Updated `loadGameResearch` in `internal/web/research.go` to resolve prop players in memory via `db.NewBatchPlayerResolver` (persisting any `NewMatches()`) instead of calling `Store.ResolvePlayerMatch` per row.
- Updated `matchPlayer` in `internal/board/board.go` to extract `teamHint` via `db.StripPlayerTag` when `team == ""`.
- Added unit and handler tests in `internal/db/players_test.go` and `internal/web/research_test.go` covering `ResolvedPlayer` fields, `(TEAM)` hint disambiguation, and in-memory research matching.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `db.BatchPlayerResolver` and `db.Store.ResolvePlayerMatch` now share the same resolution and `StripPlayerTag` team-hint semantics.
- Remaining work in Phase 1 is unifying `internal/grader/grader.go` (`normalizePlayerName` / `IsPlayerMarket`) with `db.NormalizePlayerName` and `db.IsPlayerMarket`.
