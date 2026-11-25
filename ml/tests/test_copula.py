"""Copula PSD projection, Sobol determinism, and SGP sign-flip tests."""

from __future__ import annotations

import numpy as np
import pandas as pd

from copula import (
    CANONICAL_COPULA_NODES,
    CopulaGameContext,
    CopulaLeg,
    CopulaModel,
    evaluate_sgp_log_loss,
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


def test_evaluate_sgp_log_loss_uses_matched_prediction_lines() -> None:
    n = len(CANONICAL_COPULA_NODES)
    mat = np.eye(n, dtype=np.float64)
    idx = {name: i for i, name in enumerate(CANONICAL_COPULA_NODES)}
    mat[idx["QB_PASS_YDS"], idx["WR1_REC_YDS"]] = 0.65
    mat[idx["WR1_REC_YDS"], idx["QB_PASS_YDS"]] = 0.65
    mat = higham_nearest_psd_correlation(mat)

    model = CopulaModel(
        nodes=list(CANONICAL_COPULA_NODES),
        default_matrix=mat.tolist(),
        regime_matrices={"neutral": mat.tolist()},
        within_player={"PASS_YDS:PASS_TDS": 0.65},
    )

    # Heuristic fallback lines would be qb=249.5, wr=69.5, td=1.5, under which
    # actual (240, 65, 1) would MISS. With matched lines (220.5, 55.5, 0.5),
    # all three legs HIT, yielding much lower log loss for high predicted probs.
    test_df = pd.DataFrame(
        [
            {
                "game_id": "G1",
                "team": "KC",
                "opponent": "BUF",
                "is_home": True,
                "home_spread": -3.0,
                "game_total": 48.0,
                "player_id": "qb1",
                "player_name": "QB One",
                "role": "QB",
                "ewma_pass_yds": 250.0,
                "ewma_rec_yds": np.nan,
                "ewma_rush_yds": np.nan,
                "pass_yds": 240.0,
                "pass_tds": 1.0,
                "rec_yds": 0.0,
                "rush_yds": 0.0,
                "actual_team_margin": np.nan,
                "team_spread": np.nan,
            },
            {
                "game_id": "G1",
                "team": "KC",
                "opponent": "BUF",
                "is_home": True,
                "home_spread": -3.0,
                "game_total": 48.0,
                "player_id": "wr1",
                "player_name": "WR One",
                "role": "WR1",
                "ewma_pass_yds": np.nan,
                "ewma_rec_yds": 70.0,
                "ewma_rush_yds": np.nan,
                "pass_yds": 0.0,
                "pass_tds": 0.0,
                "rec_yds": 65.0,
                "rush_yds": 0.0,
                "actual_team_margin": np.nan,
                "team_spread": np.nan,
            },
        ]
    )
    leg_preds = pd.DataFrame(
        [
            {"game_id": "G1", "player_id": "qb1", "market": "PASS_YDS", "line": 220.5, "prob": 0.90},
            {"game_id": "G1", "player_id": "qb1", "market": "PASS_TDS", "line": 0.5, "prob": 0.90},
            {"game_id": "G1", "player_id": "wr1", "market": "REC_YDS", "line": 55.5, "prob": 0.90},
        ]
    )
    res_matched = evaluate_sgp_log_loss(model, test_df, leg_preds)
    assert res_matched["sgp_count"] == 2
    assert res_matched["sgp_log_loss"] < 0.20

    # Without "line", fallback thresholds miss and high probs suffer > 1.5 log loss.
    res_fallback = evaluate_sgp_log_loss(model, test_df, leg_preds.drop(columns=["line"]))
    assert res_fallback["sgp_count"] == 2
    assert res_fallback["sgp_log_loss"] > 1.50
