package parlay_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestVigPct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		priceA  int
		priceB  int
		wantVig float64
		wantOK  bool
	}{
		{
			name:    "symmetric minus 110",
			priceA:  -110,
			priceB:  -110,
			wantVig: 10.0 / 210.0,
			wantOK:  true,
		},
		{
			name:    "even money zero vig",
			priceA:  100,
			priceB:  100,
			wantVig: 0,
			wantOK:  true,
		},
		{
			name:    "favorite longshot minus 400 plus 300",
			priceA:  -400,
			priceB:  300,
			wantVig: 0.05,
			wantOK:  true,
		},
		{
			name:    "valid slight under-round",
			priceA:  105,
			priceB:  105,
			wantVig: 200.0/205.0 - 1,
			wantOK:  true,
		},
		{
			name:   "invalid zero side A",
			priceA: 0,
			priceB: -110,
			wantOK: false,
		},
		{
			name:   "invalid zero side B",
			priceA: -110,
			priceB: 0,
			wantOK: false,
		},
		{
			name:   "corrupt under-round below 0.95",
			priceA: 200,
			priceB: 200,
			wantOK: false,
		},
		{
			name:   "corrupt over-round above 1.25",
			priceA: -300,
			priceB: -300,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vig, ok := parlay.VigPct(tt.priceA, tt.priceB)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Zero(t, vig)
				return
			}
			assert.InDelta(t, tt.wantVig, vig, 1e-12)
		})
	}
}

func TestDevigTwoWayPower(t *testing.T) {
	t.Parallel()

	t.Run("symmetric and zero-vig lines", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			priceA  int
			priceB  int
			wantVig float64
		}{
			{name: "standard minus 110", priceA: -110, priceB: -110, wantVig: 10.0 / 210.0},
			{name: "high vig minus 125", priceA: -125, priceB: -125, wantVig: 250.0/225.0 - 1},
			{name: "even money zero vig", priceA: 100, priceB: 100, wantVig: 0},
			{name: "slight under-round", priceA: 105, priceB: 105, wantVig: 200.0/205.0 - 1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				probA, probB, vig, ok := parlay.DevigTwoWayPower(tt.priceA, tt.priceB)
				require.True(t, ok)
				assert.InDelta(t, 0.5, probA, 1e-12)
				assert.InDelta(t, 0.5, probB, 1e-12)
				assert.InDelta(t, 1.0, probA+probB, 1e-9)
				assert.InDelta(t, tt.wantVig, vig, 1e-12)
			})
		}
	})

	t.Run("favorite-longshot allocates more vig to longshot than multiplicative", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			priceA int
			priceB int
		}{
			{name: "heavy favorite minus 400 plus 300", priceA: -400, priceB: 300},
			{name: "moderate favorite minus 150 plus 130", priceA: -150, priceB: 130},
			{name: "extreme favorite minus 1500 plus 900", priceA: -1500, priceB: 900},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				probA, probB, vig, ok := parlay.DevigTwoWayPower(tt.priceA, tt.priceB)
				require.True(t, ok)
				assert.InDelta(t, 1.0, probA+probB, 1e-9)
				assert.Greater(t, vig, 0.0)

				// Verify power exponent consistency: log(probA)/log(pA) == log(probB)/log(pB).
				kA := math.Log(probA) / math.Log(parlay.ImpliedProb(tt.priceA))
				kB := math.Log(probB) / math.Log(parlay.ImpliedProb(tt.priceB))
				assert.InDelta(t, kA, kB, 1e-9)
				assert.Greater(t, kA, 1.0)

				multA, multB, multOK := parlay.DevigTwoWay(tt.priceA, tt.priceB)
				require.True(t, multOK)
				assert.Greater(t, probA, multA, "favorite should have higher fair prob than multiplicative")
				assert.Less(t, probB, multB, "longshot should have lower fair prob than multiplicative")
			})
		}
	})

	t.Run("asymmetric slight under-round", func(t *testing.T) {
		t.Parallel()

		probA, probB, vig, ok := parlay.DevigTwoWayPower(-150, 180)
		require.True(t, ok)
		assert.InDelta(t, 1.0, probA+probB, 1e-9)
		assert.Less(t, vig, 0.0)
	})

	t.Run("invalid prices and overrounds", func(t *testing.T) {
		t.Parallel()

		invalid := []struct {
			name   string
			priceA int
			priceB int
		}{
			{name: "zero side A", priceA: 0, priceB: -110},
			{name: "zero side B", priceA: -110, priceB: 0},
			{name: "under-round below 0.95", priceA: 200, priceB: 200},
			{name: "over-round above 1.25", priceA: -300, priceB: -300},
		}

		for _, tt := range invalid {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				probA, probB, vig, ok := parlay.DevigTwoWayPower(tt.priceA, tt.priceB)
				assert.False(t, ok)
				assert.Zero(t, probA)
				assert.Zero(t, probB)
				assert.Zero(t, vig)
			})
		}
	})
}

