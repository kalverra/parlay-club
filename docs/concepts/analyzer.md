---
id: analyzer
title: Analyzer
type: concept
summary: How the sharp-friend banner under a slip turns legs, games, weather, player stats, and nfelo's model into plain-Go warnings, and the tone those warnings keep.
tags: [analyzer, warnings, banner, severity, tone, slips, weather, rules, nfelo, model, props, player-stats]
sources: [internal/analyzer/analyzer.go, internal/analyzer/rules.go, internal/web/analyzer.go, internal/web/pick.go, internal/db/nfelo.go, internal/db/players.go, web/templates/partials.html]
last_verified: 2026-11-11
---

# Analyzer

## Summary

The analyzer is the sharp-friend banner under a slip's legs. It reads the picks people already made and points out where
they fight each other, the weather, the key numbers, the price, player history, or nfelo's model. It never blocks a pick.
Every leg stays allowed, and the banner only adds a heads-up.

`internal/analyzer` is pure Go: no I/O, no clock, no network calls, and no LLM. It runs on every slip render, and its
output is not stored. The rule list and example messages live in [Analyzer rules](../reference/analyzer-rules.md).

## What the banner shows

The slip panel renders one banner under the legs. With no warnings it shows a green "Looks clean". Otherwise it shows
one card per warning, sorted `Danger`, `Warn`, then `Info` and colored red, amber, and blue. Each card carries the
message, the severity, and a chip for every leg involved with its owner's name.

The banner is part of the slip panel, so it refreshes with the panel on `slip-updated` and `board-updated`. A weather
change needs no separate wiring: the next board refresh re-renders the slip, and the rules see the new forecast. See
[Live updates](live-updates.md).

## The pick dock's tray

The pick board's fixed dock shows your count and the combined price of your legs. Its "Slip" button opens a sheet with
the tray, and that tray repeats only the warnings that name one of your own legs. `internal/web/pick.go` runs the same
analyzer over the whole slip, then keeps a card when any leg it names belongs to the viewer. A heads-up about a
friend's pick stays on the hub and out of your tray, where it would only be noise while you pick. A warning that names
both your leg and theirs still appears, because it bears on the pick you are making.

The dock and the tray inside its sheet refresh with the pick board on `tray-updated` and `slip-updated`, so a new leg or
a fresh forecast brings its warning without a reload. See [Slips](slips.md#the-pick-board).

## Tone

Warnings read like a sharp friend's heads-up: short, specific, and free of lectures. They name the people, the teams,
and the numbers, and they suggest a better pick or a safer number. They never say "don't," and they never explain
expected value. A warning on two people's legs names both owners, such as "Adam's BUF -7.5 + Sam's Josh Allen Over
245.5."

The tone rules live in `plan/analyzer.md`. Every message is checked against them before an analyzer change ships.

## Inputs

`AnalyzeSlip(legs, games, stats)` takes three plain values:

| Input   | Fields the rules read                                                                    |
| ------- | ---------------------------------------------------------------------------------------- |
| `legs`  | owner name, game, market, team, player, side, line, American odds                        |
| `games` | home and away teams, indoor flag, weather (wind, gusts, precipitation, temperature), nfelo model (spread, win probability, QB adjustments) |
| `stats` | per-player game logs per market, most recent game first, this season only                |

`internal/web` builds those values from the stored slip, the week's games, and stored player game logs when it renders
the panel. It keys the games map by the same `GameID` each leg carries, so no leg loses its game.

`stats` maps ESPN athlete IDs to season game logs across markets (`PASS_YDS`, `RUSH_YDS`, `REC_YDS`, `RECEPTIONS`,
`PASS_TDS`, `RUSH_ATT`, `ANYTIME_TD`, `KICKER_PTS`). `internal/web` loads `player_gamelogs` for every distinct matched
`PlayerID` on the slip. An unmatched player, a missing game log, or a market with fewer than 3 games leaves player-stat
rules quiet.

nfelo's model is an input, not a requirement. `internal/web` attaches it only when the game is `SCHEDULED` and a
successful `nfelo:projections` fetch is under 72 hours old. A live or final game, a missing model row, and a stale fetch
all leave the model nil, so the model sentences and `qb_drag` stay quiet while every other rule runs as before. The model
rows come from [Data sources](../reference/data-sources.md) through [Data refresh](data-refresh.md).

Legs the analyzer skips:

- market `OTHER`;
- an empty `GameID`;
- a `GameID` missing from the games map.

Output order is stable: severity first (`Danger` before `Warn` before `Info`), then the slip position of the first leg
named. The same slip and data always produce the same warnings.

## Related

- [Analyzer rules](../reference/analyzer-rules.md): every rule, its severity, trigger, and example message.
- [Slips](slips.md): the legs and games the banner reads.
- [Data refresh](data-refresh.md): where the weather behind a warning comes from.
