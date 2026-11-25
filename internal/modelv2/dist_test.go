package modelv2_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

func TestZeroAdjGammaAnalyticalAndEdgeThresholds(t *testing.T) {
	t.Parallel()

	// Exponential special case: Pi0 = 0, K = 1 => S(t) = exp(-t / Mu).
	expDist := modelv2.ZeroAdjGamma{Pi0: 0, Mu: 200.0, K: 1.0}
	require.NoError(t, expDist.Validate())
	assert.Equal(t, modelv2.FamilyZeroAdjGamma, expDist.Family())

	assert.InDelta(t, 1.0, expDist.Survival(-1.0), 1e-12)
	assert.InDelta(t, 1.0, expDist.Survival(0.0), 1e-12)
	assert.InDelta(t, math.Exp(-0.5/200.0), expDist.Survival(0.5), 1e-12)
	assert.InDelta(t, 0.5, expDist.Survival(200.0*math.Ln2), 1e-12)
	assert.InDelta(t, 0.1, expDist.Survival(-200.0*math.Log(0.1)), 1e-12)
	assert.InDelta(t, 0.01, expDist.Survival(-200.0*math.Log(0.01)), 1e-12)
	assert.InDelta(t, 0.0, expDist.Survival(1e6), 1e-12)

	expStats := expDist.Stats()
	assert.InDelta(t, 200.0, expStats.Mean, 1e-12)
	assert.InDelta(t, 40000.0, expStats.Variance, 1e-9)
	assert.InDelta(t, 200.0, expStats.StdDev, 1e-12)
	assert.InDelta(t, 1.0, expStats.CV, 1e-12)
	assert.InDelta(t, 2.0, expStats.Skewness, 1e-12)

	// Erlang-3 special case: K = 3, rate = 3/Mu.
	// Covers both series branch (x < 4) and continued fraction branch (x >= 4).
	erlang3 := modelv2.ZeroAdjGamma{Pi0: 0, Mu: 60.0, K: 3.0}
	require.NoError(t, erlang3.Validate())
	for _, thresh := range []float64{0.5, 25.0, 60.0, 80.0, 120.0, 240.0} {
		x := thresh * 3.0 / 60.0
		want := math.Exp(-x) * (1.0 + x + 0.5*x*x)
		assert.InDelta(t, want, erlang3.Survival(thresh), 1e-12, "threshold=%v", thresh)
	}
	e3Stats := erlang3.Stats()
	assert.InDelta(t, 60.0, e3Stats.Mean, 1e-12)
	assert.InDelta(t, 1200.0, e3Stats.Variance, 1e-10)
	assert.InDelta(t, 1.0/math.Sqrt(3.0), e3Stats.CV, 1e-12)
	assert.InDelta(t, 2.0/math.Sqrt(3.0), e3Stats.Skewness, 1e-12)

	// Half-integer shape K = 0.5: Q(0.5, x) = erfc(sqrt(x)).
	chiSq := modelv2.ZeroAdjGamma{Pi0: 0, Mu: 40.0, K: 0.5}
	require.NoError(t, chiSq.Validate())
	for _, thresh := range []float64{0.5, 18.19725, 40.0, 150.0, 265.3958} {
		want := math.Erfc(math.Sqrt(thresh / (2.0 * 40.0)))
		assert.InDelta(t, want, chiSq.Survival(thresh), 1e-12, "threshold=%v", thresh)
	}

	// Zero-inflated Gamma mixture moments cross-checked against raw moments E[Y], E[Y^2], E[Y^3].
	zaGamma := modelv2.ZeroAdjGamma{Pi0: 0.15, Mu: 72.0, K: 3.5}
	require.NoError(t, zaGamma.Validate())
	assert.InDelta(t, 1.0, zaGamma.Survival(-0.5), 1e-12)
	assert.InDelta(t, 0.85, zaGamma.Survival(0.0), 1e-12)
	assert.InDelta(t, 0.85*modelv2.ZeroAdjGamma{Pi0: 0, Mu: 72.0, K: 3.5}.Survival(65.5), zaGamma.Survival(65.5), 1e-12)

	active := 1.0 - zaGamma.Pi0
	ex1 := zaGamma.Mu
	ex2 := zaGamma.Mu * zaGamma.Mu * (1.0 + 1.0/zaGamma.K)
	ex3 := zaGamma.Mu * zaGamma.Mu * zaGamma.Mu * (1.0 + 1.0/zaGamma.K) * (1.0 + 2.0/zaGamma.K)
	ey1 := active * ex1
	ey2 := active * ex2
	ey3 := active * ex3
	wantVar := ey2 - ey1*ey1
	wantSkew := (ey3 - 3.0*ey1*ey2 + 2.0*ey1*ey1*ey1) / math.Pow(wantVar, 1.5)

	zaStats := zaGamma.Stats()
	assert.InDelta(t, ey1, zaStats.Mean, 1e-11)
	assert.InDelta(t, wantVar, zaStats.Variance, 1e-9)
	assert.InDelta(t, math.Sqrt(wantVar)/ey1, zaStats.CV, 1e-12)
	assert.InDelta(t, wantSkew, zaStats.Skewness, 1e-11)
}

