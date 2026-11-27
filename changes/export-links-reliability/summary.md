---
slug: export-links-reliability
run: 20261127-185536-export-links-reliability
outcome: done
generated: 2026-11-27T19:18:51Z
---

# Run summary: export-links-reliability

## Outcome

- Status: done
- Branch: export-links-reliability
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 2
- Checkpoints: 6
- Duration: 20m35s
- Cost: $—
- Fresh input tokens: 1061012
- Cache read tokens: 12121612
- Cache write tokens: 0
- Output tokens: 145820

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 9880082 | $— |
| gemini-4-neon-medium | 3448362 | $— |

## Phases

### Phase 1: Preserve links on upsert and prefer valid FanDuel jurisdiction links

Tasks:

- [x] Update `Store.UpsertOdds` in `internal/db/odds.go` to preserve stored `link`, `sid`, and `event_sid` when an incoming row has `NULL` on an unchanged line, and clear stale `link`/`sid` (while keeping `event_sid`) when the line moves, with unit tests in `internal/db/odds_test.go`.
- [x] Add FanDuel `marketId` prefix parsing and preference (`42.*` preferred over non-`42` prefixes like `734.*` / `717.*`) in `internal/ingest/odds/props.go` (`MergeProps`) and `internal/refresh/sources.go` (`mergeOdds`), plus regression tests using Week 12 mixed-prefix payloads.
- [x] Update `fanDuelCombiner.Combine` in `internal/export/export.go` to refuse combining legs with mismatched FanDuel `marketId` prefixes (falling back to per-leg taps), with unit tests in `internal/export/export_test.go`.

Phase totals: 3 checkpoints, 8m4s, $—, 1091384 input / 12703 output / 970771 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Update `Store.UpsertOdds` in `internal/db/odds.go` to preserve stored `link`, `sid`, and `event_sid` when an incoming row has `NULL` on an unchanged line, and clear stale `link`/`sid` (while keeping `event_sid`) when the line moves, with unit tests in `internal/db/odds_test.go`.

- Updated `Store.UpsertOdds` in `internal/db/odds.go` so `ON CONFLICT DO UPDATE` preserves stored `link` and `sid` when incoming values are `NULL` on an unchanged line (`odds.line IS excluded.line`), clears stale `link` and `sid` when the line moves, and preserves `event_sid` via `COALESCE(excluded.event_sid, odds.event_sid)`.
- Updated the `WHERE` clause in `Store.UpsertOdds` so incoming `NULL` link/sid/event_sid values on an unchanged row do not trigger spurious updates.
- Added `TestStoreUpsertOddsLinkPreservation` in `internal/db/odds_test.go` covering unchanged-line no-op, unchanged-line link preservation (both numeric and `nil` lines), moved-line `link`/`sid` clearing with `event_sid` preservation, and non-null overwrite; updated `TestUpsertOddsStoresAndRefreshesLinksWithPrices` in `internal/db/sync_test.go` to match the link-preserving contract.

### Task 2: Add FanDuel `marketId` prefix parsing and preference (`42.*` preferred over non-`42` prefixes like `734.*` / `717.*`) in `internal/ingest/odds/props.go` (`MergeProps`) and `internal/refresh/sources.go` (`mergeOdds`), plus regression tests using Week 12 mixed-prefix payloads.

- Added `PreferredFanDuelMarketPrefix` (`"42"`), `FanDuelMarketPrefix`, and `PreferFanDuelLink` in `internal/ingest/odds/props.go` to parse numeric `marketId` jurisdiction prefixes and prefer `42.*` links over non-`42` prefixes (`734.*`, `717.*`, or unprefixed links).
- Updated `MergeProps` and `betterSingleSideProp` in `internal/ingest/odds/props.go` so FanDuel rows carrying non-`42` prefixes adopt `42.*` links (and matching `Sid`, `EventSid`, and moved `Line`/`Price`) during prop merging and primary deduplication.
- Updated `mergeOdds` in `internal/refresh/sources.go` to reuse `odds.PreferFanDuelLink` so game-line and enrichment merges replace non-`42` FanDuel links with `42.*` links when available.
- Added Week 12 mixed-prefix regression tests in `internal/ingest/odds/props_test.go` (`TestFanDuelMarketPrefixAndPreference`, `TestMergePropsPrefersFanDuel42OverNon42Week12`) and `internal/refresh/sources_test.go` (`TestMergeOddsPrefersFanDuel42OverNon42Week12`).

