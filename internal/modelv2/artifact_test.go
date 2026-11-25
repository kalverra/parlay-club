package modelv2_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

func TestLoadEmbedded(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)
	require.NotNil(t, bundle)

	assert.Equal(t, modelv2.CurrentFeatureSchemaVersion, bundle.FeatureSchemaVersion)
	assert.NotEmpty(t, bundle.Version)
	assert.NotEmpty(t, bundle.TrainedSeasons)

	wantMarkets := []string{
		modelv2.MarketAnytimeTD,
		modelv2.MarketPassTDs,
		modelv2.MarketPassYds,
		modelv2.MarketReceptions,
		modelv2.MarketRecYds,
		modelv2.MarketRushAtt,
		modelv2.MarketRushYds,
	}
	assert.Equal(t, wantMarkets, bundle.SortedPropMarkets())

	for _, market := range wantMarkets {
		pm, ok := bundle.PropModel(market)
		require.True(t, ok, "missing prop model %s", market)
		assert.Equal(t, market, pm.Market)
		assert.NotEmpty(t, pm.FeatureNames)
		assert.NotEmpty(t, pm.ParamModels)
	}

	assert.Contains(t, bundle.Copula.RegimeMatrices, modelv2.RegimeNeutral)
	assert.Contains(t, bundle.Copula.RegimeMatrices, modelv2.RegimeHeavyFavorite)
	assert.Contains(t, bundle.Copula.RegimeMatrices, modelv2.RegimeShootout)

	again, err := modelv2.LoadEmbedded()
	require.NoError(t, err)
	assert.Same(t, bundle, again)
}

func TestLoadEmbeddedPathError(t *testing.T) {
	t.Parallel()

	_, err := modelv2.LoadEmbeddedPath("artifacts/does_not_exist.json")
	require.Error(t, err)
}

