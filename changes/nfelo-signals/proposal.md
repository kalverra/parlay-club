---
slug: nfelo-signals
status: implementing
created: "2026-11-05"
depends_on:
    - analyzer
effort: medium
---

# nfelo signals in the analyzer

## Context

The analyzer judges a slip from three inputs: the legs, the weather, and the market line. Every rule compares a leg to
the sportsbook number or to the other legs. None can say whether a line is sane, because the app has no opinion of its
own about how good a team is.

That blind spot shows up in the picks friends make most:

- A -7.5 spread looks the same whether the best public model thinks the game is a 4-point or a 10-point game.
- Heavy chalk gets a warning for price alone, with no sense of how likely the favorite is to win.
- A backup quarterback is a big swing, and the app knows nothing about it.

[nfelo](https://www.nfeloapp.com/) is a free, well-regarded NFL Elo model. It publishes projected spreads, win
probabilities, team ratings, and quarterback adjustments as plain CSV files in a public GitHub repo, refreshed about
every two days. No Odds API credits are involved, so the signal costs nothing against the 500-credit budget.

## Goals and non-goals

**Goals:**

- Add nfelo's projected spread, win probability, and QB adjustment to what the analyzer knows about each game.
- Add a few friendly heads-up rules that use them: key-number hooks that read differently against the model's number, a
  win-probability check on heavy chalk, and a note when a quarterback change moves the rating.
- Show nfelo's number next to the market line where a leg appears, so everyone sees the same context.
- Fail soft. When nfelo data is missing or stale, the app works as it does today and the nfelo rules stay quiet.
- Keep the analyzer pure and deterministic. nfelo data is an input, not a dependency of the rules.

**Non-goals:**

- Telling anyone to fade the market or bet a side. A heads-up says "nfelo has this nearer a 5-point game"; it never says
  "this bet is -EV."
- Using nfelo to replace the Odds API or to supply sportsbook lines. Its market columns are for model research, not live
  prices.
- Scraping nfeloapp.com pages. The repo's CSV files are the only source.
- Team-stat sources such as nflverse EPA. That is a separate change if we want opponent-strength rules for props.
- LLM calls or model-generated advice.

## Decision

Treat nfelo as one more free, lazily synced source, beside ESPN and Open-Meteo. A small client reads nfelo's published
CSV files by column name and stores the latest projections in SQLite behind the same `sync_state` staleness cache the
other sources use. The analyzer receives the per-game numbers as plain inputs and gains a handful of rules, each in the
sharp-friend tone, each Info or Warn and never blocking. The slip view shows the model number next to the line.

We chose nfelo over the alternatives for three reasons. It costs no credits. It is public and machine-readable, unlike
RBSDM, which exposed no data we could read, or CrazyNinjaOdds, which has no NFL data export. And it gives a model number
the analyzer lacks. Building our own ratings would take far more work for a worse signal.

Two risks need an answer before we ship. nfelo's repo has no license and its site says "All rights reserved", so the
author should be asked for permission; for a small private app the risk is low, but we should ask. And the files can
change shape or go stale, so the client parses by header name and the rules go quiet when the data is old. The effort is
medium: the client and the rules are small, but the work spans ingest, storage, analyzer, and slip view, and needs a
team-code mapping between nfelo and ESPN.
