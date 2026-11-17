# Phase 1: Phase 1: Canonical selection identity in `db` and `board`

## Done

### Task 1: Add `SameSelection` in `internal/db/odds.go` (matching props by canonical player + market + side, ignoring `Line`) and update `board.Row.MatchesLeg`, `sameBoardLine`, and `groupHoldsLeg` in `internal/board/board.go` and `internal/web/pick.go` so picked legs stay bound when lines move, with tests.

- Added `SelectionKey`, `IsPlayerMarket`, `LegSelection`, `OddsSelection`, and `SameSelection` in `internal/db/odds.go`, and updated `Odds.MatchesSelection` to delegate to `SameSelection`.
- Updated `board.Row.MatchesLeg` in `internal/board/board.go` to match via `db.SameSelection` (ignoring `Line` for both player props and game markets) and delegated `isPlayerMarket` in `internal/board/board.go` and `internal/web/slips.go` to `db.IsPlayerMarket`.
- Updated `sameBoardLine` in `internal/web/pick.go` to match board line groups via `db.SameSelection` (with `Side` cleared for props and `TEAM_TOTAL`, and matching `GameID` + `MarketType` for `SPREAD`, `ML`, `TOTAL`) so picked legs stay bound to their `LineGroup` when lines move (`groupHoldsLeg` continues delegating to `option.MatchesLeg`).
- Added unit tests in `internal/db/odds_test.go`, `internal/board/board_test.go`, and `internal/web/pick_test.go`.

## Deviations

### Task 1

- `db.SameSelection` takes `(left, right SelectionKey)` with `LegSelection` / `OddsSelection` helpers instead of `(leg Leg, o Odds)` because `db.Odds` has no `PlayerID` field while `board.Row` and `db.Leg` do; using `SelectionKey` lets `Odds.MatchesSelection`, `board.Row.MatchesLeg`, and `sameBoardLine` share one implementation without losing `PlayerID`.

## Open issues

### Task 1

- None.

## Next-phase notes

### Task 1

- Phase 2 (`internal/export/export.go`) can use `row.MatchesSelection(leg)` (which now delegates to `db.SameSelection`) and remove the `ReasonLineMoved` rejection when `row.Link` is valid, populating `LineMoved: !db.SameLine(row.Line, leg.Line)`.
- Phase 3 (`internal/slips/rules.go`) can reuse `db.SameSelection(db.LegSelection(existing), db.LegSelection(updated))` when deciding whether an open-slip `UpdateLeg` should preserve `leg_votes`.

## Files touched

- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/db/odds.go`
- `internal/db/odds_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: lint reused, changed: internal/web/pick_test.go, test reused
- Rounds used: 1 of 2; verify failures: 0
