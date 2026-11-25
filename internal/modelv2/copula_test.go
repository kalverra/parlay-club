package modelv2_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/modelv2"
)

func loadTestBundle(t *testing.T) *modelv2.ArtifactBundle {
	t.Helper()
	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)
	return bundle
}

func TestCopulaDeterminismAndMultiGameFactorization(t *testing.T) {
	t.Parallel()

	cop := loadTestBundle(t).Copula

	g1Legs := []modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.54},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.52},
	}
	g2Legs := []modelv2.CopulaLeg{
		{LegID: 3, GameID: "g2", Team: "KC", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.50},
		{LegID: 4, GameID: "g2", Team: "KC", Role: "RB1", Market: modelv2.MarketRushYds, Side: "OVER", Prob: 0.53},
	}
	g3Leg := modelv2.CopulaLeg{
		LegID: 5, GameID: "g3", Team: "SF", Role: "TE1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.60,
	}

	ctx := map[string]modelv2.CopulaGameContext{
		"g1": {HomeTeam: "BUF", AwayTeam: "MIA", HomeSpread: -3.5, GameTotal: 50.5, HasSpread: true, HasTotal: true},
		"g2": {HomeTeam: "KC", AwayTeam: "LV", HomeSpread: -9.5, GameTotal: 43.5, HasSpread: true, HasTotal: true},
	}

	e1, err := cop.EstimateJoint(g1Legs, ctx)
	require.NoError(t, err)
	e2, err := cop.EstimateJoint(g2Legs, ctx)
	require.NoError(t, err)

	combined := append(append([]modelv2.CopulaLeg{}, g1Legs...), g2Legs...)
	combined = append(combined, g3Leg)

	allA, err := cop.EstimateJoint(combined, ctx)
	require.NoError(t, err)
	allB, err := cop.EstimateJoint(combined, ctx)
	require.NoError(t, err)

	// Bit-exact determinism.
	assert.Equal(t, allA, allB)

	// Multi-game factorization: p_joint = p_g1 * p_g2 * p_g3.
	wantJoint := e1.JointProb * e2.JointProb * g3Leg.Prob
	wantIndep := 0.54 * 0.52 * 0.50 * 0.53 * 0.60
	assert.InDelta(t, wantJoint, allA.JointProb, 1e-12)
	assert.InDelta(t, wantIndep, allA.IndependentProb, 1e-12)
	assert.InDelta(t, wantJoint/wantIndep, allA.CorrMultiplier, 1e-12)
	assert.Equal(t, modelv2.RegimeShootout, allA.RegimesByGame["g1"])
	assert.Equal(t, modelv2.RegimeHeavyFavorite, allA.RegimesByGame["g2"])
	assert.NotContains(t, allA.RegimesByGame, "g3")

	// Cross-game legs only (1 leg per game, plus one leg with empty GameID) -> M_corr == 1.0.
	cross := []modelv2.CopulaLeg{
		g1Legs[0],
		g2Legs[0],
		g3Leg,
		{LegID: 6, GameID: "", Team: "DET", Role: "RB1", Market: modelv2.MarketRushYds, Side: "OVER", Prob: 0.48},
		{LegID: 7, GameID: "", Team: "DET", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.51},
	}
	crossEst, err := cop.EstimateJoint(cross, ctx)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, crossEst.CorrMultiplier, 1e-12)
	assert.InDelta(t, crossEst.IndependentProb, crossEst.JointProb, 1e-12)
	assert.Empty(t, crossEst.PairCorrelations)
}

func TestCopulaAnalyticalBivariateSheppardFormula(t *testing.T) {
	t.Parallel()

	for _, rho := range []float64{-0.85, -0.5, 0.0, 0.5, 0.85} {
		cop := modelv2.CopulaArtifact{
			Nodes: []string{"QB_PASS_YDS", "WR1_REC_YDS"},
			DefaultMatrix: [][]float64{
				{1.0, rho},
				{rho, 1.0},
			},
		}
		legs := []modelv2.CopulaLeg{
			{GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.5},
			{GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.5},
		}
		est, err := cop.EstimateJoint(legs, nil)
		require.NoError(t, err)

		// Sheppard's formula: P(X <= 0, Y <= 0) = 1/4 + arcsin(rho) / (2*pi).
		want := 0.25 + math.Asin(rho)/(2.0*math.Pi)
		assert.InDelta(t, want, est.JointProb, 0.002, "rho=%v", rho)
		assert.InDelta(t, 0.25, est.IndependentProb, 1e-12)
		assert.Zero(t, est.JitterApplied)
	}
}

