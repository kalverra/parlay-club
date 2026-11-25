# Phase 7: Phase 7: Phone-first four-pillar UI, living docs, and v2 promotion

## Done

### Task 17: Update `internal/web/analyzer.go`, `web/templates/partials.html`, and `web/templates/analyzer_guide.html` to render the four-pillar breakdown, approximate sharp-friend "Why" bullets, uncertainty badge, and updated v2 guide documentation cleanly at 375px phone width (falling back cleanly to factor lists for v1), with template and handler tests.

- Extended `GradeView` in `internal/web/analyzer.go` with `Uncertain`, `CorrSummary`, `Pillars` (`PillarView` with tonal badge/bar classes), and up to 3 plain-English `Why` bullets ranked from the strongest pillar drivers, plus exported `LegGradeView`/`SlipGradeView` helpers and optional `Dependencies.GradingEngine` injection in `internal/web/routes.go`, `internal/web/slips.go`, and `internal/web/pick.go` (including highlight legs and parlays).
- Extracted a shared `grade-breakdown` template in `web/templates/partials.html` used across slip panel legs, slip panel banner, pick tray legs, pick tray banner, and highlight leg/parlay cards, rendering the four-pillar meters, `"High uncertainty"` badge, `CorrSummary`, and `"Why"` bullets cleanly within 375px phone width while falling back to v1 factor lists when `Pillars` is empty.
- Updated `AnalyzerGuidePage` and `web/templates/analyzer_guide.html` to document the v2 four-pillar scoring model, Power de-vigging, Gaussian copula script multiplier, SGP correlation tax, `"High uncertainty"` badge, and v2 leg/slip weights while keeping A–F bands, warning rules, and v1 factor tables intact.
- Regenerated `web/static/app.css` via `go generate ./...` and added template rendering and HTTP handler tests (`TestGradeBreakdownsRenderFourPillarsAndV1FallbackTemplates` and `TestSlipPanelTrayAndGuideWithEngineV2Handler`) in `internal/web/analyzer_test.go`.

### Task 18: Run `mise run eval-prod` (and CI benchmark eval) to confirm `EngineV2` meets calibration, Log Loss, ECE, and SGP gates against `EngineV1`, then switch `ActiveEngine = EngineV2` in `internal/analyzer` and update any snapshot/golden tests.

- Verified the CI benchmark eval (`go run ./cmd/eval --dataset internal/eval/testdata/benchmark_dataset.json --all --format markdown`) on the 2025 NFL Weeks 7–10 replay dataset (236 legs across 70 SGPs), confirming `EngineV2` clears calibration and monotonicity gates against `EngineV1` and the market baseline (`Log Loss 0.5379` vs. `Market 0.6008`, `Brier 0.1838` vs. `0.2059`, `Classwise ECE 0.0319`, `9/10` populated bins, `SGP LL 0.4062` vs. `Naive 0.4063`, `Monotonic: true`).
- Switched `ActiveEngine = EngineV2` in `internal/analyzer/grade.go` and updated `TestGradingEngineRegistryAndV1Compatibility` in `internal/analyzer/grade_v2_test.go`.
- Preserved v1 rubric unit coverage by routing explicit v1 additive/factor tests in `internal/analyzer/analyzer_test.go`, `internal/analyzer/nflverse_grade_test.go`, `internal/analyzer/rubric_test.go`, and `internal/eval/replay_test.go` through `analyzer.EngineV1`.
- Enriched `internal/board/highlights_test.go` and `seedAGradeBoardForHubTest` in `internal/web/slips_test.go` with `EWMA`, `MarketScript`, and `Pinnacle` sharp benchmarks so board highlights run end-to-end under `EngineV2`, and updated `cmd/eval/main_test.go`, `internal/web/analyzer_test.go`, `internal/web/eval_results_test.go`, and `internal/web/pick_test.go` for `ActiveEngine = EngineV2`.
- Added build-tag guards (`internal/analyzer/grade_v2_race_test.go`, `internal/analyzer/grade_v2_norace_test.go`) on `TestEngineV2FullSlateLatencyBudget` in `internal/analyzer/grade_v2_test.go` so normal builds enforce the `<100ms` latency ceiling while `-race` full-suite runs still verify race-freedom without flaking on wall-clock load.

### Task 19: Docs: update `docs/concepts/analyzer.md` and `docs/reference/analyzer-rules.md` to describe the v2 calibrated four-pillar architecture, copula script scoring, and extensible context/injury overlays, and run `good-vibes docs lint`.

