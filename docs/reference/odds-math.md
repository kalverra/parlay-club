---
id: odds-math
title: Odds math
type: reference
summary: The formulas in internal/parlay for converting American and decimal odds, computing implied probability and two-way de-vigging, measuring price edge, combining legs, picking a reference price, and computing payout and units.
tags: [odds, parlay, math, payout, units, reference-price, implied-probability, devig, price-edge]
---

# Odds math

## Summary

`internal/parlay` holds the pure odds math behind a slip. It converts between American and decimal odds, multiplies legs
into combined odds, picks a reference price, and turns odds into a payout or a unit result. It reads no database and
takes every input as an argument. The app works in decimal odds and shows American odds.

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

`ImpliedProb` converts an American price `a` to its raw implied probability using `AmericanToDecimal(a)`:

```text
p = 1 / AmericanToDecimal(a)
```

An invalid price of `0` returns `0`.

| American | Implied probability |
| -------- | ------------------- |
| `-110`   | `0.5238…` (`110/210`) |
| `+100`   | `0.5000`            |
| `+150`   | `0.4000`            |
| `-200`   | `0.6667…` (`2/3`)   |

### Two-way de-vigging

`DevigTwoWay(priceA, priceB)` removes bookmaker vig from a two-way market by normalizing both raw implied probabilities
to sum to `1`:

```text
pA = ImpliedProb(priceA)
pB = ImpliedProb(priceB)
overround = pA + pB
probA = pA / overround
probB = pB / overround
```

It returns `ok = false` (and `0, 0`) if either price is invalid (`0`) or if `overround` falls outside `[0.95, 1.25]`,
which guards against corrupt or mispaired quotes.

### Price edge

`PriceEdge(retailAmerican, fairProb)` compares a retail American price against a sharp fair probability:

```text
edge = fairProb - ImpliedProb(retailAmerican)
```

Positive values mean the retail price pays better than the sharp fair probability; negative values mean extra juice or
shading. Invalid inputs (`retailAmerican == 0` or `fairProb` outside `(0, 1)`) return `0`.

## Where it shows up

The Slip tab uses reference odds and `Payout` for its payout calculator. Quick-add pills prefill the user's home-book
price, or the median across all books when the user has no home book or no price there. The record book uses the same
profit term, `stake × (D − 1) × (1 + boost/100)`, for dollar P/L on slips where a person logged a wager.

## API

| Function                                                    | Returns                                                 |
| ----------------------------------------------------------- | ------------------------------------------------------- |
| `AmericanToDecimal(american int) float64`                   | Decimal price, or `0` for an invalid price.             |
| `DecimalToAmerican(decimal float64) int`                    | Rounded American price, or `0` for `d ≤ 1`.             |
| `ImpliedProb(american int) float64`                         | Raw implied probability in `(0, 1)`, or `0` for `0`.    |
| `DevigTwoWay(priceA, priceB int) (float64, float64, bool)`  | Fair probabilities `(probA, probB)` and `ok`.           |
| `PriceEdge(retailAmerican int, fairProb float64) float64`   | `fairProb - ImpliedProb(retailAmerican)`, or `0`.       |
| `CombinedOdds(legs []Leg) float64`                          | Product of counting leg decimals, or `1`.               |
| `ReferencePrice(candidates []float64, own float64) float64` | Candidate median, or `own` when empty.                  |
| `Payout(stake, decimal, boostPct float64) float64`          | Total return including the stake.                       |
| `Units(result Result, decimal float64) float64`             | Profit in units for a graded slip.                      |
| `Result.Counts() bool`                                      | False for `PUSH` and `VOID`.                            |