func TestCopulaStackConflictAndUnderSignFlips(t *testing.T) {
	t.Parallel()

	cop := loadTestBundle(t).Copula
	ctx := map[string]modelv2.CopulaGameContext{
		"g1": {HomeTeam: "BUF", AwayTeam: "MIA", HomeSpread: -3.0, GameTotal: 45.5, HasSpread: true, HasTotal: true},
	}

	// Positive stack: QB PASS_YDS OVER + WR1 REC_YDS OVER -> M_corr > 1.05.
	posEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.52},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.52},
	}, ctx)
	require.NoError(t, err)
	assert.Greater(t, posEst.CorrMultiplier, 1.15)
	require.Len(t, posEst.PairCorrelations, 1)
	assert.Greater(t, posEst.PairCorrelations[0].Correlation, 0.30)

	// Opposite-side stack: QB PASS_YDS OVER + WR1 REC_YDS UNDER -> sign flip makes correlation negative.
	flipEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.52},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "UNDER", Prob: 0.52},
	}, ctx)
	require.NoError(t, err)
	assert.Less(t, flipEst.CorrMultiplier, 0.85)
	require.Len(t, flipEst.PairCorrelations, 1)
	assert.InDelta(t, -posEst.PairCorrelations[0].Correlation, flipEst.PairCorrelations[0].Correlation, 1e-12)

	// Double-Under: QB PASS_YDS UNDER + WR1 REC_YDS UNDER -> (-1)*(-1) restores positive correlation.
	doubleUnderEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "UNDER", Prob: 0.52},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "UNDER", Prob: 0.52},
	}, ctx)
	require.NoError(t, err)
	assert.InDelta(t, posEst.JointProb, doubleUnderEst.JointProb, 1e-12)
	assert.InDelta(t, posEst.PairCorrelations[0].Correlation, doubleUnderEst.PairCorrelations[0].Correlation, 1e-12)

	// Conflicting script: Heavy favorite RB1 RUSH_YDS UNDER + BUF SPREAD TEAM -> negative correlation.
	rbConflict, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Market: "SPREAD", Side: "TEAM", Prob: 0.52},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "RB1", Market: modelv2.MarketRushYds, Side: "UNDER", Prob: 0.52},
	}, ctx)
	require.NoError(t, err)
	assert.Less(t, rbConflict.CorrMultiplier, 0.90)
	assert.Less(t, rbConflict.PairCorrelations[0].Correlation, -0.25)

	// Opposing team SPREAD vs ML -> -1.0 correlation; impossible if p1 + p2 <= 1.
	oppMargin, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Market: "SPREAD", Side: "TEAM", Prob: 0.50},
		{LegID: 2, GameID: "g1", Team: "MIA", Market: "ML", Side: "TEAM", Prob: 0.50},
	}, ctx)
	require.NoError(t, err)
	assert.InDelta(t, -1.0, oppMargin.PairCorrelations[0].Correlation, 1e-12)
	assert.InDelta(t, 0.0, oppMargin.JointProb, 1e-12)
}

