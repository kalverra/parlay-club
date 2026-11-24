# Phase 1: Phase 1: Odds math helper and slip hub display

## Done

### Task 1: Add `parlay.DecimalImpliedProb` in `internal/parlay` (with `ImpliedProb` delegating to it) and `formatImpliedChance` in `internal/web`, with unit tests covering standard prices, pushed/voided combined odds, and formatting boundaries (`<0.1%`, `0.1%`–`9.9%`, `10%`–`99%`, `>99%`, and empty/invalid inputs).

- Added `parlay.DecimalImpliedProb(decimal float64) float64` in [internal/parlay/parlay.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/parlay/parlay.go) and delegated `parlay.ImpliedProb` to it.
- Added `formatImpliedChance(prob float64) string` in [internal/web/slips.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/web/slips.go) covering `<0.1%`, one-decimal (`0.1%`–`9.9%`), whole-percent (`10%`–`99%`), `>99%`, and `""` for non-betting/non-finite inputs.
- Added unit tests in [internal/parlay/parlay_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/parlay/parlay_test.go) (`TestDecimalImpliedProb`, including pushed/voided `CombinedOdds` cases) and [internal/web/slips_unit_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/web/slips_unit_test.go) (`TestFormatImpliedChance`).

### Task 2: Populate and render implied percentage chances on slip hub leg rows (`LegView`), the payout calculator (`PayoutView` combined and reference odds), and A-grade highlights (`LegHighlightView` and `ParlayHighlightView`) in `internal/web/slips.go` and `web/templates/partials.html`, with handler tests verifying rendered output and empty/all-voided omission.

- Added `ImpliedChance` to `LegView` and `LegHighlightView`, `CombinedImplied` to `ParlayHighlightView`, and `CombinedImplied` / `ReferenceImplied` to `PayoutView` in `internal/web/slips.go`, populated via `parlay.ImpliedProb`, `parlay.DecimalImpliedProb`, and `formatImpliedChance`.
- Updated `web/templates/partials.html` to render muted implied percentage chances beside leg odds, payout combined/reference odds, and A-grade leg and parlay highlights, omitting them when empty.
- Added handler tests `TestSlipHubRendersImpliedChancesAndOmitsEmptyOrVoided` and `TestSlipHubHighlightsRenderImpliedChances` in `internal/web/slips_test.go` covering leg rows, payout combined and reference odds, A-grade highlights, and empty/all-voided omission.

### Task 3: Docs: update `docs/reference/odds-math.md` for `DecimalImpliedProb` and implied chance display, and update `docs/concepts/slips.md` for slip hub leg, payout, and highlight implied chances.

- Updated [docs/reference/odds-math.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/docs/reference/odds-math.md) with `DecimalImpliedProb`, `ImpliedProb` delegation, `formatImpliedChance` display rules, and the slip hub display surfaces.
- Updated [docs/concepts/slips.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/docs/concepts/slips.md) to document muted implied percentage chances on slip hub leg rows, the payout calculator's combined and reference odds, and A-grade highlights.
- Verified both docs pass `good-vibes docs lint` with zero errors or warnings.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `formatImpliedChance` is package-private in `package web` (`internal/web/slips.go`), ready for view-model wiring in `slips.go` and `pick.go`.
- Unit tests for `formatImpliedChance` live in `internal/web/slips_unit_test.go` (`package web`), matching `research_unit_test.go`.

### Task 2

- Pick-board views (`PickRowView`, `TrayLegView`, `PickDockView`) in `internal/web/pick.go` and their template blocks in `web/templates/partials.html` remain for the next task.
- `formatImpliedChance` in `internal/web/slips.go` and `parlay.DecimalImpliedProb` in `internal/parlay/parlay.go` are ready for `internal/web/pick.go`.

### Task 3

- Phase 2 will add implied percentage chances to the pick board (`PickRowView`, `TrayLegView`, and `PickDockView`) and update `docs/concepts/slips.md` for those surfaces.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/odds-math.md`
- `internal/parlay/parlay.go`
- `internal/parlay/parlay_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `internal/web/slips_unit_test.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: test 9.9s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
