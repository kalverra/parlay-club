---
id: analyzer-rules
title: Analyzer rules
type: reference
summary: The stable IDs, severities, triggers, and example messages for all 18 analyzer rules, plus the v2 calibrated four-pillar grading model, Gaussian copula script scoring, extensible context overlays, and v1 rubric reference.
tags: [analyzer, rules, grades, pillars, copula, overlays, rubric, warnings, correlation, severity, slips, nfelo, nflverse, model, quarterback, props, player-stats]
---

# Analyzer rules

## Summary

`internal/analyzer` holds the sharp-friend checks and grading engines that run under a slip. `AnalyzeSlip(legs, games, stats)`, `EvaluateSlip(legs, games, stats)`, `ScoreLeg(leg, games, stats)`, and `ScoreSlip(legs, games, stats)` are pure and deterministic: no I/O, no clock, no network, and no LLM calls. `AnalyzeSlip` returns warnings with a stable rule ID, severity, message, and involved legs. `ScoreLeg` and `ScoreSlip` delegate to `ActiveEngine` (currently `EngineV2`, with `EngineV1` preserved in `BuiltInEngines()` for comparison). `RuleDocs()` and `GradeRubric()` expose live rule and rubric data for `/analyzer/guide`.

When nfelo's model is fresh, `hook` and `chalk` add a sentence about it, and `qb_drag` reads it for a downgraded quarterback. When player game logs exist, `median_trap` warns when a yardage over sits above recent game medians. When `nflverse` team EPA and player usage are fresh, `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk` fire alongside v2 role-archetype, EWMA, and expected-opportunity features. A game or leg without those optional inputs behaves cleanly and fails soft.

Rule IDs are stable across releases. A warning's severity is `Info`, `Warn`, or `Danger`.

## How rules see a slip

