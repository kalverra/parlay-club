---
id: odds-math
title: Odds math
type: reference
summary: Formulas in internal/parlay for odds, implied probability, multiplicative, Power, and Shin de-vigging, price edge, fair EV, payouts, and units, plus internal/progress for live leg and parlay progress and hit probability.
tags: [odds, parlay, math, payout, units, reference-price, implied-probability, devig, power-devig, shin-devig, price-edge, fair-ev, live-progress, hit-probability]
---

# Odds math

## Summary

`internal/parlay` holds the pure odds math behind a slip. It converts between American and decimal odds, multiplies legs
into combined odds, picks a reference price, and turns odds into a payout or a unit result. `internal/progress` builds
on those pre-game implied probabilities to compute deterministic live leg and parlay progress and hit probabilities from
the game clock, score, and ESPN box score. Both packages read no database and take every input as an argument. The app
works in decimal odds and shows American odds.

## Conversions

### American to decimal

| Price             | Decimal         |
| ----------------- | --------------- |
| `a > 0`           | `1 + a/100`     |
| `a < 0`           | `1 + 100/\|a\|` |
| `0` (not a price) | `0`             |

| American | Decimal  |
| -------- | -------- |
| `-110`   | `1.909…` |
| `+150`   | `2.5`    |
| `-200`   | `1.5`    |
| `+100`   | `2`      |
| `-100`   | `2`      |

### Decimal to American

The result is rounded to the nearest whole price.

| Decimal               | American                |
| --------------------- | ----------------------- |
| `d ≥ 2`               | `round((d − 1) × 100)`  |
| `d < 2`               | `round(−100 / (d − 1))` |
| `d ≤ 1` (not a price) | `0`                     |

| Decimal  | American |
| -------- | -------- |
| `1.909…` | `-110`   |
| `2.5`    | `+150`   |
| `1.5`    | `-200`   |
| `2`      | `+100`   |
| `1.25`   | `-400`   |

The conversions round-trip for most prices, so `DecimalToAmerican(AmericanToDecimal(-110)) == -110`. The exception is
`-100`, which maps to decimal `2` and comes back as `+100`.

## Combined odds

`CombinedOdds` multiplies the decimal odds of the legs that count. A leg counts unless its result is `PUSH` or `VOID`;
`PENDING`, `NEEDS_GRADING`, `WON`, and `LOST` all count. Pushed and voided legs drop out, so they neither shorten nor
extend the slip. With no counting legs the product is `1`, even money.

## Reference price

`ReferencePrice` takes the candidate decimal prices and the leg's own price. It returns the median of the candidates. An
odd count returns the middle price; an even count returns the mean of the middle two. With no candidates it returns the
leg's own price. It does not modify or reorder the input slice.

The candidates come from the `odds` rows with the same game, market, player, side, and exact line, from the home books
of the slip's members, counting each book once. Because the caller supplies the prices, the package stays pure.

The slip's **reference odds** are the product of the reference decimals of every leg not `PUSH` or `VOID`.

## Payout

`Payout` returns the total return, stake included, for a stake at decimal odds and a profit boost:

```text
stake + stake × (D − 1) × (1 + boost/100)
```

`boostPct` is a percentage, so `25` means a 25% profit boost. A boost of `0` leaves the payout at `stake × D`.

## Units

`Units` gives a graded slip's profit in units:

| Result                     | Units   |
| -------------------------- | ------- |
| `WON`                      | `D − 1` |
| `LOST`                     | `−1`    |
| `PUSH`, `VOID`             | `0`     |
| `PENDING`, `NEEDS_GRADING` | `0`     |

## Implied probability and de-vigging

### Implied probability

`DecimalImpliedProb(decimal float64)` returns the raw implied probability (`1 / decimal`) for decimal odds greater than
`1`, or `0` for non-betting values (`decimal <= 1`, `NaN`, or `Inf`). `ImpliedProb(american int)` delegates to
`DecimalImpliedProb(AmericanToDecimal(american))`, returning `0` for an invalid price of `0`.

| American | Implied probability |
| -------- | ------------------- |
| `-110`   | `0.5238…` (`110/210`) |
| `+100`   | `0.5000`            |
| `+150`   | `0.4000`            |
| `-200`   | `0.6667…` (`2/3`)   |

