---
id: analyzer-rules
title: Analyzer rules
type: reference
summary: The stable IDs, severities, triggers, and example messages for the correlation rules in internal/analyzer's Rule Set A.
tags: [analyzer, rules, warnings, correlation, severity, slips]
sources: [internal/analyzer/analyzer.go, internal/analyzer/rules.go, internal/analyzer/analyzer_test.go]
last_verified: 2026-11-07
---

# Analyzer rules

## Summary

`internal/analyzer` holds the sharp-friend checks that run under a slip. `AnalyzeSlip(legs, games, stats)` is pure and
deterministic: no I/O, no clock, and no LLM calls. It returns warnings with a stable rule ID, a severity, a message, and
the legs involved.

Rule IDs are stable across releases. A warning's severity is `Info`, `Warn`, or `Danger`.

## How rules see a slip

- Legs with market `OTHER`, an empty `GameID`, or a `GameID` missing from the game map are skipped by every rule.
- Output is sorted by severity, `Danger` first, then by the first involved leg's position in the slip. Stable sorting
  keeps the same input in the same order.
- **Same team** means an equal `Team`. **Opponent** is the other team in the leg's game.
- **Favorite**: a `SPREAD` with a line below 0, or an `ML` with negative odds. **Underdog**: a `SPREAD` with a line above
  0, or an `ML` with positive odds.
- **Passing/receiving over**: an `OVER` on `PASS_YDS`, `PASS_TDS`, `REC_YDS`, or `RECEPTIONS`.
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
