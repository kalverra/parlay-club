# Phase 2: Phase 2: Pick-time on-demand refresh rules and UI

## Done

### Task 4: Implement `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, pick-time staleness rules (`15m`/`2h` lines, `30m`/`2h` props), and the `2m` manual refresh cooldown in `internal/refresh` with unit and single-flight tests

- Added `SyncPickLines`, `SyncPickGame`, and `ForcePickRefresh` to `refresh.Service` in `internal/refresh/refresh.go`
  (using `requestTimeout` and single-flight `refreshSync`, ignoring `odds.ErrNoAPIKey` when odds providers are
  unconfigured).
- Implemented `oddsPickLinesStale` (15m on game days before the day's last kickoff or within 2h of next kickoff; 2h
  midweek), tightened `oddsPropsStale` (30m on game days or within 6h of kickoff; 2h earlier in the week), and
  `pickCooldownStale` (2m manual refresh cooldown) in `internal/refresh/rules.go`.
- Updated `TestOddsPropsStaleRules` in `internal/refresh/rules_test.go` and
  `TestOpeningSameGameTwiceWithinCacheWindowSpendsCreditsOnce` in `internal/refresh/research_test.go` for the 30m
  game-day props window, and added `TestOddsPickLinesStaleRules` and `TestPickCooldownStaleRules`.
- Added unit and single-flight tests in `internal/refresh/pick_test.go` covering `SyncPickLines`, `SyncPickGame`,
  `ForcePickRefresh`, cooldown enforcement, and concurrent caller coalescing.

### Task 5: Extend `web.Refresher`, trigger pick-time syncs on `OPEN` slips with upcoming games in `pickBoard`/`pickResults`/`pickDetail` (`internal/web/pick.go`), and add `POST /slips/{id}/pick/refresh` with the phone-first refresh button in `web/templates/partials.html` and handler/cooldown/locked-slip tests

- Extended `web.Refresher` in `internal/web/routes.go` with `SyncPickLines`, `SyncPickGame`, and `ForcePickRefresh`, and
  updated test refresher stubs in `internal/web/games_test.go` and `internal/web/research_test.go`.
- Added pick-time sync gating (`syncPickIfOpen`, `syncPickScope`) in `internal/web/pick.go` so `pickBoard` syncs lines
  (or game lines + props when an upcoming game is selected) and `pickResults`/`pickDetail` sync game lines + props only
  when `slip.Status == db.SlipOpen`, `q.GameID != ""`, and the game is upcoming, while `pickSlate` remains read-only.
- Added `POST /slips/{id}/pick/refresh` (`pickRefresh` in `internal/web/pick.go`, registered in
  `internal/web/slips.go`), enforcing group scope, `db.SlipOpen` (returning 422 on locked slips), and upcoming-game
  checks before calling `ForcePickRefresh` and returning `#pick-body`.
- Added `CanRefresh` to `PickSlateView` and `PickDetailView` and rendered the phone-first (`min-h-12`) `Refresh odds`
  button in `web/templates/partials.html` (`pick-slate` and `pick-detail`) only for `OPEN` slips with upcoming games.
- Added handler, group-scope, locked-slip, repeated-search, and 2-minute cooldown tests in `internal/web/pick_test.go`
  (`TestPickBoardAndPartialsSyncRules`, `TestPickRefreshHandlerAndGuards`,
  `TestPickRefreshCooldownAndRepeatedSearchWithService`).

### Task 6: Docs: update `docs/concepts/data-refresh.md`, `docs/concepts/slips.md`, `docs/reference/data-sources.md`, and `docs/reference/routes.md` for pick-time refreshes and the refresh route

- Updated `docs/concepts/data-refresh.md` to document `SyncPickLines`, `SyncPickGame`, `ForcePickRefresh`, pick-time
  `OPEN`-slip gating, the `15m`/`2h` lines and `30m`/`2h` props staleness rules, and the `2m` manual refresh cooldown.
- Updated `docs/concepts/slips.md` to document pick-time sync gating on `OPEN` slips with upcoming games, the
  phone-first "Refresh odds" button (`POST /slips/{id}/pick/refresh`), and read-only behavior on locked slips and
  `/pick/slate`.
- Updated `docs/reference/data-sources.md` to document the `30m`/`2h` props window, pick-time `15m`/`2h` lines window,
  `2m` manual cooldown, and `RunPrefill` Parlay-only props sweep in the Parlay API and Budget sections.
- Updated `docs/reference/routes.md` to add `POST /slips/{id}/pick/refresh` (`422` when not `OPEN`, `2m` cooldown,
  `#pick-body` response) and pick-time sync rules for `/slips/{id}/pick`, `/pick/results`, `/pick/detail`, and
  `/pick/slate`.
- Removed retired `sources` and `last_verified` front-matter fields from the four updated docs so `good-vibes docs lint`
  passes with zero errors or warnings on all four files.

## Deviations

### Task 6

None. (`docs/reference/config.md`, modified in an earlier task, still reports a retired front-matter error in
`good-vibes docs lint` and was left untouched as outside this task.)

## Open issues

None.

## Next-phase notes

### Task 4

- `web.Refresher` in `internal/web/routes.go` can now be extended with
  `SyncPickLines(ctx context.Context, weekID int64) error`,
  `SyncPickGame(ctx context.Context, weekID int64, gameID string) error`, and
  `ForcePickRefresh(ctx context.Context, weekID int64, gameID string) error`, which `*refresh.Service` already
  satisfies.

### Task 5

- The remaining task in Phase 2 can update `docs/concepts/data-refresh.md` and `docs/reference/routes.md` to document
  `POST /slips/{id}/pick/refresh`, pick-time sync rules, and the 15m/2h lines, 30m/2h props, and 2m cooldown windows.

### Task 6

- Phase 2 is complete.

## Files touched

- `docs/concepts/data-refresh.md`
- `docs/concepts/slips.md`
- `docs/reference/data-sources.md`
- `docs/reference/routes.md`
- `internal/refresh/pick_test.go`
- `internal/refresh/refresh.go`
- `internal/refresh/research_test.go`
- `internal/refresh/rules.go`
- `internal/refresh/rules_test.go`
- `internal/web/games_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/research_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `web/templates/partials.html`

## Run log

- Mode:
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks ran; skipped: lint (no matching files), test (no matching files)
- Rounds used: 1 of 2; verify failures: 0
