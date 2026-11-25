# Task 2

## Done

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

- `internal/eval/replay.go` (`attachPreKickoffSharp`) only attaches `Game.Sharp` from `preKickoffOdds`; if Phase 6
  evaluates `v2` features that depend on `Game.MarketScript`, replay should also call `analyzer.BuildMarketScript`.

## Next-phase notes

- `Game.MarketScript` excludes DFS/exchange books (`excludedTier3Books`) and deduplicates each book's two sides before
  taking per-book medians, so `HomeImpliedTotal` and `AwayImpliedTotal` are populated only when both `HasSpread` and
  `HasTotal` are true.

## Commit

Add Power sharp fields and cached market script

V2 grading and prediction features need favorite-longshot-adjusted sharp benchmarks, vig signals, and consensus
game-script anchors without spending additional odds API credits or SQLite round-trips. Computing Power de-vig metrics
and market scripts directly from the cached odds rows already fetched per week keeps signal loading fast and
within free-tier guardrails.
