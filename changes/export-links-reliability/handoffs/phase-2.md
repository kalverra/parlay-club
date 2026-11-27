# Phase 2: Phase 2: Extend lock-time enrichment to game lines and tune credit floor

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 4

`OddsLinesEnrichSource` (`"odds:lines:enrich"`) is now recorded in `sync_state` when lock-time game-line enrichment buys Odds API lines.

### Task 5

Run the 5-case phone verification matrix on iOS and Android for both DraftKings and FanDuel after deploying.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/concepts/export.md`
- `docs/reference/data-sources.md`
- `internal/refresh/pick_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/sources.go`

## Run log

- Mode: 
- Checkpoints: task-4, task-5
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
