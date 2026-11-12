---
slug: game-first-board
run: 20261112-015804-game-first-board
outcome: done
generated: 2026-11-12T02:52:14Z
---

# Run summary: game-first-board

## Outcome

- Status: done
- Branch: game-first-board
- Final verdict: claude@claude-opus-5-5 approved (issues: 0)

## Totals

- Tasks: 14 of 14
- Phases: 5
- Checkpoints: 15
- Duration: 50m0s
- Cost: $1.66
- Fresh input tokens: 2598195
- Cache read tokens: 65532188
- Cache write tokens: 0
- Output tokens: 406984

| Model | Tokens | Cost |
| --- | ---: | ---: |
| fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash-1017 | 68537367 | $1.66 |

## Phases

### Phase 1: Add the game slate and drilldown

Tasks:

- [x] Add `board.Slates` and the game-line summaries, with `Position` copied onto prop rows, plus tests for kickoff order, the spread/total/moneyline summary, window and query filtering, and dropping kicked-off games.
- [x] Replace the flat pick page with `#pick-body`: a game slate and a detail pane that shows the open game's existing rows. Keep the kickoff-window chips and the matchup search on the slate. Add `Page.Wide`, the `lg` grid, the phone drilldown and back link, the ET fallback time, and the scroll-restore handler. Link game cards with `hx-get` and a plain href fallback.
- [x] Docs: rewrite the pick board section in `docs/concepts/slips.md` and the pick board routes and parameters in `docs/reference/routes.md`.

Phase totals: 3 checkpoints, 9m31s, $0.36, 12903937 input / 85768 output / 12276626 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 2: Open a game with tabs and player clusters

Tasks:

- [x] Add `board.Detail`, `GameDetail`, `LineGroup`, and `PlayerGroup`, with tests for tab order, folding team totals into Lines, and player clustering across markets.
- [x] Add `GET /slips/{id}/pick/detail`, the detail view model, and the pinned header plus horizontal tab bar. Render the Lines tab as game lines then team totals, and prop tabs as player cards with their lines. Keep the existing add/remove buttons inside each row.
- [x] Docs: update the pick board section in `docs/concepts/slips.md` and the detail route in `docs/reference/routes.md`.

Phase totals: 3 checkpoints, 11m5s, $0.30, 14967090 input / 97078 output / 14633642 cache read / 0 cache write tokens

Checkpoints: task-4, task-5, task-6 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Add `board.Detail`, `GameDetail`, `LineGroup`, and `PlayerGroup`, with tests for tab order, folding team totals into Lines, and player clustering across markets.

- Added `board.Tab`, `board.LineGroup`, `board.PlayerGroup`, and `board.GameDetail`, plus
  `board.Detail(game, rows, query)`. `Detail` keeps one game's rows, applies `q`, resolves the open tab, and fills either
  `Lines` (game lines first, then team totals by team) or `Players` (one card per player, lines clustered by market).
- Tab order is Lines, Passing, Rushing, Receiving, TDs, Kicking. `TEAM_TOTAL` folds into Lines, an empty tab stays
  hidden unless active, and an unknown or empty category falls back to Lines, then to the first tab with rows.
- Line pairing folds a group's sides away before home for `SPREAD`/`ML`, Over before Under for totals and props, and a
  single Yes for `ANYTIME_TD`. A missing side leaves one option.
- Added `board.SortLine` and `board.DetailSorts` (player, odds, line). `Detail` resolves kickoff, category, and unknown
  sorts to player.
- Extracted `cardFor` so `Slates` and `Detail` shape the header card the same way. Tests cover tab order and hiding,
  team-total folding, player clustering, option order, the tab fallback, all three detail sorts, detail search, and an
  empty game.

### Task 5: Add `GET /slips/{id}/pick/detail`, the detail view model, and the pinned header plus horizontal tab bar. Render the Lines tab as game lines then team totals, and prop tabs as player cards with their lines. Keep the existing add/remove buttons inside each row.

- Added `GET /slips/{id}/pick/detail` (`pickDetail`) and the `pick-detail` partial. `pickPageData` now shapes the open
  game through `board.Detail` into a `PickDetailView` with tabs, sort chips, line groups, and player cards.
