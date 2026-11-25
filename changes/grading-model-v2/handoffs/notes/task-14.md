# Task 14

## Done

- Added `PillarScore`, v2 probability/uncertainty fields on `LegGrade` and `SlipGrade`, and the `GradingEngine` interface + registry (`EngineV1`, `EngineV2`, `ActiveEngine = EngineV1`, `EngineByName`, `EngineNames`, `BuiltInEngines`) in `internal/analyzer/grade.go`, keeping `ScoreLeg`, `ScoreSlip`, and `EvaluateSlip` on `EngineV1` until Phase 7.
- Implemented `EngineV2` (`V2Engine`, `V2Config`, `DefaultV2Config`, `NewEngineV2`, `NewEngineV2WithBundle`) in `internal/analyzer/grade_v2.go`, extracting stack-allocated `modelv2` feature slices, scoring props via `PredictLegWithOverlays` and game lines via Power-devigged sharp + `nfelo` / `MarketScript` anchors, and computing multi-leg joint probabilities and `CorrMultiplier` via `CopulaArtifact.EstimateJoint`.
- Decomposed leg and slip scoring into the four `PillarScore` components (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`) with SGP correlation tax detection, hard Danger/vig caps, sharp-friend plain-English reasons free of EV/ROI jargon, and backwards-compatible `GradeFactor` entries.
- Added comprehensive unit tests and `BenchmarkEngineV2ScoreLeg`, `BenchmarkEngineV2FullSlateHighlights` (~5.44 ms/slate on M3 Max across 16 games / 1,632 legs + copula evaluation), and `BenchmarkEngineV2Explorer` (~0.16 ms/op) in `internal/analyzer/grade_v2_test.go`.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 6 (`internal/eval`) can iterate `analyzer.BuiltInEngines()` to evaluate `v1` and `v2` side-by-side and read `LegGrade.CalibratedProb`, `LegGrade.MarketProb`, `SlipGrade.CalibratedProb`, and `SlipGrade.CorrMultiplier` directly for Log Loss, Brier, ECE, and SGP-vs-independence metrics.
- Phase 7 (`internal/web/analyzer.go` and templates) can render `Grade.Pillars` (`Price & Vig`, `Script Fit`, `Stability & Tail Risk`, `Context & Traps`), `Grade.Uncertain`, and `Grade.CorrMultiplier` when `len(Pillars) > 0`, falling back to `Factors` for `EngineV1`.

## Commit

Implement four-pillar EngineV2 behind GradingEngine registry

Add the calibrated four-pillar v2 grading engine alongside EngineV1 while keeping ActiveEngine set to v1 until offline and replay calibration gates pass. This gives eval and web phases a single versioned seam for decomposed leg/slip explanations, QMC copula script correlation, and pluggable context overlays without breaking existing v1 callers.