func TestZeroAdjLogNormalAnalyticalQuantilesAndMoments(t *testing.T) {
	t.Parallel()

	ln := modelv2.ZeroAdjLogNormal{Pi0: 0, Mu: 4.0, Sigma: 0.6}
	require.NoError(t, ln.Validate())
	assert.Equal(t, modelv2.FamilyZeroAdjLogNormal, ln.Family())

	// Analytical normal quantiles for 25th, 50th, 75th, 99th percentiles.
	const (
		z25 = -0.6744897501960817
		z75 = 0.6744897501960817
		z99 = 2.3263478740408408
	)
	assert.InDelta(t, 1.0, ln.Survival(-1.0), 1e-12)
	assert.InDelta(t, 1.0, ln.Survival(0.0), 1e-12)
	assert.InDelta(t, 0.75, ln.Survival(math.Exp(4.0+0.6*z25)), 1e-12)
	assert.InDelta(t, 0.50, ln.Survival(math.Exp(4.0)), 1e-12)
	assert.InDelta(t, 0.25, ln.Survival(math.Exp(4.0+0.6*z75)), 1e-12)
	assert.InDelta(t, 0.01, ln.Survival(math.Exp(4.0+0.6*z99)), 1e-12)
	assert.InDelta(t, 0.0, ln.Survival(1e9), 1e-12)

	w := math.Exp(0.6 * 0.6)
	wantMean := math.Exp(4.0 + 0.5*0.6*0.6)
	wantVar := (w - 1.0) * math.Exp(2.0*4.0+0.6*0.6)
	wantCV := math.Sqrt(w - 1.0)
	wantSkew := (w + 2.0) * math.Sqrt(w-1.0)
	stats := ln.Stats()
	assert.InDelta(t, wantMean, stats.Mean, 1e-11)
	assert.InDelta(t, wantVar, stats.Variance, 1e-9)
	assert.InDelta(t, wantCV, stats.CV, 1e-12)
	assert.InDelta(t, wantSkew, stats.Skewness, 1e-12)

	// Zero-inflated LogNormal mixture check + small-sigma numerical stability.
	zaLN := modelv2.ZeroAdjLogNormal{Pi0: 0.20, Mu: 3.5, Sigma: 0.5}
	require.NoError(t, zaLN.Validate())
	assert.InDelta(t, 0.80, zaLN.Survival(0.0), 1e-12)
	assert.InDelta(t, 0.40, zaLN.Survival(math.Exp(3.5)), 1e-12)

	active := 1.0 - zaLN.Pi0
	s2 := zaLN.Sigma * zaLN.Sigma
	ex1 := math.Exp(zaLN.Mu + 0.5*s2)
	ex2 := math.Exp(2.0*zaLN.Mu + 2.0*s2)
	ex3 := math.Exp(3.0*zaLN.Mu + 4.5*s2)
	ey1 := active * ex1
	ey2 := active * ex2
	ey3 := active * ex3
	zaVar := ey2 - ey1*ey1
	zaSkew := (ey3 - 3.0*ey1*ey2 + 2.0*ey1*ey1*ey1) / math.Pow(zaVar, 1.5)

	zaStats := zaLN.Stats()
	assert.InDelta(t, ey1, zaStats.Mean, 1e-11)
	assert.InDelta(t, zaVar, zaStats.Variance, 1e-9)
	assert.InDelta(t, zaSkew, zaStats.Skewness, 1e-11)

	tinySigma := modelv2.ZeroAdjLogNormal{Pi0: 0, Mu: 4.0, Sigma: 1e-5}
	tinyStats := tinySigma.Stats()
	assert.InDelta(t, 1e-5, tinyStats.CV, 1e-9)
	assert.InDelta(t, 3e-5, tinyStats.Skewness, 1e-8)
}

