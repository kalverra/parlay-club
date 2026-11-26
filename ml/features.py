"""Leak-free point-in-time feature builder, Weeks 1-5 EB priors, KS shift checks, and Spearman pruning."""

from __future__ import annotations

from dataclasses import dataclass
import math
from typing import Any

import numpy as np
import pandas as pd
import polars as pl
from scipy import stats

from distributions import (
    FAMILY_BERNOULLI,
    FAMILY_NEGBIN,
    FAMILY_POISSON,
    FAMILY_ZERO_ADJ_GAMMA,
)

FEATURE_SCHEMA_VERSION = 1
EWMA_HALF_LIFE_WEEKS = 3.0
SHRINK_MAX_GAMES = 5
DEFAULT_PRIOR_STRENGTH = 3.0

ROLE_QB = "QB"
ROLE_WR1 = "WR1"
ROLE_WR2 = "WR2"
ROLE_TE1 = "TE1"
ROLE_RB1 = "RB1"
ROLE_RB2 = "RB2"
ROLE_UNKNOWN = ""
ALL_ROLES = (ROLE_QB, ROLE_WR1, ROLE_WR2, ROLE_TE1, ROLE_RB1, ROLE_RB2)


@dataclass(frozen=True)
class MarketSpec:
    market: str
    family: str
    outcome_col: str
    ewma_anchor_col: str
    eligible_positions: tuple[str, ...]
    candidate_features: tuple[str, ...]
    prior_features: tuple[str, ...]
    is_discrete: bool


