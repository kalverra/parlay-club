package analyzer_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/modelv2"
)

func assertFriendlyOverlayTone(t *testing.T, findings []analyzer.OverlayFinding) {
	t.Helper()
	for _, f := range findings {
		for _, text := range []string{f.Label, f.Detail} {
			lower := strings.ToLower(text)
			assert.NotContains(t, lower, "+ev")
			assert.NotContains(t, lower, "expected value")
			assert.NotContains(t, lower, "roi")
			for tok := range strings.FieldsSeq(lower) {
				trimmed := strings.Trim(tok, ".,:;!?()[]\"'")
				assert.NotEqual(t, "ev", trimmed, "unexpected EV jargon in %q", text)
			}
		}
	}
}

func findOverlayRule(findings []analyzer.OverlayFinding, rule string) (analyzer.OverlayFinding, bool) {
	for _, f := range findings {
		if f.Rule == rule {
			return f, true
		}
	}
	return analyzer.OverlayFinding{}, false
}

func TestEvaluateOverlaysSkipsUngradableLegs(t *testing.T) {
	t.Parallel()

	g := analyzer.Game{
		ID:      "buf-mia",
		Home:    "MIA",
		Away:    "BUF",
		Weather: &analyzer.Weather{WindMPH: 25},
	}

	assert.Empty(t, analyzer.EvaluateOverlays(
		leg(1, "Adam", "buf-mia", "OTHER", "BUF", "", "", nil, -110),
		g,
		nil,
		nil,
	))
	assert.Empty(t, analyzer.EvaluateOverlays(
		leg(2, "Adam", "", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
		g,
		nil,
		nil,
	))
}

func TestEvaluateOverlaysRetainedNonBoxScoreRules(t *testing.T) {
	t.Parallel()

	t.Run("player_injury_status covers Out, Doubtful, and Questionable with uncertainty", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name     string
			side     string
			inj      analyzer.Injury
			playerID string
			wantSev  analyzer.Severity
			wantDel  int
		}{
			{
				name:     "player Out on Over",
				side:     "OVER",
				inj:      analyzer.Injury{AthleteID: "p1", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
				playerID: "p1",
				wantSev:  analyzer.Danger,
				wantDel:  -20,
			},
			{
				name:     "player Out on Under warns of void risk",
				side:     "UNDER",
				inj:      analyzer.Injury{AthleteID: "p1", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
				playerID: "p1",
				wantSev:  analyzer.Warn,
				wantDel:  -10,
			},
			{
				name: "player Doubtful on Over",
				side: "OVER",
				inj: analyzer.Injury{
					AthleteID: "p1",
					FullName:  "Dalton Kincaid",
					Position:  "TE",
					Status:    "Doubtful",
				},
				playerID: "p1",
				wantSev:  analyzer.Danger,
				wantDel:  -15,
			},
			{
				name: "player Questionable matched by normalized name",
				side: "OVER",
				inj: analyzer.Injury{
					FullName: "Marvin Harrison Jr.",
					Position: "WR",
					Status:   "Questionable",
				},
				playerID: "",
				wantSev:  analyzer.Warn,
				wantDel:  -6,
			},
			{
				name: "player Questionable on Under sets uncertainty with zero delta",
				side: "UNDER",
				inj: analyzer.Injury{
					AthleteID: "p1",
					FullName:  "Dalton Kincaid",
					Position:  "TE",
					Status:    "Questionable",
				},
				playerID: "p1",
				wantSev:  analyzer.Info,
				wantDel:  0,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				name := "Dalton Kincaid"
				if tc.playerID == "" {
					name = "Marvin Harrison"
				}
				l := withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", name, tc.side, new(45.5), -110),
					tc.playerID,
				)
				g := analyzer.Game{
					ID:       "buf-mia",
					Home:     "MIA",
					Away:     "BUF",
					Injuries: map[string][]analyzer.Injury{"BUF": {tc.inj}},
				}
				findings := analyzer.EvaluateOverlays(l, g, nil, nil)
				f, ok := findOverlayRule(findings, "player_injury_status")
				require.True(t, ok)
				assert.Equal(t, tc.wantSev, f.Severity)
				assert.Equal(t, tc.wantDel, f.ScoreDelta)
				assert.True(t, f.HeightenedUncertainty)
				assert.Equal(t, []int64{1}, f.LegIDs)
				assertFriendlyOverlayTone(t, findings)
			})
		}
	})

	t.Run("teammate_out_bump sets HeightenedUncertainty and falls back to Role", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Dawson Knox", "OVER", new(24.5), -110), "knox")
		g := analyzer.Game{
			ID:   "buf-mia",
			Home: "MIA",
			Away: "BUF",
			Injuries: map[string][]analyzer.Injury{
				"BUF": {{AthleteID: "kincaid", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"}},
			},
		}
		stats := map[string]analyzer.PlayerStats{
			"knox": {Role: analyzer.RoleTE1},
		}
		findings := analyzer.EvaluateOverlays(l, g, stats, nil)
		f, ok := findOverlayRule(findings, "teammate_out_bump")
		require.True(t, ok)
		assert.Equal(t, 5, f.ScoreDelta)
		assert.True(t, f.HeightenedUncertainty)
		assert.Contains(t, f.Detail, "Dalton Kincaid")
		assertFriendlyOverlayTone(t, findings)
	})

	t.Run("qb_drag, wind, fake_weather, hook, total_key_number, shrinking_role, td_spike_risk", func(t *testing.T) {
		t.Parallel()

		// QB drag + danger wind + shrinking role.
		qbLeg := withPlayerID(
			leg(3, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
			"allen",
		)
		qbGame := analyzer.Game{
			ID:      "buf-mia",
			Home:    "BUF",
			Away:    "MIA",
			Weather: &analyzer.Weather{WindMPH: 22},
			Nfelo:   &analyzer.Nfelo{HomeQB: "Josh Allen", HomeQBAdj: -40},
		}
		qbStats := map[string]analyzer.PlayerStats{
			"allen": {
				Usage: &analyzer.PlayerUsage{
					SeasonSnapAvg: 0.90,
					Weeks: []analyzer.PlayerWeekUsage{
						{OffensePct: 0.70}, {OffensePct: 0.70}, {OffensePct: 0.95},
					},
				},
			},
		}
		qbFindings := analyzer.EvaluateOverlays(qbLeg, qbGame, qbStats, nil)
		qbDrag, ok := findOverlayRule(qbFindings, "qb_drag")
		require.True(t, ok)
		assert.Equal(t, -8, qbDrag.ScoreDelta)

		windF, ok := findOverlayRule(qbFindings, "wind")
		require.True(t, ok)
		assert.Equal(t, analyzer.Danger, windF.Severity)
		assert.Equal(t, -18, windF.ScoreDelta)

		shrinkF, ok := findOverlayRule(qbFindings, "shrinking_role")
		require.True(t, ok)
		assert.Equal(t, analyzer.Warn, shrinkF.Severity)
		assert.Equal(t, -15, shrinkF.ScoreDelta)
		assert.True(t, shrinkF.HeightenedUncertainty)
		assertFriendlyOverlayTone(t, qbFindings)

		// Fake weather + total key number.
		totalLeg := leg(4, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(43.5), -110)
		rainGame := analyzer.Game{
			ID:      "buf-mia",
			Home:    "BUF",
			Away:    "MIA",
			Weather: &analyzer.Weather{PrecipIn: 0.05, WindMPH: 4},
		}
		totalFindings := analyzer.EvaluateOverlays(totalLeg, rainGame, nil, nil)
		fakeW, ok := findOverlayRule(totalFindings, "fake_weather")
		require.True(t, ok)
		assert.Equal(t, -3, fakeW.ScoreDelta)
		keyF, ok := findOverlayRule(totalFindings, "total_key_number")
		require.True(t, ok)
		assert.Equal(t, -5, keyF.ScoreDelta)

		// Spread hook softened by nfelo cushion.
		spreadLeg := leg(5, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110)
		hookGame := analyzer.Game{
			ID:    "buf-mia",
			Home:  "MIA",
			Away:  "BUF",
			Nfelo: &analyzer.Nfelo{HomeSpread: 5.5},
		}
		hookFindings := analyzer.EvaluateOverlays(spreadLeg, hookGame, nil, nil)
		hookF, ok := findOverlayRule(hookFindings, "hook")
		require.True(t, ok)
		assert.Equal(t, -4, hookF.ScoreDelta)
		assert.Contains(t, hookF.Detail, "nfelo has it nearer -5.5")

		// TD spike risk.
		tdLeg := withPlayerID(leg(6, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "Dawson Knox", "YES", nil, +240), "knox")
		tdStats := map[string]analyzer.PlayerStats{
			"knox": {
				Games: map[string][]float64{"ANYTIME_TD": {1, 1, 1, 0}},
				Usage: &analyzer.PlayerUsage{
					Last3TargetShare: 0.10,
					Weeks: []analyzer.PlayerWeekUsage{
						{Carries: 0}, {Carries: 1}, {Carries: 0},
					},
				},
			},
		}
		tdFindings := analyzer.EvaluateOverlays(
			tdLeg,
			analyzer.Game{ID: "buf-mia", Home: "MIA", Away: "BUF"},
			tdStats,
			nil,
		)
		tdF, ok := findOverlayRule(tdFindings, "td_spike_risk")
		require.True(t, ok)
		assert.Equal(t, analyzer.Info, tdF.Severity)
		assert.Equal(t, -6, tdF.ScoreDelta)
		assertFriendlyOverlayTone(t, tdFindings)
	})
}

func TestNarrativeStreakTrapOverlay(t *testing.T) {
	t.Parallel()

	g := analyzer.Game{ID: "buf-mia", Home: "MIA", Away: "BUF"}

	t.Run("fires when 5/5 against line yet calibrated prob < 0.50", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(58.5), -110), "p1")
		stats := map[string]analyzer.PlayerStats{
			"p1": {Games: map[string][]float64{"REC_YDS": {62, 65, 60, 71, 64}}},
		}
		pred := &modelv2.LegPrediction{ModelProb: 0.44, CalibratedProb: 0.47}
		findings := analyzer.EvaluateOverlays(l, g, stats, pred)
		f, ok := findOverlayRule(findings, "narrative_streak_trap")
		require.True(t, ok)
		assert.Equal(t, analyzer.Warn, f.Severity)
		assert.Equal(t, -8, f.ScoreDelta)
		assert.Contains(t, f.Detail, "5 straight")
		assertFriendlyOverlayTone(t, findings)
	})

	t.Run("stays quiet when 5/5 against line and fair prob >= 0.50", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(58.5), -110), "p1")
		stats := map[string]analyzer.PlayerStats{
			"p1": {Games: map[string][]float64{"REC_YDS": {62, 65, 60, 71, 64}}},
		}
		pred := &modelv2.LegPrediction{ModelProb: 0.54, CalibratedProb: 0.53}
		findings := analyzer.EvaluateOverlays(l, g, stats, pred)
		_, ok := findOverlayRule(findings, "narrative_streak_trap")
		assert.False(t, ok)
	})

	t.Run(
		"fires when 5 games beat prior baseline and line pushed materially above L5 mean and median",
		func(t *testing.T) {
			t.Parallel()
			l := withPlayerID(
				leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(74.5), -110),
				"p1",
			)
			// Prior games median = 45; L5 = {64, 66, 62, 68, 65} (all > 45, mean = 65, median = 65).
			stats := map[string]analyzer.PlayerStats{
				"p1": {Games: map[string][]float64{"REC_YDS": {64, 66, 62, 68, 65, 40, 45, 48}}},
			}
			findings := analyzer.EvaluateOverlays(l, g, stats, nil)
			f, ok := findOverlayRule(findings, "narrative_streak_trap")
			require.True(t, ok)
			assert.Equal(t, analyzer.Warn, f.Severity)
			assert.Equal(t, -8, f.ScoreDelta)
			assert.Contains(t, f.Detail, "74.5")
			assertFriendlyOverlayTone(t, findings)
		},
	)

	t.Run("does not fire without consistent 5-game streak", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(3, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(74.5), -110), "p1")
		stats := map[string]analyzer.PlayerStats{
			"p1": {Games: map[string][]float64{"REC_YDS": {80, 75, 20, 70, 68, 40, 45, 48}}},
		}
		findings := analyzer.EvaluateOverlays(l, g, stats, nil)
		_, ok := findOverlayRule(findings, "narrative_streak_trap")
		assert.False(t, ok)
	})
}

