# Phase 5: Phase 5: Extensible context/injury overlays and four-pillar v2 grading engine

## Done

### Task 13: Implement the pluggable `ContextOverlay` layer in `internal/analyzer/overlays.go` retaining all existing non-box-score rules (teammate-out bumps, injury status, nfelo QB drag, wind/fake weather, key numbers, shrinking role, TD spike), adding `narrative_streak_trap`, `model_market_divergence`, and structured `ContextSignal` ingestion, with `HeightenedUncertainty` blending tests.

- Added `ContextSignals []ContextSignal` to `analyzer.Game` and `analyzer.PlayerStats` in `internal/analyzer/analyzer.go` (`json:"context_signals,omitempty"`) and extracted shared `evalTDSpikeRisk` in `internal/analyzer/rules.go` so v1 rules and v2 overlays stay in sync.
- Implemented the pluggable `ContextOverlay` layer in `internal/analyzer/overlays.go` (`ContextSignal`, `OverlayFinding`, `ContextOverlay`, `DefaultContextOverlays`, `EvaluateOverlays`, `HasHeightenedUncertainty`, `UncertaintyBoost`, and `PredictLegWithOverlays`).
- Retained all non-box-score rules as deterministic overlays (`player_injury_status`, `teammate_out_bump`, `qb_drag`, `wind`, `fake_weather`, `hook`, `total_key_number`, `shrinking_role`, `td_spike_risk`) and added `narrative_streak_trap`, `model_market_divergence` (`>= 0.08` gap), and `external_context_signals` (deduplicating matching game and player signals).
- Added unit tests in `internal/analyzer/overlays_test.go` covering every retained and new overlay, custom overlay pluggability, sharp-friend tone checks, and `HeightenedUncertainty` (+0.15 market-blend shift capped at 0.95) across `modelv2.LoadEmbedded()` predictions.

### Task 14: Implement `EngineV2` in `internal/analyzer/grade_v2.go` behind the `GradingEngine` registry alongside `EngineV1`, computing the four decomposed `PillarScore` values (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`), calibrated `CalibratedProb` / `CorrMultiplier`, approximate sharp-friend plain-English reasons, and backwards-compatible `GradeFactor` entries, with full unit tests and highlight/explorer benchmarks.

- Added `PillarScore`, v2 probability/uncertainty fields on `LegGrade` and `SlipGrade`, and the `GradingEngine` interface + registry (`EngineV1`, `EngineV2`, `ActiveEngine = EngineV1`, `EngineByName`, `EngineNames`, `BuiltInEngines`) in `internal/analyzer/grade.go`, keeping `ScoreLeg`, `ScoreSlip`, and `EvaluateSlip` on `EngineV1` until Phase 7.
- Implemented `EngineV2` (`V2Engine`, `V2Config`, `DefaultV2Config`, `NewEngineV2`, `NewEngineV2WithBundle`) in `internal/analyzer/grade_v2.go`, extracting stack-allocated `modelv2` feature slices, scoring props via `PredictLegWithOverlays` and game lines via Power-devigged sharp + `nfelo` / `MarketScript` anchors, and computing multi-leg joint probabilities and `CorrMultiplier` via `CopulaArtifact.EstimateJoint`.
- Decomposed leg and slip scoring into the four `PillarScore` components (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`) with SGP correlation tax detection, hard Danger/vig caps, sharp-friend plain-English reasons free of EV/ROI jargon, and backwards-compatible `GradeFactor` entries.
- Added comprehensive unit tests and `BenchmarkEngineV2ScoreLeg`, `BenchmarkEngineV2FullSlateHighlights` (~5.44 ms/slate on M3 Max across 16 games / 1,632 legs + copula evaluation), and `BenchmarkEngineV2Explorer` (~0.16 ms/op) in `internal/analyzer/grade_v2_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

### Task 13

- Task 14 (`internal/analyzer/grade_v2.go`) can call `EvaluateOverlays` directly for game-line legs (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`) and `PredictLegWithOverlays` for prop legs using `modelv2.LoadEmbedded()`, passing `HasHeightenedUncertainty(findings)` to `LegGrade.Uncertain`.

### Task 14

- Phase 6 (`internal/eval`) can iterate `analyzer.BuiltInEngines()` to evaluate `v1` and `v2` side-by-side and read `LegGrade.CalibratedProb`, `LegGrade.MarketProb`, `SlipGrade.CalibratedProb`, and `SlipGrade.CorrMultiplier` directly for Log Loss, Brier, ECE, and SGP-vs-independence metrics.
- Phase 7 (`internal/web/analyzer.go` and templates) can render `Grade.Pillars` (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`), `Grade.Uncertain`, and `Grade.CorrMultiplier` when `len(Pillars) > 0`, falling back to `Factors` for `EngineV1`.

## Files touched

- `internal/analyzer/analyzer.go`
- `internal/analyzer/grade.go`
- `internal/analyzer/grade_v2.go`
- `internal/analyzer/grade_v2_test.go`
- `internal/analyzer/overlays.go`
- `internal/analyzer/overlays_test.go`
- `internal/analyzer/rules.go`
- `web/static/app.css`

## Run log

- Mode: 
- Checkpoints: task-13, task-14
- Verify: verify passed: generate changed: web/static/app.css, lint reused, test 8.4s; skipped: actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 0
