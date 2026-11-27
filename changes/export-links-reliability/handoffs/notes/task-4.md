## Done

- Lowered `TheOddsAPIEnrichFloor` from `100` to `40` in `internal/refresh/refresh.go`.
- Extended `KickSlipExportEnrich` and `syncSlipExportEnrich` in `internal/refresh/refresh.go` (including `legNeedsExportEnrich`) to detect both game-line legs and prop legs lacking DraftKings links or carrying non-`42` FanDuel `marketId` prefixes via `odds.FanDuelMarketPrefix` and `odds.PreferredFanDuelMarketPrefix`.
- Added `OddsLinesEnrichSource`, `oddsLinesEnrichSource`, and `enrichWeekLines` in `internal/refresh/sources.go` to run a one-shot, `TheOddsAPIEnrichFloor`-gated Odds API lines enrichment pass when any upcoming game-line leg on a locked slip needs links.
- Updated `TestKickSlipExportEnrich` and added `TestKickSlipExportEnrichNonPreferredFanDuelAndGameLines` in `internal/refresh/pick_test.go` covering the `40` credit floor, non-`42` FanDuel prefixes, and game-line enrichment.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`OddsLinesEnrichSource` (`"odds:lines:enrich"`) is now recorded in `sync_state` when lock-time game-line enrichment buys Odds API lines.
