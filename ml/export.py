"""Artifact bundle export, Go/Python parity fixture generation, Markdown report, and CLI entrypoint."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import math
from pathlib import Path
from typing import Any

import numpy as np

from copula import CopulaGameContext, CopulaLeg, CopulaModel
from data import load_historical_tables
from features import FEATURE_SCHEMA_VERSION, MARKET_SPECS
from train import TrainingResult, run_walk_forward_training

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_ARTIFACT_OUT = REPO_ROOT / "internal" / "modelv2" / "artifacts" / "v2.json"
DEFAULT_PARITY_OUT = REPO_ROOT / "internal" / "modelv2" / "testdata" / "parity_vectors.json"
DEFAULT_REPORT_OUT = Path(__file__).resolve().parent / "reports" / "latest.md"


def build_artifact_bundle_dict(result: TrainingResult, version: str = "v2.1.0") -> dict[str, Any]:
    """Assemble an ArtifactBundle dictionary matching Go internal/modelv2/artifact.go."""
    return {
        "version": version,
        "feature_schema_version": FEATURE_SCHEMA_VERSION,
        "trained_seasons": result.trained_seasons,
        "built_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "validation": {
            "held_out_seasons": result.validation["held_out_seasons"],
            "log_loss": result.validation["log_loss"],
            "brier_score": result.validation["brier_score"],
            "market_log_loss": result.validation["market_log_loss"],
            "market_brier_score": result.validation["market_brier_score"],
            "sgp_log_loss": result.validation["sgp_log_loss"],
            "naive_indep_log_loss": result.validation["naive_indep_log_loss"],
            "populated_bins": result.validation["populated_bins"],
            "total_bins": result.validation["total_bins"],
            "notes": result.validation["notes"],
        },
        "prop_models": {m: model.to_dict() for m, model in result.prop_models.items()},
        "priors": result.priors,
        "copula": result.copula.to_dict(),
    }


def validate_artifact_bundle_dict(bundle: dict[str, Any]) -> None:
    """Validate structural, PSD, calibrator, and schema invariants before writing v2.json."""
    if not bundle.get("version"):
        raise ValueError("version must not be empty")
    if bundle.get("feature_schema_version") != FEATURE_SCHEMA_VERSION:
        raise ValueError("unexpected feature_schema_version")
    if not bundle.get("trained_seasons"):
        raise ValueError("trained_seasons must not be empty")
    prop_models = bundle.get("prop_models", {})
    if set(prop_models.keys()) != set(MARKET_SPECS.keys()):
        raise ValueError(f"prop_models mismatch: got {sorted(prop_models.keys())}")
    for m, dm in prop_models.items():
        if not dm.get("feature_names"):
            raise ValueError(f"{m}: empty feature_names")
        w = float(dm["market_blend_weight"])
        if not (0.0 <= w <= 1.0):
            raise ValueError(f"{m}: market_blend_weight {w} out of [0, 1]")
        for pname, gbdt in dm["param_models"].items():
            if not gbdt.get("trees"):
                raise ValueError(f"{m}.{pname}: trees must not be empty")
            if not math.isfinite(float(gbdt["base_score"])):
                raise ValueError(f"{m}.{pname}: base_score must be finite")
            for tidx, tree in enumerate(gbdt["trees"]):
                for nidx, node in enumerate(tree.get("nodes", [])):
                    if not math.isfinite(float(node["threshold"])) or not math.isfinite(float(node["leaf"])):
                        raise ValueError(f"{m}.{pname}.trees[{tidx}].nodes[{nidx}]: non-finite threshold or leaf")

    cop = bundle.get("copula", {})
    nodes = cop.get("nodes", [])
    n = len(nodes)
    if n < 2:
        raise ValueError("copula nodes must have >= 2 entries")
    for label, mat in [("default_matrix", cop.get("default_matrix"))] + list(cop.get("regime_matrices", {}).items()):
        arr = np.asarray(mat, dtype=np.float64)
        if arr.shape != (n, n):
            raise ValueError(f"{label}: shape {arr.shape} != {(n, n)}")
        if not np.allclose(arr, arr.T, atol=1e-6):
            raise ValueError(f"{label}: matrix is not symmetric")
        if not np.allclose(np.diag(arr), 1.0, atol=1e-6):
            raise ValueError(f"{label}: diagonal is not 1.0")
        min_eig = float(np.min(np.linalg.eigvalsh(arr)))
        if min_eig < -1e-9:
            raise ValueError(f"{label}: not PSD (min eig={min_eig})")


def generate_parity_vectors(
    bundle: dict[str, Any],
    result: TrainingResult,
    seed: int = 42,
) -> dict[str, Any]:
    """Generate 100+ diverse leg and copula parity test vectors for Go parity_test.go."""
    rng = np.random.default_rng(seed)
    leg_vectors: list[dict[str, Any]] = []

    role_by_market = {
        "PASS_YDS": ("QB", "QB"),
        "PASS_TDS": ("QB", "QB"),
        "RUSH_YDS": ("RB1", "RB"),
        "RUSH_ATT": ("RB1", "RB"),
        "REC_YDS": ("WR1", "WR"),
        "RECEPTIONS": ("TE1", "TE"),
        "ANYTIME_TD": ("WR1", "WR"),
    }
    base_lines = {
        "PASS_YDS": [195.5, 245.5, 295.5],
        "PASS_TDS": [0.5, 1.5, 2.5],
        "RUSH_YDS": [34.5, 62.5, 92.5],
        "RUSH_ATT": [9.5, 14.5, 19.5],
        "REC_YDS": [29.5, 58.5, 88.5],
        "RECEPTIONS": [2.5, 4.5, 6.5],
        "ANYTIME_TD": [0.5],
    }

    for market, model in result.prop_models.items():
        role, pos = role_by_market[market]
        prior = result.priors.get(f"{market}:{role}") or result.priors.get(f"{market}:DEFAULT")
        prior_feats = prior.get("features", {}) if prior else {}

        # 16+ cases per market (7 * 16 = 112 leg vectors) covering Weeks 1-5 shrinkage, NaNs, Over/Under, alt lines
        cases = [
            (0, "OVER", base_lines[market][0], 0.54, 0.0, False),
            (1, "UNDER", base_lines[market][0], 0.46, 0.0, False),
            (2, "OVER", base_lines[market][-1], 0.35, 0.15, False),
            (3, "UNDER", base_lines[market][-1], 0.65, 0.0, False),
            (4, "OVER", base_lines[market][len(base_lines[market]) // 2], 0.52, 0.0, True),
            (6, "OVER", base_lines[market][0], 0.60, 0.0, False),
            (8, "UNDER", base_lines[market][0], 0.40, 0.10, False),
            (10, "OVER", base_lines[market][-1], 0.0, 0.0, False),
            (12, "UNDER", base_lines[market][-1], 0.0, 0.0, False),
            (1, "YES" if market == "ANYTIME_TD" else "OVER", base_lines[market][0], 0.42, 0.0, True),
            (3, "NO" if market == "ANYTIME_TD" else "UNDER", base_lines[market][0], 0.58, 0.15, False),
            (7, "OVER", base_lines[market][len(base_lines[market]) // 2], 0.51, 0.0, False),
            (9, "UNDER", base_lines[market][len(base_lines[market]) // 2], 0.49, 0.0, False),
            (11, "OVER", base_lines[market][-1], 0.28, 0.0, True),
            (14, "OVER", base_lines[market][0], 0.72, 0.0, False),
            (5, "UNDER", base_lines[market][0], 0.33, 0.15, False),
        ]

        for idx, (games_played, side, line, mkt_prob, unc_boost, inject_nan) in enumerate(cases):
            raw_map: dict[str, float] = {}
            for fname in model.feature_names:
                if fname == "games_played":
                    raw_map[fname] = float(games_played)
                elif fname in prior_feats:
                    raw_map[fname] = float(prior_feats[fname]) * float(rng.uniform(0.75, 1.35))
                elif "spread" in fname:
                    raw_map[fname] = float(rng.uniform(-9.5, 9.5)) if fname == "team_spread" else float(rng.uniform(0.5, 9.5))
                elif "total" in fname:
                    raw_map[fname] = float(rng.uniform(38.5, 52.5)) if fname == "game_total" else float(rng.uniform(18.0, 29.0))
                elif fname == "is_indoor":
                    raw_map[fname] = float(idx % 2)
                elif fname == "wind_mph":
                    raw_map[fname] = float(rng.uniform(0.0, 22.0))
                elif fname == "temp_f":
                    raw_map[fname] = float(rng.uniform(28.0, 82.0))
                elif "epa" in fname:
                    raw_map[fname] = float(rng.uniform(-0.25, 0.25))
                else:
                    raw_map[fname] = float(rng.uniform(0.1, 45.0))

            if inject_nan and len(model.feature_names) > 2:
                raw_map[model.feature_names[-1]] = float("nan")

            # Apply Weeks 1-5 shrinkage identically to Go ShrinkFeatureMap
            shrunk_map = dict(raw_map)
            if games_played < 5 and prior is not None:
                tau = float(prior.get("prior_strength", 3.0))
                for feat, pval in prior_feats.items():
                    obs = shrunk_map.get(feat, float("nan"))
                    if games_played <= 0 or np.isnan(obs):
                        shrunk_map[feat] = float(pval)
                    else:
                        shrunk_map[feat] = (games_played * obs + tau * float(pval)) / (games_played + tau)

            aligned = [shrunk_map.get(f, float("nan")) for f in model.feature_names]
            pred = model.predict_leg_scalar(aligned, side=side, line=line, market_prob=mkt_prob, uncertainty_boost=unc_boost)

            # Encode NaNs as None in JSON so Go can convert null -> math.NaN()
            raw_json = {k: (None if (isinstance(v, float) and np.isnan(v)) else round(float(v), 8)) for k, v in raw_map.items()}
            aligned_json = [None if np.isnan(v) else float(v) for v in aligned]
            leg_vectors.append(
                {
                    "id": f"{market}_{idx:02d}",
                    "market": market,
                    "role": role,
                    "position": pos,
                    "games_played": games_played,
                    "raw_features": raw_json,
                    "aligned_shrunk_features": aligned_json,
                    "side": side,
                    "line": float(line),
                    "market_prob": float(mkt_prob),
                    "uncertainty_boost": float(unc_boost),
                    "expected": pred,
                }
            )

    copula_model = CopulaModel(
        nodes=bundle["copula"]["nodes"],
        default_matrix=bundle["copula"]["default_matrix"],
        regime_matrices=bundle["copula"]["regime_matrices"],
        within_player=bundle["copula"]["within_player"],
    )

    copula_cases: list[tuple[str, list[CopulaLeg], dict[str, CopulaGameContext]]] = [
        (
            "neutral_qb_wr1_stack",
            [
                CopulaLeg(1, "G1", "KC", "p1", "QB One", "QB", "PASS_YDS", "OVER", 0.56),
                CopulaLeg(2, "G1", "KC", "p2", "WR One", "WR1", "REC_YDS", "OVER", 0.54),
            ],
            {"G1": CopulaGameContext("KC", "BUF", -2.5, 46.5, True, True)},
        ),
        (
            "shootout_3leg_over_under",
            [
                CopulaLeg(1, "G2", "CIN", "p3", "QB Two", "QB", "PASS_YDS", "OVER", 0.58),
                CopulaLeg(2, "G2", "CIN", "p4", "WR Two", "WR1", "REC_YDS", "OVER", 0.55),
                CopulaLeg(3, "G2", "BAL", "p5", "RB Opp", "RB1", "RUSH_YDS", "UNDER", 0.52),
            ],
            {"G2": CopulaGameContext("CIN", "BAL", -1.5, 51.5, True, True)},
        ),
        (
            "heavy_favorite_rb_spread",
            [
                CopulaLeg(1, "G3", "SF", "p6", "RB One", "RB1", "RUSH_YDS", "OVER", 0.60),
                CopulaLeg(2, "G3", "SF", "", "", "", "SPREAD", "OVER", 0.55),
                CopulaLeg(3, "G3", "SF", "p7", "QB SF", "QB", "PASS_YDS", "UNDER", 0.48),
            ],
            {"G3": CopulaGameContext("SF", "ARI", -9.5, 43.0, True, True)},
        ),
        (
            "within_player_rec_and_anytime_td",
            [
                CopulaLeg(1, "G4", "DAL", "p8", "WR Star", "WR1", "RECEPTIONS", "OVER", 0.57),
                CopulaLeg(2, "G4", "DAL", "p8", "WR Star", "WR1", "REC_YDS", "OVER", 0.53),
                CopulaLeg(3, "G4", "DAL", "p8", "WR Star", "WR1", "ANYTIME_TD", "YES", 0.41),
            ],
            {"G4": CopulaGameContext("DAL", "NYG", -5.5, 44.5, True, True)},
        ),
        (
            "cross_game_independence",
            [
                CopulaLeg(1, "G5", "DET", "p9", "QB Det", "QB", "PASS_YDS", "OVER", 0.55),
                CopulaLeg(2, "G6", "PHI", "p10", "RB Phi", "RB1", "RUSH_YDS", "OVER", 0.62),
            ],
            {
                "G5": CopulaGameContext("DET", "MIN", -3.0, 47.0, True, True),
                "G6": CopulaGameContext("PHI", "WSH", -4.5, 45.0, True, True),
            },
        ),
        (
            "same_game_total_and_team_total",
            [
                CopulaLeg(1, "G7", "MIA", "p11", "QB Mia", "QB", "PASS_TDS", "OVER", 0.53),
                CopulaLeg(2, "G7", "MIA", "", "", "", "TEAM_TOTAL", "OVER", 0.54),
                CopulaLeg(3, "G7", "MIA", "", "", "", "TOTAL", "OVER", 0.51),
            ],
            {"G7": CopulaGameContext("MIA", "NYJ", -6.0, 48.5, True, True)},
        ),
    ]

    copula_vectors: list[dict[str, Any]] = []
    for name, legs, ctx_map in copula_cases:
        est = copula_model.estimate_joint(legs, ctx_map)
        copula_vectors.append(
            {
                "id": name,
                "legs": [
                    {
                        "leg_id": leg.leg_id,
                        "game_id": leg.game_id,
                        "team": leg.team,
                        "player_id": leg.player_id,
                        "player_name": leg.player_name,
                        "role": leg.role,
                        "market": leg.market,
                        "side": leg.side,
                        "prob": leg.prob,
                    }
                    for leg in legs
                ],
                "game_context": {
                    gid: {
                        "home_team": c.home_team,
                        "away_team": c.away_team,
                        "home_spread": c.home_spread,
                        "game_total": c.game_total,
                        "has_spread": c.has_spread,
                        "has_total": c.has_total,
                    }
                    for gid, c in ctx_map.items()
                },
                "expected": est,
            }
        )

    return {
        "feature_schema_version": FEATURE_SCHEMA_VERSION,
        "leg_vectors": leg_vectors,
        "copula_vectors": copula_vectors,
    }


def render_validation_report(result: TrainingResult, bundle: dict[str, Any]) -> str:
    """Format a Markdown validation report covering walk-forward gates, KS drift, and pruning."""
    val = result.validation
    lines = [
        "# v2 Model Walk-Forward Validation Report",
        "",
        f"- **Version:** `{bundle['version']}`",
        f"- **Trained seasons:** `{result.trained_seasons}`",
        f"- **Held-out seasons:** `{result.held_out_seasons}`",
        f"- **Built at:** `{bundle['built_at']}`",
        "",
        "## Calibration & Scoring Gates",
        "",
        "| Metric | Value | Baseline / Target |",
        "| --- | --- | --- |",
        f"| Walk-forward Log Loss | `{val['log_loss']:.6f}` | Market `{val['market_log_loss']:.6f}` |",
        f"| Walk-forward Brier | `{val['brier_score']:.6f}` | Market `{val['market_brier_score']:.6f}` |",
        f"| Classwise ECE | `{val['classwise_ece']:.6f}` | Lower is better |",
        f"| Populated bins | `{val['populated_bins']}/{val['total_bins']}` | `>= 80%` |",
        f"| Copula SGP Log Loss | `{val['sgp_log_loss']:.6f}` | Naive `{val['naive_indep_log_loss']:.6f}` |",
        f"| EWMA half-life +/-1w MAE | `{val['stability_mae']:.6f}` | `<= 0.05` |",
        "",
        "## Per-Market Calibration & Collinearity Pruning",
        "",
        "| Market | Family | Blend Weight | Calibrator | Pruned Pairs (Spearman > 0.7) |",
        "| --- | --- | --- | --- | --- |",
    ]
    for market in sorted(result.prop_models.keys()):
        m = result.prop_models[market]
        pruned = result.pruned_features.get(market, [])
        pruned_txt = ", ".join(f"{p['dropped']} ({p['spearman']})" for p in pruned) or "none"
        lines.append(
            f"| `{market}` | `{m.family}` | `{m.market_blend_weight:.3f}` | `{m.calibrator.method}` | {pruned_txt} |"
        )

    lines.extend(["", "## Top Covariate Shifts (KS Two-Sample Test)", "", "| Market | Feature | KS Stat | p-value | Drifted |", "| --- | --- | --- | --- | --- |"])
    for market in sorted(result.ks_summary.keys()):
        for row in result.ks_summary[market][:3]:
            lines.append(
                f"| `{market}` | `{row['feature']}` | `{row['ks_stat']:.4f}` | `{row['p_value']:.4g}` | `{row['drifted']}` |"
            )
    lines.append("")
    return "\n".join(lines)


def export_pipeline_outputs(
    result: TrainingResult,
    artifact_path: Path = DEFAULT_ARTIFACT_OUT,
    parity_path: Path = DEFAULT_PARITY_OUT,
    report_path: Path = DEFAULT_REPORT_OUT,
    version: str = "v2.1.0",
) -> dict[str, Any]:
    """Validate and write v2.json, parity_vectors.json, and Markdown validation report."""
    bundle = build_artifact_bundle_dict(result, version=version)
    validate_artifact_bundle_dict(bundle)

    parity = generate_parity_vectors(bundle, result)
    report = render_validation_report(result, bundle)

    artifact_path.parent.mkdir(parents=True, exist_ok=True)
    parity_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.parent.mkdir(parents=True, exist_ok=True)

    artifact_path.write_text(json.dumps(bundle, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    parity_path.write_text(json.dumps(parity, indent=2, allow_nan=False) + "\n", encoding="utf-8")
    report_path.write_text(report, encoding="utf-8")
    return bundle


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description="Train and export Parlay Club v2 model artifacts.")
    parser.add_argument("--seasons", default="2019-2025", help="Season range like 2019-2025 or comma list 2020,2021,2022,2023,2024,2025")
    parser.add_argument("--trials", type=int, default=8, help="Optuna HPO trials per market per fold")
    parser.add_argument("--version", default="v2.1.0", help="Artifact semantic version")
    parser.add_argument("--refresh", action="store_true", help="Refresh cached nflverse datasets")
    parser.add_argument("--artifact-out", type=Path, default=DEFAULT_ARTIFACT_OUT)
    parser.add_argument("--parity-out", type=Path, default=DEFAULT_PARITY_OUT)
    parser.add_argument("--report-out", type=Path, default=DEFAULT_REPORT_OUT)
    args = parser.parse_args(argv)

    if "-" in args.seasons:
        start_s, end_s = args.seasons.split("-", 1)
        seasons = list(range(int(start_s), int(end_s) + 1))
    else:
        seasons = [int(s.strip()) for s in args.seasons.split(",") if s.strip()]

    player_df, team_df = load_historical_tables(seasons, refresh=args.refresh)
    result = run_walk_forward_training(player_df, team_df, seasons=seasons, n_trials=args.trials)
    export_pipeline_outputs(
        result,
        artifact_path=args.artifact_out,
        parity_path=args.parity_out,
        report_path=args.report_out,
        version=args.version,
    )
    print(f"Exported {args.artifact_out} (log_loss={result.validation['log_loss']:.4f}, sgp_ll={result.validation['sgp_log_loss']:.4f})")


if __name__ == "__main__":
    main()
