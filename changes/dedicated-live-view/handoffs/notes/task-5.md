## Done

- Added `live.Tick` and `WithLiveTick` to `internal/live/stream.go` so active `live` SSE streams invoke the tick
  callback on connect and every interval while open, stopping cleanly on client disconnect, `bye` deadline, or
  `Shutdown`.
- Wired `live.WithLiveTick(25*time.Second, refresher.KickLive)` in `cmd/server/main.go` only when `!cfg.Sandbox` so
  sandbox mode never polls live ESPN.
- Added `ForceLiveRefresh` to `web.Refresher` in `internal/web/routes.go` (and test stubs in `games_test.go`,
  `research_test.go`, and `pick_test.go`) and exposed `POST /slips/{id}/live/refresh` in `internal/web/slips.go` to run
  a cooldown-guarded manual refresh and re-render `#slip-panel`.
- Added unit tests in `internal/live/stream_test.go` and `internal/web/slips_test.go` covering live tick lifecycle,
  non-live skip, disconnect/deadline/shutdown stop, and manual refresh route behavior.
- Verified with `go test -race ./internal/live/...`, `go test -run TestSlipLiveRefresh ./internal/web/...`,
  `go build ./cmd/server`, and `golangci-lint run ./internal/live/... ./internal/web/... ./cmd/server/...` (0 issues).

## Deviations

Named the exported callback type `live.Tick` instead of `live.LiveTick` to satisfy `revive` stutter linting
(`WithLiveTick` keeps the exact requested name).

## Open issues

None.

## Next-phase notes

`POST /slips/{id}/live/refresh` is mounted and ready for the dedicated Live view Refresh button in Phase 3.
