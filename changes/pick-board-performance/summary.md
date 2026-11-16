---
slug: pick-board-performance
run: 20261116-195007-pick-board-performance
outcome: done
generated: 2026-11-16T20:37:09Z
---

# Run summary: pick-board-performance

## Outcome

- Status: done
- Branch: pick-board-performance
- Final verdict: agy@gemini-4-argon-high approved (issues: 0)

## Totals

- Tasks: 9 of 9
- Phases: 4
- Checkpoints: 10
- Duration: 29m40s
- Cost: $—
- Fresh input tokens: 494706
- Cache read tokens: 5874380
- Cache write tokens: 0
- Output tokens: 76320

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 6445406 | $— |

## Phases

### Phase 1: Pre-index roster lookups and scope `pickPageData`

Tasks:

- [x] Replace the linear roster scan in `internal/board/board.go` (`Build`, `Slates`, `matchPlayer`, `gameMatchesQuery`) with a pre-normalized `rosterIndex` built once per call, with a prod-scale benchmark in `internal/board/board_test.go`.
- [x] Trim redundant work in `internal/web/pick.go` (`pickSlate` skips `dockFrom`/`Detail`, `pickDetail` loads only the open game's odds/roster, `pickPageData` avoids calling `board.Slates` twice when `q.Q == ""`), with handler tests in `internal/web/pick_test.go`.

Phase totals: 2 checkpoints, 6m15s, $—, 165434 input / 1992 output / 114616 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Replace the linear roster scan in `internal/board/board.go` (`Build`, `Slates`, `matchPlayer`, `gameMatchesQuery`) with a pre-normalized `rosterIndex` built once per call, with a prod-scale benchmark in `internal/board/board_test.go`.

- Added `rosterIndex`, `rosterEntry`, `newRosterIndex`, and `addLookup` in `internal/board/board.go` to pre-normalize roster names once per call.
- Updated `Build`, `matchPlayer`, `Slates`, and `gameMatchesQuery` in `internal/board/board.go` to use `rosterIndex` (`Slates` skips building the index when `q.Q` normalizes to empty).
- Added `TestBuildMatchPlayerTeamPriorityAndFallback` and prod-scale benchmarks (`BenchmarkBuild`, `BenchmarkSlates`, `BenchmarkDetail`) in `internal/board/board_test.go`.

### Task 2: Trim redundant work in `internal/web/pick.go` (`pickSlate` skips `dockFrom`/`Detail`, `pickDetail` loads only the open game's odds/roster, `pickPageData` avoids calling `board.Slates` twice when `q.Q == ""`), with handler tests in `internal/web/pick_test.go`.

- Updated `pickSlate` in `internal/web/pick.go` to call `pickSlateData`, skipping `LegsForSlip`, `dockFrom`, `boardSignalsForGame`, and `board.Detail`.
- Updated `pickDetail` in `internal/web/pick.go` to call `pickDetailData`, which loads only the open game (`GameByID`, `OddsForGame`, `RosterForTeams([]string{game.AwayTeam, game.HomeTeam})`, `LegsForSlip`), checks week and `SlateScope`, and skips `dockFrom` and other games' odds/rosters.
- Extracted `buildPickSlate` and `buildPickDetail` helpers in `internal/web/pick.go`; `buildPickSlate` skips the second `board.Slates` call when `q.Q == ""`.
- Added handler tests in `internal/web/pick_test.go` (`TestPickSlateSkipsDockAndDetail`, `TestPickDetailScopesToOpenGameAndFiltersScope`, `TestPickPageDataSingleSlatesPassWhenSearchEmpty`).

### Phase 2: Separate SQLite WAL read pool and batch `UpsertOdds`

Tasks:

- [x] Update `internal/db/db.go` and `internal/db/store.go` to open a multi-connection `query_only=ON` read pool alongside the single-writer handle (falling back to one handle for `:memory:` DBs), prepare statements once in `UpsertOdds`, and test concurrent read-during-write in `internal/db/db_test.go`.

Phase totals: 1 checkpoints, 6m8s, $—, 82717 input / 996 output / 57308 cache read / 0 cache write tokens

Checkpoints: task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Update `internal/db/db.go` and `internal/db/store.go` to open a multi-connection `query_only=ON` read pool alongside the single-writer handle (falling back to one handle for `:memory:` DBs), prepare statements once in `UpsertOdds`, and test concurrent read-during-write in `internal/db/db_test.go`.

- Updated `internal/db/db.go` to support `:memory:` DSNs without creating disk directories and added `openReadDB` for a 4-connection `query_only(ON)` WAL read pool.
- Updated `internal/db/store.go` so `NewStore` inspects `PRAGMA database_list`, opens the separate read pool for file-backed databases while falling back to the single writer handle for `:memory:` DBs, exposes `ReadDB()` and `Close()`, and routes read-only queries outside transactions through `s.readConn()`.
- Updated `UpsertOdds` in `internal/db/odds.go` to prepare the upsert statement once per transaction via `tx.conn().PrepareContext` (added to `queryer`).
- Added `TestStoreOpensReadPoolForFileDB`, `TestStoreFallsBackToSingleHandleForMemoryDB`, and `TestConcurrentReadsDuringWrite` in `internal/db/db_test.go`.

### Phase 3: Move automatic pick-time refreshes off the request path

Tasks:

- [x] Add non-blocking `KickPickLines` and `KickPickGame` methods in `internal/refresh/refresh.go` (deduplicated via `singleflight`, tracked in `s.wg`, publishing `publishWeek` on change) and wire `internal/web/pick.go` to trigger them without blocking responses, with async tests in `internal/refresh/pick_test.go` and `internal/web/pick_test.go`.
- [x] Add HTMX `hx-sync` / search debounce guards in `web/templates/partials.html` and `web/static/app.js` so SSE `board-updated` and rapid typing do not stack redundant partial requests.
- [x] Docs: update `docs/reference/schema.md` (WAL read/write connection pools) and `docs/concepts/data-refresh.md` (async pick-time refresh flow).

Phase totals: 3 checkpoints, 7m17s, $—, 248151 input / 2988 output / 171924 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Add non-blocking `KickPickLines` and `KickPickGame` methods in `internal/refresh/refresh.go` (deduplicated via `singleflight`, tracked in `s.wg`, publishing `publishWeek` on change) and wire `internal/web/pick.go` to trigger them without blocking responses, with async tests in `internal/refresh/pick_test.go` and `internal/web/pick_test.go`.

- Added non-blocking `KickPickLines` and `KickPickGame` (backed by `kickPick`) in `internal/refresh/refresh.go`, guarding on `s.stopping()` under `s.stopMu`, detaching request cancellation with `context.WithoutCancel`, tracking goroutines in `s.wg`, cancelling on `s.stopCtx`, and delegating to `SyncPickLines` / `SyncPickGame` (which deduplicate via `s.flight` and call `s.publishWeek` when rows change).
- Replaced `SyncPickLines` / `SyncPickGame` on `web.Refresher` in `internal/web/routes.go` with `KickPickLines` / `KickPickGame` and wired `syncPickIfOpen` in `internal/web/pick.go` to trigger them without blocking responses (keeping `ForcePickRefresh` synchronous).
- Added async tests `TestKickPickLinesAsyncSingleFlightAndPublish` and `TestKickPickGameAsyncSingleFlightAndPublish` in `internal/refresh/pick_test.go`.
- Updated `internal/web` refresher stubs and `TestPickRefreshCooldownAndRepeatedSearchWithService`, and added `TestPickBoardAndDetailReturnCachedRowsWhileAsyncRefreshPublishes` in `internal/web/pick_test.go`.

### Task 5: Add HTMX `hx-sync` / search debounce guards in `web/templates/partials.html` and `web/static/app.js` so SSE `board-updated` and rapid typing do not stack redundant partial requests.

- Added `hx-sync="this:drop"` and `hx-disinherit` (`hx-include hx-sync` on `#pick-slate`, `hx-sync` on `#pick-detail` and `#slip-panel`) in `web/templates/partials.html` so duplicate `board-updated` refreshes do not stack while child controls remain responsive.
- Added `hx-sync="this:replace"` on the `#pick-slate` search `<form>` (`hx-trigger="input changed delay:250ms, submit"`) in `web/templates/partials.html` so rapid typing or submit replaces any in-flight `/pick/results` request.
- Added `triggerBoardUpdated` (150ms debounce via `boardUpdateTimer`, cleared when the tab becomes hidden) in `web/static/app.js` for SSE `board-updated` events and `refreshLive()` so simultaneous `visibilitychange` + `focus` or burst SSE signals collapse into a single `board-updated` trigger.
- Added assertions in `TestPickBoardRendersOpenSlateAndDetail` in `internal/web/pick_test.go` covering the `hx-sync` and `hx-disinherit` attributes.

### Task 6: Docs: update `docs/reference/schema.md` (WAL read/write connection pools) and `docs/concepts/data-refresh.md` (async pick-time refresh flow).

- Updated `docs/reference/schema.md` (`## Connection settings`) to document the single-writer handle (`SetMaxOpenConns(1)`, `_txlock=immediate`), the file-backed `query_only=ON` read pool (`SetMaxOpenConns(4)`, `SetMaxIdleConns(4)`) opened by `db.NewStore`, `:memory:` handle sharing, and `Store.Close`.
- Updated `docs/concepts/data-refresh.md` (`## What kicks a sync` and `## Odds credits`) to document the non-blocking `KickPickLines` and `KickPickGame` pick-time refresh flow (`context.WithoutCancel`, `s.wg`, `singleflight` via `SyncPickLines`/`SyncPickGame`, shutdown cancellation, and `board-updated` week publication on change) alongside synchronous `ForcePickRefresh`, `SyncGameProps`, and research syncs.
- Verified documentation with `good-vibes docs lint`.

### Phase 4: Benchmarks and performance observability

Tasks:

- [x] Add prod-scale `Benchmark*` tests across `internal/board/board_test.go` (`Build`, `Slates`, `Detail`), `internal/db/sync_test.go` (`OddsForWeek`, `UpsertOdds`, concurrent read-during-write), and `internal/web/pick_test.go` (`pickBoard`, `pickDetail`, `pickSlate`), plus a `mise` task (`bench`) in `mise.toml`.
- [x] Add slow-request and phase-timing middleware (`db_ms`, `board_ms`, `render_ms`, `Server-Timing` in non-prod), slow SQLite/refresh `zerolog` timing logs, and non-prod-only `/debug/pprof/` routes in `cmd/server`, with tests in `cmd/server/main_test.go`.
- [x] Docs: update `docs/guides/local-dev.md` with benchmark and `pprof` / `Server-Timing` debugging commands.

Phase totals: 4 checkpoints, 10m0s, $—, 5872784 input / 70344 output / 5530532 cache read / 0 cache write tokens

Checkpoints: task-7, task-8, task-9, final Review fixes: 0 Integration checks: 0

#### Done

### Task 7: Add prod-scale `Benchmark*` tests across `internal/board/board_test.go` (`Build`, `Slates`, `Detail`), `internal/db/sync_test.go` (`OddsForWeek`, `UpsertOdds`, concurrent read-during-write), and `internal/web/pick_test.go` (`pickBoard`, `pickDetail`, `pickSlate`), plus a `mise` task (`bench`) in `mise.toml`.

- Verified existing prod-scale benchmarks in `internal/board/board_test.go` (`BenchmarkBuild`, `BenchmarkSlates`, `BenchmarkDetail`) seeded with 14 games, 15,500 odds rows, and 2,100 roster rows.
- Added prod-scale benchmarks in `internal/db/sync_test.go`: `BenchmarkOddsForWeek` (15,500 rows across 14 games), `BenchmarkUpsertOdds` (1,200-row prop batch), and `BenchmarkConcurrentReadsDuringWrite` (parallel `OddsForWeek` reads during continuous `UpsertOdds` writes).
- Added prod-scale handler benchmarks in `internal/web/pick_test.go`: `BenchmarkPickBoard`, `BenchmarkPickDetail`, and `BenchmarkPickSlate` covering full SQLite load, board build, and HTML template rendering.
- Added `[tasks.bench]` to `mise.toml` running `go test -run='^$' -bench=. -benchmem ./internal/board ./internal/db ./internal/web`.

### Task 8: Add slow-request and phase-timing middleware (`db_ms`, `board_ms`, `render_ms`, `Server-Timing` in non-prod), slow SQLite/refresh `zerolog` timing logs, and non-prod-only `/debug/pprof/` routes in `cmd/server`, with tests in `cmd/server/main_test.go`.

- Added request-scoped `PhaseTimings` tracker (`internal/web/timing.go`) and instrumented `pickPageData`, `pickSlateData`, `pickDetailData` (`internal/web/pick.go`), and `renderPageCtx`/`renderPartialCtx` (`internal/web/slips.go`) to record `db`, `board`, and `render` durations.
- Added `withRequestTiming` middleware and `withPprof` wrapper in `cmd/server/main.go`: logs HTTP requests with `duration_ms`, `db_ms`, `board_ms`, and `render_ms` (`WARN` at `>= 250ms`, `DEBUG` otherwise), skips `/events` SSE streams, emits `Server-Timing` headers in non-prod, and exposes `/debug/pprof/` only when `!cfg.IsProd()` (returning `404` in prod).
- Added `Store.WithLogger` in `internal/db/store.go` and slow-query `zerolog` `WARN` logs (`>= 100ms`) in `Store.Tx`, `Store.UpsertOdds`, and `Store.OddsForWeek` (`internal/db/odds.go`), plus `zerolog` duration/row-count logs in `internal/refresh` (`kick`, `syncOdds`, `syncOddsProps`).
- Added `TestRequestTimingMiddleware` and `TestPprofRoutes` in `cmd/server/main_test.go`.

### Task 9: Docs: update `docs/guides/local-dev.md` with benchmark and `pprof` / `Server-Timing` debugging commands.

- Added a **Benchmarks and profiling** section to `docs/guides/local-dev.md` documenting `mise run bench`, targeted `go test -bench` runs, non-prod `Server-Timing` headers and request/SQLite slow-path logs, and non-prod `/debug/pprof/` `go tool pprof` commands.
- Removed the retired `sources` and `last_verified` front-matter fields from `docs/guides/local-dev.md` so `good-vibes docs lint` passes with zero errors, and regenerated `docs/llms.txt` via `good-vibes docs index`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: no commands configured
- Open issues: none
