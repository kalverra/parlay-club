# Task 1

## Done

- Implemented `VigPct`, `DevigTwoWayPower`, `DevigTwoWayShin`, and `FairEV` in `internal/parlay/devig.go`, reusing the
  `[0.95, 1.25]` overround bounds from `internal/parlay/parlay.go` while keeping `DevigTwoWay` intact.
- Added unit tests in `internal/parlay/devig_test.go` covering symmetric lines (`-110/-110`, `-125/-125`, `+100/+100`,
  `+105/+105`), favorite-longshot lines (`-400/+300`, `-150/+130`, `-1500/+900`), invalid prices and out-of-range
  overrounds, and `FairEV` edge cases.

## Deviations

None.

## Open issues

- `DevigTwoWayShin` falls back to multiplicative normalization when `overround < 1.0` (within `[0.95, 1.0)`), since
  Shin's insider-trading parameter `z` is only defined on `(0, 1)` for positive overrounds; `DevigTwoWayPower` solves
  `pA^k + pB^k = 1` across the full `[0.95, 1.25]` range.

## Next-phase notes

- `VigPct(priceA, priceB)` returns `(overround - 1, ok)` as a decimal fraction (matching `FairEV` and `PriceEdge`), so
  Task 2 can attach it directly to `SharpQuote.VigPct` alongside `DevigTwoWayPower`.

## Commit

Add Power and Shin two-way de-vigging math

Multiplicative normalization under-corrects favorite-longshot bias on props and alt lines, distorting fair probabilities
used by the v2 model. Adding Power and Shin de-vigging, overround vig, and FairEV gives sharp benchmarks and scoring an
accurate market anchor without altering v1 parity.
