"""Walk-forward distributional GBDT training, Optuna HPO, calibration, blend tuning, and validation gates."""

from __future__ import annotations

from dataclasses import dataclass, field
import math
from typing import Any

import numpy as np
import optuna
import pandas as pd
from sklearn.tree import DecisionTreeRegressor

from copula import CopulaModel, estimate_copula_artifact, evaluate_sgp_log_loss
from distributions import (
    FAMILY_BERNOULLI,
    FAMILY_NEGBIN,
    FAMILY_POISSON,
    FAMILY_ZERO_ADJ_GAMMA,
    REQUIRED_PARAMS,
    Calibrator,
    binary_log_loss,
    brier_score,
    dist_stats_scalar,
    eta_to_params,
    fit_isotonic_calibrator,
    fit_platt_calibrator,
    initial_base_scores,
    nll_and_grad_hess,
    reliability_summary,
    survival_array,
    survival_scalar,
)
from features import (
    EWMA_HALF_LIFE_WEEKS,
    FEATURE_SCHEMA_VERSION,
    MARKET_SPECS,
    MarketSpec,
    apply_eb_shrinkage_df,
    build_leak_free_features,
    compute_ks_shift_report,
    fit_role_priors,
    prune_collinear_features,
)

optuna.logging.set_verbosity(optuna.logging.WARNING)


@dataclass
class FlatTree:
    """Flattened GBDT tree matching Go internal/modelv2/gbdt.go (val < threshold -> Left)."""

    nodes: list[dict[str, Any]]

    def eval_scalar(self, features: list[float]) -> float:
        n = len(self.nodes)
        if n == 0:
            return 0.0
        idx = 0
        for _ in range(n):
            node = self.nodes[idx]
            feat = int(node["feature"])
            if feat < 0:
                return float(node["leaf"])
            if feat >= len(features) or math.isnan(features[feat]):
                idx = int(node["missing"])
            elif features[feat] < float(node["threshold"]):
                idx = int(node["left"])
            else:
                idx = int(node["right"])
            if idx < 0 or idx >= n:
                return 0.0
        return 0.0

    def eval_batch(self, x: np.ndarray) -> np.ndarray:
        n_rows = x.shape[0]
        if not self.nodes or n_rows == 0:
            return np.zeros(n_rows, dtype=np.float64)
        feat_arr = np.array([int(nd["feature"]) for nd in self.nodes], dtype=np.int64)
        thresh_arr = np.array([float(nd["threshold"]) for nd in self.nodes], dtype=np.float64)
        left_arr = np.array([int(nd["left"]) for nd in self.nodes], dtype=np.int64)
        right_arr = np.array([int(nd["right"]) for nd in self.nodes], dtype=np.int64)
        miss_arr = np.array([int(nd["missing"]) for nd in self.nodes], dtype=np.int64)
        leaf_arr = np.array([float(nd["leaf"]) for nd in self.nodes], dtype=np.float64)

        idx = np.zeros(n_rows, dtype=np.int64)
        n_cols = x.shape[1]
        for _ in range(len(self.nodes)):
            f = feat_arr[idx]
            is_leaf = f < 0
            if np.all(is_leaf):
                break
            act = np.flatnonzero(~is_leaf)
            cur = idx[act]
            fa = f[act]
            in_bounds = fa < n_cols
            vals = np.full(len(act), np.nan, dtype=np.float64)
            vals[in_bounds] = x[act[in_bounds], fa[in_bounds]]
            is_nan = np.isnan(vals)
            go_left = (~is_nan) & (vals < thresh_arr[cur])
            idx[act] = np.where(is_nan, miss_arr[cur], np.where(go_left, left_arr[cur], right_arr[cur]))
        return leaf_arr[idx]

    def to_dict(self) -> dict[str, Any]:
        return {"nodes": self.nodes}


@dataclass
class GBDTModel:
    base_score: float
    trees: list[FlatTree]

    def eval_scalar(self, features: list[float]) -> float:
        return self.base_score + sum(t.eval_scalar(features) for t in self.trees)

    def eval_batch(self, x: np.ndarray) -> np.ndarray:
        out = np.full(x.shape[0], self.base_score, dtype=np.float64)
        for t in self.trees:
            out += t.eval_batch(x)
        return out

    def to_dict(self) -> dict[str, Any]:
        return {
            "base_score": float(self.base_score),
            "trees": [t.to_dict() for t in self.trees],
        }


