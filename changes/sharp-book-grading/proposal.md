---
slug: sharp-book-grading
status: implementing
created: "2026-11-23"
accepted_by:
    - Adam Hamrick
accepted_at: "2026-11-23T19:19:00Z"
complexity: medium
risk: medium
---

# Sharp bookmaker odds comparison

## Context

Today, the analyzer grades a leg's line and price using only nfelo spread cushion, spread hooks, total key numbers, and
heavy chalk. It never compares the retail sportsbook's line or price (DraftKings, FanDuel) against sharp market-making
books.

Retail books regularly shade popular overs, favorites, and player props away from the sharp market. Without a sharp
benchmark, a leg with strong recent form can earn an A grade even when DraftKings or FanDuel is offering a worse number
or heavier vig than sharp books.

Meanwhile, the odds sync already pulls `regions=us` from Parlay API and The Odds API—which returns sharp-leaning books
such as LowVig and BetOnline alongside retail books—and stores every bookmaker's quote. The analyzer ignores those other
books completely.

```mermaid
flowchart LR
  APIs["Parlay API + The Odds API"] --> Store["Stored Odds (all books)"]
  Store --> Board["Pick Board (home book or median)"]
  Board --> Leg["Picked Leg (retail line & price)"]
  Leg --> Scorer["ScoreLeg (hooks, chalk, nfelo only)"]
  Store -. "other books unused" .-> Scorer
```

## Goals and non-goals

**Goals:**

- Build a tiered sharp-book benchmark for game lines and props, prioritizing true sharp/low-vig market makers (such as
  Pinnacle when available within credit budgets), falling back to sharp US-region books already returned by `regions=us`
  (such as LowVig and BetOnline), and using multi-book consensus when no sharp book quotes a market.
- De-vig two-way sharp quotes to estimate fair probability and fair line, then compare the leg's retail line and price
  against that sharp benchmark.
- Add sharp-line and sharp-price comparison factors (both favorable edge and bad-number penalty) to the analyzer's Line
  & Price category, with plain sharp-friend explanations and no +EV lectures.
- Stay strictly within the existing Parlay API (1,000 credits/month) and The Odds API (500 credits/month) free-tier
  budgets and credit floors.

**Non-goals:**

- Real-time arbitrage alerts, line-shopping across offshore books, or +EV percentage and Kelly staking advice.
- Adding a paid odds provider or raising the monthly credit spend.
- Blocking picks that have a bad price relative to sharp books; every pick stays allowed.
- Backtesting or tuning factor weights against historical results (covered separately by `grading-evals`).

## Decision

### Before

```mermaid
flowchart LR
  Odds["Stored Odds"] --> Select["Pick Retail Quote (DK / FD)"]
  Select --> LinePrice["Line & Price Factors (nfelo, hook, key number, chalk)"]
  LinePrice --> Grade["Leg Grade (A–F)"]
```

### After

```mermaid
flowchart LR
  Odds["Stored Odds (retail + sharp books)"] --> Select["Pick Retail Quote (DK / FD)"]
  Odds --> Sharp["Sharp Benchmark (tiered sharp books + de-vig) [new]"]
  Select --> Compare["Retail vs Sharp Comparison [new]"]
  Sharp --> Compare
  Compare --> LinePrice["Line & Price Factors (+ sharp edge / bad number)"]
  Select --> LinePrice
  LinePrice --> Grade["Leg Grade (A–F) + Sharp-Friend Note"]
```

Compute a deterministic sharp benchmark from stored multi-book odds, de-vig two-way sharp quotes, and feed the
retail-versus-sharp line and price gap into the analyzer's Line & Price scoring category.

This beats relying on nfelo alone because nfelo only covers pre-game spreads and win probabilities, missing totals,
player props, and late market moves that sharp books price immediately. It beats adding a paid sharp-odds feed because
sharp-leaning books are already present in our `regions=us` responses (and credit-neutral sharp book queries can be used
where supported), preserving our free-tier guardrails.

`complexity: medium` fits because the change spans odds ingestion, bookmaker tiering, de-vig math, and analyzer scoring
while preserving credit-budget invariants across two odds providers. `risk: medium` fits because flawed de-vigging or
treating a stale or one-sided quote as sharp could skew grades across the board, though deterministic unit tests on real
captured odds payloads and fail-soft behavior when sharp quotes are absent bound the blast radius.