For parlays, `DecimalImpliedProb(CombinedOdds(legs))` yields `1 / combined` (the product of counting legs' raw implied
probabilities) and returns `0` when no legs count (`CombinedOdds` returns `1`).

### Implied chance display

`internal/web` formats raw implied probabilities with `formatImpliedChance` as compact percentages beside American and
parlay odds:

| Condition (`pct = prob × 100`)     | Output    | Examples                          |
| ---------------------------------- | --------- | --------------------------------- |
| `prob <= 0`, `>= 1`, `NaN`, `Inf`  | `""`      | Empty or all-voided slip (omitted) |
| `pct < 0.1`                        | `"<0.1%"` | `+200000` long-shot parlay        |
| `round(pct × 10) < 100`            | `"%.1f%"` | `+1200` → `"7.7%"` (`9.96%` → `"10%"`) |
| `round(pct) >= 100`                | `">99%"`  | `-50000` extreme heavy favorite   |
| Otherwise (`10` to `99`)           | `"%.0f%"` | `-110` → `"52%"`, `+150` → `"40%"` |

### Two-way de-vigging

All two-way de-vigging helpers share the same input guardrails:

```text
pA = ImpliedProb(priceA)
pB = ImpliedProb(priceB)
overround = pA + pB
```

If either price is invalid (`0`) or `overround` falls outside `[0.95, 1.25]`, every two-way helper returns `ok = false`
(and `0` for all numeric outputs) to guard against corrupt or mispaired quotes.

`VigPct(priceA, priceB)` returns the overround vig as a decimal fraction:

```text
vig = overround - 1
```

#### Multiplicative (`DevigTwoWay`)

`DevigTwoWay(priceA, priceB)` normalizes both raw implied probabilities proportionally so they sum to `1`:

```text
probA = pA / overround
probB = pB / overround
```

#### Power (`DevigTwoWayPower`)

Multiplicative normalization under-corrects favorite-longshot bias in retail prop and touchdown markets.
`DevigTwoWayPower(priceA, priceB)` solves for the exponent `k > 0` such that the powered implied probabilities sum to
`1`:

```text
pA^k + pB^k = 1
probA = pA^k
probB = pB^k
vig   = overround - 1
```

When `|vig| ≤ 1e-13`, it returns `(pA, pB, 0, true)` directly. Otherwise it finds `k` via 80-step bisection starting on
`[0.05, 20]` (doubling the upper bound up to `1e6` if needed) until `|pA^k + pB^k − 1| ≤ 1e-13`. For positive vig
(`k > 1`), raising probabilities to `k` shrinks the smaller probability faster in relative terms, allocating more vig to
the longshot than multiplicative normalization.

#### Shin (`DevigTwoWayShin`)

`DevigTwoWayShin(priceA, priceB)` models bookmaker vig using Shin's insider-trading fraction `z ∈ (0, 1)` and solves
`probA(z) + probB(z) = 1`:

```text
prob_i(z) = (sqrt(z^2 + 4 × (1 − z) × (p_i^2 / overround)) − z) / (2 × (1 − z))
vig       = overround - 1
```

It returns `(pA, pB, 0, true)` when `|vig| ≤ 1e-13`, and falls back to multiplicative normalization when `vig < 0`
(since `z ∈ (0, 1)` requires a positive overround). For positive overround, it solves for `z` via 80-step bisection on
`[0, 1 − 1e-12]` until `|probA + probB − 1| ≤ 1e-13`. Like Power, Shin assigns a higher fair probability to the favorite
and a lower fair probability to the longshot than multiplicative normalization on asymmetric lines.

### Price edge and fair EV

`PriceEdge(retailAmerican, fairProb)` measures probability-point difference between a sharp fair probability and a
retail American price:

```text
edge = fairProb - ImpliedProb(retailAmerican)
```

Positive values mean the retail price pays better than the sharp fair probability; negative values mean extra juice or
shading. Invalid inputs (`retailAmerican == 0` or `fairProb` outside `(0, 1)`) return `0`.

`FairEV(fairProb, retailAmerican)` computes expected value per unit staked as a decimal fraction (used as an internal
scoring signal rather than user-facing `"+EV %"` copy):

