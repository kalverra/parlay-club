# Task 13

## Done

- Added `ContextSignals []ContextSignal` to `analyzer.Game` and `analyzer.PlayerStats` in `internal/analyzer/analyzer.go` (`json:"context_signals,omitempty"`) and extracted shared `evalTDSpikeRisk` in `internal/analyzer/rules.go` so v1 rules and v2 overlays stay in sync.
- Implemented the pluggable `ContextOverlay` layer in `internal/analyzer/overlays.go` (`ContextSignal`, `OverlayFinding`, `ContextOverlay`, `DefaultContextOverlays`, `EvaluateOverlays`, `HasHeightenedUncertainty`, `UncertaintyBoost`, and `PredictLegWithOverlays`).
- Retained all non-box-score rules as deterministic overlays (`player_injury_status`, `teammate_out_bump`, `qb_drag`, `wind`, `fake_weather`, `hook`, `total_key_number`, `shrinking_role`, `td_spike_risk`) and added `narrative_streak_trap`, `model_market_divergence` (`>= 0.08` gap), and `external_context_signals` (deduplicating matching game and player signals).
- Added unit tests in `internal/analyzer/overlays_test.go` covering every retained and new overlay, custom overlay pluggability, sharp-friend tone checks, and `HeightenedUncertainty` (+0.15 market-blend shift capped at 0.95) across `modelv2.LoadEmbedded()` predictions.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Task 14 (`internal/analyzer/grade_v2.go`) can call `EvaluateOverlays` directly for game-line legs (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`) and `PredictLegWithOverlays` for prop legs using `modelv2.LoadEmbedded()`, passing `HasHeightenedUncertainty(findings)` to `LegGrade.Uncertain`.

## Commit

Add pluggable ContextOverlay layer for analyzer v2

Introduce a deterministic ContextOverlay seam so v2 grading retains non-box-score injury, weather, nfelo, key-number, role, and TD-regression intelligence alongside narrative streak traps, model-market divergence flags, and structured ContextSignal events without retraining model artifacts.
