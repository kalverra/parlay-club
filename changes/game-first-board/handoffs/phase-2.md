# Phase 2: Phase 2: Open a game with tabs and player clusters

## Done

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

## Deviations

### Task 4

- `Detail` takes `(game db.Game, rows []Row, q Query)` instead of the design's
  `(game, rows, roster, category, query, sort)`. The roster is redundant because `Build` already copies `PlayerID` and
  `Position` onto every prop row, and `Query` carries the category, search, and sort together.
- `GameDetail` gained a `Sort` field the design sketch omitted, so the handler can mark the active sort chip without
  resolving the sort a second time.
- Corrected the design's approach sketch to the actual types and signature; no task text changed.

### Task 5

- Tabs and sorts reuse `PickChip` (`Label`, `Href`, `PushURL`, `Count`, `Active`) instead of a new tab view type, because
  `board.Tab` carries no links.
- Prop options show a short label ("Over 245.5", "Yes") because the player's name is the card heading. Lines rows keep
  the full `Row.Text`. The posted fields are unchanged, so add and remove behave the same.
- An unknown `sort` clears to empty in `pickQuery` and `board.Detail` resolves player, instead of writing
  `sort=player` into every slate card link.
- Fixed `board.marketHeading` to label `ANYTIME_TD` "Anytime TD"; the player card needs a market heading there.
- Added a `no-scrollbar` utility to `web/tailwind/input.css` and regenerated `web/static/app.css`.
- `PickGameGroup`, `groupByGame`, `pairKey`, `categoryChips`, and `sortChips` are gone; `board.Detail` owns those shapes.

### Task 6

- None.

## Open issues

### Task 4

- `Detail` ignores `Query.Window` and `Query.GameID`. The handler scopes the window and the open game before calling,
  which is how the slate fallback already works.
- `Tab.Count` counts rows, not line groups, so the Lines count includes both sides of a spread and both team totals.
  That is the number a tab badge should show.
- Team totals inside Lines sort by team code, not away-then-home. Only `SPREAD`/`ML` options order the teams by
  away/home.
- Search is row-level through the existing `rowMatches`, so it still matches the game's full team names and venue as
  well as the row text.

### Task 5

- The pick-body's `hx-get` live refresh URL is fixed at page load. A tab or sort swap replaces only `#pick-detail`, so a
  later `board-updated` refetches the body with the old tab and sort and resets it. `SelfURL` is seeded with the
  resolved tab and sort, which narrows the window but does not close it. Give the detail and slate their own refresh
  URLs, or sync the pick-body attribute in `app.js`, in a later phase.
- `result.Rows` from `board.Search` is now unused in `pickPageData`; only `Started` and `Unloaded` are read. Phase 5 can
  trim the call if it keeps the unloaded-props prompts.
- `LineGroup.Label` is still empty for a market with no `marketLabel`; only `ANYTIME_TD` was filled in. A new market
  needs a heading or its card renders without one.
- `/pick/detail` with no open game returns the `pick-detail-empty` placeholder, not the slate. The full page still falls
  back to the slate.
- Docs are untouched here. `docs/concepts/slips.md` and `docs/reference/routes.md` still describe the flat detail rows,
  the kickoff default sort, and the category chip row; phase 2's docs task owns those updates.

### Task 6

- The routes doc still says `#pick-body` re-fetches on `board-updated`, which is true, but the fixed live refresh URL
  from task 5 still resets the open tab and sort after a later signal. Phase 4's dock work should close that gap or the
  route doc should note the behavior.
- The tray is still described as an aside rail beside the body; phase 4 replaces it with the fixed dock and must revisit
  the layout sentence in `slips.md` and the `Page.Wide` wording in `routes.md`.

## Next-phase notes

### Task 4

- Pass the built rows from `board.Build`; the whole week's rows or the one game's rows both work, since `Detail` filters
  by `game.ID`.
- `GameDetail.Game` is built from the game's unfiltered rows, so a search narrows the tab's rows but never blanks the
  header's lines or prop count.
- `Lines` is filled only when `GameDetail.Category` is Lines; `Players` only otherwise. Both tabs and lines can be
  empty for a game with no odds, which is the detail's empty state.
- Tab links should set `cat=<tab.Category>`; never emit `team_totals`, which folds back to Lines. `Tab.Label` and
  `Tab.Count` are ready for the pinned tab bar.
- Switch `sortChips` and `knownSort` in `internal/web/pick.go` from `board.Sorts` to `board.DetailSorts` and mark the
  active chip from `detail.Sort`. Until then `sort=line` resets to kickoff in the URL.
- The web layer still builds its own `groupByGame` groups. Replace those with `detail.Lines` and `detail.Players`; the
  existing add/remove buttons keep working inside each `LineGroup.Options` row.
- Docs are untouched by this task. `docs/concepts/slips.md` and `docs/reference/routes.md` still describe the flat
  detail rows, the kickoff default sort, and the category chips; phase 2's docs task owns those updates.

### Task 5

- Phase 3's pairing replaces the options inside `PickLineGroupView.Options`. The template seam is `pick-line-group`, and
  each option is still a `.pick-row`, so the add/remove targets keep working.
- `PickLineGroupView` and `PickPlayerView` are the detail seams. `pickLineGroupView` takes `prop bool` to choose the
  short prop label.
- Tab links set `cat=<tab.Category>` and never emit `team_totals`. `detailHref`/`detailPushURL` keep q, win, game, the
  resolved category, and the resolved sort; reuse them for new detail links.
- `data-category` and `data-sort` on `#pick-detail` name the resolved tab and sort, which is the easiest test hook.
- Test helpers `pickDetail` and `addTeamTotal` live in `internal/web/pick_test.go`.

### Task 6

- The docs now name the tab values, the `team_totals` fold, the `lines` fallback, the player default sort, and
  `#pick-detail`. Pairing in phase 3 does not change any of that unless the pair row replaces the whole detail partial.
- If phase 3 changes the board answer from `#pick-body` to a single row, update `routes.md`'s `PUT /legs/{id}` row and
  the `UpdateLeg` sentence in `slips.md`.
- `good-vibes docs lint` passes. No summary changed, so `docs index` was not needed.

## Files touched

- `docs/concepts/slips.md`
- `docs/reference/routes.md`
- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/render.go`
- `internal/web/slips.go`
- `web/static/app.css`
- `web/tailwind/input.css`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-4, task-5, task-6
- Verify: verify passed: no checks configured
- Rounds used: 1 of 2; verify failures: 0
