# Phase 4: Phase 4: Offline Python walk-forward training, calibration, and Go parity pipeline

## Done

### Task 10: Build the `uv`-managed Python pipeline under `ml/` (`data.py`, `features.py`, `train.py`, `copula.py`, `export.py`) with strict expanding-window walk-forward splits across ≥3 NFL seasons, KS covariate-shift checks, Spearman > 0.7 collinearity pruning, distributional GBDT + Optuna HPO, held-out isotonic/Platt calibration, market blend weight tuning, and PSD copula estimation, plus `pytest` leakage and stability tests.

- Created the `uv`-managed Python training workspace under `ml/` (`pyproject.toml`, `.python-version` pinned to `3.12`, `uv.lock`) and excluded `ml/` and its caches/venv from `.dockerignore` and `.gitignore`.
- Implemented `ml/data.py` to ingest, normalize, cache (Parquet), and join `nflreadpy` player stats, team stats, snap counts, players, fail-soft `ff_opportunity`, and `nfldata` schedules (`home_spread = -spread_line`).
- Implemented `ml/features.py` and `ml/distributions.py` with strictly pre-kickoff point-in-time EWMA/role-archetype/defense features, Weeks 1–5 Empirical Bayes shrinkage, KS two-sample covariate-shift checks, Spearman `> 0.7` collinearity pruning, exact distributional NLLs/Fisher Hessians/survival functions, and logit-space Platt / piecewise-linear Isotonic calibrators.
- Implemented `ml/copula.py`, `ml/train.py`, and `ml/export.py` providing expanding-window walk-forward training across ≥3 held-out NFL seasons, Optuna TPE HPO, market blend weight tuning in `[0.70, 0.85]`, Higham PSD copula estimation with exact Go-parity Joe & Kuo QMC Sobol evaluation, EWMA half-life stability checks, gate enforcement, and `v2.json` / `parity_vectors.json` / Markdown report export.
- Added offline `pytest` suites (`ml/tests/test_leakage.py`, `test_features.py`, `test_distributions.py`, `test_copula.py`, `test_pipeline_stability.py`), all 18 tests passing via `cd ml && uv run pytest -v` (7.29s).

### Task 11: Add `mise run train-v2` in `mise.toml`, export real trained artifacts to `internal/modelv2/artifacts/v2.json` and parity fixtures to `internal/modelv2/testdata/parity_vectors.json`, and add `internal/modelv2/parity_test.go` asserting Go inference and copula outputs match Python within `1e-6`.

- Added `[tasks.train-v2]` to `mise.toml` (`dir = "ml"`, `run = "uv run python export.py"`) and exported the 2019–2025 walk-forward trained bundle to `internal/modelv2/artifacts/v2.json` (`v2.1.0`, held-out `[2023, 2024, 2025]`, log loss `0.603842` vs. market `0.624456`, Brier `0.208705` vs. `0.216135`, SGP log loss `0.609369` vs. naive `0.640466`, `9/10` populated bins, stability MAE `0.009546`), along with `ml/reports/latest.md`.
- Exported 112 leg parity vectors (all 7 prop markets, Over/Under/Yes/No, Weeks 1–5 Empirical Bayes shrinkage, injected `NaN` features, uncertainty boost) and 6 multi-leg copula vectors to `internal/modelv2/testdata/parity_vectors.json`.
- Updated `ml/train.py` (`_sklearn_tree_to_flat`) and `ml/export.py` (`validate_artifact_bundle_dict`, `allow_nan=False`) so sklearn's `+/-inf` missing-vs-non-missing split thresholds export as finite sentinels (`+/-1e300`), plus a regression test in `ml/tests/test_pipeline_stability.py` (all 19 `pytest` tests passing).
- Added `internal/modelv2/parity_test.go` asserting validation gates on the embedded trained bundle and `1e-6` Go/Python parity across all 112 leg inference vectors (both pre-aligned and raw->shrunk->aligned) and all 6 copula vectors, and relaxed hardcoded synthetic-baseline constants in `internal/modelv2/gbdt_test.go` and `internal/modelv2/copula_test.go` to match real trained empirical bounds.

### Task 12: Docs: update `docs/guides/local-dev.md` with local `mise run train-v2` workflow, cadence, and PR verification steps.

