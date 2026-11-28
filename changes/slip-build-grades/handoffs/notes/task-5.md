# Task 5

## Done

- Added "1. Explaining Vegas (Match First)" (`writeMarketExplanationText`, `writeMarketExplanationMarkdown`) and "2. Beating Vegas (Alpha Gate)" (`writeMarketBeatingText`, `writeMarketBeatingMarkdown`) scorecards to `cmd/eval/main.go` text and markdown outputs (JSON already serializes `Report.MarketExplanation` and `Report.MarketBeating`).
- Added `EvalMarketExplanationView`, `EvalMarketBeatingView`, `buildEvalMarketExplanation`, and `buildEvalMarketBeating` to `internal/web/eval_results.go`.
- Rendered both dual-track scorecards in a phone-first responsive grid in `web/templates/analyzer_results.html` with `Alpha Gate: PASS` / `Alpha Gate: FAIL` status badges and skill-color indicators.
- Added CLI output tests in `cmd/eval/main_test.go` (`TestWriteReportDualTrackScorecards`) and handler/render tests in `internal/web/eval_results_test.go` covering `/docs/evals` and `analyzer-results` rendering.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 2 dual-track evals are complete; Phase 3 can proceed with theme color token updates, 3-layer progressive disclosure UI, and settled-slip Process vs. Outcome badges.

## Commit

Surface dual-track eval scorecards in CLI and /docs/evals

Separating market explanation from market beating makes it clear whether the model first reproduces Vegas odds before claiming any alpha edge. Surfacing both scorecards in cmd/eval and on /docs/evals keeps evaluation transparent across CLI runs and the in-app docs page.
