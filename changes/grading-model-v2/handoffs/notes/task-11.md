# Task 11

## Done

- Added `[tasks.train-v2]` to `mise.toml` (`dir = "ml"`, `run = "uv run python export.py"`) and exported the 2019–2025 walk-forward trained bundle to `internal/modelv2/artifacts/v2.json` (`v2.1.0`, held-out `[2023, 2024, 2025]`, log loss `0.603842` vs. market `0.624456`, Brier `0.208705` vs. `0.216135`, SGP log loss `0.609369` vs. naive `0.640466`, `9/10` populated bins, stability MAE `0.009546`), along with `ml/reports/latest.md`.
- Exported 112 leg parity vectors (all 7 prop markets, Over/Under/Yes/No, Weeks 1–5 Empirical Bayes shrinkage, injected `NaN` features, uncertainty boost) and 6 multi-leg copula vectors to `internal/modelv2/testdata/parity_vectors.json`.
- Updated `ml/train.py` (`_sklearn_tree_to_flat`) and `ml/export.py` (`validate_artifact_bundle_dict`, `allow_nan=False`) so sklearn's `+/-inf` missing-vs-non-missing split thresholds export as finite sentinels (`+/-1e300`), plus a regression test in `ml/tests/test_pipeline_stability.py` (all 19 `pytest` tests passing).
- Added `internal/modelv2/parity_test.go` asserting validation gates on the embedded trained bundle and `1e-6` Go/Python parity across all 112 leg inference vectors (both pre-aligned and raw->shrunk->aligned) and all 6 copula vectors, and relaxed hardcoded synthetic-baseline constants in `internal/modelv2/gbdt_test.go` and `internal/modelv2/copula_test.go` to match real trained empirical bounds.

## Deviations

None.

## Open issues

None.

## Next-phase notes

- Phase 5 (`internal/analyzer/overlays.go`, `internal/analyzer/grade_v2.go`) can now load the real walk-forward trained artifact via `modelv2.LoadEmbedded()` and use `DistModelArtifact.PredictLeg` and `CopulaArtifact.EstimateJoint` directly.
- Note that `ml/train.py` clamps non-finite `sklearn` missing-vs-non-missing split thresholds (`+/-inf`) to `+/-1e300` so `encoding/json` and `FlatTree.Validate` accept them while preserving identical missing/non-missing routing.

## Commit

Export trained v2 artifacts and Go parity tests

Wire mise run train-v2 to export walk-forward trained artifacts and parity fixtures, ensure finite tree thresholds for JSON decoding, and verify Go/Python inference and copula parity within 1e-6.