func TestCopulaRegimeSelectionAndMatrixDifferences(t *testing.T) {
	t.Parallel()

	assert.Equal(t, modelv2.RegimeNeutral, modelv2.SelectRegime(modelv2.CopulaGameContext{
		HomeSpread: -3.5, GameTotal: 44.5, HasSpread: true, HasTotal: true,
	}))
	assert.Equal(t, modelv2.RegimeHeavyFavorite, modelv2.SelectRegime(modelv2.CopulaGameContext{
		HomeSpread: -7.0, GameTotal: 52.0, HasSpread: true, HasTotal: true,
	}))
	assert.Equal(t, modelv2.RegimeHeavyFavorite, modelv2.SelectRegime(modelv2.CopulaGameContext{
		HomeSpread: 7.5, GameTotal: 42.0, HasSpread: true, HasTotal: true,
	}))
	assert.Equal(t, modelv2.RegimeShootout, modelv2.SelectRegime(modelv2.CopulaGameContext{
		HomeSpread: -2.5, GameTotal: 48.0, HasSpread: true, HasTotal: true,
	}))

	cop := loadTestBundle(t).Copula
	// Compare bring-back correlation (QB vs OPP_QB) in neutral vs shootout regimes.
	legs := []modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.5},
		{LegID: 2, GameID: "g1", Team: "MIA", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.5},
	}

	neutralEst, err := cop.EstimateJoint(legs, map[string]modelv2.CopulaGameContext{
		"g1": {HomeSpread: -2.5, GameTotal: 43.0, HasSpread: true, HasTotal: true},
	})
	require.NoError(t, err)
	shootoutEst, err := cop.EstimateJoint(legs, map[string]modelv2.CopulaGameContext{
		"g1": {HomeSpread: -2.5, GameTotal: 52.5, HasSpread: true, HasTotal: true},
	})
	require.NoError(t, err)

	assert.Equal(t, modelv2.RegimeNeutral, neutralEst.RegimesByGame["g1"])
	assert.Equal(t, modelv2.RegimeShootout, shootoutEst.RegimesByGame["g1"])
	assert.Greater(t, shootoutEst.PairCorrelations[0].Correlation, neutralEst.PairCorrelations[0].Correlation)
	assert.Greater(t, shootoutEst.JointProb, neutralEst.JointProb)

	// Heavy favorite fallback via leg fields when gameCtx map is nil.
	heavyLegs := []modelv2.CopulaLeg{
		{
			LegID:      1,
			GameID:     "g1",
			Team:       "KC",
			Market:     "SPREAD",
			Side:       "TEAM",
			Prob:       0.52,
			HomeSpread: -10.5,
			GameTotal:  45.0,
		},
		{LegID: 2, GameID: "g1", Team: "KC", Role: "RB1", Market: modelv2.MarketRushYds, Side: "OVER", Prob: 0.52},
	}
	heavyEst, err := cop.EstimateJoint(heavyLegs, nil)
	require.NoError(t, err)
	assert.Equal(t, modelv2.RegimeHeavyFavorite, heavyEst.RegimesByGame["g1"])
	assert.Greater(t, heavyEst.CorrMultiplier, 1.10)
}

