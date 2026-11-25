"""Distributional families, NLL/gradients/Hessians, survival functions, and calibrators matching Go modelv2."""

from __future__ import annotations

import math
from dataclasses import dataclass
from typing import Any

import numpy as np
from scipy import optimize, special
from sklearn.isotonic import IsotonicRegression

MIN_LOG_ETA = -30.0
MAX_LOG_ETA = 30.0
MIN_CALIB_PROB = 1e-6
MAX_CALIB_PROB = 1.0 - 1e-6

FAMILY_ZERO_ADJ_GAMMA = "zero_adj_gamma"
FAMILY_ZERO_ADJ_LOGNORMAL = "zero_adj_lognormal"
FAMILY_NEGBIN = "negbin"
FAMILY_POISSON = "poisson"
FAMILY_BERNOULLI = "bernoulli"

REQUIRED_PARAMS: dict[str, tuple[str, ...]] = {
    FAMILY_ZERO_ADJ_GAMMA: ("pi0", "mu", "k"),
    FAMILY_ZERO_ADJ_LOGNORMAL: ("pi0", "mu", "sigma"),
    FAMILY_NEGBIN: ("mu", "r"),
    FAMILY_POISSON: ("lambda",),
    FAMILY_BERNOULLI: ("p",),
}


def clamp_unit(p: float) -> float:
    if math.isnan(p) or p <= 0.0:
        return 0.0
    if p >= 1.0:
        return 1.0
    return float(p)


def clamp_calib_prob(p: float) -> float:
    if math.isnan(p):
        return 0.5
    return float(min(max(p, MIN_CALIB_PROB), MAX_CALIB_PROB))


def clamp_log_eta(eta: np.ndarray | float) -> np.ndarray | float:
    if isinstance(eta, np.ndarray):
        clean = np.where(np.isnan(eta), 0.0, eta)
        return np.clip(clean, MIN_LOG_ETA, MAX_LOG_ETA)
    if math.isnan(eta):
        return 0.0
    return float(min(max(eta, MIN_LOG_ETA), MAX_LOG_ETA))


def sigmoid(z: np.ndarray | float) -> np.ndarray | float:
    if isinstance(z, np.ndarray):
        zc = np.clip(np.where(np.isnan(z), 0.0, z), -60.0, 60.0)
        return np.where(zc >= 0.0, 1.0 / (1.0 + np.exp(-zc)), np.exp(zc) / (1.0 + np.exp(zc)))
    if math.isnan(z):
        return 0.5
    if z >= 0.0:
        return 1.0 / (1.0 + math.exp(-z))
    e = math.exp(z)
    return e / (1.0 + e)


@dataclass(frozen=True)
class DistStats:
    mean: float
    variance: float
    std_dev: float
    cv: float
    skewness: float


def _zero_adjusted_stats(pi0: float, pos_mean: float, pos_cv2: float, pos_third_norm: float) -> DistStats:
    active = 1.0 - pi0
    mean = active * pos_mean
    norm_var = active * (pos_cv2 + pi0)
    variance = pos_mean * pos_mean * norm_var
    std_dev = pos_mean * math.sqrt(max(norm_var, 0.0))
    cv = math.sqrt((pos_cv2 + pi0) / active) if active > 0.0 else 0.0
    skew = 0.0
    if norm_var > 0.0:
        norm_third = active * (pos_third_norm + 3.0 * pi0 * pos_cv2 + pi0 * (2.0 * pi0 - 1.0))
        skew = norm_third / (norm_var * math.sqrt(norm_var))
    return DistStats(mean=mean, variance=variance, std_dev=std_dev, cv=cv, skewness=skew)


