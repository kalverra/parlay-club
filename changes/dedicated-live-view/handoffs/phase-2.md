# Phase 2: Phase 2: Fast viewer-driven free-API live refresh and SSE ticks

## Done

### Task 4: Fast live refresh

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

### Task 5: SSE ticks and manual refresh

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

### Task 6: Refresh, live-update, and route docs

- Updated `docs/concepts/data-refresh.md` with the viewer-driven `KickLive` / `SyncLive` 20-second `liveFastAge`
  cadence, zero paid odds guarantee, `FINAL` auto-grading, and 10-second per-slip `ForceLiveRefresh` cooldown.
- Updated `docs/concepts/live-updates.md` with the `## Overview` Mermaid diagram and `WithLiveTick` 25-second SSE tick
  lifecycle (including sandbox disablement and stop on disconnect, `bye` deadline, or `Shutdown`).
- Updated `docs/reference/routes.md` with `POST /slips/{id}/live/refresh` and the `GET /slips/{id}/events` live tick
  behavior.
- Verified with `good-vibes docs lint` (0 errors).

## Deviations

### Task 5

Named the exported callback type `live.Tick` instead of `live.LiveTick` to satisfy `revive` stutter linting
(`WithLiveTick` keeps the exact requested name).

## Open issues

None.

## Next-phase notes

### Task 4

`KickLive(slipID, weekID int64)` matches the `live.LiveTick` signature for Task 5 (`internal/live` +
`cmd/server/main.go`), and `ForceLiveRefresh(ctx, slipID, weekID)` is ready for `POST /slips/{id}/live/refresh` in Task
6.

### Task 5

`POST /slips/{id}/live/refresh` is mounted and ready for the dedicated Live view Refresh button in Phase 3.

### Task 6

Phase 2 documentation is complete; Phase 3 can proceed with the dedicated Live view templates, game-time ordering, and
overview spotlight.

## Files touched

- `cmd/server/main.go`
- `docs/concepts/data-refresh.md`
- `docs/concepts/live-updates.md`
- `docs/reference/routes.md`
- `internal/live/stream.go`
- `internal/live/stream_test.go`
- `internal/refresh/live.go`
- `internal/refresh/live_test.go`
- `internal/refresh/refresh.go`
- `internal/web/games_test.go`
- `internal/web/pick_test.go`
- `internal/web/research_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `server`

## Run log

- Mode:
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: test 600ms; skipped: generate (no matching files), lint (no matching files), actionlint (no
  matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
