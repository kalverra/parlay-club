package progress

import "math"

const (
	minAnchorProb = 0.02
	maxAnchorProb = 0.98
	minLiveProb   = 0.01
	maxLiveProb   = 0.99
)

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// anchorProb clamps pre-game implied probability to [0.02, 0.98], falling back
// to 0.50 when odds are missing/zero.
func anchorProb(preGameProb float64) float64 {
	if preGameProb <= 0 || math.IsNaN(preGameProb) || math.IsInf(preGameProb, 0) {
		return 0.50
	}
	return clamp(preGameProb, minAnchorProb, maxAnchorProb)
}

// normCDF returns the standard normal cumulative distribution function Φ(x).
func normCDF(x float64) float64 {
	if math.IsNaN(x) {
		return 0.5
	}
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// normInv returns the inverse standard normal CDF Φ⁻¹(p) using Acklam's
// rational approximation.
func normInv(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}

	const (
		a1 = -3.969683028665376e+01
		a2 = 2.209460984245205e+02
		a3 = -2.759285104469687e+02
		a4 = 1.383577518672690e+02
		a5 = -3.066479806614716e+01
		a6 = 2.506628277459239e+00

		b1 = -5.447609879822406e+01
		b2 = 1.615858368580409e+02
		b3 = -1.556989798598866e+02
		b4 = 6.680131188771972e+01
		b5 = -1.328068155288572e+01

		c1 = -7.784894002430293e-03
		c2 = -3.223964580411365e-01
		c3 = -2.400758277161838e+00
		c4 = -2.549732539343734e+00
		c5 = 4.374664141464968e+00
		c6 = 2.938163982698783e+00

		d1 = 7.784695709041462e-03
		d2 = 3.224671290700398e-01
		d3 = 2.445134137142996e+00
		d4 = 3.754408661907416e+00

		pLow  = 0.02425
		pHigh = 1 - pLow
	)

	if p < pLow {
		q := math.Sqrt(-2 * math.Log(p))
		return (((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) / ((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	}
	if p <= pHigh {
		q := p - 0.5
		r := q * q
		return (((((a1*r+a2)*r+a3)*r+a4)*r+a5)*r + a6) * q / (((((b1*r+b2)*r+b3)*r+b4)*r+b5)*r + 1)
	}
	q := math.Sqrt(-2 * math.Log(1-p))
	return -(((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) / ((((d1*q+d2)*q+d3)*q+d4)*q + 1)
}
