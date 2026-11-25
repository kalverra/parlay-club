# Phase 1: Phase 1: Power/Shin de-vigging and cached game-script anchors

## Done

### Task 1: Add `DevigTwoWayPower`, `DevigTwoWayShin`, `VigPct`, and `FairEV` in `internal/parlay/devig.go` with unit tests in `internal/parlay/devig_test.go` covering symmetric, favorite-longshot, and invalid-overround inputs.

- Implemented `VigPct`, `DevigTwoWayPower`, `DevigTwoWayShin`, and `FairEV` in `internal/parlay/devig.go`, reusing the
  `[0.95, 1.25]` overround bounds from `internal/parlay/parlay.go` while keeping `DevigTwoWay` intact.
- Added unit tests in `internal/parlay/devig_test.go` covering symmetric lines (`-110/-110`, `-125/-125`, `+100/+100`,
  `+105/+105`), favorite-longshot lines (`-400/+300`, `-150/+130`, `-1500/+900`), invalid prices and out-of-range
  overrounds, and `FairEV` edge cases.

### Task 2: Extend `analyzer.SharpQuote` (`PowerFairProb`, `VigPct`, `BookCount`) in `internal/analyzer/sharp.go` and derive `Game.MarketScript` (consensus spread, total, implied team totals) from cached `odds` rows in `internal/analyzer/signals.go`, with tests verifying zero extra DB queries or API calls.

- Extended `analyzer.SharpQuote` with `PowerFairProb`, `VigPct`, and `BookCount` in `internal/analyzer/sharp.go`,
  populating them in `pairML`, `pairSpread`, `pairOverUnder`, `aggregateSharpTier`, and `aggregateTier3Consensus`
  via `parlay.DevigTwoWayPower` while preserving `FairProb` and `FairAmerican` for v1 parity.
- Added `analyzer.MarketScript` in `internal/analyzer/analyzer.go` and `BuildMarketScripts` / `BuildMarketScript` in
  `internal/analyzer/signals.go`, wiring `LoadSlipSignals` and `LoadBoardSignals` to derive consensus spread, total,
  and implied team totals from the single already-loaded `OddsForWeek` slice.
- Added unit tests in `internal/analyzer/sharp_test.go` and `internal/analyzer/signals_test.go` covering Power de-vig,
  vig percentages, tier/bucket `BookCount`, `MarketScript` consensus and implied-total math, and a query-counting +
  failing-HTTP-transport test proving zero extra DB queries or API calls.

## Deviations

None.

## Open issues

### Task 1

- `DevigTwoWayShin` falls back to multiplicative normalization when `overround < 1.0` (within `[0.95, 1.0)`), since
  Shin's insider-trading parameter `z` is only defined on `(0, 1)` for positive overrounds; `DevigTwoWayPower` solves
  `pA^k + pB^k = 1` across the full `[0.95, 1.25]` range.

### Task 2

- `internal/eval/replay.go` (`attachPreKickoffSharp`) only attaches `Game.Sharp` from `preKickoffOdds`; if Phase 6
  evaluates `v2` features that depend on `Game.MarketScript`, replay should also call `analyzer.BuildMarketScript`.

## Next-phase notes

### Task 1

- `VigPct(priceA, priceB)` returns `(overround - 1, ok)` as a decimal fraction (matching `FairEV` and `PriceEdge`), so
  Task 2 can attach it directly to `SharpQuote.VigPct` alongside `DevigTwoWayPower`.

### Task 2

- `Game.MarketScript` excludes DFS/exchange books (`excludedTier3Books`) and deduplicates each book's two sides before
  taking per-book medians, so `HomeImpliedTotal` and `AwayImpliedTotal` are populated only when both `HasSpread` and
  `HasTotal` are true.

## Files touched

- `docs/llms.txt`
- `docs/reference/odds-math.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/sharp.go`
- `internal/analyzer/sharp_test.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/parlay/devig.go`
- `internal/parlay/devig_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
