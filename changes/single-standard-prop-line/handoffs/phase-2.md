# Phase 2: Phase 2: Collapse board prop rows to one line per player market

## Done

### Task 2: Update `internal/board/board.go` (`Build`, `Pair`, `LineKey`) to group and pair player props by `(GameID, MarketType, canonicalPlayerKey)` so every player has at most one `LineGroup` per prop market, with tests in `internal/board/board_test.go` and `internal/web/pick_test.go`.

- Updated `Build` in `internal/board/board.go` to match player prop rows against `rosterIndex` before grouping and to group player props by `(GameID, MarketType, canonicalPlayerKey, Side)` where `canonicalPlayerKey` uses `"id:" + playerID` when matched or `"name:" + normalize(playerName) + "|" + team` when unmatched.
- Updated `LineKey` and `lineKey` (used by `Pair`) in `internal/board/board.go` so player props key on `(MarketType, canonicalPlayerKey)` rather than raw `PlayerName` and `lineText(Line)`, keeping `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL` behavior unchanged.
- Added unit tests in `internal/board/board_test.go` (`TestBuildAndPairCollapsePlayerPropsAcrossBooksAndSpellings`, `TestPairOneSidedHomeBookStillPairsWithOtherBook`, `TestPairByPlayerIDAcrossDifferingNamesAndTeams`, `TestLineKeyCanonicalPlayerProps`) and updated `TestBuildAndDetailNormalizeAccentsAndTeamTags` for the collapsed `Build` output.
- Added `TestPickBoardCollapsesPlayerPropRowsAndSwapsAcrossLines` in `internal/web/pick_test.go` verifying one `pick-line-group` per player prop market and HTMX row-swap behavior across differing line values and name spellings.

### Task 3: Docs: update `docs/reference/data-sources.md` covering standard-line filtering and primary-line selection.

- Updated `docs/reference/data-sources.md` (`### Bulk props`) to document `odds_type` and `projection_type` standard-line filtering (`isStandardPropType`) and primary-line selection (`pickPrimaryPropQuote`) for multi-line books.
- Updated `docs/reference/data-sources.md` (`### Per-event props`) to note `MergeProps` deduplication of primary collisions on `(book, market_type, team, player_name, side)`.
- Verified `good-vibes docs lint` passes with 0 errors and no warnings on `docs/reference/data-sources.md`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 2

- `docs/` updates (e.g., `docs/concepts/slips.md` if `board.LineKey` description needs updating for player props) and any remaining phase tasks can proceed.

### Task 3

- Phase 2 is complete and ready for close-out.

## Files touched

- `docs/reference/data-sources.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/pick_test.go`

## Run log

- Mode: 
- Checkpoints: task-2, task-3
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
