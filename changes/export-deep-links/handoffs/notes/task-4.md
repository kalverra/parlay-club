# Task 4

## Done

- Updated `docs/concepts/data-refresh.md` to document `KickSlipExportEnrich` on slip lock, add `odds:props:enrich:{gameID}` to the staleness table, and explain enrichment-only refreshes (`enrichGameProps`) after Parlay-only `RunPrefill` in the odds credits section.
- Updated `docs/reference/data-sources.md` to document `odds:props:enrich:{game}`, `odds.MergeProps` player-name normalization via `normalizePropSubject`, enrichment-only refreshes after `RunPrefill`, and `KickSlipExportEnrich` on slip lock.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 1 is complete. Phase 2 can implement multi-game DraftKings `LinkGroup` grouping and manual prop notes in `internal/export`, `internal/web/export.go`, and `web/templates/partials.html`.

## Commit

Document prop enrichment and slip-lock triggers

Update data-refresh and data-sources docs so the new `odds:props:enrich:{gameID}` key, post-prefill enrichment-only path, and `KickSlipExportEnrich` slip-lock trigger are accurately recorded alongside existing credit floors and staleness rules.
