---
slug: implied-chance-display
run: 20261124-224508-implied-chance-display
outcome: done
generated: 2026-11-24T22:58:30Z
---

# Run summary: implied-chance-display

## Outcome

- Status: done
- Branch: implied-chance-display
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-low approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 2
- Checkpoints: 5
- Duration: 11m9s
- Cost: $—
- Fresh input tokens: 317262
- Cache read tokens: 1691712
- Cache write tokens: 0
- Output tokens: 49368

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 2058342 | $— |

## Phases

### Phase 1: Odds math helper and slip hub display

Tasks:

- [x] Add `parlay.DecimalImpliedProb` in `internal/parlay` (with `ImpliedProb` delegating to it) and `formatImpliedChance` in `internal/web`, with unit tests covering standard prices, pushed/voided combined odds, and formatting boundaries (`<0.1%`, `0.1%`–`9.9%`, `10%`–`99%`, `>99%`, and empty/invalid inputs).
- [x] Populate and render implied percentage chances on slip hub leg rows (`LegView`), the payout calculator (`PayoutView` combined and reference odds), and A-grade highlights (`LegHighlightView` and `ParlayHighlightView`) in `internal/web/slips.go` and `web/templates/partials.html`, with handler tests verifying rendered output and empty/all-voided omission.
- [x] Docs: update `docs/reference/odds-math.md` for `DecimalImpliedProb` and implied chance display, and update `docs/concepts/slips.md` for slip hub leg, payout, and highlight implied chances.

Phase totals: 3 checkpoints, 6m7s, $—, 1004487 input / 24684 output / 845856 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Pick board, dock, and tray display

Tasks:

- [x] Populate and render implied percentage chances on pick board option buttons (`PickRowView`), tray leg rows (`TrayLegView`), and the fixed pick dock (`PickDockView`) in `internal/web/pick.go` and `web/templates/partials.html`, with handler tests covering add/remove/switch/swap option rows, tray legs, and dock combined chance (including empty dock omission).
- [x] Docs: update `docs/concepts/slips.md` to document implied percentage chances on the pick board, dock, and tray.

Phase totals: 2 checkpoints, 4m14s, $—, 669658 input / 16456 output / 563904 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Populate and render implied percentage chances on pick board option buttons (`PickRowView`), tray leg rows (`TrayLegView`), and the fixed pick dock (`PickDockView`) in `internal/web/pick.go` and `web/templates/partials.html`, with handler tests covering add/remove/switch/swap option rows, tray legs, and dock combined chance (including empty dock omission).

- Added `ImpliedChance` to `PickRowView` and `TrayLegView`, and `CombinedImplied` to `PickDockView` in `internal/web/pick.go`, populated via `formatImpliedChance` and `parlay.ImpliedProb` / `parlay.DecimalImpliedProb`.
- Updated `web/templates/partials.html` to render implied chances on all four pick option button states (`tap to remove`, `tap to switch`, `swap in`, `tap to add`), on tray leg rows, and on the fixed pick dock when legs are present.
- Added `TestPickBoardTrayAndDockRenderImpliedChances` in `internal/web/pick_test.go` covering add/remove/switch/swap option rows, tray legs, and dock combined chance (including empty dock omission).

### Task 5: Docs: update `docs/concepts/slips.md` to document implied percentage chances on the pick board, dock, and tray.

- Updated `docs/concepts/slips.md` to document implied percentage chances on pick-board option buttons (`tap to add`, `tap to remove`, `tap to switch`, and `swap in`).
- Updated the pick dock and tray section of `docs/concepts/slips.md` to document the dock's combined implied percentage chance (omitted when no legs are picked) and the tray leg rows' muted implied percentage chance.
- Tightened the dock/tray paragraph so `docs/concepts/slips.md` stays within its 550-line budget.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
