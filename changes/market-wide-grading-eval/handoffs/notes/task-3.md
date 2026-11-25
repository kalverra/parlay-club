# Task 3

## Done

- Exported `espn.ParseBoxScoreJSON` in `internal/ingest/espn/boxscore.go` and `Store.LatestAPIResponse` in `internal/db/archive_read.go`, with unit tests in `internal/ingest/espn/boxscore_test.go` and `internal/db/archive_test.go`.
- Implemented `BuildMarketCandidates` in `internal/eval/market.go` using `board.Build` (`homeBook == ""`, `now == time.Time{}`) for median line and median price deduplication, dropping non-`FINAL` games, `OTHER`/unsupported markets, zero/invalid odds, and missing lines on line-based markets.
- Implemented `MarketGrader` (`NewMarketGrader`, `GradeLeg`, `GradeCandidates`) in `internal/eval/market_grade.go` grading game lines and `TEAM_TOTAL` from final scores, and player props from `game.BoxScoreJSON`, completed archived `espn:summary:{gameID}` responses, or same-week `nflverse_player_weeks` fallback (dropping unmatched players and `Pending`/`NeedsGrading` results).
- Added unit tests in `internal/eval/market_test.go` covering multi-book median deduplication, all markets, ESPN box score DNP voids vs. nflverse unmatched drops, and missing-data/cross-week guards.

## Deviations

None.

## Open issues

None.

## Next-phase notes

Wire `BuildMarketCandidates` and `NewMarketGrader` / `GradeCandidates` into `ModeMarket` replay (`internal/eval/replay.go`, `internal/eval/types.go`, and `internal/eval/metrics.go`).

## Commit

Add market-wide candidate builder and offline grader

Export offline ESPN box score JSON parsing and latest archived API response lookup so market-wide evaluation can deduplicate pre-kickoff odds via board.Build and deterministically grade all markets offline without post-kickoff leakage or false voids.
