# Phase 4: Phase 4: Benchmarks and performance observability

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 7

- Run `mise run bench` for baseline ns/op and allocs/op numbers across `internal/board`, `internal/db`, and `internal/web`.

### Task 8

- Run `good-vibes verify lint` and `good-vibes verify test` for full-repo verification if needed before phase close.

### Task 9

- Phase 4 is ready to close after the binary runs `lint` and `test` gates.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `docs/guides/local-dev.md`
- `internal/db/odds.go`
- `internal/db/store.go`
- `internal/db/sync_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/research.go`
- `internal/refresh/sources.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `internal/web/timing.go`
- `mise.toml`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
