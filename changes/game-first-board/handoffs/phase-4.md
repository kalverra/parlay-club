# Phase 4: Phase 4: Add the pick dock

## Done

### Task 10: Add the combined price to `PickDockView` from the viewer's legs and its tests, then replace the aside tray with the fixed dock and a `dialog` sheet. Add the dock's open/close handler, refresh it on `tray-updated` and `slip-updated`, and reserve bottom padding so rows are not hidden.

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

### Task 11: Docs: update the tray and dock in `docs/concepts/slips.md` and the pick tray description in `docs/concepts/analyzer.md`.

- `docs/concepts/slips.md` starts the pick board with the fixed bottom dock: it spans every width, keeps the picks in
  reach, reserves bottom padding so the last row stays visible, widens with the header and main to `max-w-6xl`, and
  appears only on the pick screen.
- The old tray paragraph is now the dock paragraph. It names "Your picks", `Picked of Limit`, the combined price from
  `parlay.CombinedOdds` converted with `parlay.DecimalToAmerican`, the "Slip" button that loads `/tray` into a `dialog`
  sheet, the tray's legs, warnings, one action, and the `tray-updated`/`slip-updated` refresh from `/pick/dock`.
- The pair-row paragraph says the dock and the tray both refresh after an add, update, or delete, instead of naming only
  the tray.
- `docs/concepts/analyzer.md` renamed "The pick tray" to "The pick dock's tray": the prose leads with the dock's count
  and combined price, then the sheet's tray and its viewer-only warnings, then the same two refresh signals.
- The slips summary now says "pick dock", the `dock` tag joins `tray`, and `docs/llms.txt` was regenerated.

## Deviations

### Task 10

- The design's route table had no URL for the dock to refresh from, but `/tray` answers tray markup, so a dock fetch
  from it would swap the panel into the bar. Added `GET /slips/{id}/pick/dock`, registered the `pick-dock` partial, and
  added the route to the design's table and `docs/reference/routes.md`.
- `DELETE /legs/{id}?from=tray` fired only `board-updated`, so a removal inside the sheet would leave the dock stale.
  It now fires `tray-updated, board-updated`.
- The design's `PickDockView` has no `HasLegs`; the dock template hides the combined price with `{{if .Legs}}` so an
  empty slip shows just `0 of N`.
- HTMX inherits `hx-trigger`, so the dock's "Slip" button carries an explicit `hx-trigger="click"`. Without it the
  refresh signals would also load the tray and pop the sheet open.

### Task 11

- Renamed `docs/concepts/analyzer.md#the-pick-tray` to `#the-pick-docks-tray`, so the design's read-before link now
  points at the new anchor. No task text changed.

## Open issues

### Task 10

- The wide pane reserve is a fixed `4.5rem` against a dock of about `3.5rem` plus the safe-area inset. A phone with a
  home indicator clears the dock through the layout's `pb-28`, so the fixed reserve only has to be close at `lg`.
- With JavaScript off the "Slip" button does nothing. The swap sheet has the same limit; every other board link still
  has an `href` fallback.
- `docs/reference/routes.md` now lists the dock route and the tray's sheet role. The tray prose in
  `docs/concepts/slips.md` and `docs/concepts/analyzer.md` still describes the aside and belongs to the next task.
- The design's stale `#pick-body` `SelfURL` risk is unchanged: a tab or sort swap replaces only `#pick-detail`, and a
  later `board-updated` still refetches the body from the page-load query.

### Task 11

- The dock's combined price folds every one of the viewer's legs, kicked-off legs included, the same way the payout card
  does. A locked leg still moves the number; that is existing math, not a dock decision.
- `docs/reference/routes.md` already described `/pick/dock` and the tray sheet from task 10, so this task left it alone.
- `good-vibes check` reports 10 stale-doc warnings from earlier dates. None come from this change; `docs lint` is clean.

## Next-phase notes

### Task 10

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

### Task 11

- Phase 5 reworks the search and sorts. If `#pick-detail` gets its own refresh URL, update the "Game selection replaces
  `#pick-body`" paragraph and the design's stale-self-URL risk sentence to match.
- The pick-board section of `docs/concepts/slips.md` is the one place the dock is described. Keep the routes doc to
  endpoints and triggers, and point prose there.
- The anchor `analyzer.md#the-pick-docks-tray` is now linked from the design. Renaming that heading again means fixing
  the design link.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `internal/web/static_test.go`
- `web/static/app.css`
- `web/static/app.js`
- `web/templates/partials.html`
- `web/templates/pick.html`

## Run log

- Mode: 
- Checkpoints: task-10, task-11
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
