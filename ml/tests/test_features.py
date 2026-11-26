"""Unit tests for EWMA, EB shrinkage, KS covariate shift, and Spearman collinearity pruning."""

from __future__ import annotations

import numpy as np
import pandas as pd

from data import map_team, normalize_position
from features import (
    compute_ewma_desc,
    compute_ks_shift_report,
    empirical_bayes_shrink,
    prune_collinear_features,
)


def test_team_and_position_normalization() -> None:
    assert map_team("jac") == "JAX"
    assert map_team("LA") == "LAR"
    assert map_team("WAS") == "WSH"
    assert map_team("OAK") == "LV"
    assert map_team("KC") == "KC"
    assert normalize_position("fb") == "RB"
    assert normalize_position("hb") == "RB"
    assert normalize_position("wr") == "WR"


def test_compute_ewma_and_eb_shrinkage() -> None:
    assert compute_ewma_desc([]) == 0.0
    assert abs(compute_ewma_desc([100.0, 50.0], half_life=1.0) - (125.0 / 1.5)) < 1e-9

    assert empirical_bayes_shrink(float("nan"), 60.0, 2, 3.0) == 60.0
    assert empirical_bayes_shrink(90.0, 60.0, 0, 3.0) == 60.0
    assert abs(empirical_bayes_shrink(90.0, 60.0, 2, 3.0) - ((2 * 90 + 3 * 60) / 5.0)) < 1e-9
    assert empirical_bayes_shrink(90.0, 60.0, 5, 3.0) == 90.0


def test_spearman_collinearity_pruning() -> None:
    rng = np.random.default_rng(7)
    x1 = rng.normal(50.0, 15.0, size=300)
    # x2 is nearly monotone function of x1 (Spearman > 0.95); x3 is independent
    x2 = 2.0 * x1 + rng.normal(0.0, 0.5, size=300)
    x3 = rng.normal(10.0, 3.0, size=300)
    y = 0.8 * x1 + 0.3 * x3 + rng.normal(0.0, 5.0, size=300)

    df = pd.DataFrame({"f_primary": x1, "f_dup": x2, "f_indep": x3, "target": y})
    kept, dropped = prune_collinear_features(df, ["f_primary", "f_dup", "f_indep"], "target", threshold=0.7)
    assert len(dropped) == 1
    assert "f_indep" in kept
    assert len(kept) == 2
    assert abs(df[kept].corr(method="spearman").iloc[0, 1]) <= 0.7


def test_ks_covariate_shift_detection() -> None:
    rng = np.random.default_rng(11)
    tr = pd.DataFrame({"stable": rng.normal(0.0, 1.0, 400), "shifted": rng.normal(0.0, 1.0, 400)})
    te = pd.DataFrame({"stable": rng.normal(0.0, 1.0, 400), "shifted": rng.normal(2.5, 1.0, 400)})
    report = compute_ks_shift_report(tr, te, ["stable", "shifted"], alpha=0.01)
    by_feat = {r["feature"]: r for r in report}
    assert by_feat["shifted"]["drifted"] is True
    assert by_feat["stable"]["drifted"] is False