MARKET_SPECS: dict[str, MarketSpec] = {
    "PASS_YDS": MarketSpec(
        market="PASS_YDS",
        family=FAMILY_ZERO_ADJ_GAMMA,
        outcome_col="pass_yds",
        ewma_anchor_col="ewma_pass_yds",
        eligible_positions=("QB",),
        candidate_features=(
            "games_played",
            "ewma_pass_yds",
            "ewma_exp_pass_yds",
            "ewma_pass_att",
            "ewma_cpoe",
            "ewma_total_epa",
            "team_implied_total",
            "opp_implied_total",
            "team_spread",
            "abs_spread",
            "game_total",
            "opp_role_yds_allowed",
            "opp_role_epa_allowed",
            "opp_def_pass_epa_allowed",
            "team_plays_per_game",
            "team_pass_rate",
            "is_indoor",
            "wind_mph",
            "temp_f",
        ),
        prior_features=("ewma_pass_yds", "ewma_exp_pass_yds", "ewma_pass_att", "ewma_cpoe"),
        is_discrete=False,
    ),
    "RUSH_YDS": MarketSpec(
        market="RUSH_YDS",
        family=FAMILY_ZERO_ADJ_GAMMA,
        outcome_col="rush_yds",
        ewma_anchor_col="ewma_rush_yds",
        eligible_positions=("RB", "QB"),
        candidate_features=(
            "games_played",
            "ewma_rush_yds",
            "ewma_exp_rush_yds",
            "ewma_carries",
            "ewma_carry_share",
            "ewma_offense_pct",
            "ewma_total_epa",
            "team_spread",
            "abs_spread",
            "team_implied_total",
            "game_total",
            "opp_role_yds_allowed",
            "opp_role_epa_allowed",
            "opp_def_rush_epa_allowed",
            "team_plays_per_game",
            "team_pass_rate",
        ),
        prior_features=("ewma_rush_yds", "ewma_exp_rush_yds", "ewma_carry_share", "ewma_carries"),
        is_discrete=False,
    ),
    "REC_YDS": MarketSpec(
        market="REC_YDS",
        family=FAMILY_ZERO_ADJ_GAMMA,
        outcome_col="rec_yds",
        ewma_anchor_col="ewma_rec_yds",
        eligible_positions=("WR", "TE", "RB"),
        candidate_features=(
            "games_played",
            "ewma_rec_yds",
            "ewma_exp_rec_yds",
            "ewma_target_share",
            "ewma_air_yards_share",
            "ewma_wopr",
            "ewma_targets",
            "ewma_receptions",
            "ewma_offense_pct",
            "ewma_total_epa",
            "team_implied_total",
            "team_spread",
            "abs_spread",
            "game_total",
            "opp_role_yds_allowed",
            "opp_role_epa_allowed",
            "opp_def_pass_epa_allowed",
            "team_plays_per_game",
            "team_pass_rate",
            "wind_mph",
        ),
        prior_features=("ewma_rec_yds", "ewma_exp_rec_yds", "ewma_target_share", "ewma_wopr"),
        is_discrete=False,
    ),
    "RECEPTIONS": MarketSpec(
        market="RECEPTIONS",
        family=FAMILY_NEGBIN,
        outcome_col="receptions",
        ewma_anchor_col="ewma_receptions",
        eligible_positions=("WR", "TE", "RB"),
        candidate_features=(
            "games_played",
            "ewma_receptions",
            "ewma_exp_receptions",
            "ewma_target_share",
            "ewma_targets",
            "ewma_wopr",
            "ewma_offense_pct",
            "team_implied_total",
            "team_spread",
            "abs_spread",
            "game_total",
            "opp_role_receptions_allowed",
            "opp_role_epa_allowed",
            "opp_def_pass_epa_allowed",
            "team_plays_per_game",
            "team_pass_rate",
            "wind_mph",
        ),
        prior_features=("ewma_receptions", "ewma_exp_receptions", "ewma_target_share", "ewma_targets"),
        is_discrete=True,
    ),
    "RUSH_ATT": MarketSpec(
        market="RUSH_ATT",
        family=FAMILY_NEGBIN,
        outcome_col="carries",
        ewma_anchor_col="ewma_carries",
        eligible_positions=("RB", "QB"),
        candidate_features=(
            "games_played",
            "ewma_carries",
            "ewma_carry_share",
            "ewma_offense_pct",
            "ewma_exp_rush_yds",
            "team_spread",
            "abs_spread",
            "team_implied_total",
            "game_total",
            "opp_role_yds_allowed",
            "opp_def_rush_epa_allowed",
            "team_plays_per_game",
            "team_pass_rate",
        ),
        prior_features=("ewma_carries", "ewma_carry_share", "ewma_offense_pct"),
        is_discrete=True,
    ),
    "PASS_TDS": MarketSpec(
        market="PASS_TDS",
        family=FAMILY_POISSON,
        outcome_col="pass_tds",
        ewma_anchor_col="ewma_pass_tds",
        eligible_positions=("QB",),
        candidate_features=(
            "games_played",
            "ewma_pass_tds",
            "ewma_exp_pass_tds",
            "ewma_pass_att",
            "ewma_pass_yds",
            "ewma_cpoe",
            "ewma_total_epa",
            "team_implied_total",
            "game_total",
            "team_spread",
            "opp_role_tds_allowed",
            "opp_role_epa_allowed",
            "opp_def_pass_epa_allowed",
            "team_pass_rate",
            "wind_mph",
        ),
        prior_features=("ewma_pass_tds", "ewma_exp_pass_tds", "ewma_pass_att"),
        is_discrete=True,
    ),
    "ANYTIME_TD": MarketSpec(
        market="ANYTIME_TD",
        family=FAMILY_BERNOULLI,
        outcome_col="anytime_td",
        ewma_anchor_col="ewma_exp_rush_tds",
        eligible_positions=("RB", "WR", "TE", "QB"),
        candidate_features=(
            "games_played",
            "ewma_exp_rush_tds",
            "ewma_exp_rec_tds",
            "ewma_rush_tds",
            "ewma_rec_tds",
            "ewma_carry_share",
            "ewma_target_share",
            "ewma_offense_pct",
            "ewma_total_epa",
            "team_implied_total",
            "game_total",
            "team_spread",
            "opp_role_tds_allowed",
            "opp_role_epa_allowed",
            "opp_def_pass_epa_allowed",
            "opp_def_rush_epa_allowed",
            "team_plays_per_game",
        ),
        prior_features=("ewma_rz_opps", "ewma_exp_rush_tds", "ewma_exp_rec_tds", "ewma_rush_tds", "ewma_rec_tds"),
        is_discrete=True,
    ),
}

