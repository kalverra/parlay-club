# Task 10: The combined price and the fixed pick dock

## Done

- `PickTrayView` became `PickDockView` with `Combined` and `CombinedAmerican`, computed in `dockFrom` with
  `parlay.CombinedOdds` over the viewer's legs and `parlay.DecimalToAmerican`, the same math the payout card uses. The
  builder renamed from `trayFrom`/`pickTrayData` to `dockFrom`/`pickDockData`.
- Replaced the aside tray with a fixed `#pick-dock`: "Your picks", `N of M`, the combined price, and a "Slip" button.
  The button loads `GET /slips/{id}/tray` into a `dialog` sheet that reuses the swap sheet's shell.
- The dock refreshes itself on `tray-updated` and `slip-updated` from the new `GET /slips/{id}/pick/dock` partial.
  `app.js` opens the sheet after the tray lands on `#pick-tray-body` and closes it on `data-close-dock`.
- Reserved the dock's space: the phone still clears it with the layout's `pb-28`, and the pick section is
  `lg:h-[calc(100dvh-3.5rem-4.5rem)]` so the wide panes end above the dock.
- Tests: the dock partial shows the viewer's count and combined price and ignores a partner's leg, the board page has
  the dock and sheet instead of the aside, a tray removal fires both signals, and `app.js` wires the sheet open/close.

## Deviations

- The design's route table had no URL for the dock to refresh from, but `/tray` answers tray markup, so a dock fetch
  from it would swap the panel into the bar. Added `GET /slips/{id}/pick/dock`, registered the `pick-dock` partial, and
  added the route to the design's table and `docs/reference/routes.md`.
- `DELETE /legs/{id}?from=tray` fired only `board-updated`, so a removal inside the sheet would leave the dock stale.
  It now fires `tray-updated, board-updated`.
- The design's `PickDockView` has no `HasLegs`; the dock template hides the combined price with `{{if .Legs}}` so an
  empty slip shows just `0 of N`.
- HTMX inherits `hx-trigger`, so the dock's "Slip" button carries an explicit `hx-trigger="click"`. Without it the
  refresh signals would also load the tray and pop the sheet open.

## Open issues

- The wide pane reserve is a fixed `4.5rem` against a dock of about `3.5rem` plus the safe-area inset. A phone with a
  home indicator clears the dock through the layout's `pb-28`, so the fixed reserve only has to be close at `lg`.
- With JavaScript off the "Slip" button does nothing. The swap sheet has the same limit; every other board link still
  has an `href` fallback.
- `docs/reference/routes.md` now lists the dock route and the tray's sheet role. The tray prose in
  `docs/concepts/slips.md` and `docs/concepts/analyzer.md` still describes the aside and belongs to the next task.
- The design's stale `#pick-body` `SelfURL` risk is unchanged: a tab or sort swap replaces only `#pick-detail`, and a
  later `board-updated` still refetches the body from the page-load query.

## Next-phase notes

- Docs task: the aside sentence at the end of the first pick-board paragraph in `docs/concepts/slips.md`, the tray
  paragraph after the empty states, and "The pick tray" in `docs/concepts/analyzer.md` all need the dock and sheet
  wording plus the combined price.
- `pickDockData` and `dockFrom` are the one data path for `/pick/dock`, `/tray`, and `pickSwap`. Any new count or price
  belongs there, not in the handlers.
- Keep `#pick-dock`, `#pick-tray-sheet`, `#pick-tray-body`, and `data-close-dock`: the dock's URL and the `app.js`
  handler both address them by id.
- A tray removal now costs one extra `/tray` request, because the tray refetches itself on the `tray-updated` it fires.
- Phase 5 can give `#pick-detail` its own refresh URL while it reworks the search and sorts, which closes the stale
  `#pick-body` risk.
