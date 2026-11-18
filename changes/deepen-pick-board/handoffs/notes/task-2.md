# Task 2

## Done

- Extended `board.Row` in `internal/board/board.go` with `PickedLine`, `PickedOdds`, `Drifted`, `BoardDriftNote`, and `TrayDriftNote`, and populated them in `board.Build` when binding `mine` legs.
- Added `GameCard.Unloaded`, `board.Slate`, and `board.BuildSlate` in `internal/board/board.go` to resolve `OpenGame` / `ResolvedGameID`, track `WindowCounts`, and compute `Started`, `NoGames`, `AllStarted`, `NoLines`, and `Empty`.
- Added `(LineGroup).HoldsLeg` and `board.LineGroupForLeg` in `internal/board/board.go` to apply `BoardSignals`, pair game rows, and locate the line group matching a picked or just-deleted leg.
- Added unit tests in `internal/board/board_test.go` (`TestBuildPopulatesPickedDriftFields`, `TestBuildSlateResolvesOpenGameUnloadedAndStates`, `TestLineGroupForLegAndHoldsLeg`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`board.Build`, `board.BuildSlate`, `LineGroup.HoldsLeg`, and `board.LineGroupForLeg` are ready in `internal/board/board.go`. Task 3 can now simplify `internal/web/pick.go` (`pickPageData`, `pickSlateData`, `pickDetailData`, `buildPickSlate`, `dockFrom`, `pickRowView`, and `pickPairRow`) to consume these fields and functions directly and remove the local `legDrifted`, `boardDriftNote`, `trayDriftNote`, and `groupHoldsLeg` helpers.
