# Task 3

## Done

- Updated `docs/reference/odds-math.md` to document shared two-way overround guardrails (`[0.95, 1.25]`), `VigPct`,
  multiplicative (`DevigTwoWay`), Power (`DevigTwoWayPower`), and Shin (`DevigTwoWayShin`) de-vigging formulas, and
  `FairEV` alongside `PriceEdge`.
- Added `DevigTwoWayPower`, `DevigTwoWayShin`, `VigPct`, and `FairEV` to the `internal/parlay` API table and refreshed
  `docs/llms.txt` via `good-vibes docs index`.

## Deviations

None.

## Open issues

- `docs/reference/analyzer-rules.md` and `docs/concepts/analyzer.md` still describe v1 sharp benchmarks and scoring;
  those will need updates when v2 grading and UI changes land in later phases.

## Next-phase notes

- `docs/reference/odds-math.md` now covers all `internal/parlay` v2 math helpers and notes that `FairEV` is an internal
  scoring signal rather than user-facing `"+EV %"` copy.

## Commit

Document Power/Shin devigging and FairEV formulas

Keep the odds math reference aligned with the new two-way de-vigging, vig-percentage, and expected-value helpers in `internal/parlay`. Recording the solver bounds, negative-vig fallback behavior, and input guardrails ensures callers and later phases share one accurate specification.
