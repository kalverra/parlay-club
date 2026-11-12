# Task 8 notes

## Done

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

## Deviations

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

## Open issues

- The swap dialog stays open after a successful switch. It already behaved that way: only `leg-saved` closes it and the
  board paths fire `tray-updated`. A small `app.js` handler could close it on `tray-updated`.
- A refreshed pair row is rebuilt with an empty `board.Query`. Only a swap link needs that query, and a row with a
  picked side switches in place, so the only lossy case is a fallback row with no stored odds.
- `pickPairRow` reads one game's odds, its teams' roster, and the slip's legs on each board action. That is more reads
  than the old single-row answer, but all from SQLite.
- `docs lint` passes. No doc summary changed, so `docs index` was not needed.

## Next-phase notes

- Keep `id="{{.ID}}"` and `class="pick-line-group"` on the pair row. The swap sheet's `hx-target` and every board
  response depend on both.
- `pickPairRow(ctx, slip, leg, picked)` is the one rebuild path. Pass `picked=false` only for a delete, so the fallback
  clears the pick.
- `board.LineKey(market, team, player, line)` is the shared line identity. Use it for any other row-level DOM target.
- Phase 4's dock listens for `tray-updated`; the pair row and the tray refresh from the same signal, so the dock needs
  no extra event.