def _sklearn_tree_to_flat(
    dt: DecisionTreeRegressor,
    col_map: np.ndarray,
    leaf_values: dict[int, float],
) -> FlatTree:
    """Convert fitted sklearn DecisionTreeRegressor into Go FlatTree (< threshold -> left)."""
    tree = dt.tree_
    nodes: list[dict[str, Any]] = []
    for i in range(tree.node_count):
        left = int(tree.children_left[i])
        right = int(tree.children_right[i])
        if left == right:
            nodes.append(
                {
                    "feature": -1,
                    "threshold": 0.0,
                    "left": 0,
                    "right": 0,
                    "missing": 0,
                    "leaf": float(leaf_values.get(i, 0.0)),
                }
            )
        else:
            raw_thresh = float(tree.threshold[i])
            if math.isnan(raw_thresh):
                raise ValueError(f"node {i}: unexpected NaN threshold")
            if math.isinf(raw_thresh):
                # sklearn uses +/-inf when splitting missing vs non-missing values.
                # Replace with a finite sentinel outside real feature ranges so Go JSON & Validate accept it.
                go_thresh = 1e300 if raw_thresh > 0 else -1e300
            else:
                # sklearn routes val <= raw_thresh to left; Go routes val < threshold to left.
                go_thresh = float(np.nextafter(raw_thresh, np.inf))
                if not math.isfinite(go_thresh):
                    go_thresh = 1e300
            go_left = bool(tree.missing_go_to_left[i])
            nodes.append(
                {
                    "feature": int(col_map[int(tree.feature[i])]),
                    "threshold": go_thresh,
                    "left": left,
                    "right": right,
                    "missing": left if go_left else right,
                    "leaf": 0.0,
                }
            )
    return FlatTree(nodes=nodes)


@dataclass
class DistributionalGBDT:
    """Multi-parameter 2nd-order Newton-Raphson GBDT exporting directly to Go DistModelArtifact."""

    market: str
    family: str
    feature_names: list[str]
    param_models: dict[str, GBDTModel]
    calibrator: Calibrator = field(default_factory=lambda: Calibrator(method="platt", platt_a=1.0, platt_b=0.0))
    market_blend_weight: float = 0.80

    def predict_eta(self, x: np.ndarray) -> dict[str, np.ndarray]:
        return {p: model.eval_batch(x) for p, model in self.param_models.items()}

    def predict_params(self, x: np.ndarray) -> dict[str, np.ndarray]:
        return eta_to_params(self.family, self.predict_eta(x))

    def predict_leg_scalar(
        self,
        features: list[float],
        side: str,
        line: float,
        market_prob: float,
        uncertainty_boost: float = 0.0,
    ) -> dict[str, Any]:
        """Exact scalar prediction matching Go DistModelArtifact.PredictLeg."""
        eta = {p: np.array([model.eval_scalar(features)]) for p, model in self.param_models.items()}
        params_arr = eta_to_params(self.family, eta)
        params = {k: float(v[0]) for k, v in params_arr.items()}
        p_over = survival_scalar(self.family, params, line)
        stats = dist_stats_scalar(self.family, params)

        side_u = side.strip().upper() or "OVER"
        is_under = side_u in ("UNDER", "NO")
        has_market = not math.isnan(market_prob) and not math.isinf(market_prob) and 0.0 < market_prob < 1.0
        w_eff = 0.0
        blended_over = p_over
        if has_market:
            w = min(max(self.market_blend_weight, 0.0), 1.0)
            w_eff = min(0.95, w + uncertainty_boost) if uncertainty_boost > 0.0 else w
            m_over = (1.0 - market_prob) if is_under else market_prob
            blended_over = w_eff * m_over + (1.0 - w_eff) * p_over

        cal_over = float(self.calibrator.calibrate(blended_over))
        model_prob = 1.0 - p_over if is_under else p_over
        blended_prob = 1.0 - blended_over if is_under else blended_over
        cal_prob = min(max(1.0 - cal_over, 1e-6), 1.0 - 1e-6) if is_under else cal_over
        return {
            "market": self.market,
            "family": self.family,
            "side": side_u,
            "line": float(line),
            "stats": {
                "mean": stats.mean,
                "variance": stats.variance,
                "std_dev": stats.std_dev,
                "cv": stats.cv,
                "skewness": stats.skewness,
            },
            "model_prob": float(model_prob),
            "market_prob": float(market_prob) if has_market else 0.0,
            "blend_weight": float(w_eff),
            "blended_prob": float(blended_prob),
            "calibrated_prob": float(cal_prob),
        }

    def to_dict(self) -> dict[str, Any]:
        return {
            "market": self.market,
            "family": self.family,
            "feature_names": list(self.feature_names),
            "param_models": {k: v.to_dict() for k, v in self.param_models.items()},
            "calibrator": self.calibrator.to_dict(),
            "market_blend_weight": float(self.market_blend_weight),
        }


