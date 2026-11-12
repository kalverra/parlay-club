# Task 14: Docs for the game-first pick board

## Done

- Rewrote the lazy-props paragraph in `docs/concepts/slips.md`: "Load them" sits on the slate card and again in the open
  detail, appears only for a game with stored lines but no prop rows, keeps the one week-level "No lines" state when the
  week has no odds, and refreshes the body in place through `board-updated`.
- Added the detail's own empty state to the pick-board section: a search that matches nothing says so inside the detail,
  and with no search the detail says the game's lines have not synced.
- Updated `docs/reference/routes.md`: `board.Search` no longer claims to name unloaded games; the handler marks a card
  unloaded when its game has lines but no props; the `/games/{id}/props` paragraph says the prompt sits on the card and
  in the detail; the `HX-Trigger` row says `tray-updated` refreshes the dock and tray.
- Corrected the `plan/product.md#Screens` sentence: the pick board puts its slate and detail in two columns at `lg`,
  with a fixed dock at the bottom at every width.
- Passed `good-vibes docs lint`, regenerated `docs/llms.txt`, and ran `good-vibes check` (0 errors).

## Deviations

None. Most of the pick-board prose had already landed in earlier tasks, so this turn corrected the remaining stale
paragraphs instead of rewriting the section.

## Open issues

- `board.Search` still builds `Result.Unloaded` and `board` still tests it, but nothing in the web layer reads it. The
  routes doc now describes the card flag the handler actually uses, so the field is undocumented until a cleanup drops
  it or gives it a caller.
- `good-vibes check` reports 10 stale-doc warnings from earlier dates. None come from this change.

## Next-phase notes

- The pick-board section of `docs/concepts/slips.md` and the pick-board section of `docs/reference/routes.md` now agree
  on the prop prompt. Keep its switch tied to "lines but no props"; a game with no odds stays the week-level state.
- If `#pick-detail` later gets its own refresh URL, update the "Game selection replaces `#pick-body`" paragraph in
  `slips.md` and the design's stale-self-URL risk sentence to match.
