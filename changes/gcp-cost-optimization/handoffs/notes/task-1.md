# Task 1 Notes

## Done

- Reduced `idleDeadline` from `10m` to `3m` and `liveDeadline` from `55m` to `15m` in `internal/live/stream.go`, and updated `internal/live/stream_test.go` to match.
- Added `LiveURL` to `SlipPage` (`internal/web/slips.go`) and `PickPage` (`internal/web/pick.go`) via `slipLiveURL(slip)` so `SlipGraded` slips omit `data-live-url` in `web/templates/slip.html` and `web/templates/pick.html`.
- Updated `slipHandlers.events` in `internal/web/slips.go` to return `text/event-stream` with `event: bye\ndata: {}\n\n` immediately when `slip.Status == db.SlipGraded`.
- Added unit tests in `internal/web/slips_test.go` and `internal/web/pick_test.go` covering `LOCKED` vs `GRADED` `data-live-url` rendering and the `event: bye` response on `/slips/{id}/events`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `web/static/app.js` still needs the client-side `3m` idle pause, `pagehide` cleanup, and interaction/focus reconnect + `refreshLive()` behavior.