def fit_distributional_gbdt(
    market: str,
    family: str,
    feature_names: list[str],
    x_train: np.ndarray,
    y_train: np.ndarray,
    x_val: np.ndarray | None = None,
    y_val: np.ndarray | None = None,
    params: dict[str, Any] | None = None,
    seed: int = 42,
) -> tuple[DistributionalGBDT, float]:
    """Fit a multi-parameter 2nd-order distributional GBDT with optional early stopping."""
    cfg = {
        "n_estimators": 40,
        "learning_rate": 0.08,
        "max_depth": 3,
        "min_samples_leaf": 25,
        "reg_lambda": 5.0,
        "subsample": 0.85,
        "colsample_bytree": 0.85,
        "early_stopping_rounds": 8,
    }
    if params:
        cfg.update(params)

    rng = np.random.default_rng(seed)
    base = initial_base_scores(family, y_train)
    req_params = REQUIRED_PARAMS[family]
    trees_by_param: dict[str, list[FlatTree]] = {p: [] for p in req_params}

    eta_tr = {p: np.full(len(y_train), base[p], dtype=np.float64) for p in req_params}
    eta_va = {p: np.full(len(y_val), base[p], dtype=np.float64) for p in req_params} if x_val is not None else None

    best_val_nll = math.inf
    best_round = 0
    no_improve = 0
    n_rows, n_cols = x_train.shape
    sub_n = max(1, int(round(float(cfg["subsample"]) * n_rows)))
    col_n = max(1, int(round(float(cfg["colsample_bytree"]) * n_cols)))
    lr = float(cfg["learning_rate"])
    reg_lam = float(cfg["reg_lambda"])

    for round_idx in range(int(cfg["n_estimators"])):
        _, grads, hess = nll_and_grad_hess(family, y_train, eta_tr)
        for p in req_params:
            g = grads[p]
            h = np.maximum(hess[p], 1e-4)

            if sub_n < n_rows:
                row_idx = rng.choice(n_rows, size=sub_n, replace=False)
            else:
                row_idx = np.arange(n_rows)
            if col_n < n_cols:
                col_idx = np.sort(rng.choice(n_cols, size=col_n, replace=False))
            else:
                col_idx = np.arange(n_cols)

            x_sub = x_train[row_idx][:, col_idx]
            g_sub = g[row_idx]
            h_sub = h[row_idx]
            target_sub = -g_sub / h_sub

            dt = DecisionTreeRegressor(
                max_depth=int(cfg["max_depth"]),
                min_samples_leaf=max(5, int(cfg["min_samples_leaf"])),
                random_state=int(seed + round_idx * 17),
            )
            dt.fit(x_sub, target_sub, sample_weight=h_sub)

            leaf_ids = dt.apply(x_sub)
            leaf_values: dict[int, float] = {}
            for lid in np.unique(leaf_ids):
                mask = leaf_ids == lid
                sum_g = float(np.sum(g_sub[mask]))
                sum_h = float(np.sum(h_sub[mask]))
                # Shrink Newton step with L2 penalty + learning rate; clamp step magnitude for stability.
                step = -sum_g / (sum_h + reg_lam)
                step = float(np.clip(step, -1.5, 1.5))
                leaf_values[int(lid)] = lr * step

            flat_tree = _sklearn_tree_to_flat(dt, col_idx, leaf_values)
            trees_by_param[p].append(flat_tree)
            eta_tr[p] = np.clip(eta_tr[p] + flat_tree.eval_batch(x_train), -15.0, 15.0)
            if x_val is not None and eta_va is not None:
                eta_va[p] = np.clip(eta_va[p] + flat_tree.eval_batch(x_val), -15.0, 15.0)

        if x_val is not None and y_val is not None and eta_va is not None:
            val_nll, _, _ = nll_and_grad_hess(family, y_val, eta_va)
            if val_nll + 1e-5 < best_val_nll:
                best_val_nll = val_nll
                best_round = round_idx + 1
                no_improve = 0
            else:
                no_improve += 1
                if no_improve >= int(cfg["early_stopping_rounds"]):
                    break

    if x_val is None:
        best_val_nll, _, _ = nll_and_grad_hess(family, y_train, eta_tr)
        best_round = len(trees_by_param[req_params[0]])

    # Truncate to best_round (ensuring at least 1 tree per parameter so Go GBDTModel.Validate passes)
    keep_rounds = max(1, best_round)
    param_models = {
        p: GBDTModel(base_score=base[p], trees=trees_by_param[p][:keep_rounds]) for p in req_params
    }
    model = DistributionalGBDT(
        market=market,
        family=family,
        feature_names=feature_names,
        param_models=param_models,
    )
    return model, float(best_val_nll)


