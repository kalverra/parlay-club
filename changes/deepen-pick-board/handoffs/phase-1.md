# Phase 1: Phase 1: Move `CurrentOdds` into `internal/db` and filter in `OddsForGame` / `OddsForWeek`

## Done

### Task 1: Add `IsPropRow` and `CurrentOdds` in `internal/db/odds.go`, apply `CurrentOdds` inside `Store.OddsForGame` and `Store.OddsForWeek`, remove manual `currentOdds` calls from `internal/web`, and move `internal/web/current_odds_test.go` to `internal/db/odds_test.go` (plus add a stale-prop test in `internal/web/research_test.go`).

- Added `IsPropRow` and `CurrentOdds` in `internal/db/odds.go` and updated `board.IsPropRow` in `internal/board/board.go` to delegate to `db.IsPropRow`.
- Applied `CurrentOdds` at the storage seam inside `Store.OddsForGame` and `Store.OddsForWeek`.
- Removed `isPropRow`, `currentOdds`, and all manual `currentOdds(...)` call sites across `internal/web/games.go`, `internal/web/export.go`, `internal/web/pick.go`, and `internal/web/slips.go`.
- Moved `internal/web/current_odds_test.go` into `internal/db/odds_test.go`, added `TestStoreOddsForGameAndWeekFilterCurrentOdds`, and added `TestResearchGameDropsStalePropPlayers` in `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`Store.OddsForGame` and `Store.OddsForWeek` now return only current sync-group rows, so Phase 2 can focus purely on enriching `board.Row` with picked-leg drift state in `internal/board`.

## Files touched

- `internal/board/board.go`
- `internal/db/odds.go`
- `internal/db/odds_test.go`
- `internal/web/current_odds_test.go`
- `internal/web/export.go`
- `internal/web/games.go`
- `internal/web/pick.go`
- `internal/web/research_test.go`
- `internal/web/slips.go`

## Run log

- Mode: 
- Checkpoints: task-1
- Verify: verify passed: generate 700ms, lint changed: internal/db/odds_test.go, test 8.3s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