func TestNegBinAnalyticalAndTailAccuracy(t *testing.T) {
	t.Parallel()

	// R = 1 (Geometric distribution): P(Y > k) = (Mu / (1 + Mu))^(k + 1).
	geom := modelv2.NegBin{Mu: 4.5, R: 1.0}
	require.NoError(t, geom.Validate())
	assert.Equal(t, modelv2.FamilyNegBin, geom.Family())
	qGeom := 4.5 / 5.5
	for _, k := range []int{0, 1, 5, 20, 120, 300} {
		want := math.Pow(qGeom, float64(k+1))
		assert.InDelta(t, want, geom.Survival(float64(k)+0.5), 1e-12, "k=%d", k)
		assert.InDelta(t, want, geom.Survival(float64(k)), 1e-12, "integer k=%d", k)
	}
	assert.InDelta(t, 1.0, geom.Survival(-0.1), 1e-12)

	// R = 2: P(Y > k) = q^(k+1) * (1 + (k+1)*(1-q)).
	nb2 := modelv2.NegBin{Mu: 5.0, R: 2.0}
	q2 := 5.0 / 7.0
	p2 := 2.0 / 7.0
	for _, k := range []int{0, 2, 7, 35} {
		want := math.Pow(q2, float64(k+1)) * (1.0 + float64(k+1)*p2)
		assert.InDelta(t, want, nb2.Survival(float64(k)+0.5), 1e-12, "k=%d", k)
	}

	// Fractional R cross-checked against log-space PMF sum and continuity across limit.
	frac := modelv2.NegBin{Mu: 5.3, R: 3.7}
	pmfSum := 0.0
	for j := range 200 {
		pmfSum += frac.PMF(j)
		assert.InDelta(t, 1.0-pmfSum, frac.Survival(float64(j)+0.5), 1e-12, "j=%d", j)
	}
	assert.InDelta(t, 1.0, pmfSum, 1e-12)
	assert.Less(t, frac.Survival(257.5), frac.Survival(256.5))
	assert.InDelta(t, 0.0, frac.Survival(1e9), 1e-12)

	stats := frac.Stats()
	wantVar := 5.3 * (1.0 + 5.3/3.7)
	assert.InDelta(t, 5.3, stats.Mean, 1e-12)
	assert.InDelta(t, wantVar, stats.Variance, 1e-12)
	assert.InDelta(t, math.Sqrt(1.0/5.3+1.0/3.7), stats.CV, 1e-12)
	assert.InDelta(t, (1.0+2.0*5.3/3.7)/math.Sqrt(wantVar), stats.Skewness, 1e-12)
}

func TestPoissonAnalyticalAndLimits(t *testing.T) {
	t.Parallel()

	pois := modelv2.Poisson{Lambda: 1.8}
	require.NoError(t, pois.Validate())
	assert.Equal(t, modelv2.FamilyPoisson, pois.Family())

	assert.InDelta(t, 1.0, pois.Survival(-0.5), 1e-12)
	assert.InDelta(t, 1.0-math.Exp(-1.8), pois.Survival(0.0), 1e-12)
	assert.InDelta(t, 1.0-math.Exp(-1.8), pois.Survival(0.5), 1e-12)
	assert.InDelta(t, 1.0-math.Exp(-1.8)*(1.0+1.8), pois.Survival(1.5), 1e-12)
	assert.InDelta(t, 1.0-math.Exp(-1.8)*(1.0+1.8+1.8*1.8/2.0), pois.Survival(2.5), 1e-12)

	pmfSum := 0.0
	for j := range 40 {
		pmfSum += pois.PMF(j)
		assert.InDelta(t, 1.0-pmfSum, pois.Survival(float64(j)+0.5), 1e-12, "j=%d", j)
	}
	assert.InDelta(t, 0.0, pois.Survival(1e6), 1e-12)

	// Large-R NegBin converges to Poisson.
	nbApprox := modelv2.NegBin{Mu: 1.8, R: 1e6}
	for _, thresh := range []float64{0.5, 1.5, 2.5, 4.5} {
		assert.InDelta(t, pois.Survival(thresh), nbApprox.Survival(thresh), 1e-5, "thresh=%v", thresh)
	}

	stats := pois.Stats()
	assert.InDelta(t, 1.8, stats.Mean, 1e-12)
	assert.InDelta(t, 1.8, stats.Variance, 1e-12)
	assert.InDelta(t, 1.0/math.Sqrt(1.8), stats.CV, 1e-12)
	assert.InDelta(t, 1.0/math.Sqrt(1.8), stats.Skewness, 1e-12)
}

