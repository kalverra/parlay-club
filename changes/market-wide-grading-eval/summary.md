---
slug: market-wide-grading-eval
run: 20261125-014556-market-wide-grading-eval
outcome: done
generated: 2026-11-25T02:16:56Z
---

# Run summary: market-wide-grading-eval

## Outcome

- Status: done
- Branch: market-wide-grading-eval
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 7 of 7
- Phases: 3
- Checkpoints: 7
- Duration: 28m8s
- Cost: $—
- Fresh input tokens: 1507081
- Cache read tokens: 19460600
- Cache write tokens: 0
- Output tokens: 295413

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 21263094 | $— |

## Phases

### Phase 1: Pre-kickoff sharp benchmarks and nflverse L5 fallback in eval replay

Tasks:

- [x] Export `odds.ParseLinesSnapshotJSON` in `internal/ingest/odds/lines.go` and `analyzer.HasSharpBenchmark` in `internal/analyzer/grade.go`, reconstruct `Game.Sharp` from pre-kickoff archived `odds:lines` and `odds:props:{gameID}` (with `BodySHA256` body memoization and pre-kickoff roster/prior-week nflverse player resolution) in `internal/eval/replay.go`, track `DataCoverage.WithSharp`, and add unit tests verifying sharp rule firing and strict pre-kickoff isolation.
- [x] Add strictly prior-week `nflverse_player_weeks` L5 fallback for `PlayerStats.Games` (and position backfill) in `internal/eval/replay.go` when a player lacks a pre-kickoff archived ESPN game log, track `DataCoverage.WithL5Fallback`, and add unit tests verifying L5 hit-rate/median-trap firing and zero same-week/future leakage.

Phase totals: 2 checkpoints, 8m18s, $—, 5123182 input / 74827 output / 4769358 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Export `odds.ParseLinesSnapshotJSON` in `internal/ingest/odds/lines.go` and `analyzer.HasSharpBenchmark` in `internal/analyzer/grade.go`, reconstruct `Game.Sharp` from pre-kickoff archived `odds:lines` and `odds:props:{gameID}` (with `BodySHA256` body memoization and pre-kickoff roster/prior-week nflverse player resolution) in `internal/eval/replay.go`, track `DataCoverage.WithSharp`, and add unit tests verifying sharp rule firing and strict pre-kickoff isolation.

- Exported `odds.ParseLinesSnapshotJSON` in `internal/ingest/odds/lines.go` (sharing `parseEvents` in `internal/ingest/odds/events.go`) to decode event metadata and priced game lines in one pass.
- Exported `analyzer.HasSharpBenchmark` in `internal/analyzer/grade.go` to report whether `Game.Sharp` has a benchmark for a leg's `SharpKey`.
- Added `DataCoverage.WithSharp` in `internal/eval/types.go` and reconstructed `Game.Sharp` in `internal/eval/replay.go` from pre-kickoff archived `odds:lines` (memoized per week by `BodySHA256`) and `odds:props:{gameID}` (with `db.BatchPlayerResolver` built from pre-kickoff archived rosters and prior-week `nflverse_player_weeks`).
- Added unit tests in `internal/ingest/odds/archive_parse_test.go`, `internal/analyzer/sharp_test.go`, and `internal/eval/replay_test.go` verifying `ParseLinesSnapshotJSON`, `HasSharpBenchmark`, sharp rule firing (`sharp_line_edge`, `sharp_price_edge`), `WithSharp` coverage, and strict pre-kickoff and prior-week isolation.

### Task 2: Add strictly prior-week `nflverse_player_weeks` L5 fallback for `PlayerStats.Games` (and position backfill) in `internal/eval/replay.go` when a player lacks a pre-kickoff archived ESPN game log, track `DataCoverage.WithL5Fallback`, and add unit tests verifying L5 hit-rate/median-trap firing and zero same-week/future leakage.

- Added `DataCoverage.WithL5Fallback` in `internal/eval/types.go` to track legs backed by prior-week nflverse game logs.
- Extended `loadPriorNFLverse` and `loadArchivedPlayerStats` in `internal/eval/replay.go` (with `buildNFLversePlayerGames`) to populate `PlayerStats.Games` (`PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`, `ANYTIME_TD`, `KICKER_PTS`) most-recent-first from strictly prior-week `nflverse_player_weeks` rows when no pre-kickoff archived ESPN game log exists, and to backfill `PlayerStats.Position` from the latest prior-week nflverse row when pre-kickoff rosters omit the player.
- Updated `accumulateWeekCoverage` in `internal/eval/replay.go` to count `WithL5Fallback` only when a player has no pre-kickoff ESPN game log.
- Added `TestLoadDatasetNFLverseL5Fallback` in `internal/eval/replay_test.go` verifying `hit_rate` and `median_trap` rule firing, pre-kickoff ESPN game log precedence, position backfill, `WithL5Fallback` coverage accounting, and zero same-week or future leakage.

