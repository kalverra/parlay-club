package analyzer_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/modelv2"
)

func assertFriendlyV2Tone(t *testing.T, texts ...string) {
	t.Helper()
	for _, text := range texts {
		lower := strings.ToLower(text)
		assert.NotContains(t, lower, "+ev", "forbidden +EV jargon in %q", text)
		assert.NotContains(t, lower, "expected value", "forbidden expected value jargon in %q", text)
		assert.NotContains(t, lower, "roi", "forbidden ROI jargon in %q", text)
		for tok := range strings.FieldsSeq(lower) {
			trimmed := strings.Trim(tok, ".,:;!?()[]\"'")
			assert.NotEqual(t, "ev", trimmed, "forbidden EV token in %q", text)
		}
	}
}

func assertValidFourPillars(t *testing.T, pillars []analyzer.PillarScore) map[string]analyzer.PillarScore {
	t.Helper()
	require.Len(t, pillars, 4)

	wantKeys := []string{
		analyzer.PillarPriceVig,
		analyzer.PillarScriptFit,
		analyzer.PillarStability,
		analyzer.PillarContextTraps,
	}
	wantLabels := []string{
		analyzer.PillarLabelPriceVig,
		analyzer.PillarLabelScriptFit,
		analyzer.PillarLabelStability,
		analyzer.PillarLabelContextTraps,
	}

	byKey := make(map[string]analyzer.PillarScore, 4)
	for i, p := range pillars {
		assert.Equal(t, wantKeys[i], p.Key)
		assert.Equal(t, wantLabels[i], p.Label)
		assert.GreaterOrEqual(t, p.Score, 0)
		assert.LessOrEqual(t, p.Score, 100)
		assert.Contains(t, []string{
			analyzer.ToneGood,
			analyzer.ToneNeutral,
			analyzer.ToneWarn,
			analyzer.ToneBad,
		}, p.Tone)
		assert.NotEmpty(t, p.Summary)
		assert.NotEmpty(t, p.Reasons)
		assert.LessOrEqual(t, len(p.Reasons), 3)

		assertFriendlyV2Tone(t, p.Summary)
		assertFriendlyV2Tone(t, p.Reasons...)
		byKey[p.Key] = p
	}
	return byKey
}

func richV2Game() analyzer.Game {
	return analyzer.Game{
		ID:     "buf-mia",
		Home:   "MIA",
		Away:   "BUF",
		Indoor: false,
		Weather: &analyzer.Weather{
			WindMPH:  5,
			TempF:    72,
			PrecipIn: 0,
		},
		MarketScript: analyzer.MarketScript{
			HomeSpread:       3.5,
			GameTotal:        49.5,
			HomeImpliedTotal: 23.0,
			AwayImpliedTotal: 26.5,
			HasSpread:        true,
			HasTotal:         true,
		},
		HomeEPA: &analyzer.TeamDefenseProfile{
			DefPassEPAAllowed:     0.12,
			DefRushEPAAllowed:     0.03,
			PlaysPerGame:          63.0,
			PassRate:              0.56,
			DefPassEPAAllowedRank: 24,
			DefRushEPAAllowedRank: 18,
		},
		AwayEPA: &analyzer.TeamDefenseProfile{
			DefPassEPAAllowed:     -0.05,
			DefRushEPAAllowed:     -0.04,
			PlaysPerGame:          64.5,
			PassRate:              0.59,
			DefPassEPAAllowedRank: 8,
			DefRushEPAAllowedRank: 10,
		},
		HomeRoleDefense: map[analyzer.RoleArchetype]*analyzer.RoleDefenseProfile{
			analyzer.RoleQB: {
				Role:            analyzer.RoleQB,
				Games:           6,
				PassYdsAllowed:  255.0,
				PassTDsAllowed:  1.9,
				TotalEPAAllowed: 8.5,
			},
			analyzer.RoleWR1: {
				Role:              analyzer.RoleWR1,
				Games:             6,
				RecYdsAllowed:     82.0,
				ReceptionsAllowed: 6.2,
				RecTDsAllowed:     0.7,
				TotalEPAAllowed:   4.2,
			},
			analyzer.RoleRB1: {
				Role:            analyzer.RoleRB1,
				Games:           6,
				RushYdsAllowed:  76.0,
				RushTDsAllowed:  0.7,
				RecTDsAllowed:   0.2,
				TotalEPAAllowed: 2.1,
			},
		},
	}
}