func TestCopulaWithinPlayerOverridesAndCollisions(t *testing.T) {
	t.Parallel()

	cop := loadTestBundle(t).Copula

	// Same player RECEPTIONS OVER + REC_YDS OVER -> uses WithinPlayer["RECEPTIONS:REC_YDS"].
	wantRecCorr, ok := cop.WithinPlayer["RECEPTIONS:REC_YDS"]
	require.True(t, ok)
	assert.Greater(t, wantRecCorr, 0.65)
	samePlayerEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{
			LegID:    10,
			GameID:   "g1",
			Team:     "CIN",
			PlayerID: "chase-1",
			Role:     "WR1",
			Market:   modelv2.MarketReceptions,
			Side:     "OVER",
			Prob:     0.55,
		},
		{
			LegID:    11,
			GameID:   "g1",
			Team:     "CIN",
			PlayerID: "chase-1",
			Role:     "WR1",
			Market:   modelv2.MarketRecYds,
			Side:     "OVER",
			Prob:     0.55,
		},
	}, nil)
	require.NoError(t, err)
	require.Len(t, samePlayerEst.PairCorrelations, 1)
	assert.True(t, samePlayerEst.PairCorrelations[0].WithinPlayer)
	assert.InDelta(t, wantRecCorr, samePlayerEst.PairCorrelations[0].Correlation, 1e-12)

	// Same player REC_YDS OVER + ANYTIME_TD YES -> uses WithinPlayer["REC_YDS:ANYTIME_TD"].
	wantTDCorr, ok := cop.WithinPlayer["REC_YDS:ANYTIME_TD"]
	require.True(t, ok)
	assert.Greater(t, wantTDCorr, 0.20)
	tdPairEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{
			LegID:      10,
			GameID:     "g1",
			Team:       "CIN",
			PlayerName: "Ja'Marr Chase",
			Role:       "WR1",
			Market:     modelv2.MarketRecYds,
			Side:       "OVER",
			Prob:       0.55,
		},
		{
			LegID:      11,
			GameID:     "g1",
			Team:       "CIN",
			PlayerName: "ja'marr chase",
			Role:       "WR1",
			Market:     modelv2.MarketAnytimeTD,
			Side:       "YES",
			Prob:       0.42,
		},
	}, nil)
	require.NoError(t, err)
	require.Len(t, tdPairEst.PairCorrelations, 1)
	assert.True(t, tdPairEst.PairCorrelations[0].WithinPlayer)
	assert.InDelta(t, wantTDCorr, tdPairEst.PairCorrelations[0].Correlation, 1e-12)

	// Same player, same market (ladder alt lines): rho = 1.0 -> joint prob == min(p1, p2).
	ladderEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{
			LegID:    1,
			GameID:   "g1",
			Team:     "CIN",
			PlayerID: "chase-1",
			Role:     "WR1",
			Market:   modelv2.MarketRecYds,
			Side:     "OVER",
			Prob:     0.65,
		},
		{
			LegID:    2,
			GameID:   "g1",
			Team:     "CIN",
			PlayerID: "chase-1",
			Role:     "WR1",
			Market:   modelv2.MarketRecYds,
			Side:     "OVER",
			Prob:     0.35,
		},
	}, nil)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, ladderEst.PairCorrelations[0].Correlation, 1e-12)
	assert.InDelta(t, 0.35, ladderEst.JointProb, 1e-3)
	assert.Zero(t, ladderEst.JitterApplied)

	// Distinct players sharing same role node (e.g. two WR2 depth receivers) must not get rho = 1.0.
	distinctSameNode, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{
			LegID:    1,
			GameID:   "g1",
			Team:     "CIN",
			PlayerID: "wr-a",
			Role:     "WR2",
			Market:   modelv2.MarketRecYds,
			Side:     "OVER",
			Prob:     0.50,
		},
		{
			LegID:    2,
			GameID:   "g1",
			Team:     "CIN",
			PlayerID: "wr-b",
			Role:     "WR2",
			Market:   modelv2.MarketRecYds,
			Side:     "OVER",
			Prob:     0.50,
		},
	}, nil)
	require.NoError(t, err)
	assert.False(t, distinctSameNode.PairCorrelations[0].WithinPlayer)
	assert.InDelta(t, 0.20, distinctSameNode.PairCorrelations[0].Correlation, 1e-12)
}

func TestCopulaPSDJitterFallback(t *testing.T) {
	t.Parallel()

	// Construct an impossible 3x3 correlation triple via within-player overrides:
	// (A, B) = 0.9, (A, C) = 0.9, (B, C) = -0.9 (not PSD: min eigenvalue < 0).
	cop := modelv2.CopulaArtifact{
		Nodes: []string{"WR1_REC_YDS", "WR1_RECEPTIONS", "GAME_TOTAL"},
		DefaultMatrix: [][]float64{
			{1, 0, 0},
			{0, 1, 0},
			{0, 0, 1},
		},
		WithinPlayer: map[string]float64{
			"REC_YDS:RECEPTIONS":    0.9,
			"REC_YDS:ANYTIME_TD":    0.9,
			"RECEPTIONS:ANYTIME_TD": -0.9,
		},
	}

	est, err := cop.EstimateJoint([]modelv2.CopulaLeg{
		{
			LegID:    1,
			GameID:   "g1",
			Team:     "BUF",
			PlayerID: "p1",
			Role:     "WR1",
			Market:   modelv2.MarketRecYds,
			Side:     "OVER",
			Prob:     0.55,
		},
		{
			LegID:    2,
			GameID:   "g1",
			Team:     "BUF",
			PlayerID: "p1",
			Role:     "WR1",
			Market:   modelv2.MarketReceptions,
			Side:     "OVER",
			Prob:     0.55,
		},
		{
			LegID:    3,
			GameID:   "g1",
			Team:     "BUF",
			PlayerID: "p1",
			Role:     "WR1",
			Market:   modelv2.MarketAnytimeTD,
			Side:     "YES",
			Prob:     0.45,
		},
	}, nil)
	require.NoError(t, err)
	assert.Greater(t, est.JitterApplied, 0.0)
	assert.GreaterOrEqual(t, est.JointProb, 0.0)
	assert.LessOrEqual(t, est.JointProb, 1.0)
	assert.False(t, math.IsNaN(est.CorrMultiplier))
}