def eta_to_params(family: str, eta: dict[str, np.ndarray]) -> dict[str, np.ndarray]:
    """Convert raw linear predictors eta into validated distribution parameters."""
    if family == FAMILY_ZERO_ADJ_GAMMA:
        return {
            "pi0": np.clip(sigmoid(eta["pi0"]), 1e-6, 1.0 - 1e-6),
            "mu": np.exp(clamp_log_eta(eta["mu"])),
            "k": np.exp(clamp_log_eta(eta["k"])),
        }
    if family == FAMILY_ZERO_ADJ_LOGNORMAL:
        return {
            "pi0": np.clip(sigmoid(eta["pi0"]), 1e-6, 1.0 - 1e-6),
            "mu": np.where(np.isnan(eta["mu"]), 0.0, eta["mu"]),
            "sigma": np.exp(clamp_log_eta(eta["sigma"])),
        }
    if family == FAMILY_NEGBIN:
        return {
            "mu": np.exp(clamp_log_eta(eta["mu"])),
            "r": np.exp(clamp_log_eta(eta["r"])),
        }
    if family == FAMILY_POISSON:
        out = {"lambda": np.exp(clamp_log_eta(eta["lambda"]))}
        if "r" in eta:
            out["r"] = np.exp(clamp_log_eta(eta["r"]))
        return out
    if family == FAMILY_BERNOULLI:
        return {"p": np.clip(sigmoid(eta["p"]), 1e-6, 1.0 - 1e-6)}
    raise ValueError(f"unsupported family {family!r}")


def initial_base_scores(family: str, y: np.ndarray) -> dict[str, float]:
    """Compute closed-form unconditional MLE base scores in eta-space."""
    y = np.asarray(y, dtype=np.float64)
    if family == FAMILY_ZERO_ADJ_GAMMA:
        y_pos = y[y > 0.0]
        pi0 = float(np.clip(np.mean(y <= 0.0), 1e-3, 0.99))
        mu = float(np.mean(y_pos)) if len(y_pos) > 0 else 10.0
        var = float(np.var(y_pos)) if len(y_pos) > 1 else mu * mu * 0.5
        k = float(np.clip((mu * mu) / max(var, 1e-3), 0.25, 25.0))
        return {
            "pi0": math.log(pi0 / (1.0 - pi0)),
            "mu": math.log(max(mu, 1e-3)),
            "k": math.log(k),
        }
    if family == FAMILY_ZERO_ADJ_LOGNORMAL:
        y_pos = y[y > 0.0]
        pi0 = float(np.clip(np.mean(y <= 0.0), 1e-3, 0.99))
        if len(y_pos) > 0:
            ly = np.log(y_pos)
            mu = float(np.mean(ly))
            sigma = float(np.clip(np.std(ly), 0.15, 2.5))
        else:
            mu, sigma = 2.0, 0.75
        return {
            "pi0": math.log(pi0 / (1.0 - pi0)),
            "mu": mu,
            "sigma": math.log(sigma),
        }
    if family == FAMILY_NEGBIN:
        mu = float(max(np.mean(y), 0.1))
        var = float(np.var(y))
        r = float(np.clip((mu * mu) / max(var - mu, 0.05 * mu), 0.3, 25.0))
        return {"mu": math.log(mu), "r": math.log(r)}
    if family == FAMILY_POISSON:
        lam = float(max(np.mean(y), 0.05))
        return {"lambda": math.log(lam)}
    if family == FAMILY_BERNOULLI:
        p = float(np.clip(np.mean(y > 0.0), 1e-3, 0.999))
        return {"p": math.log(p / (1.0 - p))}
    raise ValueError(f"unsupported family {family!r}")