func TestModelMarketDivergenceOverlay(t *testing.T) {
	t.Parallel()

	l := withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110), "allen")
	g := analyzer.Game{ID: "buf-mia", Home: "BUF", Away: "MIA"}

	t.Run("triggers at 0.08 gap in either direction with uncertainty", func(t *testing.T) {
		t.Parallel()
		for _, pred := range []modelv2.LegPrediction{
			{ModelProb: 0.60, MarketProb: 0.52},
			{ModelProb: 0.42, MarketProb: 0.50},
		} {
			p := pred
			findings := analyzer.EvaluateOverlays(l, g, nil, &p)
			f, ok := findOverlayRule(findings, "model_market_divergence")
			require.True(t, ok)
			assert.Equal(t, analyzer.Warn, f.Severity)
			assert.True(t, f.HeightenedUncertainty)
			assert.Contains(t, f.Detail, "Model and market see this differently")
			assertFriendlyOverlayTone(t, findings)
		}
	})

	t.Run("stays quiet below 0.08 or without market probability", func(t *testing.T) {
		t.Parallel()
		small := modelv2.LegPrediction{ModelProb: 0.579, MarketProb: 0.50}
		assert.Empty(t, analyzer.EvaluateOverlays(l, g, nil, &small))

		noMarket := modelv2.LegPrediction{ModelProb: 0.72, MarketProb: 0}
		assert.Empty(t, analyzer.EvaluateOverlays(l, g, nil, &noMarket))
	})
}