- The Lines tab renders Spread, Moneyline, Total, then one team total per team, with both sides in one row. A prop tab
  renders one card per player, one line per market, and Over/Under options under each line.
- The detail header and a horizontally scrolling tab bar share one sticky block; tab and sort links target
  `#pick-detail` only and each link keeps the other's resolved value in the URL.
- The detail sorts by player by default; `knownSort` now validates against `board.DetailSorts`, so `sort=line` sticks
  instead of resetting to kickoff.
- Add and remove stay inside every `.pick-row` through the unchanged `pick-option` partial. Tests cover the route, tab
  order, Lines ordering, player clustering, link preservation, tab fallbacks, empty states, the remove state, and no
  prop sync.

### Task 6: Docs: update the pick board section in `docs/concepts/slips.md` and the detail route in `docs/reference/routes.md`.

- Rewrote the pick board's detail in `docs/concepts/slips.md`: pinned header, the six category tabs with Lines folding in
  team totals, row-count badges, the Lines ordering, player cards, the player default sort, and `#pick-detail` tab links.
- Updated the lazy-props sentence to say switching tabs spends no credits.
- Added `GET /slips/{id}/pick/detail` to both route tables in `docs/reference/routes.md` and narrowed `/pick/results` to
  search, window, and game changes.
- Corrected the query contract: `cat` is the tab values plus `team_totals` folding into `lines`, and `sort` is `player`
  (default), `odds`, or `line`, with kickoff, category, and unknown values resolving to player.
- Added `board.Detail` to the loading prose and documented the detail's fallback, the placeholder response without an
  open game, and the per-tab player cards.

### Phase 3: Pair both sides of one line

Tasks:

- [x] Add `board.Pair` and its tests: Over/Under, away/home spread and moneyline, the single `ANYTIME_TD` side, a missing side, and deterministic option order.
- [x] Render one row per line with two side buttons. A picked side deletes, the opposite side updates that leg through `PUT /legs/{id}`, and an empty side adds. Return the refreshed pair row from the board update path and keep `Hx-Trigger: tray-updated`. Add handler tests for all three paths.
- [x] Docs: document pairing and the update response in `docs/concepts/slips.md` and `docs/reference/routes.md`.

Phase totals: 3 checkpoints, 10m51s, $0.27, 13386575 input / 69829 output / 13023220 cache read / 0 cache write tokens

Checkpoints: task-7, task-8, task-9 Review fixes: 0 Integration checks: 0

#### Done

### Task 7: Add `board.Pair` and its tests: Over/Under, away/home spread and moneyline, the single `ANYTIME_TD` side, a missing side, and deterministic option order.

- Added `board.Pair(game, rows) []LineGroup`, the exported keyed fold that puts both sides of one line in a single
  group.
- `lineGroups` now calls `Pair` and only adds the detail's sort, so the detail and any later pair-row refresh share one
  pairing rule.
- Option order is fixed: away then home for spread and moneyline, Over then Under for totals and props, and the single
  Yes for `ANYTIME_TD`. Field-level tiebreaks keep duplicate-rank options stable regardless of input order.
- Added tests for Over/Under on a total and a prop, away/home spread and moneyline, the single anytime-TD side, one
  stored side, and deterministic option order under reversed input.

### Task 8: Render one row per line with two side buttons. A picked side deletes, the opposite side updates that leg through `PUT /legs/{id}`, and an empty side adds. Return the refreshed pair row from the board update path and keep `Hx-Trigger: tray-updated`. Add handler tests for all three paths.

- A pair row now renders one line with two side buttons: a picked side deletes, the opposite side updates that leg
  through `PUT /legs/{id}`, and an empty side adds. Both picked sides (from the custom form) stay deletable.
- The add, update, and delete board paths all rebuild the line with `board.Pair` and answer with the refreshed
  `pick-line-group` partial plus `Hx-Trigger: tray-updated`, so only that row swaps. An update that moves the leg to
  another line also fires `board-updated`, so the line it left behind cannot keep a stale delete button.
- Added handler tests for add, opposite-side update through PUT, a cross-line update, delete, and the swap sheet's
  pair-row target.
- Exported `board.LineKey` so the line group's DOM id, the stale-line check, and the swap sheet's target use the same
  identity.
- Updated `docs/concepts/slips.md` and `docs/reference/routes.md` for the pair-row answers.

