---
slug: sharp-book-grading
run: 20261124-003711-sharp-book-grading
outcome: done
generated: 2026-11-24T00:59:27Z
---

# Run summary: sharp-book-grading

## Outcome

- Status: done
- Branch: sharp-book-grading
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 5 of 5
- Phases: 2
- Checkpoints: 6
- Duration: 16m7s
- Cost: $—
- Fresh input tokens: 846259
- Cache read tokens: 7943831
- Cache write tokens: 0
- Output tokens: 101863

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 8891953 | $— |

## Phases

### Phase 1: De-vig math and sharp benchmark builder

Tasks:

- [x] Add `ImpliedProb`, `DevigTwoWay`, and `PriceEdge` to `internal/parlay/parlay.go` with table-driven tests in `internal/parlay/parlay_test.go`, and document the formulas in `docs/reference/odds-math.md`.
- [x] Add `SharpKey`, `SharpQuote`, and `BuildSharpBenchmarks` in `internal/analyzer/sharp.go` with unit tests covering tier precedence, two-way pairing for all markets, one-sided fail-soft, and captured `regions=us` fixture payloads.

Phase totals: 2 checkpoints, 4m21s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1, task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add `ImpliedProb`, `DevigTwoWay`, and `PriceEdge` to `internal/parlay/parlay.go` with table-driven tests in `internal/parlay/parlay_test.go`, and document the formulas in `docs/reference/odds-math.md`.

- Added `ImpliedProb`, `DevigTwoWay`, and `PriceEdge` to `internal/parlay/parlay.go` with overround bounds `[0.95, 1.25]` and invalid-input guards.
- Added table-driven unit tests `TestImpliedProb`, `TestDevigTwoWay`, and `TestPriceEdge` to `internal/parlay/parlay_test.go`.
- Documented the new formulas and API entries in `docs/reference/odds-math.md` and removed deprecated `sources`/`last_verified` front-matter fields so `good-vibes docs lint` passes cleanly.

### Task 2: Add `SharpKey`, `SharpQuote`, and `BuildSharpBenchmarks` in `internal/analyzer/sharp.go` with unit tests covering tier precedence, two-way pairing for all markets, one-sided fail-soft, and captured `regions=us` fixture payloads.

- Added `SharpKey`, `SharpQuote`, and `BuildSharpBenchmarks` in `internal/analyzer/sharp.go` with bookmaker tiering (Tier 1: `pinnacle`/`circa`/`bookmaker`, Tier 2: `lowvig`/`betonlineag`, Tier 3: non-DFS/non-exchange consensus with `>= 3` books on the same line).
- Implemented two-way quote pairing and de-vigging across `ML`, `SPREAD`, `TOTAL`, `TEAM_TOTAL`, and player props using `parlay.DevigTwoWay`, `parlay.DecimalToAmerican`, and `db.NormalizePlayerName`, while failing soft on one-sided markets (`ANYTIME_TD`), mismatched lines, and corrupt overrounds.
- Added unit tests in `internal/analyzer/sharp_test.go` covering tier precedence, all market types and player-name normalization, one-sided/invalid fail-soft behavior, and captured `regions=us` fixture payloads from `internal/ingest/odds/testdata/odds_nfl.json`.

### Phase 2: Sharp grading factors, signal wiring, and guide docs

Tasks:

- [x] Add `Sharp` to `analyzer.Game`, implement `sharp_line_edge`, `sharp_line_fade`, `sharp_price_edge`, and `sharp_price_fade` in `internal/analyzer/grade.go` (raising `linePriceMax` to `15`), register them in `GradeRubric()` (`internal/analyzer/docs.go`), and cover all branches and tone rules in `internal/analyzer/analyzer_test.go`.
- [x] Wire `store.OddsForWeek` + `BuildSharpBenchmarks` into `LoadSlipSignals` in `internal/analyzer/signals.go`, add integration tests in `internal/analyzer/signals_test.go` and `internal/web/analyzer_test.go` verifying sharp factors render in the slip panel, pick tray, and `/analyzer/guide`.
- [x] Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` with the sharp-book benchmark tiers, thresholds, deltas, and sharp-friend copy, then run `good-vibes docs lint`.

Phase totals: 4 checkpoints, 11m9s, $—, 7564543 input / 88546 output / 6876476 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, task-5, final Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Add `Sharp` to `analyzer.Game`, implement `sharp_line_edge`, `sharp_line_fade`, `sharp_price_edge`, and `sharp_price_fade` in `internal/analyzer/grade.go` (raising `linePriceMax` to `15`), register them in `GradeRubric()` (`internal/analyzer/docs.go`), and cover all branches and tone rules in `internal/analyzer/analyzer_test.go`.

- Added `Sharp map[SharpKey]SharpQuote` to `analyzer.Game` in `internal/analyzer/analyzer.go` and raised `RubricV1.LinePriceMax` from `10` to `15` in `internal/analyzer/rubric.go`.
- Added `sharp_line_edge`, `sharp_line_fade`, `sharp_price_edge`, and `sharp_price_fade` rule IDs in `internal/analyzer/rules.go` and implemented `scoreSharpBenchmark` inside `scoreLineAndPrice` in `internal/analyzer/grade.go`.
- Registered all four sharp factors in `GradeRubricFor` (`internal/analyzer/docs.go`).
- Added `TestScoreLegSharpBenchmarks` in `internal/analyzer/analyzer_test.go` covering fail-soft behavior, line and price edge/fade thresholds across Tier 1/2 and Tier 3 consensus, player prop thresholds and name normalization, `LinePriceMax` clamping, `GradeRubric()` registration, and friendly tone checks.

### Task 4: Wire `store.OddsForWeek` + `BuildSharpBenchmarks` into `LoadSlipSignals` in `internal/analyzer/signals.go`, add integration tests in `internal/analyzer/signals_test.go` and `internal/web/analyzer_test.go` verifying sharp factors render in the slip panel, pick tray, and `/analyzer/guide`.

- Wired `store.OddsForWeek` and `BuildSharpBenchmarks` into `LoadSlipSignals` in `internal/analyzer/signals.go`, attaching per-game `Sharp` benchmarks to each `analyzer.Game` and failing soft when `games` is empty or `OddsForWeek` returns an error or empty rows.
- Added `TestLoadSlipSignalsPopulatesSharpBenchmarks` in `internal/analyzer/signals_test.go` verifying `Game.Sharp` population for game lines and player props, fail-soft behavior when no odds exist, and end-to-end `ScoreLeg` grading.
- Added `TestSlipPanelTrayAndGuideRenderSharpFactors` in `internal/web/analyzer_test.go` verifying `sharp_line_edge` and `sharp_price_fade` render in `/slips/{id}/panel` and `/slips/{id}/tray`, and all four sharp factors render on `/analyzer/guide`.

### Task 5: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` with the sharp-book benchmark tiers, thresholds, deltas, and sharp-friend copy, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` with the raised Line & price cap (`-20` to `+15`), sharp-book line/price summary, `LoadSlipSignals` odds input, three-tier benchmark rules (Tier 1 Pinnacle/Circa/Bookmaker, Tier 2 LowVig/BetOnline, Tier 3 market consensus), and sharp-friend price copy examples.
- Updated `docs/reference/analyzer-rules.md` with the sharp-book benchmark tiering and de-vigging rules, raised `Line & Price` clamp (`-20` to `+15`), the four sharp scoring factors (`sharp_line_edge`, `sharp_line_fade`, `sharp_price_edge`, `sharp_price_fade`) with their thresholds and deltas, and sharp-friend copy examples.
- Ran `good-vibes docs lint` and verified it exited with 0 errors.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
