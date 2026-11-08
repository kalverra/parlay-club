// Package parlay holds the pure odds math behind a slip: converting between
// American and decimal odds, combining leg prices, picking a reference price,
// and turning odds into a payout or a unit result.
package parlay

import (
	"math"
	"slices"
)

// Result is how a leg or a slip finished.
type Result string

// The results a leg or slip can carry.
const (
	Pending      Result = "PENDING"
	NeedsGrading Result = "NEEDS_GRADING"
	Won          Result = "WON"
	Lost         Result = "LOST"
	Push         Result = "PUSH"
	Void         Result = "VOID"
)

// Counts reports whether a leg with this result still counts toward the
// slip's odds. Pushed and voided legs drop out.
func (r Result) Counts() bool {
	return r != Push && r != Void
}

// IsLegResult reports whether r is a result a leg can carry.
func (r Result) IsLegResult() bool {
	switch r {
	case Pending, NeedsGrading, Won, Lost, Push, Void:
		return true
	default:
		return false
	}
}

// IsSlipResult reports whether r is a result a slip can carry. A slip settles
// to won, lost, or push; needs-grading is only for its legs.
func (r Result) IsSlipResult() bool {
	switch r {
	case Pending, Won, Lost, Push:
		return true
	default:
		return false
	}
}

// Leg is one leg's contribution to a slip's combined odds.
type Leg struct {
	Decimal float64
	Result  Result
}

// AmericanToDecimal converts American odds to decimal odds. A positive price
// pays that many units per 100 staked; a negative price is what must be staked
// to win 100. An invalid price of 0 converts to 0.
func AmericanToDecimal(american int) float64 {
	switch {
	case american > 0:
		return 1 + float64(american)/100
	case american < 0:
		return 1 + 100/-float64(american)
	default:
		return 0
	}
}

// DecimalToAmerican converts decimal odds to American odds, rounded to the
// nearest whole price. A decimal of 1 or less has no American price and
// converts to 0.
func DecimalToAmerican(decimal float64) int {
	if decimal <= 1 {
		return 0
	}
	if decimal >= 2 {
		return int(math.Round((decimal - 1) * 100))
	}
	return int(math.Round(-100 / (decimal - 1)))
}

// CombinedOdds returns the product of the decimal odds of the legs that count.
// Pushed and voided legs are dropped. No counting legs is an even-money
// product of 1.
func CombinedOdds(legs []Leg) float64 {
	combined := 1.0
	for _, leg := range legs {
		if !leg.Result.Counts() {
			continue
		}
		combined *= leg.Decimal
	}
	return combined
}

// ReferencePrice returns the median of the candidate decimal prices. With no
// candidates it returns own, the leg's own decimal price. An even count uses
// the mean of the two middle prices. It does not modify candidates.
func ReferencePrice(candidates []float64, own float64) float64 {
	if len(candidates) == 0 {
		return own
	}

	prices := slices.Clone(candidates)
	slices.Sort(prices)

	middle := len(prices) / 2
	if len(prices)%2 == 1 {
		return prices[middle]
	}
	return (prices[middle-1] + prices[middle]) / 2
}

// Payout returns the total return for a stake at decimal odds, including the
// stake and any profit boost. boostPct is a percentage, so 25 means +25%.
func Payout(stake, decimal, boostPct float64) float64 {
	return stake + stake*(decimal-1)*(1+boostPct/100)
}

// Units returns the profit in units of a graded slip at decimal odds: decimal
// minus 1 for a win, -1 for a loss, and 0 for a push or void. A result that is
// not graded yet has no units and returns 0.
func Units(result Result, decimal float64) float64 {
	switch result {
	case Won:
		return decimal - 1
	case Lost:
		return -1
	case Push, Void, Pending, NeedsGrading:
		return 0
	default:
		return 0
	}
}