func TestBernoulliAnalytical(t *testing.T) {
	t.Parallel()

	bern := modelv2.Bernoulli{P: 0.35}
	require.NoError(t, bern.Validate())
	assert.Equal(t, modelv2.FamilyBernoulli, bern.Family())

	assert.InDelta(t, 1.0, bern.Survival(-0.5), 1e-12)
	assert.InDelta(t, 0.35, bern.Survival(0.0), 1e-12)
	assert.InDelta(t, 0.35, bern.Survival(0.5), 1e-12)
	assert.InDelta(t, 0.35, bern.Survival(0.99), 1e-12)
	assert.InDelta(t, 0.0, bern.Survival(1.0), 1e-12)
	assert.InDelta(t, 0.0, bern.Survival(1.5), 1e-12)

	stats := bern.Stats()
	assert.InDelta(t, 0.35, stats.Mean, 1e-12)
	assert.InDelta(t, 0.35*0.65, stats.Variance, 1e-12)
	assert.InDelta(t, math.Sqrt(0.35*0.65), stats.StdDev, 1e-12)
	assert.InDelta(t, math.Sqrt(0.65/0.35), stats.CV, 1e-12)
	assert.InDelta(t, (1.0-2.0*0.35)/math.Sqrt(0.35*0.65), stats.Skewness, 1e-12)

	// Boundary P=0 and P=1 degenerate stats do not produce NaN.
	zeroStats := (modelv2.Bernoulli{P: 0}).Stats()
	assert.Zero(t, zeroStats.Variance)
	assert.Zero(t, zeroStats.Skewness)
	oneStats := (modelv2.Bernoulli{P: 1}).Stats()
	assert.Zero(t, oneStats.Variance)
	assert.Zero(t, oneStats.Skewness)
}

func TestDistValidationAndEdgeInputs(t *testing.T) {
	t.Parallel()

	invalid := []modelv2.Distribution{
		modelv2.ZeroAdjGamma{Pi0: -0.1, Mu: 10, K: 2},
		modelv2.ZeroAdjGamma{Pi0: 1.0, Mu: 10, K: 2},
		modelv2.ZeroAdjGamma{Pi0: 0.1, Mu: 0, K: 2},
		modelv2.ZeroAdjGamma{Pi0: 0.1, Mu: 10, K: -1},
		modelv2.ZeroAdjLogNormal{Pi0: 1.0, Mu: 3, Sigma: 0.5},
		modelv2.ZeroAdjLogNormal{Pi0: 0.1, Mu: math.NaN(), Sigma: 0.5},
		modelv2.ZeroAdjLogNormal{Pi0: 0.1, Mu: 3, Sigma: 0},
		modelv2.NegBin{Mu: 0, R: 2},
		modelv2.NegBin{Mu: 4, R: math.Inf(1)},
		modelv2.Poisson{Lambda: 0},
		modelv2.Bernoulli{P: -0.01},
		modelv2.Bernoulli{P: 1.01},
	}
	for _, dist := range invalid {
		require.Error(t, dist.Validate())
	}

	g := modelv2.ZeroAdjGamma{Pi0: 0.05, Mu: 50, K: 2}
	assert.InDelta(t, 0.0, g.Survival(math.NaN()), 1e-12)
	assert.InDelta(t, 0.0, g.Survival(math.Inf(1)), 1e-12)
	assert.InDelta(t, 1.0, g.Survival(math.Inf(-1)), 1e-12)
}

