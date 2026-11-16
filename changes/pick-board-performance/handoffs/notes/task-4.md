# Task 4

## Done

- Added non-blocking `KickPickLines` and `KickPickGame` (backed by `kickPick`) in `internal/refresh/refresh.go`, guarding on `s.stopping()` under `s.stopMu`, detaching request cancellation with `context.WithoutCancel`, tracking goroutines in `s.wg`, cancelling on `s.stopCtx`, and delegating to `SyncPickLines` / `SyncPickGame` (which deduplicate via `s.flight` and call `s.publishWeek` when rows change).
- Replaced `SyncPickLines` / `SyncPickGame` on `web.Refresher` in `internal/web/routes.go` with `KickPickLines` / `KickPickGame` and wired `syncPickIfOpen` in `internal/web/pick.go` to trigger them without blocking responses (keeping `ForcePickRefresh` synchronous).
- Added async tests `TestKickPickLinesAsyncSingleFlightAndPublish` and `TestKickPickGameAsyncSingleFlightAndPublish` in `internal/refresh/pick_test.go`.
- Updated `internal/web` refresher stubs and `TestPickRefreshCooldownAndRepeatedSearchWithService`, and added `TestPickBoardAndDetailReturnCachedRowsWhileAsyncRefreshPublishes` in `internal/web/pick_test.go`.

## Deviations

- `KickPickLines` and `KickPickGame` accept `ctx context.Context` (matching every other method on `web.Refresher` and `KickContext`) and detach cancellation via `context.WithoutCancel(ctx)`.

## Open issues

None.

## Next-phase notes

- Automatic pick-time refreshes now return immediately from `pickBoard`, `pickResults`, and `pickDetail` and signal `board-updated` via `live.Hub` when odds change, so the next task can add `hx-sync` / debouncing on search and `board-updated` in `web/templates/partials.html` and `web/static/app.js`.
