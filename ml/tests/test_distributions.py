"""Distributional NLL gradient checks, analytical survival quantiles, and calibrator tests."""

from __future__ import annotations

import math

import numpy as np
import pytest

from distributions import (
    FAMILY_BERNOULLI,
    FAMILY_NEGBIN,
    FAMILY_POISSON,
    FAMILY_ZERO_ADJ_GAMMA,
    FAMILY_ZERO_ADJ_LOGNORMAL,
    dist_stats_scalar,
    fit_isotonic_calibrator,
    fit_platt_calibrator,
    nll_and_grad_hess,
    survival_scalar,
)


@pytest.mark.parametrize(
    ("family", "y_vals", "eta_vals"),
    [
        (FAMILY_ZERO_ADJ_GAMMA, [0.0, 15.0, 64.0, 112.0], {"pi0": -2.0, "mu": 4.0, "k": 1.1}),
        (FAMILY_ZERO_ADJ_LOGNORMAL, [0.0, 12.0, 45.0, 98.0], {"pi0": -2.2, "mu": 3.5, "sigma": -0.4}),
        (FAMILY_NEGBIN, [0.0, 2.0, 5.0, 8.0], {"mu": 1.4, "r": 1.1}),
        (FAMILY_POISSON, [0.0, 1.0, 2.0, 3.0], {"lambda": 0.5}),
        (FAMILY_BERNOULLI, [0.0, 1.0, 0.0, 1.0], {"p": -0.6}),
    ],
)
def test_nll_analytical_gradients_match_finite_differences(
    family: str,
    y_vals: list[float],
    eta_vals: dict[str, float],
) -> None:
    y = np.asarray(y_vals, dtype=np.float64)
    n = len(y)
    eta = {k: np.full(n, v, dtype=np.float64) for k, v in eta_vals.items()}
    _, grads, hess = nll_and_grad_hess(family, y, eta)

    eps = 1e-6
    for pname in eta_vals:
        assert np.all(hess[pname] > 0.0), f"{family}.{pname} Hessian must be strictly positive"
        eta_p = {k: v.copy() for k, v in eta.items()}
        eta_m = {k: v.copy() for k, v in eta.items()}
        eta_p[pname] = eta_p[pname] + eps
        eta_m[pname] = eta_m[pname] - eps
        nll_p, _, _ = nll_and_grad_hess(family, y, eta_p)
        nll_m, _, _ = nll_and_grad_hess(family, y, eta_m)
        fd_mean = (nll_p - nll_m) / (2.0 * eps)
        ana_mean = float(np.mean(grads[pname]))
        assert abs(fd_mean - ana_mean) < 1e-5, f"{family}.{pname}: fd={fd_mean} vs ana={ana_mean}"


def test_survival_analytical_quantiles() -> None:
    # Exponential (k=1) with zero inflation pi0=0.2, mu=50 -> P(Y > 50) = 0.8 * exp(-1)
    p = survival_scalar(FAMILY_ZERO_ADJ_GAMMA, {"pi0": 0.2, "mu": 50.0, "k": 1.0}, 50.0)
    assert abs(p - 0.8 * math.exp(-1.0)) < 1e-12

    # LogNormal median = exp(mu) -> survival at exp(mu) is (1-pi0)*0.5
    p_ln = survival_scalar(FAMILY_ZERO_ADJ_LOGNORMAL, {"pi0": 0.1, "mu": 3.5, "sigma": 0.6}, math.exp(3.5))
    assert abs(p_ln - 0.45) < 1e-12

    # Geometric (NegBin r=1, mu=4 -> p = 1/5, q = 4/5) -> P(Y > 2) = q^3 = 0.512
    p_nb = survival_scalar(FAMILY_NEGBIN, {"mu": 4.0, "r": 1.0}, 2.5)
    assert abs(p_nb - (0.8**3)) < 1e-12

    # Poisson(1.5) at threshold 0.5 -> 1 - exp(-1.5)
    p_poi = survival_scalar(FAMILY_POISSON, {"lambda": 1.5}, 0.5)
    assert abs(p_poi - (-math.expm1(-1.5))) < 1e-12

    # Bernoulli stats
    st = dist_stats_scalar(FAMILY_BERNOULLI, {"p": 0.4})
    assert abs(st.mean - 0.4) < 1e-12
    assert abs(st.variance - 0.24) < 1e-12


def test_calibrators_monotonic_and_bounded() -> None:
    rng = np.random.default_rng(9)
    probs = rng.uniform(0.05, 0.95, size=500)
    y = rng.binomial(1, np.clip(0.8 * probs + 0.1, 0.02, 0.98)).astype(np.float64)

    platt = fit_platt_calibrator(probs, y)
    iso = fit_isotonic_calibrator(probs, y)
    grid = np.linspace(0.01, 0.99, 50)
    for cal in (platt, iso):
        out = cal.calibrate(grid)
        assert np.all(np.diff(out) >= -1e-12)
        assert np.all((out >= 1e-6) & (out <= 1.0 - 1e-6))
