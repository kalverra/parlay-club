# Task 4: `board.Detail`, `GameDetail`, `LineGroup`, and `PlayerGroup`

## Done

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

## Deviations

- `Detail` takes `(game db.Game, rows []Row, q Query)` instead of the design's
  `(game, rows, roster, category, query, sort)`. The roster is redundant because `Build` already copies `PlayerID` and
  `Position` onto every prop row, and `Query` carries the category, search, and sort together.
- `GameDetail` gained a `Sort` field the design sketch omitted, so the handler can mark the active sort chip without
  resolving the sort a second time.
- Corrected the design's approach sketch to the actual types and signature; no task text changed.

## Open issues

- `Detail` ignores `Query.Window` and `Query.GameID`. The handler scopes the window and the open game before calling,
  which is how the slate fallback already works.
- `Tab.Count` counts rows, not line groups, so the Lines count includes both sides of a spread and both team totals.
  That is the number a tab badge should show.
- Team totals inside Lines sort by team code, not away-then-home. Only `SPREAD`/`ML` options order the teams by
  away/home.
- Search is row-level through the existing `rowMatches`, so it still matches the game's full team names and venue as
  well as the row text.

## Next-phase notes

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