EWMA_METRIC_COLS: tuple[tuple[str, str], ...] = (
    ("offense_pct", "ewma_offense_pct"),
    ("target_share", "ewma_target_share"),
    ("air_yards_share", "ewma_air_yards_share"),
    ("wopr", "ewma_wopr"),
    ("carry_share", "ewma_carry_share"),
    ("pass_att", "ewma_pass_att"),
    ("carries", "ewma_carries"),
    ("targets", "ewma_targets"),
    ("receptions", "ewma_receptions"),
    ("pass_yds", "ewma_pass_yds"),
    ("rush_yds", "ewma_rush_yds"),
    ("rec_yds", "ewma_rec_yds"),
    ("pass_tds", "ewma_pass_tds"),
    ("rush_tds", "ewma_rush_tds"),
    ("rec_tds", "ewma_rec_tds"),
    ("rz_opps", "ewma_rz_opps"),
    ("total_epa", "ewma_total_epa"),
    ("cpoe", "ewma_cpoe"),
    ("exp_pass_yds", "ewma_exp_pass_yds"),
    ("exp_rush_yds", "ewma_exp_rush_yds"),
    ("exp_rec_yds", "ewma_exp_rec_yds"),
    ("exp_receptions", "ewma_exp_receptions"),
    ("exp_pass_tds", "ewma_exp_pass_tds"),
    ("exp_rush_tds", "ewma_exp_rush_tds"),
    ("exp_rec_tds", "ewma_exp_rec_tds"),
)


def compute_ewma_desc(values: list[float], half_life: float = EWMA_HALF_LIFE_WEEKS) -> float:
    """Compute normalized EWMA matching Go analyzer.ComputeEWMA (values ordered most recent first)."""
    if not values:
        return 0.0
    hl = half_life if half_life > 0.0 else EWMA_HALF_LIFE_WEEKS
    lam = math.pow(0.5, 1.0 / hl)
    weight = 1.0
    w_sum = 0.0
    w_tot = 0.0
    for v in values:
        w_sum += weight * float(v)
        w_tot += weight
        weight *= lam
    return w_sum / w_tot if w_tot > 0.0 else 0.0


def empirical_bayes_shrink(observed: float, prior_mean: float, games_played: int, prior_strength: float = DEFAULT_PRIOR_STRENGTH) -> float:
    """Apply Weeks 1-5 Empirical Bayes shrinkage matching Go modelv2.EmpiricalBayesShrink."""
    if games_played <= 0 or math.isnan(observed) or math.isinf(observed):
        return float(prior_mean)
    if games_played >= SHRINK_MAX_GAMES or prior_strength <= 0.0:
        return float(observed)
    n = float(games_played)
    return float((n * observed + prior_strength * prior_mean) / (n + prior_strength))


def _has_participation(ewma: dict[str, float]) -> bool:
    return any(
        ewma.get(k, 0.0) > 0.0
        for k in (
            "ewma_offense_pct",
            "ewma_target_share",
            "ewma_carry_share",
            "ewma_wopr",
            "ewma_targets",
            "ewma_carries",
            "ewma_pass_att",
            "ewma_exp_pass_yds",
            "ewma_exp_rush_yds",
            "ewma_exp_rec_yds",
        )
    )


