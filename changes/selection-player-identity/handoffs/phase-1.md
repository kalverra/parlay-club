# Phase 1: Phase 1: Unify player normalization and resolution across `db`, `grader`, `board`, and `research`

## Done

### Task 1: Extend `db.ResolvedPlayer` with `Position` and `HeadshotURL`, refactor `db.Store.ResolvePlayerMatch` in `internal/db/players.go` to delegate to `db.BatchPlayerResolver` (honoring `StripPlayerTag` team hints), and update `loadGameResearch` in `internal/web/research.go` and `matchPlayer` in `internal/board/board.go` to use it, with tests in `internal/db/players_test.go` and `internal/web/research_test.go`.

- Extended `db.ResolvedPlayer` in `internal/db/canonical_players.go` with `Position` and `HeadshotURL`, populated from `RosterPlayer` on both cache hits and roster matches.
- Refactored `db.Store.ResolvePlayerMatch` in `internal/db/players.go` to load `RosterForTeams` and `PlayerMatchesForTeams`, delegate to `db.BatchPlayerResolver.ResolveGamePlayer`, and persist `NewMatches()`, honoring `StripPlayerTag` team hints and collision safety.
- Updated `loadGameResearch` in `internal/web/research.go` to resolve prop players in memory via `db.NewBatchPlayerResolver` (persisting any `NewMatches()`) instead of calling `Store.ResolvePlayerMatch` per row.
- Updated `matchPlayer` in `internal/board/board.go` to extract `teamHint` via `db.StripPlayerTag` when `team == ""`.
- Added unit and handler tests in `internal/db/players_test.go` and `internal/web/research_test.go` covering `ResolvedPlayer` fields, `(TEAM)` hint disambiguation, and in-memory research matching.

### Task 2: Replace `normalizePlayerName` and `IsPlayerMarket` in `internal/grader/grader.go` with `db.NormalizePlayerName` and `db.IsPlayerMarket`, and add accent-folded and `(TEAM)`-tagged prop grading tests in `internal/grader/grader_test.go`.

- Removed `IsPlayerMarket` and `normalizePlayerName` from [internal/grader/grader.go](internal/grader/grader.go) and replaced them with `db.IsPlayerMarket` and `db.NormalizePlayerName` (also extracting `teamHint` via `db.StripPlayerTag` when `leg.Team` is empty).
- Updated [internal/refresh/grade.go](internal/refresh/grade.go) and [docs/concepts/data-refresh.md](docs/concepts/data-refresh.md) to reference `db.IsPlayerMarket` instead of `grader.IsPlayerMarket`.
- Added `TestGradeLeg_PlayerNameNormalization` in [internal/grader/grader_test.go](internal/grader/grader_test.go) covering accent-folded name matching and `(TEAM)`-tagged prop disambiguation, and updated `TestIsPlayerMarket` to call `db.IsPlayerMarket`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `db.BatchPlayerResolver` and `db.Store.ResolvePlayerMatch` now share the same resolution and `StripPlayerTag` team-hint semantics.
- Remaining work in Phase 1 is unifying `internal/grader/grader.go` (`normalizePlayerName` / `IsPlayerMarket`) with `db.NormalizePlayerName` and `db.IsPlayerMarket`.

### Task 2

- Phase 1 (unifying player normalization and resolution across `db`, `grader`, `board`, and `research`) is complete; Phase 2 can proceed with `SameLineGroup`, `SamePlayer`, `MatchBookOdds`, and moved-line selection checks.

## Files touched

- `docs/concepts/data-refresh.md`
- `internal/board/board.go`
- `internal/db/canonical_players.go`
- `internal/db/players.go`
- `internal/db/players_test.go`
- `internal/grader/grader.go`
- `internal/grader/grader_test.go`
- `internal/refresh/grade.go`
- `internal/web/research.go`
- `internal/web/research_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 400ms, lint 2.5s, test 8.3s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
