## Done

- Added `SyncLive`, `KickLive`, and cooldown-guarded `ForceLiveRefresh` to `internal/refresh/live.go` with `liveFastAge`
  (20s), `liveRefreshCooldown` (10s), `liveSyncTimeout` (10s), and `liveSummaryConcurrency` (4).
- Scoped `SyncLive` strictly to free ESPN sources (`espn:scoreboard`, concurrent `espn:summary:{gameID}` for `LIVE`
  games on the slip or week fallback, and immediate `syncGrade` when any slip game is `FINAL` with `PENDING` legs), with
  zero paid odds calls.
- Added per-slip `liveCooldown` tracking on `refresh.Service` in `internal/refresh/refresh.go` and cleared it alongside
  `backoff` in `ClearBackoff`.
- Added unit tests in `internal/refresh/live_test.go` covering slip scoping, zero paid calls, 20-second `liveFastAge`,
  5-minute `failureBackoff`, `singleflight` deduplication, `FINAL` auto-grading, change-only `slip:{id}` publishing,
  per-slip 10-second `ForceLiveRefresh` cooldown, and `KickLive` background/shutdown behavior.
- Verified with `go test -race ./internal/refresh/...` (passed) and `golangci-lint run ./internal/refresh/...` (0
  issues).

## Deviations

None.

## Open issues

None.

## Next-phase notes

`KickLive(slipID, weekID int64)` matches the `live.LiveTick` signature for Task 5 (`internal/live` +
`cmd/server/main.go`), and `ForceLiveRefresh(ctx, slipID, weekID)` is ready for `POST /slips/{id}/live/refresh` in Task
6.