func richV2Stats() map[string]analyzer.PlayerStats {
	return map[string]analyzer.PlayerStats{
		"allen": {
			Position: "QB",
			Role:     analyzer.RoleQB,
			Games: map[string][]float64{
				"PASS_YDS": {285, 270, 295, 260, 278},
				"PASS_TDS": {2, 3, 2, 2, 2},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6,
				PassAtt:     36.0,
				PassYds:     274.0,
				PassTDs:     2.2,
				CPOE:        4.2,
				TotalEPA:    11.5,
				ExpPassYds:  268.0,
			},
		},
		"shakir": {
			Position: "WR",
			Role:     analyzer.RoleWR1,
			Games: map[string][]float64{
				"REC_YDS":    {78, 84, 72, 68, 75},
				"RECEPTIONS": {6, 7, 6, 5, 6},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed:   6,
				OffensePct:    0.85,
				TargetShare:   0.24,
				WOPR:          0.55,
				Targets:       8.2,
				Receptions:    6.1,
				RecYds:        76.0,
				RecTDs:        0.5,
				TotalEPA:      5.1,
				ExpRecYds:     72.0,
				ExpReceptions: 5.8,
			},
		},
		"cook": {
			Position: "RB",
			Role:     analyzer.RoleRB1,
			Games: map[string][]float64{
				"RUSH_YDS": {82, 75, 69, 90, 72},
				"RUSH_ATT": {17, 16, 15, 18, 16},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6,
				OffensePct:  0.66,
				CarryShare:  0.58,
				Carries:     16.4,
				RushYds:     78.0,
				RushTDs:     0.7,
				TotalEPA:    2.6,
				ExpRushYds:  75.0,
			},
		},
	}
}

func TestGradingEngineRegistryAndV1Compatibility(t *testing.T) {
	t.Parallel()

	// ActiveEngine is EngineV2 following calibration gate clearance.
	assert.Equal(t, "v2", analyzer.ActiveEngine.Name())
	assert.Equal(t, analyzer.EngineV2, analyzer.ActiveEngine)

	v1, ok := analyzer.EngineByName("v1")
	require.True(t, ok)
	assert.Equal(t, "v1", v1.Name())

	v2, ok := analyzer.EngineByName("v2")
	require.True(t, ok)
	assert.Equal(t, "v2", v2.Name())

	_, ok = analyzer.EngineByName("missing")
	assert.False(t, ok)

	names := analyzer.EngineNames()
	assert.Equal(t, []string{"v1", "v2"}, names)

	engines := analyzer.BuiltInEngines()
	require.Len(t, engines, 2)
	assert.Equal(t, "v1", engines[0].Name())
	assert.Equal(t, "v2", engines[1].Name())

	// Mutating BuiltInEngines slice must not mutate registry.
	engines[0] = nil
	assert.NotNil(t, analyzer.BuiltInEngines()[0])

	// Custom NewEngineV2WithBundle and custom ContextOverlay pluggability.
	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)
	customEngine := analyzer.NewEngineV2WithBundle(
		analyzer.DefaultV2Config,
		bundle,
		func(leg analyzer.Leg, _ analyzer.Game, _ map[string]analyzer.PlayerStats, _ *modelv2.LegPrediction) []analyzer.OverlayFinding {
			return []analyzer.OverlayFinding{{
				Rule:       "custom_beat_report",
				Label:      "Beat note",
				Detail:     "Custom beat note applied.",
				Severity:   analyzer.Warn,
				ScoreDelta: -10,
				LegIDs:     []int64{leg.ID},
			}}
		},
	)
	assert.Equal(t, "v2", customEngine.Name())
	customLeg := customEngine.ScoreLeg(
		withPlayerID(leg(99, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110), "allen"),
		map[string]analyzer.Game{"buf-mia": richV2Game()},
		richV2Stats(),
	)
	assert.Contains(t, factorRules(customLeg.Factors), "custom_beat_report")

	// v1 output remains zero-value on v2-only fields.
	g := richV2Game()
	l := withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110), "allen")
	v1Leg := v1.ScoreLeg(l, map[string]analyzer.Game{"buf-mia": g}, richV2Stats())
	assert.Empty(t, v1Leg.Pillars)
	assert.Zero(t, v1Leg.CalibratedProb)
	assert.Zero(t, v1Leg.MarketProb)
	assert.False(t, v1Leg.Uncertain)

	v1Slip := v1.EvaluateSlip([]analyzer.Leg{l}, map[string]analyzer.Game{"buf-mia": g}, richV2Stats())
	assert.Empty(t, v1Slip.Grade.Pillars)
	assert.Zero(t, v1Slip.Grade.CalibratedProb)
	assert.Zero(t, v1Slip.Grade.CorrMultiplier)
}

