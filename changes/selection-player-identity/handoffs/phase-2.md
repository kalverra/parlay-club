# Phase 2: Phase 2: Unify selection, line-group, and book-odds matching

## Done

### Task 3: Add `SamePlayer`, `SameLineGroup`, and `MatchBookOdds` in `internal/db/odds.go`, and wire them into `internal/board/signals.go` (`rowMatchesAnalyzerLeg`, `sameBoardPlayer`), `internal/web/pick.go` (`sameBoardLine`), `internal/export/export.go` (`matchOdds`), and `internal/web/slips.go` (`candidatePrices`), with tests in `internal/db/odds_test.go`, `internal/board/signals_test.go`, and `internal/web/slips_test.go`.

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

### Task 3

- Phase 2 (unifying selection, line-group, and book-odds matching) is complete; Phase 3 can proceed with centralizing `MarketLabel`, `FormatSelection`, and `BuildLegSelection` in `internal/db/odds.go`.

## Files touched

- `internal/board/board.go`
- `internal/board/signals.go`
- `internal/board/signals_test.go`
- `internal/db/odds.go`
- `internal/db/odds_test.go`
- `internal/export/export.go`
- `internal/web/pick.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`

## Run log

- Mode: 
- Checkpoints: task-3
- Verify: verify passed: generate 300ms, lint 5.3s, test 8.2s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
