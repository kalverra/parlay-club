"""Shared deterministic synthetic multi-season NFL fixtures for offline ml/ tests."""

from __future__ import annotations

import numpy as np
import polars as pl
import pytest

from data import assemble_joined_tables


def build_synthetic_nfl_dataset(
    seasons: list[int] | None = None,
    n_weeks: int = 10,
    seed: int = 123,
) -> tuple[pl.DataFrame, pl.DataFrame]:
    """Generate a realistic multi-season NFL dataset with correlated game scripts and props."""
    seasons = seasons or [2021, 2022, 2023, 2024]
    rng = np.random.default_rng(seed)
    teams = ["KC", "BUF", "SF", "DAL", "PHI", "DET"]
    n_teams = len(teams)

    # Roster template per team: (suffix, pos, base_target_share, base_carry_share, base_snap)
    roster_tpl = [
        ("QB", "QB", 0.0, 0.12, 0.98),
        ("WR1", "WR", 0.27, 0.01, 0.90),
        ("WR2", "WR", 0.18, 0.01, 0.78),
        ("TE1", "TE", 0.17, 0.0, 0.80),
        ("RB1", "RB", 0.10, 0.58, 0.68),
        ("RB2", "RB", 0.06, 0.26, 0.32),
    ]

    players_rows = []
    for t in teams:
        for role_tag, pos, _, _, _ in roster_tpl:
            players_rows.append(
                {
                    "gsis_id": f"GSIS_{t}_{role_tag}",
                    "pfr_id": f"PFR_{t}_{role_tag}",
                    "display_name": f"{t} {role_tag}",
                    "position": pos,
                }
            )
    players_df = pl.DataFrame(players_rows)

    ps_rows = []
    ts_rows = []
    sc_rows = []
    ff_rows = []
    sched_rows = []

    for season in seasons:
        team_strength = {t: float(rng.normal(0.0, 3.5)) for t in teams}
        for week in range(1, n_weeks + 1):
            perm = rng.permutation(n_teams)
            for m in range(0, n_teams, 2):
                home = teams[int(perm[m])]
                away = teams[int(perm[m + 1])]
                gid = f"{season}_{week:02d}_{away}_{home}"

                # nflverse spread_line: positive = home favored
                spread_line = round(float(np.clip(team_strength[home] - team_strength[away] + rng.normal(1.5, 2.0), -13.5, 13.5)) * 2) / 2
                total_line = round(float(np.clip(45.0 + rng.normal(0.0, 4.5), 37.5, 55.5)) * 2) / 2
                roof = "dome" if home in ("DET", "DAL") else "outdoors"
                wind = 0.0 if roof == "dome" else float(np.clip(rng.normal(9.0, 5.0), 0.0, 24.0))
                temp = 70.0 if roof == "dome" else float(np.clip(rng.normal(58.0, 14.0), 22.0, 88.0))

                # Latent correlated game-environment shocks
                pace_shock = float(rng.normal(0.0, 1.0))
                margin_shock = float(rng.normal(0.0, 9.0))
                actual_margin = spread_line + margin_shock
                actual_total = float(max(14.0, total_line + pace_shock * 7.5 + abs(margin_shock) * 0.15))
                home_score = max(0.0, round((actual_total + actual_margin) / 2.0))
                away_score = max(0.0, round((actual_total - actual_margin) / 2.0))

                sched_rows.append(
                    {
                        "game_id": gid,
                        "season": season,
                        "game_type": "REG",
                        "week": week,
                        "home_team": home,
                        "away_team": away,
                        "spread_line": spread_line,
                        "total_line": total_line,
                        "roof": roof,
                        "temp_f": temp,
                        "wind_mph": wind,
                        "home_score": home_score,
                        "away_score": away_score,
                        "actual_home_margin": home_score - away_score,
                        "actual_game_total": home_score + away_score,
                    }
                )

                for team, opp, is_home in ((home, away, True), (away, home, False)):
                    t_margin = (home_score - away_score) if is_home else (away_score - home_score)
                    t_spread = -spread_line if is_home else spread_line
                    imp_tot = (total_line - t_spread) / 2.0
                    pass_shock = float(rng.normal(0.0, 1.0)) + 0.55 * pace_shock - 0.15 * (t_margin / 14.0)
                    rush_shock = float(rng.normal(0.0, 1.0)) + 0.50 * (t_margin / 10.0) + 0.20 * pace_shock

                    team_att = int(np.clip(round(34 + 2.5 * pass_shock - 0.15 * wind), 20, 52))
                    team_carries = int(np.clip(round(26 + 3.0 * rush_shock), 14, 42))
                    pass_epa = float(0.08 * pass_shock + rng.normal(0.0, 0.08))
                    rush_epa = float(0.06 * rush_shock + rng.normal(0.0, 0.07))

                    ts_rows.append(
                        {
                            "season": season,
                            "week": week,
                            "season_type": "REG",
                            "team": team,
                            "opponent_team": opp,
                            "passing_epa": pass_epa,
                            "rushing_epa": rush_epa,
                            "attempts": team_att,
                            "carries": team_carries,
                        }
                    )

                    # Simulate each player cleanly
                    qb_pass_yds = max(60.0, 235.0 + (imp_tot - 22.5) * 4.5 + 38.0 * pass_shock - 1.5 * wind + rng.normal(0.0, 24.0))
                    qb_pass_tds = int(np.clip(rng.poisson(max(0.4, 1.5 + 0.08 * (imp_tot - 22.5) + 0.45 * pass_shock)), 0, 5))

                    for role_tag, pos, t_share, c_share, snap in roster_tpl:
                        pid = f"GSIS_{team}_{role_tag}"
                        pfr_id = f"PFR_{team}_{role_tag}"
                        if pos == "QB":
                            att = team_att
                            carries = int(np.clip(rng.poisson(3.5), 0, 11))
                            rush_yds = float(max(0.0, carries * 4.5 + rng.normal(0.0, 6.0)))
                            pass_yds = float(round(qb_pass_yds))
                            pass_tds = qb_pass_tds
                            rush_tds = int(rng.binomial(1, 0.16))
                            targets = 0
                            receptions = 0
                            rec_yds = 0.0
                            rec_tds = 0
                            exp_pass_yds = float(230.0 + (imp_tot - 22.5) * 3.5)
                            exp_rush_yds = float(carries * 4.2)
                            exp_rec_yds = 0.0
                            exp_rec = 0.0
                            exp_pass_tds = float(max(0.4, 1.45 + 0.06 * (imp_tot - 22.5)))
                            exp_rush_tds = 0.16
                            exp_rec_tds = 0.0
                            cpoe = float(rng.normal(1.2 * pass_shock, 4.5))
                        elif pos == "RB":
                            att = 0
                            pass_yds = 0.0
                            pass_tds = 0
                            carries = int(np.clip(round(team_carries * c_share + rng.normal(0.0, 2.0)), 1, 28))
                            rush_yds = float(max(0.0, round(carries * 4.3 + 12.0 * rush_shock + rng.normal(0.0, 14.0))))
                            rush_tds = int(rng.binomial(1, 0.36 if role_tag == "RB1" else 0.16))
                            targets = int(np.clip(rng.poisson(max(0.5, team_att * t_share)), 0, 10))
                            receptions = int(rng.binomial(targets, 0.76)) if targets > 0 else 0
                            rec_yds = float(max(0.0, round(receptions * 7.5 + rng.normal(0.0, 6.0))))
                            rec_tds = int(rng.binomial(1, 0.09 if role_tag == "RB1" else 0.04))
                            exp_pass_yds = 0.0
                            exp_rush_yds = float(carries * 4.2)
                            exp_rec_yds = float(targets * 5.8)
                            exp_rec = float(targets * 0.74)
                            exp_pass_tds = 0.0
                            exp_rush_tds = 0.35 if role_tag == "RB1" else 0.15
                            exp_rec_tds = 0.08 if role_tag == "RB1" else 0.04
                            cpoe = 0.0
                        else:
                            att = 0
                            pass_yds = 0.0
                            pass_tds = 0
                            carries = 0
                            rush_yds = 0.0
                            rush_tds = 0
                            targets = int(np.clip(rng.poisson(max(1.0, team_att * t_share + 0.8 * pass_shock)), 0, 15))
                            catch_rate = 0.67 if pos == "WR" else 0.73
                            receptions = int(rng.binomial(targets, catch_rate)) if targets > 0 else 0
                            ypr = 13.0 if role_tag == "WR1" else (11.5 if role_tag == "WR2" else 10.2)
                            rec_yds = float(max(0.0, round(receptions * ypr + 8.0 * pass_shock + rng.normal(0.0, 12.0))))
                            td_prob = 0.36 if role_tag == "WR1" else (0.24 if role_tag == "TE1" else 0.20)
                            rec_tds = int(rng.binomial(1, float(np.clip(td_prob + 0.05 * pass_shock, 0.05, 0.75))))
                            exp_pass_yds = 0.0
                            exp_rush_yds = 0.0
                            exp_rec_yds = float(targets * catch_rate * ypr)
                            exp_rec = float(targets * catch_rate)
                            exp_pass_tds = 0.0
                            exp_rush_tds = 0.0
                            exp_rec_tds = td_prob
                            cpoe = 0.0

                        ps_rows.append(
                            {
                                "player_id": pid,
                                "player_display_name": f"{team} {role_tag}",
                                "position": pos,
                                "team": team,
                                "opponent_team": opp,
                                "season": season,
                                "week": week,
                                "season_type": "REG",
                                "target_share": float(targets / max(team_att, 1)),
                                "air_yards_share": float(t_share * 1.15) if pos in ("WR", "TE") else 0.0,
                                "wopr": float(1.5 * (targets / max(team_att, 1)) + 0.7 * (t_share * 1.15)) if pos in ("WR", "TE") else 0.0,
                                "passing_epa": pass_epa if pos == "QB" else 0.0,
                                "rushing_epa": rush_epa * c_share if pos in ("RB", "QB") else 0.0,
                                "receiving_epa": pass_epa * t_share if pos in ("WR", "TE", "RB") else 0.0,
                                "passing_cpoe": cpoe,
                                "targets": targets,
                                "carries": carries,
                                "attempts": att,
                                "receptions": receptions,
                                "receiving_yards": rec_yds,
                                "rushing_yards": rush_yds,
                                "passing_yards": pass_yds,
                                "passing_tds": pass_tds,
                                "rushing_tds": rush_tds,
                                "receiving_tds": rec_tds,
                            }
                        )
                        sc_rows.append(
                            {
                                "pfr_player_id": pfr_id,
                                "player": f"{team} {role_tag}",
                                "team": team,
                                "season": season,
                                "game_type": "REG",
                                "week": week,
                                "offense_pct": float(np.clip(snap + rng.normal(0.0, 0.04), 0.1, 1.0)),
                            }
                        )
                        ff_rows.append(
                            {
                                "season": season,
                                "week": week,
                                "posteam": team,
                                "player_id": pid,
                                "receptions_exp": exp_rec,
                                "pass_yards_gained_exp": exp_pass_yds,
                                "rec_yards_gained_exp": exp_rec_yds,
                                "rush_yards_gained_exp": exp_rush_yds,
                                "pass_touchdown_exp": exp_pass_tds,
                                "rec_touchdown_exp": exp_rec_tds,
                                "rush_touchdown_exp": exp_rush_tds,
                            }
                        )

    schedules_df = pl.DataFrame(sched_rows).with_columns(
        pl.lit(2).alias("season_type"),
        (-pl.col("spread_line")).alias("home_spread"),
        pl.col("total_line").alias("game_total"),
    )

    return assemble_joined_tables(
        player_stats=pl.DataFrame(ps_rows),
        team_stats=pl.DataFrame(ts_rows),
        snap_counts=pl.DataFrame(sc_rows),
        players=players_df,
        schedules=schedules_df,
        ff_opportunity=pl.DataFrame(ff_rows),
    )


@pytest.fixture(scope="session")
def synthetic_nfl_data() -> tuple[pl.DataFrame, pl.DataFrame]:
    return build_synthetic_nfl_dataset(seasons=[2021, 2022, 2023, 2024], n_weeks=8, seed=123)