### Phase 2: Market-wide candidate builder, deterministic grader, and evaluator

Tasks:

- [x] Export `espn.ParseBoxScoreJSON` in `internal/ingest/espn/boxscore.go` and `Store.LatestAPIResponse` in `internal/db/archive_read.go`, then implement the market-wide candidate builder (`internal/eval/market.go`) using `board.Build` median deduplication and deterministic offline grader (`internal/eval/market_grade.go`) over final scores, completed archived ESPN box scores, and same-week `nflverse_player_weeks` with unit tests covering all markets and missing-data guards.
- [x] Add `ReplayMode` (`ModeSlips` / `ModeMarket`) to `ReplayFilter`, `MarketLegs` to `ReplayedWeek`, and market-leg scoring and rubric migration matching in `internal/eval/metrics.go` and `internal/eval/replay.go`, with end-to-end replay and comparison tests verifying calibration, lift, flat ROI, Brier score, factor attribution, and empty slip sections.

Phase totals: 2 checkpoints, 8m22s, $—, 5621210 input / 72599 output / 5253264 cache read / 0 cache write tokens

Checkpoints: task-3, task-4 Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Export `espn.ParseBoxScoreJSON` in `internal/ingest/espn/boxscore.go` and `Store.LatestAPIResponse` in `internal/db/archive_read.go`, then implement the market-wide candidate builder (`internal/eval/market.go`) using `board.Build` median deduplication and deterministic offline grader (`internal/eval/market_grade.go`) over final scores, completed archived ESPN box scores, and same-week `nflverse_player_weeks` with unit tests covering all markets and missing-data guards.

- Exported `espn.ParseBoxScoreJSON` in `internal/ingest/espn/boxscore.go` and `Store.LatestAPIResponse` in `internal/db/archive_read.go`, with unit tests in `internal/ingest/espn/boxscore_test.go` and `internal/db/archive_test.go`.
- Implemented `BuildMarketCandidates` in `internal/eval/market.go` using `board.Build` (`homeBook == ""`, `now == time.Time{}`) for median line and median price deduplication, dropping non-`FINAL` games, `OTHER`/unsupported markets, zero/invalid odds, and missing lines on line-based markets.
- Implemented `MarketGrader` (`NewMarketGrader`, `GradeLeg`, `GradeCandidates`) in `internal/eval/market_grade.go` grading game lines and `TEAM_TOTAL` from final scores, and player props from `game.BoxScoreJSON`, completed archived `espn:summary:{gameID}` responses, or same-week `nflverse_player_weeks` fallback (dropping unmatched players and `Pending`/`NeedsGrading` results).
- Added unit tests in `internal/eval/market_test.go` covering multi-book median deduplication, all markets, ESPN box score DNP voids vs. nflverse unmatched drops, and missing-data/cross-week guards.

### Task 4: Add `ReplayMode` (`ModeSlips` / `ModeMarket`) to `ReplayFilter`, `MarketLegs` to `ReplayedWeek`, and market-leg scoring and rubric migration matching in `internal/eval/metrics.go` and `internal/eval/replay.go`, with end-to-end replay and comparison tests verifying calibration, lift, flat ROI, Brier score, factor attribution, and empty slip sections.

- Added `ReplayMode` (`ModeSlips`, `ModeMarket`) to `ReplayFilter` and `MarketLegs` (`market_legs`) to `ReplayedWeek` in `internal/eval/types.go` and `internal/eval/replay.go`.
- Updated `Evaluate` and `legOutcomeKey` in `internal/eval/metrics.go` to score `MarketLegs` with `analyzer.ScoreLegWithRubric`, compute leg calibration, lift, flat ROI, Brier score, factor attribution, and cross-week rubric migrations while leaving `SlipBands` and `SlipFactors` empty.
- Wired `ModeMarket` into `LoadDataset` in `internal/eval/replay.go` using `BuildMarketCandidates` and `NewMarketGrader`, reconstructing pre-kickoff game signals, sharp benchmarks, and archived/nflverse L5 player stats without loading user slips, and updated `LoadDatasetJSON` to count `MarketLegs`.
- Added unit and end-to-end tests (`TestEvaluateMarketLegsAndMigrations` in `internal/eval/metrics_test.go` and `TestLoadDatasetMarketModeEndToEnd` in `internal/eval/replay_test.go`).

### Phase 3: Cache invalidation, CLI market mode, and `/analyzer/results` UI

Tasks:

- [x] Extend `eval.Cache` with `GetOrComputeMode` and `InvalidateMarket`, wire `refresh.Service.SetOnMarketDataChange` (on newly `FINAL` games in `syncScoreboard` and `syncNFLverse` changes) in `cmd/server/main.go`, and add `--mode slips|market` plus sharp/L5-fallback coverage output to `cmd/eval/main.go` with cache, refresh, and CLI tests.
- [x] Add `?source=slips|market` to `GET /analyzer/results` (`internal/web/eval_results.go`) and phone-first source switcher pills plus sharp/L5-fallback coverage tiles to `web/templates/analyzer_results.html` (hiding slip-only sections in market mode and preserving `?source=` on filter URLs), run `go generate ./...`, and add HTTP tests.
- [x] Docs: update `docs/concepts/analyzer.md`, `docs/reference/routes.md`, and `docs/guides/local-dev.md` for market-wide grading evaluation, sharp/L5 signal reconstruction, `--mode market`, and `?source=market`, and run `good-vibes docs lint`.

