# Phase 1: Phase 1: Add the game slate and drilldown

## Done

### Task 1: Add `board.Slates` and the game-line summaries, with `Position` copied onto prop rows, plus tests for kickoff order, the spread/total/moneyline summary, window and query filtering, and dropping kicked-off games.

- Added `board.GameCard`, `board.GameLines`, and `board.LineSummary`, plus `board.Slates(games, rows, roster, query)`.
  `Slates` keeps upcoming games only, filters by the query's text (matchup, team, venue, or roster player) and kickoff
  window, orders cards by kickoff, and derives each card's spread, total, and moneyline from the built rows.
- Added `Row.Position` and copied it, with `PlayerID`, from the matching roster player in `Build`. The old
  `matchPlayerID` became `matchPlayer` and returns both values.
- Added tests for kickoff order, the spread/total/moneyline summary, prop counts, window and query filtering, and
  dropping kicked-off games, plus one for `Position` on prop rows.

### Task 2: Replace the flat pick page with `#pick-body`: a game slate and a detail pane that shows the open game's existing rows. Keep the kickoff-window chips and the matchup search on the slate. Add `Page.Wide`, the `lg` grid, the phone drilldown and back link, the ET fallback time, and the scroll-restore handler. Link game cards with `hx-get` and a plain href fallback.

- Replaced the flat results partial with `#pick-body`: a slate of game cards plus a detail pane for the open game.
- The slate keeps the matchup search and kickoff-window chips, and each card shows the matchup, venue, ET fallback
  kickoff, spread, total, moneyline, and `Explore N props` (or `No props yet`).
- Game cards are links with `hx-get` plus a plain `href` fallback. They swap `#pick-body` and push the full page URL.
- The detail shows the open game's existing grouped rows, category and sort chips, and an `All games` back link. An
  unknown game, a kicked-off game, or a game hidden by the window falls back to the slate.
- Added `Page.Wide` (`max-w-6xl` header inner and main), the `lg` two-pane grid with independent scrolling, the phone
  hide-the-slate drilldown, the ET server fallback in each `<time>`, and the `data-keep-scroll` save/restore handler.
- `/pick/results` and the swap update path now return `pick-body`, and `board-updated` still refreshes the board.

### Task 3: Docs: rewrite the pick board section in `docs/concepts/slips.md` and the pick board routes and parameters in `docs/reference/routes.md`.

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

### Task 2

- The tray aside stays for now. Phase 4 owns the fixed dock, so at `lg` the layout is the `22rem + 1fr` slate/detail
  body plus an `18rem` tray rail. This is temporary and phase 4 should remove the rail.
- The detail reuses the existing one-game option groups instead of `board.Detail`, tabs, and player clusters. That is
  phase 2's work; this task deliberately only shows the open game's existing rows.
- Category and sort chips target `#pick-body` until the separate `/pick/detail` route exists in phase 2. The design's
  "tabs target `#pick-detail` only" applies once that route lands.
- The ET fallback lives in `internal/web/pick.go` rather than `internal/board`, because it is a render detail. It loads
  `America/New_York` and falls back to UTC if the zone data is missing.

### Task 3

- None.

## Open issues

### Task 1

- `GameCard.Props` counts every non-Lines row, so team totals count as props, matching `IsPropRow` and the existing
  `hasProps` check in `Search`. Task 2's "Explore N props" label should use the same definition.
- `Slates` does not return a started-game count. The slate handler in task 2 should compute it from `games` and the
  clock for the "all games started" empty state and the fallback message.
- Search matching now includes `VenueName` and `VenueID` for every caller of `gameMatchesQuery`, including
  `Search`'s unloaded-props prompt. That is a small, intended widening.

### Task 2

- `#pick-body` carries the current `SelfURL`, so a `board-updated` signal reloads both the slate and the open detail.
- Search text still drives both the slate's game filter and the detail's row filter. Phase 5 scopes `q` to the open
  game and adds the odds and line sorts.
- A week with games but no odds still renders its cards alongside the one "No lines for this week yet" message. Phase 5
  can decide whether to hide the cards there.
- The tray rail is temporary; phase 4 replaces it with the dock and dialog sheet.

### Task 3

- Both sections describe the state after phase 1: the tray is still the aside rail, category and sort chips swap
  `#pick-body`, kickoff is still the default sort, and there is no `/pick/detail`, tabs, player clusters, or dock.
  Phases 2–5 must revisit these sections as they land.
- The docs say the prop count is every row outside the lines category, so team totals count as props. That matches the
  current `board.propCount`; keep it in step if the count changes.

## Next-phase notes

### Task 1

- `GameCard.Away` and `GameCard.Home` are team codes, not full names, so option ordering and the away/home spread
  comparison can key on `row.Team`. Full names are not on the card; fetch them from the game in the handler if the
  detail header needs them.
- `GameCard.Matchup` is the existing `"AWAY @ HOME"` code form and `Venue` is `VenueName` (empty when unset).
- A game with no odds still produces a card, with nil `Lines` and zero `Props`; nil spread, total, or moneyline means
  the market was never stored, not that the price is missing.

### Task 2

- `PickBodyView`, `PickSlateView`, `PickGameCardView`, and `PickDetailView` are the seams for phase 2. Add `board.Detail`
  and its tabs/player groups there instead of changing the slate view.
- `pickResults` and the board branch of `updateLeg` both render `pick-body`. Add `GET /slips/{id}/pick/detail` and point
  tab links at `#pick-detail`; keep game cards pointing at `pick-body`.
- `groupByGame` still builds the detail's option rows for this task. Phase 3's `board.Pair` should replace it when the
  paired row becomes the real unit.
- `data-keep-scroll` is on the slate section only. Add the handler to any other pane that must survive a swap.
- Tests assert the card `href`/`hx-get` pair, the ET fallback, the `Wide` classes, the phone hide class, and the scroll
  handler strings in `app.js`. Update those if the names change.

### Task 3

- Phase 2 adds `GET /slips/{id}/pick/detail` and points `cat` and `sort` at `#pick-detail`. Update the route table and
  the query table in `routes.md`, plus the chip and fallback sentences in both docs.
- Phase 2 changes the default detail sort to player. `routes.md` currently says kickoff (default).
- Phase 4 removes the tray rail and adds the fixed dock. Update the layout sentences in `slips.md` and the `Page.Wide`
  wording that currently names only the header and main.
- Any change to a doc summary needs `good-vibes docs index`; `good-vibes docs lint` passes with the current text.

## Files touched

- `docs/concepts/slips.md`
- `docs/llms.txt`
- `docs/reference/routes.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/layout_test.go`
- `internal/web/page.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `internal/web/static_test.go`
- `web/static/app.js`
- `web/templates/layout.html`
- `web/templates/partials.html`
- `web/templates/pick.html`

## Run log

- Mode: 
- Checkpoints: task-1, task-2, task-3
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
