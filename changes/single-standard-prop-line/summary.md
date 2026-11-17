---
slug: single-standard-prop-line
run: 20261117-023105-single-standard-prop-line
outcome: done
generated: 2026-11-17T02:53:17Z
---

# Run summary: single-standard-prop-line

## Outcome

- Status: done
- Branch: single-standard-prop-line
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 3 of 3
- Phases: 2
- Checkpoints: 4
- Duration: 11m34s
- Cost: $—
- Fresh input tokens: 110380
- Cache read tokens: 631378
- Cache write tokens: 0
- Output tokens: 8112

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 749870 | $— |

## Phases

### Phase 1: Filter non-standard lines in prop ingest

Tasks:

- [x] Parse `odds_type` / `projection_type` in `internal/ingest/odds/props.go`, drop non-`"standard"` DFS projections (`goblin`, `demon`), and pick the single balanced primary line when a book returns multiple unlabeled lines, with tests in `internal/ingest/odds/props_test.go` using prod payload shapes.

Phase totals: 1 checkpoints, 3m30s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Parse `odds_type` / `projection_type` in `internal/ingest/odds/props.go`, drop non-`"standard"` DFS projections (`goblin`, `demon`), and pick the single balanced primary line when a book returns multiple unlabeled lines, with tests in `internal/ingest/odds/props_test.go` using prod payload shapes.

- Extended `propResponse` in [props.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/single-standard-prop-line/internal/ingest/odds/props.go) with `odds_type` and `projection_type` and added `isStandardPropType` to drop non-`"standard"` DFS projections (`goblin`, `demon`, `alt`, case-insensitive).
- Grouped non-`ANYTIME_TD` quotes in `parseProps` by `(book, marketType, normalizePropSubject(subject))` (merging split over/under items at the same line) and added `pickPrimaryPropQuote` / `betterPropQuote` to keep only the single most balanced primary line (two-sided over one-sided, minimum `|impliedProb(over) - impliedProb(under)|`, tie-breaking toward `-110` and smallest price spread).
- Updated `MergeProps` to deduplicate duplicate `(book, marketType, team, playerName, side)` rows in `primary`, preferring linked rows and then prices closest to `-110`.
- Added `TestPropsFiltersNonStandardDFSAndSelectsPrimaryLines` and `TestPropsPrimaryLineSelectionEdgeCases` in [props_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/single-standard-prop-line/internal/ingest/odds/props_test.go) covering PrizePicks/Underdog DFS tags, Novig 6-line and Bovada 5-line ladders, split over/under quotes, tie-breaking, and `ANYTIME_TD` filtering.

### Phase 2: Collapse board prop rows to one line per player market

Tasks:

- [x] Update `internal/board/board.go` (`Build`, `Pair`, `LineKey`) to group and pair player props by `(GameID, MarketType, canonicalPlayerKey)` so every player has at most one `LineGroup` per prop market, with tests in `internal/board/board_test.go` and `internal/web/pick_test.go`.
- [x] Docs: update `docs/reference/data-sources.md` covering standard-line filtering and primary-line selection.

Phase totals: 3 checkpoints, 8m4s, $—, 741758 input / 8112 output / 631378 cache read / 0 cache write tokens

Checkpoints: task-2, task-3, final Review fixes: 0 Integration checks: 0

#### Done

### Task 2: Update `internal/board/board.go` (`Build`, `Pair`, `LineKey`) to group and pair player props by `(GameID, MarketType, canonicalPlayerKey)` so every player has at most one `LineGroup` per prop market, with tests in `internal/board/board_test.go` and `internal/web/pick_test.go`.

- Updated `Build` in `internal/board/board.go` to match player prop rows against `rosterIndex` before grouping and to group player props by `(GameID, MarketType, canonicalPlayerKey, Side)` where `canonicalPlayerKey` uses `"id:" + playerID` when matched or `"name:" + normalize(playerName) + "|" + team` when unmatched.
- Updated `LineKey` and `lineKey` (used by `Pair`) in `internal/board/board.go` so player props key on `(MarketType, canonicalPlayerKey)` rather than raw `PlayerName` and `lineText(Line)`, keeping `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL` behavior unchanged.
- Added unit tests in `internal/board/board_test.go` (`TestBuildAndPairCollapsePlayerPropsAcrossBooksAndSpellings`, `TestPairOneSidedHomeBookStillPairsWithOtherBook`, `TestPairByPlayerIDAcrossDifferingNamesAndTeams`, `TestLineKeyCanonicalPlayerProps`) and updated `TestBuildAndDetailNormalizeAccentsAndTeamTags` for the collapsed `Build` output.
- Added `TestPickBoardCollapsesPlayerPropRowsAndSwapsAcrossLines` in `internal/web/pick_test.go` verifying one `pick-line-group` per player prop market and HTMX row-swap behavior across differing line values and name spellings.

### Task 3: Docs: update `docs/reference/data-sources.md` covering standard-line filtering and primary-line selection.

- Updated `docs/reference/data-sources.md` (`### Bulk props`) to document `odds_type` and `projection_type` standard-line filtering (`isStandardPropType`) and primary-line selection (`pickPrimaryPropQuote`) for multi-line books.
- Updated `docs/reference/data-sources.md` (`### Per-event props`) to note `MergeProps` deduplication of primary collisions on `(book, market_type, team, player_name, side)`.
- Verified `good-vibes docs lint` passes with 0 errors and no warnings on `docs/reference/data-sources.md`.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: no commands configured
- Open issues: none
