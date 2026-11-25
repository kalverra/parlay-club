"""PSD Gaussian copula estimation, Higham projection, and exact Go-parity QMC SGP evaluation."""

from __future__ import annotations

from dataclasses import dataclass
import math
from typing import Any

import numpy as np
import pandas as pd
from scipy import stats

from distributions import binary_log_loss

QMC_SAMPLES = 8192
MAX_QMC_DIMS = 32
MIN_COPULA_PROB = 1e-4
MAX_COPULA_PROB = 1.0 - 1e-4
HEAVY_FAVORITE_SPREAD = 7.0
SHOOTOUT_TOTAL = 48.0
DISTINCT_PLAYER_SAME_NODE_CORR = 0.20

REGIME_NEUTRAL = "neutral"
REGIME_HEAVY_FAVORITE = "heavy_favorite"
REGIME_SHOOTOUT = "shootout"

CANONICAL_COPULA_NODES: tuple[str, ...] = (
    "TEAM_MARGIN",
    "GAME_TOTAL",
    "QB_PASS_YDS",
    "QB_PASS_TDS",
    "WR1_REC_YDS",
    "WR1_RECEPTIONS",
    "WR2_REC_YDS",
    "WR2_RECEPTIONS",
    "TE1_REC_YDS",
    "TE1_RECEPTIONS",
    "RB1_RUSH_YDS",
    "RB1_RUSH_ATT",
    "RB1_REC_YDS",
    "RB2_RUSH_YDS",
    "OPP_QB_PASS_YDS",
    "OPP_QB_PASS_TDS",
    "OPP_WR1_REC_YDS",
    "OPP_WR2_REC_YDS",
    "OPP_TE1_REC_YDS",
    "OPP_RB1_RUSH_YDS",
)

# Joe & Kuo (2010) new-joe-kuo-6.21201 primitive polynomials & init direction numbers (dims 2..32).
SOBOL_DIM_PARAMS: tuple[tuple[int, int, tuple[int, ...]], ...] = (
    (1, 0, (1,)),
    (2, 1, (1, 3)),
    (3, 1, (1, 3, 1)),
    (3, 2, (1, 1, 1)),
    (4, 1, (1, 1, 3, 3)),
    (4, 4, (1, 3, 5, 13)),
    (5, 2, (1, 1, 5, 5, 17)),
    (5, 4, (1, 1, 5, 5, 5)),
    (5, 7, (1, 1, 7, 11, 19)),
    (5, 11, (1, 1, 5, 1, 1)),
    (5, 13, (1, 1, 1, 3, 11)),
    (5, 14, (1, 3, 5, 5, 31)),
    (6, 1, (1, 3, 3, 9, 7, 49)),
    (6, 13, (1, 1, 1, 15, 21, 21)),
    (6, 16, (1, 3, 1, 13, 27, 49)),
    (6, 19, (1, 1, 1, 15, 7, 5)),
    (6, 22, (1, 3, 1, 15, 13, 25)),
    (6, 25, (1, 1, 5, 5, 19, 61)),
    (7, 1, (1, 3, 7, 11, 23, 15, 103)),
    (7, 4, (1, 3, 7, 13, 13, 15, 69)),
    (7, 7, (1, 1, 3, 13, 7, 35, 63)),
    (7, 8, (1, 3, 5, 9, 1, 25, 53)),
    (7, 14, (1, 3, 1, 13, 9, 35, 107)),
    (7, 19, (1, 3, 1, 5, 27, 61, 31)),
    (7, 21, (1, 1, 5, 11, 19, 41, 61)),
    (7, 28, (1, 3, 5, 3, 3, 13, 69)),
    (7, 31, (1, 1, 7, 13, 1, 19, 1)),
    (7, 32, (1, 3, 7, 5, 13, 49, 47)),
    (7, 37, (1, 1, 5, 11, 21, 17, 19)),
    (7, 41, (1, 1, 1, 1, 5, 29, 89)),
    (7, 42, (1, 3, 3, 15, 15, 41, 95)),
)

HALTON_EXTRA_PRIMES = (137, 139, 149, 151, 157, 163, 167, 173, 179, 181, 191, 193, 197, 199, 211, 223)

JITTER_STEPS = (
    0.0,
    1e-9,
    1e-7,
    1e-5,
    1e-4,
    1e-3,
    0.005,
    0.01,
    0.025,
    0.05,
    0.1,
    0.2,
    0.4,
    0.8,
    1.6,
    3.2,
    6.4,
    12.8,
    25.6,
)


