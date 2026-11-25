package modelv2

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// Supported post-hoc probability calibration methods.
const (
	CalibMethodIsotonic = "isotonic"
	CalibMethodPlatt    = "platt"
)

const (
	minCalibProb = 1e-6
	maxCalibProb = 1.0 - 1e-6
)

// CalibratorArtifact holds post-hoc probability calibration parameters trained
// on held-out chronological splits.
//
//   - "isotonic": piecewise-linear interpolation over non-decreasing (KnotsX, KnotsY),
//     clamped outside [KnotsX[0], KnotsX[last]].
//   - "platt": logistic calibration in logit space, sigmoid(PlattA*logit(p) + PlattB).
type CalibratorArtifact struct {
	Method string    `json:"method"`
	KnotsX []float64 `json:"knots_x,omitempty"`
	KnotsY []float64 `json:"knots_y,omitempty"`
	PlattA float64   `json:"platt_a,omitempty"`
	PlattB float64   `json:"platt_b,omitempty"`
}

// Validate checks that the calibrator configuration is well-formed and monotonic.
func (c CalibratorArtifact) Validate() error {
	switch c.Method {
	case CalibMethodIsotonic:
		if len(c.KnotsX) < 2 {
			return fmt.Errorf("isotonic requires at least 2 knots, got %d", len(c.KnotsX))
		}
		if len(c.KnotsX) != len(c.KnotsY) {
			return fmt.Errorf("isotonic knots_x (%d) and knots_y (%d) length mismatch", len(c.KnotsX), len(c.KnotsY))
		}
		for i := range c.KnotsX {
			x, y := c.KnotsX[i], c.KnotsY[i]
			if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > 1 {
				return fmt.Errorf("knots_x[%d]=%v must be finite in [0, 1]", i, x)
			}
			if math.IsNaN(y) || math.IsInf(y, 0) || y < 0 || y > 1 {
				return fmt.Errorf("knots_y[%d]=%v must be finite in [0, 1]", i, y)
			}
			if i > 0 && x < c.KnotsX[i-1] {
				return fmt.Errorf("knots_x must be non-decreasing: %v < %v at index %d", x, c.KnotsX[i-1], i)
			}
			if i > 0 && y < c.KnotsY[i-1] {
				return fmt.Errorf("knots_y must be non-decreasing: %v < %v at index %d", y, c.KnotsY[i-1], i)
			}
		}
		return nil
	case CalibMethodPlatt:
		if math.IsNaN(c.PlattA) || math.IsInf(c.PlattA, 0) || c.PlattA <= 0 {
			return fmt.Errorf("platt_a %v must be finite > 0", c.PlattA)
		}
		if math.IsNaN(c.PlattB) || math.IsInf(c.PlattB, 0) {
			return fmt.Errorf("platt_b %v must be finite", c.PlattB)
		}
		return nil
	case "":
		return errors.New("method must not be empty")
	default:
		return fmt.Errorf("unsupported calibrator method %q", c.Method)
	}
}

// Calibrate maps a raw or blended probability p onto a calibrated probability
// in [1e-6, 1-1e-6].
func (c CalibratorArtifact) Calibrate(p float64) float64 {
	if math.IsNaN(p) {
		return 0.5
	}
	p = clampCalibProb(p)

	switch c.Method {
	case CalibMethodPlatt:
		logit := math.Log(p / (1.0 - p))
		z := c.PlattA*logit + c.PlattB
		return clampCalibProb(sigmoid(z))
	case CalibMethodIsotonic:
		n := len(c.KnotsX)
		if n < 2 || n != len(c.KnotsY) {
			return p
		}
		if p <= c.KnotsX[0] {
			return clampCalibProb(c.KnotsY[0])
		}
		if p >= c.KnotsX[n-1] {
			return clampCalibProb(c.KnotsY[n-1])
		}
		hi := sort.Search(n, func(i int) bool { return c.KnotsX[i] > p })
		lo := hi - 1
		dx := c.KnotsX[hi] - c.KnotsX[lo]
		if dx <= 0 {
			return clampCalibProb(c.KnotsY[lo])
		}
		t := (p - c.KnotsX[lo]) / dx
		return clampCalibProb(c.KnotsY[lo] + t*(c.KnotsY[hi]-c.KnotsY[lo]))
	default:
		return p
	}
}

func clampCalibProb(p float64) float64 {
	if p < minCalibProb {
		return minCalibProb
	}
	if p > maxCalibProb {
		return maxCalibProb
	}
	return p
}

func sigmoid(z float64) float64 {
	if z >= 0 {
		return 1.0 / (1.0 + math.Exp(-z))
	}
	e := math.Exp(z)
	return e / (1.0 + e)
}
