# Task 1

## Done

- Exported `odds.ParseLinesSnapshotJSON` in `internal/ingest/odds/lines.go` (sharing `parseEvents` in `internal/ingest/odds/events.go`) to decode event metadata and priced game lines in one pass.
- Exported `analyzer.HasSharpBenchmark` in `internal/analyzer/grade.go` to report whether `Game.Sharp` has a benchmark for a leg's `SharpKey`.
- Added `DataCoverage.WithSharp` in `internal/eval/types.go` and reconstructed `Game.Sharp` in `internal/eval/replay.go` from pre-kickoff archived `odds:lines` (memoized per week by `BodySHA256`) and `odds:props:{gameID}` (with `db.BatchPlayerResolver` built from pre-kickoff archived rosters and prior-week `nflverse_player_weeks`).
- Added unit tests in `internal/ingest/odds/archive_parse_test.go`, `internal/analyzer/sharp_test.go`, and `internal/eval/replay_test.go` verifying `ParseLinesSnapshotJSON`, `HasSharpBenchmark`, sharp rule firing (`sharp_line_edge`, `sharp_price_edge`), `WithSharp` coverage, and strict pre-kickoff and prior-week isolation.

## Deviations

None.

## Open issues

None.

## Next-phase notes

`loadPriorNFLverse` in `internal/eval/replay.go` already loads `NFLversePlayerWeeksForSeason` for prior-week player resolution; the nflverse L5 fallback task can reuse or extend that per-week query to populate `PlayerStats.Games` when no pre-kickoff `espn:gamelog:{id}` exists.
