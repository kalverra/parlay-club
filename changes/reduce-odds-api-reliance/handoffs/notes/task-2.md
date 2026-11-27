## Done

- Updated `syncOdds` (`internal/refresh/sources.go`) to skip `fallbackLines` when Parlay API succeeds with rows, while preserving fallback when Parlay API is unconfigured, errors, sits below `OddsLinesFloor`, or returns zero rows.
- Updated `syncOddsProps` (`internal/refresh/research.go`) to skip `fetchFallbackProps` when Parlay API succeeds with props, while preserving fallback on unconfigured, error, below-`OddsPropsFloor`, or zero-prop outcomes.
- Removed the browse-time `oddsPropsEnrichSource` tail calls from `SyncGameProps` and `syncPickProps` (`internal/refresh/refresh.go`) so opening a game on the pick board never triggers a 9-credit Odds API fetch.
- Updated `sources_test.go`, `research_test.go`, `pick_test.go`, `refresh_test.go`, and `prefill_test.go` to verify zero Odds API calls when Parlay API succeeds and fallback behavior on outages, zero rows, and credit floors.

## Deviations

Also updated `internal/refresh/refresh_test.go` and `internal/refresh/prefill_test.go` (in addition to `sources_test.go`, `research_test.go`, and `pick_test.go`) because existing browse-time enrichment tests in those files asserted that The Odds API ran when Parlay API succeeded.

## Open issues

None.

## Next-phase notes

- `SyncGameProps` and `syncPickProps` no longer call `oddsPropsEnrichSource`, and `syncOdds` / `syncOddsProps` no longer call The Odds API when Parlay API succeeds; only `syncSlipExportEnrich` (and direct `enrichGameProps` / `enrichWeekLines`) triggers The Odds API enrichment.

## Commit

Skip The Odds API on browse when Parlay API succeeds

Stop calling The Odds API during game-line and prop browsing when Parlay API returns rows, and remove browse-time prop enrichment from SyncGameProps and syncPickProps. Outage, zero-row, and credit-floor fallbacks remain intact.
