# Task 2

## Done

- Added `PreferredFanDuelMarketPrefix` (`"42"`), `FanDuelMarketPrefix`, and `PreferFanDuelLink` in `internal/ingest/odds/props.go` to parse numeric `marketId` jurisdiction prefixes and prefer `42.*` links over non-`42` prefixes (`734.*`, `717.*`, or unprefixed links).
- Updated `MergeProps` and `betterSingleSideProp` in `internal/ingest/odds/props.go` so FanDuel rows carrying non-`42` prefixes adopt `42.*` links (and matching `Sid`, `EventSid`, and moved `Line`/`Price`) during prop merging and primary deduplication.
- Updated `mergeOdds` in `internal/refresh/sources.go` to reuse `odds.PreferFanDuelLink` so game-line and enrichment merges replace non-`42` FanDuel links with `42.*` links when available.
- Added Week 12 mixed-prefix regression tests in `internal/ingest/odds/props_test.go` (`TestFanDuelMarketPrefixAndPreference`, `TestMergePropsPrefersFanDuel42OverNon42Week12`) and `internal/refresh/sources_test.go` (`TestMergeOddsPrefersFanDuel42OverNon42Week12`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`odds.FanDuelMarketPrefix` and `odds.PreferredFanDuelMarketPrefix` are exported from `internal/ingest/odds` and can be reused directly by `internal/export` and `internal/refresh` (such as `fanDuelCombiner` and `syncSlipExportEnrich`) when checking FanDuel `marketId` prefixes.

## Commit

Prefer 42.* FanDuel marketId links when merging odds

Parlay API returns mixed FanDuel marketId jurisdiction prefixes (such as 734.* and 717.*) that fail when opened or combined in the US. Preferring 42.* links from The Odds API during prop and game-line merges replaces non-US links before they reach storage or export.