### Task 3: Update `fanDuelCombiner.Combine` in `internal/export/export.go` to refuse combining legs with mismatched FanDuel `marketId` prefixes (falling back to per-leg taps), with unit tests in `internal/export/export_test.go`.

- Updated `fanDuelCombiner.Combine` in `internal/export/export.go` to extract each leg's FanDuel `marketId` jurisdiction prefix via `odds.FanDuelMarketPrefix` and return `"", false` (falling back to per-leg taps) when legs carry mismatched prefixes (including prefixed mixed with unprefixed `marketId`s).
- Added `HasPreferredFanDuelPrefix` in `internal/export/export.go` using `odds.FanDuelMarketPrefix` and `odds.PreferredFanDuelMarketPrefix` so non-US links can be gated or flagged cleanly via one constant if needed.
- Added unit tests in `internal/export/export_test.go` covering mixed `42.*`/`734.*`/`717.*` and unprefixed `marketId` combinations (verifying per-leg fallback), matching non-`42` prefixes, and `TestHasPreferredFanDuelPrefix`.

### Phase 2: Extend lock-time enrichment to game lines and tune credit floor

Tasks:

- [x] Lower `TheOddsAPIEnrichFloor` to `40` and extend `syncSlipExportEnrich` in `internal/refresh/refresh.go` to detect and enrich both game-line legs and prop legs that lack DraftKings links or carry non-`42` FanDuel `marketId` prefixes, with unit tests in `internal/refresh/pick_test.go`.
- [x] Docs: update `docs/concepts/export.md`, `docs/concepts/data-refresh.md`, and `docs/reference/data-sources.md` to document link-preserving upserts, FanDuel jurisdiction prefix handling, the updated enrichment floor, and the 5-case phone verification matrix.

Phase totals: 3 checkpoints, 11m53s, $—, 10768078 input / 119690 output / 9984704 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, final Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Lower `TheOddsAPIEnrichFloor` to `40` and extend `syncSlipExportEnrich` in `internal/refresh/refresh.go` to detect and enrich both game-line legs and prop legs that lack DraftKings links or carry non-`42` FanDuel `marketId` prefixes, with unit tests in `internal/refresh/pick_test.go`.

- Lowered `TheOddsAPIEnrichFloor` from `100` to `40` in `internal/refresh/refresh.go`.
- Extended `KickSlipExportEnrich` and `syncSlipExportEnrich` in `internal/refresh/refresh.go` (including `legNeedsExportEnrich`) to detect both game-line legs and prop legs lacking DraftKings links or carrying non-`42` FanDuel `marketId` prefixes via `odds.FanDuelMarketPrefix` and `odds.PreferredFanDuelMarketPrefix`.
- Added `OddsLinesEnrichSource`, `oddsLinesEnrichSource`, and `enrichWeekLines` in `internal/refresh/sources.go` to run a one-shot, `TheOddsAPIEnrichFloor`-gated Odds API lines enrichment pass when any upcoming game-line leg on a locked slip needs links.
- Updated `TestKickSlipExportEnrich` and added `TestKickSlipExportEnrichNonPreferredFanDuelAndGameLines` in `internal/refresh/pick_test.go` covering the `40` credit floor, non-`42` FanDuel prefixes, and game-line enrichment.

### Task 5: Docs: update `docs/concepts/export.md`, `docs/concepts/data-refresh.md`, and `docs/reference/data-sources.md` to document link-preserving upserts, FanDuel jurisdiction prefix handling, the updated enrichment floor, and the 5-case phone verification matrix.

- Updated `docs/concepts/export.md` to document link-preserving `Store.UpsertOdds` behavior, FanDuel `42.*` jurisdiction prefix preference and mixed-prefix combiner fallback, and the 5-case phone verification matrix.
- Updated `docs/concepts/data-refresh.md` to document lock-time game-line and prop export enrichment (`KickSlipExportEnrich` and `odds:lines:enrich`), the `40`-credit `TheOddsAPIEnrichFloor`, and FanDuel `42.*` link preference.
- Updated `docs/reference/data-sources.md` to document `40`-credit `TheOddsAPIEnrichFloor`, FanDuel `42.*` jurisdiction prefix handling, and link-preserving upserts.
- Verified with `good-vibes docs lint` (0 errors) and refreshed `docs/llms.txt` and `docs/decisions/llms.txt` via `good-vibes docs index`.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