- Legs with market `OTHER`, an empty `GameID`, or a `GameID` missing from the game map are skipped by every rule and receive no grade (`Letter == ""`).
- Output is sorted by severity, `Danger` first, then by the first involved leg's position in the slip. Stable sorting keeps the same input in the same order.
- **Same team** means an equal `Team`. **Opponent** is the other team in the leg's game.
- **Favorite**: a `SPREAD` with a line below 0, or an `ML` with negative odds. **Underdog**: a `SPREAD` with a line above 0, or an `ML` with positive odds.
- **Passing/receiving over**: an `OVER` on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, or `RECEPTIONS`.
- **Model data**: nfelo's pre-game spread, win probability, and QB adjustments for the game. `LoadSlipSignals` attaches it only when the game is `SCHEDULED` and the last successful `nfelo:projections` fetch is under 72 hours old. Otherwise the game has no model, and the model sentences and `qb_drag` stay quiet.
- **MarketScript**: `Game.MarketScript` derives consensus `HomeSpread`, `GameTotal`, `HomeImpliedTotal`, and `AwayImpliedTotal` strictly from already-cached `SPREAD` and `TOTAL` odds rows.
- **Player stats & EWMA usage**: `stats` maps ESPN athlete IDs to recent game logs per market (most recent first), `Position` from `roster_players`, deterministic `Role` (`RoleArchetype`: `QB`, `WR1`, `WR2`, `TE1`, `RB1`, `RB2` ranked by trailing team target or carry/snap share), `PlayerUsage` (requiring `>= 3` weeks for usage warning rules), and `PlayerEWMAUsage` (3-week half-life EWMA populated whenever `>= 1` prior week exists, including `ff_opportunity` expected opportunity and `cpoe`).
- **Role defense allowed**: `Game.HomeRoleDefense` and `Game.AwayRoleDefense` map opposing `RoleArchetype` values to per-game yards, receptions, TDs, and EPA allowed (`RoleDefenseProfile`) when `nflverse:season:{year}` is under 7 days old.
- **Team injuries (roster + summary merge)**: `Game.Injuries` maps each team abbreviation to its merged injury list. `LoadSlipSignals` combines `games.injuries_json` (ESPN summary, kept primary) with key-position injuries (`QB`, `RB`, `WR`, `TE`, `LT`, `C`, `CB`, `S`) from `roster_players.injury_status`, backfilling any player truncated past ESPN's 5-per-team summary cap.
- **External context signals**: `Game.ContextSignals` and `PlayerStats.ContextSignals` accept optional structured `ContextSignal` events (`injury`, `news`, `line_move`, `inactive`) consumed by `external_context_signals` without model retraining.
- **nflverse team EPA**: `Game.HomeEPA` and `Game.AwayEPA` hold league ranks (`1..32`, where `1` is toughest defense, highest offense, fastest pace, or highest pass rate) for `DefPassEPAAllowedRank`, `DefRushEPAAllowedRank`, `OffPassEPARank`, `PlaysPerGameRank`, and `PassRateRank` when `nflverse:season:{year}` is under 7 days old.
- **Sharp-book benchmarks**: `Game.Sharp` maps `SharpKey` (`MarketType`, `Team`, normalized `PlayerName`, `Side`) to a de-vigged `SharpQuote` built by `BuildSharpBenchmarks` over `db.CurrentOdds`. Books are tiered: **Tier 1** (`Pinnacle`, `Circa`, `Bookmaker`), **Tier 2** (`LowVig`, `BetOnline`), and **Tier 3** (`market consensus` across `>= 3` traditional sportsbooks quoting the exact same line, excluding DFS pick'em apps and peer-to-peer exchanges). Two-way quotes pair when overround falls in `[0.95, 1.25]` and populate multiplicative `FairProb`, Power-devigged `PowerFairProb`, `VigPct`, and `BookCount`. One-sided markets (`ANYTIME_TD`) and unpaired quotes fail soft cleanly.
- When a rule's legs belong to more than one person, the message names each owner.

## Rule Set A: Correlation

| Rule                   | Severity | Fires when                                                                                          |
| ---------------------- | -------- | --------------------------------------------------------------------------------------------------- |
| `blowout_passing`      | `Warn`   | A spread at −7.0 or lower and a passing or receiving yards over on that team.                       |
| `trailing_rb`          | `Danger` | An underdog leg and a rushing over on that team.                                                    |
| `total_contradiction`  | `Danger` | A total under and at least two legs in that game that need points.                                  |
| `stack_clock_drain`    | `Info`   | A favorite spread, a rush over on that team, and a team total under on the opponent, same game.     |
| `stack_shootout`       | `Info`   | A pass over, a different receiver's over on the same team, and a game total over, same game.        |

### `blowout_passing` — Warn

A `SPREAD` leg with a line of −7.0 or lower and a `PASS_YDS` or `REC_YDS` `OVER` on the same team in the same game. One
warning per spread leg, listing every matching prop. A big favorite tends to run the clock in the fourth quarter, while
passing and receiving overs need the volume.

> If BUF -7.5 covers, they'll likely run the clock in Q4. Josh Allen Over 245.5 pairs better with a closer spread, or
> try MIA's WR for garbage time.

### `trailing_rb` — Danger

An underdog leg and a `RUSH_YDS` or `RUSH_ATT` `OVER` on the same team in the same game. The underdog leg is a `SPREAD`
with a line of +4.5 or higher, or any underdog `ML`. Trailing teams stop running, so the rush over fights the script.
One warning per underdog leg, listing every matching prop.

> MIA +6.5 means they're probably trailing, and trailing teams stop running. De'Von Achane's rush over fights that
> script.

### `total_contradiction` — Danger

A `TOTAL` `UNDER` and two or more legs in the same game that need points. The qualifying legs are `ANYTIME_TD` or any
passing/receiving over, on either team. One warning per total under.

> Under 44.5 needs a quiet game, but 2 other legs here need points. One of these sides is going to lose.

### `stack_clock_drain` — Info

A favorite `SPREAD`, a `RUSH_YDS` or `RUSH_ATT` `OVER` on the same team, and a `TEAM_TOTAL` `UNDER` on the opponent, all
in the same game. The three picks share one quiet, run-heavy script, so this is a friendly note rather than a warning.

> Nice stack: BUF -7.5 covers, James Cook grinds the clock, MIA Team Total Under 17.5 stays quiet. These all win
> together.

### `stack_shootout` — Info

A `PASS_YDS` `OVER`, a `REC_YDS` or `RECEPTIONS` `OVER` by a different player on the same team, and a `TOTAL` `OVER` in
the same game. One warning per passing over, using the first matching receiver and total in slip order.

> Shootout stack: Josh Allen Over 245.5 Pass Yds, Khalil Shakir Over 54.5 Rec Yds, game Over 47.5. If it's a track meet,
> all three hit.

## Rule Set B: Weather

These rules read only outdoor games with a known forecast. Indoor games, games with a nil `Weather`, and legs whose game
is missing from the map are skipped.

| Rule           | Severity                                                                                         | Fires when                                                                              |
| -------------- | ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------- |
| `wind`         | `Danger` at sustained wind ≥ 20 mph; otherwise `Warn` at sustained wind ≥ 15 mph or gusts ≥ 25 mph | A passing, receiving, kicking, or game-total `OVER` in that game.                       |
| `fake_weather` | `Info`                                                                                           | Precipitation > 0 and < 0.1 in/hr, wind < 10 mph, and a `TOTAL` `UNDER` in that game.   |

### `wind` — Danger or Warn

A windy forecast flags every `OVER` leg in the game on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, `RECEPTIONS`, `KICKER_PTS`, or
`TOTAL`. One warning per game, listing every flagged leg. Gusts only raise the level to `Warn`; the message names the
sustained wind when it meets the 15 mph bar, otherwise the gusts.

> Heads up: 22 mph winds in Buffalo today. Passing and kicking overs take a real hit in that, so Josh Allen Over 245.5
> Pass Yds gets tougher. Field goals drop to about 77%.

### `fake_weather` — Info

Light rain without wind is not the scoring brake the slip might expect. Fires when precipitation is above 0 and below
0.1 in/hr, sustained wind is below 10 mph, and the game has at least one `TOTAL` `UNDER`. One warning per game, listing
every under.

> FYI: light rain without wind doesn't actually lower NFL scoring. Under 45.5 gets no extra help here.

## Rule Set C: Key numbers

| Rule                | Severity | Fires when                                                              |
| ------------------- | -------- | ----------------------------------------------------------------------- |
| `hook`              | `Warn`   | A `SPREAD` leg at exactly −3.5, −7.5, +2.5, or +6.5.                     |
| `total_key_number`  | `Info`   | A `TOTAL` leg whose line is within 0.5 of 37, 41, 43, 44, 47, or 51.     |

### `hook` — Warn

A spread sitting on the hook loses when the game lands on the key number. One warning per spread leg. At −3.5 the
message notes that about 15% of games land on exactly 3 and suggests buying to −3 or switching to the ML. At −7.5 it
suggests buying to −7 or switching to the ML. At +2.5 and +6.5 it says landing on 3 or 7 loses by the hook and suggests
buying to +3 or +7.

> BUF -3.5 is on the hook. About 15% of games land on exactly 3, and that's a loss here. Buying to -3 or switching to
> the ML is worth a look.

For a plus spread on the hook:

> MIA +2.5 is on the hook. Landing on exactly 3 loses by the hook. Buying to +3 is worth a look.

With fresh model data, `hook` appends one sentence. The cushion is the leg's line minus the model's spread for the same
team. At +2 or more the model sits farther from the key number and the sentence says the hook matters less. At −2 or
less the model is closer to the key number and the sentence says the leg asks for more than the model does. Between them
the sentence says the model sees it near the number too, so the hook is live. The trigger and severity do not change.

> BUF -7.5 is on the hook. Landing on exactly 7 loses by the hook. Buying to -7 or switching to the ML is worth a look.
> nfelo has it nearer -10, so the hook matters less.

### `total_key_number` — Info

A game total within half a point of a common NFL final total: 37, 41, 43, 44, 47, or 51. One warning per total leg. A
line exactly on a key number gets a different message than a line half a point away or between two keys.

> 43 is one of the most common NFL final totals. A half point either way decides it.

Between two key numbers:

> 43.5 sits right between 43 and 44, two of the most common NFL final totals. The half point matters here.

## Chalk

| Rule    | Severity | Fires when                             |
| ------- | -------- | -------------------------------------- |
| `chalk` | `Warn`   | Any leg with American odds of −230 or lower. |

### `chalk` — Warn

A heavy favorite adds little to the payout while still losing the slip often. One warning per qualifying leg. The payout
multiplier is `1 + 100/|odds|`, and the bust chance is `1 − |odds|/(|odds| + 100)`.

> This -300 leg only bumps the payout 1.33× but still busts the slip about 25% of the time.

On an `ML` leg with fresh model data, the message also gives nfelo's win probability for that team. Other markets keep
the plain sentence.

> This -300 leg only bumps the payout 1.33× but still busts the slip about 25% of the time. nfelo has BUF at about 76%
> to win.

## Model (nfelo)

| Rule      | Severity | Fires when                                                                                                                       |
| --------- | -------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `qb_drag` | `Info`   | nfelo marks a team's QB down by 30 Elo or more and the slip has that team's favorite spread or moneyline, or an over on its passing, receiving, or team total. |

### `qb_drag` — Info

nfelo's QB adjustment is in Elo points, and 25 Elo is about a point of spread, so −30 Elo is about 1.2 points. The rule
reads the level of the adjustment, not a change: nfelo has no game-day feed, so the message never says a QB is out. It
fires for that team's favorite `SPREAD` or `ML` and its `OVER` on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, `RECEPTIONS`, or
`TEAM_TOTAL`. An underdog spread, an underdog moneyline, a rush over, and an under stay quiet. One warning per game and
team, naming the QB and every matching leg; when two people own legs, it names both.

> nfelo marks down MIN's QB, Carson Wentz, by about 1.4 points. That drags on Adam's MIN -3 and Sam's MIN Team Total
> Over 20.5.

A game without fresh model data never triggers the rule.

## Props and player stats

| Rule          | Severity | Fires when                                                                                           |
| ------------- | -------- | ---------------------------------------------------------------------------------------------------- |
| `median_trap` | `Warn`   | A yardage over on a matched player with ≥ 3 games where the line is above the player's last-5 median. |

### `median_trap` — Warn

A yardage `OVER` (`PASS_YDS`, `RUSH_YDS`, or `REC_YDS`) on a matched player with at least 3 games in that market this season,
whose line sits higher than their median over the last 5 games (or all season games when under 5). One warning per
qualifying leg.

> Khalil Shakir's last-5 median is 41 yards, so 52.5 is a stretch.

When the line is also below the player's season average, the message adds that the average is propped up by one big
game:

> Khalil Shakir's last-5 median is 41 yards, so 52.5 is a stretch. His 58-yard average is propped up by one big game.

An unmatched player (`PlayerID` is empty), an `UNDER` side, non-yardage markets (`RECEPTIONS`, `PASS_TDS`, `ANYTIME_TD`),
and players with fewer than 3 games stay quiet.

## nflverse Efficiency, Usage, and Pace

| Rule             | Severity | Fires when |
| ---------------- | -------- | ---------- |
| `tough_matchup`  | `Warn`   | Yardage or TD `OVER` facing an opposing defense ranked `#1`–`#5` in matching `DefPassEPAAllowed` or `DefRushEPAAllowed`. |
| `shrinking_role` | `Warn`   | Player prop `OVER` where last-2-game `OffensePct` dropped `>= 15` percentage points or `TargetShare` dropped `>= 8` percentage points vs. season average. |
| `pace_clash`     | `Warn`   | `TOTAL` `OVER` when both teams rank `#25`–`#32` in `PlaysPerGame` and `#23`–`#32` in `PassRate`, or `TOTAL` `UNDER` when both rank `#1`–`#8` in `PlaysPerGame` and `OffPassEPA`. |
| `td_spike_risk`  | `Info`   | `ANYTIME_TD` `YES` with `>= 3` TDs in last 3 games on `< 15%` target share and `< 8` carries/game, or `PASS_TDS` `OVER` with `>= 6` TDs in last 3 games on `<= 0` avg `TotalEPA`. |

### `tough_matchup` — Warn

A yardage or TD `OVER` (`PASS_YDS`, `PASS_TDS`, `REC_YDS`, `RECEPTIONS` against `DefPassEPAAllowedRank` `#1`–`#5`, or `RUSH_YDS`, `RUSH_ATT` against `DefRushEPAAllowedRank` `#1`–`#5`). One warning per qualifying leg. Missing or zero rank fails soft.

> BAL ranks #2 against the rush by EPA allowed, so James Cook Over 68.5 Rush Yds is a tough matchup.

### `shrinking_role` — Warn

A player prop `OVER` (`PASS_YDS`, `PASS_TDS`, `RUSH_YDS`, `RUSH_ATT`, `REC_YDS`, `RECEPTIONS`) on a matched player with `>= 3` usage weeks where the average `OffensePct` over the last 2 games dropped by at least 15 percentage points below the season average, or the last-2-game `TargetShare` dropped by at least 8 percentage points below the season average.

> Dalton Kincaid's snap share dropped from 78% on the season to 55% over the last 2 games — recent volume is fading for Dalton Kincaid Over 45.5 Rec Yds.

### `pace_clash` — Warn

A game `TOTAL` leg that fights both teams' pace and efficiency profile. Fires on `TOTAL` `OVER` when both teams sit bottom-8 (`#25`–`#32`) in `PlaysPerGameRank` and bottom-10 (`#23`–`#32`) in `PassRateRank`, and on `TOTAL` `UNDER` when both teams sit top-8 (`#1`–`#8`) in `PlaysPerGameRank` and `OffPassEPARank`.

> Both PIT and CLE sit bottom-8 in pace and bottom-10 in pass rate, so Over 42.5 needs a lot of efficiency.

### `td_spike_risk` — Info

Touchdown scoring outpacing underlying opportunity over the last 3 games (requires `>= 3` game logs and `>= 3` usage weeks):

- `ANYTIME_TD` `YES`: `>= 3` TDs in the last 3 games with `< 15%` last-3-game `TargetShare` and `< 8` carries/game.
- `PASS_TDS` `OVER`: `>= 6` passing TDs in the last 3 games with `<= 0` average `TotalEPA` over the last 3 games.

> Dawson Knox has 3 TDs in the last 3 games on a 11% target share and 0 carries/game — TD luck can cool off.

## Sportsbook eligibility

| Rule | Severity | Fires when |
| ---- | -------- | ---------- |
| `book_combo_ineligible` | `Danger` | Same-game duplicate selection, opposite sides of the same market, same-team `ML` + `SPREAD`, `ML` + opponent favorite `SPREAD` (`< 0`), or mutually exclusive `TEAM_TOTAL`s vs. `TOTAL`. |
| `book_combo_restricted` | `Warn` | Same-player inverse prop pair (`PASS_YDS` vs. `PASS_TDS`, `REC_YDS` vs. `RECEPTIONS`, `RUSH_YDS` vs. `RUSH_ATT`), duplicate player-market milestones, or `KICKER_PTS` paired with `TOTAL` or `TEAM_TOTAL`. |

### `book_combo_ineligible` — Danger

Same-game combinations that DraftKings and FanDuel reject on a single parlay or SGP: duplicate selections, opposite sides of the same market (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`, or same-player prop `OVER`/`UNDER`), same-team `ML` + `SPREAD`, `ML` on one team paired with a favorite `SPREAD` (`< 0`) on the opponent, or mutually exclusive `TEAM_TOTAL`s against a game `TOTAL`. Also powers inline pick-board heads-up notes via `analyzer.CheckComboConflict`.

> Heads up: DraftKings and FanDuel usually won't combine BUF ML with BUF -3.5 on the same slip.

### `book_combo_restricted` — Warn

Same-game combinations that DraftKings or FanDuel frequently drop or block from SGP builders: `KICKER_PTS` paired with `TOTAL` or `TEAM_TOTAL`, same-player inverse prop pairs (`PASS_YDS` vs. `PASS_TDS`, `REC_YDS` vs. `RECEPTIONS`, `RUSH_YDS` vs. `RUSH_ATT` on opposite `OVER`/`UNDER` sides), and duplicate player-market milestones.

> Heads up: DraftKings and FanDuel frequently block combining Tyler Bass Over 7.5 Kicking Pts with Over 47.5 on an SGP.

## Score Bands and Pillar Tones

Both `EngineV2` and `EngineV1` map clamped `0..100` integer scores to `A`–`F` letter grades:

| Letter | Range    | Description |
| ------ | -------- | ----------- |
| `A`    | `85–100` | Strong form, matchup, and price with clean script alignment. |
| `B`    | `73–84`  | Solid leg or slip with minor or no red flags. |
| `C`    | `60–72`  | Mixed signals: playable, but fighting a hook, chalk, or tough spot. |
| `D`    | `48–59`  | Multiple headwinds or script friction dragging down the ticket. |
| `F`    | `0–47`   | Severe script conflict, heavy chalk, or stacked environmental headwinds. |

In v2, each individual `PillarScore` (`0..100`) maps to one tonal UI badge: `good` (`>= 80`), `neutral` (`68–79`),
`warn` (`56–67`), or `bad` (`< 56`).

## v2 Four-Pillar Grading Model (`ActiveEngine = EngineV2`)

`EngineV2` (`DefaultV2Config`) grades matched legs and slips across four independent `0..100` pillars—`Price & Vig`
(`price_vig`), `Script Fit` (`script_fit`), `Stability & Tail Risk` (`stability_risk`), and `Context & Traps`
(`context_traps`)—while populating `CalibratedProb`, `MarketProb`, `CorrMultiplier`, `Uncertain`, and backwards-compatible
`Factors`.

### Pillar Rollup Weights and Score Caps

| Context | `Price & Vig` | `Script Fit` | `Stability & Tail Risk` | `Context & Traps` | Score caps |
| ------- | ------------- | ------------ | ----------------------- | ----------------- | ---------- |
| Single leg (no script signal) | `40%` | `0%` | `25%` | `35%` | `<= 72` (`C`) if any `Danger` overlay fires or `Price & Vig < 45` |
| Single leg (with script signal) | `30%` | `15%` | `25%` | `30%` | `<= 72` (`C`) if any `Danger` overlay fires or `Price & Vig < 45` |
| Multi-leg slip | `35%` | `25%` | `20%` | `20%` | `<= 72` (`C`) on `Danger` conflict, `CorrMultiplier < 0.85`, or `Price & Vig < 45`; `<= 84` (`B`) on `Warn` conflict or `Uncertain` |

A single-leg slip mirrors that leg's score, letter, pillars, and `CalibratedProb` with `CorrMultiplier = 1.0`.

### Prop Distributions, Shrinkage, and Calibration (`internal/modelv2`)

The embedded `v2.json.gz` artifact (`FeatureSchemaVersion = 1`) serves seven player prop markets with flat GBDT ensembles
predicting distribution parameters rather than point totals:

| Market | Distribution family | Parameters | Survival & tail risk |
| ------ | ------------------- | ---------- | -------------------- |
| `PASS_YDS`, `RUSH_YDS`, `REC_YDS` | `zero_adj_gamma` (also supports `zero_adj_lognormal`) | Zero-mass `pi0`, mean `mu`, shape `k` | `(1 - pi0) * Q(k, line * k / mu)`; exact mixture `CV` and `Skewness` |
| `RECEPTIONS`, `RUSH_ATT` | `negbin` | Mean `mu`, dispersion `r` | Log-space PMF sum (`<= 256`) or regularized incomplete beta `I` tail |
| `PASS_TDS` | `poisson` (optional `negbin` when `r` present) | Rate `lambda` | Exact Poisson–Gamma upper tail `P(floor(line)+1, lambda)` |
| `ANYTIME_TD` | `bernoulli` | Event probability `p` | Direct `p` for `YES` (or `1 - p` for `NO`), high-swing binary variance |

Inference pipeline per prop leg:

1. **Weeks 1–5 Empirical Bayes shrinkage (`ShrinkFeatureSlice`)**: When `games_played < 5` (`ShrinkMaxGames = 5`),
   observed EWMA usage and production features shrink toward the matching `RolePrior` (`MARKET:ROLE`, falling back to
   position default then `MARKET:DEFAULT`) using `weight = n / (n + tau)` (`DefaultPriorStrength = 3.0`).
2. **Market anchor blend**: Survival probability blends with the exact-line Power-devigged `SharpQuote.PowerFairProb`
   (`w * p_market + (1 - w) * p_model`, artifact `market_blend_weight = 0.70`). If any active `ContextOverlay` sets
   `HeightenedUncertainty = true`, `w` increases by `+0.15` (`HeightenedUncertaintyBlendBoost`, capped at `0.95`).
3. **Post-hoc calibration**: The blended (or raw) probability passes through the market's learned `platt` (logistic in
   logit space) or `isotonic` (monotone piecewise-linear) calibrator and clamps to `[0.01, 0.99]`.

For non-prop game lines (`SPREAD`, `ML`, `TOTAL`, `TEAM_TOTAL`), `EngineV2` blends Power-devigged sharp/consensus anchors
(`GameLineMarketBlend = 0.70`, adjusting different lines via normal CDF with `SpreadMarginStdDev = 13.45`,
`GameTotalStdDev = 13.80`, `TeamTotalStdDev = 9.80`) with nfelo or `MarketScript` normal-model baselines, falling back to
retail implied probability stripped of `4.5%` assumed two-way vig when no sharp benchmark is available.

### Four-Pillar Scoring Mechanics

- **`Price & Vig` (`price_vig`)**: Evaluates `parlay.FairEV(calibratedProb, leg.Odds)` (or per-leg equivalent joint EV
  `pow(1 + jointEV, 1/n) - 1` on multi-leg slips) along a piecewise-linear monotonic curve (`+8% -> 100`, `+5% -> 98`,
  `0% -> 85`, `-4.5% -> 74`, `-10% -> 48`, `-15% -> 33`, `-25% -> 10`). On multi-leg slips, `ParlayFreeLegs = 3` deducts
  `ParlayVigDragStep = 2` points per extra leg beyond 3, and **SGP correlation tax** deducts `10` points
  (`SGPCorrTaxPenalty`) when `CorrMultiplier > 1.05` (`SGPCorrTaxMinMult`) yet `1 / combinedDecimal > p_joint`.
  Copy always cites American odds and fair prices, never `"+EV %"`.
- **`Script Fit` (`script_fit`)**:
  - *Single leg*: starts at `75` and adjusts for spread/total alignment on rushing Overs/Unders (`+12` when favored
    `<= -7.0`, `+6` at `<= -3.5`, `-12` at `>= +4.5`, `-18` at `>= +7.0` for rush Overs) and passing/receiving
    Overs/Unders (`-8` on double-digit favorites `<= -10.0` or low totals `<= 39.5`; `+8` on shootouts `>= 48.0`).
  - *Multi-leg slip*: maps copula `CorrMultiplier` (`>= 1.10 -> 90..98`, `1.05..1.10 -> 82..89`, `0.95..1.05 -> 75`,
    `0.85..0.95 -> 45..62`, `< 0.85 -> 15..44`), capped at `<= 44` for `Danger` script conflicts (`trailing_rb`,
    `total_contradiction`), `<= 62` for `Warn` conflicts (`blowout_passing`), and floored at `>= 86` for clean positive
    stacks with `CorrMultiplier >= 1.0`.
- **`Stability & Tail Risk` (`stability_risk`)**: Starts at `82` (`v2StabilityBase`), penalizes heavy chalk (`<= -230`),
  low calibrated probability (`< 0.25 -> -18..-28`, `< 0.38 -> -8`, `>= 0.56` non-chalk `-> +6`), touchdown markets
  (`ANYTIME_TD -10`, `PASS_TDS -5`), wide distribution tails (`CV >= 0.85` or `Skewness >= 1.5 -> -8`; `CV >= 0.55` or
  `Skewness >= 1.0 -> -4`; `CV <= 0.32` and `Skewness < 0.7 -> +5`), and early-season `<= 2` game samples (`-4`).
  Multi-leg slips blend `0.6 * avgLegStability + 0.4 * minLegStability`, minus `3` per leg beyond 2, `-10` when the
  weakest leg is `< 0.25` (or `-5` when `< 0.35`), and `-5` per extra chalk leg beyond the first.
- **`Context & Traps` (`context_traps`)**: Starts at `75`, sums `ScoreDelta` across active `ContextOverlay` findings
  (clamped to `[-55, +20]`), and surfaces the top 3 findings by severity then magnitude. Multi-leg slips average leg
  scores or weight `0.4 * avg + 0.6 * worst` whenever any leg sits below `75`.

### Gaussian Copula Script Scoring (`CopulaArtifact.EstimateJoint`)

Multi-leg slips compute joint hit probability deterministically without random jitter:

1. Legs are grouped by `GameID`; independent games multiply (`CorrMultiplier = 1.0` across games).
2. Each same-game leg resolves to a canonical node (`TEAM_MARGIN`, `GAME_TOTAL`, `<ROLE>_<MARKET>`, or
   `OPP_<ROLE>_<MARKET>`) with direction sign `+1` (`OVER`/`YES`/team line) or `-1` (`UNDER`/`NO`).
3. Each game selects its correlation matrix from `CopulaArtifact`: `heavy_favorite` when `|HomeSpread| >= 7.0`,
   `shootout` when `GameTotal >= 48.0`, otherwise `neutral`. Same-player pairs apply `WithinPlayer` correlation
   overrides, same-node distinct players default to `0.20`, and opposite-sign legs negate pairwise correlation.
4. Cholesky factorization (`L * L^T`) with a unit-diagonal-preserving jitter fallback decomposes the submatrix, and
   `8,192` precomputed deterministic QMC (Sobol dims 1–32 + Halton primes) normal draws estimate
   `p_joint = P(forall i: (L*u)_i <= Phi^-1(p_i))` in sub-millisecond time.

### Extensible Context & Injury Overlays (`DefaultContextOverlays`)

`EvaluateOverlays` runs 12 ordered `ContextOverlay` checks on every leg without modifying or retraining `v2.json.gz`:

| Overlay rule | Severity | Pillar delta | Heightened uncertainty | Trigger |
| ------------ | -------- | ------------ | ---------------------- | ------- |
| `player_injury_status` | `Danger`/`Warn`/`Info` | `-20` to `0` | Yes | Player listed `Out`/`IR` (`-20` Over, `-10` Under), `Doubtful` (`-15`/`-5`), or `Questionable` (`-6` Over, `0` Under) in `Game.Injuries`. |
| `teammate_out_bump` | `Info` | `+5` | Yes | Same-team, same-position (`WR`/`TE`/`RB`) teammate listed `Out` on a receiving/rushing Over or `ANYTIME_TD` `YES`. |
| `qb_drag` | `Info` | `-8` | No | Fresh nfelo QB adjustment `<= -30` Elo on favorite spread/ML or passing/receiving/team-total Over. |
| `wind` | `Warn`/`Danger` | `-8` / `-18` | No | Outdoor wind `>= 15` mph / gusts `>= 25` mph (`-8`) or wind `>= 20` mph (`-18`) on passing/receiving/kicking/total Overs. |
| `fake_weather` | `Info` | `-3` | No | Outdoor light rain (`0 < precip < 0.1` in/hr) and wind `< 10` mph on `TOTAL` `UNDER`. |
| `hook` | `Warn` | `-10` (`-4` w/ cushion) | No | Spread at `-3.5`, `+2.5`, `-7.5`, or `+6.5`; softened to `-4` with `>= +2` nfelo spread cushion. |
| `total_key_number` | `Info` | `-5` | No | Game `TOTAL` within `0.5` of `37`, `41`, `43`, `44`, `47`, or `51`. |
| `shrinking_role` | `Warn` | `-15` | Yes | Last-2-game `OffensePct` dropped `>= 15` pp or `TargetShare` dropped `>= 8` pp vs. season average. |
| `td_spike_risk` | `Info` | `-6` | No | `ANYTIME_TD` `YES` with `>= 3` L3 TDs on `< 15%` target share and `< 8` carries/g, or `PASS_TDS` Over with `>= 6` L3 TDs on `<= 0` avg `TotalEPA`. |
| `narrative_streak_trap` | `Warn` | `-8` | No | `OVER` cleared 5 straight games, yet fair probability `< 0.50` or line jumped materially (`>= max(3, 5%*L5Mean)` yds / `>= max(0.5, 8%*L5Mean)` count) above recent average/median with fair prob `< 0.55`. |
| `model_market_divergence` | `Warn` | `-4` | Yes | Raw model probability and exact-line sharp market anchor differ by `>= 0.08`. |
| `external_context_signals` | Caller-defined | Caller-defined | Caller-defined | Structured `ContextSignal` items attached to `Game.ContextSignals` or `PlayerStats.ContextSignals`. |

## v1 Additive Rubric Reference (`EngineV1` / `RubricV1`)

`EngineV1` (`ScoreLegWithRubric` and `ScoreSlipWithRubric` using `RubricV1`) remains registered for side-by-side offline
and `/analyzer/results` comparison, and `GradeRubric()` continues to expose its baseline factor list on
`/analyzer/guide`.

### v1 Leg Scoring Factors

Every matched leg starts at a baseline of `75` (`B`). Adjustments are summed within three capped categories:

- **Form & Usage** (clamped to `-20` to `+15`)
- **Matchup & Environment** (clamped to `-20` to `+15`)
- **Line & Price** (clamped to `-20` to `+15`)

| Factor                 | Category              | Delta              | Trigger |
| ---------------------- | --------------------- | ------------------ | ------- |
| `hit_rate`             | Form & Usage          | `+10`              | Cleared the line in at least 4 of the last 5 games. |
| `rising_usage`         | Form & Usage          | `+5`               | Last-3-game snap share (`+8` pp) or target share (`+4` pp) trending above season average. |
| `median_trap`          | Form & Usage          | `-15` (`-5` w/ teammate Out) | Yardage `OVER` line sits above the player's last-5 game median; softened to `-5` when a same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out`. |
| `teammate_out_bump`    | Form & Usage          | `+5`               | Same-team, same-position (`WR`/`TE`/`RB`) teammate is `Out` on a `REC_YDS`, `RECEPTIONS`, `RUSH_YDS`, or `RUSH_ATT` `OVER`, or `ANYTIME_TD` `YES` prop (ignores `Questionable`, `Doubtful`, `Injured Reserve`, and self-injuries). |
| `shrinking_role`       | Form & Usage          | `-15`              | Last-2-game snap share (`-15` pp) or target share (`-8` pp) dropped below season average. |
| `favorable_matchup`    | Matchup & Environment | `+10`              | Opposing defense ranks `#25`–`#32` in matching pass or rush EPA allowed. |
| `tough_matchup`        | Matchup & Environment | `-12`              | Opposing defense ranks `#1`–`#5` in matching pass or rush EPA allowed. |
| `wind`                 | Matchup & Environment | `-8` / `-18`       | `-8` for `Warn` wind/gusts; `-18` for `Danger` wind (`>= 20` mph) on passing/receiving/kicking/total `OVER`s. |
| `qb_drag`              | Matchup & Environment | `-8`               | nfelo marks down the team's QB by `<= -30` Elo on a favorite or passing/receiving/team-total `OVER`. |
| `fake_weather`         | Matchup & Environment | `-3`               | Light rain without wind gives a game total `UNDER` no real scoring brake. |
| `nfelo_cushion`        | Line & Price          | `+8`               | nfelo projects at least 2 points of cushion on a spread leg. |
| `hook`                 | Line & Price          | `-10` (`-4` w/ cushion) | Spread sits on `-3.5`, `+2.5`, `-7.5`, or `+6.5`; softened to `-4` with `>= +2` nfelo cushion. |
| `total_key_number`     | Line & Price          | `-5`               | Game total sits within `0.5` points of `37`, `41`, `43`, `44`, `47`, or `51`. |
| `chalk`                | Line & Price          | `-10` to `-18`     | Odds `<= -230` scale linearly from `-10` at `-230` down to `-18` at `-500` or worse. |
| `sharp_line_edge`      | Line & Price          | `+8` (`+5` consensus) | Line beats sharp bookmakers by at least `0.5` points (or `2.0` yards/attempts on `PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`). |
| `sharp_line_fade`      | Line & Price          | `-8` (`-5` consensus) | Line trails sharp bookmakers by at least `0.5` points (or `2.0` yards/attempts on `PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RUSH_ATT`). |
| `sharp_price_edge`     | Line & Price          | `+6` (`+4` consensus) | Retail odds pay at least `2.5` percentage points better than sharp de-vigged fair probability on the same line (or `ML`). |
| `sharp_price_fade`     | Line & Price          | `-6` (`-4` consensus) | Retail odds carry at least `4.5` percentage points more juice than sharp de-vigged fair probability on the same line (or `ML`). |

#### Sharp-book comparison details and copy

When `Game.Sharp` has a benchmark for a leg, `scoreLineAndPrice` checks line differences first (when both the leg and benchmark have non-nil lines that differ) and falls back to price comparison when lines match within `1e-6` (or `ML` with a nil line):

- **Line edge (`sharp_line_edge`)**: `"Getting BUF -2.5 while LowVig sits at -3.5."`
- **Line fade (`sharp_line_fade`)**: `"LowVig sits at -2.5, so -3.5 gives up a full point."`
- **Price edge (`sharp_price_edge`)**: `"Strong price at +105 vs. LowVig fair price near -108."`
- **Price fade (`sharp_price_fade`)**: `"Steep price at -135 vs. LowVig fair price near -112."`

### v1 Slip Scoring Factors

`ScoreSlipWithRubric` starts from the rounded average of its graded legs' numeric scores (returning an empty grade if no legs are gradable), then applies slip-level correlation and drag factors:

| Factor                | Delta         | Trigger |
| --------------------- | ------------- | ------- |
| `stack_shootout`      | `+8`          | Pass `OVER` + same-team receiver `OVER` + game `TOTAL` `OVER` in the same game. |
| `stack_clock_drain`   | `+8`          | Favorite spread/ML + same-team rush `OVER` + opponent `TEAM_TOTAL` `UNDER`. |
| `blowout_passing`     | `-12`         | Big favorite spread (`<= -7.0`) paired with same-team passing/receiving `OVER`. |
| `trailing_rb`         | `-20`         | Underdog spread (`>= +4.5`) or ML paired with same-team rushing `OVER`. |
| `total_contradiction` | `-20`         | Game `TOTAL` `UNDER` paired with 2+ point-scoring `OVER`s or `ANYTIME_TD`s. |
| `parlay_drag`         | `-3` / leg    | Applied for every leg beyond 3 legs. |
| `chalk_drag`          | `-5` / leg    | Applied for every heavy favorite leg (`<= -230`) beyond the first. |
