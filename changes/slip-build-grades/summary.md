---
slug: slip-build-grades
run: 20261128-140712-slip-build-grades
outcome: done
generated: 2026-11-28T14:56:42Z
---

# Run summary: slip-build-grades

## Outcome

- Status: done
- Branch: slip-build-grades
- Close-out: written
- Close: pending
- Final verdict: agy@gemini-4-neon-medium approved (issues: 0)

## Totals

- Tasks: 9 of 9
- Phases: 4
- Checkpoints: 10
- Duration: 46m36s
- Cost: $—
- Fresh input tokens: 3489018
- Cache read tokens: 56821147
- Cache write tokens: 0
- Output tokens: 563700

| Model | Tokens | Cost |
| --- | ---: | ---: |
| gemini-4-hydrogen-medium | 38934855 | $— |
| gemini-4-hydrogen-high | 17672631 | $— |
| gemini-4-neon-medium | 4266379 | $— |

## Phases

### Phase 1: Core build-grade diagnostics, provenance math, and Process vs. Outcome classifier

Tasks:

- [x] Add human-first implied probability helpers (`FormatImpliedReality` / `1 in X`) in `internal/parlay` and propagate unblended `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `BuildDiagnostics` (2-word descriptor, max 3 `SignalPills`, max 2 `ContextRadar` notes tagged priced-in, and optional 1-line `CleanupNudge`) onto `LegGrade` and `SlipGrade` in `internal/analyzer`, with comprehensive unit tests.
- [x] Add `ClassifyProcessOutcome` (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) in `internal/analyzer` with tests covering all 4 matrix states and ungraded/push edge cases.
- [x] Docs: add `Build grade`, `Correlation tax`, and `Process vs. Outcome` terms to `GLOSSARY.md`.

Phase totals: 3 checkpoints, 10m1s, $—, 10305098 input / 119148 output / 9505652 cache read / 0 cache write tokens

Checkpoints: task-1, task-2, task-3 Review fixes: 0 Integration checks: 0

#### Done

### Task 1: Add human-first implied probability helpers (`FormatImpliedReality` / `1 in X`) in `internal/parlay` and propagate unblended `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `BuildDiagnostics` (2-word descriptor, max 3 `SignalPills`, max 2 `ContextRadar` notes tagged priced-in, and optional 1-line `CleanupNudge`) onto `LegGrade` and `SlipGrade` in `internal/analyzer`, with comprehensive unit tests.

- Added `FormatOneIn` and `FormatImpliedReality` in `internal/parlay/parlay.go` with unit tests in `internal/parlay/parlay_test.go` covering underdog/favorite ratios and `<1%` / `>99%` / invalid bounds.
- Extended `LegGrade` and `SlipGrade` in `internal/analyzer/grade.go` with `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `Diagnostics` (`BuildDiagnostics`).
- Implemented `internal/analyzer/diagnostics.go` with `SignalPill` (capped at 3, severity-ordered), `ContextNote` (`ContextRadar` capped at 2, tagged `"Already in the line"` or `"Likely Priced In"`), `DisplayGrade` (`+`/`-` band subdivisions), dominant-driver `Descriptor` mapping, `HoldPct`, `CorrTaxPct`, `CleanupNudge` (including heavy-chalk payout impact), and `ClassifyProcessOutcome`.
- Wired unblended `RawModelProb`, `VegasImpliedProb`, `FairNoVigProb`, and `BuildDiagnostics` into `EngineV2` in `internal/analyzer/grade_v2.go`, and added unit tests in `internal/analyzer/diagnostics_test.go`.

### Task 2: Add `ClassifyProcessOutcome` (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) in `internal/analyzer` with tests covering all 4 matrix states and ungraded/push edge cases.

- Verified `ClassifyProcessOutcome(letter string, won bool) ProcessOutcome` in `internal/analyzer/diagnostics.go` covering all 4 matrix cells (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) and ungraded/unknown letters.
- Added `ClassifyProcessResult(letter string, result parlay.Result) ProcessOutcome` in `internal/analyzer/diagnostics.go` so settled-slip callers can pass `parlay.Result` directly and get zero values for `Push`, `Void`, `Pending`, and `NeedsGrading`.
- Expanded `TestClassifyProcessOutcome` and added `TestClassifyProcessResult` in `internal/analyzer/diagnostics_test.go` covering all 4 matrix states across A–F grades (including `+`/`-`, lowercase, and whitespace), tone assertions, and ungraded/push/void/pending edge cases.

### Task 3: Docs: add `Build grade`, `Correlation tax`, and `Process vs. Outcome` terms to `GLOSSARY.md`.

- Added `Build grade`, `Correlation tax`, and `Process vs. Outcome` definitions to `GLOSSARY.md`.
- Verified `good-vibes docs lint GLOSSARY.md` passes with 0 errors and 0 warnings.

### Phase 2: Dual-track evals — Explaining Vegas vs. Beating Vegas

Tasks:

- [x] Add `MarketExplanationMetrics` (MAE, RMSE, KL divergence, Pearson r, within-5pp rate on `RawModelProb` vs. `MarketProb`) and `MarketBeatingMetrics` (raw vs. blended vs. market log loss, skill scores, ROI, and `AlphaGatePassed`) plus `SuiteReport.SlipGrades` in `internal/eval`, with unit tests verifying zero-error matching, missing-benchmark skipping, and alpha-gate thresholds.
- [x] Surface both "1. Explaining Vegas (Match First)" and "2. Beating Vegas (Alpha Gate)" scorecards in `cmd/eval` output and on `/docs/evals` (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`), with handler and render tests.

