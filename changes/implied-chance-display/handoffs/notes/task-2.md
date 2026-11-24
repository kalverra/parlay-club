## Done

- Added `ImpliedChance` to `LegView` and `LegHighlightView`, `CombinedImplied` to `ParlayHighlightView`, and `CombinedImplied` / `ReferenceImplied` to `PayoutView` in `internal/web/slips.go`, populated via `parlay.ImpliedProb`, `parlay.DecimalImpliedProb`, and `formatImpliedChance`.
- Updated `web/templates/partials.html` to render muted implied percentage chances beside leg odds, payout combined/reference odds, and A-grade leg and parlay highlights, omitting them when empty.
- Added handler tests `TestSlipHubRendersImpliedChancesAndOmitsEmptyOrVoided` and `TestSlipHubHighlightsRenderImpliedChances` in `internal/web/slips_test.go` covering leg rows, payout combined and reference odds, A-grade highlights, and empty/all-voided omission.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Pick-board views (`PickRowView`, `TrayLegView`, `PickDockView`) in `internal/web/pick.go` and their template blocks in `web/templates/partials.html` remain for the next task.
- `formatImpliedChance` in `internal/web/slips.go` and `parlay.DecimalImpliedProb` in `internal/parlay/parlay.go` are ready for `internal/web/pick.go`.

## Commit

Show implied chance on slip hub legs and payouts

People read American odds faster when paired with an implied percentage chance. Surfacing these percentages on slip hub leg rows, the payout calculator, and A-grade highlights gives the group immediate intuition without changing grades or payouts.