def nll_and_grad_hess(
    family: str,
    y: np.ndarray,
    eta: dict[str, np.ndarray],
) -> tuple[float, dict[str, np.ndarray], dict[str, np.ndarray]]:
    """Return mean NLL plus per-parameter gradients and positive Fisher/Newton Hessians."""
    y = np.asarray(y, dtype=np.float64)
    params = eta_to_params(family, eta)
    n = max(len(y), 1)

    if family == FAMILY_ZERO_ADJ_GAMMA:
        pi0, mu, k = params["pi0"], params["mu"], params["k"]
        is_zero = y <= 0.0
        y_safe = np.where(is_zero, 1.0, np.maximum(y, 1e-6))

        nll_zero = -np.log(pi0)
        nll_pos = (
            -np.log1p(-pi0)
            - k * np.log(k / mu)
            + special.gammaln(k)
            - (k - 1.0) * np.log(y_safe)
            + (k / mu) * y_safe
        )
        total_nll = float(np.sum(np.where(is_zero, nll_zero, nll_pos)) / n)

        g_pi0 = np.where(is_zero, pi0 - 1.0, pi0)
        h_pi0 = np.maximum(pi0 * (1.0 - pi0), 1e-4)

        g_mu = np.where(is_zero, 0.0, k * (1.0 - y_safe / mu))
        h_mu = np.where(is_zero, 1e-4, np.maximum(k, 1e-4))

        score_k = np.log(k / mu) + 1.0 - special.digamma(k) + np.log(y_safe) - y_safe / mu
        g_k = np.where(is_zero, 0.0, -k * score_k)
        # Fisher information for log-shape eta_k is k^2 * trigamma(k) - k > 0 for all k > 0.
        fisher_k = k * k * special.polygamma(1, k) - k
        h_k = np.where(is_zero, 1e-4, np.clip(fisher_k, 1e-3, 100.0))

        return (
            total_nll,
            {"pi0": g_pi0, "mu": g_mu, "k": g_k},
            {"pi0": h_pi0, "mu": h_mu, "k": h_k},
        )

    if family == FAMILY_ZERO_ADJ_LOGNORMAL:
        pi0, mu, sigma = params["pi0"], params["mu"], params["sigma"]
        is_zero = y <= 0.0
        y_safe = np.where(is_zero, 1.0, np.maximum(y, 1e-6))
        z = (np.log(y_safe) - mu) / sigma

        nll_zero = -np.log(pi0)
        nll_pos = -np.log1p(-pi0) + np.log(y_safe) + np.log(sigma) + 0.5 * math.log(2.0 * math.pi) + 0.5 * z * z
        total_nll = float(np.sum(np.where(is_zero, nll_zero, nll_pos)) / n)

        g_pi0 = np.where(is_zero, pi0 - 1.0, pi0)
        h_pi0 = np.maximum(pi0 * (1.0 - pi0), 1e-4)
        g_mu = np.where(is_zero, 0.0, -z / sigma)
        h_mu = np.where(is_zero, 1e-4, np.maximum(1.0 / (sigma * sigma), 1e-4))
        g_sigma = np.where(is_zero, 0.0, 1.0 - z * z)
        h_sigma = np.where(is_zero, 1e-4, 2.0)

        return (
            total_nll,
            {"pi0": g_pi0, "mu": g_mu, "sigma": g_sigma},
            {"pi0": h_pi0, "mu": h_mu, "sigma": h_sigma},
        )

    if family == FAMILY_NEGBIN:
        mu, r = params["mu"], params["r"]
        y_cnt = np.maximum(np.rint(y), 0.0)
        log_p = -np.log1p(mu / r)
        log_q = np.log(mu / (r + mu))
        ll = (
            special.gammaln(y_cnt + r)
            - special.gammaln(r)
            - special.gammaln(y_cnt + 1.0)
            + r * log_p
            + y_cnt * log_q
        )
        total_nll = float(-np.mean(ll))

        g_mu = r * (mu - y_cnt) / (r + mu)
        h_mu = np.maximum((r * mu) / (r + mu), 1e-4)

        score_r = special.digamma(y_cnt + r) - special.digamma(r) + log_p + (mu - y_cnt) / (r + mu)
        g_r = -r * score_r
        # Observed negative curvature of log P(Y=y|r) wrt log(r), floored for Newton stability.
        obs_curv = -(
            r * r * (special.polygamma(1, y_cnt + r) - special.polygamma(1, r) + (y_cnt + r) / ((r + mu) ** 2) - 1.0 / (r + mu))
            + r * score_r
        )
        h_r = np.clip(obs_curv, 0.05, 50.0)
        return total_nll, {"mu": g_mu, "r": g_r}, {"mu": h_mu, "r": h_r}

    if family == FAMILY_POISSON:
        lam = params["lambda"]
        y_cnt = np.maximum(np.rint(y), 0.0)
        ll = -lam + y_cnt * np.log(lam) - special.gammaln(y_cnt + 1.0)
        total_nll = float(-np.mean(ll))
        g_lam = lam - y_cnt
        h_lam = np.maximum(lam, 1e-4)
        return total_nll, {"lambda": g_lam}, {"lambda": h_lam}

    if family == FAMILY_BERNOULLI:
        p = params["p"]
        y_bin = (y > 0.5).astype(np.float64)
        ll = y_bin * np.log(p) + (1.0 - y_bin) * np.log1p(-p)
        total_nll = float(-np.mean(ll))
        g_p = p - y_bin
        h_p = np.maximum(p * (1.0 - p), 1e-4)
        return total_nll, {"p": g_p}, {"p": h_p}

    raise ValueError(f"unsupported family {family!r}")


