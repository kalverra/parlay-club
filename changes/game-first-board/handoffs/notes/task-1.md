# Task 1: Add `board.Slates` and the game-line summaries

## Done

- Added `board.GameCard`, `board.GameLines`, and `board.LineSummary`, plus `board.Slates(games, rows, roster, query)`.
  `Slates` keeps upcoming games only, filters by the query's text (matchup, team, venue, or roster player) and kickoff
  window, orders cards by kickoff, and derives each card's spread, total, and moneyline from the built rows.
- Added `Row.Position` and copied it, with `PlayerID`, from the matching roster player in `Build`. The old
  `matchPlayerID` became `matchPlayer` and returns both values.
- Added tests for kickoff order, the spread/total/moneyline summary, prop counts, window and query filtering, and
  dropping kicked-off games, plus one for `Position` on prop rows.

## Deviations

None.

## Open issues

- `GameCard.Props` counts every non-Lines row, so team totals count as props, matching `IsPropRow` and the existing
  `hasProps` check in `Search`. Task 2's "Explore N props" label should use the same definition.
- `Slates` does not return a started-game count. The slate handler in task 2 should compute it from `games` and the
  clock for the "all games started" empty state and the fallback message.
- Search matching now includes `VenueName` and `VenueID` for every caller of `gameMatchesQuery`, including
  `Search`'s unloaded-props prompt. That is a small, intended widening.

## Next-phase notes

- `GameCard.Away` and `GameCard.Home` are team codes, not full names, so option ordering and the away/home spread
  comparison can key on `row.Team`. Full names are not on the card; fetch them from the game in the handler if the
  detail header needs them.
- `GameCard.Matchup` is the existing `"AWAY @ HOME"` code form and `Venue` is `VenueName` (empty when unset).
- A game with no odds still produces a card, with nil `Lines` and zero `Props`; nil spread, total, or moneyline means
  the market was never stored, not that the price is missing.
