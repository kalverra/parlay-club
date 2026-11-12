# Task 2: Game slate and drilldown

## Done

- Replaced the flat results partial with `#pick-body`: a slate of game cards plus a detail pane for the open game.
- The slate keeps the matchup search and kickoff-window chips, and each card shows the matchup, venue, ET fallback
  kickoff, spread, total, moneyline, and `Explore N props` (or `No props yet`).
- Game cards are links with `hx-get` plus a plain `href` fallback. They swap `#pick-body` and push the full page URL.
- The detail shows the open game's existing grouped rows, category and sort chips, and an `All games` back link. An
  unknown game, a kicked-off game, or a game hidden by the window falls back to the slate.
- Added `Page.Wide` (`max-w-6xl` header inner and main), the `lg` two-pane grid with independent scrolling, the phone
  hide-the-slate drilldown, the ET server fallback in each `<time>`, and the `data-keep-scroll` save/restore handler.
- `/pick/results` and the swap update path now return `pick-body`, and `board-updated` still refreshes the board.

## Deviations

- The tray aside stays for now. Phase 4 owns the fixed dock, so at `lg` the layout is the `22rem + 1fr` slate/detail
  body plus an `18rem` tray rail. This is temporary and phase 4 should remove the rail.
- The detail reuses the existing one-game option groups instead of `board.Detail`, tabs, and player clusters. That is
  phase 2's work; this task deliberately only shows the open game's existing rows.
- Category and sort chips target `#pick-body` until the separate `/pick/detail` route exists in phase 2. The design's
  "tabs target `#pick-detail` only" applies once that route lands.
- The ET fallback lives in `internal/web/pick.go` rather than `internal/board`, because it is a render detail. It loads
  `America/New_York` and falls back to UTC if the zone data is missing.

## Open issues

- `#pick-body` carries the current `SelfURL`, so a `board-updated` signal reloads both the slate and the open detail.
- Search text still drives both the slate's game filter and the detail's row filter. Phase 5 scopes `q` to the open
  game and adds the odds and line sorts.
- A week with games but no odds still renders its cards alongside the one "No lines for this week yet" message. Phase 5
  can decide whether to hide the cards there.
- The tray rail is temporary; phase 4 replaces it with the dock and dialog sheet.

## Next-phase notes

- `PickBodyView`, `PickSlateView`, `PickGameCardView`, and `PickDetailView` are the seams for phase 2. Add `board.Detail`
  and its tabs/player groups there instead of changing the slate view.
- `pickResults` and the board branch of `updateLeg` both render `pick-body`. Add `GET /slips/{id}/pick/detail` and point
  tab links at `#pick-detail`; keep game cards pointing at `pick-body`.
- `groupByGame` still builds the detail's option rows for this task. Phase 3's `board.Pair` should replace it when the
  paired row becomes the real unit.
- `data-keep-scroll` is on the slate section only. Add the handler to any other pane that must survive a swap.
- Tests assert the card `href`/`hx-get` pair, the ET fallback, the `Wide` classes, the phone hide class, and the scroll
  handler strings in `app.js`. Update those if the names change.