def survival_scalar(family: str, params: dict[str, float], threshold: float) -> float:
    """Evaluate P(Y > threshold) identically to Go internal/modelv2/dist.go."""
    if math.isnan(threshold) or (math.isinf(threshold) and threshold > 0):
        return 0.0
    if (math.isinf(threshold) and threshold < 0) or threshold < 0.0:
        return 1.0

    if family == FAMILY_ZERO_ADJ_GAMMA:
        pi0, mu, k = params["pi0"], params["mu"], params["k"]
        active = 1.0 - pi0
        if threshold == 0.0:
            return clamp_unit(active)
        if mu <= 0.0 or k <= 0.0:
            return 0.0
        x = threshold * (k / mu)
        q = float(special.gammaincc(k, x))
        return clamp_unit(active * q)

    if family == FAMILY_ZERO_ADJ_LOGNORMAL:
        pi0, mu, sigma = params["pi0"], params["mu"], params["sigma"]
        active = 1.0 - pi0
        if threshold == 0.0:
            return clamp_unit(active)
        if sigma <= 0.0:
            return 0.0
        z = (math.log(threshold) - mu) / sigma
        surv_pos = 0.5 * math.erfc(z / math.sqrt(2.0))
        return clamp_unit(active * surv_pos)

    if family == FAMILY_NEGBIN:
        mu, r = params["mu"], params["r"]
        if mu <= 0.0 or r <= 0.0:
            return 0.0
        k_floor = math.floor(threshold)
        q = mu / (r + mu)
        return clamp_unit(float(special.betainc(k_floor + 1.0, r, q)))

    if family == FAMILY_POISSON:
        if "r" in params:
            return survival_scalar(FAMILY_NEGBIN, {"mu": params["lambda"], "r": params["r"]}, threshold)
        lam = params["lambda"]
        if lam <= 0.0:
            return 0.0
        k_floor = math.floor(threshold)
        if k_floor == 0.0:
            return clamp_unit(-math.expm1(-lam))
        return clamp_unit(float(special.gammainc(k_floor + 1.0, lam)))

    if family == FAMILY_BERNOULLI:
        p = params["p"]
        return clamp_unit(p) if threshold < 1.0 else 0.0

    raise ValueError(f"unsupported family {family!r}")