func TestDevigTwoWayShin(t *testing.T) {
	t.Parallel()

	t.Run("symmetric and zero-vig lines", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			priceA  int
			priceB  int
			wantVig float64
		}{
			{name: "standard minus 110", priceA: -110, priceB: -110, wantVig: 10.0 / 210.0},
			{name: "high vig minus 125", priceA: -125, priceB: -125, wantVig: 250.0/225.0 - 1},
			{name: "even money zero vig", priceA: 100, priceB: 100, wantVig: 0},
			{name: "slight under-round", priceA: 105, priceB: 105, wantVig: 200.0/205.0 - 1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				probA, probB, vig, ok := parlay.DevigTwoWayShin(tt.priceA, tt.priceB)
				require.True(t, ok)
				assert.InDelta(t, 0.5, probA, 1e-12)
				assert.InDelta(t, 0.5, probB, 1e-12)
				assert.InDelta(t, 1.0, probA+probB, 1e-9)
				assert.InDelta(t, tt.wantVig, vig, 1e-12)
			})
		}
	})

	t.Run("favorite-longshot adjusts for insider bias", func(t *testing.T) {
		t.Parallel()

		probA, probB, vig, ok := parlay.DevigTwoWayShin(-400, 300)
		require.True(t, ok)
		assert.InDelta(t, 1.0, probA+probB, 1e-9)
		assert.InDelta(t, 0.05, vig, 1e-12)
		assert.InDelta(t, 0.775, probA, 1e-9)
		assert.InDelta(t, 0.225, probB, 1e-9)

		multA, multB, multOK := parlay.DevigTwoWay(-400, 300)
		require.True(t, multOK)
		assert.Greater(t, probA, multA)
		assert.Less(t, probB, multB)

		probAsymA, probAsymB, _, okAsym := parlay.DevigTwoWayShin(-150, 130)
		require.True(t, okAsym)
		assert.InDelta(t, 1.0, probAsymA+probAsymB, 1e-9)
		multAsymA, multAsymB, _ := parlay.DevigTwoWay(-150, 130)
		assert.Greater(t, probAsymA, multAsymA)
		assert.Less(t, probAsymB, multAsymB)
	})

	t.Run("invalid prices and overrounds", func(t *testing.T) {
		t.Parallel()

		invalid := []struct {
			name   string
			priceA int
			priceB int
		}{
			{name: "zero side A", priceA: 0, priceB: -110},
			{name: "zero side B", priceA: -110, priceB: 0},
			{name: "under-round below 0.95", priceA: 200, priceB: 200},
			{name: "over-round above 1.25", priceA: -300, priceB: -300},
		}

		for _, tt := range invalid {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				probA, probB, vig, ok := parlay.DevigTwoWayShin(tt.priceA, tt.priceB)
				assert.False(t, ok)
				assert.Zero(t, probA)
				assert.Zero(t, probB)
				assert.Zero(t, vig)
			})
		}
	})
}

func TestFairEV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		fairProb       float64
		retailAmerican int
		want           float64
	}{
		{
			name:           "even money zero EV",
			fairProb:       0.5,
			retailAmerican: 100,
			want:           0,
		},
		{
			name:           "standard minus 110 at 50 percent fair",
			fairProb:       0.5,
			retailAmerican: -110,
			want:           -5.0 / 110.0,
		},
		{
			name:           "positive EV at minus 110",
			fairProb:       0.55,
			retailAmerican: -110,
			want:           0.05,
		},
		{
			name:           "positive EV longshot plus 350",
			fairProb:       0.25,
			retailAmerican: 350,
			want:           0.125,
		},
		{
			name:           "invalid zero retail price",
			fairProb:       0.5,
			retailAmerican: 0,
			want:           0,
		},
		{
			name:           "invalid fair prob zero",
			fairProb:       0,
			retailAmerican: -110,
			want:           0,
		},
		{
			name:           "invalid fair prob one",
			fairProb:       1,
			retailAmerican: -110,
			want:           0,
		},
		{
			name:           "invalid fair prob negative",
			fairProb:       -0.1,
			retailAmerican: -110,
			want:           0,
		},
		{
			name:           "invalid fair prob above one",
			fairProb:       1.1,
			retailAmerican: -110,
			want:           0,
		},
		{
			name:           "invalid fair prob NaN",
			fairProb:       math.NaN(),
			retailAmerican: -110,
			want:           0,
		},
		{
			name:           "invalid fair prob Inf",
			fairProb:       math.Inf(1),
			retailAmerican: -110,
			want:           0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, parlay.FairEV(tt.fairProb, tt.retailAmerican), 1e-12)
		})
	}
}
