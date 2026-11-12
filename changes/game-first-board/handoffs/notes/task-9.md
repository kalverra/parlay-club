# Task 9: Pairing and the update response in the docs

## Done

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

- The design said to key on `market + team + player + line`. That would split a spread or moneyline, because each side
  carries its own team. The code keys `SPREAD` and `ML` on the market alone and `TOTAL` on the market and number.
  Corrected the design bullet and documented the real key. No task text changed.

## Open issues

- None. `good-vibes docs lint` passes, no summary changed, so `docs index` was not needed. `go test ./internal/board
  ./internal/web` still passes; this task touched no code.

## Next-phase notes

- Phase 4 replaces the tray aside with the fixed dock. Update the "A tray holds the viewer's own picks beside the body at
  `lg` and below it on a phone" sentence and the tray paragraph in `docs/concepts/slips.md`, and the `GET
  /slips/{id}/tray` row and pick-board prose in `docs/reference/routes.md`.
- The routes doc says `#pick-body` re-fetches on `board-updated`. The fixed `SelfURL` from task 5 can still reset the
  open tab and sort after a later signal, so close that gap in the dock work or note the behavior.
