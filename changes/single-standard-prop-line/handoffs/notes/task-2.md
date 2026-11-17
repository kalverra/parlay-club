# Task 2

## Done

- Updated `Build` in `internal/board/board.go` to match player prop rows against `rosterIndex` before grouping and to group player props by `(GameID, MarketType, canonicalPlayerKey, Side)` where `canonicalPlayerKey` uses `"id:" + playerID` when matched or `"name:" + normalize(playerName) + "|" + team` when unmatched.
- Updated `LineKey` and `lineKey` (used by `Pair`) in `internal/board/board.go` so player props key on `(MarketType, canonicalPlayerKey)` rather than raw `PlayerName` and `lineText(Line)`, keeping `SPREAD`, `ML`, `TOTAL`, and `TEAM_TOTAL` behavior unchanged.
- Added unit tests in `internal/board/board_test.go` (`TestBuildAndPairCollapsePlayerPropsAcrossBooksAndSpellings`, `TestPairOneSidedHomeBookStillPairsWithOtherBook`, `TestPairByPlayerIDAcrossDifferingNamesAndTeams`, `TestLineKeyCanonicalPlayerProps`) and updated `TestBuildAndDetailNormalizeAccentsAndTeamTags` for the collapsed `Build` output.
- Added `TestPickBoardCollapsesPlayerPropRowsAndSwapsAcrossLines` in `internal/web/pick_test.go` verifying one `pick-line-group` per player prop market and HTMX row-swap behavior across differing line values and name spellings.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `docs/` updates (e.g., `docs/concepts/slips.md` if `board.LineKey` description needs updating for player props) and any remaining phase tasks can proceed.
