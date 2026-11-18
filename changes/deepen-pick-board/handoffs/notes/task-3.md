# Task 3

## Done

- Simplified `pickPageData`, `pickSlateData`, `pickDetailData`, and `buildPickSlate` in `internal/web/pick.go` to use `board.BuildSlate`, consuming `OpenGame`, `ResolvedGameID`, `WindowCounts`, `GameCard.Unloaded`, and slate state flags directly.
- Updated `windowChips`, `pickGameCardView`, and `buildPickDetail` to consume `board.Slate.WindowCounts` and `board.GameCard.Unloaded` without rescanning `rows`.
- Simplified `dockFrom`, `pickDetailView`, `pickLineGroupView`, and `pickRowView` in `internal/web/pick.go` to read `Drifted`, `BoardDriftNote`, and `TrayDriftNote` directly from `board.Row`.
- Simplified `pickPairRow` in `internal/web/pick.go` to call `board.LineGroupForLeg` and removed the redundant local `legMap`, `legDrifted`, `boardDriftNote`, `trayDriftNote`, and `groupHoldsLeg` helpers.
- Verified with `go test -short -race ./internal/web/...` and `golangci-lint run ./internal/web/...` (0 issues).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`internal/web/pick.go` now delegates slate resolution, drift detection/formatting, and line-group lookup to `internal/board`, ready for final docs/verification in the next task.