func TestParseArtifactBundleValidation(t *testing.T) {
	t.Parallel()

	base, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	cloneBundle := func(t *testing.T) *modelv2.ArtifactBundle {
		t.Helper()
		raw, err := json.Marshal(base)
		require.NoError(t, err)
		var out modelv2.ArtifactBundle
		require.NoError(t, json.Unmarshal(raw, &out))
		return &out
	}

	t.Run("empty payload and bad json", func(t *testing.T) {
		t.Parallel()

		_, err := modelv2.ParseArtifactBundle(nil)
		require.Error(t, err)

		_, err = modelv2.ParseArtifactBundle([]byte("{not-json"))
		require.Error(t, err)
	})

	t.Run("nil bundle", func(t *testing.T) {
		t.Parallel()

		var b *modelv2.ArtifactBundle
		require.Error(t, b.Validate())
	})

	tests := []struct {
		name   string
		mutate func(*modelv2.ArtifactBundle)
	}{
		{
			name:   "empty version",
			mutate: func(b *modelv2.ArtifactBundle) { b.Version = "" },
		},
		{
			name:   "wrong feature schema version",
			mutate: func(b *modelv2.ArtifactBundle) { b.FeatureSchemaVersion = 999 },
		},
		{
			name:   "empty trained seasons",
			mutate: func(b *modelv2.ArtifactBundle) { b.TrainedSeasons = nil },
		},
		{
			name:   "out of range trained season",
			mutate: func(b *modelv2.ArtifactBundle) { b.TrainedSeasons = []int{1980} },
		},
		{
			name:   "empty built_at",
			mutate: func(b *modelv2.ArtifactBundle) { b.BuiltAt = "" },
		},
		{
			name:   "invalid built_at timestamp",
			mutate: func(b *modelv2.ArtifactBundle) { b.BuiltAt = "not-a-timestamp" },
		},
		{
			name:   "empty prop_models",
			mutate: func(b *modelv2.ArtifactBundle) { b.PropModels = nil },
		},
		{
			name: "mismatched prop model key",
			mutate: func(b *modelv2.ArtifactBundle) {
				pm := b.PropModels[modelv2.MarketPassYds]
				pm.Market = modelv2.MarketRushYds
				b.PropModels[modelv2.MarketPassYds] = pm
			},
		},
		{
			name: "unsupported family",
			mutate: func(b *modelv2.ArtifactBundle) {
				pm := b.PropModels[modelv2.MarketPassYds]
				pm.Family = "weibull"
				b.PropModels[modelv2.MarketPassYds] = pm
			},
		},
		{
			name: "duplicate feature name",
			mutate: func(b *modelv2.ArtifactBundle) {
				pm := b.PropModels[modelv2.MarketPassYds]
				pm.FeatureNames = []string{modelv2.FeatEWMAPassYds, modelv2.FeatEWMAPassYds}
				b.PropModels[modelv2.MarketPassYds] = pm
			},
		},
		{
			name: "blend weight out of bounds",
			mutate: func(b *modelv2.ArtifactBundle) {
				pm := b.PropModels[modelv2.MarketPassYds]
				pm.MarketBlendWeight = 1.2
				b.PropModels[modelv2.MarketPassYds] = pm
			},
		},
		{
			name: "missing required param model",
			mutate: func(b *modelv2.ArtifactBundle) {
				pm := b.PropModels[modelv2.MarketPassYds]
				delete(pm.ParamModels, "k")
				b.PropModels[modelv2.MarketPassYds] = pm
			},
		},
		{
			name:   "empty priors",
			mutate: func(b *modelv2.ArtifactBundle) { b.Priors = nil },
		},
		{
			name: "mismatched prior key",
			mutate: func(b *modelv2.ArtifactBundle) {
				p := b.Priors["PASS_YDS:QB"]
				p.Role = "WR1"
				b.Priors["PASS_YDS:QB"] = p
			},
		},
		{
			name: "copula too few nodes",
			mutate: func(b *modelv2.ArtifactBundle) {
				b.Copula.Nodes = []string{"TEAM_MARGIN"}
			},
		},
		{
			name: "copula duplicate node",
			mutate: func(b *modelv2.ArtifactBundle) {
				b.Copula.Nodes[1] = b.Copula.Nodes[0]
			},
		},
		{
			name: "copula non-symmetric default matrix",
			mutate: func(b *modelv2.ArtifactBundle) {
				b.Copula.DefaultMatrix[0][1] = 0.25
				b.Copula.DefaultMatrix[1][0] = -0.25
			},
		},
		{
			name: "copula non-unit diagonal",
			mutate: func(b *modelv2.ArtifactBundle) {
				b.Copula.DefaultMatrix[0][0] = 0.95
			},
		},
		{
			name: "copula non-psd regime matrix",
			mutate: func(b *modelv2.ArtifactBundle) {
				n := len(b.Copula.Nodes)
				mat := make([][]float64, n)
				for i := range n {
					mat[i] = make([]float64, n)
					mat[i][i] = 1.0
				}
				// Construct impossible 3x3 correlation triple: (0,1)=0.9, (0,2)=0.9, (1,2)=-0.9
				mat[0][1], mat[1][0] = 0.9, 0.9
				mat[0][2], mat[2][0] = 0.9, 0.9
				mat[1][2], mat[2][1] = -0.9, -0.9
				b.Copula.RegimeMatrices[modelv2.RegimeNeutral] = mat
			},
		},
		{
			name: "copula within_player out of bounds",
			mutate: func(b *modelv2.ArtifactBundle) {
				b.Copula.WithinPlayer["RECEPTIONS:REC_YDS"] = 1.05
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := cloneBundle(t)
			tt.mutate(b)
			require.Error(t, b.Validate())
		})
	}
}

func TestValidateCorrMatrix(t *testing.T) {
	t.Parallel()

	t.Run("valid 2x2 and rank-deficient PSD", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, modelv2.ValidateCorrMatrix([][]float64{
			{1, 0.5},
			{0.5, 1},
		}, 2))

		// Singular but valid PSD (rank 1).
		require.NoError(t, modelv2.ValidateCorrMatrix([][]float64{
			{1, 1},
			{1, 1},
		}, 2))
	})

	t.Run("rejects non-PSD and NaN", func(t *testing.T) {
		t.Parallel()

		require.Error(t, modelv2.ValidateCorrMatrix([][]float64{
			{1, 0.9, 0.9},
			{0.9, 1, -0.9},
			{0.9, -0.9, 1},
		}, 3))

		require.Error(t, modelv2.ValidateCorrMatrix([][]float64{
			{1, math.NaN()},
			{math.NaN(), 1},
		}, 2))
	})
}

func BenchmarkLoadEmbedded(b *testing.B) {
	for b.Loop() {
		bundle, err := modelv2.LoadEmbedded()
		if err != nil || bundle == nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkParseArtifactBundle(b *testing.B) {
	bundle, err := modelv2.LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		parsed, err := modelv2.ParseArtifactBundle(raw)
		if err != nil || parsed == nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
