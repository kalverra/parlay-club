## Done

- Added `ImpliedProb`, `DevigTwoWay`, and `PriceEdge` to `internal/parlay/parlay.go` with overround bounds `[0.95, 1.25]` and invalid-input guards.
- Added table-driven unit tests `TestImpliedProb`, `TestDevigTwoWay`, and `TestPriceEdge` to `internal/parlay/parlay_test.go`.
- Documented the new formulas and API entries in `docs/reference/odds-math.md` and removed deprecated `sources`/`last_verified` front-matter fields so `good-vibes docs lint` passes cleanly.

## Deviations

- `PriceEdge` returns `0` when `retailAmerican == 0` or `fairProb` is outside `(0, 1)` rather than returning a raw `fairProb - 0` difference, preventing invalid quotes from producing false positive edges.

## Open issues

- None.

## Next-phase notes

- `DevigTwoWay` returns `ok=false` when either price is `0` or the overround falls outside `[0.95, 1.25]`; `BuildSharpBenchmarks` can rely on `ok` directly to skip corrupt or mispaired quotes.

## Commit

Add implied probability, de-vig, and price edge math

Sharp bookmaker grading needs pure implied-probability, two-way de-vigging, and retail-vs-fair edge helpers in `internal/parlay` so downstream benchmark building and leg scoring can compare prices deterministically without extra API calls.