Phase totals: 2 checkpoints, 6m2s, $—, 2197107 input / 31241 output / 1976417 cache read / 0 cache write tokens

Checkpoints: task-4, task-5 Review fixes: 0 Integration checks: 0

#### Done

### Task 4: Add `MarketExplanationMetrics` (MAE, RMSE, KL divergence, Pearson r, within-5pp rate on `RawModelProb` vs. `MarketProb`) and `MarketBeatingMetrics` (raw vs. blended vs. market log loss, skill scores, ROI, and `AlphaGatePassed`) plus `SuiteReport.SlipGrades` in `internal/eval`, with unit tests verifying zero-error matching, missing-benchmark skipping, and alpha-gate thresholds.

- Added `MarketExplanationMetrics` and `MarketExplanation` on `eval.Report` in `internal/eval/metrics.go` to compute MAE, RMSE, Bernoulli KL divergence, Pearson correlation `r`, and within-5pp rate for `RawModelProb` vs. `MarketProb`, skipping legs without a de-vigged sharp benchmark or model probability.
- Added `MarketBeatingMetrics` and `MarketBeating` on `eval.Report` to compare `RawModelLogLoss`, `BlendedLogLoss`, and `MarketLogLoss`, compute `LogLossSkill`, `BrierSkill`, and `FlatROI`, and evaluate `AlphaGatePassed` (`SampleCount >= 250 && LogLossSkill > 0 && FlatROI > 0`).
- Added `slipOutcome` tracking to `evaluateWithEngine` and `SuiteReport.SlipGrades(engineName string)` with the same `ActiveRubric` and `Reports[0]` fallback behavior as `LegGrades`.
- Added unit tests in `internal/eval/metrics_test.go` covering zero-error matching, missing-benchmark skipping, alpha-gate sample/skill/ROI thresholds, and `SuiteReport.SlipGrades`.

### Task 5: Surface both "1. Explaining Vegas (Match First)" and "2. Beating Vegas (Alpha Gate)" scorecards in `cmd/eval` output and on `/docs/evals` (`internal/web/eval_results.go` and `web/templates/analyzer_results.html`), with handler and render tests.

- Added "1. Explaining Vegas (Match First)" (`writeMarketExplanationText`, `writeMarketExplanationMarkdown`) and "2. Beating Vegas (Alpha Gate)" (`writeMarketBeatingText`, `writeMarketBeatingMarkdown`) scorecards to `cmd/eval/main.go` text and markdown outputs (JSON already serializes `Report.MarketExplanation` and `Report.MarketBeating`).
- Added `EvalMarketExplanationView`, `EvalMarketBeatingView`, `buildEvalMarketExplanation`, and `buildEvalMarketBeating` to `internal/web/eval_results.go`.
- Rendered both dual-track scorecards in a phone-first responsive grid in `web/templates/analyzer_results.html` with `Alpha Gate: PASS` / `Alpha Gate: FAIL` status badges and skill-color indicators.
- Added CLI output tests in `cmd/eval/main_test.go` (`TestWriteReportDualTrackScorecards`) and handler/render tests in `internal/web/eval_results_test.go` covering `/docs/evals` and `analyzer-results` rendering.

### Phase 3: Semantic grade colors and 3-layer progressive disclosure UI

Tasks:

