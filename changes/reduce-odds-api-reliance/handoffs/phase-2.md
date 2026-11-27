# Phase 2: Phase 2: Switch browsing to Parlay-only and lock-time enrichment to market-scoped calls

## Done

### Task 2: Update `syncOdds` (`internal/refresh/sources.go`), `syncOddsProps` (`internal/refresh/research.go`), and `SyncGameProps` / `syncPickProps` (`internal/refresh/refresh.go`) to skip The Odds API enrichment when Parlay API succeeds while preserving outage and credit-floor fallback, updating `sources_test.go`, `research_test.go`, and `pick_test.go`.

- Updated `syncOdds` (`internal/refresh/sources.go`) to skip `fallbackLines` when Parlay API succeeds with rows, while preserving fallback when Parlay API is unconfigured, errors, sits below `OddsLinesFloor`, or returns zero rows.
- Updated `syncOddsProps` (`internal/refresh/research.go`) to skip `fetchFallbackProps` when Parlay API succeeds with props, while preserving fallback on unconfigured, error, below-`OddsPropsFloor`, or zero-prop outcomes.
- Removed the browse-time `oddsPropsEnrichSource` tail calls from `SyncGameProps` and `syncPickProps` (`internal/refresh/refresh.go`) so opening a game on the pick board never triggers a 9-credit Odds API fetch.
- Updated `sources_test.go`, `research_test.go`, `pick_test.go`, `refresh_test.go`, and `prefill_test.go` to verify zero Odds API calls when Parlay API succeeds and fallback behavior on outages, zero rows, and credit floors.

### Task 3: Update `syncSlipExportEnrich` in `internal/refresh/refresh.go` to batch one free `Events` check and run market-scoped `LinesForMarkets` and per-game `EventPropsForMarkets` calls (`draftkings,fanduel` only) for missing locked-slip links, with unit tests in `internal/refresh/pick_test.go`.

- Added optional `marketLinesClient` and `marketPropsClient` interfaces in `internal/refresh/refresh.go` (implemented by `*odds.Client`) and scoped lock-time enrichment bookmakers to `draftkings,fanduel`.
- Updated `syncSlipExportEnrich` in `internal/refresh/refresh.go` (via `collectMissingExportMarkets`, `dueExportEnrichTargets`, `fetchEnrichMatches`, and `runFlightEnrich`) to collect only missing markets across upcoming locked-slip legs, batch one free `Events` call + `TheOddsAPIEnrichFloor` check per pass, and run market-scoped `LinesForMarkets` and per-game `EventPropsForMarkets` calls.
- Added `enrichWeekLinesForMarkets` (`internal/refresh/sources.go`) and `enrichGamePropsForMarkets` (`internal/refresh/research.go`) sharing `commitWeekLinesEnrich` and `commitGamePropsEnrich` with the full-slate helpers so merged rows commit at a unified `FetchedAt` and publish SSE topics.
- Updated `internal/refresh/research_test.go` and `internal/refresh/pick_test.go` (including `TestKickSlipExportEnrichBatchesEventsAndScopesMarkets`) to verify single `Events` batching, exact `markets` and `draftkings,fanduel` bookmaker scoping, credit-floor gating, and SSE publishing.

### Task 4: Docs: update `docs/concepts/data-refresh.md`, `docs/concepts/export.md`, and `docs/reference/data-sources.md` to document Parlay-only browsing and lock-only, market-scoped Odds API enrichment.

- Updated `docs/concepts/data-refresh.md` to document Parlay-only browsing, pre-fill, and pick-time syncs, fallback-only Odds API calls in `syncOdds` / `syncOddsProps`, and lock-only, market-scoped enrichment in `KickSlipExportEnrich`.
- Updated `docs/concepts/export.md` to reflect lock-only, `draftkings,fanduel`-scoped link enrichment and in-place `slip-updated` SSE updates after lock.
- Updated `docs/reference/data-sources.md` to document `LinesForMarkets`, `EventPropsForMarkets`, `odds.APIMarketKey`, lock-only market-scoped enrichment, and updated monthly credit budget behavior.
- Ran `good-vibes docs lint` (0 errors; no warnings on the edited docs) and `good-vibes docs index`.

## Deviations

### Task 2

Also updated `internal/refresh/refresh_test.go` and `internal/refresh/prefill_test.go` (in addition to `sources_test.go`, `research_test.go`, and `pick_test.go`) because existing browse-time enrichment tests in those files asserted that The Odds API ran when Parlay API succeeded.

### Task 3

- Also updated `fakeFallbackPropsClient` in `internal/refresh/research_test.go` alongside `internal/refresh/pick_test.go` so the shared test fake records `EventPropsForMarkets` calls and market/bookmaker parameters.

## Open issues

None.

## Next-phase notes

### Task 2

- `SyncGameProps` and `syncPickProps` no longer call `oddsPropsEnrichSource`, and `syncOdds` / `syncOddsProps` no longer call The Odds API when Parlay API succeeds; only `syncSlipExportEnrich` (and direct `enrichGameProps` / `enrichWeekLines`) triggers The Odds API enrichment.

### Task 3

- `syncSlipExportEnrich` now handles lock-time enrichment through `LinesForMarkets` and `EventPropsForMarkets` (`draftkings,fanduel` only) after one batched `Events` check; only `enrichWeekLines` / `enrichGameProps` (the latter still exercised by `TestEnrichGamePropsMergesUnresolvedVariantPlayerName`) use the unscoped fallback methods.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/concepts/export.md`
- `docs/reference/data-sources.md`
- `internal/refresh/pick_test.go`
- `internal/refresh/prefill_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/refresh_test.go`
- `internal/refresh/research.go`
- `internal/refresh/research_test.go`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`

## Run log

- Mode: 
- Checkpoints: task-2, task-3, task-4
- Verify: verify passed: test 400ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
