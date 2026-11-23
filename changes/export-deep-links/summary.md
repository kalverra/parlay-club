---
slug: export-deep-links
run: 20261123-180203-export-deep-links
outcome: done
generated: 2026-11-23T18:32:02Z
---

# Run summary: export-deep-links

## Outcome

- Status: done
- Branch: export-deep-links
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 2
- Checkpoints: 8
- Duration: 23m38s
- Cost: $—
- Fresh input tokens: 985126
- Cache read tokens: 9657756
- Cache write tokens: 0
- Output tokens: 148575

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 10791457 | $— |

## Phases

### Phase 1: Separate prop enrichment from Parlay-only pre-fill

Tasks:

- [x] Normalize player names with `normalizePropSubject` in `odds.MergeProps` (`internal/ingest/odds/props.go`) so cross-provider enrichment matches punctuation and generational-suffix variants, with unit tests in `internal/ingest/odds/props_test.go`.
- [x] Add `OddsPropsEnrichSource(gameID)` and `enrichGameProps` in `internal/refresh/research.go` and `internal/refresh/refresh.go` so `syncOddsProps` records enrichment freshness when The Odds API buys props, and `SyncPickGame`/`SyncGameProps` run enrichment-only when `odds:props:{gameID}` was populated by Parlay-only `RunPrefill`, merging against existing current prop rows at a unified `FetchedAt` and enforcing `TheOddsAPIEnrichFloor`, with tests in `internal/refresh/prefill_test.go` and `internal/refresh/pick_test.go`.
- [x] Add `KickSlipExportEnrich(ctx, slipID)` to `internal/refresh` and `web.Refresher`, and invoke it from `slipHandlers.lockIn` (`internal/web/slips.go`) when a slip transitions to `LOCKED` so upcoming games with unlinked prop legs enrich in the background and push `slip-updated` over SSE, with tests in `internal/refresh/` and `internal/web/export_test.go`.
- [x] Docs: update `docs/concepts/data-refresh.md` and `docs/reference/data-sources.md` to document `odds:props:enrich:{gameID}`, enrichment-only refreshes after `RunPrefill`, and `KickSlipExportEnrich` on slip lock.

Phase totals: 4 checkpoints, 12m15s, $—, 4923209 input / 64343 output / 4587877 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Multi-game DraftKings grouping and clearer manual prop guidance

Tasks:

- [x] Extend `export.BookPlan` in `internal/export/export.go` with `LinkGroup` per-event grouping, `MultiGameNote` for multi-game DraftKings slips, and synced-prop awareness on `ReasonNoLine` manual legs, with unit tests in `internal/export/export_test.go`.
- [x] Update `internal/web/export.go` and `web/templates/partials.html` (`slip-export`) to render DraftKings' single-game explanation, per-game combined taps when a multi-game slip has 2+ legs in one game, and clearer unquoted-prop manual notes, with tests in `internal/web/export_test.go`.
- [x] Docs: update `docs/concepts/export.md` to document multi-game DraftKings per-game grouping, the single-game note, and the unquoted-prop manual note.

Phase totals: 4 checkpoints, 10m43s, $—, 4008804 input / 70516 output / 3569250 cache read / 0 cache write tokens

Checkpoints: task-5, task-6, task-7, final Review fixes: 0 Integration checks: 0

#### Done

### Task 5: Extend `export.BookPlan` in `internal/export/export.go` with `LinkGroup` per-event grouping, `MultiGameNote` for multi-game DraftKings slips, and synced-prop awareness on `ReasonNoLine` manual legs, with unit tests in `internal/export/export_test.go`.

- Added `LinkGroup` and `BookPlan.Groups` / `BookPlan.MultiGameNote` in `internal/export/export.go` with per-event grouping in `draftKingsCombiner.CombineGroups` when linked DraftKings legs span multiple games.
- Added `ManualLeg.PropsSynced` in `internal/export/export.go` so `ReasonNoLine` on player and team-total prop legs indicates whether the game already has stored prop rows for other markets or books.
- Added unit tests in `internal/export/export_test.go` covering single-game vs multi-game DraftKings grouping, first-appearance order, missing `Sid` handling, and `ReasonNoLine` `PropsSynced` detection.

### Task 6: Update `internal/web/export.go` and `web/templates/partials.html` (`slip-export`) to render DraftKings' single-game explanation, per-game combined taps when a multi-game slip has 2+ legs in one game, and clearer unquoted-prop manual notes, with tests in `internal/web/export_test.go`.

- Added `ExportBookView.MultiGameNote`, `ExportBookView.Groups`, and `ExportGroupView` in `internal/web/export.go` to surface DraftKings' single-game explanation and per-game combined groups (with `AwayTeam @ HomeTeam` matchup labels) when a multi-game slip has 2+ linked legs in a game.
- Updated `manualNote` in `internal/web/export.go` to distinguish `ReasonNoLine` with `PropsSynced: true` ("isn't offering this prop right now, so add it by hand if it appears on their board.") from unsynced games ("has no line for this pick yet, so add it by hand.").
- Updated `slip-export` in `web/templates/partials.html` to render `MultiGameNote`, per-game combined taps with leg selections, prices, and drift notes alongside single-leg taps when `Groups` is populated, and the flat numbered list when every game has a single leg.
- Updated `TestExportCardIgnoresStaleOddsRows` and added `TestExportCardDraftKingsMultiGameWithCombinedGroup`, `TestExportCardDraftKingsMultiGameAllSingleLegs`, and `TestExportCardUnquotedPropVsUnsyncedGame` in `internal/web/export_test.go`.

### Task 7: Docs: update `docs/concepts/export.md` to document multi-game DraftKings per-game grouping, the single-game note, and the unquoted-prop manual note.

- Updated `docs/concepts/export.md` to document `BookPlan.Groups` (`LinkGroup`) and `BookPlan.MultiGameNote` for multi-game DraftKings per-game grouping.
- Documented the DraftKings single-game heads-up note and per-game combined button layout in the export card section.
- Documented `ManualLeg.PropsSynced` and the unquoted-prop manual note that distinguishes an unquoted market from an unsynced game.
- Added the `## Overview` Mermaid diagram and trimmed redundant inline code spans so `good-vibes docs lint` passes on `docs/concepts/export.md`.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