def norm_inv(p: float) -> float:
    """Acklam's inverse normal CDF matching Go internal/modelv2/copula.go."""
    if p <= 0.0:
        return -math.inf
    if p >= 1.0:
        return math.inf

    a1, a2, a3, a4, a5, a6 = (
        -3.969683028665376e01,
        2.209460984245205e02,
        -2.759285104469687e02,
        1.383577518672690e02,
        -3.066479806614716e01,
        2.506628277459239e00,
    )
    b1, b2, b3, b4, b5 = (
        -5.447609879822406e01,
        1.615858368580409e02,
        -1.556989798598866e02,
        6.680131188771972e01,
        -1.328068155288572e01,
    )
    c1, c2, c3, c4, c5, c6 = (
        -7.784894002430293e-03,
        -3.223964580411365e-01,
        -2.400758277161838e00,
        -2.549732539343734e00,
        4.374664141464968e00,
        2.938163982698783e00,
    )
    d1, d2, d3, d4 = (
        7.784695709041462e-03,
        3.224671290700398e-01,
        2.445134137142996e00,
        3.754408661907416e00,
    )
    p_low = 0.02425
    p_high = 1.0 - p_low

    if p < p_low:
        q = math.sqrt(-2.0 * math.log(p))
        return (((((c1 * q + c2) * q + c3) * q + c4) * q + c5) * q + c6) / (
            (((d1 * q + d2) * q + d3) * q + d4) * q + 1.0
        )
    if p <= p_high:
        q = p - 0.5
        r = q * q
        return (((((a1 * r + a2) * r + a3) * r + a4) * r + a5) * r + a6) * q / (
            ((((b1 * r + b2) * r + b3) * r + b4) * r + b5) * r + 1.0
        )
    q = math.sqrt(-2.0 * math.log(1.0 - p))
    return -(((((c1 * q + c2) * q + c3) * q + c4) * q + c5) * q + c6) / (
        (((d1 * q + d2) * q + d3) * q + d4) * q + 1.0
    )


def _trailing_ones_ctz_inv(r: int) -> int:
    """Count trailing zeros of ~uint32(r) (position of rightmost zero bit)."""
    v = (~r) & 0xFFFFFFFF
    return (v & -v).bit_length() - 1


def build_sobol_normals(n_samples: int = QMC_SAMPLES, dims: int = MAX_QMC_DIMS) -> np.ndarray:
    """Generate exact Gray-code midpoint-shifted Joe & Kuo Sobol normal draws matching Go."""
    num_bits = 32
    v: list[list[int]] = []
    for d in range(dims):
        vd = [0] * num_bits
        if d == 0:
            for k in range(num_bits):
                vd[k] = 1 << (31 - k)
        else:
            s, poly, init_m = SOBOL_DIM_PARAMS[d - 1]
            for k in range(s):
                vd[k] = init_m[k] << (31 - k)
            for k in range(s, num_bits):
                val = vd[k - s] ^ (vd[k - s] >> s)
                for i in range(1, s):
                    if (poly >> (s - 1 - i)) & 1:
                        val ^= vd[k - i]
                vd[k] = val & 0xFFFFFFFF
        v.append(vd)

    out = np.empty((n_samples, dims), dtype=np.float64)
    x = [0] * dims
    inv_2p32 = 1.0 / 4294967296.0
    midpoint = 0.5 / float(n_samples)

    for r in range(n_samples):
        for d in range(dims):
            u = float(x[d]) * inv_2p32 + midpoint
            out[r, d] = norm_inv(u)
        if r + 1 < n_samples:
            c = _trailing_ones_ctz_inv(r)
            for d in range(dims):
                x[d] ^= v[d][c]
    return out


_QMC_NORMALS: np.ndarray | None = None


def get_qmc_normals() -> np.ndarray:
    global _QMC_NORMALS
    if _QMC_NORMALS is None:
        _QMC_NORMALS = build_sobol_normals(QMC_SAMPLES, MAX_QMC_DIMS)
    return _QMC_NORMALS


def higham_nearest_psd_correlation(mat: np.ndarray, min_eig: float = 1e-6, max_iter: int = 100) -> np.ndarray:
    """Project a symmetric matrix onto the unit-diagonal PSD correlation cone via Higham's method."""
    n = mat.shape[0]
    y = 0.5 * (np.asarray(mat, dtype=np.float64) + np.asarray(mat, dtype=np.float64).T)
    y = np.nan_to_num(y, nan=0.0)
    np.fill_diagonal(y, 1.0)
    ds = np.zeros_like(y)

    for _ in range(max_iter):
        r = y - ds
        eigvals, eigvecs = np.linalg.eigh(r)
        eigvals_clipped = np.maximum(eigvals, min_eig)
        x = (eigvecs * eigvals_clipped) @ eigvecs.T
        x = 0.5 * (x + x.T)
        ds = x - r
        y_next = x.copy()
        np.fill_diagonal(y_next, 1.0)
        y_next = np.clip(y_next, -1.0, 1.0)
        if np.max(np.abs(y_next - y)) < 1e-10:
            y = y_next
            break
        y = y_next

    # Final safety shrink toward identity if unit-diagonal projection created tiny negative eigenvalue.
    eig_min = float(np.min(np.linalg.eigvalsh(y)))
    if eig_min < min_eig:
        alpha = (min_eig - eig_min) / max(1.0 - eig_min, 1e-9)
        alpha = float(np.clip(alpha * 1.05, 0.0, 1.0))
        y = (1.0 - alpha) * y + alpha * np.eye(n)
    y = 0.5 * (y + y.T)
    np.fill_diagonal(y, 1.0)
    return np.clip(y, -1.0, 1.0)


@dataclass
class CopulaLeg:
    leg_id: int
    game_id: str
    team: str
    player_id: str
    player_name: str
    role: str
    market: str
    side: str
    prob: float
    home_spread: float = 0.0
    game_total: float = 0.0


