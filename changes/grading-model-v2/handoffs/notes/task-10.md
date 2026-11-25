# Task 10

## Done

- Created the `uv`-managed Python training workspace under `ml/` (`pyproject.toml`, `.python-version` pinned to `3.12`, `uv.lock`) and excluded `ml/` and its caches/venv from `.dockerignore` and `.gitignore`.
- Implemented `ml/data.py` to ingest, normalize, cache (Parquet), and join `nflreadpy` player stats, team stats, snap counts, players, fail-soft `ff_opportunity`, and `nfldata` schedules (`home_spread = -spread_line`).
- Implemented `ml/features.py` and `ml/distributions.py` with strictly pre-kickoff point-in-time EWMA/role-archetype/defense features, Weeks 1–5 Empirical Bayes shrinkage, KS two-sample covariate-shift checks, Spearman `> 0.7` collinearity pruning, exact distributional NLLs/Fisher Hessians/survival functions, and logit-space Platt / piecewise-linear Isotonic calibrators.
- Implemented `ml/copula.py`, `ml/train.py`, and `ml/export.py` providing expanding-window walk-forward training across ≥3 held-out NFL seasons, Optuna TPE HPO, market blend weight tuning in `[0.70, 0.85]`, Higham PSD copula estimation with exact Go-parity Joe & Kuo QMC Sobol evaluation, EWMA half-life stability checks, gate enforcement, and `v2.json` / `parity_vectors.json` / Markdown report export.
- Added offline `pytest` suites (`ml/tests/test_leakage.py`, `test_features.py`, `test_distributions.py`, `test_copula.py`, `test_pipeline_stability.py`), all 18 tests passing via `cd ml && uv run pytest -v` (7.29s).

## Deviations

- `xgboost` and `lightgbm` pip wheels fail to load on macOS without Homebrew `libomp.dylib`, and `nflreadpy.load_schedules` currently returns HTTP 404 on `schedules/games.parquet`. Implemented `ml/train.py` with a 2nd-order Newton-Raphson `DistributionalGBDT` using `scikit-learn`'s `DecisionTreeRegressor` (with native `NaN` routing and exact analytical Fisher/Newton gradients) and loaded schedules from `nflverse/nfldata` `games.csv` with `nflreadpy.load_schedules` fallback.

## Open issues

None.

## Next-phase notes

- Task 11 can wire `mise run train-v2` to `cd ml && uv run python export.py --seasons 2019-2025` (or desired season range), generating `internal/modelv2/artifacts/v2.json`, `internal/modelv2/testdata/parity_vectors.json`, and `ml/reports/latest.md`.
- Note that `ml/data.py` caches downloaded Parquet files under `ml/cache/` (gitignored) and fails soft if `ff_opportunity` lacks a given season.
- `FlatTree` exports sklearn `<=` thresholds as `nextafter(threshold, +inf)` and evaluates using `<` semantics directly in Python, matching Go's `internal/modelv2/gbdt.go` (`val < Threshold -> Left`, `NaN -> Missing`) to machine precision.

## Commit

Add uv-managed offline ML training and copula pipeline

Implement leak-free walk-forward distributional GBDT training, Optuna HPO, calibration, PSD copula estimation, and artifact export under ml/ with pytest leakage and stability suites.
