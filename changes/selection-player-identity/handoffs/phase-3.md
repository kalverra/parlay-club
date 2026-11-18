# Phase 3: Phase 3: Centralize market labels and selection text formatting

## Done

### Task 4: Add `MarketLabel`, `FormatSelection`, and `BuildLegSelection` in `internal/db/odds.go` and replace the duplicate implementations in `internal/board/board.go`, `internal/web/slips.go`, and `internal/web/research.go`, with unit tests in `internal/db/odds_test.go`.

- Added `MarketLabel`, `FormatSelection`, and `BuildLegSelection` (with shared `titleSide`, `strictTitleSide`, `signedLine`, and `formatLine` helpers) in `internal/db/odds.go`.
- Replaced `rowText`, `marketLabel`, and `titleSide` in `internal/board/board.go` with calls to `db.FormatSelection` and `db.MarketLabel`.
- Replaced `buildSelection` and `marketLabels` in `internal/web/slips.go` with `db.BuildLegSelection`.
- Replaced `marketLabel` in `internal/web/research.go` with `db.MarketLabel` (falling back to the raw market string when empty).
- Added unit tests `TestMarketLabel`, `TestFormatSelection`, and `TestBuildLegSelection` in `internal/db/odds_test.go`.

### Task 5: Docs: update `docs/concepts/slips.md` and `docs/reference/data-sources.md` to reflect unified selection/book matching and player resolution across grading and research.

- Updated [docs/concepts/slips.md](docs/concepts/slips.md) to document unified `db.SameSelection`, `db.SamePlayer`, `db.SameLineGroup`, `db.MatchBookOdds` (used by both sportsbook export and slip reference odds), shared selection formatting (`db.MarketLabel`, `db.FormatSelection`, `db.BuildLegSelection`), in-memory game research resolution via `db.BatchPlayerResolver`, and grader player-name normalization (`db.IsPlayerMarket`, `db.StripPlayerTag`, `db.NormalizePlayerName`).
- Updated [docs/reference/data-sources.md](docs/reference/data-sources.md) under **Player matching** to document `db.BatchPlayerResolver` as the single player-matching engine across prop ingest (`mapPropsToOdds`), game research (`loadGameResearch`), and `Store.ResolvePlayerMatch`, plus `ResolvedPlayer` metadata (`Position`, `HeadshotURL`) and board/grader alignment.

## Deviations

### Task 4

- `internal/web/research.go` previously used long market names (`"Passing Yards"`, `"Anytime Touchdown"`, `"Kicker Points"`) in its local `marketLabel`, whereas `db.MarketLabel` uses the shared short labels (`"Pass Yds"`, `"Anytime TD"`, `"Kicking Pts"`). Replaced `research.marketLabel` with `db.MarketLabel` per the design.

## Open issues

None.

## Next-phase notes

### Task 4

- Phase 3 market label and selection formatting centralization is in place for Task 5 (verification/docs).

### Task 5

- All Phase 3 tasks (including docs) are complete.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/data-sources.md`
- `internal/board/board.go`
- `internal/db/odds.go`
- `internal/db/odds_test.go`
- `internal/web/research.go`
- `internal/web/slips.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