def _derive_roles_from_history(history_rows: list[dict[str, Any]], half_life: float) -> dict[str, tuple[str, dict[str, float]]]:
    """Derive deterministic player EWMA and role archetypes strictly from prior-week rows."""
    by_player: dict[str, list[dict[str, Any]]] = {}
    for row in history_rows:
        by_player.setdefault(row["player_id"], []).append(row)

    profiles: dict[str, tuple[str, str, str, dict[str, float]]] = {}
    by_team_pos: dict[tuple[str, str], list[tuple[str, dict[str, float]]]] = {}

    for pid, rows in by_player.items():
        # Order most recent first: descending (season, season_type, week)
        ordered = sorted(rows, key=lambda r: (r["season"], r["season_type"], r["week"]), reverse=True)
        ewma: dict[str, float] = {"games_played": float(len(ordered))}
        for src, dst in EWMA_METRIC_COLS:
            ewma[dst] = compute_ewma_desc([float(r.get(src, 0.0) or 0.0) for r in ordered], half_life)

        team = next((r["team"] for r in ordered if r.get("team")), "")
        pos = next((r["position"] for r in ordered if r.get("position")), "")
        role = ROLE_QB if pos == "QB" else ROLE_UNKNOWN
        profiles[pid] = (team, pos, role, ewma)

        if team and _has_participation(ewma) and pos in ("WR", "TE", "RB"):
            by_team_pos.setdefault((team, pos), []).append((pid, ewma))

    for (team, pos), group in by_team_pos.items():
        if pos == "WR":
            group.sort(
                key=lambda item: (
                    -item[1]["ewma_target_share"],
                    -item[1]["ewma_wopr"],
                    -item[1]["ewma_offense_pct"],
                    -item[1]["ewma_targets"],
                    -item[1]["games_played"],
                    item[0],
                )
            )
            for idx, (pid, ewma) in enumerate(group[:2]):
                profiles[pid] = (team, pos, ROLE_WR1 if idx == 0 else ROLE_WR2, ewma)
        elif pos == "TE":
            group.sort(
                key=lambda item: (
                    -item[1]["ewma_target_share"],
                    -item[1]["ewma_wopr"],
                    -item[1]["ewma_offense_pct"],
                    -item[1]["ewma_targets"],
                    -item[1]["games_played"],
                    item[0],
                )
            )
            pid, ewma = group[0]
            profiles[pid] = (team, pos, ROLE_TE1, ewma)
        elif pos == "RB":
            group.sort(
                key=lambda item: (
                    -(0.60 * item[1]["ewma_carry_share"] + 0.40 * item[1]["ewma_offense_pct"]),
                    -item[1]["ewma_carry_share"],
                    -item[1]["ewma_carries"],
                    -item[1]["ewma_offense_pct"],
                    -item[1]["ewma_target_share"],
                    -item[1]["games_played"],
                    item[0],
                )
            )
            for idx, (pid, ewma) in enumerate(group[:2]):
                profiles[pid] = (team, pos, ROLE_RB1 if idx == 0 else ROLE_RB2, ewma)

    return {pid: (role, ewma) for pid, (_, _, role, ewma) in profiles.items()}