def _baseline_market_grid(
    df: pd.DataFrame,
    spec: MarketSpec,
    priors: dict[str, dict[str, Any]],
) -> pd.DataFrame:
    """Construct causal pre-game prop evaluation lines and market-proxy probabilities.

    Lines span standard expectations plus alt lines so reliability bins cover [0.15, 0.85]
    without peeking at future games.
    """
    shrunk = apply_eb_shrinkage_df(df, spec.market, priors)
    anchor_col = spec.ewma_anchor_col
    if spec.market == "ANYTIME_TD":
        anchor = (
            shrunk["ewma_exp_rush_tds"].fillna(0.0) + shrunk["ewma_exp_rec_tds"].fillna(0.0)
        ).to_numpy(dtype=np.float64)
        anchor = np.clip(1.0 - np.exp(-np.maximum(anchor, 0.05)), 0.05, 0.75)
    else:
        anchor = shrunk[anchor_col].fillna(0.0).to_numpy(dtype=np.float64)

    y_actual = shrunk[spec.outcome_col].to_numpy(dtype=np.float64)
    # Adjust slightly for team implied total vs league avg ~22.5 to mirror sportsbook line shading.
    imp_tot = shrunk["team_implied_total"].fillna(22.5).to_numpy(dtype=np.float64)
    scale = np.clip(imp_tot / 22.5, 0.75, 1.30)
    mu_mkt = np.maximum(anchor * scale, 0.10 if spec.is_discrete else 1.0)

    if spec.market == "ANYTIME_TD":
        # Binary market has threshold 0.5; market probability has realistic mild shading/dispersion.
        p_mkt = np.clip(0.85 * anchor * scale + 0.15 * float(np.mean(y_actual > 0.5)), 0.04, 0.85)
        out = shrunk[["game_id", "player_id", "player_name", "position", "role", "team", "opponent", "season", "week", "games_played"]].copy()
        out["line"] = 0.5
        out["y_hit"] = (y_actual > 0.5).astype(np.float64)
        out["p_market"] = p_mkt
        out["orig_idx"] = np.arange(len(shrunk))
        return out

    # Generate 3 half-point thresholds per player-week: low alt, main line, high alt
    rows: list[pd.DataFrame] = []
    if spec.is_discrete:
        multipliers = (0.75, 1.0, 1.35)
        for mult in multipliers:
            lines = np.maximum(0.5, np.floor(mu_mkt * mult) + 0.5)
            # Market baseline uses causal Poisson/NB approximation centered on mu_mkt
            disp = 2.5 if spec.family == FAMILY_NEGBIN else 100.0
            q = mu_mkt / (disp + mu_mkt)
            from scipy import special

            p_mkt = np.clip(special.betainc(np.floor(lines) + 1.0, disp, q), 0.03, 0.97)
            chunk = shrunk[["game_id", "player_id", "player_name", "position", "role", "team", "opponent", "season", "week", "games_played"]].copy()
            chunk["line"] = lines
            chunk["y_hit"] = (y_actual > lines).astype(np.float64)
            chunk["p_market"] = p_mkt
            chunk["orig_idx"] = np.arange(len(shrunk))
            rows.append(chunk)
    else:
        multipliers = (0.72, 1.0, 1.32)
        shape_k = 4.0 if spec.market == "PASS_YDS" else 2.0
        for mult in multipliers:
            lines = np.maximum(4.5, np.floor(mu_mkt * mult) + 0.5)
            from scipy import special

            p_mkt = np.clip(0.92 * special.gammaincc(shape_k, lines * (shape_k / np.maximum(mu_mkt, 1.0))), 0.03, 0.97)
            chunk = shrunk[["game_id", "player_id", "player_name", "position", "role", "team", "opponent", "season", "week", "games_played"]].copy()
            chunk["line"] = lines
            chunk["y_hit"] = (y_actual > lines).astype(np.float64)
            chunk["p_market"] = p_mkt
            chunk["orig_idx"] = np.arange(len(shrunk))
            rows.append(chunk)

    return pd.concat(rows, ignore_index=True)


def tune_blend_and_calibrator(
    p_model: np.ndarray,
    p_market: np.ndarray,
    y_hit: np.ndarray,
) -> tuple[float, Calibrator, float]:
    """Tune blend weight w in [0.70, 0.85] and select held-out Platt vs Isotonic calibrator."""
    n = len(y_hit)
    if n < 40:
        return 0.80, Calibrator(method="platt", platt_a=1.0, platt_b=0.0), binary_log_loss(p_market, y_hit)

    # Split calibration slice chronologically: first 60% fits blend & calibrators, last 40% selects method.
    split = max(20, int(0.60 * n))
    pm_a, mk_a, y_a = p_model[:split], p_market[:split], y_hit[:split]
    pm_b, mk_b, y_b = p_model[split:], p_market[split:], y_hit[split:]

    best_w = 0.80
    best_ll = math.inf
    for w in np.linspace(0.70, 0.85, 16):
        blend_a = w * mk_a + (1.0 - w) * pm_a
        ll = binary_log_loss(blend_a, y_a) + 1e-4 * ((w - 0.80) ** 2)
        if ll < best_ll:
            best_ll = ll
            best_w = float(w)

    blend_a = best_w * mk_a + (1.0 - best_w) * pm_a
    blend_b = best_w * mk_b + (1.0 - best_w) * pm_b

    platt = fit_platt_calibrator(blend_a, y_a)
    iso = fit_isotonic_calibrator(blend_a, y_a)

    ll_platt = binary_log_loss(platt.calibrate(blend_b), y_b)
    ll_iso = binary_log_loss(iso.calibrate(blend_b), y_b)

    # Refit winning method on full calibration slice so final calibrator uses all chronological cal data.
    blend_full = best_w * p_market + (1.0 - best_w) * p_model
    if ll_iso + 0.002 < ll_platt and n >= 300:
        cal = fit_isotonic_calibrator(blend_full, y_hit)
        chosen_ll = ll_iso
    else:
        cal = fit_platt_calibrator(blend_full, y_hit)
        chosen_ll = ll_platt

    return best_w, cal, float(chosen_ll)