@dataclass
class CopulaGameContext:
    home_team: str = ""
    away_team: str = ""
    home_spread: float = 0.0
    game_total: float = 0.0
    has_spread: bool = False
    has_total: bool = False


def select_regime(ctx: CopulaGameContext) -> str:
    if ctx.has_spread and abs(ctx.home_spread) >= HEAVY_FAVORITE_SPREAD:
        return REGIME_HEAVY_FAVORITE
    if ctx.has_total and ctx.game_total >= SHOOTOUT_TOTAL:
        return REGIME_SHOOTOUT
    return REGIME_NEUTRAL


def _normalize_copula_role(role: str, market: str) -> str:
    r = role.strip().upper()
    if r in ("QB", "WR1", "WR2", "TE1", "RB1", "RB2"):
        return r
    if r == "WR":
        return "WR2"
    if r == "RB":
        return "RB2"
    if r == "TE":
        return "TE1"
    if market in ("PASS_YDS", "PASS_TDS"):
        return "QB"
    if market in ("RUSH_YDS", "RUSH_ATT"):
        return "RB2"
    return "WR2"


def _proxy_candidates(prefix: str, role: str, market: str) -> list[tuple[str, float]]:
    if market == "ANYTIME_TD":
        if role == "QB":
            return [(f"{prefix}QB_PASS_TDS", 0.45), ("TEAM_MARGIN", 0.30)]
        if role in ("RB1", "RB2"):
            return [(f"{prefix}{role}_RUSH_YDS", 0.65), (f"{prefix}RB1_RUSH_YDS", 0.55)]
        return [(f"{prefix}{role}_REC_YDS", 0.65), (f"{prefix}WR2_REC_YDS", 0.60)]
    if market == "RECEPTIONS":
        if role in ("RB1", "RB2"):
            return [(f"{prefix}RB1_REC_YDS", 0.85), (f"{prefix}QB_PASS_YDS", 0.30)]
        return [(f"{prefix}{role}_REC_YDS", 0.85), (f"{prefix}WR2_REC_YDS", 0.80)]
    if market == "RUSH_ATT":
        return [(f"{prefix}{role}_RUSH_YDS", 0.85), (f"{prefix}RB1_RUSH_YDS", 0.65)]
    if market == "RUSH_YDS":
        if role == "QB":
            return [("GAME_TOTAL", 0.25)]
        return [(f"{prefix}RB1_RUSH_YDS", 0.50)]
    if market == "REC_YDS":
        return [(f"{prefix}QB_PASS_YDS", 0.30), ("GAME_TOTAL", 0.25)]
    if market in ("PASS_YDS", "PASS_TDS"):
        return [(f"{prefix}QB_{market}", 0.75)]
    return []


def _cholesky_with_jitter(sub: np.ndarray) -> tuple[np.ndarray, float]:
    m = sub.shape[0]
    for lam in JITTER_STEPS:
        scale = 1.0 / (1.0 + lam)
        scaled = sub * scale
        np.fill_diagonal(scaled, 1.0)
        try:
            chol = np.linalg.cholesky(scaled)
            return chol, lam
        except np.linalg.LinAlgError:
            continue
    raise ValueError(f"cholesky factorization failed for {m}x{m} correlation matrix")


def _is_same_player(a: CopulaLeg, b: CopulaLeg) -> bool:
    if a.player_id and b.player_id:
        return a.player_id == b.player_id
    if not a.player_name or not b.player_name:
        return False
    if a.team and b.team and a.team.upper() != b.team.upper():
        return False
    return a.player_name.strip().lower() == b.player_name.strip().lower()


def _lookup_within_player(within: dict[str, float], m_a: str, m_b: str) -> float | None:
    if f"{m_a}:{m_b}" in within:
        return within[f"{m_a}:{m_b}"]
    if f"{m_b}:{m_a}" in within:
        return within[f"{m_b}:{m_a}"]
    return None