- [x] Replace green `.grade-a` tokens with cool/neutral build-grade tokens across all 4 themes in `web/tailwind/input.css` (reserving `--positive` strictly for settled wins), run `go generate ./...`, and verify no grade badge class maps to `--positive`.
- [x] Update `internal/web/analyzer.go` and templates (`partials.html`, `research_bet.html`, `analyzer_guide.html`) to render Layer 1 (Build Grade + 2-word descriptor, human-first `Vegas Implied` `1 in X` copy, max 3 clickable Signal Pills), Layer 2 (breakdown drawer with Ground-Rule Banner, separated Market Reality vs. Slip Diagnostics with provenance tags, Context Radar `Already in the line` pills, and optional 1-line cleanup nudge), and Layer 3 (inline live-slip "Why does this matter?" popovers + 4-bullet Model Card), plus reframed highlights strip, with full web handler and template tests.

Phase totals: 2 checkpoints, 15m6s, $—, 17526739 input / 145892 output / 16731592 cache read / 0 cache write tokens

Checkpoints: task-6, task-7 Review fixes: 0 Integration checks: 0

#### Done

### Task 6: Replace green `.grade-a` tokens with cool/neutral build-grade tokens across all 4 themes in `web/tailwind/input.css` (reserving `--positive` strictly for settled wins), run `go generate ./...`, and verify no grade badge class maps to `--positive`.

- Added `--grade-clean`, `--grade-clean-soft`, `--grade-clean-ink` (teal/cyan) and `--grade-solid`, `--grade-solid-soft`, `--grade-solid-ink` (slate/blue) tokens across all four themes (`paper-turf`, `redzone`, `almanac`, `connor-mode`) and exposed them in `@theme` in `web/tailwind/input.css`.
- Remapped `.grade-a` to `--grade-clean*` and `.grade-b` to `--grade-solid*`, keeping `.grade-c` on `--warning*`, `.grade-d`/`.grade-f` on `--danger*`, and `.grade-neutral` on `--line-strong`/`--surface-2`/`--ink-muted` so no grade badge class maps to `--positive`.
- Regenerated `web/static/app.css` via `go generate ./...`.
- Added `TestBuildGradeThemeTokensAndClasses` in `web/embed_test.go` to assert non-empty build-grade tokens in all four themes and verify that no `.grade-*` rule in `web/tailwind/input.css` or `web/static/app.css` references `--positive`.

### Task 7: Update `internal/web/analyzer.go` and templates (`partials.html`, `research_bet.html`, `analyzer_guide.html`) to render Layer 1 (Build Grade + 2-word descriptor, human-first `Vegas Implied` `1 in X` copy, max 3 clickable Signal Pills), Layer 2 (breakdown drawer with Ground-Rule Banner, separated Market Reality vs. Slip Diagnostics with provenance tags, Context Radar `Already in the line` pills, and optional 1-line cleanup nudge), and Layer 3 (inline live-slip "Why does this matter?" popovers + 4-bullet Model Card), plus reframed highlights strip, with full web handler and template tests.

- Extended `GradeView` and view builders in `internal/web/analyzer.go` with `DisplayGrade`, 2-word `Descriptor`, `GroundRule`, provenance-tagged probabilities (`VegasImplied`, `FairNoVig`, `RawModel`, `HitReality`), `HoldText`, `CorrTaxText`, `RealismLabel`/`RealismPct`, capped `SignalPills` (max 3), `ContextRadar` (max 2), `CleanupNudge`, live-slip `WhyPopovers`, and the 4-bullet `ModelCard`, while reframing `PayoutLabel` and `pillarToneClasses` off `--positive`.
- Added human-first `Vegas Implied` `1 in X` formatting across `internal/web/slips.go`, `internal/web/pick.go`, and `internal/web/research.go`, and updated `gameGradeBadgeClass` in `internal/web/pick.go` to use `.grade-a`–`.grade-f`.
- Added `build-grade-l1` and updated `grade-breakdown` in `web/templates/partials.html` (with drawer open/focus/close handlers in `web/static/app.js`) to render Layer 1 (Build Grade + descriptor, human-first `Vegas Implied` `1 in X` copy, max 3 clickable Signal Pills), Layer 2 (`.leg-sheet` drawer with Ground-Rule Banner, separated Market Reality vs. Slip Diagnostics, Hit-Rate Realism bar, `Already in the line` pills, and optional cleanup nudge), Layer 3 (inline live-slip "Why does this matter?" popovers + 4-bullet Model Card), and reframed highlights (`Cleanest builds on the board` / `Cleanest builds in this game`).
- Updated `web/templates/research_bet.html` and `web/templates/analyzer_guide.html` with 3-layer disclosure, provenance tags, and the 4-bullet Model Card, regenerated `web/static/app.css`, and updated/expanded tests in `internal/web/analyzer_test.go`, `internal/web/slips_test.go`, `internal/web/pick_test.go`, and `internal/web/research_test.go`.