def survival_array(family: str, params: dict[str, np.ndarray], thresholds: np.ndarray) -> np.ndarray:
    """Vectorized survival probability P(Y > threshold) matching Go Survival."""
    t = np.asarray(thresholds, dtype=np.float64)
    out = np.zeros_like(t, dtype=np.float64)
    neg_mask = t < 0.0
    zero_mask = t == 0.0
    pos_mask = t > 0.0

    if family == FAMILY_ZERO_ADJ_GAMMA:
        pi0, mu, k = params["pi0"], params["mu"], params["k"]
        active = 1.0 - pi0
        out[neg_mask] = 1.0
        out[zero_mask] = active[zero_mask]
        if np.any(pos_mask):
            x = t[pos_mask] * (k[pos_mask] / mu[pos_mask])
            out[pos_mask] = active[pos_mask] * special.gammaincc(k[pos_mask], x)
        return np.clip(out, 0.0, 1.0)

    if family == FAMILY_ZERO_ADJ_LOGNORMAL:
        pi0, mu, sigma = params["pi0"], params["mu"], params["sigma"]
        active = 1.0 - pi0
        out[neg_mask] = 1.0
        out[zero_mask] = active[zero_mask]
        if np.any(pos_mask):
            z = (np.log(t[pos_mask]) - mu[pos_mask]) / sigma[pos_mask]
            out[pos_mask] = active[pos_mask] * 0.5 * special.erfc(z / math.sqrt(2.0))
        return np.clip(out, 0.0, 1.0)

    if family == FAMILY_NEGBIN:
        mu, r = params["mu"], params["r"]
        out[neg_mask] = 1.0
        non_neg = ~neg_mask
        if np.any(non_neg):
            k_floor = np.floor(t[non_neg])
            q = mu[non_neg] / (r[non_neg] + mu[non_neg])
            out[non_neg] = special.betainc(k_floor + 1.0, r[non_neg], q)
        return np.clip(out, 0.0, 1.0)

    if family == FAMILY_POISSON:
        if "r" in params:
            return survival_array(FAMILY_NEGBIN, {"mu": params["lambda"], "r": params["r"]}, t)
        lam = params["lambda"]
        out[neg_mask] = 1.0
        zero_floor = (~neg_mask) & (np.floor(t) == 0.0)
        hi_floor = (~neg_mask) & (np.floor(t) >= 1.0)
        if np.any(zero_floor):
            out[zero_floor] = -np.expm1(-lam[zero_floor])
        if np.any(hi_floor):
            out[hi_floor] = special.gammainc(np.floor(t[hi_floor]) + 1.0, lam[hi_floor])
        return np.clip(out, 0.0, 1.0)

    if family == FAMILY_BERNOULLI:
        p = params["p"]
        out[neg_mask] = 1.0
        mid = (~neg_mask) & (t < 1.0)
        out[mid] = p[mid]
        return np.clip(out, 0.0, 1.0)

    raise ValueError(f"unsupported family {family!r}")


def dist_stats_scalar(family: str, params: dict[str, float]) -> DistStats:
    """Compute exact analytical distribution moments matching Go DistStats."""
    if family == FAMILY_ZERO_ADJ_GAMMA:
        pi0, mu, k = params["pi0"], params["mu"], params["k"]
        return _zero_adjusted_stats(pi0, mu, 1.0 / k, 2.0 / (k * k))
    if family == FAMILY_ZERO_ADJ_LOGNORMAL:
        pi0, mu, sigma = params["pi0"], params["mu"], params["sigma"]
        s2 = sigma * sigma
        v = math.expm1(s2)
        pos_mean = math.exp(mu + 0.5 * s2)
        pos_third_norm = (v + 3.0) * v * v
        return _zero_adjusted_stats(pi0, pos_mean, v, pos_third_norm)
    if family == FAMILY_NEGBIN:
        mu, r = params["mu"], params["r"]
        var = mu * (1.0 + mu / r)
        sd = math.sqrt(var)
        return DistStats(
            mean=mu,
            variance=var,
            std_dev=sd,
            cv=math.sqrt(1.0 / mu + 1.0 / r),
            skewness=(1.0 + 2.0 * mu / r) / sd if sd > 0 else 0.0,
        )
    if family == FAMILY_POISSON:
        if "r" in params:
            return dist_stats_scalar(FAMILY_NEGBIN, {"mu": params["lambda"], "r": params["r"]})
        lam = params["lambda"]
        sd = math.sqrt(lam)
        inv_sd = 1.0 / sd if sd > 0 else 0.0
        return DistStats(mean=lam, variance=lam, std_dev=sd, cv=inv_sd, skewness=inv_sd)
    if family == FAMILY_BERNOULLI:
        p = params["p"]
        var = p * (1.0 - p)
        sd = math.sqrt(var)
        cv = math.sqrt((1.0 - p) / p) if p > 0 else 0.0
        skew = (1.0 - 2.0 * p) / sd if sd > 0 else 0.0
        return DistStats(mean=p, variance=var, std_dev=sd, cv=cv, skewness=skew)
    raise ValueError(f"unsupported family {family!r}")


