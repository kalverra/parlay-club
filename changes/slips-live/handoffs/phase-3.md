# Phase 3: Live updates

## Done

- Added `internal/live` with two pieces:
  - `Hub` fans out string topics. `Subscribe(topic)` returns a receive-only channel with room for one signal and an
    idempotent unsubscribe func. `Publish(topic)` never blocks: a full buffer keeps the one queued wake-up and drops
    the rest, so bursts coalesce. The zero value is ready to use.
  - `Stream.ServeSlip(w, r, slipID, weekID)` serves one SSE connection. It subscribes to `slip:{id}` and
    `week:{weekID}`, sends `slip-updated` and `board-updated` signals with `data: {}`, pings every 25 seconds, and sets
    the streaming headers (`text/event-stream`, `no-cache`, `keep-alive`, `X-Accel-Buffering: no`) and flushes every
    frame.
  - Deadlines: a live window lives 55 minutes from stream start; otherwise the stream closes after 10 minutes without a
    slip or board event (pings do not reset it). Both paths send `event: bye` and return. Timers come from a `Clock` /
    `Timer` seam, so tests fire them by hand.
  - `Stream` takes an `IsLive(weekID) bool` func. Nothing wires a game-aware answer yet, so callers pass false until
    `slate-and-lines`.
- Added the `EventSource` client in `web/static/app.js`:
  - Opens one stream from the page's `data-live-url`.
  - `slip-updated` and `board-updated` trigger the matching events on `document.body` for HTMX partials.
  - `bye` closes the stream without retrying.
  - Hiding the tab closes the stream; becoming visible again or gaining focus reopens it and triggers both refreshes to
    cover missed events.
- Tests:
  - Hub: topic isolation, multiple subscribers, burst coalescing, unsubscribe (including idempotence and leaving other
    subscribers alone), re-subscribing after a topic empties, and concurrent publish/unsubscribe.
  - Stream: two subscribers both receive slip and board signals, both deadlines send `bye` and close the connection,
    pings arrive as comments, and the anti-buffering headers are present.
- Docs: added `docs/concepts/live-updates.md`, removed the Live updates section from `plan/architecture.md`, and updated
  the architecture, plan README, and `docs/llms.txt` pointers.

## Deviations

- The design called the live-window func `LiveWindow(weekID) bool`; the implementation names the type `IsLive` and takes
  a `Clock` option as well. The behavior is the same, but handlers in phase 4 will pass `live.IsLive`.
- The stream subscribes to both `slip:{id}` and `week:{weekID}` because the design lists both in the deleted
  architecture section. Nothing publishes week topics until `internal/refresh` lands.
- The idle deadline resets on slip and board events, not on pings, so a quiet connection still closes on schedule.
- `ServeSlip` takes the slip and week ids as arguments. Route parsing and the `data-live-url` body attribute stay in the
  phase 4 handler and template.
- There is no automated browser test for the client. The repository has no JS test runner, so the visibility
  close/reopen path is covered by the handler-side stream tests and a manual check; the server behavior it depends on
  is tested.

## Open issues

- The stream is not wired to a route or server yet. Phase 4 must add `GET /slips/{id}/events`, call `ServeSlip`, and
  set `data-live-url` on the slip page.
- `IsLive` has no game-aware implementation. Until `slate-and-lines`, the server should pass a func that returns false;
  otherwise every stream takes the longer live deadline.
- No `week:{weekID}` publisher exists. `internal/refresh` must publish it when a sync changes games or odds, and should
  also publish affected `slip:{id}` topics because odds and analyzer output depend on game data.
- The client's refresh behavior needs the phase 4 partials to listen with `hx-trigger="slip-updated from:body"` and
  `board-updated from:body`. Until those exist, the events fire with nothing listening.
- An open stream keeps the instance billed, so do not remove the deadlines or the visibility close while wiring the
  route.

## Next-phase notes

- Wiring shape: `live.NewStream(hub, isLive, live.WithClock(clock))`, then `stream.ServeSlip(w, r, slipID, weekID)`.
  `WithClock` is test-only; production uses the wall clock.
- Topics and events: `slip:{id}` -> `slip-updated`, `week:{weekID}` -> `board-updated`, deadline -> `bye`. Every event
  carries `data: {}`.
- The handler test for two subscribers can open two `httptest` clients against the new route, perform the add-leg
  request through the regular handler, and assert both streams receive `slip-updated`.
- For deadline tests, reuse the `Clock` / `Timer` seam in `internal/live`; do not sleep in tests.
- The slip page must render `data-live-url="/slips/{id}/events"` on an element (the client looks for
  `[data-live-url]` and falls back to the body trigger).
- Keep the hub in the single process. It is in-memory, so a second Cloud Run instance would not see publishes from the
  first.

## Files touched

- `docs/concepts/live-updates.md`
- `docs/llms.txt`
- `internal/live/hub.go`
- `internal/live/hub_test.go`
- `internal/live/stream.go`
- `internal/live/stream_test.go`
- `plan/README.md`
- `plan/architecture.md`
- `web/static/app.js`

## Run log

- Mode: 
- Checkpoints: task-6, task-7, task-8
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
