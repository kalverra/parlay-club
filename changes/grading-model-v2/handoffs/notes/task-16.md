# Task 16

## Done

- Updated `internal/web/eval_results.go` (`EvalResultsPage`, `EvalEngineOption`, `EvalEngineComparisonView`, `EvalCalibrationView`, `EvalReliabilityBinView`) to support `?engine=` (with backwards-compatible `?rubric=` fallback), map `report.Calibration` metrics, format 10-bin reliability rows, compute SGP correlation lift (`NaiveIndepLogLoss - SGPLogLoss`), and build side-by-side `v1` vs. `v2` comparison summaries.
- Updated `web/templates/analyzer_results.html` with the `Engine:` switcher (`v1` / `v2`), Probability Calibration summary tiles (Log Loss, Brier Score, Classwise ECE, ≥80% Bin Spread, Market Log Loss, Market Brier), SGP Correlation Lift panel, phone-first Reliability Table (`sm:hidden` compact bin rows + `sm:block` table), and side-by-side Engine Comparison cards/table.
- Added handler and rendering tests in `internal/web/eval_results_test.go` (`TestEvalResultsEngineSelectionAndCalibration`, `TestAnalyzerResultsTemplateRendersCalibrationAndPhoneWidthLayout`) covering `v1` vs. `v2` selection, `?rubric=` fallback, market-mode SGP hiding, calibration tiles, reliability bins, SGP correlation lift, and phone-width markup.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 7 (`internal/web/analyzer.go`, `web/templates/partials.html`, `web/templates/analyzer_guide.html`, and `docs/reference/routes.md`) should document `?engine=` alongside `?rubric=` on `GET /analyzer/results` when updating living docs after v2 promotion.

## Commit

Show v1 vs v2 calibration and SGP lift on results

Surface engine selection, proper scoring calibration tiles, reliability bins, and SGP copula correlation lift on /analyzer/results at phone width. This lets users and maintainers compare EngineV1 and EngineV2 side-by-side in the app before promoting v2.