func TestEngineV2SkipsUngradableAndIsDeterministic(t *testing.T) {
	t.Parallel()

	v2 := analyzer.EngineV2
	gamesMap := map[string]analyzer.Game{"buf-mia": richV2Game()}
	statsMap := richV2Stats()

	otherLeg := leg(1, "Adam", "buf-mia", "OTHER", "BUF", "", "", nil, -110)
	assert.Equal(t, analyzer.LegGrade{LegID: 1}, v2.ScoreLeg(otherLeg, gamesMap, statsMap))

	missingGameLeg := leg(2, "Adam", "unknown", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110)
	assert.Equal(t, analyzer.LegGrade{LegID: 2}, v2.ScoreLeg(missingGameLeg, gamesMap, statsMap))

	emptyEval := v2.EvaluateSlip([]analyzer.Leg{otherLeg, missingGameLeg}, gamesMap, statsMap)
	assert.Empty(t, emptyEval.Grade.Letter)
	assert.Zero(t, emptyEval.Grade.Score)
	assert.Empty(t, emptyEval.Grade.Pillars)
	require.Len(t, emptyEval.Grade.LegGrades, 2)

	// Determinism check across repeated evaluations.
	legs := []analyzer.Leg{
		withPlayerID(leg(10, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110), "allen"),
		withPlayerID(leg(11, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(70.5), -110), "shakir"),
	}
	eval1 := v2.EvaluateSlip(legs, gamesMap, statsMap)
	eval2 := v2.EvaluateSlip(legs, gamesMap, statsMap)
	assert.Equal(t, eval1, eval2)
}

