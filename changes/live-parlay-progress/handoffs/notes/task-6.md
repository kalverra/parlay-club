# Task 6

## Done

- Added live player prop evaluation in `internal/progress/props.go` for `PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, and `ANYTIME_TD`, plus `OTHER` and unlinked fallbacks in `internal/progress/progress.go`.
- Handled non-monotonic yardage props (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`) as `StateLive` until final, monotonic early cash/bust for `RUSH_ATT`, `RECEPTIONS`, `KICKER_PTS`, `PASS_TDS`, and `ANYTIME_TD`, absent players in populated live box scores as `0`, and unpopulated box scores (`box == nil || !box.HasStats`) as `"Live · stats pending"` at pre-game probability.
- Implemented `EvaluateSlip(legs []LegProgress) SlipProgress` in `internal/progress/slip.go`, multiplying live probabilities across counting legs, excluding `PUSH`/`VOID` legs, averaging leg progress, and collapsing busted parlays to `0%`.
- Added unit tests in `internal/progress/props_test.go` covering kickoff anchoring for all 8 player prop markets, yardage non-monotonicity, monotonic early settlement, absent players, pending box scores, unlinked legs, pushes/voids, and busted/cashed parlays.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/progress` now exports `EvaluateLeg(leg db.Leg, game *db.Game, box *espn.BoxScore) LegProgress` and `EvaluateSlip(legs []LegProgress) SlipProgress`, ready for Phase 3 wiring in `internal/web` (`slips.go`, `home.go`, and templates).

## Commit

Add live player prop evaluation and slip aggregation

Completes the deterministic live progress engine by evaluating all player prop markets with Normal and Poisson models anchored to pre-game odds and aggregating counting legs into slip-level live hit probability and bar progress.
