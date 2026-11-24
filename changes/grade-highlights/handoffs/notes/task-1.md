# Task 1: Add LoadBoardSignals

## Done

- Extracted shared `loadSignals` helper in [signals.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/analyzer/signals.go) used by both `LoadSlipSignals` and `LoadBoardSignals`.
- Added `LoadBoardSignals` taking `games []db.Game` and `playerIDs []string`, with `distinctNonEmptyIDs` to deduplicate and skip empty player IDs.
- Added `TestLoadBoardSignals` in [signals_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/analyzer/signals_test.go) covering batched signal loading, sharp benchmarks, injury merging, ID deduplication, and empty-input fail-soft behavior.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`LoadBoardSignals` deduplicates and filters empty strings from `playerIDs` internally, so callers in `internal/board` or `internal/web` can pass raw player ID slices collected from `[]board.Row`.

## Commit

Add LoadBoardSignals for board-wide scoring

Share the batched SQLite signal loader between slip scoring and board-wide highlight selection so every A-grade leg and parlay candidate is evaluated against the exact same cached data without extra queries.