@dataclass
class Calibrator:
    """Post-hoc calibrator supporting isotonic piecewise-linear and logit-space Platt scaling."""

    method: str
    knots_x: list[float] | None = None
    knots_y: list[float] | None = None
    platt_a: float = 1.0
    platt_b: float = 0.0

    def calibrate(self, probs: np.ndarray | float) -> np.ndarray | float:
        is_scalar = np.ndim(probs) == 0
        arr = np.atleast_1d(np.asarray(probs, dtype=np.float64))
        arr = np.where(np.isnan(arr), 0.5, arr)
        arr = np.clip(arr, MIN_CALIB_PROB, MAX_CALIB_PROB)

        if self.method == "platt":
            logits = np.log(arr / (1.0 - arr))
            out = np.clip(sigmoid(self.platt_a * logits + self.platt_b), MIN_CALIB_PROB, MAX_CALIB_PROB)
        elif self.method == "isotonic":
            assert self.knots_x and self.knots_y and len(self.knots_x) >= 2
            kx = np.asarray(self.knots_x, dtype=np.float64)
            ky = np.asarray(self.knots_y, dtype=np.float64)
            out = np.clip(np.interp(arr, kx, ky), MIN_CALIB_PROB, MAX_CALIB_PROB)
        else:
            raise ValueError(f"unsupported calibrator method {self.method!r}")

        return float(out[0]) if is_scalar else out

    def to_dict(self) -> dict[str, Any]:
        if self.method == "isotonic":
            return {
                "method": "isotonic",
                "knots_x": [float(x) for x in (self.knots_x or [])],
                "knots_y": [float(y) for y in (self.knots_y or [])],
            }
        return {
            "method": "platt",
            "platt_a": float(self.platt_a),
            "platt_b": float(self.platt_b),
        }


def fit_platt_calibrator(probs: np.ndarray, y: np.ndarray) -> Calibrator:
    """Fit Platt logistic calibrator in logit(p) space with a > 0 via L-BFGS-B."""
    p = np.clip(np.asarray(probs, dtype=np.float64), MIN_CALIB_PROB, MAX_CALIB_PROB)
    target = np.asarray(y, dtype=np.float64)
    logits = np.log(p / (1.0 - p))

    def objective(params: np.ndarray) -> float:
        a, b = float(params[0]), float(params[1])
        z = a * logits + b
        # Stable softplus NLL + mild L2 shrinkage toward identity (a=1, b=0).
        loss = np.mean(np.logaddexp(0.0, z) - target * z)
        reg = 1e-3 * ((a - 1.0) ** 2 + b * b)
        return float(loss + reg)

    res = optimize.minimize(
        objective,
        x0=np.array([1.0, 0.0]),
        bounds=[(0.15, 5.0), (-2.5, 2.5)],
        method="L-BFGS-B",
    )
    a, b = float(res.x[0]), float(res.x[1])
    return Calibrator(method="platt", platt_a=a, platt_b=b)


