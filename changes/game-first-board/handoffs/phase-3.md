# Phase 3: Phase 3: Pair both sides of one line

## Done

### Task 7: Add `board.Pair` and its tests: Over/Under, away/home spread and moneyline, the single `ANYTIME_TD` side, a missing side, and deterministic option order.

- Added `board.Pair(game, rows) []LineGroup`, the exported keyed fold that puts both sides of one line in a single
  group.
- `lineGroups` now calls `Pair` and only adds the detail's sort, so the detail and any later pair-row refresh share one
  pairing rule.
- Option order is fixed: away then home for spread and moneyline, Over then Under for totals and props, and the single
  Yes for `ANYTIME_TD`. Field-level tiebreaks keep duplicate-rank options stable regardless of input order.
- Added tests for Over/Under on a total and a prop, away/home spread and moneyline, the single anytime-TD side, one
  stored side, and deterministic option order under reversed input.

### Task 8: Render one row per line with two side buttons. A picked side deletes, the opposite side updates that leg through `PUT /legs/{id}`, and an empty side adds. Return the refreshed pair row from the board update path and keep `Hx-Trigger: tray-updated`. Add handler tests for all three paths.

- A pair row now renders one line with two side buttons: a picked side deletes, the opposite side updates that leg
  through `PUT /legs/{id}`, and an empty side adds. Both picked sides (from the custom form) stay deletable.
- The add, update, and delete board paths all rebuild the line with `board.Pair` and answer with the refreshed
  `pick-line-group` partial plus `Hx-Trigger: tray-updated`, so only that row swaps. An update that moves the leg to
  another line also fires `board-updated`, so the line it left behind cannot keep a stale delete button.
- Added handler tests for add, opposite-side update through PUT, a cross-line update, delete, and the swap sheet's
  pair-row target.
- Exported `board.LineKey` so the line group's DOM id, the stale-line check, and the swap sheet's target use the same
  identity.
- Updated `docs/concepts/slips.md` and `docs/reference/routes.md` for the pair-row answers.

### Task 9: Docs: document pairing and the update response in `docs/concepts/slips.md` and `docs/reference/routes.md`.

- Rewrote the pairing paragraph in `docs/concepts/slips.md`: `board.Pair` is the one keyed fold, `board.LineKey` names
  the line identity, the option order is fixed (away/home, Over/Under, single Yes), a one-sided line keeps one button,
  and a line with both sides picked from the custom form paints both and lets either be removed.
- Documented the board answer: add, update, and delete all return the refreshed pair row for that line plus
  `tray-updated`, the update path no longer returns the whole results partial, and a cross-line switch also fires
  `board-updated`.
- Updated the same paragraph in `docs/reference/routes.md` to name the shared `pick-line-group` response, the in-place
  `closest .pick-line-group` swap, the swap sheet's row-id target, and the single-option fallback for a line the board
  cannot rebuild.
- Corrected the design's pairing-key bullet to match `board.LineKey`: `SPREAD`/`ML` key on the market alone, `TOTAL` on
  the market and number, and `TEAM_TOTAL`, props, and `ANYTIME_TD` on market, team, player, and number.

## Deviations

### Task 7

- None. The design names pairing as a keyed fold but does not give `Pair` a signature; it mirrors `lineGroups` minus the
  sort: `Pair(game db.Game, rows []Row) []LineGroup`.

### Task 8

- The swap sheet also sends `PUT /legs/{id}`, and it lives outside the pair row in a dialog, so `closest` cannot find
  the target. Added `PickLineGroupView.ID` (`pick-line-<game>-<LineKey>`), `PickSwapView.Target`, and
  `hx-target="{{$.Target}}"` on the swap form. The design assumes the refreshed pair row lands in place but does not
  say how the sheet addresses it.
- Removed the now-unused `pick-row` partial and the `Matchup`/`KickoffISO` fields that only it read. The line group is
  the swap unit.
- When the board can no longer rebuild a line (for example a custom leg whose odds were never stored), the handler
  falls back to one option built from the leg, so the row still adds, removes, or switches.
- A switch that leaves a line adds `board-updated` to `Hx-Trigger` as well as `tray-updated`. Returning only the pair
  row would leave the old line's delete button pointing at a leg that moved, so tapping it would delete the new pick.
- Fixed a task 7 float comparison in `board_test.go` that failed `testifylint`; no behavior change.

### Task 9

- The design said to key on `market + team + player + line`. That would split a spread or moneyline, because each side
  carries its own team. The code keys `SPREAD` and `ML` on the market alone and `TOTAL` on the market and number.
  Corrected the design bullet and documented the real key. No task text changed.

## Open issues

### Task 7

- `Pair` keeps groups in first-seen order. `lineGroups` sorts them afterwards for the detail; a caller that uses `Pair`
  directly must sort or pick the group it wants.
- `Pair` does not filter by `game.ID`; callers pass rows already scoped to one game, as `Detail` does.
- `sortOptions` now compares side, team, line, then text after the rank. That is only a tiebreak for degenerate input,
  but it means the option order is total rather than merely stable.

### Task 8

- The swap dialog stays open after a successful switch. It already behaved that way: only `leg-saved` closes it and the
  board paths fire `tray-updated`. A small `app.js` handler could close it on `tray-updated`.
- A refreshed pair row is rebuilt with an empty `board.Query`. Only a swap link needs that query, and a row with a
  picked side switches in place, so the only lossy case is a fallback row with no stored odds.
- `pickPairRow` reads one game's odds, its teams' roster, and the slip's legs on each board action. That is more reads
  than the old single-row answer, but all from SQLite.
- `docs lint` passes. No doc summary changed, so `docs index` was not needed.

### Task 9

- None. `good-vibes docs lint` passes, no summary changed, so `docs index` was not needed. `go test ./internal/board
  ./internal/web` still passes; this task touched no code.

## Next-phase notes

### Task 7

- Use `Pair(game, rows)` to rebuild the refreshed row after add, update, or delete. Match the updated leg on market,
  team, player, and line, the same fields `lineKey` uses; group options already carry `Picked` from `board.Build`.
- `Pair` returns `[]LineGroup`, so the handler can find the one group by `Market`, `Team`, `Player`, and `Line` or by
  the first option that matches the posted selection.
- The template seam is still `pick-line-group`; each option remains a `.pick-row`.
- No docs changed in this task. Pairing and the update response belong to the next task's docs step.

### Task 8

- Keep `id="{{.ID}}"` and `class="pick-line-group"` on the pair row. The swap sheet's `hx-target` and every board
  response depend on both.
- `pickPairRow(ctx, slip, leg, picked)` is the one rebuild path. Pass `picked=false` only for a delete, so the fallback
  clears the pick.
- `board.LineKey(market, team, player, line)` is the shared line identity. Use it for any other row-level DOM target.
- Phase 4's dock listens for `tray-updated`; the pair row and the tray refresh from the same signal, so the dock needs
  no extra event.

### Task 9

- Phase 4 replaces the tray aside with the fixed dock. Update the "A tray holds the viewer's own picks beside the body at
  `lg` and below it on a phone" sentence and the tray paragraph in `docs/concepts/slips.md`, and the `GET
  /slips/{id}/tray` row and pick-board prose in `docs/reference/routes.md`.
- The routes doc says `#pick-body` re-fetches on `board-updated`. The fixed `SelfURL` from task 5 can still reset the
  open tab and sort after a later signal, so close that gap in the dock work or note the behavior.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-7, task-8, task-9
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