def _build_opp_role_defense(history_rows: list[dict[str, Any]], roles: dict[str, tuple[str, dict[str, float]]]) -> dict[tuple[str, str], dict[str, float]]:
    """Aggregate opponent per-game stats allowed to each role archetype strictly from prior weeks."""
    by_game: dict[tuple[str, str, int, int], dict[str, float]] = {}
    for row in history_rows:
        opp = row.get("opponent") or ""
        if not opp:
            continue
        prof = roles.get(row["player_id"])
        if not prof or not prof[0]:
            continue
        role = prof[0]
        key = (opp, role, row["season_type"], row["week"])
        cur = by_game.setdefault(
            key,
            {"yds": 0.0, "receptions": 0.0, "tds": 0.0, "epa": 0.0},
        )
        if role == ROLE_QB:
            cur["yds"] += float(row.get("pass_yds", 0.0) or 0.0)
            cur["tds"] += float(row.get("pass_tds", 0.0) or 0.0)
        elif role in (ROLE_RB1, ROLE_RB2):
            cur["yds"] += float(row.get("rush_yds", 0.0) or 0.0)
            cur["tds"] += float(row.get("rush_tds", 0.0) or 0.0) + float(row.get("rec_tds", 0.0) or 0.0)
        else:
            cur["yds"] += float(row.get("rec_yds", 0.0) or 0.0)
            cur["tds"] += float(row.get("rec_tds", 0.0) or 0.0)
        cur["receptions"] += float(row.get("receptions", 0.0) or 0.0)
        cur["epa"] += float(row.get("total_epa", 0.0) or 0.0)

    agg: dict[tuple[str, str], dict[str, float]] = {}
    counts: dict[tuple[str, str], int] = {}
    for (opp, role, _, _), vals in by_game.items():
        k = (opp, role)
        counts[k] = counts.get(k, 0) + 1
        cur = agg.setdefault(k, {"yds": 0.0, "receptions": 0.0, "tds": 0.0, "epa": 0.0})
        for metric in ("yds", "receptions", "tds", "epa"):
            cur[metric] += vals[metric]

    out: dict[tuple[str, str], dict[str, float]] = {}
    for k, cur in agg.items():
        n = float(counts[k])
        out[k] = {
            "opp_role_yds_allowed": cur["yds"] / n,
            "opp_role_receptions_allowed": cur["receptions"] / n,
            "opp_role_tds_allowed": cur["tds"] / n,
            "opp_role_epa_allowed": cur["epa"] / n,
        }
    return out


def _build_team_profiles(team_rows: list[dict[str, Any]]) -> dict[str, dict[str, float]]:
    by_team: dict[str, list[dict[str, Any]]] = {}
    for row in team_rows:
        by_team.setdefault(row["team"], []).append(row)
    out: dict[str, dict[str, float]] = {}
    for team, rows in by_team.items():
        n = float(len(rows))
        out[team] = {
            "plays_per_game": sum(float(r["plays_per_game"]) for r in rows) / n,
            "pass_rate": sum(float(r["pass_rate"]) for r in rows) / n,
            "def_pass_epa_allowed": sum(float(r["def_pass_epa_allowed"]) for r in rows) / n,
            "def_rush_epa_allowed": sum(float(r["def_rush_epa_allowed"]) for r in rows) / n,
        }
    return out


