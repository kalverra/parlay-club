# Phase 3: Phase 3: Surface slip-level factor attribution in CLI and Grading Results page

## Done

### Task 3: Render `SlipFactors` in `cmd/eval/main.go` (text and markdown outputs) and in `internal/web/eval_results.go` + `web/templates/analyzer-results.html`, updating `cmd/eval/main_test.go` and `internal/web/eval_results_test.go`.

- Rendered `SlipFactors` in `cmd/eval/main.go` for both text (`Slip Factor Attribution:` / `Candidate Slip Factor Attribution`) and markdown (`#### Slip Factor Attribution` / `### Candidate Slip Factor Attribution`) outputs when non-empty, omitting the section cleanly when empty.
- Added `SlipFactors []EvalFactorView` to `EvalResultsPage` in `internal/web/eval_results.go` populated via `buildEvalFactors(report.SlipFactors)`.
- Added a secondary `Slip Factor Attribution` section (mobile cards and desktop table, with sample counts `N`, `W-L`, `Hit %`, `Lift`, and `Avg Δ`) to `web/templates/analyzer_results.html`, guarded by `{{if .SlipFactors}}`.
- Updated `cmd/eval/main_test.go` and `internal/web/eval_results_test.go` to verify slip-factor rendering when present and clean omission when empty.

### Task 4: Docs: update `docs/concepts/analyzer.md` to document slip-level factor attribution and full-slip replay behavior on `/analyzer/results` and `cmd/eval`, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` (`### Versioned rubrics and offline evals`) to document full-slip replay (retaining unsettled legs on early-busted graded slips for `ScoreSlipWithRubric` while counting only settled legs in leg metrics), `(SlipID, LegID)` migration matching / `CompareRubrics`, and slip-level factor attribution (`SlipFactors`, `GradeRubricFor`) in `cmd/eval`.
- Updated `docs/concepts/analyzer.md` (`## In-app grading guide and results`) to document the secondary **Slip Factor Attribution** section on `GET /analyzer/results`.
- Ran `good-vibes docs lint` and verified `docs/concepts/analyzer.md` passes with 0 errors.

## Deviations

### Task 3

- The design and task prompt referenced `web/templates/analyzer-results.html`; the actual template path in the repository is `web/templates/analyzer_results.html`.

## Open issues

None.

## Next-phase notes

### Task 3

- None.

## Files touched

- `cmd/eval/main.go`
- `cmd/eval/main_test.go`
- `docs/concepts/analyzer.md`
- `internal/web/eval_results.go`
- `internal/web/eval_results_test.go`
- `web/templates/analyzer_results.html`

## Run log

- Mode: 
- Checkpoints: task-3, task-4
- Verify: verify passed: test 500ms; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
