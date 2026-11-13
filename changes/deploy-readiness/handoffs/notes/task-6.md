# Task 6 notes

## Done

- `cmd/server/main.go` replaces the static `healthz` with `newHealthzHandler(database *sql.DB, logger zerolog.Logger)`.
  Each probe pings SQLite under a `healthzPingTimeout` of 2 seconds derived from `r.Context()`, answers `200 ok` when
  the ping succeeds, and logs the error and answers `503 unavailable` with `text/plain; charset=utf-8` when it fails.
  The body never carries the SQLite error.
- `newMux` now takes the health handler next to the static and screenshot handlers and registers it with `mux.Handle` on
  `GET /healthz`, so `run` builds it once with the real database.
- `cmd/server/main_test.go` rewrites `TestHealthz`: an open handle answers `200 ok`, and after `database.Close()` the
  same mux answers `503 unavailable` with a generic body. Every other `newMux` call site, in `main_test.go` and
  `cmd/server/sandbox_test.go`, passes `http.NotFoundHandler()` for the new argument.
- `docs/reference/routes.md` describes the probe rule in the public-routes row and in prose, bumps `last_verified` to
  2026-11-13, and `docs/decisions/foundation.md` no longer claims `/healthz` never queries the database.

## Deviations

None. The handler signature, timeout, status codes, and body words match the design.

## Open issues

- The test proves the failure path with a closed handle. Nothing exercises the 2-second deadline itself, such as a busy
  `MaxOpenConns(1)` connection; the timeout is only pinned by inspection.
- A closed database answers 503, but `database.Close()` in `run` runs only after `serve` returns, so the probe cannot
  observe that state during a normal shutdown. It is a safety net for a handle that fails early.
- The probe is a write-free `PingContext`, so it does not detect a database that answers but cannot write. Litestream
  replication health is still outside the probe.

## Next-phase notes

- The deploy workflow's `/healthz` check now fails on a revision whose database does not answer, not only on one that
  never booted. Keep the retry loop: the first probes can arrive before migrations finish.
- `newMux` has nine parameters now. A later task that needs another handler should group the public handlers instead of
  adding a tenth.
- `healthzPingTimeout` in `cmd/server/main.go` is the single knob if Cloud Run probes need more or less room.
