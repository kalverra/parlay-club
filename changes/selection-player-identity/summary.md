---
slug: selection-player-identity
run: 20261118-021809-selection-player-identity
outcome: done
generated: 2026-11-18T02:36:07Z
---

# Run summary: selection-player-identity

## Outcome

- Status: done
- Branch: selection-player-identity
- Close-out: drafted
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 3
- Checkpoints: 5
- Duration: 14m30s
- Cost: $—
- Fresh input tokens: 0
- Cache read tokens: 0
- Cache write tokens: 0
- Output tokens: 0

## Phases

### Phase 1: Unify player normalization and resolution across `db`, `grader`, `board`, and `research`

Tasks:

- [x] Extend `db.ResolvedPlayer` with `Position` and `HeadshotURL`, refactor `db.Store.ResolvePlayerMatch` in `internal/db/players.go` to delegate to `db.BatchPlayerResolver` (honoring `StripPlayerTag` team hints), and update `loadGameResearch` in `internal/web/research.go` and `matchPlayer` in `internal/board/board.go` to use it, with tests in `internal/db/players_test.go` and `internal/web/research_test.go`.
- [x] Replace `normalizePlayerName` and `IsPlayerMarket` in `internal/grader/grader.go` with `db.NormalizePlayerName` and `db.IsPlayerMarket`, and add accent-folded and `(TEAM)`-tagged prop grading tests in `internal/grader/grader_test.go`.

Phase totals: 2 checkpoints, 6m6s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Unify selection, line-group, and book-odds matching

Tasks:

- [x] Add `SamePlayer`, `SameLineGroup`, and `MatchBookOdds` in `internal/db/odds.go`, and wire them into `internal/board/signals.go` (`rowMatchesAnalyzerLeg`, `sameBoardPlayer`), `internal/web/pick.go` (`sameBoardLine`), `internal/export/export.go` (`matchOdds`), and `internal/web/slips.go` (`candidatePrices`), with tests in `internal/db/odds_test.go`, `internal/board/signals_test.go`, and `internal/web/slips_test.go`.

Phase totals: 1 checkpoints, 3m12s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `SamePlayer`, `SameLineGroup`, and `MatchBookOdds` in `internal/db/odds.go`, and wire them into `internal/board/signals.go` (`rowMatchesAnalyzerLeg`, `sameBoardPlayer`), `internal/web/pick.go` (`sameBoardLine`), `internal/export/export.go` (`matchOdds`), and `internal/web/slips.go` (`candidatePrices`), with tests in `internal/db/odds_test.go`, `internal/board/signals_test.go`, and `internal/web/slips_test.go`.

- Added `SamePlayer`, `SameLineGroup`, and `MatchBookOdds` in `internal/db/odds.go` and refactored `SameSelection` to delegate player matching to `SamePlayer`.
- Wired `db.SamePlayer` and `db.SameSelection` into `internal/board/signals.go` (`sameBoardPlayer`, `rowMatchesAnalyzerLeg`) and removed the now-unused `lineEqual` helper from `internal/board/board.go`.
- Wired `db.SameLineGroup` into `internal/web/pick.go` (`sameBoardLine`).
- Wired `db.MatchBookOdds` into `internal/export/export.go` (replacing `matchOdds`) and `internal/web/slips.go` (`candidatePrices`).
- Added unit and handler tests in `internal/db/odds_test.go`, `internal/board/signals_test.go`, and `internal/web/slips_test.go`.

### Phase 3: Centralize market labels and selection text formatting

Tasks:

- [x] Add `MarketLabel`, `FormatSelection`, and `BuildLegSelection` in `internal/db/odds.go` and replace the duplicate implementations in `internal/board/board.go`, `internal/web/slips.go`, and `internal/web/research.go`, with unit tests in `internal/db/odds_test.go`.
- [x] Docs: update `docs/concepts/slips.md` and `docs/reference/data-sources.md` to reflect unified selection/book matching and player resolution across grading and research.

Phase totals: 2 checkpoints, 4m32s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Add `MarketLabel`, `FormatSelection`, and `BuildLegSelection` in `internal/db/odds.go` and replace the duplicate implementations in `internal/board/board.go`, `internal/web/slips.go`, and `internal/web/research.go`, with unit tests in `internal/db/odds_test.go`.

- Added `MarketLabel`, `FormatSelection`, and `BuildLegSelection` (with shared `titleSide`, `strictTitleSide`, `signedLine`, and `formatLine` helpers) in `internal/db/odds.go`.
- Replaced `rowText`, `marketLabel`, and `titleSide` in `internal/board/board.go` with calls to `db.FormatSelection` and `db.MarketLabel`.
- Replaced `buildSelection` and `marketLabels` in `internal/web/slips.go` with `db.BuildLegSelection`.
- Replaced `marketLabel` in `internal/web/research.go` with `db.MarketLabel` (falling back to the raw market string when empty).
- Added unit tests `TestMarketLabel`, `TestFormatSelection`, and `TestBuildLegSelection` in `internal/db/odds_test.go`.

### Task 5: Docs: update `docs/concepts/slips.md` and `docs/reference/data-sources.md` to reflect unified selection/book matching and player resolution across grading and research.

- Updated [docs/concepts/slips.md](docs/concepts/slips.md) to document unified `db.SameSelection`, `db.SamePlayer`, `db.SameLineGroup`, `db.MatchBookOdds` (used by both sportsbook export and slip reference odds), shared selection formatting (`db.MarketLabel`, `db.FormatSelection`, `db.BuildLegSelection`), in-memory game research resolution via `db.BatchPlayerResolver`, and grader player-name normalization (`db.IsPlayerMarket`, `db.StripPlayerTag`, `db.NormalizePlayerName`).
- Updated [docs/reference/data-sources.md](docs/reference/data-sources.md) under **Player matching** to document `db.BatchPlayerResolver` as the single player-matching engine across prop ingest (`mapPropsToOdds`), game research (`loadGameResearch`), and `Store.ResolvePlayerMatch`, plus `ResolvedPlayer` metadata (`Position`, `HeadshotURL`) and board/grader alignment.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
