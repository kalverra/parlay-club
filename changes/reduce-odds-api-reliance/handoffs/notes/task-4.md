## Done

- Updated `docs/concepts/data-refresh.md` to document Parlay-only browsing, pre-fill, and pick-time syncs, fallback-only Odds API calls in `syncOdds` / `syncOddsProps`, and lock-only, market-scoped enrichment in `KickSlipExportEnrich`.
- Updated `docs/concepts/export.md` to reflect lock-only, `draftkings,fanduel`-scoped link enrichment and in-place `slip-updated` SSE updates after lock.
- Updated `docs/reference/data-sources.md` to document `LinesForMarkets`, `EventPropsForMarkets`, `odds.APIMarketKey`, lock-only market-scoped enrichment, and updated monthly credit budget behavior.
- Ran `good-vibes docs lint` (0 errors; no warnings on the edited docs) and `good-vibes docs index`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

None.

## Commit

Document Parlay-only browsing and lock-only enrichment

Updates the data-refresh, export, and data-sources docs so living documentation matches the Parlay-only browsing behavior and lock-only, market-scoped Odds API enrichment implemented in Phase 2.