class CopulaModel:
    """Pure-Python equivalent of Go modelv2.CopulaArtifact for exact parity and SGP evaluation."""

    def __init__(
        self,
        nodes: list[str],
        default_matrix: list[list[float]],
        regime_matrices: dict[str, list[list[float]]],
        within_player: dict[str, float],
    ) -> None:
        self.nodes = list(nodes)
        self.node_index = {n: i for i, n in enumerate(self.nodes)}
        self.default_matrix = np.asarray(default_matrix, dtype=np.float64)
        self.regime_matrices = {k: np.asarray(v, dtype=np.float64) for k, v in regime_matrices.items()}
        self.within_player = dict(within_player)

    def to_dict(self) -> dict[str, Any]:
        return {
            "nodes": list(self.nodes),
            "default_matrix": [[round(float(x), 6) for x in row] for row in self.default_matrix.tolist()],
            "regime_matrices": {
                k: [[round(float(x), 6) for x in row] for row in mat.tolist()]
                for k, mat in self.regime_matrices.items()
            },
            "within_player": {k: round(float(v), 6) for k, v in self.within_player.items()},
        }

    def estimate_joint(
        self,
        legs: list[CopulaLeg],
        game_ctx: dict[str, CopulaGameContext] | None = None,
    ) -> dict[str, Any]:
        if not legs:
            return {"joint_prob": 0.0, "independent_prob": 0.0, "corr_multiplier": 1.0, "pair_correlations": []}
        if len(legs) == 1:
            p = float(legs[0].prob)
            return {"joint_prob": p, "independent_prob": p, "corr_multiplier": 1.0, "pair_correlations": []}

        game_ctx = game_ctx or {}
        groups: dict[str, list[int]] = {}
        order: list[str] = []
        anon = 0
        for i, leg in enumerate(legs):
            key = leg.game_id.strip()
            if not key:
                key = f"__independent_{anon}"
                anon += 1
            if key not in groups:
                order.append(key)
                groups[key] = []
            groups[key].append(i)

        indep_prob = 1.0
        joint_prob = 1.0
        max_jitter = 0.0
        pairs: list[dict[str, Any]] = []
        regimes_by_game: dict[str, str] = {}

        for gkey in order:
            idxs = groups[gkey]
            if len(idxs) == 1:
                p = float(legs[idxs[0]].prob)
                indep_prob *= p
                joint_prob *= p
                continue

            ctx = self._resolve_game_ctx(legs, idxs, game_ctx)
            regime, mat = self._select_matrix(ctx)
            regimes_by_game[gkey] = regime
            for idx in idxs:
                indep_prob *= float(legs[idx].prob)

            g_joint, g_jitter, g_pairs = self._estimate_game_joint(gkey, legs, idxs, ctx, mat)
            joint_prob *= g_joint
            max_jitter = max(max_jitter, g_jitter)
            pairs.extend(g_pairs)

        pairs.sort(
            key=lambda p: (
                -abs(p["correlation"]),
                -p["correlation"],
                p["leg_a_index"],
                p["leg_b_index"],
            )
        )
        corr_mult = joint_prob / indep_prob if indep_prob > 0.0 else 1.0
        return {
            "joint_prob": float(np.clip(joint_prob, 0.0, 1.0)),
            "independent_prob": float(np.clip(indep_prob, 0.0, 1.0)),
            "corr_multiplier": float(corr_mult),
            "pair_correlations": pairs,
            "regimes_by_game": regimes_by_game,
            "jitter_applied": float(max_jitter),
        }

    def _resolve_game_ctx(
        self,
        legs: list[CopulaLeg],
        idxs: list[int],
        game_ctx: dict[str, CopulaGameContext],
    ) -> CopulaGameContext:
        gid = legs[idxs[0]].game_id.strip()
        if gid in game_ctx:
            return game_ctx[gid]
        fb = CopulaGameContext()
        for idx in idxs:
            leg = legs[idx]
            if not fb.has_spread and leg.home_spread != 0.0 and not math.isnan(leg.home_spread):
                fb.home_spread = leg.home_spread
                fb.has_spread = True
            if not fb.has_total and leg.game_total > 0.0 and not math.isnan(leg.game_total):
                fb.game_total = leg.game_total
                fb.has_total = True
        return fb

    def _select_matrix(self, ctx: CopulaGameContext) -> tuple[str, np.ndarray]:
        regime = select_regime(ctx)
        if regime in self.regime_matrices:
            return regime, self.regime_matrices[regime]
        if REGIME_NEUTRAL in self.regime_matrices:
            return REGIME_NEUTRAL, self.regime_matrices[REGIME_NEUTRAL]
        return REGIME_NEUTRAL, self.default_matrix

    def _resolve_leg(self, orig_idx: int, leg: CopulaLeg, ref_team: str) -> dict[str, Any]:
        market = leg.market.strip().upper()
        side = leg.side.strip().upper()
        sign = -1.0 if side in ("UNDER", "NO") else 1.0
        team = leg.team.strip()
        is_opp = bool(ref_team and team and team.lower() != ref_team.lower())

        if market in ("SPREAD", "ML"):
            if is_opp:
                sign = -sign
            return {
                "orig_idx": orig_idx,
                "leg": leg,
                "market": market,
                "node_idx": self.node_index.get("TEAM_MARGIN", -1),
                "node_name": "TEAM_MARGIN",
                "sign": sign,
                "loading": 1.0,
                "game_level": True,
            }
        if market == "TOTAL":
            return {
                "orig_idx": orig_idx,
                "leg": leg,
                "market": market,
                "node_idx": self.node_index.get("GAME_TOTAL", -1),
                "node_name": "GAME_TOTAL",
                "sign": sign,
                "loading": 1.0,
                "game_level": True,
            }
        if market == "TEAM_TOTAL":
            prefix = "OPP_" if is_opp else ""
            name = f"{prefix}TEAM_TOTAL"
            if name in self.node_index:
                return {
                    "orig_idx": orig_idx,
                    "leg": leg,
                    "market": market,
                    "node_idx": self.node_index[name],
                    "node_name": name,
                    "sign": sign,
                    "loading": 1.0,
                    "game_level": True,
                }
            proxy = f"{prefix}QB_PASS_TDS"
            idx = self.node_index.get(proxy, -1)
            if idx < 0:
                idx = self.node_index.get("GAME_TOTAL", -1)
            return {
                "orig_idx": orig_idx,
                "leg": leg,
                "market": market,
                "node_idx": idx,
                "node_name": name,
                "sign": sign,
                "loading": 0.75,
                "game_level": True,
            }

        prefix = "OPP_" if is_opp else ""
        role = _normalize_copula_role(leg.role, market)
        canonical = f"{prefix}{role}_{market}"
        if canonical in self.node_index:
            return {
                "orig_idx": orig_idx,
                "leg": leg,
                "market": market,
                "node_idx": self.node_index[canonical],
                "node_name": canonical,
                "sign": sign,
                "loading": 1.0,
                "game_level": False,
            }
        idx = -1
        loading = 1.0
        for pname, pload in _proxy_candidates(prefix, role, market):
            if pname in self.node_index:
                idx = self.node_index[pname]
                loading = pload
                break
        return {
            "orig_idx": orig_idx,
            "leg": leg,
            "market": market,
            "node_idx": idx,
            "node_name": canonical,
            "sign": sign,
            "loading": loading,
            "game_level": False,
        }

    def _estimate_game_joint(
        self,
        game_id: str,
        legs: list[CopulaLeg],
        idxs: list[int],
        ctx: CopulaGameContext,
        mat: np.ndarray,
    ) -> tuple[float, float, list[dict[str, Any]]]:
        ref_team = next((legs[i].team.strip() for i in idxs if legs[i].team.strip()), ctx.home_team.strip())
        resolved = [self._resolve_leg(i, legs[i], ref_team) for i in idxs]
        m = len(resolved)
        sub = np.eye(m, dtype=np.float64)
        z = np.empty(m, dtype=np.float64)
        pairs: list[dict[str, Any]] = []

        for i in range(m):
            p_clamped = float(np.clip(resolved[i]["leg"].prob, MIN_COPULA_PROB, MAX_COPULA_PROB))
            z[i] = norm_inv(p_clamped)
            for j in range(i + 1, m):
                rho, within = self._pair_corr(resolved[i], resolved[j], mat)
                sub[i, j] = rho
                sub[j, i] = rho
                pairs.append(
                    {
                        "leg_a_index": resolved[i]["orig_idx"],
                        "leg_b_index": resolved[j]["orig_idx"],
                        "leg_a_id": resolved[i]["leg"].leg_id,
                        "leg_b_id": resolved[j]["leg"].leg_id,
                        "game_id": game_id,
                        "node_a": resolved[i]["node_name"],
                        "node_b": resolved[j]["node_name"],
                        "correlation": rho,
                        "within_player": within,
                    }
                )

        chol, jitter = _cholesky_with_jitter(sub)
        normals = get_qmc_normals()
        if m <= MAX_QMC_DIMS:
            u = normals[:, :m]
            sim = u @ chol.T
            joint_prob = float(np.mean(np.all(sim <= z[None, :], axis=1)))
        else:
            # Matches Go Halton extension past 32 dims
            u_full = np.empty((QMC_SAMPLES, m), dtype=np.float64)
            u_full[:, :MAX_QMC_DIMS] = normals
            for d in range(MAX_QMC_DIMS, m):
                prime = HALTON_EXTRA_PRIMES[(d - MAX_QMC_DIMS) % len(HALTON_EXTRA_PRIMES)]
                u_full[:, d] = [norm_inv(_halton_point(r + 1, prime)) for r in range(QMC_SAMPLES)]
            sim = u_full @ chol.T
            joint_prob = float(np.mean(np.all(sim <= z[None, :], axis=1)))

        return joint_prob, jitter, pairs

    def _pair_corr(self, a: dict[str, Any], b: dict[str, Any], mat: np.ndarray) -> tuple[float, bool]:
        rho = 0.0
        if a["node_idx"] >= 0 and b["node_idx"] >= 0:
            ai, bi = a["node_idx"], b["node_idx"]
            rho = 0.5 * (float(mat[ai, bi]) + float(mat[bi, ai])) * a["loading"] * b["loading"]
        within = False
        if not a["game_level"] and not b["game_level"]:
            if _is_same_player(a["leg"], b["leg"]):
                within = True
                if a["market"] == b["market"]:
                    rho = 1.0
                else:
                    wp = _lookup_within_player(self.within_player, a["market"], b["market"])
                    if wp is not None:
                        rho = wp
            elif a["node_idx"] >= 0 and a["node_idx"] == b["node_idx"]:
                rho *= DISTINCT_PLAYER_SAME_NODE_CORR
        signed = float(np.clip(a["sign"] * b["sign"] * rho, -1.0, 1.0))
        return signed, within


