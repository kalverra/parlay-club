# Task 7 Notes

## Done

- Added `BoardSignals` (`TeamEPA`, `PlayerUsage`, `Injuries`, `SlipLegs`), `Row.Badge`, `PlayerGroup.Tags`, `Query.Signals`, and `ApplySignals` in `internal/board`.
- Implemented the four suggestion badges (`Smash matchup`, `High volume`, `Role bump`, `Fits slip`) with priority ordering and a 2-tag cap on `PlayerGroup.Tags`.
- Added `SortMatchup` (`"matchup"`, label `"Matchup"`) to `DetailSorts` and wired signal-strength + opponent defense EPA rank sorting into `sortLineGroups` and `sortPlayers`.
- Added table-driven tests for all four badges, tag priority/capping, fail-soft nil/empty inputs, and `SortMatchup` ordering in `internal/board/signals_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `internal/web/pick.go` can pass `Signals: board.BoardSignals{TeamEPA: ..., PlayerUsage: ..., Injuries: ..., SlipLegs: ...}` in `board.Query` when calling `board.Detail`.
- `BoardSignals.Injuries` is keyed by ESPN team abbreviation (`map[string][]espn.Injury`) and expects `Status: "Out"` for `Role bump` detection.
