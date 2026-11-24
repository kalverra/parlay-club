# Task 4: Wire `LoadBoardSignals` and `board.SelectHighlights` into the hub render path

## Done

- Added `HighlightsView`, `LegHighlightView`, `ParlayHighlightView`, `slipHighlights`, `highlightsView`, and `legHighlightView` in `internal/web/slips.go`, wiring `analyzer.LoadBoardSignals` and `board.SelectHighlights` into `slipPageData` when `slip.Status == db.SlipOpen` and `next.Step == slips.StepPick`.
- Rendered the phone-first `"Top A-grade ideas"` strip inside `#slip-panel` in `web/templates/partials.html` (below the step banner), showing up to 3 A-grade legs and up to 2 A-grade parlay combos with green `A` pills, inline `<details>` factor breakdowns, one-tap `+ Add` forms, and `"Add all N"` batch forms (or disabled `"Needs N open slots"` buttons when slots are insufficient), hidden when empty.
- Added handler tests `TestSlipHubRendersTopAGradeIdeasDuringPickStep` and `TestSlipHubHighlightsHideAndDisableWhenSlotsFullOrLocked` in `internal/web/slips_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`HighlightsView`, `LegHighlightView`, `ParlayHighlightView`, `highlightsView`, and `legHighlightView` in `internal/web/slips.go` can be reused directly for `#pick-detail`'s `"Top plays in this game"` strip in Phase 3.

## Commit

Render Top A-grade ideas strip on the slip hub

Surface A-grade single legs and parlay combos from cached board signals on the slip hub during the viewer's pick step, with one-tap single and batch add actions.
