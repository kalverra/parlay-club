# Phase 2: Phase 2: Deepen `board.Build`, `board.BuildSlate`, and `board.LineGroupForLeg`

## Done

### Task 2: Extend `board.Row` with picked-leg drift fields (`PickedLine`, `PickedOdds`, `Drifted`, `BoardDriftNote`, `TrayDriftNote`) in `board.Build`, and add `BuildSlate`, `LineGroup.HoldsLeg`, and `LineGroupForLeg` in `internal/board/board.go`, with unit tests in `internal/board/board_test.go`.

- Extended `board.Row` in `internal/board/board.go` with `PickedLine`, `PickedOdds`, `Drifted`, `BoardDriftNote`, and `TrayDriftNote`, and populated them in `board.Build` when binding `mine` legs.
- Added `GameCard.Unloaded`, `board.Slate`, and `board.BuildSlate` in `internal/board/board.go` to resolve `OpenGame` / `ResolvedGameID`, track `WindowCounts`, and compute `Started`, `NoGames`, `AllStarted`, `NoLines`, and `Empty`.
- Added `(LineGroup).HoldsLeg` and `board.LineGroupForLeg` in `internal/board/board.go` to apply `BoardSignals`, pair game rows, and locate the line group matching a picked or just-deleted leg.
- Added unit tests in `internal/board/board_test.go` (`TestBuildPopulatesPickedDriftFields`, `TestBuildSlateResolvesOpenGameUnloadedAndStates`, `TestLineGroupForLegAndHoldsLeg`).

### Task 3: Simplify `pickPageData`, `pickSlateData`, `pickDetailData`, `buildPickSlate`, `dockFrom`, `pickRowView`, and `pickPairRow` in `internal/web/pick.go` to use the deepened `board` types and functions, and verify with `internal/web/pick_test.go`.

- Simplified `pickPageData`, `pickSlateData`, `pickDetailData`, and `buildPickSlate` in `internal/web/pick.go` to use `board.BuildSlate`, consuming `OpenGame`, `ResolvedGameID`, `WindowCounts`, `GameCard.Unloaded`, and slate state flags directly.
- Updated `windowChips`, `pickGameCardView`, and `buildPickDetail` to consume `board.Slate.WindowCounts` and `board.GameCard.Unloaded` without rescanning `rows`.
- Simplified `dockFrom`, `pickDetailView`, `pickLineGroupView`, and `pickRowView` in `internal/web/pick.go` to read `Drifted`, `BoardDriftNote`, and `TrayDriftNote` directly from `board.Row`.
- Simplified `pickPairRow` in `internal/web/pick.go` to call `board.LineGroupForLeg` and removed the redundant local `legMap`, `legDrifted`, `boardDriftNote`, `trayDriftNote`, and `groupHoldsLeg` helpers.
- Verified with `go test -short -race ./internal/web/...` and `golangci-lint run ./internal/web/...` (0 issues).

### Task 4: Docs: update `docs/concepts/slips.md` if board/drift mechanics are referenced.

- Updated `docs/concepts/slips.md` to reflect `db.CurrentOdds` filtering at the storage seam (`Store.OddsForWeek` / `Store.OddsForGame`), picked-leg drift fields on `board.Row` (`PickedLine`, `PickedOdds`, `Drifted`, `BoardDriftNote`, `TrayDriftNote`), `board.BuildSlate`, and `board.LineGroupForLeg` / `LineGroup.HoldsLeg`.
- Replaced the stale `legDrifted` reference in `docs/concepts/slips.md` with `board.Row.Drifted`, `BoardDriftNote`, and `TrayDriftNote`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 2

`board.Build`, `board.BuildSlate`, `LineGroup.HoldsLeg`, and `board.LineGroupForLeg` are ready in `internal/board/board.go`. Task 3 can now simplify `internal/web/pick.go` (`pickPageData`, `pickSlateData`, `pickDetailData`, `buildPickSlate`, `dockFrom`, `pickRowView`, and `pickPairRow`) to consume these fields and functions directly and remove the local `legDrifted`, `boardDriftNote`, `trayDriftNote`, and `groupHoldsLeg` helpers.

### Task 3

`internal/web/pick.go` now delegates slate resolution, drift detection/formatting, and line-group lookup to `internal/board`, ready for final docs/verification in the next task.

### Task 4

Phase 2 is complete and ready for the binary verification gates.

## Files touched

- `docs/concepts/slips.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`

## Run log

- Mode: 
- Checkpoints: task-2, task-3, task-4
- Verify: verify passed: test 300ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