def build_walk_forward_folds(seasons: list[int], min_held_out: int = 3) -> list[dict[str, Any]]:
    """Build strict expanding-window walk-forward splits with >= min_held_out test seasons."""
    ordered = sorted({int(s) for s in seasons})
    if len(ordered) < min_held_out + 1:
        raise ValueError(
            f"need at least {min_held_out + 1} NFL seasons for {min_held_out} expanding-window held-out folds, got {ordered}"
        )
    test_seasons = ordered[-min_held_out:]
    folds: list[dict[str, Any]] = []
    for test_s in test_seasons:
        prior = [s for s in ordered if s < test_s]
        folds.append({"train_seasons": prior, "test_season": test_s})
    return folds


def _split_train_fit_and_cal(df: pd.DataFrame) -> tuple[pd.DataFrame, pd.DataFrame]:
    """Chronologically split pre-test rows into tree-fitting and held-out calibration/HPO slices."""
    seasons = sorted(df["season"].unique().tolist())
    if len(seasons) >= 2:
        cal_season = seasons[-1]
        return df[df["season"] < cal_season].copy(), df[df["season"] == cal_season].copy()
    ordered = df.sort_values(["season", "season_type", "week"]).reset_index(drop=True)
    split = max(1, int(0.65 * len(ordered)))
    return ordered.iloc[:split].copy(), ordered.iloc[split:].copy()


def _optimize_market_hpo(
    spec: MarketSpec,
    features: list[str],
    x_tr: np.ndarray,
    y_tr: np.ndarray,
    x_cal: np.ndarray,
    y_cal: np.ndarray,
    n_trials: int,
    seed: int,
) -> dict[str, Any]:
    """Run Optuna TPE hyperparameter search minimizing distributional NLL on the validation slice."""
    if n_trials <= 1 or len(y_tr) < 50 or len(y_cal) < 30:
        return {
            "n_estimators": 35,
            "learning_rate": 0.08,
            "max_depth": 3,
            "min_samples_leaf": 25,
            "reg_lambda": 5.0,
            "subsample": 0.85,
            "colsample_bytree": 0.85,
        }

    def objective(trial: optuna.Trial) -> float:
        params = {
            "n_estimators": trial.suggest_int("n_estimators", 20, 60),
            "learning_rate": trial.suggest_float("learning_rate", 0.03, 0.14, log=True),
            "max_depth": trial.suggest_int("max_depth", 2, 4),
            "min_samples_leaf": trial.suggest_int("min_samples_leaf", 15, 60),
            "reg_lambda": trial.suggest_float("reg_lambda", 1.0, 15.0, log=True),
            "subsample": trial.suggest_float("subsample", 0.70, 1.0),
            "colsample_bytree": trial.suggest_float("colsample_bytree", 0.70, 1.0),
            "early_stopping_rounds": 6,
        }
        _, val_nll = fit_distributional_gbdt(
            spec.market,
            spec.family,
            features,
            x_tr,
            y_tr,
            x_cal,
            y_cal,
            params=params,
            seed=seed,
        )
        return val_nll

    sampler = optuna.samplers.TPESampler(seed=seed)
    study = optuna.create_study(direction="minimize", sampler=sampler)
    study.optimize(objective, n_trials=n_trials)
    best = dict(study.best_params)
    best["early_stopping_rounds"] = 8
    return best


