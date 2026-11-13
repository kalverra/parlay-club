# Phase 1: Phase 1: Drain SSE streams and refresh workers on shutdown

## Done

### Task 1: Add `Stream.Shutdown()` in `internal/live/stream.go` so open SSE streams exit immediately on server stop without sending `bye`, and test SSE + normal HTTP shutdown together.

- Added `Stream.done` plus `Stream.Shutdown()` in `internal/live/stream.go`. `Shutdown` closes the channel once via
  `sync.Once`, and `ServeSlip` selects on it and returns without `event: bye`.
- Gave `serve` and `listenAndServe` in `cmd/server/main.go` an optional `beforeShutdown func()` hook, called after the
  serve context cancels and before `http.Server.Shutdown`. `run` passes `stream.Shutdown`.
- Added `TestStreamShutdownEndsOpenStreamsWithoutBye` (two open streams, idempotent double shutdown, empty body) and
  `TestStreamServesAfterShutdownWithoutHanging` (a stream opened after shutdown returns a `200` header in under
  100 ms).
- Added `TestServeDrainsOpenStreamAndInflightRequestsOnShutdown` in `cmd/server/main_test.go`: an open SSE stream and a
  blocked slow request drain together, the stream body stays empty (no `bye`), and the drain still waits for the slow
  handler before `serve` returns.
- Documented the shutdown behavior in `docs/concepts/live-updates.md` under a new "Shutdown" subsection.

### Task 2: Add lifecycle cancellation and `Service.Shutdown(ctx)` in `internal/refresh/refresh.go` (and matching `sandboxRuntime.Shutdown(ctx)` in `cmd/server/sandbox.go`), wire both into `serve` in `cmd/server/main.go` before `database.Close()`, and test cancellation and timeout behavior.

- Added lifecycle cancellation to `refresh.Service`. `New` builds a `stopCtx`/`stopCancel` pair, `KickContext` refuses new
  passes once it is cancelled, and every source's per-source timeout context also cancels on the stop via
  `context.AfterFunc` (unregistered when the source returns). `Shutdown(ctx)` cancels, waits on the service's
  `WaitGroup`, and returns `ctx.Err()` when the caller's deadline expires first. A shutdown-cancelled source logs no
  outage and leaves no failure backoff.
- Gave `sandboxRuntime` a `stopCtx`/`kickWg` pair and `Shutdown(ctx)`: `Kick` refuses new passes after a stop, its
  serialized pass runs `RunOnce` on `stopCtx` so it aborts on shutdown, and `Shutdown` waits for that pass and then drains
  the refresher it owns.
- Replaced `serve`'s single hook with a `shutdownHooks{beforeHTTP, afterHTTP}` value. `beforeHTTP` (`stream.Shutdown`)
  runs first, `afterHTTP` (`refresher.Shutdown` / `runtime.Shutdown`) runs with the same 8-second `shutdownCtx` right
  after `http.Server.Shutdown`, and it still runs when the HTTP drain expired. `run` wires `drain` per mode and
  `database.Close()` still happens in `run`'s defer, after `serve` returns.
- Tests: `TestShutdownCancelsAnInFlightSync`, `TestKickAfterShutdownStartsNoWork`, and
  `TestShutdownReturnsTheDeadlineWhenASourceIgnoresIt` in `internal/refresh/shutdown_test.go`;
  `TestSandboxShutdownCancelsPageLoadKick` and `TestSandboxShutdownReportsTheDrainDeadline` in `cmd/server`;
  `TestServeDrainsOpenStreamAndInflightRequestsOnShutdown` now runs a real `refresh.Service` whose slow fetch must stay
  alive until the HTTP drain ends, and `TestServeReportsTheRefreshDrainError` proves a drain timeout fails the serve.
- Documented the drain in `docs/concepts/data-refresh.md` ("Shutdown" plus guardrails) and corrected the shutdown order in
  `docs/concepts/persistence.md`.

### Task 3: Docs: update `docs/concepts/live-updates.md` and `docs/concepts/data-refresh.md` for shutdown behavior.

- `docs/concepts/live-updates.md` now has a full "Shutdown" section: `Stream.Shutdown` closes the `done` channel once,
  every open `ServeSlip` returns without `bye`, `EventSource` reconnects on its own, a second `Shutdown` is safe, and a
  stream opened after it returns closes at once. It also explains why streams end before `http.Server.Shutdown` (that
  call waits on handlers without cancelling their request contexts) and links the 8 second budget to Persistence and
  Data refresh.
- `docs/concepts/data-refresh.md` "Shutdown" now gives the order end streams -> drain HTTP -> drain refresh -> close the
  database, says the stop check and goroutine start are one step so no pass slips past the wait, treats a cancelled
  source as a stop rather than a failure, notes the refresh drain still runs when the HTTP drain expires, and records
  that `sandboxRuntime.Shutdown` drains the page-load pass while stage and reset syncs are covered by the HTTP drain.
