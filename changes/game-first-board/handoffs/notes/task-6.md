# Task 6 notes

## Done

- Rewrote the pick board's detail in `docs/concepts/slips.md`: pinned header, the six category tabs with Lines folding in
  team totals, row-count badges, the Lines ordering, player cards, the player default sort, and `#pick-detail` tab links.
- Updated the lazy-props sentence to say switching tabs spends no credits.
- Added `GET /slips/{id}/pick/detail` to both route tables in `docs/reference/routes.md` and narrowed `/pick/results` to
  search, window, and game changes.
- Corrected the query contract: `cat` is the tab values plus `team_totals` folding into `lines`, and `sort` is `player`
  (default), `odds`, or `line`, with kickoff, category, and unknown values resolving to player.
- Added `board.Detail` to the loading prose and documented the detail's fallback, the placeholder response without an
  open game, and the per-tab player cards.

## Deviations

- None.

## Open issues

- The routes doc still says `#pick-body` re-fetches on `board-updated`, which is true, but the fixed live refresh URL
  from task 5 still resets the open tab and sort after a later signal. Phase 4's dock work should close that gap or the
  route doc should note the behavior.
- The tray is still described as an aside rail beside the body; phase 4 replaces it with the fixed dock and must revisit
  the layout sentence in `slips.md` and the `Page.Wide` wording in `routes.md`.

## Next-phase notes

- The docs now name the tab values, the `team_totals` fold, the `lines` fallback, the player default sort, and
  `#pick-detail`. Pairing in phase 3 does not change any of that unless the pair row replaces the whole detail partial.
- If phase 3 changes the board answer from `#pick-body` to a single row, update `routes.md`'s `PUT /legs/{id}` row and
  the `UpdateLeg` sentence in `slips.md`.
- `good-vibes docs lint` passes. No summary changed, so `docs index` was not needed.