def _halton_point(index: int, base: int) -> float:
    f = 1.0
    r = 0.0
    i = index
    fb = float(base)
    while i > 0:
        f /= fb
        r += f * float(i % base)
        i //= base
    return float(np.clip(r, 1e-12, 1.0 - 1e-12))


def _build_game_node_table(feat_df: pd.DataFrame) -> pd.DataFrame:
    """Build perspective-symmetrized game residual rows over CANONICAL_COPULA_NODES."""
    needed = feat_df.copy()
    # Residualize player stats around trailing pre-game EWMA (falls back to 0 when missing)
    needed["res_pass_yds"] = needed["pass_yds"] - needed["ewma_pass_yds"].fillna(needed["pass_yds"].mean())
    needed["res_pass_tds"] = needed["pass_tds"] - needed["ewma_pass_tds"].fillna(needed["pass_tds"].mean())
    needed["res_rec_yds"] = needed["rec_yds"] - needed["ewma_rec_yds"].fillna(needed["rec_yds"].mean())
    needed["res_receptions"] = needed["receptions"] - needed["ewma_receptions"].fillna(needed["receptions"].mean())
    needed["res_rush_yds"] = needed["rush_yds"] - needed["ewma_rush_yds"].fillna(needed["rush_yds"].mean())
    needed["res_rush_att"] = needed["carries"] - needed["ewma_carries"].fillna(needed["carries"].mean())

    # Map each team-game's primary role holders
    role_col_map = {
        "QB": [("PASS_YDS", "res_pass_yds"), ("PASS_TDS", "res_pass_tds")],
        "WR1": [("REC_YDS", "res_rec_yds"), ("RECEPTIONS", "res_receptions")],
        "WR2": [("REC_YDS", "res_rec_yds"), ("RECEPTIONS", "res_receptions")],
        "TE1": [("REC_YDS", "res_rec_yds"), ("RECEPTIONS", "res_receptions")],
        "RB1": [("RUSH_YDS", "res_rush_yds"), ("RUSH_ATT", "res_rush_att"), ("REC_YDS", "res_rec_yds")],
        "RB2": [("RUSH_YDS", "res_rush_yds")],
    }

    game_teams: dict[tuple[str, str], dict[str, Any]] = {}
    for _, row in needed.iterrows():
        gid = str(row["game_id"])
        team = str(row["team"])
        opp = str(row["opponent"])
        if not gid or not team or not opp:
            continue
        rec = game_teams.setdefault(
            (gid, team),
            {
                "game_id": gid,
                "team": team,
                "opponent": opp,
                "abs_spread": float(row["abs_spread"]) if pd.notna(row["abs_spread"]) else 0.0,
                "game_total": float(row["game_total"]) if pd.notna(row["game_total"]) else 44.0,
                "TEAM_MARGIN": (
                    float(row["actual_team_margin"]) + float(row["team_spread"])
                    if pd.notna(row["actual_team_margin"]) and pd.notna(row["team_spread"])
                    else np.nan
                ),
                "GAME_TOTAL": (
                    float(row["actual_game_total"]) - float(row["game_total"])
                    if pd.notna(row["actual_game_total"]) and pd.notna(row["game_total"])
                    else np.nan
                ),
            },
        )
        role = str(row["role"])
        if role in role_col_map:
            for mkt, col in role_col_map[role]:
                node_name = f"{role}_{mkt}"
                if node_name not in rec:
                    rec[node_name] = float(row[col])

    # Join opponent side stats for OPP_* nodes
    rows: list[dict[str, Any]] = []
    opp_nodes = {
        "OPP_QB_PASS_YDS": "QB_PASS_YDS",
        "OPP_QB_PASS_TDS": "QB_PASS_TDS",
        "OPP_WR1_REC_YDS": "WR1_REC_YDS",
        "OPP_WR2_REC_YDS": "WR2_REC_YDS",
        "OPP_TE1_REC_YDS": "TE1_REC_YDS",
        "OPP_RB1_RUSH_YDS": "RB1_RUSH_YDS",
    }
    for (gid, team), rec in game_teams.items():
        opp_rec = game_teams.get((gid, rec["opponent"]))
        full = dict(rec)
        if opp_rec is not None:
            for dst_node, src_node in opp_nodes.items():
                if src_node in opp_rec:
                    full[dst_node] = opp_rec[src_node]
        rows.append(full)

    return pd.DataFrame(rows)


