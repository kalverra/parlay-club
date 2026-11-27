## Done

- Added `splitLineBadge` and `formatBookQuote` in `internal/web/pick.go` to render `DK` vs. `FD` split lines with the viewer's `homeBook` first whenever `LinesDiffer` is true.
- Wired `SplitLine` (and `SpreadSplit` / `TotalSplit` on slate cards) through `internal/web/pick.go` (`PickGameCardView`, `PickRowView`, `TrayLegView`), `internal/web/research.go` (`ResearchCardView`, `BetResearchView`), and `internal/web/slips.go` (`LegView`, `LegHighlightView`).
- Updated `web/templates/partials.html` (slate cards, `pick-option` buttons, `pick-tray`, `slip-panel` legs, `highlights-strip`, and `research-explore` cards) and `web/templates/research_bet.html` (bet summary and related cards) to render split badges cleanly at phone width, and regenerated `web/static/app.css` via `go generate ./...`.
- Added handler tests `TestPickBoardRendersSplitLinesWithHomeBookFirst` in `internal/web/pick_test.go` and `TestResearchExploreAndBetRenderSplitLinesWithHomeBookFirst` in `internal/web/research_test.go`, and verified `good-vibes verify generate`, `good-vibes verify lint`, and `go test -short ./internal/web/...` pass.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Remaining Phase 2 tasks can build on `splitLineBadge` and `homeBook` ordering for the profile selector and export card.
