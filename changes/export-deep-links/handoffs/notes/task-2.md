# Task 2

## Done

- Added `OddsPropsEnrichSource(gameID)` in `internal/refresh/refresh.go` and `oddsPropsEnrichSource` / `enrichGameProps` in `internal/refresh/research.go`.
- Updated `writeOddsProps` to accept `enriched bool` and record `OddsPropsEnrichSource(game.ID)` in the same transaction when `syncOddsProps` buys fallback props from The Odds API (while keeping `prefillProps` Parlay-only with `enriched=false`).
- Updated `SyncGameProps` and `syncPickProps` (used by `SyncPickGame` and `ForcePickRefresh`) to run `oddsPropsEnrichSource` when `odds:props:{gameID}` is already fresh (such as after `RunPrefill`) and enrichment has not run for that sync, merging against existing current prop rows at a unified `FetchedAt` and enforcing `TheOddsAPIEnrichFloor` without entering failure backoff.
- Added `TestRunPrefillThenPickEnrichment` in `internal/refresh/prefill_test.go` and `TestSyncGamePropsEnrichesPrefilledPropsAndEnforcesEnrichFloor` and `TestSyncOddsPropsRecordsEnrichFreshnessWhenFallbackBought` in `internal/refresh/pick_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`enrichGameProps` and `OddsPropsEnrichSource` are ready for `KickSlipExportEnrich` in the next task. `enrichGameProps` is package-private on `*Service` and can be invoked via `s.refreshSync(ctx, s.oddsPropsEnrichSource(game, now, false), now)`.

## Commit

Separate prop link enrichment from Parlay pre-fill

Sunday pre-fill populates Parlay-only props and marks `odds:props:{gameID}` fresh without calling The Odds API, which previously caused pick-time refreshes to skip prop link enrichment. Tracking enrichment freshness separately under `odds:props:enrich:{gameID}` lets pick views enrich pre-filled props with DraftKings and FanDuel deep links without re-spending Parlay API credits or hiding Parlay-only rows under `CurrentOdds`.
