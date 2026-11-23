# Task 3

## Done

- Added `KickSlipExportEnrich(ctx, slipID)` to `refresh.Service` (`internal/refresh/refresh.go`) and `web.Refresher` (`internal/web/routes.go`), sharing background lifecycle management via `kickAsync`.
- Implemented `syncSlipExportEnrich` to load a locked slip's legs and week games and run `oddsPropsEnrichSource` only for upcoming games with at least one prop leg (`isPropLeg`) missing a DraftKings or FanDuel link (`legNeedsExportEnrich`).
- Updated `slipHandlers.lockIn` (`internal/web/slips.go`) to reload the slip when `h.deps.Refresh != nil` and call `KickSlipExportEnrich` when the slip transitions to `LOCKED`.
- Updated `Refresher` test stubs in `internal/web/games_test.go`, `internal/web/pick_test.go`, and `internal/web/research_test.go`.
- Added `TestKickSlipExportEnrich` in `internal/refresh/pick_test.go` and `TestLockInKicksExportEnrichOnlyWhenSlipTransitionsToLocked` and `TestLockInEnrichesUnlinkedPropInBackgroundAndPushesSSE` in `internal/web/export_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Phase 1 prop enrichment triggers (`odds.MergeProps` normalization, `OddsPropsEnrichSource` / `enrichGameProps`, and `KickSlipExportEnrich` on slip lock) are complete and tested. Phase 2 can proceed with multi-game DraftKings grouping and manual prop notes in `internal/export`, `internal/web/export.go`, and `web/templates/partials.html`.

## Commit

Kick background prop enrichment when a slip locks

Slips containing prop legs on pre-filled games can lock without anyone having opened the game detail view to fetch sportsbook deep links. Enriching upcoming unlinked prop games in the background on lock-in populates those links and pushes `slip-updated` over SSE so the export card updates automatically.
