## Done

- Added `rosterIndex`, `rosterEntry`, `newRosterIndex`, and `addLookup` in `internal/board/board.go` to pre-normalize roster names once per call.
- Updated `Build`, `matchPlayer`, `Slates`, and `gameMatchesQuery` in `internal/board/board.go` to use `rosterIndex` (`Slates` skips building the index when `q.Q` normalizes to empty).
- Added `TestBuildMatchPlayerTeamPriorityAndFallback` and prod-scale benchmarks (`BenchmarkBuild`, `BenchmarkSlates`, `BenchmarkDetail`) in `internal/board/board_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- `BenchmarkBuild`, `BenchmarkSlates`, and `BenchmarkDetail` are in place in `internal/board/board_test.go` (`14` games, `2,100` roster rows, `~14,268` odds rows).
