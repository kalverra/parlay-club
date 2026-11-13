# Task 1: Stream shutdown

## Done

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

## Deviations

None. The design's pre-HTTP hook parameter is what landed; the post-HTTP refresh drain belongs to the later task.

## Open issues

- `serve` takes the hook as a plain `func()`. The later task that calls `refresher.Shutdown` needs a context-aware
  drain, so it will have to extend the signature or group the hooks into a small struct then.
- `Stream` relies on `NewStream` to allocate `done`. A zero-value `Stream` would panic on `Shutdown` (nil channel
  close) and block `ServeSlip` on a nil channel. Every caller uses `NewStream`, so this is only a guard if someone
  starts constructing the struct directly.

## Next-phase notes

- The shutdown order is now: cancel signal -> `beforeShutdown()` (streams) -> `server.Shutdown(shutdownCtx)` ->
  `serve` returns -> deferred `database.Close()` in `run`.
- Put the refresh drain between `server.Shutdown` and the return from `serve` so it shares the same `shutdownCtx`, as
  the design's sequence diagram shows.
- `TestServeDrainsOpenStreamAndInflightRequestsOnShutdown` is the place to extend when the refresh drain lands: pass the
  real runtime drain as the post-HTTP hook and assert it runs before `serve` returns.