def build_leak_free_features(
    player_df: pl.DataFrame,
    team_df: pl.DataFrame,
    half_life: float = EWMA_HALF_LIFE_WEEKS,
) -> pd.DataFrame:
    """Build strictly pre-kickoff features for every (season, season_type, week) player row.

    Every rolling EWMA, role archetype, team pace, and opponent-defense feature at
    (season S, week W) reads ONLY rows strictly earlier than (season_type, W) in season S.
    """
    p_records = player_df.sort(["season", "season_type", "week", "player_id"]).to_dicts()
    t_records = team_df.sort(["season", "season_type", "week", "team"]).to_dicts()

    players_by_season: dict[int, dict[tuple[int, int], list[dict[str, Any]]]] = {}
    for row in p_records:
        players_by_season.setdefault(int(row["season"]), {}).setdefault(
            (int(row["season_type"]), int(row["week"])), []
        ).append(row)

    teams_by_season: dict[int, dict[tuple[int, int], list[dict[str, Any]]]] = {}
    for row in t_records:
        teams_by_season.setdefault(int(row["season"]), {}).setdefault(
            (int(row["season_type"]), int(row["week"])), []
        ).append(row)

    output_rows: list[dict[str, Any]] = []
    for season in sorted(players_by_season.keys()):
        season_weeks = sorted(players_by_season[season].keys())
        prior_player_rows: list[dict[str, Any]] = []
        prior_team_rows: list[dict[str, Any]] = []

        for st, wk in season_weeks:
            roles = _derive_roles_from_history(prior_player_rows, half_life=half_life)
            opp_def = _build_opp_role_defense(prior_player_rows, roles)
            team_prof = _build_team_profiles(prior_team_rows)

            current_rows = players_by_season[season][(st, wk)]
            for row in current_rows:
                pid = row["player_id"]
                team = row["team"]
                opp = row["opponent"]
                pos = row["position"]
                prof = roles.get(pid)

                if prof is not None:
                    role, ewma = prof
                    if not role and pos == "QB":
                        role = ROLE_QB
                else:
                    role = ROLE_QB if pos == "QB" else ROLE_UNKNOWN
                    ewma = {"games_played": 0.0}
                    for _, dst in EWMA_METRIC_COLS:
                        ewma[dst] = np.nan

                opp_role_key = (opp, role) if role else None
                opp_role_vals = opp_def.get(opp_role_key) if opp_role_key else None
                t_off = team_prof.get(team)
                t_opp = team_prof.get(opp)

                feat_row: dict[str, Any] = {
                    "game_id": row.get("game_id") or f"{season}_{wk:02d}_{team}_{opp}",
                    "player_id": pid,
                    "player_name": row.get("player_name", ""),
                    "position": pos,
                    "role": role,
                    "team": team,
                    "opponent": opp,
                    "season": season,
                    "season_type": st,
                    "week": wk,
                    "is_home": bool(row.get("is_home", False)),
                    # Actual outcomes for training/eval (never used as features)
                    "pass_yds": float(row["pass_yds"]),
                    "rush_yds": float(row["rush_yds"]),
                    "rec_yds": float(row["rec_yds"]),
                    "receptions": float(row["receptions"]),
                    "carries": float(row["carries"]),
                    "pass_tds": float(row["pass_tds"]),
                    "anytime_td": float(row["anytime_td"]),
                    "actual_team_margin": row.get("actual_team_margin", np.nan),
                    "actual_game_total": row.get("actual_game_total", np.nan),
                    "home_spread": row.get("home_spread", np.nan),
                    # Features
                    "games_played": float(ewma.get("games_played", 0.0)),
                    "team_spread": float(row["team_spread"]) if row.get("team_spread") is not None else np.nan,
                    "abs_spread": float(row["abs_spread"]) if row.get("abs_spread") is not None else np.nan,
                    "game_total": float(row["game_total"]) if row.get("game_total") is not None else np.nan,
                    "team_implied_total": float(row["team_implied_total"]) if row.get("team_implied_total") is not None else np.nan,
                    "opp_implied_total": float(row["opp_implied_total"]) if row.get("opp_implied_total") is not None else np.nan,
                    "opp_role_yds_allowed": opp_role_vals["opp_role_yds_allowed"] if opp_role_vals else np.nan,
                    "opp_role_receptions_allowed": opp_role_vals["opp_role_receptions_allowed"] if opp_role_vals else np.nan,
                    "opp_role_tds_allowed": opp_role_vals["opp_role_tds_allowed"] if opp_role_vals else np.nan,
                    "opp_role_epa_allowed": opp_role_vals["opp_role_epa_allowed"] if opp_role_vals else np.nan,
                    "opp_def_pass_epa_allowed": t_opp["def_pass_epa_allowed"] if t_opp else np.nan,
                    "opp_def_rush_epa_allowed": t_opp["def_rush_epa_allowed"] if t_opp else np.nan,
                    "team_plays_per_game": t_off["plays_per_game"] if t_off else np.nan,
                    "team_pass_rate": t_off["pass_rate"] if t_off else np.nan,
                    "is_indoor": float(row["is_indoor"]) if row.get("is_indoor") is not None else np.nan,
                    "wind_mph": float(row["wind_mph"]) if row.get("wind_mph") is not None else np.nan,
                    "precip_in": float(row["precip_in"]) if row.get("precip_in") is not None else np.nan,
                    "temp_f": float(row["temp_f"]) if row.get("temp_f") is not None else np.nan,
                }
                for _, dst in EWMA_METRIC_COLS:
                    feat_row[dst] = float(ewma.get(dst, np.nan))
                output_rows.append(feat_row)

            # Only AFTER scoring week (st, wk) do we append (st, wk) to prior history.
            prior_player_rows.extend(current_rows)
            prior_team_rows.extend(teams_by_season.get(season, {}).get((st, wk), []))

    return pd.DataFrame(output_rows)


