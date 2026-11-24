## Done

- Rendered `SlipFactors` in `cmd/eval/main.go` for both text (`Slip Factor Attribution:` / `Candidate Slip Factor Attribution`) and markdown (`#### Slip Factor Attribution` / `### Candidate Slip Factor Attribution`) outputs when non-empty, omitting the section cleanly when empty.
- Added `SlipFactors []EvalFactorView` to `EvalResultsPage` in `internal/web/eval_results.go` populated via `buildEvalFactors(report.SlipFactors)`.
- Added a secondary `Slip Factor Attribution` section (mobile cards and desktop table, with sample counts `N`, `W-L`, `Hit %`, `Lift`, and `Avg Δ`) to `web/templates/analyzer_results.html`, guarded by `{{if .SlipFactors}}`.
- Updated `cmd/eval/main_test.go` and `internal/web/eval_results_test.go` to verify slip-factor rendering when present and clean omission when empty.

## Deviations

- The design and task prompt referenced `web/templates/analyzer-results.html`; the actual template path in the repository is `web/templates/analyzer_results.html`.

## Open issues

None.

## Next-phase notes

- None.

## Commit

Surface slip-level factor attribution in CLI and UI

Expose parlay-level rule attribution (`SlipFactors`) in `cmd/eval` text/markdown reports and on the Grading Results page so users and rubric authors can inspect how parlay-level adjustments correlate with settled slip outcomes.