- Added `cmd/server/main.go` to the `live-updates` front-matter sources, regenerated `docs/llms.txt`, and ran
  `good-vibes docs lint` (0 errors; the 11 stale warnings are pre-existing in unrelated docs).

## Deviations

### Task 1

None. The design's pre-HTTP hook parameter is what landed; the post-HTTP refresh drain belongs to the later task.

### Task 2

- The design had `KickContext` derive one cancellable context for the whole pass. That kills the pass's child source
  goroutines, because `kick` returns as soon as it starts them. The cancellation now lives in `refreshWithTimeout`, which
  is the chokepoint for every source, so a stop ends the fetch without cutting off the pass's children. `KickContext`
  still refuses new passes. The design's approach bullet is corrected in place.
- Task 1 noted the hook signature would have to grow; the two hooks are grouped in a `shutdownHooks` struct instead of
  adding a sixth parameter.

### Task 3

None. Task 1 and task 2 had already added draft shutdown prose to both docs; this task checked it against the code and
completed it rather than duplicating it.

## Open issues

### Task 1

- `serve` takes the hook as a plain `func()`. The later task that calls `refresher.Shutdown` needs a context-aware
  drain, so it will have to extend the signature or group the hooks into a small struct then.
- `Stream` relies on `NewStream` to allocate `done`. A zero-value `Stream` would panic on `Shutdown` (nil channel
  close) and block `ServeSlip` on a nil channel. Every caller uses `NewStream`, so this is only a guard if someone
  starts constructing the struct directly.

### Task 2

- `refresh.Service.Shutdown` and `sandboxRuntime.Shutdown` leave one waiter goroutine behind when the caller's deadline
  expires. It exits as soon as the stuck work finishes, which is the point of the deadline, so the leak is bounded.
- A drain that expires while a sync is still writing makes `serve` return an error and `main` exit 1. `database.Close()`
  then waits for that query rather than failing fast; Cloud Run's hard kill is the backstop.
- Sandbox tests that boot a runtime without calling `Shutdown` rely on `t.Context()` cancelling the runtime's `stopCtx` at
  cleanup to end a page-load pass. The database is closed in the same cleanup, so a race there would only log.
- `waitFor` in `internal/refresh` and `waitForGroup` in `cmd/server` are the same small helper duplicated to avoid a new
  package.

### Task 3

- None for the docs. `good-vibes check` still reports the pre-existing stale-source warnings in access, analyzer,
  export, gcp setup, local dev, and other docs; they are outside this change.

## Next-phase notes

### Task 1

- The shutdown order is now: cancel signal -> `beforeShutdown()` (streams) -> `server.Shutdown(shutdownCtx)` ->
  `serve` returns -> deferred `database.Close()` in `run`.
- Put the refresh drain between `server.Shutdown` and the return from `serve` so it shares the same `shutdownCtx`, as
  the design's sequence diagram shows.
- `TestServeDrainsOpenStreamAndInflightRequestsOnShutdown` is the place to extend when the refresh drain lands: pass the
  real runtime drain as the post-HTTP hook and assert it runs before `serve` returns.

### Task 2

- The shutdown order is: signal -> `stream.Shutdown()` -> `server.Shutdown(shutdownCtx)` -> `afterHTTP(shutdownCtx)` ->
  `serve` returns -> deferred `database.Close()`. Later drains belong in `afterHTTP` so they share the 8-second budget.
- Anything new that writes in the background must join `refresh.Service.wg` (or `sandboxRuntime.kickWg`), or the drain
  cannot see it.
- If a later task needs a faster or configurable drain, `shutdownTimeout` in `cmd/server/main.go` is the single knob, and
  `TestServeReportsTheRefreshDrainError` assumes `serve` propagates the hook's error as-is.

### Task 3

- Phase 2 rewrites `GET /healthz` and passes a database handle into `newMux`. Keep the `shutdownHooks` value and the
  `serve` signature as they are; `beforeHTTP` and `afterHTTP` are the only shutdown seam now.
- `config.Load` and `auth.NewGoogleProvider` both run before `serve` on the same signal context, so a SIGTERM during
  startup already cancels OIDC discovery. No extra drain work is needed there.
- The docs now say a drain timeout exits the process non-zero (`context.DeadlineExceeded` from `Shutdown`). Keep that
  behavior if later tasks touch `serve`'s error handling.

## Files touched

- `cmd/server/main.go`
- `cmd/server/main_test.go`
- `cmd/server/sandbox.go`
- `cmd/server/sandbox_test.go`
- `docs/concepts/data-refresh.md`
- `docs/concepts/live-updates.md`
- `docs/concepts/persistence.md`
- `docs/llms.txt`
- `internal/live/stream.go`
- `internal/live/stream_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/shutdown_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