def fit_role_priors(train_df: pd.DataFrame) -> dict[str, dict[str, Any]]:
    """Fit Empirical Bayes role priors strictly on training seasons."""
    priors: dict[str, dict[str, Any]] = {}
    default_roles_by_pos = {"QB": ROLE_QB, "WR": ROLE_WR2, "TE": ROLE_TE1, "RB": ROLE_RB2}

    for market, spec in MARKET_SPECS.items():
        sub = train_df[train_df["position"].isin(spec.eligible_positions)].copy()
        if sub.empty:
            continue
        sub["effective_role"] = np.where(
            sub["role"].isin(ALL_ROLES),
            sub["role"],
            sub["position"].map(default_roles_by_pos).fillna(""),
        )

        roles_to_fit = sorted({r for r in sub["effective_role"].unique() if r in ALL_ROLES})
        for role in roles_to_fit:
            if market in ("PASS_YDS", "PASS_TDS") and role != ROLE_QB:
                continue
            if market in ("RUSH_YDS", "RUSH_ATT") and role not in (ROLE_QB, ROLE_RB1, ROLE_RB2):
                continue
            if market in ("REC_YDS", "RECEPTIONS") and role not in (ROLE_WR1, ROLE_WR2, ROLE_TE1, ROLE_RB1, ROLE_RB2):
                continue

            r_df = sub[sub["effective_role"] == role]
            if len(r_df) < 10:
                continue
            vals = r_df[spec.outcome_col].dropna().to_numpy(dtype=np.float64)
            mean_val = float(np.mean(vals)) if len(vals) > 0 else 0.0
            std_val = float(np.std(vals)) if len(vals) > 1 else 0.0
            feat_means: dict[str, float] = {}
            for feat in spec.prior_features:
                if feat in r_df.columns:
                    col = r_df[feat].dropna()
                    feat_means[feat] = float(col.mean()) if len(col) > 0 else 0.0
            key = f"{market}:{role}"
            priors[key] = {
                "market": market,
                "role": role,
                "prior_strength": DEFAULT_PRIOR_STRENGTH,
                "mean": mean_val,
                "std": std_val,
                "features": feat_means,
            }
    return priors


def apply_eb_shrinkage_df(
    df: pd.DataFrame,
    market: str,
    priors: dict[str, dict[str, Any]],
) -> pd.DataFrame:
    """Apply Weeks 1-5 Empirical Bayes shrinkage identically to Go modelv2.ShrinkFeatureSlice."""
    out = df.copy()
    default_roles_by_pos = {"QB": ROLE_QB, "WR": ROLE_WR2, "TE": ROLE_TE1, "RB": ROLE_RB2}
    games = out["games_played"].fillna(0.0).to_numpy(dtype=np.int64)
    sparse_mask = games < SHRINK_MAX_GAMES
    if not np.any(sparse_mask):
        return out

    roles = out["role"].fillna("").astype(str).to_numpy()
    positions = out["position"].fillna("").astype(str).to_numpy()

    for idx in np.flatnonzero(sparse_mask):
        role = roles[idx]
        pos = positions[idx]
        prior = priors.get(f"{market}:{role}") if role else None
        if prior is None and pos in default_roles_by_pos:
            prior = priors.get(f"{market}:{default_roles_by_pos[pos]}")
        if prior is None:
            prior = priors.get(f"{market}:DEFAULT")
        if prior is None:
            continue
        strength = float(prior.get("prior_strength", DEFAULT_PRIOR_STRENGTH)) or DEFAULT_PRIOR_STRENGTH
        n_games = int(games[idx])
        for feat, pval in prior.get("features", {}).items():
            if feat in out.columns:
                obs = float(out.iloc[idx, out.columns.get_loc(feat)])
                out.iloc[idx, out.columns.get_loc(feat)] = empirical_bayes_shrink(obs, float(pval), n_games, strength)
    return out