- Updated `docs/guides/local-dev.md` with a `Train the v2 grading model` section covering `uv` prerequisites, `.dockerignore` exclusion from Cloud Run builds, `mise run train-v2` usage and flags, walk-forward validation gates, generated outputs (`internal/modelv2/artifacts/v2.json`, `internal/modelv2/testdata/parity_vectors.json`, `ml/reports/latest.md`), retraining cadence, and PR verification commands.
- Updated `docs/guides/local-dev.md` front matter (`summary`, `tags`, and `budget`) and regenerated `docs/llms.txt` via `good-vibes docs index`.
- Verified `docs/guides/local-dev.md` and `docs/llms.txt` pass `good-vibes docs lint` (`0` errors) and `typos`, with zero new `rumdl` warnings on the added section.

## Deviations

### Task 10

- `xgboost` and `lightgbm` pip wheels fail to load on macOS without Homebrew `libomp.dylib`, and `nflreadpy.load_schedules` currently returns HTTP 404 on `schedules/games.parquet`. Implemented `ml/train.py` with a 2nd-order Newton-Raphson `DistributionalGBDT` using `scikit-learn`'s `DecisionTreeRegressor` (with native `NaN` routing and exact analytical Fisher/Newton gradients) and loaded schedules from `nflverse/nfldata` `games.csv` with `nflreadpy.load_schedules` fallback.

## Open issues

None.

## Next-phase notes

### Task 10

- Task 11 can wire `mise run train-v2` to `cd ml && uv run python export.py --seasons 2019-2025` (or desired season range), generating `internal/modelv2/artifacts/v2.json`, `internal/modelv2/testdata/parity_vectors.json`, and `ml/reports/latest.md`.
- Note that `ml/data.py` caches downloaded Parquet files under `ml/cache/` (gitignored) and fails soft if `ff_opportunity` lacks a given season.
- `FlatTree` exports sklearn `<=` thresholds as `nextafter(threshold, +inf)` and evaluates using `<` semantics directly in Python, matching Go's `internal/modelv2/gbdt.go` (`val < Threshold -> Left`, `NaN -> Missing`) to machine precision.

### Task 11

- Phase 5 (`internal/analyzer/overlays.go`, `internal/analyzer/grade_v2.go`) can now load the real walk-forward trained artifact via `modelv2.LoadEmbedded()` and use `DistModelArtifact.PredictLeg` and `CopulaArtifact.EstimateJoint` directly.
- Note that `ml/train.py` clamps non-finite `sklearn` missing-vs-non-missing split thresholds (`+/-inf`) to `+/-1e300` so `encoding/json` and `FlatTree.Validate` accept them while preserving identical missing/non-missing routing.

### Task 12

- Phase 5 (`internal/analyzer/overlays.go`, `internal/analyzer/grade_v2.go`) can build directly on `internal/modelv2` and the training/verification workflow documented in `docs/guides/local-dev.md`.

## Files touched

- `.dockerignore`
- `.gitignore`
- `docs/guides/local-dev.md`
- `docs/llms.txt`
- `internal/modelv2/artifacts/v2.json`
- `internal/modelv2/copula_test.go`
- `internal/modelv2/gbdt_test.go`
- `internal/modelv2/parity_test.go`
- `internal/modelv2/testdata/parity_vectors.json`
- `mise.toml`
- `ml/.python-version`
- `ml/__init__.py`
- `ml/copula.py`
- `ml/data.py`
- `ml/distributions.py`
- `ml/export.py`
- `ml/features.py`
- `ml/pyproject.toml`
- `ml/reports/latest.md`
- `ml/tests/conftest.py`
- `ml/tests/test_copula.py`
- `ml/tests/test_distributions.py`
- `ml/tests/test_features.py`
- `ml/tests/test_leakage.py`
- `ml/tests/test_pipeline_stability.py`
- `ml/train.py`
- `ml/uv.lock`

## Run log

- Mode: 
- Checkpoints: task-10, task-11, task-12
- Verify: verify passed: test 1.3s; skipped: generate (no matching files), lint (no matching files), actionlint (no matching files), deploy-blackout (no matching files)
- Rounds used: 1 of 2; verify failures: 6
