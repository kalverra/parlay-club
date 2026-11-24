---
slug: deepen-eval-fidelity
run: 20261124-001953-deepen-eval-fidelity
outcome: done
generated: 2026-11-24T00:35:09Z
---

# Run summary: deepen-eval-fidelity

## Outcome

- Status: done
- Branch: deepen-eval-fidelity
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 4 of 4
- Phases: 3
- Checkpoints: 5
- Duration: 11m27s
- Cost: $—
- Fresh input tokens: 135140
- Cache read tokens: 885308
- Cache write tokens: 0
- Output tokens: 8139

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 1028587 | $— |

## Phases

### Phase 1: Preserve full pre-kickoff slips in replay and filter unsettled legs during leg aggregation

Tasks:

- [x] Update `loadWeekSlips` in `internal/eval/replay.go` to keep all pre-kickoff legs on `SlipGraded` slips (computing `CombinedOdds` only from counting settled legs), and update `Evaluate` in `internal/eval/metrics.go` to score the full slip while restricting leg-band, leg-factor, and outcome accumulation to settled legs, with replay and metrics tests for early-busted slips.

Phase totals: 1 checkpoints, 2m7s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-1 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Update `loadWeekSlips` in `internal/eval/replay.go` to keep all pre-kickoff legs on `SlipGraded` slips (computing `CombinedOdds` only from counting settled legs), and update `Evaluate` in `internal/eval/metrics.go` to score the full slip while restricting leg-band, leg-factor, and outcome accumulation to settled legs, with replay and metrics tests for early-busted slips.

- Updated `loadWeekSlips` in `internal/eval/replay.go` to retain all pre-kickoff legs (including `PENDING` and `NEEDS_GRADING`) on `SlipGraded` slips while computing `CombinedOdds` only from counting settled legs (`isSettledResult(dl.Result) && dl.Result.Counts()`).
- Updated `Evaluate` in `internal/eval/metrics.go` to pass all pre-kickoff legs to `analyzer.ScoreSlipWithRubric` while skipping unsettled legs during leg-band, leg-factor, and `legOutcomes` accumulation.
- Updated `slipDecimalOdds` in `internal/eval/metrics.go` to match `loadWeekSlips` by only including counting settled legs when deriving fallback slip decimal odds.
- Added `TestLoadDatasetEarlyBustedSlip` in `internal/eval/replay_test.go` and `TestEvaluateEarlyBustedSlip` in `internal/eval/metrics_test.go`.

### Phase 2: Track slip-level factor attribution and harden rubric comparison

Tasks:

- [x] Switch `rubricFactorLabels` in `internal/eval/metrics.go` to `analyzer.GradeRubricFor(rubric)`, add `SlipFactors []FactorStats` to `eval.Report` accumulated from `slipGrade.Factors` on decided slips, and key `legOutcome` by `(SlipID, LegID)` (with `(SlipID, index)` fallback for zero-ID fixtures) alongside `CompareRubrics`, with unit tests covering slip factors and mismatched/reordered comparisons.

Phase totals: 1 checkpoints, 2m39s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-2 Review fixes: 0 Integration checks: 0

#### Done

### Task 2: Switch `rubricFactorLabels` in `internal/eval/metrics.go` to `analyzer.GradeRubricFor(rubric)`, add `SlipFactors []FactorStats` to `eval.Report` accumulated from `slipGrade.Factors` on decided slips, and key `legOutcome` by `(SlipID, LegID)` (with `(SlipID, index)` fallback for zero-ID fixtures) alongside `CompareRubrics`, with unit tests covering slip factors and mismatched/reordered comparisons.

- Verified `rubricFactorLabels` in `internal/eval/metrics.go` uses `analyzer.GradeRubricFor(rubric)` so factor labels match the evaluated rubric.
- Added `SlipFactors []FactorStats` (`json:"slip_factors,omitempty"`) to `eval.Report` and accumulated `slipGrade.Factors` on decided slips (`parlay.Won` / `parlay.Lost`) using shared `recordFactors` and `finalizeFactorStats` helpers.
- Keyed `legOutcome` by `(SlipID, LegID)` with a `(SlipID, legIdx)` fallback when `LegID == 0`, and updated `computeMigrations` to join outcomes by identity key rather than slice index.
- Added `CompareRubrics(dataset ReplayDataset, baseline, candidate analyzer.Rubric) (Report, Report, DiffReport)` to `internal/eval/metrics.go`.
- Added unit tests in `internal/eval/metrics_test.go` covering `SlipFactors` attribution on decided vs. push slips, `CompareRubrics`, and reordered, subset, empty, and zero-ID migration comparisons.

### Phase 3: Surface slip-level factor attribution in CLI and Grading Results page

Tasks:

- [x] Render `SlipFactors` in `cmd/eval/main.go` (text and markdown outputs) and in `internal/web/eval_results.go` + `web/templates/analyzer-results.html`, updating `cmd/eval/main_test.go` and `internal/web/eval_results_test.go`.
- [x] Docs: update `docs/concepts/analyzer.md` to document slip-level factor attribution and full-slip replay behavior on `/analyzer/results` and `cmd/eval`, then run `good-vibes docs lint`.

Phase totals: 3 checkpoints, 6m4s, $—, 0 input / 0 output / 0 cache read / 0 cache write tokens

Checkpoints: task-3, task-4, final Review fixes: 0 Integration checks: 0

#### Done

### Task 3: Render `SlipFactors` in `cmd/eval/main.go` (text and markdown outputs) and in `internal/web/eval_results.go` + `web/templates/analyzer-results.html`, updating `cmd/eval/main_test.go` and `internal/web/eval_results_test.go`.

- Rendered `SlipFactors` in `cmd/eval/main.go` for both text (`Slip Factor Attribution:` / `Candidate Slip Factor Attribution`) and markdown (`#### Slip Factor Attribution` / `### Candidate Slip Factor Attribution`) outputs when non-empty, omitting the section cleanly when empty.
- Added `SlipFactors []EvalFactorView` to `EvalResultsPage` in `internal/web/eval_results.go` populated via `buildEvalFactors(report.SlipFactors)`.
- Added a secondary `Slip Factor Attribution` section (mobile cards and desktop table, with sample counts `N`, `W-L`, `Hit %`, `Lift`, and `Avg Δ`) to `web/templates/analyzer_results.html`, guarded by `{{if .SlipFactors}}`.
- Updated `cmd/eval/main_test.go` and `internal/web/eval_results_test.go` to verify slip-factor rendering when present and clean omission when empty.

### Task 4: Docs: update `docs/concepts/analyzer.md` to document slip-level factor attribution and full-slip replay behavior on `/analyzer/results` and `cmd/eval`, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` (`### Versioned rubrics and offline evals`) to document full-slip replay (retaining unsettled legs on early-busted graded slips for `ScoreSlipWithRubric` while counting only settled legs in leg metrics), `(SlipID, LegID)` migration matching / `CompareRubrics`, and slip-level factor attribution (`SlipFactors`, `GradeRubricFor`) in `cmd/eval`.
- Updated `docs/concepts/analyzer.md` (`## In-app grading guide and results`) to document the secondary **Slip Factor Attribution** section on `GET /analyzer/results`.
- Ran `good-vibes docs lint` and verified `docs/concepts/analyzer.md` passes with 0 errors.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none