def compute_ks_shift_report(
    train_df: pd.DataFrame,
    test_df: pd.DataFrame,
    features: list[str],
    alpha: float = 0.01,
) -> list[dict[str, Any]]:
    """Run two-sample KS covariate-shift checks between training and held-out seasons."""
    report: list[dict[str, Any]] = []
    for feat in features:
        if feat not in train_df.columns or feat not in test_df.columns:
            continue
        tr = train_df[feat].dropna().to_numpy(dtype=np.float64)
        te = test_df[feat].dropna().to_numpy(dtype=np.float64)
        if len(tr) < 20 or len(te) < 20:
            continue
        res = stats.ks_2samp(tr, te)
        report.append(
            {
                "feature": feat,
                "ks_stat": float(res.statistic),
                "p_value": float(res.pvalue),
                "drifted": bool(res.pvalue < alpha and res.statistic > 0.10),
            }
        )
    return sorted(report, key=lambda r: r["ks_stat"], reverse=True)


def prune_collinear_features(
    train_df: pd.DataFrame,
    candidate_features: list[str],
    target_col: str,
    threshold: float = 0.7,
) -> tuple[list[str], list[dict[str, Any]]]:
    """Deterministically prune redundant features with pairwise |Spearman rho| > threshold on train_df."""
    available = [f for f in candidate_features if f in train_df.columns]
    if len(available) <= 1:
        return available, []

    sub = train_df[available + [target_col]].copy()
    valid_feats = [f for f in available if sub[f].nunique(dropna=True) > 1]
    dropped: list[dict[str, Any]] = []
    if len(valid_feats) <= 1:
        return valid_feats, dropped

    # Rank features by |Spearman(feat, target)| descending, preserving candidate_features priority on ties.
    priority = {f: i for i, f in enumerate(candidate_features)}
    target_vals = sub[target_col].to_numpy(dtype=np.float64)
    target_corr: dict[str, float] = {}
    for f in valid_feats:
        col_vals = sub[f].to_numpy(dtype=np.float64)
        mask = np.isfinite(col_vals) & np.isfinite(target_vals)
        if np.sum(mask) < 10:
            target_corr[f] = 0.0
        else:
            rho, _ = stats.spearmanr(col_vals[mask], target_vals[mask])
            target_corr[f] = 0.0 if np.isnan(rho) else abs(float(rho))

    # Compute pairwise Spearman correlation matrix on training rows only.
    corr_df = sub[valid_feats].corr(method="spearman").abs()
    kept: list[str] = []
    # Keep games_played always if present; evaluate remaining features in candidate priority order
    # while breaking collinear pairs in favor of higher target correlation / domain priority.
    ordered = sorted(valid_feats, key=lambda f: (0 if f == "games_played" else 1, priority[f]))
    for feat in ordered:
        conflict = None
        conflict_rho = 0.0
        for prev in kept:
            if prev == "games_played" or feat == "games_played":
                continue
            rho = float(corr_df.loc[feat, prev])
            if not math.isnan(rho) and rho > threshold:
                conflict = prev
                conflict_rho = rho
                break
        if conflict is None:
            kept.append(feat)
            continue

        # Prefer whichever feature has stronger target correlation (with small margin for domain primary order).
        if target_corr[feat] > target_corr[conflict] + 0.05:
            dropped.append({"dropped": conflict, "kept": feat, "spearman": round(conflict_rho, 4)})
            kept = [f for f in kept if f != conflict]
            kept.append(feat)
        else:
            dropped.append({"dropped": feat, "kept": conflict, "spearman": round(conflict_rho, 4)})

    # Restore original canonical ordering.
    kept_sorted = sorted(kept, key=lambda f: priority[f])
    return kept_sorted, dropped
