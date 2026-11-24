# Phase 1: Phase 1: Board signal loader and pure highlight selector

## Done

### Task 1: Add `LoadBoardSignals` to `internal/analyzer/signals.go` (sharing internal helpers with `LoadSlipSignals`) with tests in `internal/analyzer/signals_test.go`.

- Extracted shared `loadSignals` helper in [signals.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/analyzer/signals.go) used by both `LoadSlipSignals` and `LoadBoardSignals`.
- Added `LoadBoardSignals` taking `games []db.Game` and `playerIDs []string`, with `distinctNonEmptyIDs` to deduplicate and skip empty player IDs.
- Added `TestLoadBoardSignals` in [signals_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/analyzer/signals_test.go) covering batched signal loading, sharp benchmarks, injury merging, ID deduplication, and empty-input fail-soft behavior.

### Task 2: Implement `RowToAnalyzerLeg`, `LegHighlight`, `ParlayHighlight`, and `SelectHighlights` in `internal/board/highlights.go` with unit tests and a full-slate benchmark in `internal/board/highlights_test.go`.

- Implemented `RowToAnalyzerLeg`, `LegHighlight`, `ParlayHighlight`, `HighlightOptions`, `HighlightSet`, and `SelectHighlights` in [highlights.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/board/highlights.go).
- Added deterministic ranking, per-player and same-line-group deduplication, and bounded 2- and 3-leg parlay combo generation (top 12 A-grade legs plus up to 6 B-grade stack partners for `stack_shootout` and `stack_clock_drain`).
- Added unit tests and `BenchmarkSelectHighlightsFullSlate` in [highlights_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/board/highlights_test.go) (~3.25 ms/op across 16 games / ~1,632 rows on Apple M3 Max).
- Verified `golangci-lint run ./internal/board/... --fix` (0 issues) and `go test -short ./internal/board/...` pass.

## Deviations

### Task 2

- `board.Row` stores American odds in `Row.Odds` rather than `Row.Price`, so ranking and `parlay.CombinedOdds` use `Row.Odds`.
- Negative values in `HighlightOptions.MaxLegs`, `MaxParlays`, and `MaxPerPlayer` explicitly disable that cap (0 legs/parlays or unlimited per player), while `0` selects the design defaults (`3`, `2`, `1`).

## Open issues

None.

## Next-phase notes

### Task 1

`LoadBoardSignals` deduplicates and filters empty strings from `playerIDs` internally, so callers in `internal/board` or `internal/web` can pass raw player ID slices collected from `[]board.Row`.

### Task 2

`SelectHighlights` is pure and expects `games` and `stats` from `analyzer.LoadBoardSignals`. In `internal/web`, pass `SameGameOnly: true` and `MaxParlays: 1` for `#pick-detail`, and `ExcludePicked: true` on both hub and game-detail views.

## Files touched

- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/docs.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/board/highlights.go`
- `internal/board/highlights_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: generate 1s, lint 8.3s, test 8.9s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