def _train_and_eval_market_fold(
    spec: MarketSpec,
    train_df: pd.DataFrame,
    test_df: pd.DataFrame,
    priors: dict[str, dict[str, Any]],
    n_trials: int,
    seed: int,
) -> dict[str, Any]:
    tr_sub = train_df[train_df["position"].isin(spec.eligible_positions)].copy()
    te_sub = test_df[test_df["position"].isin(spec.eligible_positions)].copy()
    tr_fit_raw, tr_cal_raw = _split_train_fit_and_cal(tr_sub)

    tr_fit = apply_eb_shrinkage_df(tr_fit_raw, spec.market, priors)
    tr_cal = apply_eb_shrinkage_df(tr_cal_raw, spec.market, priors)
    te_eval = apply_eb_shrinkage_df(te_sub, spec.market, priors)

    kept_features, pruned_log = prune_collinear_features(
        tr_fit, list(spec.candidate_features), spec.outcome_col, threshold=0.7
    )
    ks_report = compute_ks_shift_report(tr_fit, te_eval, kept_features)

    x_fit = tr_fit[kept_features].to_numpy(dtype=np.float64)
    y_fit = tr_fit[spec.outcome_col].to_numpy(dtype=np.float64)
    x_cal = tr_cal[kept_features].to_numpy(dtype=np.float64)
    y_cal = tr_cal[spec.outcome_col].to_numpy(dtype=np.float64)
    x_te = te_eval[kept_features].to_numpy(dtype=np.float64)

    best_hp = _optimize_market_hpo(spec, kept_features, x_fit, y_fit, x_cal, y_cal, n_trials=n_trials, seed=seed)
    model, val_nll = fit_distributional_gbdt(
        spec.market, spec.family, kept_features, x_fit, y_fit, x_cal, y_cal, params=best_hp, seed=seed
    )

    # Calibrate & tune blend weight on strictly held-out chronological tr_cal_raw
    cal_grid = _baseline_market_grid(tr_cal_raw, spec, priors)
    cal_params = model.predict_params(x_cal)
    cal_row_params = {k: v[cal_grid["orig_idx"].to_numpy()] for k, v in cal_params.items()}
    p_model_cal = survival_array(spec.family, cal_row_params, cal_grid["line"].to_numpy(dtype=np.float64))
    w_blend, calibrator, _ = tune_blend_and_calibrator(
        p_model_cal,
        cal_grid["p_market"].to_numpy(dtype=np.float64),
        cal_grid["y_hit"].to_numpy(dtype=np.float64),
    )
    model.market_blend_weight = w_blend
    model.calibrator = calibrator

    # Evaluate on held-out test_season
    te_grid = _baseline_market_grid(te_sub, spec, priors)
    te_params = model.predict_params(x_te)
    te_row_params = {k: v[te_grid["orig_idx"].to_numpy()] for k, v in te_params.items()}
    p_model_te = survival_array(spec.family, te_row_params, te_grid["line"].to_numpy(dtype=np.float64))
    p_mkt_te = te_grid["p_market"].to_numpy(dtype=np.float64)
    y_te = te_grid["y_hit"].to_numpy(dtype=np.float64)
    p_blend_te = w_blend * p_mkt_te + (1.0 - w_blend) * p_model_te
    p_cal_te = calibrator.calibrate(p_blend_te)

    # Collect main-line prediction lookup for SGP evaluation
    main_lines = te_grid.iloc[len(te_sub) : 2 * len(te_sub)] if len(te_grid) >= 2 * len(te_sub) else te_grid
    main_probs = p_cal_te[len(te_sub) : 2 * len(te_sub)] if len(te_grid) >= 2 * len(te_sub) else p_cal_te
    sgp_leg_rows = main_lines[["game_id", "player_id"]].copy()
    sgp_leg_rows["market"] = spec.market
    sgp_leg_rows["prob"] = main_probs

    return {
        "model": model,
        "best_hp": best_hp,
        "val_nll": val_nll,
        "pruned_features": pruned_log,
        "ks_report": ks_report,
        "p_cal": p_cal_te,
        "p_market": p_mkt_te,
        "y_hit": y_te,
        "sgp_leg_rows": sgp_leg_rows,
    }


@dataclass
class TrainingResult:
    trained_seasons: list[int]
    held_out_seasons: list[int]
    prop_models: dict[str, DistributionalGBDT]
    priors: dict[str, dict[str, Any]]
    copula: CopulaModel
    validation: dict[str, Any]
    ks_summary: dict[str, list[dict[str, Any]]]
    pruned_features: dict[str, list[dict[str, Any]]]
    stability_mae: float


