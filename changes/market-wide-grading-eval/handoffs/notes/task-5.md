## Done

- Extended `eval.Cache` in `internal/eval/cache.go` with `GetOrComputeMode` and `InvalidateMarket`, keeping `ModeSlips` and `ModeMarket` entries and generation counters isolated while `InvalidateAll` clears both.
- Added `refresh.Service.SetOnMarketDataChange` in `internal/refresh/refresh.go`, firing on newly `FINAL` games in `syncScoreboard` (`internal/refresh/sources.go`) and committed changes in `syncNFLverse` (`internal/refresh/nflverse.go`).
- Wired `refresher.SetOnMarketDataChange(evalCache.InvalidateMarket)` in both sandbox and production setup in `cmd/server/main.go`.
- Added `--mode slips|market` (rejecting invalid modes and `--mode` with `--dataset`), `sharp` and `l5_fallback` coverage output, and empty `SlipBands` / `SlipBandDiffs` section omission to `cmd/eval/main.go`.
- Added cache mode-isolation/invalidation tests (`internal/eval/cache_test.go`), refresh callback tests (`internal/refresh/sources_test.go`, `internal/refresh/nflverse_test.go`), and CLI market-mode tests (`cmd/eval/main_test.go`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`eval.Cache.GetOrComputeMode(ctx, store, mode, groupID, season)` is ready for `/analyzer/results?source=market` in Task 6 (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`).

## Commit

Add market mode to eval cache, refresh, and CLI

Isolating market-wide cache invalidation from user-slip settlement prevents unnecessary recomputation on unrelated updates, and exposing `--mode market` lets operators evaluate league-wide calibration offline before wiring the web UI.