Phase totals: 3 checkpoints, 10m47s, $—, 9473123 input / 137016 output / 8773643 cache read / 0 cache write tokens

Checkpoints: task-5, task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 5: Extend `eval.Cache` with `GetOrComputeMode` and `InvalidateMarket`, wire `refresh.Service.SetOnMarketDataChange` (on newly `FINAL` games in `syncScoreboard` and `syncNFLverse` changes) in `cmd/server/main.go`, and add `--mode slips|market` plus sharp/L5-fallback coverage output to `cmd/eval/main.go` with cache, refresh, and CLI tests.

- Extended `eval.Cache` in `internal/eval/cache.go` with `GetOrComputeMode` and `InvalidateMarket`, keeping `ModeSlips` and `ModeMarket` entries and generation counters isolated while `InvalidateAll` clears both.
- Added `refresh.Service.SetOnMarketDataChange` in `internal/refresh/refresh.go`, firing on newly `FINAL` games in `syncScoreboard` (`internal/refresh/sources.go`) and committed changes in `syncNFLverse` (`internal/refresh/nflverse.go`).
- Wired `refresher.SetOnMarketDataChange(evalCache.InvalidateMarket)` in both sandbox and production setup in `cmd/server/main.go`.
- Added `--mode slips|market` (rejecting invalid modes and `--mode` with `--dataset`), `sharp` and `l5_fallback` coverage output, and empty `SlipBands` / `SlipBandDiffs` section omission to `cmd/eval/main.go`.
- Added cache mode-isolation/invalidation tests (`internal/eval/cache_test.go`), refresh callback tests (`internal/refresh/sources_test.go`, `internal/refresh/nflverse_test.go`), and CLI market-mode tests (`cmd/eval/main_test.go`).

### Task 6: Add `?source=slips|market` to `GET /analyzer/results` (`internal/web/eval_results.go`) and phone-first source switcher pills plus sharp/L5-fallback coverage tiles to `web/templates/analyzer_results.html` (hiding slip-only sections in market mode and preserving `?source=` on filter URLs), run `go generate ./...`, and add HTTP tests.

- Added `Store.FinalSeasons` in `internal/db/record.go` to query seasons with `FINAL` games, newest first.
- Added `?source=slips|market` parsing (returning `400 Bad Request` on invalid values), `EvalCache.GetOrComputeMode` lookup, `FINAL`-game default season resolution, `Source`/`IsMarket`/`SlipsURL`/`Sources` view state, and `WithSharp`/`WithL5Fallback` coverage fields to `internal/web/eval_results.go`, preserving `?source=` on all season and rubric filter URLs.
- Updated `web/templates/analyzer_results.html` with phone-first `User slips` / `Market-wide` source switcher pills, market-mode empty-state copy, hidden `Slip Calibration (A-F)` and `Slip Factor Attribution` sections (with a link note to `User slips`), and `nflverse L5 fallback` + `Sharp benchmarks` tiles in `Pre-Kickoff Data Coverage`, then ran `go generate ./...`.
- Added HTTP tests in `internal/web/eval_results_test.go` covering invalid `?source=` handling, empty market state, `FINAL`-game default season resolution, hidden slip-only sections, `?source=` preservation on filter URLs, and `InvalidateMarket` cache eviction.

### Task 7: Docs: update `docs/concepts/analyzer.md`, `docs/reference/routes.md`, and `docs/guides/local-dev.md` for market-wide grading evaluation, sharp/L5 signal reconstruction, `--mode market`, and `?source=market`, and run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` to document pre-kickoff sharp benchmark and prior-week nflverse L5 fallback reconstruction, `ModeMarket` consensus deduplication and offline deterministic grading, `?source=slips|market` on `/analyzer/results`, and mode-isolated `eval.Cache` invalidation.
- Updated `docs/reference/routes.md` to document `?source=slips|market` on `GET /analyzer/results`, `FinalSeasons` default season resolution, `400 Bad Request` handling, hidden slip-only sections, extra coverage tiles, and `InvalidateMarket` cache eviction.
- Updated `docs/guides/local-dev.md` to document `cmd/eval --mode slips|market`, `sharp` and `l5_fallback` coverage counts, `mise run eval-prod -- --mode market --all`, and removed the stale `./data/` backtick path lint error.
- Ran `good-vibes docs lint`, passing with `0 error(s)`.

## Final gate

- Rounds used: 0; verify failures: 0
- Final verify: passed
- Open issues: none
