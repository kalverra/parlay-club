# Phase 1: Phase 1: Tighten SSE stream scope and lifetimes

## Done

### Task 1: Reduce `idleDeadline` (`3m`) and `liveDeadline` (`15m`) in `internal/live/stream.go`, omit `data-live-url` on `SlipGraded` pages in `internal/web/slips.go`, `internal/web/pick.go`, `web/templates/slip.html`, and `web/templates/pick.html`, and make `/slips/{id}/events` return `event: bye` for graded slips, with tests in `internal/live/stream_test.go`, `internal/web/slips_test.go`, and `internal/web/pick_test.go`.

- Reduced `idleDeadline` from `10m` to `3m` and `liveDeadline` from `55m` to `15m` in `internal/live/stream.go`, and updated `internal/live/stream_test.go` to match.
- Added `LiveURL` to `SlipPage` (`internal/web/slips.go`) and `PickPage` (`internal/web/pick.go`) via `slipLiveURL(slip)` so `SlipGraded` slips omit `data-live-url` in `web/templates/slip.html` and `web/templates/pick.html`.
- Updated `slipHandlers.events` in `internal/web/slips.go` to return `text/event-stream` with `event: bye\ndata: {}\n\n` immediately when `slip.Status == db.SlipGraded`.
- Added unit tests in `internal/web/slips_test.go` and `internal/web/pick_test.go` covering `LOCKED` vs `GRADED` `data-live-url` rendering and the `event: bye` response on `/slips/{id}/events`.

### Task 2: Add a `3m` user-inactivity pause + interaction/focus/pagehide lifecycle for `EventSource` in `web/static/app.js` that refreshes partials on resume.

- Added a `3m` (`liveIdleTimeoutMs`) client-side user-inactivity timer in `web/static/app.js` reset by `pointerdown`, `keydown`, `scroll`, `touchstart`, and `htmx:beforeRequest` (registered with `{ capture: true, passive: true }` only when `data-live-url` is non-empty).
- Updated `pauseLive()` and the `bye` event listener in `web/static/app.js` to close `liveSource`, clear the idle timer, and set `liveIdlePaused = true` so the next user interaction, `focus`, or visible `visibilitychange` calls `resumeLive()` to reopen `EventSource` and run `refreshLive()`.
- Added `teardownLive()` on `pagehide` and `visibilitychange` (`document.hidden`) in `web/static/app.js` to close `liveSource` and clear pending board-update and idle timers.
- Added `TestAppJSPausesAndResumesLiveEventSource` in `internal/web/static_test.go` and verified syntax and state transitions with `node --check`, a Node `vm` behavioral script, `go test ./internal/web/...`, and `golangci-lint run ./internal/web/... --fix`.

## Deviations

### Task 2

- Registered activity listeners with `{ capture: true, passive: true }` instead of `{ passive: true }` alone so `scroll` events inside overflow containers like `[data-keep-scroll]`, which do not bubble, still reset the `3m` inactivity timer.

## Open issues

None.

## Next-phase notes

### Task 1

- `web/static/app.js` still needs the client-side `3m` idle pause, `pagehide` cleanup, and interaction/focus reconnect + `refreshLive()` behavior.

### Task 2

- Phase 1 is complete. Phase 2 covers `deploy/litestream.yml`, `deploy/setup.sh`, and updating `docs/concepts/live-updates.md`, `docs/concepts/persistence.md`, and `docs/guides/gcp-setup.md`.

## Files touched

- `internal/live/stream.go`
- `internal/live/stream_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `internal/web/static_test.go`
- `web/static/app.js`
- `web/templates/pick.html`
- `web/templates/slip.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: lint 6.1s, test 8.5s
- Rounds used: 1 of 2; verify failures: 0