def run_walk_forward_training(
    player_df: Any,
    team_df: Any,
    seasons: list[int],
    n_trials: int = 8,
    min_held_out_seasons: int = 3,
    seed: int = 42,
    enforce_gates: bool = True,
) -> TrainingResult:
    """Execute end-to-end expanding-window walk-forward training, calibration, copula, and gate checks."""
    feat_df = build_leak_free_features(player_df, team_df, half_life=EWMA_HALF_LIFE_WEEKS)
    folds = build_walk_forward_folds(seasons, min_held_out=min_held_out_seasons)

    oos_p_cal: list[np.ndarray] = []
    oos_p_mkt: list[np.ndarray] = []
    oos_y: list[np.ndarray] = []
    sgp_joint_total = 0
    sgp_joint_ll_sum = 0.0
    sgp_indep_ll_sum = 0.0

    last_hp_by_market: dict[str, dict[str, Any]] = {}
    last_features_by_market: dict[str, list[str]] = {}
    ks_by_market: dict[str, list[dict[str, Any]]] = {}
    pruned_by_market: dict[str, list[dict[str, Any]]] = {}

    for fold in folds:
        tr_seasons = fold["train_seasons"]
        te_season = fold["test_season"]
        tr_df = feat_df[feat_df["season"].isin(tr_seasons)].copy()
        te_df = feat_df[feat_df["season"] == te_season].copy()

        priors = fit_role_priors(tr_df)
        copula_fold = estimate_copula_artifact(tr_df)
        fold_sgp_legs: list[pd.DataFrame] = []

        for market, spec in MARKET_SPECS.items():
            res = _train_and_eval_market_fold(spec, tr_df, te_df, priors, n_trials=n_trials, seed=seed)
            oos_p_cal.append(res["p_cal"])
            oos_p_mkt.append(res["p_market"])
            oos_y.append(res["y_hit"])
            fold_sgp_legs.append(res["sgp_leg_rows"])
            last_hp_by_market[market] = res["best_hp"]
            last_features_by_market[market] = res["model"].feature_names
            ks_by_market[market] = res["ks_report"]
            pruned_by_market[market] = res["pruned_features"]

        sgp_df = pd.concat(fold_sgp_legs, ignore_index=True) if fold_sgp_legs else pd.DataFrame()
        sgp_metrics = evaluate_sgp_log_loss(copula_fold, te_df, sgp_df)
        cnt = int(sgp_metrics["sgp_count"])
        if cnt > 0:
            sgp_joint_total += cnt
            sgp_joint_ll_sum += cnt * float(sgp_metrics["sgp_log_loss"])
            sgp_indep_ll_sum += cnt * float(sgp_metrics["naive_indep_log_loss"])

    all_p_cal = np.concatenate(oos_p_cal)
    all_p_mkt = np.concatenate(oos_p_mkt)
    all_y = np.concatenate(oos_y)

    model_ll = binary_log_loss(all_p_cal, all_y)
    model_brier = brier_score(all_p_cal, all_y)
    mkt_ll = binary_log_loss(all_p_mkt, all_y)
    mkt_brier = brier_score(all_p_mkt, all_y)
    rel = reliability_summary(all_p_cal, all_y, n_bins=10)

    sgp_ll = sgp_joint_ll_sum / sgp_joint_total if sgp_joint_total > 0 else 0.0
    naive_ll = sgp_indep_ll_sum / sgp_joint_total if sgp_joint_total > 0 else 0.0

    # Fit final artifacts chronologically on full history (tree fit on earlier, cal/blend on latest season)
    final_priors = fit_role_priors(feat_df)
    final_copula = estimate_copula_artifact(feat_df)
    final_models: dict[str, DistributionalGBDT] = {}

    for market, spec in MARKET_SPECS.items():
        sub = feat_df[feat_df["position"].isin(spec.eligible_positions)].copy()
        fit_raw, cal_raw = _split_train_fit_and_cal(sub)
        fit_df = apply_eb_shrinkage_df(fit_raw, market, final_priors)
        cal_df = apply_eb_shrinkage_df(cal_raw, market, final_priors)
        feats = last_features_by_market[market]
        hp = last_hp_by_market[market]
        x_fit = fit_df[feats].to_numpy(dtype=np.float64)
        y_fit = fit_df[spec.outcome_col].to_numpy(dtype=np.float64)
        x_cal = cal_df[feats].to_numpy(dtype=np.float64)
        y_cal = cal_df[spec.outcome_col].to_numpy(dtype=np.float64)

        model, _ = fit_distributional_gbdt(
            market, spec.family, feats, x_fit, y_fit, x_cal, y_cal, params=hp, seed=seed
        )
        cal_grid = _baseline_market_grid(cal_raw, spec, final_priors)
        cal_params = model.predict_params(x_cal)
        cal_row_params = {k: v[cal_grid["orig_idx"].to_numpy()] for k, v in cal_params.items()}
        p_model_cal = survival_array(spec.family, cal_row_params, cal_grid["line"].to_numpy(dtype=np.float64))
        w_blend, calibrator, _ = tune_blend_and_calibrator(
            p_model_cal,
            cal_grid["p_market"].to_numpy(dtype=np.float64),
            cal_grid["y_hit"].to_numpy(dtype=np.float64),
        )
        model.market_blend_weight = w_blend
        model.calibrator = calibrator
        final_models[market] = model

    # Stability check: perturb EWMA half-life +/- 1 week on the latest held-out season
    latest_season = folds[-1]["test_season"]
    stability_mae = check_ewma_half_life_stability(
        player_df=player_df,
        team_df=team_df,
        base_feat_df=feat_df[feat_df["season"] == latest_season],
        models=final_models,
        priors=final_priors,
        target_season=latest_season,
    )

    held_out = [int(f["test_season"]) for f in folds]
    validation = {
        "held_out_seasons": held_out,
        "log_loss": round(model_ll, 6),
        "brier_score": round(model_brier, 6),
        "market_log_loss": round(mkt_ll, 6),
        "market_brier_score": round(mkt_brier, 6),
        "sgp_log_loss": round(sgp_ll, 6),
        "naive_indep_log_loss": round(naive_ll, 6),
        "populated_bins": int(rel["populated_bins"]),
        "total_bins": int(rel["total_bins"]),
        "classwise_ece": round(float(rel["ece"]), 6),
        "stability_mae": round(stability_mae, 6),
        "notes": (
            f"Walk-forward across {len(held_out)} held-out seasons ({held_out}); "
            f"feature_schema_version={FEATURE_SCHEMA_VERSION}"
        ),
    }

    if enforce_gates:
        _assert_pipeline_gates(validation, sgp_joint_total)

    return TrainingResult(
        trained_seasons=sorted({int(s) for s in seasons}),
        held_out_seasons=held_out,
        prop_models=final_models,
        priors=final_priors,
        copula=final_copula,
        validation=validation,
        ks_summary=ks_by_market,
        pruned_features=pruned_by_market,
        stability_mae=stability_mae,
    )


