# Task 3: Docs for the game-first pick board

## Done

- Rewrote `docs/concepts/slips.md#the-pick-board` around the slate and drilldown: the wide layout, `board.Slates`
  cards, search and window chips, card contents and links, the open game's header, chips, and grouped rows, the
  slate fallback, scroll restore, the ET fallback time, empty states, and the unchanged add, remove, swap, tray, and
  lazy-props rules.
- Rewrote `docs/reference/routes.md#pick-board`: the route table, a query-contract table for `game`, `cat`, `q`, `win`,
  and `sort`, the card and chip swap behavior, the `#pick-body` responses for add, remove, and swap, the props load,
  and the week-level empty states.
- Updated the slips route table, the `PUT /legs/{id}` response row, the `HX-Trigger` note for `board-updated`, and the
  doc summary and tags, then regenerated `docs/llms.txt` with `good-vibes docs index`.

## Deviations

- None.

## Open issues

- Both sections describe the state after phase 1: the tray is still the aside rail, category and sort chips swap
  `#pick-body`, kickoff is still the default sort, and there is no `/pick/detail`, tabs, player clusters, or dock.
  Phases 2–5 must revisit these sections as they land.
- The docs say the prop count is every row outside the lines category, so team totals count as props. That matches the
  current `board.propCount`; keep it in step if the count changes.

## Next-phase notes

- Phase 2 adds `GET /slips/{id}/pick/detail` and points `cat` and `sort` at `#pick-detail`. Update the route table and
  the query table in `routes.md`, plus the chip and fallback sentences in both docs.
- Phase 2 changes the default detail sort to player. `routes.md` currently says kickoff (default).
- Phase 4 removes the tray rail and adds the fixed dock. Update the layout sentences in `slips.md` and the `Page.Wide`
  wording that currently names only the header and main.
- Any change to a doc summary needs `good-vibes docs index`; `good-vibes docs lint` passes with the current text.
