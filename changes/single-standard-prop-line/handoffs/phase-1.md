# Phase 1: Phase 1: Filter non-standard lines in prop ingest

## Done

### Task 1: Parse `odds_type` / `projection_type` in `internal/ingest/odds/props.go`, drop non-`"standard"` DFS projections (`goblin`, `demon`), and pick the single balanced primary line when a book returns multiple unlabeled lines, with tests in `internal/ingest/odds/props_test.go` using prod payload shapes.

- Extended `propResponse` in [props.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/single-standard-prop-line/internal/ingest/odds/props.go) with `odds_type` and `projection_type` and added `isStandardPropType` to drop non-`"standard"` DFS projections (`goblin`, `demon`, `alt`, case-insensitive).
- Grouped non-`ANYTIME_TD` quotes in `parseProps` by `(book, marketType, normalizePropSubject(subject))` (merging split over/under items at the same line) and added `pickPrimaryPropQuote` / `betterPropQuote` to keep only the single most balanced primary line (two-sided over one-sided, minimum `|impliedProb(over) - impliedProb(under)|`, tie-breaking toward `-110` and smallest price spread).
- Updated `MergeProps` to deduplicate duplicate `(book, marketType, team, playerName, side)` rows in `primary`, preferring linked rows and then prices closest to `-110`.
- Added `TestPropsFiltersNonStandardDFSAndSelectsPrimaryLines` and `TestPropsPrimaryLineSelectionEdgeCases` in [props_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/single-standard-prop-line/internal/ingest/odds/props_test.go) covering PrizePicks/Underdog DFS tags, Novig 6-line and Bovada 5-line ladders, split over/under quotes, tie-breaking, and `ANYTIME_TD` filtering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `parseProps` now guarantees at most one `OVER` and one `UNDER` row per `(book, marketType, normalizePropSubject(subject))` from Parlay API `/props`.
- Phase 2 (`internal/board/board.go`) still needs to group and pair prop rows by `(MarketType, canonicalPlayerKey)` so differing lines or raw player name spellings across different books collapse into one `LineGroup` row per player and market.

## Files touched

- `internal/ingest/odds/props.go`
- `internal/ingest/odds/props_test.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: lint 7.3s, test 9s
- Rounds used: 1 of 2; verify failures: 0
