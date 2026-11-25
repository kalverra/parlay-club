package parlay

import "math"

const (
	powerMinK   = 0.05
	powerMaxK   = 20.0
	powerMaxCap = 1e6
	devigIters  = 80
	devigTol    = 1e-13
	shinMaxZ    = 1.0 - 1e-12
)

func twoWayImplied(priceA, priceB int) (pA, pB, overround float64, ok bool) {
	pA = ImpliedProb(priceA)
	pB = ImpliedProb(priceB)
	if pA <= 0 || pB <= 0 {
		return 0, 0, 0, false
	}
	overround = pA + pB
	if overround < minTwoWayOverround || overround > maxTwoWayOverround {
		return 0, 0, 0, false
	}
	return pA, pB, overround, true
}

// VigPct returns the overround vig (pA + pB - 1) for a two-way market.
// It returns ok=false if either price is invalid or the combined overround
// falls outside [0.95, 1.25].
func VigPct(priceA, priceB int) (vig float64, ok bool) {
	_, _, overround, ok := twoWayImplied(priceA, priceB)
	if !ok {
		return 0, false
	}
	return overround - 1, true
}

// DevigTwoWayPower solves pA^k + pB^k = 1 via bisection on k in (0.05, 20)
// and returns (pA^k, pB^k, vigPct, ok). It returns ok=false if either price
// is invalid or the combined overround falls outside [0.95, 1.25].
func DevigTwoWayPower(priceA, priceB int) (probA, probB, vig float64, ok bool) {
	pA, pB, overround, ok := twoWayImplied(priceA, priceB)
	if !ok {
		return 0, 0, 0, false
	}
	vig = overround - 1
	if math.Abs(vig) <= devigTol {
		return pA, pB, 0, true
	}

	lo, hi := powerMinK, powerMaxK
	for math.Pow(pA, hi)+math.Pow(pB, hi) > 1 && hi < powerMaxCap {
		hi *= 2
	}

	var k float64
	for range devigIters {
		k = (lo + hi) / 2
		sum := math.Pow(pA, k) + math.Pow(pB, k)
		if math.Abs(sum-1) <= devigTol {
			break
		}
		if sum > 1 {
			lo = k
		} else {
			hi = k
		}
	}
	return math.Pow(pA, k), math.Pow(pB, k), vig, true
}

// DevigTwoWayShin solves Shin's insider-trading two-way model for z in (0, 1)
// and returns fair probabilities and overround vig. When overround <= 1 (no
// positive vig), it falls back to multiplicative normalization. It returns
// ok=false if either price is invalid or the combined overround falls outside
// [0.95, 1.25].
func DevigTwoWayShin(priceA, priceB int) (probA, probB, vig float64, ok bool) {
	pA, pB, overround, ok := twoWayImplied(priceA, priceB)
	if !ok {
		return 0, 0, 0, false
	}
	vig = overround - 1
	if math.Abs(vig) <= devigTol {
		return pA, pB, 0, true
	}
	if vig < 0 {
		return pA / overround, pB / overround, vig, true
	}

	lo, hi := 0.0, shinMaxZ
	for range devigIters {
		z := (lo + hi) / 2
		probA = shinProb(pA, overround, z)
		probB = shinProb(pB, overround, z)
		sum := probA + probB
		if math.Abs(sum-1) <= devigTol {
			break
		}
		if sum > 1 {
			lo = z
		} else {
			hi = z
		}
	}
	return probA, probB, vig, true
}

func shinProb(implied, overround, z float64) float64 {
	oneMinusZ := 1 - z
	disc := z*z + 4*oneMinusZ*(implied*implied/overround)
	return (math.Sqrt(disc) - z) / (2 * oneMinusZ)
}

// FairEV returns fairProb * AmericanToDecimal(retailAmerican) - 1 (internal
// scoring signal; never rendered as raw "+EV %" copy). Returns 0 if
// retailAmerican is 0 or fairProb is outside (0, 1).
func FairEV(fairProb float64, retailAmerican int) float64 {
	if !(fairProb > 0 && fairProb < 1) {
		return 0
	}
	decimal := AmericanToDecimal(retailAmerican)
	if decimal <= 1 {
		return 0
	}
	return fairProb*decimal - 1
}
