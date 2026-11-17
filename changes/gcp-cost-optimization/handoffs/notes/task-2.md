# Task 2 Notes

## Done

- Added a `3m` (`liveIdleTimeoutMs`) client-side user-inactivity timer in `web/static/app.js` reset by `pointerdown`, `keydown`, `scroll`, `touchstart`, and `htmx:beforeRequest` (registered with `{ capture: true, passive: true }` only when `data-live-url` is non-empty).
- Updated `pauseLive()` and the `bye` event listener in `web/static/app.js` to close `liveSource`, clear the idle timer, and set `liveIdlePaused = true` so the next user interaction, `focus`, or visible `visibilitychange` calls `resumeLive()` to reopen `EventSource` and run `refreshLive()`.
- Added `teardownLive()` on `pagehide` and `visibilitychange` (`document.hidden`) in `web/static/app.js` to close `liveSource` and clear pending board-update and idle timers.
- Added `TestAppJSPausesAndResumesLiveEventSource` in `internal/web/static_test.go` and verified syntax and state transitions with `node --check`, a Node `vm` behavioral script, `go test ./internal/web/...`, and `golangci-lint run ./internal/web/... --fix`.

## Deviations

- Registered activity listeners with `{ capture: true, passive: true }` instead of `{ passive: true }` alone so `scroll` events inside overflow containers like `[data-keep-scroll]`, which do not bubble, still reset the `3m` inactivity timer.

## Open issues

None.

## Next-phase notes

- Phase 1 is complete. Phase 2 covers `deploy/litestream.yml`, `deploy/setup.sh`, and updating `docs/concepts/live-updates.md`, `docs/concepts/persistence.md`, and `docs/guides/gcp-setup.md`.
