# Phase 2: Phase 2: Sharp grading factors, signal wiring, and guide docs

## Done

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

## Deviations

### Task 3

- `LinePriceMax` lives on `Rubric` (`RubricV1.LinePriceMax` in `internal/analyzer/rubric.go`) rather than as a package constant in `grade.go`, so `RubricV1.LinePriceMax` was raised from `10` to `15`.

## Open issues

### Task 3

- None.

## Next-phase notes

### Task 3

- `LoadSlipSignals` can now populate `Game.Sharp` directly from `BuildSharpBenchmarks` output; `ScoreLeg` and `GradeRubric()` already handle `Game.Sharp` and fail soft when it is nil.

### Task 4

- Both `LoadSlipSignals` callers (`/slips/{id}/panel` and `/slips/{id}/tray`) now receive `Game.Sharp` automatically from stored week odds.

### Task 5

- All Phase 2 tasks are complete; ready for close-out.

## Files touched

- `docs/concepts/analyzer.md`
- `docs/reference/analyzer-rules.md`
- `internal/analyzer/analyzer.go`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/docs.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/rubric.go`
- `internal/analyzer/rules.go`
- `internal/analyzer/signals.go`
- `internal/analyzer/signals_test.go`
- `internal/web/analyzer_test.go`

## Run log

- Mode: 
- Checkpoints: task-3, task-4, task-5
- Verify: verify passed: test 1.3s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 1
