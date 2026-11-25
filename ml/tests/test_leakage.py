"""Zero-future-leakage and walk-forward temporal isolation tests."""

from __future__ import annotations

import numpy as np
import polars as pl
import pytest

from features import MARKET_SPECS, build_leak_free_features, fit_role_priors
from train import _baseline_market_grid, build_walk_forward_folds


def test_mutating_week_w_and_future_does_not_change_week_w_features(
    synthetic_nfl_data: tuple[pl.DataFrame, pl.DataFrame],
) -> None:
    player_df, team_df = synthetic_nfl_data
    base_feats = build_leak_free_features(player_df, team_df)

    cutoff_season = 2023
    cutoff_week = 5

    # Violently mutate outcomes and usage in (cutoff_season, week >= cutoff_week) and future seasons
    mutate_mask = (pl.col("season") > cutoff_season) | (
        (pl.col("season") == cutoff_season) & (pl.col("week") >= cutoff_week)
    )
    mut_player = player_df.with_columns(
        pl.when(mutate_mask).then(999.0).otherwise(pl.col("pass_yds")).alias("pass_yds"),
        pl.when(mutate_mask).then(555.0).otherwise(pl.col("rush_yds")).alias("rush_yds"),
        pl.when(mutate_mask).then(444.0).otherwise(pl.col("rec_yds")).alias("rec_yds"),
        pl.when(mutate_mask).then(25).otherwise(pl.col("receptions")).alias("receptions"),
        pl.when(mutate_mask).then(40).otherwise(pl.col("carries")).alias("carries"),
        pl.when(mutate_mask).then(0.99).otherwise(pl.col("target_share")).alias("target_share"),
        pl.when(mutate_mask).then(0.99).otherwise(pl.col("offense_pct")).alias("offense_pct"),
        pl.when(mutate_mask).then(99.0).otherwise(pl.col("exp_rec_yds")).alias("exp_rec_yds"),
    )
    mut_team = team_df.with_columns(
        pl.when(mutate_mask).then(100.0).otherwise(pl.col("plays_per_game")).alias("plays_per_game"),
        pl.when(mutate_mask).then(0.95).otherwise(pl.col("def_pass_epa_allowed")).alias("def_pass_epa_allowed"),
    )
    mut_feats = build_leak_free_features(mut_player, mut_team)

    pre_mask_base = (base_feats["season"] < cutoff_season) | (
        (base_feats["season"] == cutoff_season) & (base_feats["week"] <= cutoff_week)
    )
    pre_mask_mut = (mut_feats["season"] < cutoff_season) | (
        (mut_feats["season"] == cutoff_season) & (mut_feats["week"] <= cutoff_week)
    )

    b_sub = base_feats[pre_mask_base].sort_values(["season", "week", "player_id"]).reset_index(drop=True)
    m_sub = mut_feats[pre_mask_mut].sort_values(["season", "week", "player_id"]).reset_index(drop=True)
    assert len(b_sub) == len(m_sub)

    feature_cols = [
        c
        for c in b_sub.columns
        if c not in ("pass_yds", "rush_yds", "rec_yds", "receptions", "carries", "pass_tds", "anytime_td", "actual_team_margin", "actual_game_total")
    ]
    for col in feature_cols:
        s1 = b_sub[col]
        s2 = m_sub[col]
        if s1.dtype.kind in ("f", "i"):
            np.testing.assert_array_equal(s1.to_numpy(), s2.to_numpy(), err_msg=f"leakage detected in {col}")
        else:
            assert list(s1) == list(s2), f"leakage detected in {col}"


def test_role_priors_exclude_held_out_season(
    synthetic_nfl_data: tuple[pl.DataFrame, pl.DataFrame],
) -> None:
    player_df, team_df = synthetic_nfl_data
    feat_df = build_leak_free_features(player_df, team_df)
    tr_df = feat_df[feat_df["season"] < 2024].copy()
    priors_a = fit_role_priors(tr_df)

    # Corrupt held-out 2024 data in full frame; priors fit on < 2024 must remain identical.
    feat_corrupt = feat_df.copy()
    feat_corrupt.loc[feat_corrupt["season"] == 2024, "pass_yds"] = 9999.0
    priors_b = fit_role_priors(feat_corrupt[feat_corrupt["season"] < 2024])
    assert priors_a == priors_b


def test_walk_forward_folds_strict_chronology() -> None:
    folds = build_walk_forward_folds([2019, 2020, 2021, 2022, 2023, 2024], min_held_out=3)
    assert len(folds) == 3
    assert [f["test_season"] for f in folds] == [2022, 2023, 2024]
    for f in folds:
        assert max(f["train_seasons"]) < f["test_season"]

    with pytest.raises(ValueError, match="at least 4"):
        build_walk_forward_folds([2022, 2023, 2024], min_held_out=3)


def test_anytime_td_baseline_market_grid_does_not_leak_test_outcomes(
    synthetic_nfl_data: tuple[pl.DataFrame, pl.DataFrame],
) -> None:
    player_df, team_df = synthetic_nfl_data
    feat_df = build_leak_free_features(player_df, team_df)
    tr_df = feat_df[feat_df["season"] < 2024].copy()
    te_df = feat_df[feat_df["season"] == 2024].copy()
    priors = fit_role_priors(tr_df)

    spec = MARKET_SPECS["ANYTIME_TD"]
    grid_base = _baseline_market_grid(te_df, spec, priors)

    te_all_td = te_df.copy()
    te_all_td["anytime_td"] = 1.0
    grid_all_td = _baseline_market_grid(te_all_td, spec, priors)

    te_no_td = te_df.copy()
    te_no_td["anytime_td"] = 0.0
    grid_no_td = _baseline_market_grid(te_no_td, spec, priors)

    np.testing.assert_array_equal(grid_base["p_market"].to_numpy(), grid_all_td["p_market"].to_numpy())
    np.testing.assert_array_equal(grid_base["p_market"].to_numpy(), grid_no_td["p_market"].to_numpy())
