"""End-to-end walk-forward pipeline, stability perturbation, and export tests."""

from __future__ import annotations

import gzip
import json
import math
from pathlib import Path

import numpy as np
import polars as pl
from sklearn.tree import DecisionTreeRegressor

from export import export_pipeline_outputs, validate_artifact_bundle_dict
from train import _sklearn_tree_to_flat, run_walk_forward_training


def test_walk_forward_pipeline_and_export(
    synthetic_nfl_data: tuple[pl.DataFrame, pl.DataFrame],
    tmp_path: Path,
) -> None:
    player_df, team_df = synthetic_nfl_data
    result = run_walk_forward_training(
        player_df,
        team_df,
        seasons=[2021, 2022, 2023, 2024],
        n_trials=2,
        min_held_out_seasons=3,
        seed=42,
        enforce_gates=True,
    )

    assert result.held_out_seasons == [2022, 2023, 2024]
    val = result.validation
    assert val["populated_bins"] >= 8
    assert val["log_loss"] <= val["market_log_loss"] + 1e-4
    assert val["brier_score"] <= val["market_brier_score"] + 1e-4
    assert val["sgp_log_loss"] < val["naive_indep_log_loss"]
    assert result.stability_mae <= 0.05

    artifact_out = tmp_path / "v2.json.gz"
    parity_out = tmp_path / "parity_vectors.json"
    report_out = tmp_path / "latest.md"

    bundle = export_pipeline_outputs(
        result,
        artifact_path=artifact_out,
        parity_path=parity_out,
        report_path=report_out,
        version="v2.0.0-test",
    )
    validate_artifact_bundle_dict(bundle)

    exported = json.loads(gzip.decompress(artifact_out.read_bytes()))
    assert exported["version"] == "v2.0.0-test"
    assert exported["feature_schema_version"] == bundle["feature_schema_version"]

    parity = json.loads(parity_out.read_text(encoding="utf-8"))
    assert len(parity["leg_vectors"]) >= 100
    assert len(parity["copula_vectors"]) >= 5
    assert report_out.exists()


def test_sklearn_missing_only_split_produces_finite_threshold() -> None:
    """When sklearn splits purely on NaN vs constant non-NaN, exported threshold must be finite."""
    x = np.array([[1.0], [1.0], [1.0], [np.nan], [np.nan], [np.nan]], dtype=np.float64)
    y = np.array([0.0, 0.0, 0.0, 1.0, 1.0, 1.0], dtype=np.float64)
    dt = DecisionTreeRegressor(max_depth=2, min_samples_leaf=1, random_state=42)
    dt.fit(x, y)

    leaf_ids = dt.apply(x)
    leaf_values = {int(lid): float(np.mean(y[leaf_ids == lid])) for lid in np.unique(leaf_ids)}
    flat = _sklearn_tree_to_flat(dt, np.array([0], dtype=np.int64), leaf_values)

    for node in flat.nodes:
        assert math.isfinite(node["threshold"])
        assert math.isfinite(node["leaf"])
    assert math.isclose(flat.eval_scalar([1.0]), 0.0, abs_tol=1e-12)
    assert math.isclose(flat.eval_scalar([float("nan")]), 1.0, abs_tol=1e-12)
    json.dumps(flat.to_dict(), allow_nan=False)
