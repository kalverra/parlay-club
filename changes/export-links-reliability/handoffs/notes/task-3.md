# Task 3

## Done

- Updated `fanDuelCombiner.Combine` in `internal/export/export.go` to extract each leg's FanDuel `marketId` jurisdiction prefix via `odds.FanDuelMarketPrefix` and return `"", false` (falling back to per-leg taps) when legs carry mismatched prefixes (including prefixed mixed with unprefixed `marketId`s).
- Added `HasPreferredFanDuelPrefix` in `internal/export/export.go` using `odds.FanDuelMarketPrefix` and `odds.PreferredFanDuelMarketPrefix` so non-US links can be gated or flagged cleanly via one constant if needed.
- Added unit tests in `internal/export/export_test.go` covering mixed `42.*`/`734.*`/`717.*` and unprefixed `marketId` combinations (verifying per-leg fallback), matching non-`42` prefixes, and `TestHasPreferredFanDuelPrefix`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`export.HasPreferredFanDuelPrefix` and `odds.FanDuelMarketPrefix` are ready for any Phase 2 lock-time enrichment checks on FanDuel legs.
