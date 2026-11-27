# Task 3

## Done

- Added optional `marketLinesClient` and `marketPropsClient` interfaces in `internal/refresh/refresh.go` (implemented by `*odds.Client`) and scoped lock-time enrichment bookmakers to `draftkings,fanduel`.
- Updated `syncSlipExportEnrich` in `internal/refresh/refresh.go` (via `collectMissingExportMarkets`, `dueExportEnrichTargets`, `fetchEnrichMatches`, and `runFlightEnrich`) to collect only missing markets across upcoming locked-slip legs, batch one free `Events` call + `TheOddsAPIEnrichFloor` check per pass, and run market-scoped `LinesForMarkets` and per-game `EventPropsForMarkets` calls.
- Added `enrichWeekLinesForMarkets` (`internal/refresh/sources.go`) and `enrichGamePropsForMarkets` (`internal/refresh/research.go`) sharing `commitWeekLinesEnrich` and `commitGamePropsEnrich` with the full-slate helpers so merged rows commit at a unified `FetchedAt` and publish SSE topics.
- Updated `internal/refresh/research_test.go` and `internal/refresh/pick_test.go` (including `TestKickSlipExportEnrichBatchesEventsAndScopesMarkets`) to verify single `Events` batching, exact `markets` and `draftkings,fanduel` bookmaker scoping, credit-floor gating, and SSE publishing.

## Deviations

- Also updated `fakeFallbackPropsClient` in `internal/refresh/research_test.go` alongside `internal/refresh/pick_test.go` so the shared test fake records `EventPropsForMarkets` calls and market/bookmaker parameters.

## Open issues

None.

## Next-phase notes

- `syncSlipExportEnrich` now handles lock-time enrichment through `LinesForMarkets` and `EventPropsForMarkets` (`draftkings,fanduel` only) after one batched `Events` check; only `enrichWeekLines` / `enrichGameProps` (the latter still exercised by `TestEnrichGamePropsMergesUnresolvedVariantPlayerName`) use the unscoped fallback methods.

## Commit

Scope lock-time slip enrichment to missing markets

Fetching all 9 prop markets and all 3 game-line markets per lock wastes The Odds API credits when a locked slip only needs links for specific markets. Batching one free Events lookup and requesting only missing markets for DraftKings and FanDuel cuts lock-time enrichment to 1 credit per needed market.
