# Task 2: Pure highlight selector in `internal/board`

## Done

- Implemented `RowToAnalyzerLeg`, `LegHighlight`, `ParlayHighlight`, `HighlightOptions`, `HighlightSet`, and `SelectHighlights` in [highlights.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/board/highlights.go).
- Added deterministic ranking, per-player and same-line-group deduplication, and bounded 2- and 3-leg parlay combo generation (top 12 A-grade legs plus up to 6 B-grade stack partners for `stack_shootout` and `stack_clock_drain`).
- Added unit tests and `BenchmarkSelectHighlightsFullSlate` in [highlights_test.go](file:///Users/adamhamrick/Projects/parlay-club/.good-vibes/worktrees/grade-highlights/internal/board/highlights_test.go) (~3.25 ms/op across 16 games / ~1,632 rows on Apple M3 Max).
- Verified `golangci-lint run ./internal/board/... --fix` (0 issues) and `go test -short ./internal/board/...` pass.

## Deviations

- `board.Row` stores American odds in `Row.Odds` rather than `Row.Price`, so ranking and `parlay.CombinedOdds` use `Row.Odds`.
- Negative values in `HighlightOptions.MaxLegs`, `MaxParlays`, and `MaxPerPlayer` explicitly disable that cap (0 legs/parlays or unlimited per player), while `0` selects the design defaults (`3`, `2`, `1`).

## Open issues

None.

## Next-phase notes

`SelectHighlights` is pure and expects `games` and `stats` from `analyzer.LoadBoardSignals`. In `internal/web`, pass `SameGameOnly: true` and `MaxParlays: 1` for `#pick-detail`, and `ExcludePicked: true` on both hub and game-detail views.

## Commit

Add pure A-grade highlight selector in board package

Score cached board rows with the analyzer engine to surface top A-grade single legs and conflict-free 2- and 3-leg parlay combos without extra API calls. Bounding the combo pool keeps full-slate selection in low single-digit milliseconds for synchronous hub and pick-board renders.
