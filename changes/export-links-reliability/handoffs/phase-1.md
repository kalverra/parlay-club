# Phase 1: Phase 1: Preserve links on upsert and prefer valid FanDuel jurisdiction links

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

`Store.UpsertOdds` now assumes `NULL` means "no link supplied" and preserves existing links only when `odds.line IS excluded.line`. Callers that want to replace a non-preferred link (such as a non-`42.` FanDuel link) must supply the preferred non-null link in the merged row.

### Task 2

`odds.FanDuelMarketPrefix` and `odds.PreferredFanDuelMarketPrefix` are exported from `internal/ingest/odds` and can be reused directly by `internal/export` and `internal/refresh` (such as `fanDuelCombiner` and `syncSlipExportEnrich`) when checking FanDuel `marketId` prefixes.

### Task 3

`export.HasPreferredFanDuelPrefix` and `odds.FanDuelMarketPrefix` are ready for any Phase 2 lock-time enrichment checks on FanDuel legs.

## Files touched

- `internal/db/odds.go`
- `internal/db/odds_test.go`
- `internal/db/sync_test.go`
- `internal/export/export.go`
- `internal/export/export_test.go`
- `internal/ingest/odds/props.go`
- `internal/ingest/odds/props_test.go`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: generate 500ms, lint reused, test reused; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 1
