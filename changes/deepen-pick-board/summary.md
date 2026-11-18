---
slug: deepen-pick-board
run: 20261118-131126-deepen-pick-board
outcome: done
generated: 2026-11-18T13:28:42Z
---

# Run summary: deepen-pick-board

## Outcome

- Status: done
- Branch: deepen-pick-board
- Close-out: drafted
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 2
- Checkpoints: 4
- Duration: 13m40s
- Cost: $—
- Fresh input tokens: 0
- Cache read tokens: 0
- Cache write tokens: 0
- Output tokens: 0

## Phases

### Phase 1: Move `CurrentOdds` into `internal/db` and filter in `OddsForGame` / `OddsForWeek`

Tasks:

- [x] Add `IsPropRow` and `CurrentOdds` in `internal/db/odds.go`, apply `CurrentOdds` inside `Store.OddsForGame` and `Store.OddsForWeek`, remove manual `currentOdds` calls from `internal/web`, and move `internal/web/current_odds_test.go` to `internal/db/odds_test.go` (plus add a stale-prop test in `internal/web/research_test.go`).

Phase totals: 1 checkpoints, 3m54s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `IsPropRow` and `CurrentOdds` in `internal/db/odds.go`, apply `CurrentOdds` inside `Store.OddsForGame` and `Store.OddsForWeek`, remove manual `currentOdds` calls from `internal/web`, and move `internal/web/current_odds_test.go` to `internal/db/odds_test.go` (plus add a stale-prop test in `internal/web/research_test.go`).

- Added `IsPropRow` and `CurrentOdds` in `internal/db/odds.go` and updated `board.IsPropRow` in `internal/board/board.go` to delegate to `db.IsPropRow`.
- Applied `CurrentOdds` at the storage seam inside `Store.OddsForGame` and `Store.OddsForWeek`.
- Removed `isPropRow`, `currentOdds`, and all manual `currentOdds(...)` call sites across `internal/web/games.go`, `internal/web/export.go`, `internal/web/pick.go`, and `internal/web/slips.go`.
- Moved `internal/web/current_odds_test.go` into `internal/db/odds_test.go`, added `TestStoreOddsForGameAndWeekFilterCurrentOdds`, and added `TestResearchGameDropsStalePropPlayers` in `internal/web/research_test.go`.

### Phase 2: Deepen `board.Build`, `board.BuildSlate`, and `board.LineGroupForLeg`

Tasks:

- [x] Extend `board.Row` with picked-leg drift fields (`PickedLine`, `PickedOdds`, `Drifted`, `BoardDriftNote`, `TrayDriftNote`) in `board.Build`, and add `BuildSlate`, `LineGroup.HoldsLeg`, and `LineGroupForLeg` in `internal/board/board.go`, with unit tests in `internal/board/board_test.go`.
- [x] Simplify `pickPageData`, `pickSlateData`, `pickDetailData`, `buildPickSlate`, `dockFrom`, `pickRowView`, and `pickPairRow` in `internal/web/pick.go` to use the deepened `board` types and functions, and verify with `internal/web/pick_test.go`.
- [x] Docs: update `docs/concepts/slips.md` if board/drift mechanics are referenced.

Phase totals: 3 checkpoints, 8m54s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-2, task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