func TestEmbeddedBaselineDistAndPredictLeg(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	sampleLines := map[string]float64{
		modelv2.MarketPassYds:    245.5,
		modelv2.MarketRushYds:    62.5,
		modelv2.MarketRecYds:     54.5,
		modelv2.MarketReceptions: 4.5,
		modelv2.MarketRushAtt:    13.5,
		modelv2.MarketPassTDs:    1.5,
		modelv2.MarketAnytimeTD:  0.5,
	}

	for _, market := range bundle.SortedPropMarkets() {
		pm, ok := bundle.PropModel(market)
		require.True(t, ok)

		feats := modelv2.AlignFeatures(pm.FeatureNames, map[string]float64{
			modelv2.FeatEWMAPassYds:      260.0,
			modelv2.FeatEWMARushYds:      68.0,
			modelv2.FeatEWMARecYds:       58.0,
			modelv2.FeatEWMAReceptions:   4.8,
			modelv2.FeatEWMACarries:      15.0,
			modelv2.FeatEWMAPassTDs:      1.7,
			modelv2.FeatEWMARZOpps:       1.8,
			modelv2.FeatTeamImpliedTotal: 25.5,
		})

		line := sampleLines[market]
		overPred, err := pm.PredictLeg(feats, "OVER", line, 0.52, 0.0)
		require.NoError(t, err)
		assert.Equal(t, market, overPred.Market)
		assert.Equal(t, pm.Family, overPred.Family)
		assert.Greater(t, overPred.Stats.Mean, 0.0)
		assert.Greater(t, overPred.Stats.Variance, 0.0)
		assert.Greater(t, overPred.Stats.CV, 0.0)
		assert.Greater(t, overPred.CalibratedProb, 0.0)
		assert.Less(t, overPred.CalibratedProb, 1.0)
		assert.InDelta(t, pm.MarketBlendWeight, overPred.BlendWeight, 1e-12)

		underPred, err := pm.PredictLeg(feats, "UNDER", line, 0.48, 0.0)
		require.NoError(t, err)
		assert.InDelta(t, 1.0, overPred.CalibratedProb+underPred.CalibratedProb, 1e-12)

		// Missing market probability disables market blending cleanly.
		noMarket, err := pm.PredictLeg(feats, "OVER", line, 0.0, 0.15)
		require.NoError(t, err)
		assert.Zero(t, noMarket.BlendWeight)
		assert.InDelta(t, noMarket.ModelProb, noMarket.BlendedProb, 1e-12)
	}

	// Uncertainty boost shifts blend weight toward market anchor capped at 0.95.
	passModel, ok := bundle.PropModel(modelv2.MarketPassYds)
	require.True(t, ok)
	feats := modelv2.AlignFeatures(passModel.FeatureNames, map[string]float64{
		modelv2.FeatEWMAPassYds: 280.0,
	})
	basePred, err := passModel.PredictLeg(feats, "OVER", 240.5, 0.42, 0.0)
	require.NoError(t, err)
	boostedPred, err := passModel.PredictLeg(feats, "OVER", 240.5, 0.42, 0.15)
	require.NoError(t, err)
	assert.Greater(t, boostedPred.BlendWeight, basePred.BlendWeight)
	assert.LessOrEqual(t, boostedPred.BlendWeight, 0.95)
	assert.Less(t, math.Abs(boostedPred.BlendedProb-0.42), math.Abs(basePred.BlendedProb-0.42))

	// Zero-adjusted LogNormal and Poisson-with-optional-r artifact paths.
	lnArtifact := modelv2.DistModelArtifact{
		Market:            modelv2.MarketRecYds,
		Family:            modelv2.FamilyZeroAdjLogNormal,
		FeatureNames:      []string{modelv2.FeatEWMARecYds},
		MarketBlendWeight: 0.75,
		Calibrator:        modelv2.CalibratorArtifact{Method: modelv2.CalibMethodPlatt, PlattA: 1.0, PlattB: 0.0},
		ParamModels: map[string]modelv2.GBDTModel{
			"pi0":   {BaseScore: -3.0, Trees: []modelv2.FlatTree{{Nodes: []modelv2.TreeNode{{Feature: -1}}}}},
			"mu":    {BaseScore: 4.0, Trees: []modelv2.FlatTree{{Nodes: []modelv2.TreeNode{{Feature: -1}}}}},
			"sigma": {BaseScore: -0.5, Trees: []modelv2.FlatTree{{Nodes: []modelv2.TreeNode{{Feature: -1}}}}},
		},
	}
	require.NoError(t, lnArtifact.Validate())
	lnPred, err := lnArtifact.PredictLeg([]float64{60.0}, "OVER", math.Exp(4.0), 0.0, 0.0)
	require.NoError(t, err)
	assert.InDelta(t, (1.0/(1.0+math.Exp(-3.0)))*0.5, lnPred.ModelProb, 1e-12)

	poisWithR := modelv2.DistModelArtifact{
		Market:            modelv2.MarketPassTDs,
		Family:            modelv2.FamilyPoisson,
		FeatureNames:      []string{modelv2.FeatEWMAPassTDs},
		MarketBlendWeight: 0.75,
		Calibrator:        modelv2.CalibratorArtifact{Method: modelv2.CalibMethodPlatt, PlattA: 1.0, PlattB: 0.0},
		ParamModels: map[string]modelv2.GBDTModel{
			"lambda": {BaseScore: 0.5, Trees: []modelv2.FlatTree{{Nodes: []modelv2.TreeNode{{Feature: -1}}}}},
			"r":      {BaseScore: 2.0, Trees: []modelv2.FlatTree{{Nodes: []modelv2.TreeNode{{Feature: -1}}}}},
		},
	}
	require.NoError(t, poisWithR.Validate())
	dist, err := poisWithR.EvalDistribution([]float64{1.6})
	require.NoError(t, err)
	assert.Equal(t, modelv2.FamilyNegBin, dist.Family())

	_, err = passModel.PredictLeg(feats, "TEAM", 240.5, 0.5, 0)
	require.Error(t, err)
	_, err = passModel.PredictLeg(feats, "OVER", math.NaN(), 0.5, 0)
	require.Error(t, err)
}