def check_ewma_half_life_stability(
    player_df: Any,
    team_df: Any,
    base_feat_df: pd.DataFrame,
    models: dict[str, DistributionalGBDT],
    priors: dict[str, dict[str, Any]],
    target_season: int,
) -> float:
    """Measure MAE in predicted probabilities when EWMA half-life shifts by +/- 1 week."""
    season_p = player_df.filter(player_df["season"] == target_season)
    season_t = team_df.filter(team_df["season"] == target_season)
    diffs: list[float] = []

    for alt_hl in (EWMA_HALF_LIFE_WEEKS - 1.0, EWMA_HALF_LIFE_WEEKS + 1.0):
        alt_feat_df = build_leak_free_features(season_p, season_t, half_life=alt_hl)
        for market, spec in MARKET_SPECS.items():
            model = models[market]
            base_sub = apply_eb_shrinkage_df(
                base_feat_df[base_feat_df["position"].isin(spec.eligible_positions)], market, priors
            )
            alt_sub = apply_eb_shrinkage_df(
                alt_feat_df[alt_feat_df["position"].isin(spec.eligible_positions)], market, priors
            )
            if base_sub.empty or alt_sub.empty:
                continue
            merged = base_sub.merge(
                alt_sub,
                on=["game_id", "player_id"],
                suffixes=("_base", "_alt"),
            )
            if merged.empty:
                continue
            base_cols = [f"{f}_base" if f"{f}_base" in merged.columns else f for f in model.feature_names]
            alt_cols = [f"{f}_alt" if f"{f}_alt" in merged.columns else f for f in model.feature_names]
            p_base = model.predict_params(merged[base_cols].to_numpy(dtype=np.float64))
            p_alt = model.predict_params(merged[alt_cols].to_numpy(dtype=np.float64))
            lines = (
                np.full(len(merged), 0.5)
                if market == "ANYTIME_TD"
                else np.maximum(0.5, np.floor(merged[f"{spec.ewma_anchor_col}_base"].fillna(10.0).to_numpy()) + 0.5)
            )
            surv_base = survival_array(spec.family, p_base, lines)
            surv_alt = survival_array(spec.family, p_alt, lines)
            diffs.append(float(np.mean(np.abs(surv_base - surv_alt))))

    return float(np.mean(diffs)) if diffs else 0.0


def _assert_pipeline_gates(validation: dict[str, Any], sgp_count: int) -> None:
    held_out = validation["held_out_seasons"]
    if len(held_out) < 3:
        raise RuntimeError(f"walk-forward gate failed: need >= 3 held-out seasons, got {held_out}")
    if validation["populated_bins"] < math.ceil(0.80 * validation["total_bins"]):
        raise RuntimeError(
            f"calibration bin-spread gate failed: {validation['populated_bins']}/{validation['total_bins']} < 80%"
        )
    if validation["log_loss"] > validation["market_log_loss"] + 1e-4:
        raise RuntimeError(
            f"log-loss gate failed: model={validation['log_loss']} > market={validation['market_log_loss']}"
        )
    if validation["brier_score"] > validation["market_brier_score"] + 1e-4:
        raise RuntimeError(
            f"brier gate failed: model={validation['brier_score']} > market={validation['market_brier_score']}"
        )
    if sgp_count > 0 and validation["sgp_log_loss"] >= validation["naive_indep_log_loss"]:
        raise RuntimeError(
            f"copula SGP gate failed: sgp={validation['sgp_log_loss']} >= naive={validation['naive_indep_log_loss']}"
        )
    if validation["stability_mae"] > 0.05:
        raise RuntimeError(f"EWMA stability gate failed: stability_mae={validation['stability_mae']} > 0.05")