func TestExternalContextSignalsAndPluggability(t *testing.T) {
	t.Parallel()

	l := withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(58.5), -110), "shakir")
	dupSig := analyzer.ContextSignal{
		Kind:                  "news",
		Team:                  "BUF",
		PlayerID:              "shakir",
		Label:                 "Limited practice",
		Detail:                "Logged limited Friday session on ankle.",
		Severity:              analyzer.Warn,
		ScoreDelta:            -7,
		HeightenedUncertainty: true,
	}
	g := analyzer.Game{
		ID:   "buf-mia",
		Home: "MIA",
		Away: "BUF",
		ContextSignals: []analyzer.ContextSignal{
			dupSig,
			{
				Kind:       "line_move",
				Team:       "BUF",
				Detail:     "BUF team total steamed up 1.5 points.",
				Severity:   analyzer.Info,
				ScoreDelta: 3,
			},
			{
				// Game-wide signal (empty Team and PlayerID) applies to any leg.
				Kind:       "news",
				Label:      "Wind gust alert",
				Detail:     "Pre-game gusts picking up near kickoff.",
				Severity:   analyzer.Info,
				ScoreDelta: -2,
			},
			{
				// Unrelated player signal must not apply.
				Kind:       "inactive",
				Team:       "BUF",
				PlayerID:   "other",
				Detail:     "Other player inactive.",
				Severity:   analyzer.Danger,
				ScoreDelta: -20,
			},
			{
				// Unrelated team signal must not apply.
				Kind:       "line_move",
				Team:       "MIA",
				Detail:     "MIA move.",
				Severity:   analyzer.Warn,
				ScoreDelta: -10,
			},
		},
	}
	stats := map[string]analyzer.PlayerStats{
		"shakir": {
			ContextSignals: []analyzer.ContextSignal{dupSig},
		},
	}

	findings := analyzer.EvaluateOverlays(l, g, stats, nil)
	require.Len(t, findings, 3)
	assert.True(t, analyzer.HasHeightenedUncertainty(findings))
	assert.InDelta(t, analyzer.HeightenedUncertaintyBlendBoost, analyzer.UncertaintyBoost(findings), 1e-12)
	// Sorted Danger -> Warn -> Info: Limited practice (-7, Warn) first.
	assert.Equal(t, "external_context_signals", findings[0].Rule)
	assert.Equal(t, "Limited practice", findings[0].Label)
	assert.Equal(t, -7, findings[0].ScoreDelta)
	assertFriendlyOverlayTone(t, findings)

	// Custom ContextOverlay plugs in cleanly without modifying defaults.
	customOverlay := func(
		leg analyzer.Leg,
		_ analyzer.Game,
		_ map[string]analyzer.PlayerStats,
		_ *modelv2.LegPrediction,
	) []analyzer.OverlayFinding {
		return []analyzer.OverlayFinding{{
			Rule:       "custom_beat_note",
			Label:      "Custom note",
			Detail:     "Custom overlay active.",
			Severity:   analyzer.Info,
			ScoreDelta: 2,
			LegIDs:     []int64{leg.ID},
		}}
	}
	customFindings := analyzer.EvaluateOverlays(l, g, stats, nil, customOverlay)
	require.Len(t, customFindings, 1)
	assert.Equal(t, "custom_beat_note", customFindings[0].Rule)
}

