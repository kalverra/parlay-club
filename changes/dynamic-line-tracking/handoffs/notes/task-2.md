# Task 2: Dynamic sportsbook export links with drift callout

## Done

- Updated `internal/export/export.go` (`LegLink.LineMoved`, `planLeg`, `matchOdds`) to resolve deep links by `db.SameSelection` via `row.MatchesSelection(leg)` across line moves and player name variants, preferring an exact line match when multiple rows match and setting `LineMoved: !db.SameLine(row.Line, leg.Line)` instead of rejecting moved lines with `ReasonLineMoved`.
- Added `LineMoved` and `DriftNote` (`"Book at <live> · slip <saved>"`) to `ExportLinkView` in `internal/web/export.go` and rendered the drift callout in `web/templates/partials.html` for both per-leg links and combined `OpenURL` betslip links.
- Updated existing export tests and added prod Leg 19 (`"Ollie Gordon"` -> `"Ollie Gordon II"`) and prod Leg 20 (`248.5` -> `254.5` with `LineMoved: true` and combined betslip URL) coverage in `internal/export/export_test.go` and `internal/web/export_test.go`.

## Deviations

- None.

## Open issues

- None.

## Next-phase notes

- Phase 3 can now focus on board selection controls (`internal/board/board.go`, `internal/web/pick.go`), one-tap open-slip line refresh, and preserving `leg_votes` on line-drift updates in `internal/slips/rules.go`.
