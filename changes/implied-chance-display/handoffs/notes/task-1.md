## Done

- Added `parlay.DecimalImpliedProb(decimal float64) float64` in [internal/parlay/parlay.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/parlay/parlay.go) and delegated `parlay.ImpliedProb` to it.
- Added `formatImpliedChance(prob float64) string` in [internal/web/slips.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/web/slips.go) covering `<0.1%`, one-decimal (`0.1%`–`9.9%`), whole-percent (`10%`–`99%`), `>99%`, and `""` for non-betting/non-finite inputs.
- Added unit tests in [internal/parlay/parlay_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/parlay/parlay_test.go) (`TestDecimalImpliedProb`, including pushed/voided `CombinedOdds` cases) and [internal/web/slips_unit_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/internal/web/slips_unit_test.go) (`TestFormatImpliedChance`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `formatImpliedChance` is package-private in `package web` (`internal/web/slips.go`), ready for view-model wiring in `slips.go` and `pick.go`.
- Unit tests for `formatImpliedChance` live in `internal/web/slips_unit_test.go` (`package web`), matching `research_unit_test.go`.

## Commit

Add implied probability helpers for decimal odds and display

Parlay combined and reference odds are stored as decimal prices, and empty or all-voided slips collapse to decimal 1.0. Adding `parlay.DecimalImpliedProb` and `web.formatImpliedChance` provides a single, consistent path for turning both leg and parlay prices into compact UI percentages without exposing misleading 100% values.
