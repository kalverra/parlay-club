---
slug: reduce-odds-api-reliance
run: 20261127-194044-reduce-odds-api-reliance
outcome: done
generated: 2026-11-27T20:08:55Z
---

# Run summary: reduce-odds-api-reliance

## Outcome

- Status: done
- Branch: reduce-odds-api-reliance
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 2
- Checkpoints: 5
- Duration: 24m37s
- Cost: $—
- Fresh input tokens: 1108481
- Cache read tokens: 12194250
- Cache write tokens: 0
- Output tokens: 145144

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 9840584 | $— |
| gemini-4-neon-medium | 3607291 | $— |

## Phases

### Phase 1: Add market- and bookmaker-scoped Odds API queries

Tasks:

- [x] Add `APIMarketKey`, `LinesForMarkets`, and `EventPropsForMarkets` in `internal/ingest/odds/lines.go` and `internal/ingest/odds/props.go` (supporting `markets` and `bookmakers` query params), with unit tests in `internal/ingest/odds/lines_test.go` and `internal/ingest/odds/props_test.go`.

Phase totals: 1 checkpoints, 2m28s, $—, 946624 input / 13888 output / 853179 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `APIMarketKey`, `LinesForMarkets`, and `EventPropsForMarkets` in `internal/ingest/odds/lines.go` and `internal/ingest/odds/props.go` (supporting `markets` and `bookmakers` query params), with unit tests in `internal/ingest/odds/lines_test.go` and `internal/ingest/odds/props_test.go`.

- Added `LinesForMarkets(ctx, markets, bookmakers)` in `internal/ingest/odds/lines.go` and refactored `Lines(ctx)` to delegate to it, using shared `buildOddsQuery` and `cleanCSVTerms` helpers to support `bookmakers` (falling back to `regions=us` when empty) and deduplicated `markets` with `includeLinks=true` and `includeSids=true`.
- Added `APIMarketKey(marketType)` and `EventPropsForMarkets(ctx, eventID, markets, bookmakers)` in `internal/ingest/odds/props.go`, and refactored `EventProps(ctx, eventID)` to delegate to `EventPropsForMarkets`.
- Added unit tests in `internal/ingest/odds/lines_test.go` and `internal/ingest/odds/props_test.go` covering `APIMarketKey` for all 12 market types, scoped `markets` and `bookmakers` query params, `regions=us` fallback, and empty-argument validation.

### Phase 2: Switch browsing to Parlay-only and lock-time enrichment to market-scoped calls

Tasks:

- [x] Update `syncOdds` (`internal/refresh/sources.go`), `syncOddsProps` (`internal/refresh/research.go`), and `SyncGameProps` / `syncPickProps` (`internal/refresh/refresh.go`) to skip The Odds API enrichment when Parlay API succeeds while preserving outage and credit-floor fallback, updating `sources_test.go`, `research_test.go`, and `pick_test.go`.
- [x] Update `syncSlipExportEnrich` in `internal/refresh/refresh.go` to batch one free `Events` check and run market-scoped `LinesForMarkets` and per-game `EventPropsForMarkets` calls (`draftkings,fanduel` only) for missing locked-slip links, with unit tests in `internal/refresh/pick_test.go`.
- [x] Docs: update `docs/concepts/data-refresh.md`, `docs/concepts/export.md`, and `docs/reference/data-sources.md` to document Parlay-only browsing and lock-only, market-scoped Odds API enrichment.

Phase totals: 4 checkpoints, 21m26s, $—, 9723119 input / 117465 output / 8911711 cache read / 0 cache write tokens

Checkpoints: task-2, task-3, task-4, final Review fixes: 0 Integration checks: 0

#### Done

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

## Final gate

- Rounds used: 2; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
