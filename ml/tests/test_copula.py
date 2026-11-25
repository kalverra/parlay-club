"""Copula PSD projection, Sobol determinism, and SGP sign-flip tests."""

from __future__ import annotations

import numpy as np

from copula import (
    CANONICAL_COPULA_NODES,
    CopulaGameContext,
    CopulaLeg,
    CopulaModel,
    get_qmc_normals,
    higham_nearest_psd_correlation,
)


def test_higham_nearest_psd_fixes_indefinite_matrix() -> None:
    bad = np.array(
        [
            [1.0, 0.9, 0.9],
            [0.9, 1.0, -0.9],
            [0.9, -0.9, 1.0],
        ],
        dtype=np.float64,
    )
    assert np.min(np.linalg.eigvalsh(bad)) < 0.0
    fixed = higham_nearest_psd_correlation(bad)
    assert np.allclose(fixed, fixed.T, atol=1e-9)
    assert np.allclose(np.diag(fixed), 1.0, atol=1e-9)
    assert np.min(np.linalg.eigvalsh(fixed)) > 0.0


def test_sobol_qmc_normals_deterministic_and_standard_normal() -> None:
    n1 = get_qmc_normals()
    n2 = get_qmc_normals()
    assert n1.shape == (8192, 32)
    assert np.array_equal(n1, n2)
    assert abs(float(np.mean(n1[:, 0]))) < 0.01
    assert abs(float(np.std(n1[:, 0])) - 1.0) < 0.02


def test_copula_stack_vs_conflict_and_cross_game() -> None:
    n = len(CANONICAL_COPULA_NODES)
    mat = np.eye(n, dtype=np.float64)
    idx = {name: i for i, name in enumerate(CANONICAL_COPULA_NODES)}
    mat[idx["QB_PASS_YDS"], idx["WR1_REC_YDS"]] = 0.65
    mat[idx["WR1_REC_YDS"], idx["QB_PASS_YDS"]] = 0.65
    mat[idx["TEAM_MARGIN"], idx["RB1_RUSH_YDS"]] = 0.45
    mat[idx["RB1_RUSH_YDS"], idx["TEAM_MARGIN"]] = 0.45
    mat = higham_nearest_psd_correlation(mat)

    model = CopulaModel(
        nodes=list(CANONICAL_COPULA_NODES),
        default_matrix=mat.tolist(),
        regime_matrices={"neutral": mat.tolist()},
        within_player={"RECEPTIONS:REC_YDS": 0.84},
    )
    ctx = {"G1": CopulaGameContext("KC", "BUF", -3.0, 46.5, True, True)}

    pos = model.estimate_joint(
        [
            CopulaLeg(1, "G1", "KC", "p1", "QB", "QB", "PASS_YDS", "OVER", 0.55),
            CopulaLeg(2, "G1", "KC", "p2", "WR1", "WR1", "REC_YDS", "OVER", 0.55),
        ],
        ctx,
    )
    assert pos["corr_multiplier"] > 1.10

    neg = model.estimate_joint(
        [
            CopulaLeg(1, "G1", "KC", "p1", "QB", "QB", "PASS_YDS", "OVER", 0.55),
            CopulaLeg(2, "G1", "KC", "p2", "WR1", "WR1", "REC_YDS", "UNDER", 0.45),
        ],
        ctx,
    )
    assert neg["corr_multiplier"] < 0.85
    assert neg["pair_correlations"][0]["correlation"] < -0.5

    cross = model.estimate_joint(
        [
            CopulaLeg(1, "G1", "KC", "p1", "QB", "QB", "PASS_YDS", "OVER", 0.55),
            CopulaLeg(2, "G2", "SF", "p3", "RB1", "RB1", "RUSH_YDS", "OVER", 0.60),
        ],
        ctx,
    )
    assert abs(cross["corr_multiplier"] - 1.0) < 1e-12
