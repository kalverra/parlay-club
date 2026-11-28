## Done

- Added `GET /bets/add-sheet` (`betAddSheet`, `BetAddSheetView`, `betAddSheetHref`) in `internal/web/research.go`, registered `"bet-add-sheet"` in `internal/web/render.go`, and added the `"bet-add-sheet"` partial in `web/templates/partials.html` to list eligible open slips in `StepPick` or render an empty/kicked-off state with a `+ New slip` link.
- Mounted `<dialog id="bet-add-dialog" class="leg-sheet">` with `#bet-add-dialog-body` in `web/templates/layout.html` and wired open/close/reset/HTMX `afterSwap` and `slip-updated` handlers in `web/static/app.js`.
- Wired `+ Add` buttons on Research Explorer cards (`web/templates/partials.html`), Related Bets (`web/templates/research_bet.html`, posting directly to `POST /slips/{id}/legs` when `slip_id` is an eligible active slip and opening `#bet-add-dialog` otherwise), and cross-slip leg rows in `slip-panel` (`internal/web/slips.go`, `web/templates/partials.html`).
- Added `TestBetAddSheetListsEligibleSlipsAndEmptyState` and `TestAddButtonsOnResearchCardsAndRelatedBets` in `internal/web/research_test.go`, plus `TestSlipHubCrossSlipLegAddButtons` in `internal/web/slips_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`GET /bets/add-sheet`, `#bet-add-dialog`, and universal `+ Add` actions are in place across Research Explorer, Related Bets, and cross-slip leg rows.
