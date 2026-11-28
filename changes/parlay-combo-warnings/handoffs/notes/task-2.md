# Task 2

## Done

- Added `Row.ComboWarning` to `internal/board/board.go` and wired `analyzer.CheckComboConflict` into `ApplySignals` in `internal/board/signals.go` for unpicked board rows.
- Mapped `Row.ComboWarning` onto `PickRowView.ComboWarning` in `internal/web/pick.go` and rendered a compact non-blocking `data-combo-warning` heads-up note in `web/templates/partials.html` (`pick-option`).
- Added unit tests in `internal/board/signals_test.go` (`TestApplySignalsComboWarning`) and `internal/web/pick_test.go` (`TestPickBoardRendersInlineComboWarningWithoutBlocking`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `Row.ComboWarning` is populated automatically in `board.ApplySignals` whenever `BoardSignals.SlipLegs` is present and `row.Picked == 0`.
- Both `pick-detail` and single-line-group HTMX refreshes (`board.LineGroupForLeg`) go through `ApplySignals`, so inline combo warnings stay current as legs are added or removed.

## Commit

Surface inline combo warnings on the pick board

Warn users directly on conflicting pick-board options before they add
them to an open slip, using the shared analyzer combo rules without
blocking any pick.