func TestEngineV2LegFourPillarsAndMarkets(t *testing.T) {
	t.Parallel()

	v2 := analyzer.EngineV2
	statsMap := richV2Stats()

	t.Run("price_vig reflects sharp edge vs steep juice and line shopping", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		key := analyzer.SharpKey{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}
		g.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			key: {
				Line:          new(270.5),
				FairProb:      0.51,
				PowerFairProb: 0.51,
				VigPct:        0.045,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		gamesMap := map[string]analyzer.Game{"buf-mia": g}

		plusOdds := withPlayerID(
			leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(270.5), +110),
			"allen",
		)
		stdOdds := withPlayerID(
			leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(270.5), -110),
			"allen",
		)
		steepOdds := withPlayerID(
			leg(3, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(270.5), -165),
			"allen",
		)

		gPlus := v2.ScoreLeg(plusOdds, gamesMap, statsMap)
		gStd := v2.ScoreLeg(stdOdds, gamesMap, statsMap)
		gSteep := v2.ScoreLeg(steepOdds, gamesMap, statsMap)

		pPlus := assertValidFourPillars(t, gPlus.Pillars)
		pStd := assertValidFourPillars(t, gStd.Pillars)
		pSteep := assertValidFourPillars(t, gSteep.Pillars)

		assert.Greater(t, pPlus[analyzer.PillarPriceVig].Score, pStd[analyzer.PillarPriceVig].Score)
		assert.Greater(t, pStd[analyzer.PillarPriceVig].Score, pSteep[analyzer.PillarPriceVig].Score)
		assert.InDelta(t, 0.51, gStd.MarketProb, 1e-6)
		assert.Greater(t, gStd.CalibratedProb, 0.0)
		assert.Less(t, gStd.CalibratedProb, 1.0)
		assert.Greater(t, gPlus.Score, gSteep.Score)

		// Spread line shopping (-3 vs Pinnacle -3.5) boosts Price & Vig and emits sharp_line_edge factor.
		g.Sharp[analyzer.SharpKey{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}] = analyzer.SharpQuote{
			Line:          new(-3.5),
			FairProb:      0.50,
			PowerFairProb: 0.50,
			VigPct:        0.045,
			Source:        "Pinnacle",
			Tier:          1,
		}
		gamesMap["buf-mia"] = g
		goodLine := leg(4, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110)
		badLine := leg(5, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-4.5), -110)
		gGoodLine := v2.ScoreLeg(goodLine, gamesMap, statsMap)
		gBadLine := v2.ScoreLeg(badLine, gamesMap, statsMap)

		pGoodLine := assertValidFourPillars(t, gGoodLine.Pillars)
		pBadLine := assertValidFourPillars(t, gBadLine.Pillars)
		assert.Greater(t, pGoodLine[analyzer.PillarPriceVig].Score, pBadLine[analyzer.PillarPriceVig].Score)
		assert.Contains(t, factorRules(gGoodLine.Factors), "sharp_line_edge")
		assert.Contains(t, factorRules(gBadLine.Factors), "sharp_line_fade")
	})

	t.Run("script_fit single-leg aligns rushing props with MarketScript spread", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		// BUF is -3.5 favorite; make BUF -8.5 heavy favorite vs +7.5 big underdog.
		gFav := g
		gFav.MarketScript.HomeSpread = 8.5 // Away BUF is -8.5
		gDog := g
		gDog.MarketScript.HomeSpread = -7.5 // Away BUF is +7.5

		rushLeg := withPlayerID(
			leg(1, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(72.5), -110),
			"cook",
		)
		favGrade := v2.ScoreLeg(rushLeg, map[string]analyzer.Game{"buf-mia": gFav}, statsMap)
		dogGrade := v2.ScoreLeg(rushLeg, map[string]analyzer.Game{"buf-mia": gDog}, statsMap)

		pFav := assertValidFourPillars(t, favGrade.Pillars)
		pDog := assertValidFourPillars(t, dogGrade.Pillars)
		assert.Greater(t, pFav[analyzer.PillarScriptFit].Score, 80)
		assert.Less(t, pDog[analyzer.PillarScriptFit].Score, 65)
		assert.Equal(t, analyzer.ToneGood, pFav[analyzer.PillarScriptFit].Tone)
		assert.Contains(t, []string{analyzer.ToneWarn, analyzer.ToneBad}, pDog[analyzer.PillarScriptFit].Tone)
	})

	t.Run(
		"stability_risk penalizes ANYTIME_TD and extreme longshots while rewarding steady volume",
		func(t *testing.T) {
			t.Parallel()
			gamesMap := map[string]analyzer.Game{"buf-mia": richV2Game()}

			steadyRec := withPlayerID(
				leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(55.5), -110),
				"shakir",
			)
			longshotRec := withPlayerID(
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(145.5), +450),
				"shakir",
			)
			tdLeg := withPlayerID(
				leg(3, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "James Cook", "YES", nil, +115),
				"cook",
			)

			gSteady := v2.ScoreLeg(steadyRec, gamesMap, statsMap)
			gLong := v2.ScoreLeg(longshotRec, gamesMap, statsMap)
			gTD := v2.ScoreLeg(tdLeg, gamesMap, statsMap)

			pSteady := assertValidFourPillars(t, gSteady.Pillars)
			pLong := assertValidFourPillars(t, gLong.Pillars)
			pTD := assertValidFourPillars(t, gTD.Pillars)

			assert.Greater(t, pSteady[analyzer.PillarStability].Score, pLong[analyzer.PillarStability].Score)
			assert.Greater(t, pSteady[analyzer.PillarStability].Score, pTD[analyzer.PillarStability].Score)
			assert.Less(t, gLong.CalibratedProb, 0.25)
		})

	t.Run("context_traps captures overlays, HeightenedUncertainty, and hard Danger cap", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		g.Injuries = map[string][]analyzer.Injury{
			"BUF": {{AthleteID: "shakir", FullName: "Khalil Shakir", Position: "WR", Status: "Out"}},
		}
		g.Weather = &analyzer.Weather{WindMPH: 23, TempF: 40}
		g.ContextSignals = []analyzer.ContextSignal{{
			Kind:                  "news",
			PlayerID:              "shakir",
			Label:                 "Ruled out",
			Detail:                "Team ruled Shakir out before kickoff.",
			Severity:              analyzer.Danger,
			ScoreDelta:            -10,
			HeightenedUncertainty: true,
		}}

		outLeg := withPlayerID(
			leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(55.5), +150),
			"shakir",
		)
		grade := v2.ScoreLeg(outLeg, map[string]analyzer.Game{"buf-mia": g}, statsMap)
		p := assertValidFourPillars(t, grade.Pillars)

		assert.True(t, grade.Uncertain)
		assert.Less(t, p[analyzer.PillarContextTraps].Score, 55)
		assert.Equal(t, analyzer.ToneBad, p[analyzer.PillarContextTraps].Tone)
		assert.LessOrEqual(t, grade.Score, 72, "Danger overlay must hard-cap leg score at C or lower")
		assert.Contains(t, factorRules(grade.Factors), "player_injury_status")
		assert.Contains(t, factorRules(grade.Factors), "wind")
	})

	t.Run("handles missing prop line and sparse Weeks 1-2 prior shrinkage gracefully", func(t *testing.T) {
		t.Parallel()
		gamesMap := map[string]analyzer.Game{"buf-mia": richV2Game()}

		noLineProp := withPlayerID(
			leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", nil, -110),
			"shakir",
		)
		gNoLine := v2.ScoreLeg(noLineProp, gamesMap, statsMap)
		assert.NotEmpty(t, gNoLine.Letter)
		assertValidFourPillars(t, gNoLine.Pillars)

		sparseStats := map[string]analyzer.PlayerStats{
			"rookie": {
				Position: "WR",
				Role:     analyzer.RoleWR2,
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 2,
					RecYds:      48.0,
				},
			},
		}
		rookieLeg := withPlayerID(
			leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(45.5), -110),
			"rookie",
		)
		gRookie := v2.ScoreLeg(rookieLeg, gamesMap, sparseStats)
		assert.NotEmpty(t, gRookie.Letter)
		assert.Greater(t, gRookie.CalibratedProb, 0.0)
		assertValidFourPillars(t, gRookie.Pillars)
	})
}

