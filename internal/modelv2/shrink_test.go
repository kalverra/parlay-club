package modelv2_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

func TestEmpiricalBayesShrink(t *testing.T) {
	t.Parallel()

	const (
		obs   = 100.0
		prior = 60.0
		tau   = 3.0
	)

	tests := []struct {
		name        string
		observed    float64
		gamesPlayed int
		strength    float64
		want        float64
	}{
		{name: "week 1 zero games returns prior", observed: obs, gamesPlayed: 0, strength: tau, want: prior},
		{name: "week 2 one game", observed: obs, gamesPlayed: 1, strength: tau, want: (1*obs + 3*prior) / 4},
		{name: "week 3 two games", observed: obs, gamesPlayed: 2, strength: tau, want: (2*obs + 3*prior) / 5},
		{name: "week 4 three games", observed: obs, gamesPlayed: 3, strength: tau, want: (3*obs + 3*prior) / 6},
		{name: "week 5 four games", observed: obs, gamesPlayed: 4, strength: tau, want: (4*obs + 3*prior) / 7},
		{name: "week 6 five games unshrunk", observed: obs, gamesPlayed: 5, strength: tau, want: obs},
		{name: "week 10 nine games unshrunk", observed: obs, gamesPlayed: 9, strength: tau, want: obs},
		{name: "nan observed falls back to prior", observed: math.NaN(), gamesPlayed: 3, strength: tau, want: prior},
		{name: "zero strength unshrunk", observed: obs, gamesPlayed: 2, strength: 0, want: obs},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := modelv2.EmpiricalBayesShrink(tt.observed, prior, tt.gamesPlayed, tt.strength)
			assert.InDelta(t, tt.want, got, 1e-12)
		})
	}
}

func TestLookupPrior(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	p, ok := bundle.LookupPrior(modelv2.MarketRecYds, "WR1", "WR")
	require.True(t, ok)
	assert.Equal(t, "WR1", p.Role)
	assert.Greater(t, p.Mean, 60.0)

	// Week 1 unknown role falls back by position.
	pWR, ok := bundle.LookupPrior(modelv2.MarketRecYds, "", "WR")
	require.True(t, ok)
	assert.Equal(t, "WR2", pWR.Role)

	pRB, ok := bundle.LookupPrior(modelv2.MarketRushYds, "", "RB")
	require.True(t, ok)
	assert.Equal(t, "RB2", pRB.Role)

	pTE, ok := bundle.LookupPrior(modelv2.MarketReceptions, "", "TE")
	require.True(t, ok)
	assert.Equal(t, "TE1", pTE.Role)

	_, ok = bundle.LookupPrior("UNKNOWN_MARKET", "WR1", "WR")
	assert.False(t, ok)
}

func TestShrinkFeatureMapAndSlice(t *testing.T) {
	t.Parallel()

	prior := modelv2.RolePrior{
		Market:        modelv2.MarketRecYds,
		Role:          "WR1",
		PriorStrength: 3.0,
		Mean:          68.0,
		Features: map[string]float64{
			modelv2.FeatEWMARecYds:      68.0,
			modelv2.FeatEWMATargetShare: 0.24,
		},
	}
	raw := map[string]float64{
		modelv2.FeatEWMARecYds:       108.0,
		modelv2.FeatTeamImpliedTotal: 26.5,
	}

	// Week 3 (2 games played): rec_yds shrunk, missing target_share gets prior, non-prior feature untouched.
	shrunk := modelv2.ShrinkFeatureMap(raw, 2, prior)
	assert.InDelta(t, (2*108.0+3*68.0)/5.0, shrunk[modelv2.FeatEWMARecYds], 1e-12)
	assert.InDelta(t, 0.24, shrunk[modelv2.FeatEWMATargetShare], 1e-12)
	assert.InDelta(t, 26.5, shrunk[modelv2.FeatTeamImpliedTotal], 1e-12)
	// Original map is not mutated.
	assert.NotContains(t, raw, modelv2.FeatEWMATargetShare)

	// Slice path matches map path.
	names := []string{modelv2.FeatEWMARecYds, modelv2.FeatEWMATargetShare, modelv2.FeatTeamImpliedTotal}
	aligned := modelv2.AlignFeatures(names, raw)
	require.Len(t, aligned, 3)
	assert.True(t, math.IsNaN(aligned[1]))

	dst := make([]float64, len(names))
	dst = modelv2.ShrinkFeatureSlice(dst, aligned, names, 2, prior)
	assert.InDelta(t, shrunk[modelv2.FeatEWMARecYds], dst[0], 1e-12)
	assert.InDelta(t, shrunk[modelv2.FeatEWMATargetShare], dst[1], 1e-12)
	assert.InDelta(t, shrunk[modelv2.FeatTeamImpliedTotal], dst[2], 1e-12)

	// Week 6+ (5+ games played): observed values remain unshrunk and missing stays NaN.
	late := modelv2.ShrinkFeatureSlice(dst, aligned, names, 5, prior)
	assert.InDelta(t, 108.0, late[0], 1e-12)
	assert.True(t, math.IsNaN(late[1]))
}

func TestRolePriorValidate(t *testing.T) {
	t.Parallel()

	valid := modelv2.RolePrior{
		Market:        modelv2.MarketPassYds,
		Role:          "QB",
		PriorStrength: 3.0,
		Mean:          235.0,
		Std:           60.0,
		Features:      map[string]float64{modelv2.FeatEWMAPassYds: 235.0},
	}
	require.NoError(t, valid.Validate("PASS_YDS:QB"))

	badKey := valid
	require.Error(t, badKey.Validate("PASS_YDS:WR1"))

	negStrength := valid
	negStrength.PriorStrength = -1
	require.Error(t, negStrength.Validate("PASS_YDS:QB"))

	nanFeat := valid
	nanFeat.Features = map[string]float64{modelv2.FeatEWMAPassYds: math.NaN()}
	require.Error(t, nanFeat.Validate("PASS_YDS:QB"))
}

func BenchmarkShrinkFeatureSlice(b *testing.B) {
	prior := modelv2.RolePrior{
		Market:        modelv2.MarketRecYds,
		Role:          "WR1",
		PriorStrength: 3.0,
		Mean:          68.0,
		Features: map[string]float64{
			modelv2.FeatEWMARecYds:      68.0,
			modelv2.FeatEWMATargetShare: 0.24,
			modelv2.FeatEWMAExpRecYds:   66.0,
		},
	}
	names := []string{
		modelv2.FeatEWMARecYds,
		modelv2.FeatEWMAExpRecYds,
		modelv2.FeatEWMATargetShare,
		modelv2.FeatTeamImpliedTotal,
		modelv2.FeatOppRoleYdsAllowed,
	}
	obs := []float64{92.0, 84.0, 0.28, 25.5, 70.0}
	dst := make([]float64, len(names))

	b.ReportAllocs()
	for b.Loop() {
		dst = modelv2.ShrinkFeatureSlice(dst, obs, names, 2, prior)
	}
}