```text
ev = fairProb × AmericanToDecimal(retailAmerican) - 1
```

It returns `0` when `fairProb` is not strictly inside `(0, 1)` (including `NaN`, `Inf`, `≤ 0`, or `≥ 1`) or when
`retailAmerican == 0` (`AmericanToDecimal(retailAmerican) ≤ 1`).

## Live progress and hit probability

`internal/progress` evaluates live leg and parlay progress deterministically without network calls or live odds feeds.
Every live market anchors to the leg's pre-game implied probability `preProb = parlay.ImpliedProb(leg.Odds)`, clamped to
`p0 ∈ [0.02, 0.98]` (defaulting to `0.50` when odds are missing or invalid). As remaining game time approaches `0`, live
probability converges toward `0` or `1`, clamped to `[0.01, 0.99]` while a leg is still `LIVE`.

### Time remaining

Regulation is `3600` seconds (four `900`-second quarters). `RemainingFraction(status, period, clock)` returns `remFrac`
in `[0, 1]`:

| Game state | `remFrac` |
| ---------- | --------- |
| `SCHEDULED` (or unknown status) | `1.0` |
| `FINAL` | `0.0` |
| `LIVE` — `"Halftime"` or `"Half"` | `0.50` |
| `LIVE` — `"End of ..."` (`period` `1..4`) | `clamp((4 - period) × 900 / 3600, 0.005, 1.0)` (or `0.75`/`0.50`/`0.25`/`0.005` by label) |
| `LIVE` — regulation `MM:SS` (`period` `1..4`) | `clamp(((4 - period) × 900 + min(clockSec, 900)) / 3600, 0.005, 1.0)` |
| `LIVE` — overtime (`period ≥ 5`) | `clamp(clockSec / 3600, 0.005, 0.167)` |
| `LIVE` — missing or unparseable clock/period | `0.50` |

`ClockLabel` formats `"Final"`, `""` (scheduled), `"Halftime"`, `"End of ..."`, `"Q1 8:42"`, `"OT 4:15"`, `"2OT 4:15"`,
or `"Live"` when both period and clock are absent.

### Leg states and fallbacks

- **Settled results** (`leg.Result != PENDING`) take precedence over game status: `WON` → `CASHED` (`LiveProb = 1.0`,
  `ProgressPct = 100`), `LOST` → `BUSTED` (`0.0`, `0`), `PUSH` → `PUSH` (`Counts = false`), `VOID` → `VOID`
  (`Counts = false`).
- **Scheduled or unlinked games** (`game == nil` or `SCHEDULED`) return `SCHEDULED` with `LiveProb = preProb`,
  `HasLive = false`, and `Counts = true`.
- **Unsupported or incomplete inputs** (`OTHER`, unknown market/side/team, missing line, or missing player ID and name)
  return `UNAVAILABLE` with `LiveProb = preProb`, `HasLive = false`, and `Counts = true`.
- **Pending live box scores** (`box == nil || !box.HasStats`) return `LIVE` with `StatLabel = "Live · stats pending"`,
  `ProgressPct = 0`, `Tone = "neutral"`, and `LiveProb = preProb`.
- **Absent players in a populated live box score** (`box.HasStats == true`) evaluate with `0` stats rather than voiding
  before the game is `FINAL`.
- **Game progress and probability delta:** Every returned `LegProgress` sets
  `GameProgressPct = clamp((1 - remFrac) × 100, 0, 100)` (`0` when `game == nil` or `SCHEDULED`, `100` when `FINAL`),
  `ProbDelta = LiveProb - PreGameProb` (`0` when `Counts == false`), and `ProbTrend` (`"up"` for `ProbDelta > 0.005`,
  `"down"` for `ProbDelta < -0.005`, `"flat"` within the `0.005` deadband and for `PUSH`/`VOID`).

### Normal-model markets

