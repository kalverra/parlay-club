package modelv2

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSobolPolynomialsAndDirectionNumbers(t *testing.T) {
	t.Parallel()

	for i, spec := range sobolDimParams {
		dim := i + 2
		require.Len(t, spec.initM, spec.degree, "dim %d initM length", dim)
		for k, mk := range spec.initM {
			assert.Equal(t, uint32(1), mk&1, "dim %d m[%d]=%d must be odd", dim, k+1, mk)
			maxVal := uint32(1) << (k + 1)
			assert.Less(t, mk, maxVal, "dim %d m[%d]=%d must be < 2^%d", dim, k+1, mk, k+1)
		}
		assert.True(
			t,
			isPrimitivePolyGF2(spec.degree, spec.poly),
			"dim %d poly (s=%d, a=%d) not primitive",
			dim,
			spec.degree,
			spec.poly,
		)
	}
}

func TestSobolNormalsQualityAcrossAllDimensions(t *testing.T) {
	t.Parallel()

	normals := qmcNormalsOnce()
	require.Len(t, normals, qmcSamples*maxQMCDims)

	means := make([]float64, maxQMCDims)
	vars := make([]float64, maxQMCDims)
	for r := range qmcSamples {
		base := r * maxQMCDims
		for d := range maxQMCDims {
			v := normals[base+d]
			means[d] += v
			vars[d] += v * v
		}
	}
	for d := range maxQMCDims {
		means[d] /= float64(qmcSamples)
		vars[d] /= float64(qmcSamples)
		assert.InDelta(t, 0.0, means[d], 1e-12, "dim %d mean", d)
		assert.InDelta(t, 1.0, vars[d], 2e-3, "dim %d variance", d)
	}

	maxPairCorr := 0.0
	maxPairCorr16 := 0.0
	for i := range maxQMCDims {
		for j := i + 1; j < maxQMCDims; j++ {
			var cov float64
			var orthant int
			for r := range qmcSamples {
				ui := normals[r*maxQMCDims+i]
				uj := normals[r*maxQMCDims+j]
				cov += ui * uj
				if ui <= 0 && uj <= 0 {
					orthant++
				}
			}
			corr := cov / float64(qmcSamples)
			if math.Abs(corr) > maxPairCorr {
				maxPairCorr = math.Abs(corr)
			}
			if j < 16 && math.Abs(corr) > maxPairCorr16 {
				maxPairCorr16 = math.Abs(corr)
			}
			assert.InDelta(t, 0.0, corr, 0.025, "pair (%d,%d) correlation", i, j)
			assert.InDelta(t, 0.25, float64(orthant)/float64(qmcSamples), 0.005, "pair (%d,%d) orthant", i, j)
		}
	}
	t.Logf("max pairwise Sobol normal sample correlation (16 dims: %.6f, 32 dims: %.6f)", maxPairCorr16, maxPairCorr)
}

func TestNormInvRoundTrip(t *testing.T) {
	t.Parallel()

	assert.True(t, math.IsInf(NormInv(0), -1))
	assert.True(t, math.IsInf(NormInv(1), 1))
	assert.InDelta(t, 0.0, NormInv(0.5), 1e-12)
	assert.InDelta(t, 0.5, NormCDF(math.NaN()), 1e-12)

	for _, p := range []float64{1e-6, 1e-4, 0.01, 0.02425, 0.1, 0.25, 0.5, 0.75, 0.9, 0.97575, 0.99, 1 - 1e-4, 1 - 1e-6} {
		z := NormInv(p)
		roundTrip := NormCDF(z)
		assert.InDelta(t, p, roundTrip, 1e-9, "p=%v", p)
	}
}

// isPrimitivePolyGF2 checks that x^s + sum_{i=1}^{s-1} a_i x^{s-i} + 1 has
// multiplicative period 2^s - 1 in GF(2)[x]/(P(x)).
func isPrimitivePolyGF2(degree int, polyBits uint32) bool {
	if degree < 1 || degree > 15 {
		return false
	}
	period := (1 << degree) - 1
	// Build full polynomial bitmask: x^s + (polyBits << 1) + 1.
	fullPoly := uint32(1<<degree) | (polyBits << 1) | 1
	mask := uint32(period)

	// Multiply state by x mod fullPoly using LFSR shift.
	state := uint32(1)
	for step := 1; step <= period; step++ {
		highBit := (state >> (degree - 1)) & 1
		state = (state << 1) & mask
		if highBit != 0 {
			state ^= fullPoly & mask
		}
		if step < period && state == 1 {
			return false
		}
	}
	return state == 1
}
