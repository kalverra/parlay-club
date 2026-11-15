# Task 5

## Done

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

## Deviations

None.

## Open issues

None.

## Next-phase notes

- The remaining task in Phase 2 can update `docs/concepts/data-refresh.md` and `docs/reference/routes.md` to document
  `POST /slips/{id}/pick/refresh`, pick-time sync rules, and the 15m/2h lines, 30m/2h props, and 2m cooldown windows.