func TestEngineV2SlipCopulaScriptTaxAndConflicts(t *testing.T) {
	t.Parallel()

	v2 := analyzer.EngineV2
	statsMap := richV2Stats()

	t.Run("positive shootout stack yields CorrMultiplier > 1 and shootout factor", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		g.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}: {
				Line:          new(255.5),
				FairProb:      0.55,
				PowerFairProb: 0.55,
				Source:        "Pinnacle",
				Tier:          1,
			},
			{MarketType: "REC_YDS", Team: "BUF", PlayerName: "khalil shakir", Side: "OVER"}: {
				Line:          new(68.5),
				FairProb:      0.55,
				PowerFairProb: 0.55,
				Source:        "Pinnacle",
				Tier:          1,
			},
			{MarketType: "TOTAL", Side: "OVER"}: {
				Line:          new(49.5),
				FairProb:      0.55,
				PowerFairProb: 0.55,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		legs := []analyzer.Leg{
			withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), +100), "allen"),
			withPlayerID(
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(68.5), +100),
				"shakir",
			),
			leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(49.5), +100),
		}

		eval := v2.EvaluateSlip(legs, map[string]analyzer.Game{"buf-mia": g}, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)
		assertFriendlyV2Tone(t, eval.Grade.Summary)

		assert.Greater(t, eval.Grade.CorrMultiplier, 1.05)
		assert.Greater(t, eval.Grade.CalibratedProb, 0.0)
		assert.GreaterOrEqual(t, p[analyzer.PillarScriptFit].Score, 82)
		assert.Equal(t, analyzer.ToneGood, p[analyzer.PillarScriptFit].Tone)
		assert.Contains(t, factorRules(eval.Grade.Factors), "stack_shootout")
	})

	t.Run("heavily taxed SGP stack keeps Script Fit green while flagging Price & Vig", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		g.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}: {
				Line:          new(265.5),
				FairProb:      0.50,
				PowerFairProb: 0.50,
				Source:        "Pinnacle",
				Tier:          1,
			},
			{MarketType: "REC_YDS", Team: "BUF", PlayerName: "khalil shakir", Side: "OVER"}: {
				Line:          new(72.5),
				FairProb:      0.50,
				PowerFairProb: 0.50,
				Source:        "Pinnacle",
				Tier:          1,
			},
			{MarketType: "TOTAL", Side: "OVER"}: {
				Line:          new(49.5),
				FairProb:      0.50,
				PowerFairProb: 0.50,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		// Heavily juiced leg prices (-175 each) tax the correlated stack.
		legs := []analyzer.Leg{
			withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -175), "allen"),
			withPlayerID(
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(72.5), -175),
				"shakir",
			),
			leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(49.5), -175),
		}

		eval := v2.EvaluateSlip(legs, map[string]analyzer.Game{"buf-mia": g}, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)

		assert.Greater(t, eval.Grade.CorrMultiplier, 1.05)
		assert.Equal(t, analyzer.ToneGood, p[analyzer.PillarScriptFit].Tone)
		assert.Less(t, p[analyzer.PillarPriceVig].Score, 68)
		assert.Contains(t, factorRules(eval.Grade.Factors), "sgp_tax")
		assert.Contains(t, eval.Grade.Summary, "tax the correlation")
	})

	t.Run("opposing same-game legs produce copula CorrMultiplier < 0.90", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		legs := []analyzer.Leg{
			leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(49.5), -110),
			withPlayerID(leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110), "allen"),
		}

		eval := v2.EvaluateSlip(legs, map[string]analyzer.Game{"buf-mia": g}, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)

		assert.Less(t, eval.Grade.CorrMultiplier, 0.90)
		assert.Less(t, p[analyzer.PillarScriptFit].Score, 68)
		assert.Contains(t, factorRules(eval.Grade.Factors), "script_fit")
	})

	t.Run("conflicting script triggers Danger cap and low Script Fit", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		legs := []analyzer.Leg{
			leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(49.5), -110),
			withPlayerID(leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110), "allen"),
			withPlayerID(
				leg(3, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(72.5), -110),
				"shakir",
			),
		}

		eval := v2.EvaluateSlip(legs, map[string]analyzer.Game{"buf-mia": g}, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)

		assert.Less(t, eval.Grade.CorrMultiplier, 1.0)
		assert.LessOrEqual(t, p[analyzer.PillarScriptFit].Score, 44)
		assert.Equal(t, analyzer.ToneBad, p[analyzer.PillarScriptFit].Tone)
		assert.LessOrEqual(t, eval.Grade.Score, 72)
		assert.NotEqual(t, "A", eval.Grade.Letter)
		assert.NotEqual(t, "B", eval.Grade.Letter)
		assert.Contains(t, factorRules(eval.Grade.Factors), "total_contradiction")
	})

	t.Run("cross-game legs remain independent with CorrMultiplier 1.0", func(t *testing.T) {
		t.Parallel()
		g1 := richV2Game()
		g2 := analyzer.Game{
			ID:   "dal-phi",
			Home: "PHI",
			Away: "DAL",
			MarketScript: analyzer.MarketScript{
				HomeSpread:       -2.5,
				GameTotal:        46.5,
				HomeImpliedTotal: 24.5,
				AwayImpliedTotal: 22.0,
				HasSpread:        true,
				HasTotal:         true,
			},
		}
		legs := []analyzer.Leg{
			withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110), "allen"),
			leg(2, "Adam", "dal-phi", "SPREAD", "PHI", "", "TEAM", new(-2.5), -110),
		}

		eval := v2.EvaluateSlip(legs, map[string]analyzer.Game{"buf-mia": g1, "dal-phi": g2}, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)
		assert.InDelta(t, 1.0, eval.Grade.CorrMultiplier, 1e-9)
		assert.Equal(t, 75, p[analyzer.PillarScriptFit].Score)
		assert.Contains(t, p[analyzer.PillarScriptFit].Summary, "separate games")
	})

	t.Run("five-leg slip and extra chalk apply parlay_drag and chalk_drag", func(t *testing.T) {
		t.Parallel()
		g1 := richV2Game()
		g2 := analyzer.Game{ID: "dal-phi", Home: "PHI", Away: "DAL"}
		g3 := analyzer.Game{ID: "kc-den", Home: "DEN", Away: "KC"}
		g4 := analyzer.Game{ID: "sf-sea", Home: "SEA", Away: "SF"}
		g5 := analyzer.Game{ID: "bal-cin", Home: "CIN", Away: "BAL"}

		legs := []analyzer.Leg{
			leg(1, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -300),
			leg(2, "Adam", "dal-phi", "ML", "PHI", "", "TEAM", nil, -280),
			leg(3, "Adam", "kc-den", "ML", "KC", "", "TEAM", nil, -110),
			leg(4, "Adam", "sf-sea", "SPREAD", "SF", "", "TEAM", new(-3.0), -110),
			leg(5, "Adam", "bal-cin", "TOTAL", "", "", "OVER", new(45.5), -110),
		}
		gamesMap := map[string]analyzer.Game{
			"buf-mia": g1, "dal-phi": g2, "kc-den": g3, "sf-sea": g4, "bal-cin": g5,
		}

		eval := v2.EvaluateSlip(legs, gamesMap, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)
		assert.Contains(t, factorRules(eval.Grade.Factors), "parlay_drag")
		assert.Contains(t, factorRules(eval.Grade.Factors), "chalk_drag")
		assert.Contains(t, strings.Join(p[analyzer.PillarPriceVig].Reasons, " "), "5 legs of book juice")
	})

	t.Run("leading ungraded OTHER leg does not shift Script Fit pair labels", func(t *testing.T) {
		t.Parallel()
		g := richV2Game()
		legs := []analyzer.Leg{
			leg(1, "Adam", "", "OTHER", "", "Coin Toss Heads", "OTHER", nil, -105),
			withPlayerID(
				leg(2, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
				"allen",
			),
			withPlayerID(
				leg(3, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(62.5), -110),
				"shakir",
			),
		}

		eval := v2.EvaluateSlip(legs, map[string]analyzer.Game{"buf-mia": g}, statsMap)
		p := assertValidFourPillars(t, eval.Grade.Pillars)

		assert.Greater(t, eval.Grade.CorrMultiplier, 1.05)
		reasonsText := strings.Join(p[analyzer.PillarScriptFit].Reasons, " ")
		assert.Contains(t, reasonsText, "Josh Allen")
		assert.Contains(t, reasonsText, "Khalil Shakir")
		assert.NotContains(t, reasonsText, "Coin Toss Heads")
	})
}

