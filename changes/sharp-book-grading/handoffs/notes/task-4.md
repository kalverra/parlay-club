# Task 4

## Done

- Wired `store.OddsForWeek` and `BuildSharpBenchmarks` into `LoadSlipSignals` in `internal/analyzer/signals.go`, attaching per-game `Sharp` benchmarks to each `analyzer.Game` and failing soft when `games` is empty or `OddsForWeek` returns an error or empty rows.
- Added `TestLoadSlipSignalsPopulatesSharpBenchmarks` in `internal/analyzer/signals_test.go` verifying `Game.Sharp` population for game lines and player props, fail-soft behavior when no odds exist, and end-to-end `ScoreLeg` grading.
- Added `TestSlipPanelTrayAndGuideRenderSharpFactors` in `internal/web/analyzer_test.go` verifying `sharp_line_edge` and `sharp_price_fade` render in `/slips/{id}/panel` and `/slips/{id}/tray`, and all four sharp factors render on `/analyzer/guide`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Both `LoadSlipSignals` callers (`/slips/{id}/panel` and `/slips/{id}/tray`) now receive `Game.Sharp` automatically from stored week odds.

## Commit

Wire sharp benchmarks into LoadSlipSignals

LoadSlipSignals now reads stored week odds via OddsForWeek and attaches BuildSharpBenchmarks to each Game so slip panel, pick tray, and leg grading include sharp-book comparisons with zero extra API calls.
