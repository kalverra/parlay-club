# Phase 3: Phase 3: Move automatic pick-time refreshes off the request path

## Done

### Task 4: Add non-blocking `KickPickLines` and `KickPickGame` methods in `internal/refresh/refresh.go` (deduplicated via `singleflight`, tracked in `s.wg`, publishing `publishWeek` on change) and wire `internal/web/pick.go` to trigger them without blocking responses, with async tests in `internal/refresh/pick_test.go` and `internal/web/pick_test.go`.

- Added non-blocking `KickPickLines` and `KickPickGame` (backed by `kickPick`) in `internal/refresh/refresh.go`, guarding on `s.stopping()` under `s.stopMu`, detaching request cancellation with `context.WithoutCancel`, tracking goroutines in `s.wg`, cancelling on `s.stopCtx`, and delegating to `SyncPickLines` / `SyncPickGame` (which deduplicate via `s.flight` and call `s.publishWeek` when rows change).
- Replaced `SyncPickLines` / `SyncPickGame` on `web.Refresher` in `internal/web/routes.go` with `KickPickLines` / `KickPickGame` and wired `syncPickIfOpen` in `internal/web/pick.go` to trigger them without blocking responses (keeping `ForcePickRefresh` synchronous).
- Added async tests `TestKickPickLinesAsyncSingleFlightAndPublish` and `TestKickPickGameAsyncSingleFlightAndPublish` in `internal/refresh/pick_test.go`.
- Updated `internal/web` refresher stubs and `TestPickRefreshCooldownAndRepeatedSearchWithService`, and added `TestPickBoardAndDetailReturnCachedRowsWhileAsyncRefreshPublishes` in `internal/web/pick_test.go`.

### Task 5: Add HTMX `hx-sync` / search debounce guards in `web/templates/partials.html` and `web/static/app.js` so SSE `board-updated` and rapid typing do not stack redundant partial requests.

- Added `hx-sync="this:drop"` and `hx-disinherit` (`hx-include hx-sync` on `#pick-slate`, `hx-sync` on `#pick-detail` and `#slip-panel`) in `web/templates/partials.html` so duplicate `board-updated` refreshes do not stack while child controls remain responsive.
- Added `hx-sync="this:replace"` on the `#pick-slate` search `<form>` (`hx-trigger="input changed delay:250ms, submit"`) in `web/templates/partials.html` so rapid typing or submit replaces any in-flight `/pick/results` request.
- Added `triggerBoardUpdated` (150ms debounce via `boardUpdateTimer`, cleared when the tab becomes hidden) in `web/static/app.js` for SSE `board-updated` events and `refreshLive()` so simultaneous `visibilitychange` + `focus` or burst SSE signals collapse into a single `board-updated` trigger.
- Added assertions in `TestPickBoardRendersOpenSlateAndDetail` in `internal/web/pick_test.go` covering the `hx-sync` and `hx-disinherit` attributes.

### Task 6: Docs: update `docs/reference/schema.md` (WAL read/write connection pools) and `docs/concepts/data-refresh.md` (async pick-time refresh flow).

- Updated `docs/reference/schema.md` (`## Connection settings`) to document the single-writer handle (`SetMaxOpenConns(1)`, `_txlock=immediate`), the file-backed `query_only=ON` read pool (`SetMaxOpenConns(4)`, `SetMaxIdleConns(4)`) opened by `db.NewStore`, `:memory:` handle sharing, and `Store.Close`.
- Updated `docs/concepts/data-refresh.md` (`## What kicks a sync` and `## Odds credits`) to document the non-blocking `KickPickLines` and `KickPickGame` pick-time refresh flow (`context.WithoutCancel`, `s.wg`, `singleflight` via `SyncPickLines`/`SyncPickGame`, shutdown cancellation, and `board-updated` week publication on change) alongside synchronous `ForcePickRefresh`, `SyncGameProps`, and research syncs.
- Verified documentation with `good-vibes docs lint`.

## Deviations

### Task 4

- `KickPickLines` and `KickPickGame` accept `ctx context.Context` (matching every other method on `web.Refresher` and `KickContext`) and detach cancellation via `context.WithoutCancel(ctx)`.

## Open issues

None.

## Next-phase notes

### Task 4

- Automatic pick-time refreshes now return immediately from `pickBoard`, `pickResults`, and `pickDetail` and signal `board-updated` via `live.Hub` when odds change, so the next task can add `hx-sync` / debouncing on search and `board-updated` in `web/templates/partials.html` and `web/static/app.js`.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/reference/schema.md`
- `internal/refresh/pick_test.go`
- `internal/refresh/refresh.go`
- `internal/web/games_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research_test.go`
- `internal/web/routes.go`
- `web/static/app.js`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