Let `Φ` be the standard normal CDF and `Φ⁻¹` its inverse (Acklam's rational approximation). With
`σ_rem = σ_full × √remFrac`:

**`SPREAD` and `ML`** (`line = 0` for `ML`, `σ_full = 13.5` points):

```text
μ_full   = -line + σ_full × Φ⁻¹(p0)
μ_rem    = μ_full × remFrac
cur      = (teamScore - oppScore) + line
liveProb = clamp(Φ((cur + μ_rem) / σ_rem), 0.01, 0.99)
pct      = liveProb × 100
```

**`TOTAL`, `TEAM_TOTAL`, and yardage/volume player props** use `σ_full`:

| Market | `σ_full` | Monotonic |
| ------ | -------- | --------- |
| `TOTAL` | `13.5` pts | Yes |
| `TEAM_TOTAL` | `9.5` pts | Yes |
| `PASS_YDS` | `0.30 × max(line, 1)` | No |
| `RUSH_YDS` | `0.45 × max(line, 1)` | No |
| `REC_YDS` | `0.50 × max(line, 1)` | No |
| `RUSH_ATT` | `0.30 × max(line, 1)` | Yes |
| `RECEPTIONS` | `0.40 × max(line, 1)` | Yes |
| `KICKER_PTS` | `0.45 × max(line, 1)` | Yes |

For monotonic markets, `cur > line` while `LIVE` settles early: `OVER` becomes `CASHED` (`1.0`, `100%`) and `UNDER`
becomes `BUSTED` (`0.0`, `0%`). Yardage markets can lose yards on negative plays, so they stay `LIVE` until `FINAL`.
Otherwise:

```text
OVER:
  μ_full   = line + σ_full × Φ⁻¹(p0)
  liveProb = clamp(Φ((cur + μ_full×remFrac - line) / σ_rem), 0.01, 0.99)
  pct      = clamp(cur / line × 100, 0, 100)
UNDER:
  μ_full   = line - σ_full × Φ⁻¹(p0)
  liveProb = clamp(Φ((line - cur - μ_full×remFrac) / σ_rem), 0.01, 0.99)
  pct      = clamp((line - cur) / line × 100, 0, 100)
```

### Poisson touchdown models

- **`ANYTIME_TD`** (`YES` or `NO`, defaulting to `YES` if side is empty): if `curTDs ≥ 1` while `LIVE`, `YES` becomes
  `CASHED` and `NO` becomes `BUSTED`. While `curTDs == 0`, with `p0_yes = p0` (or `1 - p0` for `NO`):
  `λ_full = -ln(1 - p0_yes)` and `p_yes = 1 - exp(-λ_full × remFrac)`. `YES` gets `liveProb = clamp(p_yes, 0.01, 0.99)`
  and `0%` fill; `NO` gets `clamp(1 - p_yes, 0.01, 0.99)` and `100%` fill.
- **`PASS_TDS`** (`OVER` or `UNDER`): if `cur > line` while `LIVE`, `OVER` becomes `CASHED` and `UNDER` becomes
  `BUSTED`. Otherwise `solvePoissonLambda` solves `P(Poisson(λ_full) ≥ k_full) = target` over `[0, 20]` via 20-step
  binary search (`k_full = max(floor(line) + 1, 1)` with `target = p0` for `OVER`; `k_full = max(ceil(line), 1)` with
  `target = 1 - p0` for `UNDER`), then evaluates the remaining Poisson tail at `λ_rem = λ_full × remFrac` for
  `needed = floor(line - cur) + 1` (`OVER`) or `1 - P(X_rem ≥ ceil(line - cur))` (`UNDER`).

### Combined parlay aggregation

`EvaluateSlip(legs)` aggregates counting legs (`Counts == true`, excluding `PUSH` and `VOID`):

- `PreGameProb` multiplies `leg.PreGameProb` across counting legs; `LiveProb` multiplies `leg.LiveProb` (`0` if any
  counting leg is `BUSTED`, `1.0` if all counting legs are `CASHED`).
- `ProgressPct` averages counting leg progress (`CASHED = 100`, `LIVE = clamp(leg.ProgressPct, 0, 100)`, `BUSTED` /
  `SCHEDULED` / `UNAVAILABLE = 0`), or `100` when all counting legs are `CASHED`.
- `ScheduledCount` counts counting legs that are neither `CASHED`, `BUSTED`, nor `LIVE` (`SCHEDULED` or `UNAVAILABLE`).
  When `Show` is `true` and the slip is `LIVE`, `Summary` joins `"{HitCount}/{TotalCount} hit"` with
  `"{LiveCount} live"` (if `> 0`) and `"{ScheduledCount} upcoming"` (if `> 0`) using `" · "`.
- `Show` is `true` if any leg (including `PUSH`/`VOID`) has `HasLive == true`. If all legs are `PUSH`/`VOID` on a
  non-empty slip, `State` is `PUSH`.
- `ProbDelta` (`LiveProb - PreGameProb`) and `ProbTrend` (`"up"`, `"down"`, `"flat"` with the `0.005` deadband) are
  computed for the slip (`0` and `"flat"` when empty or all `PUSH`/`VOID`).
- `Tone` is `"lost"` if any leg busted, `"won"` if all counting legs cashed, `"on_track"` when `LiveProb ≥ PreGameProb`,
  and `"trailing"` otherwise.

## Where it shows up

The Slip tab uses reference odds and `Payout` for its payout calculator, and displays `formatImpliedChance` (via
`ImpliedProb` and `DecimalImpliedProb`) on leg rows, payout combined/reference odds, and A-grade highlights. Quick-add
pills prefill the user's home-book price, or the median across all books when the user has no home book or no price
there. The record book uses the same profit term, `stake × (D − 1) × (1 + boost/100)`, for dollar P/L on slips where a
person logged a wager.

## API

### `internal/parlay`

| Function                                                    | Returns                                                         |
| ----------------------------------------------------------- | --------------------------------------------------------------- |
| `AmericanToDecimal(american int) float64`                   | Decimal price, or `0` for an invalid price.                     |
| `DecimalToAmerican(decimal float64) int`                    | Rounded American price, or `0` for `d ≤ 1`.                     |
| `DecimalImpliedProb(decimal float64) float64`               | Raw implied probability in `(0, 1)`, or `0` for `d ≤ 1`, `NaN`, or `Inf`. |
| `ImpliedProb(american int) float64`                         | Raw implied probability in `(0, 1)`, or `0` for `0`.            |
| `DevigTwoWay(priceA, priceB int) (float64, float64, bool)` | Multiplicative fair probabilities `(probA, probB)` and `ok`. |
| `DevigTwoWayPower(priceA, priceB int) (float64, float64, float64, bool)` | Power-method fair probabilities `(probA, probB, vig, ok)`. |
| `DevigTwoWayShin(priceA, priceB int) (float64, float64, float64, bool)` | Shin-model fair probabilities `(probA, probB, vig, ok)`. |
| `VigPct(priceA, priceB int) (float64, bool)` | Two-way overround vig (`overround - 1`) and `ok`. |
| `PriceEdge(retailAmerican int, fairProb float64) float64` | `fairProb - ImpliedProb(retailAmerican)`, or `0`. |
| `FairEV(fairProb float64, retailAmerican int) float64` | `fairProb * AmericanToDecimal(retailAmerican) - 1`, or `0`. |
| `CombinedOdds(legs []Leg) float64`                          | Product of counting leg decimals, or `1`.               |
| `ReferencePrice(candidates []float64, own float64) float64` | Candidate median, or `own` when empty.                  |
| `Payout(stake, decimal, boostPct float64) float64`          | Total return including the stake.                       |
| `Units(result Result, decimal float64) float64`             | Profit in units for a graded slip.                      |
| `Result.Counts() bool`                                      | False for `PUSH` and `VOID`.                            |

### `internal/progress`

| Function                                                                  | Returns                                                                 |
| ------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `RemainingFraction(status db.GameStatus, period *int, clock *string) float64` | Fraction of regulation remaining in `[0, 1]` (up to `0.167` in OT). |
| `ClockLabel(status db.GameStatus, period *int, clock *string) string`     | Compact clock display (`"Q3 8:42"`, `"Halftime"`, `"Final"`, etc.).     |
| `EvaluateLeg(leg db.Leg, game *db.Game, box *espn.BoxScore) LegProgress`  | Live or settled progress, `GameProgressPct`, tone, stat/clock labels, hit probability, and `ProbDelta`/`ProbTrend`. |
| `EvaluateSlip(legs []LegProgress) SlipProgress`                           | Combined parlay progress, summary, `ScheduledCount`, hit probability, and `ProbDelta`/`ProbTrend`. |
