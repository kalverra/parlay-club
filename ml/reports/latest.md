# v2 Model Walk-Forward Validation Report

- **Version:** `v2.1.0`
- **Trained seasons:** `[2019, 2020, 2021, 2022, 2023, 2024, 2025]`
- **Held-out seasons:** `[2023, 2024, 2025]`
- **Built at:** `2026-11-25T23:48:25Z`

## Calibration & Scoring Gates

| Metric | Value | Baseline / Target |
| --- | --- | --- |
| Walk-forward Log Loss | `0.603852` | Market `0.624527` |
| Walk-forward Brier | `0.208707` | Market `0.216153` |
| Classwise ECE | `0.008518` | Lower is better |
| Populated bins | `9/10` | `>= 80%` |
| Copula SGP Log Loss | `0.590423` | Naive `0.618393` |
| EWMA half-life +/-1w MAE | `0.009546` | `<= 0.05` |

## Per-Market Calibration & Collinearity Pruning

| Market | Family | Blend Weight | Calibrator | Pruned Pairs (Spearman > 0.7) |
| --- | --- | --- | --- | --- |
| `ANYTIME_TD` | `bernoulli` | `0.700` | `platt` | ewma_rush_tds (0.8355), ewma_rec_tds (0.8041), ewma_carry_share (0.8303), ewma_exp_rec_tds (0.7946), team_spread (0.8062) |
| `PASS_TDS` | `poisson` | `0.700` | `platt` | ewma_exp_pass_tds (0.75), ewma_pass_yds (0.7081), team_spread (0.8114), opp_def_pass_epa_allowed (0.9686) |
| `PASS_YDS` | `zero_adj_gamma` | `0.700` | `platt` | ewma_exp_pass_yds (0.9011), ewma_pass_att (0.8492), team_spread (0.8114), opp_def_pass_epa_allowed (0.9686), wind_mph (0.8013) |
| `RECEPTIONS` | `negbin` | `0.700` | `platt` | ewma_exp_receptions (0.9811), ewma_target_share (0.9521), ewma_targets (0.9668), ewma_wopr (0.8708), ewma_offense_pct (0.8124), team_spread (0.8055) |
| `REC_YDS` | `zero_adj_gamma` | `0.700` | `platt` | ewma_exp_rec_yds (0.9558), ewma_target_share (0.9282), ewma_air_yards_share (0.77), ewma_wopr (0.9418), ewma_targets (0.8745), ewma_receptions (0.8591), ewma_offense_pct (0.7801), team_spread (0.8055) |
| `RUSH_ATT` | `negbin` | `0.700` | `platt` | ewma_carry_share (0.984), ewma_carries (0.9431), team_implied_total (0.8052), ewma_offense_pct (0.7823) |
| `RUSH_YDS` | `zero_adj_gamma` | `0.700` | `platt` | ewma_exp_rush_yds (0.9711), ewma_carries (0.9571), ewma_carry_share (0.9431), team_implied_total (0.8052) |

## Top Covariate Shifts (KS Two-Sample Test)

| Market | Feature | KS Stat | p-value | Drifted |
| --- | --- | --- | --- | --- |
| `ANYTIME_TD` | `team_plays_per_game` | `0.1999` | `4.216e-175` | `True` |
| `ANYTIME_TD` | `opp_def_rush_epa_allowed` | `0.1667` | `1.481e-121` | `True` |
| `ANYTIME_TD` | `game_total` | `0.0941` | `3.427e-41` | `False` |
| `PASS_TDS` | `team_pass_rate` | `0.1860` | `6.423e-17` | `True` |
| `PASS_TDS` | `ewma_pass_att` | `0.1513` | `6.744e-12` | `True` |
| `PASS_TDS` | `game_total` | `0.0990` | `2.464e-05` | `False` |
| `PASS_YDS` | `opp_role_yds_allowed` | `0.2870` | `5.331e-40` | `True` |
| `PASS_YDS` | `team_plays_per_game` | `0.2095` | `2.235e-21` | `True` |
| `PASS_YDS` | `team_pass_rate` | `0.1860` | `6.423e-17` | `True` |
| `RECEPTIONS` | `team_plays_per_game` | `0.1987` | `3.204e-154` | `True` |
| `RECEPTIONS` | `team_pass_rate` | `0.1953` | `5.323e-149` | `True` |
| `RECEPTIONS` | `game_total` | `0.0943` | `6.337e-37` | `False` |
| `REC_YDS` | `team_plays_per_game` | `0.1987` | `3.204e-154` | `True` |
| `REC_YDS` | `team_pass_rate` | `0.1953` | `5.323e-149` | `True` |
| `REC_YDS` | `game_total` | `0.0943` | `6.337e-37` | `False` |
| `RUSH_ATT` | `team_plays_per_game` | `0.1964` | `1.069e-64` | `True` |
| `RUSH_ATT` | `team_pass_rate` | `0.1887` | `9.815e-60` | `True` |
| `RUSH_ATT` | `opp_def_rush_epa_allowed` | `0.1708` | `6.617e-49` | `True` |
| `RUSH_YDS` | `team_plays_per_game` | `0.1964` | `1.069e-64` | `True` |
| `RUSH_YDS` | `team_pass_rate` | `0.1887` | `9.815e-60` | `True` |
| `RUSH_YDS` | `opp_def_rush_epa_allowed` | `0.1708` | `6.617e-49` | `True` |