//nolint:paralleltest // testing.AllocsPerRun cannot run concurrently with allocating tests
func TestPredictLegZeroAllocs(t *testing.T) {
	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	pm, ok := bundle.PropModel(modelv2.MarketPassYds)
	require.True(t, ok)

	feats := modelv2.AlignFeatures(pm.FeatureNames, map[string]float64{
		modelv2.FeatEWMAPassYds:      255.0,
		modelv2.FeatTeamImpliedTotal: 25.0,
		modelv2.FeatWindMPH:          8.0,
	})

	allocs := testing.AllocsPerRun(100, func() {
		_, err := pm.PredictLeg(feats, "OVER", 249.5, 0.52, 0.0)
		if err != nil {
			t.Fatal(err)
		}
	})
	assert.Zero(t, allocs)
}

func BenchmarkSurvivalZeroAdjGamma(b *testing.B) {
	g := modelv2.ZeroAdjGamma{Pi0: 0.03, Mu: 242.0, K: 8.5}
	b.ReportAllocs()
	for b.Loop() {
		_ = g.Survival(250.5)
	}
}

func BenchmarkSurvivalNegBin(b *testing.B) {
	nb := modelv2.NegBin{Mu: 5.2, R: 4.1}
	b.ReportAllocs()
	for b.Loop() {
		_ = nb.Survival(4.5)
	}
}

func BenchmarkPredictLeg(b *testing.B) {
	bundle, err := modelv2.LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	pm, ok := bundle.PropModel(modelv2.MarketPassYds)
	if !ok {
		b.Fatal("missing PASS_YDS model")
	}
	feats := modelv2.AlignFeatures(pm.FeatureNames, map[string]float64{
		modelv2.FeatEWMAPassYds:      255.0,
		modelv2.FeatTeamImpliedTotal: 25.0,
		modelv2.FeatWindMPH:          8.0,
	})

	b.ReportAllocs()
	for b.Loop() {
		_, _ = pm.PredictLeg(feats, "OVER", 249.5, 0.52, 0.0)
	}
}