//nolint:paralleltest // Wall-clock latency budget test must not contend with parallel tests for CPU.
func TestEngineV2FullSlateLatencyBudget(t *testing.T) {
	gamesMap, statsMap, legs := fullSlateV2Fixture(16, 12)
	v2 := analyzer.EngineV2

	// Warm up lazy embedded artifact load once before timing.
	_ = v2.ScoreLeg(legs[0], gamesMap, statsMap)

	start := time.Now()
	aCount := 0
	for _, l := range legs {
		lg := v2.ScoreLeg(l, gamesMap, statsMap)
		if lg.Letter == "A" {
			aCount++
		}
	}
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, aCount, 0)
	// Design budget is < 15ms; enforce a 100ms ceiling on normal builds while
	// skipping the wall-clock check under -race instrumentation + parallel load
	// (the loop above still executes under -race to verify race-freedom).
	if !raceDetectorEnabled {
		assert.Less(t, elapsed, 100*time.Millisecond)
	}
}

func factorRules(factors []analyzer.GradeFactor) []string {
	out := make([]string, len(factors))
	for i, f := range factors {
		out[i] = f.Rule
	}
	return out
}

func fullSlateV2Fixture(numGames, playersPerGame int) (
	map[string]analyzer.Game,
	map[string]analyzer.PlayerStats,
	[]analyzer.Leg,
) {
	gamesMap := make(map[string]analyzer.Game, numGames)
	statsMap := make(map[string]analyzer.PlayerStats, numGames*playersPerGame)
	legs := make([]analyzer.Leg, 0, numGames*(6+playersPerGame*8))

	var legID int64
	for gIdx := range numGames {
		gid := fmt.Sprintf("g%02d", gIdx)
		home := fmt.Sprintf("H%02d", gIdx)
		away := fmt.Sprintf("A%02d", gIdx)
		gamesMap[gid] = analyzer.Game{
			ID:   gid,
			Home: home,
			Away: away,
			MarketScript: analyzer.MarketScript{
				HomeSpread:       -3.5,
				GameTotal:        46.5,
				HomeImpliedTotal: 25.0,
				AwayImpliedTotal: 21.5,
				HasSpread:        true,
				HasTotal:         true,
			},
			HomeEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowed:     0.02,
				DefRushEPAAllowed:     -0.01,
				PlaysPerGame:          62.0,
				PassRate:              0.56,
				DefPassEPAAllowedRank: 16 + (gIdx % 16),
				DefRushEPAAllowedRank: 15 + (gIdx % 16),
			},
			AwayEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowed:     0.03,
				DefRushEPAAllowed:     0.00,
				PlaysPerGame:          63.0,
				PassRate:              0.58,
				DefPassEPAAllowedRank: 17 + (gIdx % 16),
				DefRushEPAAllowedRank: 14 + (gIdx % 16),
			},
			Nfelo: &analyzer.Nfelo{
				HomeSpread:  -3.5,
				HomeWinProb: 0.60,
			},
		}

		legID++
		legs = append(legs, analyzer.Leg{
			ID: legID, GameID: gid, Market: "SPREAD", Team: home, Side: "TEAM", Line: new(-3.5), Odds: -110,
		})
		legID++
		legs = append(legs, analyzer.Leg{
			ID: legID, GameID: gid, Market: "SPREAD", Team: away, Side: "TEAM", Line: new(3.5), Odds: -110,
		})
		legID++
		legs = append(legs, analyzer.Leg{
			ID: legID, GameID: gid, Market: "TOTAL", Side: "OVER", Line: new(46.5), Odds: -110,
		})
		legID++
		legs = append(legs, analyzer.Leg{
			ID: legID, GameID: gid, Market: "TOTAL", Side: "UNDER", Line: new(46.5), Odds: -110,
		})
		legID++
		legs = append(legs, analyzer.Leg{
			ID: legID, GameID: gid, Market: "ML", Team: home, Side: "TEAM", Odds: -165,
		})
		legID++
		legs = append(legs, analyzer.Leg{
			ID: legID, GameID: gid, Market: "ML", Team: away, Side: "TEAM", Odds: 140,
		})

		for pIdx := range playersPerGame {
			pid := fmt.Sprintf("%s_p%02d", gid, pIdx)
			team := home
			if pIdx >= playersPerGame/2 {
				team = away
			}
			pos := "WR"
			role := analyzer.RoleWR1
			markets := []string{"REC_YDS", "RECEPTIONS", "ANYTIME_TD", "RUSH_YDS"}
			if pIdx%6 == 0 {
				pos = "QB"
				role = analyzer.RoleQB
				markets = []string{"PASS_YDS", "PASS_TDS", "RUSH_YDS", "ANYTIME_TD"}
			}
			statsMap[pid] = analyzer.PlayerStats{
				Position: pos,
				Role:     role,
				Games: map[string][]float64{
					"PASS_YDS":   {270, 265, 255, 250, 245},
					"PASS_TDS":   {2, 2, 2, 1, 2},
					"REC_YDS":    {75, 72, 68, 65, 60},
					"RECEPTIONS": {6, 6, 5, 5, 5},
					"RUSH_YDS":   {65, 62, 58, 55, 50},
				},
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 6,
					OffensePct:  0.82,
					TargetShare: 0.22,
					PassAtt:     34.0,
					PassYds:     262.0,
					PassTDs:     1.8,
					RecYds:      70.0,
					Receptions:  5.5,
					RushYds:     60.0,
					TotalEPA:    4.5,
				},
			}
			for _, m := range markets {
				var linePtr *float64
				if m != "ANYTIME_TD" {
					linePtr = new(55.5)
				}
				legID++
				legs = append(legs, analyzer.Leg{
					ID: legID, GameID: gid, Market: m, Team: team,
					PlayerID: pid, PlayerName: "Player " + pid,
					Side: "OVER", Line: linePtr, Odds: -110,
				})
				legID++
				legs = append(legs, analyzer.Leg{
					ID: legID, GameID: gid, Market: m, Team: team,
					PlayerID: pid, PlayerName: "Player " + pid,
					Side: "UNDER", Line: linePtr, Odds: -110,
				})
			}
		}
	}
	return gamesMap, statsMap, legs
}

