# Task 4

## Done

- Added `ImpliedChance` to `PickRowView` and `TrayLegView`, and `CombinedImplied` to `PickDockView` in `internal/web/pick.go`, populated via `formatImpliedChance` and `parlay.ImpliedProb` / `parlay.DecimalImpliedProb`.
- Updated `web/templates/partials.html` to render implied chances on all four pick option button states (`tap to remove`, `tap to switch`, `swap in`, `tap to add`), on tray leg rows, and on the fixed pick dock when legs are present.
- Added `TestPickBoardTrayAndDockRenderImpliedChances` in `internal/web/pick_test.go` covering add/remove/switch/swap option rows, tray legs, and dock combined chance (including empty dock omission).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Next task should update `docs/concepts/slips.md` to document implied percentage chances across the slip hub and pick board surfaces.

## Commit

Show implied chances on pick board, tray, and dock

Surface the market-implied probability beside prices on pick option buttons, tray leg rows, and the fixed pick dock so members can gauge single-leg and combined parlay likelihoods while building a slip.
