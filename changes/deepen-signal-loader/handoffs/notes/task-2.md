# Task 2

## Done

- Added `LoadSlipSignals`, `LoadGameSignals`, `PlayerResearchSignals`, and `LoadPlayerResearchSignals` in `internal/analyzer/signals.go`, centralizing 72h `nfelo` freshness, 7d `nflverse` freshness, team defense profiles, player usage, and `player_gamelogs` JSON decoding.
- Added fallback-aware `PlayerUsage` accessor methods (`EffectiveTargetShare`, `EffectiveSnapAvg`, `EffectiveSeasonSnapAvg`, `RecentWOPR`, `AverageCarries`) in `internal/analyzer/analyzer.go` and updated `internal/board/signals.go` to use them, removing duplicate fallback loops from `internal/board`.
- Updated `internal/web/slips.go` (`slipPageData`), `internal/web/pick.go` (`dockFrom`, `boardSignalsForGame`), and `internal/web/research.go` (`populateNFLverseResearch`) to call the new `analyzer` signal loaders, and removed the obsolete wrappers from `internal/web/analyzer.go`.
- Added unit tests for `LoadSlipSignals`, `LoadGameSignals`, `LoadPlayerResearchSignals`, staleness cutoffs, and `PlayerUsage` accessor fallbacks in `internal/analyzer/signals_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/web/analyzer.go` now only handles view-model mapping (`AnalyzerView`, `GradeView`, `analyzerGuide`, `legModel`, `analyzerSeverity`) and `internal/analyzer` owns all signal loading and aggregation.