func TestCopulaRankedPairCorrelationsAndProxies(t *testing.T) {
	t.Parallel()

	cop := loadTestBundle(t).Copula
	// 4-leg SGP: TOTAL OVER, QB PASS_YDS OVER, WR1 REC_YDS OVER, RB1 RUSH_YDS UNDER,
	// plus TEAM_TOTAL and OPP_WR1_RECEPTIONS proxies.
	legs := []modelv2.CopulaLeg{
		{LegID: 101, GameID: "g1", Market: "TOTAL", Side: "OVER", Prob: 0.52},
		{LegID: 102, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.53},
		{LegID: 103, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.51},
		{LegID: 104, GameID: "g1", Team: "BUF", Role: "RB1", Market: modelv2.MarketRushYds, Side: "UNDER", Prob: 0.50},
		{
			LegID:  105,
			GameID: "g1",
			Team:   "MIA",
			Role:   "WR1",
			Market: modelv2.MarketReceptions,
			Side:   "OVER",
			Prob:   0.54,
		},
		{LegID: 106, GameID: "g1", Team: "BUF", Market: "TEAM_TOTAL", Side: "OVER", Prob: 0.52},
	}

	est, err := cop.EstimateJoint(legs, map[string]modelv2.CopulaGameContext{
		"g1": {HomeTeam: "BUF", AwayTeam: "MIA", HomeSpread: -2.5, GameTotal: 49.5, HasSpread: true, HasTotal: true},
	})
	require.NoError(t, err)
	require.Len(t, est.PairCorrelations, 15) // 6 choose 2

	// Ranked strictly descending by |Correlation|.
	for i := 1; i < len(est.PairCorrelations); i++ {
		prev := est.PairCorrelations[i-1]
		curr := est.PairCorrelations[i]
		assert.GreaterOrEqual(t, math.Abs(prev.Correlation), math.Abs(curr.Correlation), "pair %d not sorted", i)
		assert.Equal(t, "g1", curr.GameID)
		assert.NotEmpty(t, curr.NodeA)
		assert.NotEmpty(t, curr.NodeB)
	}

	// Strongest pair should show meaningful correlation (> 0.35).
	assert.Greater(t, math.Abs(est.PairCorrelations[0].Correlation), 0.35)
}

func TestCopulaEdgeCasesAndValidation(t *testing.T) {
	t.Parallel()

	cop := loadTestBundle(t).Copula

	t.Run("empty and single leg", func(t *testing.T) {
		t.Parallel()

		emptyEst, err := cop.EstimateJoint(nil, nil)
		require.NoError(t, err)
		assert.Zero(t, emptyEst.JointProb)
		assert.InDelta(t, 1.0, emptyEst.CorrMultiplier, 1e-12)

		oneEst, err := cop.EstimateJoint([]modelv2.CopulaLeg{
			{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.57},
		}, nil)
		require.NoError(t, err)
		assert.InDelta(t, 0.57, oneEst.JointProb, 1e-12)
		assert.InDelta(t, 0.57, oneEst.IndependentProb, 1e-12)
		assert.InDelta(t, 1.0, oneEst.CorrMultiplier, 1e-12)
	})

	t.Run("rejects invalid prob and invalid artifact", func(t *testing.T) {
		t.Parallel()

		for _, badProb := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1)} {
			_, err := cop.EstimateJoint([]modelv2.CopulaLeg{
				{GameID: "g1", Market: "TOTAL", Side: "OVER", Prob: badProb},
			}, nil)
			require.Error(t, err)
		}

		badCop := modelv2.CopulaArtifact{Nodes: []string{"ONLY_ONE"}}
		_, err := badCop.EstimateJoint([]modelv2.CopulaLeg{
			{GameID: "g1", Market: "TOTAL", Side: "OVER", Prob: 0.5},
			{GameID: "g1", Market: "SPREAD", Side: "TEAM", Prob: 0.5},
		}, nil)
		require.Error(t, err)
	})

	t.Run("unknown market fails soft as independent within game", func(t *testing.T) {
		t.Parallel()

		est, err := cop.EstimateJoint([]modelv2.CopulaLeg{
			{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.50},
			{LegID: 2, GameID: "g1", Team: "BUF", Role: "LB", Market: "DEF_TACKLES", Side: "OVER", Prob: 0.50},
		}, nil)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, est.PairCorrelations[0].Correlation, 1e-12)
		assert.InDelta(t, 0.25, est.JointProb, 0.002)
	})

	t.Run("over 32 legs in one game uses Halton extension", func(t *testing.T) {
		t.Parallel()

		legs := make([]modelv2.CopulaLeg, 35)
		for i := range legs {
			legs[i] = modelv2.CopulaLeg{
				LegID:  int64(i + 1),
				GameID: "g1",
				Team:   "BUF",
				Market: "CUSTOM_PROP",
				Side:   "OVER",
				Prob:   0.98,
			}
		}
		est, err := cop.EstimateJoint(legs, nil)
		require.NoError(t, err)
		assert.Greater(t, est.JointProb, 0.40)
		assert.Less(t, est.JointProb, 0.65)
	})
}