### Phase 4: Post-game Process vs. Outcome badges and living docs

Tasks:

- [x] Render post-game Process vs. Outcome badges (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) on settled slip cards (`/slips/{id}`) and Record Book views (`/record` and `/record/members/{id}`), backed by `SuiteReport.SlipGrades` and unit/handler tests.
- [x] Docs: update `docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, and `docs/concepts/docs-hub.md` to document Build Grades, the 3-layer progressive disclosure UX, Process vs. Outcome badges, and the Explaining Vegas vs. Beating Vegas eval tracks, then run `good-vibes docs lint`.

Phase totals: 3 checkpoints, 14m42s, $—, 27832761 input / 253287 output / 26411628 cache read / 0 cache write tokens

Checkpoints: task-8, task-9, final Review fixes: 0 Integration checks: 0

#### Done

### Task 8: Render post-game Process vs. Outcome badges (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`) on settled slip cards (`/slips/{id}`) and Record Book views (`/record` and `/record/members/{id}`), backed by `SuiteReport.SlipGrades` and unit/handler tests.

- Added `ProcessOutcomeView` and `newProcessOutcomeView` in `internal/web/analyzer.go` mapping `analyzer.ClassifyProcessResult` tones (`positive`, `neutral`, `warn`, `bad`) to semantic stamp classes, reserving `--positive` for `Clean Hit` and settled wins.
- Wired `loadEvalGrades` (backed by `SuiteReport.SlipGrades`) into `slipPageData` (`internal/web/slips.go`) and updated `web/templates/partials.html` so settled `/slips/{id}` cards render the Process vs. Outcome stamp (`Clean Hit`, `Tough Beat`, `Total Bailout`, `Book Donation`), its summary, and result-aware `GRADED` status pills (`badge-lost`, `badge-push`, `badge-void`, or positive for `WON`).
- Wired `SuiteReport.SlipGrades` into `internal/web/record.go` (`RecordSlipView` on `RecordPage` and `SlipGrade`/`SlipProcessOutcome` on `RecordMemberBetView`) and rendered Build Grade and Process vs. Outcome badges in `web/templates/record.html` and `web/templates/record_member.html`.
- Added unit tests (`internal/web/record_unit_test.go`) and handler tests (`TestProcessOutcomeBadgesOnSettledSlipAndRecordViews` in `internal/web/record_test.go`) covering `/slips/{id}`, `/record`, and `/record/members/{id}`.

### Task 9: Docs: update `docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, and `docs/concepts/docs-hub.md` to document Build Grades, the 3-layer progressive disclosure UX, Process vs. Outcome badges, and the Explaining Vegas vs. Beating Vegas eval tracks, then run `good-vibes docs lint`.

- Updated `docs/concepts/analyzer.md` to document Build Grades, non-green grade color tokens (`.grade-a` / `--grade-clean` and `.grade-b` / `--grade-solid`), the 3-layer progressive disclosure UX (Glance Card, Breakdown Drawer, inline Why? popovers + Model Card), "Cleanest builds on the board" highlights, post-game Process vs. Outcome badges, and the Explaining Vegas vs. Beating Vegas eval tracks.
- Updated `docs/reference/analyzer-rules.md` with the `BuildDiagnostics` contract (subdivided `DisplayGrade`, 2-word `Descriptor`, probability provenance, Signal Pills, Context Radar, Cleanup Nudge, and `parlay.FormatImpliedReality`) and the `ClassifyProcessOutcome` / `ClassifyProcessResult` 2x2 matrix.
- Updated `docs/concepts/docs-hub.md` to document `/docs/grading` Build Grade ground rules, the two-track Explaining Vegas vs. Beating Vegas scorecards on `/docs/evals`, `SuiteReport.SlipGrades` in `eval.Cache`, and how Layer 2/3 progressive disclosure pairs with `?` help bubbles.
- Ran `good-vibes docs index` and `good-vibes docs lint`, passing with 0 errors.

## Final gate

- Rounds used: 1; verify failures: 0
- Final verify: passed
- Open issues: none

## Ship

- Sync: none
- Recipe: ask