func BenchmarkEngineV2ScoreLeg(b *testing.B) {
	gamesMap := map[string]analyzer.Game{"buf-mia": richV2Game()}
	statsMap := richV2Stats()
	propLeg := withPlayerID(
		leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
		"allen",
	)
	v2 := analyzer.EngineV2
	_ = v2.ScoreLeg(propLeg, gamesMap, statsMap)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = v2.ScoreLeg(propLeg, gamesMap, statsMap)
	}
}

func BenchmarkEngineV2FullSlateHighlights(b *testing.B) {
	gamesMap, statsMap, legs := fullSlateV2Fixture(16, 12)
	v2 := analyzer.EngineV2
	_ = v2.ScoreLeg(legs[0], gamesMap, statsMap)

	topParlay := []analyzer.Leg{legs[6], legs[14], legs[2]}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, l := range legs {
			_ = v2.ScoreLeg(l, gamesMap, statsMap)
		}
		_ = v2.EvaluateSlip(topParlay, gamesMap, statsMap)
	}
}

func BenchmarkEngineV2Explorer(b *testing.B) {
	gamesMap := map[string]analyzer.Game{"buf-mia": richV2Game()}
	statsMap := richV2Stats()

	explorerLegs := make([]analyzer.Leg, 0, 120)
	var id int64
	for _, line := range []float64{215.5, 225.5, 235.5, 245.5, 255.5, 265.5, 275.5, 285.5, 295.5, 305.5} {
		for _, side := range []string{"OVER", "UNDER"} {
			id++
			explorerLegs = append(explorerLegs, withPlayerID(
				leg(id, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", side, new(line), -110),
				"allen",
			))
		}
	}
	for _, line := range []float64{45.5, 55.5, 65.5, 75.5, 85.5, 95.5, 105.5, 115.5} {
		for _, side := range []string{"OVER", "UNDER"} {
			id++
			explorerLegs = append(explorerLegs, withPlayerID(
				leg(id, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", side, new(line), -110),
				"shakir",
			))
			id++
			explorerLegs = append(explorerLegs, withPlayerID(
				leg(id, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", side, new(line), -110),
				"cook",
			))
		}
	}

	v2 := analyzer.EngineV2
	_ = v2.ScoreLeg(explorerLegs[0], gamesMap, statsMap)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, l := range explorerLegs {
			_ = v2.ScoreLeg(l, gamesMap, statsMap)
		}
	}
}