func BenchmarkCopulaEstimateJoint2LegSGP(b *testing.B) {
	cop, err := modelv2.LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	legs := []modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.54},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.52},
	}
	ctx := map[string]modelv2.CopulaGameContext{
		"g1": {HomeTeam: "BUF", AwayTeam: "MIA", HomeSpread: -3.5, GameTotal: 49.5, HasSpread: true, HasTotal: true},
	}

	// Warm up sync.Once Sobol table outside measured loop.
	if _, err := cop.Copula.EstimateJoint(legs, ctx); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := cop.Copula.EstimateJoint(legs, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCopulaEstimateJoint4LegSGP(b *testing.B) {
	cop, err := modelv2.LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	legs := []modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.54},
		{LegID: 2, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.52},
		{LegID: 3, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketReceptions, Side: "OVER", Prob: 0.55},
		{LegID: 4, GameID: "g1", Team: "MIA", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.51},
	}
	ctx := map[string]modelv2.CopulaGameContext{
		"g1": {HomeTeam: "BUF", AwayTeam: "MIA", HomeSpread: -3.5, GameTotal: 49.5, HasSpread: true, HasTotal: true},
	}

	if _, err := cop.Copula.EstimateJoint(legs, ctx); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := cop.Copula.EstimateJoint(legs, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCopulaEstimateJoint8LegSGP(b *testing.B) {
	cop, err := modelv2.LoadEmbedded()
	if err != nil {
		b.Fatal(err)
	}
	legs := []modelv2.CopulaLeg{
		{LegID: 1, GameID: "g1", Market: "TOTAL", Side: "OVER", Prob: 0.52},
		{LegID: 2, GameID: "g1", Team: "BUF", Market: "SPREAD", Side: "TEAM", Prob: 0.52},
		{LegID: 3, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.54},
		{LegID: 4, GameID: "g1", Team: "BUF", Role: "QB", Market: modelv2.MarketPassTDs, Side: "OVER", Prob: 0.53},
		{LegID: 5, GameID: "g1", Team: "BUF", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.52},
		{LegID: 6, GameID: "g1", Team: "BUF", Role: "TE1", Market: modelv2.MarketReceptions, Side: "OVER", Prob: 0.55},
		{LegID: 7, GameID: "g1", Team: "MIA", Role: "QB", Market: modelv2.MarketPassYds, Side: "OVER", Prob: 0.51},
		{LegID: 8, GameID: "g1", Team: "MIA", Role: "WR1", Market: modelv2.MarketRecYds, Side: "OVER", Prob: 0.51},
	}
	ctx := map[string]modelv2.CopulaGameContext{
		"g1": {HomeTeam: "BUF", AwayTeam: "MIA", HomeSpread: -3.5, GameTotal: 51.5, HasSpread: true, HasTotal: true},
	}

	if _, err := cop.Copula.EstimateJoint(legs, ctx); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := cop.Copula.EstimateJoint(legs, ctx); err != nil {
			b.Fatal(err)
		}
	}
}