- Rewrote `docs/concepts/analyzer.md` to document the promoted `EngineV2` architecture, phone-first four-pillar breakdown and `"High uncertainty"` badge, `internal/modelv2` distributional prop models with Weeks 1–5 Empirical Bayes shrinkage and Power-devigged market blending, deterministic 8,192-sample QMC Gaussian copula script scoring, extensible `ContextOverlay` checks, `EngineV2` vs. `EngineV1` offline evals, and updated overview Mermaid diagram.
- Updated `docs/reference/analyzer-rules.md` with `DefaultV2Config` pillar weights and score caps, `internal/modelv2` distribution families and calibration pipeline, `CopulaArtifact.EstimateJoint` regime matrices and SGP correlation tax rules, all 12 built-in `DefaultContextOverlays` entries (including `narrative_streak_trap`, `model_market_divergence`, and `external_context_signals`), and a clearly scoped `EngineV1` / `RubricV1` reference.
- Updated `docs/reference/routes.md` for `/analyzer/guide` and `?engine=` (with `?rubric=` fallback alias) and probability calibration metrics on `/analyzer/results`, then regenerated `docs/llms.txt` via `good-vibes docs index` and verified `good-vibes docs lint` passes with exit code 0 and zero findings on modified docs.

## Deviations

### Task 17

- Added an optional `GradingEngine` field to `web.Dependencies` (wiring it in `slips.go`, `pick.go`, and `analyzer.go`) and updated `highlightsView`/`legHighlightView` in `slips.go` to use `SlipGradeView`/`LegGradeView` so v2 pillars flow into highlight cards and handler tests can exercise `EngineV2` in parallel without mutating global `analyzer.ActiveEngine`.

### Task 18

- Skipped `mise run eval-prod` because Google Application Default Credentials are not configured in this environment (`gcloud auth application-default print-access-token` fails without ADC, blocking `litestream restore` for `gs://parlay-club-parlay-litestream`); the user approved proceeding on the 2023–2025 walk-forward report (`ml/reports/latest.md`) and real-data CI benchmark instead.
- Replaced `internal/eval/testdata/benchmark_dataset.json` with real 2025 NFL Weeks 7–10 data (user approved replacing the synthetic v1 fixture rather than gating on its synthetic 95% hit rates).

### Task 19

- Also updated `docs/reference/routes.md` and regenerated `docs/llms.txt` alongside the two docs named in the task, following the Phase 6/7 handoff notes and `gv-docs` map sync rule so `?engine=` on `GET /analyzer/results` and updated doc summaries stay aligned.

## Open issues

### Task 17

- When Task 18 promotes `analyzer.ActiveEngine = analyzer.EngineV2`, `TestAnalyzerGuideRendersAllRulesAndAnchors` (`Active engine: v1`) and `TestSlipPanelTrayAndGuideRenderSharpFactors` (which expects v1 `Sharp line edge` / `+8` factor rows on panel and tray) in `internal/web/analyzer_test.go` will need updating for v2 outputs.

### Task 18

- Run `gcloud auth application-default login && LITESTREAM_BUCKET=parlay-club-parlay-litestream mise run eval-prod` from an authenticated shell whenever you want an extra calibration double-check against live group slips.

## Next-phase notes

### Task 17

- `web_test` already passes with `EngineV2` wired via `Dependencies.GradingEngine`; once `eval-prod` clears gates and `ActiveEngine` switches to `EngineV2`, check `TestAnalyzerGuideRendersAllRulesAndAnchors` and `TestSlipPanelTrayAndGuideRenderSharpFactors` alongside any other `ActiveEngine == EngineV1` assertions.

### Task 18

- Living docs (`docs/concepts/analyzer.md`, `docs/reference/analyzer-rules.md`, `docs/reference/routes.md`, etc.) still need updating in the remaining Phase 7 docs task to reflect `ActiveEngine = EngineV2`, the four-pillar UI, and `?engine=` on `/analyzer/results`.

### Task 19

- Phase 7 is complete and `good-vibes docs lint` passes with exit code 0 across all living docs.

## Files touched

- `cmd/eval/main_test.go`
- `docs/concepts/analyzer.md`
- `docs/llms.txt`
- `docs/reference/analyzer-rules.md`
- `docs/reference/routes.md`
- `internal/analyzer/analyzer_test.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/grade_v2_norace_test.go`
- `internal/analyzer/grade_v2_race_test.go`
- `internal/analyzer/grade_v2_test.go`
- `internal/analyzer/nflverse_grade_test.go`
- `internal/analyzer/rubric_test.go`
- `internal/board/highlights_test.go`
- `internal/eval/replay_test.go`
- `internal/eval/testdata/benchmark_dataset.json`
- `internal/web/analyzer.go`
- `internal/web/analyzer_test.go`
- `internal/web/eval_results_test.go`
- `internal/web/pick.go`
- `internal/web/pick_test.go`
- `internal/web/routes.go`
- `internal/web/slips.go`
- `internal/web/slips_test.go`
- `web/static/app.css`
- `web/templates/analyzer_guide.html`
- `web/templates/partials.html`

## Run log

- Mode: 
- Checkpoints: task-17, task-18, task-19
- Verify: verify passed: test 1.3s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
