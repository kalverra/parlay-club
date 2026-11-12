# Task 12: Detail search, odds and line sorts, and the slate search

## Done

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

## Deviations

None. The production behavior described by this task had already landed in tasks 4 and 5 (`board.Detail`,
`DetailSorts`, the detail chips, and the slate search), so this turn added the missing test coverage and verified the
behavior instead of rewriting it.

## Open issues

- `board.Sorts` (kickoff, player, category, odds) is now used only by `TestSorts`. The slate has no sort chips, and
  the detail uses `DetailSorts`. Nothing breaks; the next cleanup can drop it or give it a caller.
- A game-level search such as a venue or a roster player with no props matches the game card but no row, so opening that
  game shows "Nothing matches that search." The design says the detail matches rows only, so this is the intended
  behavior, not a bug.

## Next-phase notes

- The lazy-props task does not need to touch search or sort. Every route stay is credit-free: `TestPickBoardNeverSyncsPropsOnItsOwn`
  covers the board, search, filter, sort, detail, dock, and tray paths.
- Keep `q` as one parameter. The slate reads it game-level through `gameMatchesQuery`; the detail reads it row-level
  through `rowMatches`. Changing either rule needs both board tests and the two web tests above.
- The detail's empty state distinguishes no rows from no search match with `.Search`, so a new empty state should keep
  that switch intact.
