# Task 7

## Done

- Updated `docs/reference/odds-math.md` front matter, summary, and API table to cover `internal/progress` alongside `internal/parlay`.
- Documented `RemainingFraction` and `ClockLabel` game-clock parsing rules, regulation/OT/halftime clamping, and missing-clock fallbacks.
- Documented `EvaluateLeg` anchoring, settled/scheduled/unavailable/pending-box-score states, Normal-model formulas (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, and yardage/volume props with market CVs and monotonic early settlement), and Poisson models (`ANYTIME_TD`, `PASS_TDS`).
- Documented `EvaluateSlip` parlay aggregation, `PUSH`/`VOID` exclusion, progress averaging, and tone rules.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 is complete; Phase 3 can wire `progress.EvaluateLeg` and `progress.EvaluateSlip` into `internal/web` (`slips.go`, `home.go`, and templates) and update `docs/` for the UI behavior if needed.

## Commit

Document live progress and hit probability formulas

Record the deterministic clock-remaining, Normal, Poisson, and parlay aggregation formulas in `docs/reference/odds-math.md` so the reference documentation matches `internal/progress`.
