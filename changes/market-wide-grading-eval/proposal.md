---
slug: market-wide-grading-eval
status: proposed
created: 2026-11-24
complexity: medium
risk: low
---

# Market-wide grading evaluation

## Context

Today, grading calibration on `/analyzer/results` and in the eval CLI only replays settled user slips and legs. Even
when scoped across all groups, the sample stays tiny because groups only bet on a small slice of the NFL slate each
week.

Meanwhile, the app already collects league-wide pre-kickoff data—archived game lines and player props, injury reports,
weather, nfelo projections, and prior-week nflverse team and player stats—plus final scores and league-wide weekly
nflverse player stats once games end (along with post-final ESPN box scores for games that had user prop legs). Almost
none of that market data is used to measure how well the A–F grading rubric predicts outcomes.

There are also two signal gaps that hurt grading-eval accuracy today:

- The eval replayer does not rebuild pre-kickoff sharp-book benchmarks from archived odds, so sharp-book line and price
  rules go untested.
- ESPN player game logs are only fetched for players on user slips or opened in research, leaving recent-form (L5) rules
  silent on unpicked players unless eval falls back to strictly prior-week nflverse player stats.

```mermaid
flowchart LR
  SLIPS[Settled user slips] --> REPLAY[Eval replay]
  ARCHIVE[Archived pre-kickoff lines & props] -. ignored unless bet on .-> REPLAY
  OUTCOMES["Final scores, nflverse stats & archived box scores"] -. ignored unless bet on .-> REPLAY
  REPLAY --> EVAL[Calibration & factor report]
```

## Goals and non-goals

**Goals**

- Measure A–F leg calibration, lift over implied odds, flat ROI, Brier score, and factor attribution across all
  collected pre-kickoff game lines and player props for finished games—not just user-submitted slips.
- Deduplicate multi-book quotes into one representative candidate per market selection side (matching how the pick board
  and highlights present lines) so sample sizes and metrics are not inflated by counting the same outcome once per
  sportsbook.
- Reconstruct the full pre-kickoff signal set for every synthetic market leg, including sharp-book benchmarks from
  archived pre-kickoff odds and strictly prior-week nflverse player stats when a player has no pre-kickoff archived ESPN
  game log.
- Stay strictly leak-free: only use pre-kickoff archived odds, lines, injuries, weather, projections, and prior-week
  stats when scoring, and grade outcomes deterministically from final scores, post-final archived ESPN box scores (when
  present), and league-wide nflverse weekly player stats.
- Keep user-slip evaluation intact (including slip-level parlay drag, correlation, and stack rules) alongside
  market-wide leg calibration on `/analyzer/results` and in the offline eval CLI, with zero external API calls during
  evaluation.

**Non-goals**

- Persisting synthetic market legs into the production slip or leg tables, or mixing synthetic market legs into group
  Record Books or slip-level parlay stats.
- Adding new live or post-game ESPN polling to fetch box scores for games nobody bet on.
- Backfilling historical seasons that lack archived pre-kickoff odds, final scores, or post-game player stats.
- Tuning or promoting a `v2` rubric in this change (this change builds the market-wide eval dataset so `v2` tuning has
  real sample size).

## Decision

Extend the eval dataset loader with a market-wide leg mode that parses archived pre-kickoff game lines and player props
for every finished game, deduplicates them into one representative candidate per selection side, rebuilds the full
pre-kickoff signal set (including sharp-book benchmarks and prior-week nflverse L5 fallback), grades each candidate
deterministically against final scores and post-game player stats, and feeds those graded legs through the existing
rubric evaluator.

### Before

```mermaid
flowchart LR
  S[Settled slips] --> L[Eval loader]
  L --> E[Rubric evaluator]
  E --> UI["/analyzer/results & CLI"]
```

### After

```mermaid
flowchart LR
  S[Settled slips] --> L[Eval loader]
  M["Archived pre-kickoff lines & props"] --> B["Candidate builder & sharp benchmarks [NEW]"]
  B --> G["Deterministic market grader [NEW]"]
  F["Final scores, nflverse stats & archived box scores"] --> G
  G --> L
  L --> E[Rubric evaluator]
  E --> UI["/analyzer/results & CLI"]
```

**Why this approach beats alternatives:**

- *Synthesizing at eval replay time vs. persisting synthetic slips:* Replaying read-only from archived snapshots keeps
  slip and leg tables clean, avoids database bloat and migrations, and lets us change sampling or book-selection rules
  without rewriting stored rows.
- *One representative line per selection side vs. scoring every book row:* Sportsbooks quote 6–10 copies of the same
  market. Counting every book row inflates sample size with correlated duplicate outcomes and skews calibration; one
  representative line per side matches what the pick board and highlights actually score.
- *Post-final archived box scores + nflverse weekly stats vs. polling ESPN for every final game:* Combining
  already-archived ESPN box scores with league-wide nflverse weekly player stats covers finished games offline with zero
  new API calls or background polling.
- *Evaluating market legs alongside slip stats vs. replacing slip evals:* Market-wide legs give large sample sizes for
  A–F leg calibration and single-leg factors, while real user slips remain the only source of parlay-level slip grades
  and multi-leg correlation rules.

**Ratings:**

- `complexity: medium`: Coordinates across archive parsing, multi-book candidate deduplication, pre-kickoff
  sharp-benchmark and L5-fallback reconstruction, deterministic market grading across ESPN and nflverse stats, and the
  eval/web layers while enforcing strict pre-kickoff leak-free boundaries, without changing live request paths or
  database schemas.
- `risk: low`: Read-only evaluation over archived and final game data with no effect on live slip creation, payouts, or
  paid API budgets, and every market-grading and signal-reconstruction rule is verifiable with deterministic unit tests.
