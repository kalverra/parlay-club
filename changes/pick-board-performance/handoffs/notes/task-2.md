# Task 2: Trim redundant work in `internal/web/pick.go`

## Done

- Updated `pickSlate` in `internal/web/pick.go` to call `pickSlateData`, skipping `LegsForSlip`, `dockFrom`, `boardSignalsForGame`, and `board.Detail`.
- Updated `pickDetail` in `internal/web/pick.go` to call `pickDetailData`, which loads only the open game (`GameByID`, `OddsForGame`, `RosterForTeams([]string{game.AwayTeam, game.HomeTeam})`, `LegsForSlip`), checks week and `SlateScope`, and skips `dockFrom` and other games' odds/rosters.
- Extracted `buildPickSlate` and `buildPickDetail` helpers in `internal/web/pick.go`; `buildPickSlate` skips the second `board.Slates` call when `q.Q == ""`.
- Added handler tests in `internal/web/pick_test.go` (`TestPickSlateSkipsDockAndDetail`, `TestPickDetailScopesToOpenGameAndFiltersScope`, `TestPickPageDataSingleSlatesPassWhenSearchEmpty`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 can replace `syncPickIfOpen` in `pickBoard`, `pickResults`, and `pickDetail` with non-blocking `KickPickLines` / `KickPickGame` calls.
