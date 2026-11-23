## Done

- Added `LinkGroup` and `BookPlan.Groups` / `BookPlan.MultiGameNote` in `internal/export/export.go` with per-event grouping in `draftKingsCombiner.CombineGroups` when linked DraftKings legs span multiple games.
- Added `ManualLeg.PropsSynced` in `internal/export/export.go` so `ReasonNoLine` on player and team-total prop legs indicates whether the game already has stored prop rows for other markets or books.
- Added unit tests in `internal/export/export_test.go` covering single-game vs multi-game DraftKings grouping, first-appearance order, missing `Sid` handling, and `ReasonNoLine` `PropsSynced` detection.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`internal/web/export.go` and `web/templates/partials.html` can now render `BookPlan.MultiGameNote`, `BookPlan.Groups` (per-game combined taps and single-leg taps), and `ManualLeg.PropsSynced` for `ReasonNoLine` manual notes.

## Commit

Add multi-game DraftKings grouping and synced-prop flags

DraftKings' combined betslip URL only loads selections from a single event, so multi-game slips need per-game grouping and an explanation for the user. Distinguishing synced-but-unquoted props on ReasonNoLine manual legs also lets the UI explain why a prop is missing without blaming an unsynced game.
