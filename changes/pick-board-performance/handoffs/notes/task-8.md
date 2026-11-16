## Done

- Added request-scoped `PhaseTimings` tracker (`internal/web/timing.go`) and instrumented `pickPageData`, `pickSlateData`, `pickDetailData` (`internal/web/pick.go`), and `renderPageCtx`/`renderPartialCtx` (`internal/web/slips.go`) to record `db`, `board`, and `render` durations.
- Added `withRequestTiming` middleware and `withPprof` wrapper in `cmd/server/main.go`: logs HTTP requests with `duration_ms`, `db_ms`, `board_ms`, and `render_ms` (`WARN` at `>= 250ms`, `DEBUG` otherwise), skips `/events` SSE streams, emits `Server-Timing` headers in non-prod, and exposes `/debug/pprof/` only when `!cfg.IsProd()` (returning `404` in prod).
- Added `Store.WithLogger` in `internal/db/store.go` and slow-query `zerolog` `WARN` logs (`>= 100ms`) in `Store.Tx`, `Store.UpsertOdds`, and `Store.OddsForWeek` (`internal/db/odds.go`), plus `zerolog` duration/row-count logs in `internal/refresh` (`kick`, `syncOdds`, `syncOddsProps`).
- Added `TestRequestTimingMiddleware` and `TestPprofRoutes` in `cmd/server/main_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Run `good-vibes verify lint` and `good-vibes verify test` for full-repo verification if needed before phase close.
