# Task 3

## Done

- Added `SamePlayer`, `SameLineGroup`, and `MatchBookOdds` in `internal/db/odds.go` and refactored `SameSelection` to delegate player matching to `SamePlayer`.
- Wired `db.SamePlayer` and `db.SameSelection` into `internal/board/signals.go` (`sameBoardPlayer`, `rowMatchesAnalyzerLeg`) and removed the now-unused `lineEqual` helper from `internal/board/board.go`.
- Wired `db.SameLineGroup` into `internal/web/pick.go` (`sameBoardLine`).
- Wired `db.MatchBookOdds` into `internal/export/export.go` (replacing `matchOdds`) and `internal/web/slips.go` (`candidatePrices`).
- Added unit and handler tests in `internal/db/odds_test.go`, `internal/board/signals_test.go`, and `internal/web/slips_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 (unifying selection, line-group, and book-odds matching) is complete; Phase 3 can proceed with centralizing `MarketLabel`, `FormatSelection`, and `BuildLegSelection` in `internal/db/odds.go`.