func TestPredictLegWithOverlaysHeightenedUncertaintyBlending(t *testing.T) {
	t.Parallel()

	bundle, err := modelv2.LoadEmbedded()
	require.NoError(t, err)

	passModel, ok := bundle.PropModel(modelv2.MarketPassYds)
	require.True(t, ok)
	recModel, ok := bundle.PropModel(modelv2.MarketRecYds)
	require.True(t, ok)

	passFeats := modelv2.AlignFeatures(passModel.FeatureNames, map[string]float64{
		modelv2.FeatEWMAPassYds:      275.0,
		modelv2.FeatTeamImpliedTotal: 26.5,
	})
	recFeats := modelv2.AlignFeatures(recModel.FeatureNames, map[string]float64{
		modelv2.FeatEWMARecYds:       58.0,
		modelv2.FeatTeamImpliedTotal: 25.0,
	})

	baseGame := analyzer.Game{ID: "buf-mia", Home: "MIA", Away: "BUF"}

	t.Run("clean context uses baseline blend weight", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(245.5), -110), "allen")
		// Get raw model prob first to choose a nearby market prob (< 0.08 gap).
		probe, err := passModel.PredictLeg(passFeats, "OVER", 245.5, 0, 0)
		require.NoError(t, err)
		marketProb := math.Min(0.95, math.Max(0.05, probe.ModelProb-0.03))

		pred, findings, err := analyzer.PredictLegWithOverlays(passModel, passFeats, l, baseGame, nil, marketProb)
		require.NoError(t, err)
		assert.False(t, analyzer.HasHeightenedUncertainty(findings))
		assert.InDelta(t, passModel.MarketBlendWeight, pred.BlendWeight, 1e-12)
	})

	t.Run("teammate_out_bump shifts blend weight toward market", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Dawson Knox", "OVER", new(24.5), -110), "knox")
		probe, err := recModel.PredictLeg(recFeats, "OVER", 24.5, 0, 0)
		require.NoError(t, err)
		marketProb := math.Min(0.95, math.Max(0.05, probe.ModelProb-0.04))

		cleanPred, _, err := analyzer.PredictLegWithOverlays(recModel, recFeats, l, baseGame, nil, marketProb)
		require.NoError(t, err)

		teammateOutGame := analyzer.Game{
			ID:   "buf-mia",
			Home: "MIA",
			Away: "BUF",
			Injuries: map[string][]analyzer.Injury{
				"BUF": {{AthleteID: "kincaid", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"}},
			},
		}
		stats := map[string]analyzer.PlayerStats{
			"knox": {Position: "TE"},
		}
		uncertainPred, findings, err := analyzer.PredictLegWithOverlays(
			recModel,
			recFeats,
			l,
			teammateOutGame,
			stats,
			marketProb,
		)
		require.NoError(t, err)
		assert.True(t, analyzer.HasHeightenedUncertainty(findings))
		wantWeight := math.Min(0.95, recModel.MarketBlendWeight+0.15)
		assert.InDelta(t, wantWeight, uncertainPred.BlendWeight, 1e-12)
		assert.Less(t, math.Abs(uncertainPred.BlendedProb-marketProb), math.Abs(cleanPred.BlendedProb-marketProb))
	})

	t.Run(
		"player injury, shrinking role, external signal, and model_market_divergence all trigger shift",
		func(t *testing.T) {
			t.Parallel()
			l := withPlayerID(
				leg(3, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(55.5), -110),
				"shakir",
			)
			probe, err := recModel.PredictLeg(recFeats, "OVER", 55.5, 0, 0)
			require.NoError(t, err)
			nearMarket := math.Min(0.95, math.Max(0.05, probe.ModelProb-0.03))
			wantWeight := math.Min(0.95, recModel.MarketBlendWeight+0.15)

			// 1. Player Questionable injury.
			injGame := analyzer.Game{
				ID:   "buf-mia",
				Home: "MIA",
				Away: "BUF",
				Injuries: map[string][]analyzer.Injury{
					"BUF": {{AthleteID: "shakir", FullName: "Khalil Shakir", Position: "WR", Status: "Questionable"}},
				},
			}
			injPred, injFindings, err := analyzer.PredictLegWithOverlays(
				recModel,
				recFeats,
				l,
				injGame,
				nil,
				nearMarket,
			)
			require.NoError(t, err)
			assert.True(t, analyzer.HasHeightenedUncertainty(injFindings))
			assert.InDelta(t, wantWeight, injPred.BlendWeight, 1e-12)

			// 2. Shrinking role.
			shrinkStats := map[string]analyzer.PlayerStats{
				"shakir": {
					Usage: &analyzer.PlayerUsage{
						SeasonSnapAvg: 0.85,
						Weeks: []analyzer.PlayerWeekUsage{
							{OffensePct: 0.65}, {OffensePct: 0.65}, {OffensePct: 0.90},
						},
					},
				},
			}
			shrinkPred, shrinkFindings, err := analyzer.PredictLegWithOverlays(
				recModel,
				recFeats,
				l,
				baseGame,
				shrinkStats,
				nearMarket,
			)
			require.NoError(t, err)
			assert.True(t, analyzer.HasHeightenedUncertainty(shrinkFindings))
			assert.InDelta(t, wantWeight, shrinkPred.BlendWeight, 1e-12)

			// 3. External ContextSignal.
			sigGame := analyzer.Game{
				ID:   "buf-mia",
				Home: "MIA",
				Away: "BUF",
				ContextSignals: []analyzer.ContextSignal{{
					Kind:                  "news",
					PlayerID:              "shakir",
					Detail:                "Snap count rumor.",
					Severity:              analyzer.Warn,
					HeightenedUncertainty: true,
				}},
			}
			sigPred, sigFindings, err := analyzer.PredictLegWithOverlays(
				recModel,
				recFeats,
				l,
				sigGame,
				nil,
				nearMarket,
			)
			require.NoError(t, err)
			assert.True(t, analyzer.HasHeightenedUncertainty(sigFindings))
			assert.InDelta(t, wantWeight, sigPred.BlendWeight, 1e-12)

			// 4. Model-market divergence >= 0.08 (second-pass blend shift).
			farMarket := 0.20
			if probe.ModelProb < 0.50 {
				farMarket = 0.80
			}
			divPred, divFindings, err := analyzer.PredictLegWithOverlays(
				recModel,
				recFeats,
				l,
				baseGame,
				nil,
				farMarket,
			)
			require.NoError(t, err)
			_, hasDiv := findOverlayRule(divFindings, "model_market_divergence")
			require.True(t, hasDiv)
			assert.True(t, analyzer.HasHeightenedUncertainty(divFindings))
			assert.InDelta(t, wantWeight, divPred.BlendWeight, 1e-12)
		},
	)
}
