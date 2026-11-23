# Phase 1: Phase 1: Separate prop enrichment from Parlay-only pre-fill

## Done

### Task 1: Normalize player names with `normalizePropSubject` in `odds.MergeProps` (`internal/ingest/odds/props.go`) so cross-provider enrichment matches punctuation and generational-suffix variants, with unit tests in `internal/ingest/odds/props_test.go`.

- Normalized `prop.PlayerName` via `normalizePropSubject` in `propKeyOf` (`internal/ingest/odds/props.go`) so `odds.MergeProps` matches player names across punctuation, casing, and generational-suffix differences.
- Updated the `normalizePropSubject` doc comment to reflect its use in both quote grouping and cross-provider prop merging.
- Added `TestMergePropsMatchesNormalizedPlayerNames` in `internal/ingest/odds/props_test.go` covering generational suffixes (`III`, `Jr.`), initials with periods (`D.J.` vs `DJ`), and apostrophe/case variants.

### Task 2: Add `OddsPropsEnrichSource(gameID)` and `enrichGameProps` in `internal/refresh/research.go` and `internal/refresh/refresh.go` so `syncOddsProps` records enrichment freshness when The Odds API buys props, and `SyncPickGame`/`SyncGameProps` run enrichment-only when `odds:props:{gameID}` was populated by Parlay-only `RunPrefill`, merging against existing current prop rows at a unified `FetchedAt` and enforcing `TheOddsAPIEnrichFloor`, with tests in `internal/refresh/prefill_test.go` and `internal/refresh/pick_test.go`.

- Added `OddsPropsEnrichSource(gameID)` in `internal/refresh/refresh.go` and `oddsPropsEnrichSource` / `enrichGameProps` in `internal/refresh/research.go`.
- Updated `writeOddsProps` to accept `enriched bool` and record `OddsPropsEnrichSource(game.ID)` in the same transaction when `syncOddsProps` buys fallback props from The Odds API (while keeping `prefillProps` Parlay-only with `enriched=false`).
- Updated `SyncGameProps` and `syncPickProps` (used by `SyncPickGame` and `ForcePickRefresh`) to run `oddsPropsEnrichSource` when `odds:props:{gameID}` is already fresh (such as after `RunPrefill`) and enrichment has not run for that sync, merging against existing current prop rows at a unified `FetchedAt` and enforcing `TheOddsAPIEnrichFloor` without entering failure backoff.
- Added `TestRunPrefillThenPickEnrichment` in `internal/refresh/prefill_test.go` and `TestSyncGamePropsEnrichesPrefilledPropsAndEnforcesEnrichFloor` and `TestSyncOddsPropsRecordsEnrichFreshnessWhenFallbackBought` in `internal/refresh/pick_test.go`.

### Task 3: Add `KickSlipExportEnrich(ctx, slipID)` to `internal/refresh` and `web.Refresher`, and invoke it from `slipHandlers.lockIn` (`internal/web/slips.go`) when a slip transitions to `LOCKED` so upcoming games with unlinked prop legs enrich in the background and push `slip-updated` over SSE, with tests in `internal/refresh/` and `internal/web/export_test.go`.

- Added `KickSlipExportEnrich(ctx, slipID)` to `refresh.Service` (`internal/refresh/refresh.go`) and `web.Refresher` (`internal/web/routes.go`), sharing background lifecycle management via `kickAsync`.
- Implemented `syncSlipExportEnrich` to load a locked slip's legs and week games and run `oddsPropsEnrichSource` only for upcoming games with at least one prop leg (`isPropLeg`) missing a DraftKings or FanDuel link (`legNeedsExportEnrich`).
- Updated `slipHandlers.lockIn` (`internal/web/slips.go`) to reload the slip when `h.deps.Refresh != nil` and call `KickSlipExportEnrich` when the slip transitions to `LOCKED`.
- Updated `Refresher` test stubs in `internal/web/games_test.go`, `internal/web/pick_test.go`, and `internal/web/research_test.go`.
- Added `TestKickSlipExportEnrich` in `internal/refresh/pick_test.go` and `TestLockInKicksExportEnrichOnlyWhenSlipTransitionsToLocked` and `TestLockInEnrichesUnlinkedPropInBackgroundAndPushesSSE` in `internal/web/export_test.go`.

### Task 4: Docs: update `docs/concepts/data-refresh.md` and `docs/reference/data-sources.md` to document `odds:props:enrich:{gameID}`, enrichment-only refreshes after `RunPrefill`, and `KickSlipExportEnrich` on slip lock.

- Updated `docs/concepts/data-refresh.md` to document `KickSlipExportEnrich` on slip lock, add `odds:props:enrich:{gameID}` to the staleness table, and explain enrichment-only refreshes (`enrichGameProps`) after Parlay-only `RunPrefill` in the odds credits section.
- Updated `docs/reference/data-sources.md` to document `odds:props:enrich:{game}`, `odds.MergeProps` player-name normalization via `normalizePropSubject`, enrichment-only refreshes after `RunPrefill`, and `KickSlipExportEnrich` on slip lock.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`odds.MergeProps` now merges variant player names into the primary row's `PlayerName` without duplicating selections, ready for the enrichment-only refresh path.

### Task 2

`enrichGameProps` and `OddsPropsEnrichSource` are ready for `KickSlipExportEnrich` in the next task. `enrichGameProps` is package-private on `*Service` and can be invoked via `s.refreshSync(ctx, s.oddsPropsEnrichSource(game, now, false), now)`.

### Task 3

Phase 1 prop enrichment triggers (`odds.MergeProps` normalization, `OddsPropsEnrichSource` / `enrichGameProps`, and `KickSlipExportEnrich` on slip lock) are complete and tested. Phase 2 can proceed with multi-game DraftKings grouping and manual prop notes in `internal/export`, `internal/web/export.go`, and `web/templates/partials.html`.

### Task 4

Phase 1 is complete. Phase 2 can implement multi-game DraftKings `LinkGroup` grouping and manual prop notes in `internal/export`, `internal/web/export.go`, and `web/templates/partials.html`.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/reference/data-sources.md`
- `internal/ingest/odds/props.go`
- `internal/ingest/odds/props_test.go`
- `internal/refresh/pick_test.go`
- `internal/refresh/prefill_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/research.go`
- `internal/web/export_test.go`
- `internal/web/games_test.go`
- `internal/web/pick_test.go`
- `internal/web/research_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3, task-4
- Verify: verify passed: test 300ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
