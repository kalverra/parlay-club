package modelv2_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

func TestCalibratorPlatt(t *testing.T) {
	t.Parallel()

	identity := modelv2.CalibratorArtifact{
		Method: modelv2.CalibMethodPlatt,
		PlattA: 1.0,
		PlattB: 0.0,
	}
	require.NoError(t, identity.Validate())

	for _, p := range []float64{0.01, 0.15, 0.50, 0.72, 0.99} {
		assert.InDelta(t, p, identity.Calibrate(p), 1e-12)
	}
	assert.InDelta(t, 0.5, identity.Calibrate(math.NaN()), 1e-12)
	assert.GreaterOrEqual(t, identity.Calibrate(-0.5), 1e-6)
	assert.LessOrEqual(t, identity.Calibrate(1.5), 1.0-1e-6)

	shifted := modelv2.CalibratorArtifact{
		Method: modelv2.CalibMethodPlatt,
		PlattA: 1.25,
		PlattB: -0.20,
	}
	require.NoError(t, shifted.Validate())

	// Monotonicity and exact logit formula check.
	p1 := shifted.Calibrate(0.30)
	p2 := shifted.Calibrate(0.50)
	p3 := shifted.Calibrate(0.70)
	assert.Less(t, p1, p2)
	assert.Less(t, p2, p3)
	wantMid := 1.0 / (1.0 + math.Exp(0.20))
	assert.InDelta(t, wantMid, p2, 1e-12)
}

func TestCalibratorIsotonic(t *testing.T) {
	t.Parallel()

	iso := modelv2.CalibratorArtifact{
		Method: modelv2.CalibMethodIsotonic,
		KnotsX: []float64{0.10, 0.40, 0.40, 0.70, 0.90},
		KnotsY: []float64{0.12, 0.38, 0.42, 0.68, 0.88},
	}
	require.NoError(t, iso.Validate())

	tests := []struct {
		name string
		p    float64
		want float64
	}{
		{name: "below first knot clamps", p: 0.02, want: 0.12},
		{name: "at first knot", p: 0.10, want: 0.12},
		{name: "midpoint between 0.10 and 0.40", p: 0.25, want: 0.25},
		{name: "at duplicate x knot", p: 0.40, want: 0.42},
		{name: "midpoint between 0.40 and 0.70", p: 0.55, want: 0.55},
		{name: "above last knot clamps", p: 0.98, want: 0.88},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, iso.Calibrate(tt.p), 1e-12)
		})
	}
}

func TestCalibratorValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cal  modelv2.CalibratorArtifact
	}{
		{name: "empty method", cal: modelv2.CalibratorArtifact{}},
		{name: "unknown method", cal: modelv2.CalibratorArtifact{Method: "beta"}},
		{
			name: "platt non-positive A",
			cal:  modelv2.CalibratorArtifact{Method: modelv2.CalibMethodPlatt, PlattA: 0, PlattB: 0},
		},
		{
			name: "platt nan B",
			cal:  modelv2.CalibratorArtifact{Method: modelv2.CalibMethodPlatt, PlattA: 1, PlattB: math.NaN()},
		},
		{
			name: "isotonic too few knots",
			cal: modelv2.CalibratorArtifact{
				Method: modelv2.CalibMethodIsotonic,
				KnotsX: []float64{0.5},
				KnotsY: []float64{0.5},
			},
		},
		{
			name: "isotonic length mismatch",
			cal: modelv2.CalibratorArtifact{
				Method: modelv2.CalibMethodIsotonic,
				KnotsX: []float64{0.2, 0.8},
				KnotsY: []float64{0.2},
			},
		},
		{
			name: "isotonic decreasing X",
			cal: modelv2.CalibratorArtifact{
				Method: modelv2.CalibMethodIsotonic,
				KnotsX: []float64{0.6, 0.4},
				KnotsY: []float64{0.4, 0.6},
			},
		},
		{
			name: "isotonic decreasing Y",
			cal: modelv2.CalibratorArtifact{
				Method: modelv2.CalibMethodIsotonic,
				KnotsX: []float64{0.2, 0.8},
				KnotsY: []float64{0.7, 0.3},
			},
		},
		{
			name: "isotonic knot out of [0,1]",
			cal: modelv2.CalibratorArtifact{
				Method: modelv2.CalibMethodIsotonic,
				KnotsX: []float64{0.2, 1.1},
				KnotsY: []float64{0.2, 0.9},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, tt.cal.Validate())
		})
	}
}

func BenchmarkCalibratePlatt(b *testing.B) {
	cal := modelv2.CalibratorArtifact{
		Method: modelv2.CalibMethodPlatt,
		PlattA: 1.15,
		PlattB: -0.05,
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = cal.Calibrate(0.54)
	}
}

func BenchmarkCalibrateIsotonic(b *testing.B) {
	knotsX := make([]float64, 32)
	knotsY := make([]float64, 32)
	for i := range knotsX {
		v := float64(i) / 31.0
		knotsX[i] = v
		knotsY[i] = v
	}
	cal := modelv2.CalibratorArtifact{
		Method: modelv2.CalibMethodIsotonic,
		KnotsX: knotsX,
		KnotsY: knotsY,
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = cal.Calibrate(0.54)
	}
}