def _spearman_gaussian_matrix(
    df: pd.DataFrame,
    nodes: tuple[str, ...],
    prior_mat: np.ndarray | None = None,
    shrink_n: float = 120.0,
) -> np.ndarray:
    """Estimate Spearman -> Gaussian copula correlation matrix with empirical Bayes shrinkage."""
    n = len(nodes)
    mat = np.eye(n, dtype=np.float64) if prior_mat is None else prior_mat.copy()

    for i in range(n):
        col_i = nodes[i]
        if col_i not in df.columns:
            continue
        vi = df[col_i].to_numpy(dtype=np.float64)
        for j in range(i + 1, n):
            col_j = nodes[j]
            if col_j not in df.columns:
                continue
            vj = df[col_j].to_numpy(dtype=np.float64)
            mask = np.isfinite(vi) & np.isfinite(vj)
            k = int(np.sum(mask))
            if k < 25:
                continue
            rho_s, _ = stats.spearmanr(vi[mask], vj[mask])
            if np.isnan(rho_s):
                continue
            # Convert Spearman rank correlation to Gaussian copula Pearson parameter
            rho_g = float(2.0 * math.sin(math.pi * float(rho_s) / 6.0))
            prior_rho = float(prior_mat[i, j]) if prior_mat is not None else 0.0
            shrunk = (k * rho_g + shrink_n * prior_rho) / (k + shrink_n)
            mat[i, j] = shrunk
            mat[j, i] = shrunk

    return higham_nearest_psd_correlation(mat)


