package modelv2_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

func TestFlatTreeEval(t *testing.T) {
	t.Parallel()

	// Tree:
	//   if f[0] < 10.0:
	//     if f[1] < 0.5 -> 1.25 else -> 2.50 (missing f[1] -> 1.75)
	//   else:
	//     3.75
	//   missing f[0] -> 0.50
	tree := modelv2.FlatTree{
		Nodes: []modelv2.TreeNode{
			{Feature: 0, Threshold: 10.0, Left: 1, Right: 2, Missing: 3},
			{Feature: 1, Threshold: 0.5, Left: 4, Right: 5, Missing: 6},
			{Feature: -1, Leaf: 3.75},
			{Feature: -1, Leaf: 0.50},
			{Feature: -1, Leaf: 1.25},
			{Feature: -1, Leaf: 2.50},
			{Feature: -1, Leaf: 1.75},
		},
	}
	require.NoError(t, tree.Validate(2))

	tests := []struct {
		name     string
		features []float64
		want     float64
	}{
		{name: "left-left", features: []float64{5.0, 0.2}, want: 1.25},
		{name: "left-right exact threshold", features: []float64{9.9, 0.5}, want: 2.50},
		{name: "right exact root threshold", features: []float64{10.0, 0.1}, want: 3.75},
		{name: "missing root NaN", features: []float64{math.NaN(), 0.2}, want: 0.50},
		{name: "missing inner NaN", features: []float64{4.0, math.NaN()}, want: 1.75},
		{name: "missing truncated slice", features: []float64{4.0}, want: 1.75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, tree.Eval(tt.features), 1e-12)
		})
	}
}

func TestGBDTModelEvalAndEmbeddedBaseline(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	pm, ok := bundle.PropModel(modelv2.MarketPassYds)
	require.True(t, ok)

	muModel := pm.ParamModels["mu"]

	lowVals := map[string]float64{
		modelv2.FeatEWMAPassYds:      195.0,
		modelv2.FeatTeamImpliedTotal: 19.5,
		modelv2.FeatWindMPH:          22.0,
	}
	highVals := map[string]float64{
		modelv2.FeatEWMAPassYds:      285.0,
		modelv2.FeatTeamImpliedTotal: 27.5,
		modelv2.FeatWindMPH:          3.0,
	}

	lowEta := muModel.Eval(modelv2.AlignFeatures(pm.FeatureNames, lowVals))
	highEta := muModel.Eval(modelv2.AlignFeatures(pm.FeatureNames, highVals))

	assert.Greater(t, highEta, lowEta)
	baseMean := math.Exp(muModel.BaseScore)
	assert.Greater(t, baseMean, 180.0)
	assert.Less(t, baseMean, 260.0)
}

func TestGBDTValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		model       modelv2.GBDTModel
		numFeatures int
	}{
		{
			name:        "nan base_score",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				BaseScore: math.NaN(),
				Trees:     []modelv2.FlatTree{{Nodes: []modelv2.TreeNode{{Feature: -1, Leaf: 1}}}},
			},
		},
		{
			name:        "empty trees",
			numFeatures: 2,
			model:       modelv2.GBDTModel{BaseScore: 0},
		},
		{
			name:        "empty tree nodes",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				Trees: []modelv2.FlatTree{{}},
			},
		},
		{
			name:        "feature index out of bounds",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				Trees: []modelv2.FlatTree{{
					Nodes: []modelv2.TreeNode{
						{Feature: 2, Threshold: 1, Left: 1, Right: 1, Missing: 1},
						{Feature: -1, Leaf: 0},
					},
				}},
			},
		},
		{
			name:        "invalid negative feature index",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				Trees: []modelv2.FlatTree{{
					Nodes: []modelv2.TreeNode{{Feature: -2, Leaf: 0}},
				}},
			},
		},
		{
			name:        "nan leaf",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				Trees: []modelv2.FlatTree{{
					Nodes: []modelv2.TreeNode{{Feature: -1, Leaf: math.NaN()}},
				}},
			},
		},
		{
			name:        "child index out of bounds",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				Trees: []modelv2.FlatTree{{
					Nodes: []modelv2.TreeNode{
						{Feature: 0, Threshold: 1, Left: 1, Right: 5, Missing: 1},
						{Feature: -1, Leaf: 0},
					},
				}},
			},
		},
		{
			name:        "cycle in tree nodes",
			numFeatures: 2,
			model: modelv2.GBDTModel{
				Trees: []modelv2.FlatTree{{
					Nodes: []modelv2.TreeNode{
						{Feature: 0, Threshold: 1, Left: 1, Right: 2, Missing: 2},
						{Feature: 1, Threshold: 2, Left: 0, Right: 2, Missing: 2},
						{Feature: -1, Leaf: 0},
					},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, tt.model.Validate(tt.numFeatures))
		})
	}
}

//nolint:paralleltest // testing.AllocsPerRun cannot run concurrently with allocating tests
func TestGBDTModelEvalZeroAllocs(t *testing.T) {
	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	pm, ok := bundle.PropModel(modelv2.MarketPassYds)
	require.True(t, ok)
	muModel := pm.ParamModels["mu"]

	feats := modelv2.AlignFeatures(pm.FeatureNames, map[string]float64{
		modelv2.FeatEWMAPassYds:      255.0,
		modelv2.FeatTeamImpliedTotal: 25.0,
		modelv2.FeatWindMPH:          8.0,
	})

	allocs := testing.AllocsPerRun(100, func() {
		_ = muModel.Eval(feats)
	})
	assert.Zero(t, allocs)
}

func BenchmarkGBDTModelEval(b *testing.B) {
	// Simulate a realistic 100-tree depth-4 ensemble (15 nodes per tree).
	trees := make([]modelv2.FlatTree, 100)
	for i := range trees {
		trees[i] = modelv2.FlatTree{
			Nodes: []modelv2.TreeNode{
				{Feature: i % 8, Threshold: 10.0, Left: 1, Right: 2, Missing: 1},
				{Feature: (i + 1) % 8, Threshold: 5.0, Left: 3, Right: 4, Missing: 3},
				{Feature: (i + 2) % 8, Threshold: 15.0, Left: 5, Right: 6, Missing: 6},
				{Feature: -1, Leaf: -0.02},
				{Feature: -1, Leaf: 0.01},
				{Feature: -1, Leaf: 0.02},
				{Feature: -1, Leaf: 0.05},
			},
		}
	}
	model := modelv2.GBDTModel{BaseScore: 4.1, Trees: trees}
	feats := []float64{12.0, 4.5, 16.0, 24.0, -3.5, 0.25, 65.0, 8.0}

	b.ReportAllocs()
	for b.Loop() {
		_ = model.Eval(feats)
	}
}