def fit_isotonic_calibrator(probs: np.ndarray, y: np.ndarray) -> Calibrator:
    """Fit monotonic IsotonicRegression and extract compressed piecewise-linear knots."""
    p = np.clip(np.asarray(probs, dtype=np.float64), MIN_CALIB_PROB, MAX_CALIB_PROB)
    target = np.asarray(y, dtype=np.float64)
    iso = IsotonicRegression(y_min=MIN_CALIB_PROB, y_max=MAX_CALIB_PROB, out_of_bounds="clip")
    iso.fit(p, target)

    kx = np.asarray(iso.X_thresholds_, dtype=np.float64)
    ky = np.asarray(iso.y_thresholds_, dtype=np.float64)
    # Deduplicate strictly increasing X knots while preserving monotonicity in Y.
    uniq_x: list[float] = []
    uniq_y: list[float] = []
    for xv, yv in zip(kx, ky, strict=True):
        xv_c = float(np.clip(xv, 0.0, 1.0))
        yv_c = float(np.clip(yv, MIN_CALIB_PROB, MAX_CALIB_PROB))
        if uniq_x and abs(xv_c - uniq_x[-1]) < 1e-9:
            uniq_y[-1] = max(uniq_y[-1], yv_c)
            continue
        if uniq_y and yv_c < uniq_y[-1]:
            yv_c = uniq_y[-1]
        uniq_x.append(xv_c)
        uniq_y.append(yv_c)

    if len(uniq_x) < 2:
        return Calibrator(method="isotonic", knots_x=[0.0, 1.0], knots_y=[float(np.mean(target))] * 2)
    return Calibrator(method="isotonic", knots_x=uniq_x, knots_y=uniq_y)


def binary_log_loss(probs: np.ndarray, y: np.ndarray) -> float:
    p = np.clip(np.asarray(probs, dtype=np.float64), MIN_CALIB_PROB, MAX_CALIB_PROB)
    target = np.asarray(y, dtype=np.float64)
    return float(-np.mean(target * np.log(p) + (1.0 - target) * np.log1p(-p)))


def brier_score(probs: np.ndarray, y: np.ndarray) -> float:
    p = np.clip(np.asarray(probs, dtype=np.float64), 0.0, 1.0)
    target = np.asarray(y, dtype=np.float64)
    return float(np.mean((p - target) ** 2))


def reliability_summary(probs: np.ndarray, y: np.ndarray, n_bins: int = 10) -> dict[str, Any]:
    """Compute ECE, bin spread, and reliability bin table matching Go eval."""
    p = np.clip(np.asarray(probs, dtype=np.float64), 0.0, 1.0)
    target = np.asarray(y, dtype=np.float64)
    n = max(len(p), 1)
    bins: list[dict[str, Any]] = []
    populated = 0
    ece = 0.0

    for i in range(n_bins):
        lo = i / n_bins
        hi = (i + 1) / n_bins
        if i == n_bins - 1:
            mask = (p >= lo) & (p <= hi)
        else:
            mask = (p >= lo) & (p < hi)
        cnt = int(np.sum(mask))
        if cnt > 0:
            populated += 1
            mean_p = float(np.mean(p[mask]))
            emp_y = float(np.mean(target[mask]))
            ece += (cnt / n) * abs(mean_p - emp_y)
        else:
            mean_p = 0.0
            emp_y = 0.0
        bins.append(
            {
                "bin_low": lo,
                "bin_high": hi,
                "count": cnt,
                "mean_pred_prob": mean_p,
                "empirical_hit": emp_y,
            }
        )

    return {
        "ece": float(ece),
        "populated_bins": populated,
        "total_bins": n_bins,
        "bin_spread_ok": populated >= math.ceil(0.80 * n_bins),
        "reliability": bins,
    }