def estimate_copula_artifact(train_feat_df: pd.DataFrame) -> CopulaModel:
    """Estimate PSD default, regime, and within-player copula matrices from training seasons."""
    node_df = _build_game_node_table(train_feat_df)
    default_mat = _spearman_gaussian_matrix(node_df, CANONICAL_COPULA_NODES, prior_mat=None, shrink_n=60.0)

    neutral_df = node_df[(node_df["abs_spread"] < HEAVY_FAVORITE_SPREAD) & (node_df["game_total"] < SHOOTOUT_TOTAL)]
    hf_df = node_df[node_df["abs_spread"] >= HEAVY_FAVORITE_SPREAD]
    shootout_df = node_df[node_df["game_total"] >= SHOOTOUT_TOTAL]

    neutral_mat = _spearman_gaussian_matrix(neutral_df, CANONICAL_COPULA_NODES, prior_mat=default_mat, shrink_n=80.0)
    hf_mat = _spearman_gaussian_matrix(hf_df, CANONICAL_COPULA_NODES, prior_mat=default_mat, shrink_n=140.0)
    shootout_mat = _spearman_gaussian_matrix(shootout_df, CANONICAL_COPULA_NODES, prior_mat=default_mat, shrink_n=140.0)

    # Estimate within-player correlations from player-level residuals
    wp_pairs = (
        ("RECEPTIONS:REC_YDS", "receptions", "ewma_receptions", "rec_yds", "ewma_rec_yds", ("WR", "TE", "RB")),
        ("RUSH_ATT:RUSH_YDS", "carries", "ewma_carries", "rush_yds", "ewma_rush_yds", ("RB", "QB")),
        ("PASS_YDS:PASS_TDS", "pass_yds", "ewma_pass_yds", "pass_tds", "ewma_pass_tds", ("QB",)),
        ("REC_YDS:ANYTIME_TD", "rec_yds", "ewma_rec_yds", "anytime_td", "ewma_exp_rec_tds", ("WR", "TE", "RB")),
        ("RUSH_YDS:ANYTIME_TD", "rush_yds", "ewma_rush_yds", "anytime_td", "ewma_exp_rush_tds", ("RB", "QB")),
        ("RECEPTIONS:ANYTIME_TD", "receptions", "ewma_receptions", "anytime_td", "ewma_exp_rec_tds", ("WR", "TE", "RB")),
    )
    within_player: dict[str, float] = {}
    for key, col_a, exp_a, col_b, exp_b, pos_list in wp_pairs:
        sub = train_feat_df[train_feat_df["position"].isin(pos_list)]
        ra = (sub[col_a] - sub[exp_a].fillna(0.0)).to_numpy(dtype=np.float64)
        rb = (sub[col_b] - sub[exp_b].fillna(0.0)).to_numpy(dtype=np.float64)
        mask = np.isfinite(ra) & np.isfinite(rb)
        if np.sum(mask) >= 30:
            rho_s, _ = stats.spearmanr(ra[mask], rb[mask])
            if not np.isnan(rho_s):
                rho_g = float(np.clip(2.0 * math.sin(math.pi * float(rho_s) / 6.0), -0.95, 0.95))
                within_player[key] = rho_g
                continue
        within_player[key] = 0.50

    return CopulaModel(
        nodes=list(CANONICAL_COPULA_NODES),
        default_matrix=default_mat.tolist(),
        regime_matrices={
            REGIME_NEUTRAL: neutral_mat.tolist(),
            REGIME_HEAVY_FAVORITE: hf_mat.tolist(),
            REGIME_SHOOTOUT: shootout_mat.tolist(),
        },
        within_player=within_player,
    )