### Task 9: Docs: document pairing and the update response in `docs/concepts/slips.md` and `docs/reference/routes.md`.

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

### Phase 4: Add the pick dock

Tasks:

- [x] Add the combined price to `PickDockView` from the viewer's legs and its tests, then replace the aside tray with the fixed dock and a `dialog` sheet. Add the dock's open/close handler, refresh it on `tray-updated` and `slip-updated`, and reserve bottom padding so rows are not hidden.
- [x] Docs: update the tray and dock in `docs/concepts/slips.md` and the pick tray description in `docs/concepts/analyzer.md`.

Phase totals: 2 checkpoints, 5m21s, $0.19, 6676906 input / 46315 output / 6353236 cache read / 0 cache write tokens

Checkpoints: task-10, task-11 Review fixes: 0 Integration checks: 0

#### Done

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

### Phase 5: Finish search, sorts, and lazy prompts

Tasks:

- [x] Scope `q` to the open game in the detail, add the odds and line sorts and their tests, and keep the slate search at matchup, team, venue, and roster player.
- [x] Move the unloaded-props prompt onto the slate and the open detail, move the week empty states to the slate, and add the test that browsing never syncs while "Load them" syncs one game.
- [x] Docs: update `docs/reference/routes.md` and the pick board section in `docs/concepts/slips.md`, and correct the tray-at-`lg` sentence in `plan/product.md`.

Phase totals: 4 checkpoints, 13m12s, $0.53, 20195875 input / 107994 output / 19245464 cache read / 0 cache write tokens

Checkpoints: task-12, task-13, task-14, final Review fixes: 0 Integration checks: 0

#### Done

### Task 12: Scope `q` to the open game in the detail, add the odds and line sorts and their tests, and keep the slate search at matchup, team, venue, and roster player.

- Confirmed `board.Detail` keeps only the open game's rows and applies `q` to them, so the detail route, its tab links,
  and its sort links all carry one scoped search.
- Confirmed `board.DetailSorts` names player, odds, and line; `Detail` resolves kickoff, category, and unknown sorts to
  player. The web layer renders those three chips and `knownSort` accepts `sort=line`.
- Confirmed the slate search stays game-level through `gameMatchesQuery`: matchup, team, venue, and roster player, never
  a row's text. Added matchup and full-name assertions to `TestSlatesFilterByWindowAndQuery`.
- Added `TestPickDetailSortsByOddsAndLine`: player is alphabetical, odds puts the shortest price first, and line puts the
  highest number first, asserted from the rendered player cards.
- Added `TestPickDetailSearchStaysInsideTheOpenGame`: the open game's rows match `q`, another game's player does not
  leak in, and a query that matches nothing shows the empty state.

### Task 13: Move the unloaded-props prompt onto the slate and the open detail, move the week empty states to the slate, and add the test that browsing never syncs while "Load them" syncs one game.

- Moved "Load them" from the slate's standalone prompt list onto each game card. The card is now a wrapper with the
  game link plus a bordered footer that names the game and loads its props.
- Added the same prompt to the open detail. The detail's button carries the open game's id, so loading props there
  refreshes `#pick-body` through `board-updated` and the game stays open.
- A prompt appears only for a game that already has stored lines but no prop rows. A week with no odds at all keeps the
  single "No lines for this week yet." state and shows no per-game button. The no-games, no-lines, all-started, and
  no-match states stay on the slate.
- Removed `PickSlateView.Unloaded` and its template loop; `PickGameCardView` now carries `SlipID`, `Unloaded`, and
  `PickDetailView` carries `Unloaded`.
- Kept `TestPickBoardNeverSyncsPropsOnItsOwn` and gave it two unloaded games: browsing every board route still spends no
  credits, and one "Load them" syncs exactly that game. Added `TestPickLoadPropsPromptSitsOnTheCardAndTheDetail` (one
  prompt on the card and one in the detail) and `TestPickGameWithNoLinesShowsTheWeekStateInsteadOfAPropPrompt`.

### Task 14: Docs: update `docs/reference/routes.md` and the pick board section in `docs/concepts/slips.md`, and correct the tray-at-`lg` sentence in `plan/product.md`.

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

## Final gate

- Rounds used: 2; verify failures: 0
- Final verify: no commands configured
- Open issues: none
