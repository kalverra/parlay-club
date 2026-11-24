---
id: analyzer-rules
title: Analyzer rules
type: reference
summary: The stable IDs, severities, triggers, and example messages for the correlation, weather, key-number, chalk, player stats, nfelo model, and nflverse rules, plus the A–F leg and slip grading rubric in internal/analyzer.
tags: [analyzer, rules, grades, rubric, warnings, correlation, severity, slips, nfelo, nflverse, model, quarterback, props, player-stats]
---

# Analyzer rules

## Summary

`internal/analyzer` holds the sharp-friend checks and grading functions that run under a slip. `AnalyzeSlip(legs, games, stats)`, `ScoreLeg(leg, games, stats)`, and `ScoreSlip(legs, games, stats)` are pure and deterministic: no I/O, no clock, and no LLM calls. `AnalyzeSlip` returns warnings with a stable rule ID, a severity, a message, and the legs involved; `ScoreLeg` and `ScoreSlip` return `A`–`F` grades with `GradeFactor` breakdowns. `RuleDocs()` and `GradeRubric()` expose the live rules and scoring weights for `/analyzer/guide`.

When nfelo's model is fresh, `hook` and `chalk` add a sentence about it, and `qb_drag` reads it for a downgraded quarterback. When player game logs exist, `median_trap` warns when a yardage over sits above recent game medians. When `nflverse` team EPA and player usage are fresh, `tough_matchup`, `shrinking_role`, `pace_clash`, and `td_spike_risk` fire alongside EPA and usage grading factors. A game or leg without those optional inputs behaves cleanly and fails soft.

Rule IDs are stable across releases. A warning's severity is `Info`, `Warn`, or `Danger`.

## How rules see a slip

- Legs with market `OTHER`, an empty `GameID`, or a `GameID` missing from the game map are skipped by every rule and receive no grade (`Letter == ""`).
- Output is sorted by severity, `Danger` first, then by the first involved leg's position in the slip. Stable sorting keeps the same input in the same order.
- **Same team** means an equal `Team`. **Opponent** is the other team in the leg's game.
- **Favorite**: a `SPREAD` with a line below 0, or an `ML` with negative odds. **Underdog**: a `SPREAD` with a line above 0, or an `ML` with positive odds.
- **Passing/receiving over**: an `OVER` on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, or `RECEPTIONS`.
- **Model data**: nfelo's pre-game spread, win probability, and QB adjustments for the game. `internal/analyzer` (`LoadSlipSignals`) attaches it only when the game is `SCHEDULED` and the last successful `nfelo:projections` fetch is under 72 hours old. Otherwise the game has no model, and the model sentences and `qb_drag` stay quiet.
- **Player stats**: `stats` maps ESPN athlete IDs to recent game logs per market, most recent first, and includes `Position` from `roster_players`. `internal/analyzer` (`LoadSlipSignals`) populates it from `player_gamelogs` for every matched `PlayerID` on the slip. An unmatched player, a missing game log, or a market with fewer than 3 games leaves player-stat rules quiet.
- **Team injuries (roster + summary merge)**: `Game.Injuries` maps each team abbreviation to its merged injury list. `internal/analyzer` (`LoadSlipSignals` via `MergeGameInjuries` and `espn.MergeTeamInjuries`) combines `games.injuries_json` (ESPN summary, kept primary) with key-position injuries (`QB`, `RB`, `WR`, `TE`, `LT`, `C`, `CB`, `S`) from `roster_players.injury_status`, backfilling any player truncated past ESPN's 5-per-team summary cap.
- **nflverse team EPA & player usage**: `Game.HomeEPA` and `Game.AwayEPA` hold league ranks (`1..32`, where `1` is the toughest defense, highest offense, fastest pace, or highest pass rate) for `DefPassEPAAllowedRank`, `DefRushEPAAllowedRank`, `OffPassEPARank`, `PlaysPerGameRank`, and `PassRateRank`. `PlayerStats.Usage` holds weekly `OffensePct`, `TargetShare`, `AirYardsShare`, `WOPR`, `Carries`, `Targets`, and `TotalEPA` (most recent first, requiring ≥ 3 weeks for usage rules). `internal/analyzer` attaches both only when `nflverse:season:{year}` is under 7 days old.
- **Sharp-book benchmarks**: `Game.Sharp` maps `SharpKey` (`MarketType`, `Team`, normalized `PlayerName`, `Side`) to a de-vigged `SharpQuote` built by `BuildSharpBenchmarks` over `db.CurrentOdds`. Books are tiered: **Tier 1** (`Pinnacle`, `Circa`, `Bookmaker`), **Tier 2** (`LowVig`, `BetOnline`), and **Tier 3** (`market consensus` across `>= 3` traditional sportsbooks quoting the exact same line, excluding DFS pick'em apps and peer-to-peer exchanges). Two-way quotes are paired and de-vigged when overround falls in `[0.95, 1.25]`; one-sided markets (`ANYTIME_TD`) and unpaired quotes are skipped cleanly.
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

## Leg and Slip Grading Rubric

`ScoreLeg` and `ScoreSlip` compute deterministic `0..100` scores (clamped) and map them to `A`–`F` letter grades with `GradeFactor` breakdowns. `GradeRubric()` exposes this exact rubric for `/analyzer/guide`.

### Score Bands

| Letter | Range    | Description |
| ------ | -------- | ----------- |
| `A`    | `85–100` | Strong form, matchup, and price with clean script alignment. |
| `B`    | `73–84`  | Solid leg or slip with minor or no red flags. |
| `C`    | `60–72`  | Mixed signals: playable, but fighting a hook, chalk, or tough spot. |
| `D`    | `48–59`  | Multiple headwinds or script friction dragging down the ticket. |
| `F`    | `0–47`   | Severe script conflict, heavy chalk, or stacked environmental headwinds. |

### Leg Scoring Factors (`ScoreLeg`)

Every matched leg (not `OTHER`, with a valid `GameID` in the games map) starts at a baseline of `75` (`B`). Adjustments are summed within three capped categories:

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

### Slip Scoring Factors (`ScoreSlip`)

`ScoreSlip` starts from the rounded average of its graded legs' numeric scores (returning an empty grade if no legs are gradable), then applies slip-level correlation and drag factors:

| Factor                | Delta         | Trigger |
| --------------------- | ------------- | ------- |
| `stack_shootout`      | `+8`          | Pass `OVER` + same-team receiver `OVER` + game `TOTAL` `OVER` in the same game. |
| `stack_clock_drain`   | `+8`          | Favorite spread/ML + same-team rush `OVER` + opponent `TEAM_TOTAL` `UNDER`. |
| `blowout_passing`     | `-12`         | Big favorite spread (`<= -7.0`) paired with same-team passing/receiving `OVER`. |
| `trailing_rb`         | `-20`         | Underdog spread (`>= +4.5`) or ML paired with same-team rushing `OVER`. |
| `total_contradiction` | `-20`         | Game `TOTAL` `UNDER` paired with 2+ point-scoring `OVER`s or `ANYTIME_TD`s. |
| `parlay_drag`         | `-3` / leg    | Applied for every leg beyond 3 legs. |
| `chalk_drag`          | `-5` / leg    | Applied for every heavy favorite leg (`<= -230`) beyond the first. |