def evaluate_sgp_log_loss(
    copula_model: CopulaModel,
    test_feat_df: pd.DataFrame,
    leg_predictions: pd.DataFrame | None = None,
) -> dict[str, Any]:
    """Evaluate Gaussian copula SGP Log Loss vs naive independence on held-out multi-leg slips."""
    # Construct canonical 2-leg and 3-leg same-game parlays per game where roles exist:
    # (1) QB PASS_YDS + WR1 REC_YDS (positive stack)
    # (2) QB PASS_YDS + WR1 REC_YDS + GAME_TOTAL Over (3-leg positive stack)
    # (3) RB1 RUSH_YDS + TEAM_MARGIN Cover (positive game-script stack)
    # (4) QB PASS_YDS + OPP_QB_PASS_YDS (shootout stack)
    if leg_predictions is not None and not leg_predictions.empty:
        pred_lookup = leg_predictions.set_index(["game_id", "player_id", "market"]).to_dict("index")
    else:
        pred_lookup = {}

    by_game: dict[str, dict[tuple[str, str], pd.Series]] = {}
    for _, row in test_feat_df.iterrows():
        role = str(row["role"])
        if role not in ("QB", "WR1", "RB1"):
            continue
        gid = str(row["game_id"])
        by_game.setdefault(gid, {})[(str(row["team"]), role)] = row

    joint_probs: list[float] = []
    indep_probs: list[float] = []
    outcomes: list[float] = []

    for gid, holders in by_game.items():
        teams = sorted({t for t, _ in holders})
        for team in teams:
            qb = holders.get((team, "QB"))
            wr1 = holders.get((team, "WR1"))
            rb1 = holders.get((team, "RB1"))

            if qb is not None and wr1 is not None and pd.notna(qb["ewma_pass_yds"]) and pd.notna(wr1["ewma_rec_yds"]):
                qb_line = max(150.5, round(float(qb["ewma_pass_yds"])) - 0.5)
                wr_line = max(25.5, round(float(wr1["ewma_rec_yds"])) - 0.5)
                p_qb = float(pred_lookup.get((gid, qb["player_id"], "PASS_YDS"), {}).get("prob", 0.50))
                p_wr = float(pred_lookup.get((gid, wr1["player_id"], "REC_YDS"), {}).get("prob", 0.50))
                ctx = CopulaGameContext(
                    home_team=team if bool(qb["is_home"]) else str(qb["opponent"]),
                    away_team=str(qb["opponent"]) if bool(qb["is_home"]) else team,
                    home_spread=float(qb["home_spread"]) if pd.notna(qb["home_spread"]) else 0.0,
                    game_total=float(qb["game_total"]) if pd.notna(qb["game_total"]) else 44.0,
                    has_spread=pd.notna(qb["home_spread"]),
                    has_total=pd.notna(qb["game_total"]),
                )
                est = copula_model.estimate_joint(
                    [
                        CopulaLeg(1, gid, team, str(qb["player_id"]), str(qb["player_name"]), "QB", "PASS_YDS", "OVER", p_qb),
                        CopulaLeg(2, gid, team, str(wr1["player_id"]), str(wr1["player_name"]), "WR1", "REC_YDS", "OVER", p_wr),
                    ],
                    {gid: ctx},
                )
                hit = 1.0 if (float(qb["pass_yds"]) > qb_line and float(wr1["rec_yds"]) > wr_line) else 0.0
                joint_probs.append(est["joint_prob"])
                indep_probs.append(est["independent_prob"])
                outcomes.append(hit)

                # Same-player QB PASS_YDS + PASS_TDS stack
                p_td = float(pred_lookup.get((gid, qb["player_id"], "PASS_TDS"), {}).get("prob", 0.52))
                est_qb = copula_model.estimate_joint(
                    [
                        CopulaLeg(1, gid, team, str(qb["player_id"]), str(qb["player_name"]), "QB", "PASS_YDS", "OVER", p_qb),
                        CopulaLeg(2, gid, team, str(qb["player_id"]), str(qb["player_name"]), "QB", "PASS_TDS", "OVER", p_td),
                    ],
                    {gid: ctx},
                )
                hit_qb = 1.0 if (float(qb["pass_yds"]) > qb_line and float(qb["pass_tds"]) > 1.5) else 0.0
                joint_probs.append(est_qb["joint_prob"])
                indep_probs.append(est_qb["independent_prob"])
                outcomes.append(hit_qb)

            if rb1 is not None and pd.notna(rb1["ewma_rush_yds"]) and pd.notna(rb1["actual_team_margin"]) and pd.notna(rb1["team_spread"]):
                rb_line = max(25.5, round(float(rb1["ewma_rush_yds"])) - 0.5)
                p_rb = float(pred_lookup.get((gid, rb1["player_id"], "RUSH_YDS"), {}).get("prob", 0.50))
                ctx = CopulaGameContext(
                    home_team=team if bool(rb1["is_home"]) else str(rb1["opponent"]),
                    away_team=str(rb1["opponent"]) if bool(rb1["is_home"]) else team,
                    home_spread=float(rb1["home_spread"]) if pd.notna(rb1["home_spread"]) else 0.0,
                    game_total=float(rb1["game_total"]) if pd.notna(rb1["game_total"]) else 44.0,
                    has_spread=pd.notna(rb1["home_spread"]),
                    has_total=pd.notna(rb1["game_total"]),
                )
                est_rb = copula_model.estimate_joint(
                    [
                        CopulaLeg(1, gid, team, str(rb1["player_id"]), str(rb1["player_name"]), "RB1", "RUSH_YDS", "OVER", p_rb),
                        CopulaLeg(2, gid, team, "", "", "", "SPREAD", "OVER", 0.50),
                    ],
                    {gid: ctx},
                )
                covered = float(rb1["actual_team_margin"]) + float(rb1["team_spread"]) > 0.0
                hit_rb = 1.0 if (float(rb1["rush_yds"]) > rb_line and covered) else 0.0
                joint_probs.append(est_rb["joint_prob"])
                indep_probs.append(est_rb["independent_prob"])
                outcomes.append(hit_rb)

    if not outcomes:
        return {"sgp_count": 0, "sgp_log_loss": 0.0, "naive_indep_log_loss": 0.0}

    y_arr = np.asarray(outcomes, dtype=np.float64)
    return {
        "sgp_count": len(outcomes),
        "sgp_log_loss": binary_log_loss(np.asarray(joint_probs, dtype=np.float64), y_arr),
        "naive_indep_log_loss": binary_log_loss(np.asarray(indep_probs, dtype=np.float64), y_arr),
    }
