# Phase 3: Phase 3: Cache invalidation, CLI market mode, and `/analyzer/results` UI

## Done

### Task 5: Extend `eval.Cache` with `GetOrComputeMode` and `InvalidateMarket`, wire `refresh.Service.SetOnMarketDataChange` (on newly `FINAL` games in `syncScoreboard` and `syncNFLverse` changes) in `cmd/server/main.go`, and add `--mode slips|market` plus sharp/L5-fallback coverage output to `cmd/eval/main.go` with cache, refresh, and CLI tests.

- Extended `eval.Cache` in `internal/eval/cache.go` with `GetOrComputeMode` and `InvalidateMarket`, keeping `ModeSlips` and `ModeMarket` entries and generation counters isolated while `InvalidateAll` clears both.
- Added `refresh.Service.SetOnMarketDataChange` in `internal/refresh/refresh.go`, firing on newly `FINAL` games in `syncScoreboard` (`internal/refresh/sources.go`) and committed changes in `syncNFLverse` (`internal/refresh/nflverse.go`).
- Wired `refresher.SetOnMarketDataChange(evalCache.InvalidateMarket)` in both sandbox and production setup in `cmd/server/main.go`.
- Added `--mode slips|market` (rejecting invalid modes and `--mode` with `--dataset`), `sharp` and `l5_fallback` coverage output, and empty `SlipBands` / `SlipBandDiffs` section omission to `cmd/eval/main.go`.
- Added cache mode-isolation/invalidation tests (`internal/eval/cache_test.go`), refresh callback tests (`internal/refresh/sources_test.go`, `internal/refresh/nflverse_test.go`), and CLI market-mode tests (`cmd/eval/main_test.go`).

### Task 6: Add `?source=slips|market` to `GET /analyzer/results` (`internal/web/eval_results.go`) and phone-first source switcher pills plus sharp/L5-fallback coverage tiles to `web/templates/analyzer_results.html` (hiding slip-only sections in market mode and preserving `?source=` on filter URLs), run `go generate ./...`, and add HTTP tests.

- Added `Store.FinalSeasons` in `internal/db/record.go` to query seasons with `FINAL` games, newest first.
- Added `?source=slips|market` parsing (returning `400 Bad Request` on invalid values), `EvalCache.GetOrComputeMode` lookup, `FINAL`-game default season resolution, `Source`/`IsMarket`/`SlipsURL`/`Sources` view state, and `WithSharp`/`WithL5Fallback` coverage fields to `internal/web/eval_results.go`, preserving `?source=` on all season and rubric filter URLs.
- Updated `web/templates/analyzer_results.html` with phone-first `User slips` / `Market-wide` source switcher pills, market-mode empty-state copy, hidden `Slip Calibration (A-F)` and `Slip Factor Attribution` sections (with a link note to `User slips`), and `nflverse L5 fallback` + `Sharp benchmarks` tiles in `Pre-Kickoff Data Coverage`, then ran `go generate ./...`.
- Added HTTP tests in `internal/web/eval_results_test.go` covering invalid `?source=` handling, empty market state, `FINAL`-game default season resolution, hidden slip-only sections, `?source=` preservation on filter URLs, and `InvalidateMarket` cache eviction.

### Task 7: Docs: update `docs/concepts/analyzer.md`, `docs/reference/routes.md`, and `docs/guides/local-dev.md` for market-wide grading evaluation, sharp/L5 signal reconstruction, `--mode market`, and `?source=market`, and run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` to document pre-kickoff sharp benchmark and prior-week nflverse L5 fallback reconstruction, `ModeMarket` consensus deduplication and offline deterministic grading, `?source=slips|market` on `/analyzer/results`, and mode-isolated `eval.Cache` invalidation.
- Updated `docs/reference/routes.md` to document `?source=slips|market` on `GET /analyzer/results`, `FinalSeasons` default season resolution, `400 Bad Request` handling, hidden slip-only sections, extra coverage tiles, and `InvalidateMarket` cache eviction.
- Updated `docs/guides/local-dev.md` to document `cmd/eval --mode slips|market`, `sharp` and `l5_fallback` coverage counts, `mise run eval-prod -- --mode market --all`, and removed the stale `./data/` backtick path lint error.
- Ran `good-vibes docs lint`, passing with `0 error(s)`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 5

`eval.Cache.GetOrComputeMode(ctx, store, mode, groupID, season)` is ready for `/analyzer/results?source=market` in Task 6 (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`).

### Task 6

`GET /analyzer/results?source=market` is wired end-to-end and verified with `go generate ./...` and `internal/web` HTTP tests.

### Task 7

All Phase 3 tasks and living doc updates are complete.

## Files touched

- `cmd/eval/main.go`
- `cmd/eval/main_test.go`
- `cmd/server/main.go`
- `docs/concepts/analyzer.md`
- `docs/guides/local-dev.md`
- `docs/reference/routes.md`
- `internal/db/record.go`
- `internal/eval/cache.go`
- `internal/eval/cache_test.go`
- `internal/refresh/nflverse.go`
- `internal/refresh/nflverse_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/sources.go`
- `internal/refresh/sources_test.go`
- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `web/static/app.css`
- `web/templates/analyzer_results.html`

## Run log

- Mode: 
- Checkpoints: task-5, task-6, task-7
- Verify: verify passed: test 700ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
