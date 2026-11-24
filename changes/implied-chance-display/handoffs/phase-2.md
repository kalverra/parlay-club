# Phase 2: Phase 2: Pick board, dock, and tray display

## Done

### Task 4: Populate and render implied percentage chances on pick board option buttons (`PickRowView`), tray leg rows (`TrayLegView`), and the fixed pick dock (`PickDockView`) in `internal/web/pick.go` and `web/templates/partials.html`, with handler tests covering add/remove/switch/swap option rows, tray legs, and dock combined chance (including empty dock omission).

- Added `ImpliedChance` to `PickRowView` and `TrayLegView`, and `CombinedImplied` to `PickDockView` in `internal/web/pick.go`, populated via `formatImpliedChance` and `parlay.ImpliedProb` / `parlay.DecimalImpliedProb`.
- Updated `web/templates/partials.html` to render implied chances on all four pick option button states (`tap to remove`, `tap to switch`, `swap in`, `tap to add`), on tray leg rows, and on the fixed pick dock when legs are present.
- Added `TestPickBoardTrayAndDockRenderImpliedChances` in `internal/web/pick_test.go` covering add/remove/switch/swap option rows, tray legs, and dock combined chance (including empty dock omission).

### Task 5: Docs: update `docs/concepts/slips.md` to document implied percentage chances on the pick board, dock, and tray.

- Updated `docs/concepts/slips.md` to document implied percentage chances on pick-board option buttons (`tap to add`, `tap to remove`, `tap to switch`, and `swap in`).
- Updated the pick dock and tray section of `docs/concepts/slips.md` to document the dock's combined implied percentage chance (omitted when no legs are picked) and the tray leg rows' muted implied percentage chance.
- Tightened the dock/tray paragraph so `docs/concepts/slips.md` stays within its 550-line budget.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

- Next task should update `docs/concepts/slips.md` to document implied percentage chances across the slip hub and pick board surfaces.

## Files touched

- `docs/concepts/slips.md`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: test 800ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
