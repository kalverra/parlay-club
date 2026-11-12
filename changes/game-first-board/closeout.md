# Close-out

## Consequences

The pick board is now structured around games rather than a flat list of hundreds of disconnected rows. A user browses
the week's slate with kickoff, matchup, lines, and prop counts, opens a specific game detail, navigates by category tabs
(Lines, Passing, Rushing, Receiving, TDs, Kicking), and selects sides from cleanly paired rows.

What got easier:

- Both sides of a line (Over/Under, home/away spread, home/away moneyline) live in one row via `board.Pair`. Adding,
  removing, or switching sides to the opposite choice updates in place through `PUT /legs/{id}` without full-page
  reloads.
- The fixed bottom dock keeps the pick count and live combined odds visible at every viewport width, using the same odds
  calculation as the payout card (`parlay.CombinedOdds` and `parlay.DecimalToAmerican`). Tapping "Slip" opens the tray
  sheet without losing scroll position or context.
- Navigation state (`game`, `cat`, `q`, `win`, `sort`) is preserved in URL query parameters, so browser refresh,
  back/forward buttons, and shared links land on the exact game and tab.
- At `lg`, the slate and game detail scroll independently side-by-side above the fixed dock. On phone screens, opening a
  game transitions into a focused drilldown with a back link to the slate.
- Free-tier guardrails are preserved: browsing games, changing tabs, filtering, and searching consume zero API credits.
  Only an explicit "Load them" tap fetches props for that specific game.

What got harder:

- More partials and URL parameters must stay coordinated (`#pick-body`, `#pick-detail`, `#pick-dock`,
  `pick-line-group`).
- Switching a pick across different lines requires firing both `tray-updated` and `board-updated` so the previously
  picked line does not retain a stale delete button.
- Viewport height math on wide screens (`lg:h-[calc(100dvh-3.5rem-4.5rem)]`) requires precise coordinate sizing to
  prevent the scroll panes from overflowing into the bottom dock.
- The local Sunday sandbox replay needed labeled empty fallbacks so opening non-featured games renders realistic empty
  states rather than failing closed on uncaptured fixtures.

We would revisit showing partner picks and group votes directly on the pick board rows, and caching built board rows if
keystroke latency appears.

## Alternatives considered

- **Client-side React/SPA or JSON index.** Rejected: the app is built on server-rendered Go templates and HTMX. Adding a
  client-side state framework would introduce a second rendering architecture and duplicate grouping, validation, and
  sync logic.
- **Adding more filter chips to the flat list.** Rejected: filters alone cannot provide per-game context or group player
  props across multiple markets; users still had to scroll through hundreds of rows across the entire slate.
- **Three-column desktop layout (slate, detail, tray rail).** Rejected: a three-column layout does not fit comfortably
  at 1024px (`lg`). Replacing the tray rail with a fixed bottom dock and an on-demand tray sheet preserves horizontal
  workspace for lines and player cards.
- **Eager or scheduled prop syncing across the slate.** Rejected: would exhaust the 500-credit monthly Odds API budget.
  Prop fetching remains strictly lazy and scoped to individual games.
- **Full partial reload on leg updates.** Rejected: reloading `#pick-body` on every pick caused visual flicker and lost
  focus. Returning the refreshed `pick-line-group` partial updates the line in place.

## Implementation notes

- **`board.Detail` signature simplification:** `board.Detail` accepts `(game db.Game, rows []Row, q Query)`. The planned
  `roster` argument was omitted because `board.Build` already copies `PlayerID` and `Position` onto each `Row`.
- **Dedicated dock partial:** The design did not specify an endpoint for the dock to refresh independently. Because
  `/tray` returns full tray markup, a new `GET /slips/{id}/pick/dock` partial was added to serve `#pick-dock` updates.
- **HTMX trigger inheritance:** HTMX elements inherit `hx-trigger`. The dock's "Slip" button required an explicit
  `hx-trigger="click"` so background `tray-updated` events would not unintentionally trigger the tray sheet to open.
- **Dual triggers on tray delete:** Deleting a leg from within the tray sheet (`DELETE /legs/{id}?from=tray`) triggers
  both `tray-updated` and `board-updated` so both the dock summary and the board line buttons update immediately.
- **Uncaptured sandbox research fallbacks:** Opening non-featured games in the local Sunday sandbox previously failed
  closed due to uncaptured fixtures. The replay transport now supplies labeled empty fallbacks
  (`X-Sandbox-Replay: empty`) for uncaptured props, rosters, game logs, and summaries so secondary games render normal
  empty states.
- **ET kickoff fallback:** Server-rendered `<time>` elements compute ET formatted kickoffs using `America/New_York`
  (falling back to UTC) to avoid blank timestamps before client-side JavaScript executes.

## Follow-ups

- **Capture prop fixtures for additional sandbox games:** Expand the sandbox registry with captured props for secondary
  slate games so exploring games outside the featured matchup shows populated lines.
- **Display partner picks and votes on pick rows:** Show teammate activity badges or vote tallies directly on the pick
  board's line options to guide collaborative selection.
- **Deep link to player cards:** Allow query parameters or URL anchors to highlight or scroll to specific player cards
  within a game's detail tab.
