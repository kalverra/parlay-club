# Task 2: Refresh shutdown

## Done

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

## Deviations

- The design had `KickContext` derive one cancellable context for the whole pass. That kills the pass's child source
  goroutines, because `kick` returns as soon as it starts them. The cancellation now lives in `refreshWithTimeout`, which
  is the chokepoint for every source, so a stop ends the fetch without cutting off the pass's children. `KickContext`
  still refuses new passes. The design's approach bullet is corrected in place.
- Task 1 noted the hook signature would have to grow; the two hooks are grouped in a `shutdownHooks` struct instead of
  adding a sixth parameter.

## Open issues

- `refresh.Service.Shutdown` and `sandboxRuntime.Shutdown` leave one waiter goroutine behind when the caller's deadline
  expires. It exits as soon as the stuck work finishes, which is the point of the deadline, so the leak is bounded.
- A drain that expires while a sync is still writing makes `serve` return an error and `main` exit 1. `database.Close()`
  then waits for that query rather than failing fast; Cloud Run's hard kill is the backstop.
- Sandbox tests that boot a runtime without calling `Shutdown` rely on `t.Context()` cancelling the runtime's `stopCtx` at
  cleanup to end a page-load pass. The database is closed in the same cleanup, so a race there would only log.
- `waitFor` in `internal/refresh` and `waitForGroup` in `cmd/server` are the same small helper duplicated to avoid a new
  package.

## Next-phase notes

- The shutdown order is: signal -> `stream.Shutdown()` -> `server.Shutdown(shutdownCtx)` -> `afterHTTP(shutdownCtx)` ->
  `serve` returns -> deferred `database.Close()`. Later drains belong in `afterHTTP` so they share the 8-second budget.
- Anything new that writes in the background must join `refresh.Service.wg` (or `sandboxRuntime.kickWg`), or the drain
  cannot see it.
- If a later task needs a faster or configurable drain, `shutdownTimeout` in `cmd/server/main.go` is the single knob, and
  `TestServeReportsTheRefreshDrainError` assumes `serve` propagates the hook's error as-is.
