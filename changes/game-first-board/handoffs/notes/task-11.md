# Task 11: docs for the dock and tray

## Done

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

- Renamed `docs/concepts/analyzer.md#the-pick-tray` to `#the-pick-docks-tray`, so the design's read-before link now
  points at the new anchor. No task text changed.

## Open issues

- The dock's combined price folds every one of the viewer's legs, kicked-off legs included, the same way the payout card
  does. A locked leg still moves the number; that is existing math, not a dock decision.
- `docs/reference/routes.md` already described `/pick/dock` and the tray sheet from task 10, so this task left it alone.
- `good-vibes check` reports 10 stale-doc warnings from earlier dates. None come from this change; `docs lint` is clean.

## Next-phase notes

- Phase 5 reworks the search and sorts. If `#pick-detail` gets its own refresh URL, update the "Game selection replaces
  `#pick-body`" paragraph and the design's stale-self-URL risk sentence to match.
- The pick-board section of `docs/concepts/slips.md` is the one place the dock is described. Keep the routes doc to
  endpoints and triggers, and point prose there.
- The anchor `analyzer.md#the-pick-docks-tray` is now linked from the design. Renaming that heading again means fixing
  the design link.
