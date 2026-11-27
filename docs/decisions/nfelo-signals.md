---
id: adr-nfelo-signals
title: nfelo signals in the analyzer
type: decision
summary: Treat nfelo as one more free, lazily synced source, beside ESPN and Open-Meteo.
area: analyzer
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

## Consequences

The analyzer now has independent model context—projected spreads, win probabilities, and quarterback ratings—without
spending Odds API credits. Friends see friendlier, sharper advice on slips: `hook` explains whether the model agrees
with key numbers, `chalk` gives win probability on heavy moneyline favorites, and `qb_drag` highlights when a backup
quarterback downgrades an offense. Slip views show model projections on spread and moneyline legs.

When nfelo sync fails, is missing, or is older than 72 hours, all model signals silently deactivate without erroring or
blocking.

Maintenance requires watching team code mappings between nfelo and ESPN (`OAK`→`LV`, `WAS`→`WSH`). Because stale or
unmatched data turns signals off silently, maintainers must rely on sync logs and archive history rather than UI
warnings to detect upstream feed issues.

We would revisit batching `NfeloForGame` lookups if per-game queries slow slip rendering.

## Alternatives considered

- **Team-stat sources (e.g. nflverse EPA):** Rejected for now. EPA measures opponent strength for props rather than
  spread and win-probability sanity checks, and requires heavier ingest pipelines.
- **RBSDM / CrazyNinjaOdds:** Rejected because RBSDM provides no machine-readable export, and CrazyNinjaOdds lacks an
  NFL data export. nfelo offers clean, public CSV files.
- **Custom ratings and Elo modeling:** Rejected because building and calibrating custom models takes substantial effort
  for weaker signals than established models provide.
- **Scraping nfeloapp.com:** Rejected in favor of the public GitHub CSV repository to avoid brittle HTML scraping.
- **Reading `output_data/projected_spreads.csv` or `nfelo_games.csv`:** Rejected because `projected_spreads.csv` is
  absent on `main`, and `nfelo_games.csv` omits team abbreviations and quarterback names.
- **Passing nfelo via the `stats` map:** Rejected because `stats` is keyed per-player while nfelo projections apply
  per-game.
- **Dynamic CSV fetch on render:** Rejected to keep slip rendering pure, offline-capable, and fast without external
  network dependencies.
- **New analyzer rules for every signal:** Rejected in favor of augmenting `hook` and `chalk` to keep the warning banner
  concise.

## Implementation notes

The design expected team-code checks against the weather venue map, but that map contains coordinates and venue IDs
instead of team codes. The test validates against the 32 ESPN abbreviations directly.

The CSV fixture contains 1,551 rows through 2026 week 9 (1,449 with complete quarterback cells). Blank quarterback cells
drop that row as designed. The sample week 9 game in the design (BUF vs MIA) was illustrative; actual 2026 week 9 data
featured MIN vs BUF (Carson Wentz vs Josh Allen), providing a real test case for `qb_drag`.

Maintainer limits:

- `qb_drag` uses static pre-game quarterback adjustments (threshold −30 Elo, ~1.2 points). It cannot detect game-day
  scratch announcements unless upstream files update.
- Freshness checks read `sync_state` for `nfelo:projections`, not `nfelo_games.updated_at` (which only bumps on value
  changes). Sync errors or data over 72 hours old silently silence all model output.
- nfelo's repository lacks an open source license ("All rights reserved"). A person should ask the author for permission
  before shipping.
- The 0.6 MB CSV fetches every 12 hours. Archive storage should be verified against runbooks after the first week.

## Follow-ups

- **Batch nfelo lookups in slip views:** Replace individual `db.NfeloForGame` calls per game with a single batch query
  if slip rendering profiles show database contention.
- **Seek author permission for nfelo CSV usage:** Contact the nfelo maintainer for explicit confirmation of automated
  CSV ingest for private club use.
- **Archive size check:** Monitor SQLite raw response archive growth after seven days of automated 12-hour syncs.
