## Done

- Updated [docs/reference/odds-math.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/docs/reference/odds-math.md) with `DecimalImpliedProb`, `ImpliedProb` delegation, `formatImpliedChance` display rules, and the slip hub display surfaces.
- Updated [docs/concepts/slips.md](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/implied-chance-display/docs/concepts/slips.md) to document muted implied percentage chances on slip hub leg rows, the payout calculator's combined and reference odds, and A-grade highlights.
- Verified both docs pass `good-vibes docs lint` with zero errors or warnings.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 will add implied percentage chances to the pick board (`PickRowView`, `TrayLegView`, and `PickDockView`) and update `docs/concepts/slips.md` for those surfaces.

## Commit

Document implied probability math and hub display

Update the odds-math reference and slips concept docs so the documentation matches the new `DecimalImpliedProb` helper, `formatImpliedChance` formatting rules, and slip hub implied chance display.
