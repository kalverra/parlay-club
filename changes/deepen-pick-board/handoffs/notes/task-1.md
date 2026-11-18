# Task 1: Move `CurrentOdds` into `internal/db` and filter in `OddsForGame` / `OddsForWeek`

## Done

- Added `IsPropRow` and `CurrentOdds` in `internal/db/odds.go` and updated `board.IsPropRow` in `internal/board/board.go` to delegate to `db.IsPropRow`.
- Applied `CurrentOdds` at the storage seam inside `Store.OddsForGame` and `Store.OddsForWeek`.
- Removed `isPropRow`, `currentOdds`, and all manual `currentOdds(...)` call sites across `internal/web/games.go`, `internal/web/export.go`, `internal/web/pick.go`, and `internal/web/slips.go`.
- Moved `internal/web/current_odds_test.go` into `internal/db/odds_test.go`, added `TestStoreOddsForGameAndWeekFilterCurrentOdds`, and added `TestResearchGameDropsStalePropPlayers` in `internal/web/research_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`Store.OddsForGame` and `Store.OddsForWeek` now return only current sync-group rows, so Phase 2 can focus purely on enriching `board.Row` with picked-leg drift state in `internal/board`.
