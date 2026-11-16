# Phase 1: Phase 1: Pre-index roster lookups and scope `pickPageData`

## Done

### Task 1: Replace the linear roster scan in `internal/board/board.go` (`Build`, `Slates`, `matchPlayer`, `gameMatchesQuery`) with a pre-normalized `rosterIndex` built once per call, with a prod-scale benchmark in `internal/board/board_test.go`.

- Added `rosterIndex`, `rosterEntry`, `newRosterIndex`, and `addLookup` in `internal/board/board.go` to pre-normalize roster names once per call.
- Updated `Build`, `matchPlayer`, `Slates`, and `gameMatchesQuery` in `internal/board/board.go` to use `rosterIndex` (`Slates` skips building the index when `q.Q` normalizes to empty).
- Added `TestBuildMatchPlayerTeamPriorityAndFallback` and prod-scale benchmarks (`BenchmarkBuild`, `BenchmarkSlates`, `BenchmarkDetail`) in `internal/board/board_test.go`.

### Task 2: Trim redundant work in `internal/web/pick.go` (`pickSlate` skips `dockFrom`/`Detail`, `pickDetail` loads only the open game's odds/roster, `pickPageData` avoids calling `board.Slates` twice when `q.Q == ""`), with handler tests in `internal/web/pick_test.go`.

- Updated `pickSlate` in `internal/web/pick.go` to call `pickSlateData`, skipping `LegsForSlip`, `dockFrom`, `boardSignalsForGame`, and `board.Detail`.
- Updated `pickDetail` in `internal/web/pick.go` to call `pickDetailData`, which loads only the open game (`GameByID`, `OddsForGame`, `RosterForTeams([]string{game.AwayTeam, game.HomeTeam})`, `LegsForSlip`), checks week and `SlateScope`, and skips `dockFrom` and other games' odds/rosters.
- Extracted `buildPickSlate` and `buildPickDetail` helpers in `internal/web/pick.go`; `buildPickSlate` skips the second `board.Slates` call when `q.Q == ""`.
- Added handler tests in `internal/web/pick_test.go` (`TestPickSlateSkipsDockAndDetail`, `TestPickDetailScopesToOpenGameAndFiltersScope`, `TestPickPageDataSingleSlatesPassWhenSearchEmpty`).

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 1

- `BenchmarkBuild`, `BenchmarkSlates`, and `BenchmarkDetail` are in place in `internal/board/board_test.go` (`14` games, `2,100` roster rows, `~14,268` odds rows).

### Task 2

- Phase 2 can replace `syncPickIfOpen` in `pickBoard`, `pickResults`, and `pickDetail` with non-blocking `KickPickLines` / `KickPickGame` calls.

## Files touched

- `internal/board/board.go`
- `internal/board/board_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`

## Run log

- Mode: 
- Checkpoints: task-1, task-2
- Verify: verify passed: lint reused, test reused
- Rounds used: 1 of 2; verify failures: 0
