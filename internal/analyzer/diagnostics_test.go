package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestBuildDiagnostics_ProvenanceAndUnblendedRawModelProb(t *testing.T) {
	t.Parallel()

	game := richV2Game()
	line := 255.5
	game.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
		{
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "josh allen",
			Side:       "OVER",
		}: {
			Line:          &line,
			FairProb:      0.51,
			PowerFairProb: 0.515,
			VigPct:        0.045,
			Source:        "Pinnacle",
			Tier:          1,
			BookCount:     1,
		},
	}

	leg := analyzer.Leg{
		ID:         101,
		GameID:     game.ID,
		Team:       "BUF",
		PlayerID:   "allen-1",
		PlayerName: "Josh Allen",
		Market:     "PASS_YDS",
		Side:       "OVER",
		Line:       &line,
		Odds:       -110,
	}
	stats := map[string]analyzer.PlayerStats{
		"allen-1": {
			Position: "QB",
			Role:     analyzer.RoleQB,
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 8,
				PassYds:     272.0,
				PassAtt:     35.0,
				OffensePct:  1.0,
			},
		},
	}
	games := map[string]analyzer.Game{game.ID: game}

	lg := analyzer.ScoreLeg(leg, games, stats)
	require.NotEmpty(t, lg.Letter)
	assert.InDelta(t, parlay.ImpliedProb(-110), lg.VegasImpliedProb, 1e-6)
	assert.InDelta(t, 0.515, lg.FairNoVigProb, 1e-6)
	assert.Greater(t, lg.RawModelProb, 0.0)
	assert.Less(t, lg.RawModelProb, 1.0)
	// RawModelProb is unblended and distinct from the market-blended CalibratedProb.
	assert.NotEqual(t, lg.RawModelProb, lg.CalibratedProb)

	assert.InDelta(t, lg.VegasImpliedProb, lg.Diagnostics.VegasImpliedProb, 1e-12)
	assert.InDelta(t, lg.FairNoVigProb, lg.Diagnostics.FairNoVigProb, 1e-12)
	assert.InDelta(t, lg.RawModelProb, lg.Diagnostics.RawModelProb, 1e-12)
	assert.NotEmpty(t, lg.Diagnostics.DisplayGrade)
	assert.NotEmpty(t, lg.Diagnostics.Descriptor)
	assert.LessOrEqual(t, len(lg.Diagnostics.SignalPills), 3)
	assert.LessOrEqual(t, len(lg.Diagnostics.ContextRadar), 2)
	for _, note := range lg.Diagnostics.ContextRadar {
		assert.Contains(t, []string{analyzer.TagAlreadyInLine, analyzer.TagLikelyPricedIn}, note.Tag)
	}
	assertFriendlyV2Tone(t, lg.Diagnostics.Descriptor, lg.Diagnostics.CleanupNudge)
}

func TestBuildDiagnostics_NoModelRanSetsRawModelProbZero(t *testing.T) {
	t.Parallel()

	game := analyzer.Game{
		ID:   "no-model",
		Home: "KC",
		Away: "LV",
	}
	leg := analyzer.Leg{
		ID:     1,
		GameID: game.ID,
		Team:   "KC",
		Market: "ML",
		Side:   "TEAM",
		Odds:   -150,
	}
	lg := analyzer.ScoreLeg(leg, map[string]analyzer.Game{game.ID: game}, nil)
	assert.InDelta(t, 0.0, lg.RawModelProb, 1e-12)
	assert.InDelta(t, 0.0, lg.Diagnostics.RawModelProb, 1e-12)
	assert.InDelta(t, parlay.ImpliedProb(-150), lg.VegasImpliedProb, 1e-6)
	assert.InDelta(t, 0.0, lg.FairNoVigProb, 1e-12)
}

func TestBuildDiagnostics_SignalPillsCappedAndOrderedBySeverity(t *testing.T) {
	t.Parallel()

	game := richV2Game()
	game.Weather = &analyzer.Weather{WindMPH: 24, TempF: 32}
	game.MarketScript = analyzer.MarketScript{
		HomeSpread:       -7.5,
		GameTotal:        49.5,
		HomeImpliedTotal: 28.5,
		AwayImpliedTotal: 21.0,
		HasSpread:        true,
		HasTotal:         true,
	}

	// Build a bloated 5-leg slip with a script conflict, heavy chalk, and bad prices.
	legs := []analyzer.Leg{
		{ID: 1, GameID: game.ID, Team: "MIA", Market: "SPREAD", Side: "TEAM", Line: new(-7.5), Odds: -450},
		{
			ID: 2, GameID: game.ID, Team: "MIA", PlayerID: "qb1", PlayerName: "Tua",
			Market: "PASS_YDS", Side: "OVER", Line: new(250.5), Odds: -145,
		},
		{
			ID: 3, GameID: game.ID, Team: "MIA", PlayerID: "rb1", PlayerName: "Achane",
			Market: "RUSH_YDS", Side: "UNDER", Line: new(65.5), Odds: -145,
		},
		{
			ID: 4, GameID: game.ID, Team: "BUF", PlayerID: "rb2", PlayerName: "Cook",
			Market: "RUSH_YDS", Side: "OVER", Line: new(70.5), Odds: -300,
		},
		{ID: 5, GameID: game.ID, Team: "MIA", Market: "TOTAL", Side: "UNDER", Line: new(49.5), Odds: -140},
	}
	stats := map[string]analyzer.PlayerStats{
		"rb2": {
			Position: "RB",
			Role:     analyzer.RoleRB1,
			EWMA:     &analyzer.PlayerEWMAUsage{GamesPlayed: 6, RushYds: 72, Carries: 16},
		},
	}

	ev := analyzer.EvaluateSlip(legs, map[string]analyzer.Game{game.ID: game}, stats)
	diag := ev.Grade.Diagnostics

	require.NotEmpty(t, diag.SignalPills)
	assert.LessOrEqual(t, len(diag.SignalPills), 3)
	assert.LessOrEqual(t, len(diag.ContextRadar), 2)

	// Verify severity ordering: bad (0) <= warn (1) <= info (2).
	rank := func(tone string) int {
		switch tone {
		case analyzer.ToneBad:
			return 0
		case analyzer.ToneWarn:
			return 1
		default:
			return 2
		}
	}
	for i := 1; i < len(diag.SignalPills); i++ {
		assert.LessOrEqual(t, rank(diag.SignalPills[i-1].Tone), rank(diag.SignalPills[i].Tone))
	}

	for _, note := range diag.ContextRadar {
		assert.Contains(t, []string{analyzer.TagAlreadyInLine, analyzer.TagLikelyPricedIn}, note.Tag)
		assert.NotEmpty(t, note.Label)
		assert.NotEmpty(t, note.PricedIn)
	}
}

func TestBuildDiagnostics_DescriptorsMatchDominantTraps(t *testing.T) {
	t.Parallel()

	t.Run("self sabotage on danger script conflict", func(t *testing.T) {
		t.Parallel()
		game := richV2Game()
		game.MarketScript.HomeSpread = -10.5
		legs := []analyzer.Leg{
			{ID: 1, GameID: game.ID, Team: "MIA", Market: "SPREAD", Side: "TEAM", Line: new(-10.5), Odds: -110},
			{
				ID: 2, GameID: game.ID, Team: "BUF", PlayerID: "rb1", PlayerName: "Cook",
				Market: "RUSH_YDS", Side: "OVER", Line: new(72.5), Odds: -110,
			},
		}
		ev := analyzer.EvaluateSlip(legs, map[string]analyzer.Game{game.ID: game}, nil)
		assert.Equal(t, "Self-Sabotage", ev.Grade.Diagnostics.Descriptor)
		assert.NotEmpty(t, ev.Grade.Diagnostics.CleanupNudge)
	})

	t.Run("vig trap on steep juice", func(t *testing.T) {
		t.Parallel()
		game := richV2Game()
		game.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "ML", Team: "BUF", Side: "TEAM"}: {
				FairProb:      0.52,
				PowerFairProb: 0.52,
				VigPct:        0.04,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		// Retail -200 (66.7% implied) vs 52% fair is a massive vig trap.
		leg := analyzer.Leg{ID: 1, GameID: game.ID, Team: "BUF", Market: "ML", Side: "TEAM", Odds: -200}
		lg := analyzer.ScoreLeg(leg, map[string]analyzer.Game{game.ID: game}, nil)
		assert.Equal(t, "Vig Trap", lg.Diagnostics.Descriptor)
		assert.Greater(t, lg.Diagnostics.HoldPct, 10.0)
	})

	t.Run("longshot on low probability leg", func(t *testing.T) {
		t.Parallel()
		game := richV2Game()
		game.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "ML", Team: "BUF", Side: "TEAM"}: {
				FairProb:      0.13,
				PowerFairProb: 0.13,
				VigPct:        0.05,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		leg := analyzer.Leg{
			ID:     1,
			GameID: game.ID,
			Team:   "BUF",
			Market: "ML",
			Side:   "TEAM",
			Odds:   700,
		}
		lg := analyzer.ScoreLeg(leg, map[string]analyzer.Game{game.ID: game}, nil)
		assert.Equal(t, "Longshot", lg.Diagnostics.Descriptor)
	})

	t.Run("clean or tight build on sharp A grade", func(t *testing.T) {
		t.Parallel()
		game := richV2Game()
		game.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}: {
				Line:          new(-3.5),
				FairProb:      0.57,
				PowerFairProb: 0.57,
				VigPct:        0.04,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		leg := analyzer.Leg{
			ID:     1,
			GameID: game.ID,
			Team:   "BUF",
			Market: "SPREAD",
			Side:   "TEAM",
			Line:   new(-3.5),
			Odds:   105,
		}
		lg := analyzer.ScoreLeg(leg, map[string]analyzer.Game{game.ID: game}, nil)
		require.Equal(t, "A", lg.Letter)
		assert.Contains(t, []string{"Clean Build", "Tight Build"}, lg.Diagnostics.Descriptor)
		assert.Empty(t, lg.Diagnostics.CleanupNudge)
	})
}

func TestBuildDiagnostics_CleanupNudgeHeavyChalkPayoutImpact(t *testing.T) {
	t.Parallel()

	game1 := richV2Game()
	game2 := analyzer.Game{
		ID:   "kc-lv",
		Home: "KC",
		Away: "LV",
		MarketScript: analyzer.MarketScript{
			HomeSpread: -9.5,
			GameTotal:  44.5,
			HasSpread:  true,
			HasTotal:   true,
		},
	}

	legs := []analyzer.Leg{
		{ID: 1, GameID: game1.ID, Team: "BUF", Market: "SPREAD", Side: "TEAM", Line: new(-3.5), Odds: -110},
		{ID: 2, GameID: game2.ID, Team: "KC", Market: "ML", Side: "TEAM", Odds: -450},
	}
	ev := analyzer.EvaluateSlip(legs, map[string]analyzer.Game{
		game1.ID: game1,
		game2.ID: game2,
	}, nil)

	// -450 decimal is 1 + 100/450 = 1.2222 -> dropping it cuts payout by (1 - 1/1.2222) = 18%.
	assert.Contains(t, ev.Grade.Diagnostics.CleanupNudge, "Dropping the -450 leg cuts your payout by 18%")
	assertFriendlyV2Tone(t, ev.Grade.Diagnostics.CleanupNudge)
}

func TestClassifyProcessOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		letter    string
		won       bool
		wantKey   string
		wantLabel string
		wantTone  string
	}{
		{
			name:      "A won is Clean Hit",
			letter:    "A",
			won:       true,
			wantKey:   "clean_hit",
			wantLabel: "Clean Hit",
			wantTone:  "positive",
		},
		{
			name:      "A- won is Clean Hit",
			letter:    "A-",
			won:       true,
			wantKey:   "clean_hit",
			wantLabel: "Clean Hit",
			wantTone:  "positive",
		},
		{
			name:      "B won is Clean Hit",
			letter:    "B",
			won:       true,
			wantKey:   "clean_hit",
			wantLabel: "Clean Hit",
			wantTone:  "positive",
		},
		{
			name:      "A lost is Tough Beat",
			letter:    "A",
			won:       false,
			wantKey:   "tough_beat",
			wantLabel: "Tough Beat",
			wantTone:  "neutral",
		},
		{
			name:      "B+ lost is Tough Beat",
			letter:    "B+",
			won:       false,
			wantKey:   "tough_beat",
			wantLabel: "Tough Beat",
			wantTone:  "neutral",
		},
		{
			name:      "B lost is Tough Beat",
			letter:    "B",
			won:       false,
			wantKey:   "tough_beat",
			wantLabel: "Tough Beat",
			wantTone:  "neutral",
		},
		{
			name:      "C won is Total Bailout",
			letter:    "C",
			won:       true,
			wantKey:   "total_bailout",
			wantLabel: "Total Bailout",
			wantTone:  "warn",
		},
		{
			name:      "D- won is Total Bailout",
			letter:    "D-",
			won:       true,
			wantKey:   "total_bailout",
			wantLabel: "Total Bailout",
			wantTone:  "warn",
		},
		{
			name:      "F won is Total Bailout",
			letter:    "F",
			won:       true,
			wantKey:   "total_bailout",
			wantLabel: "Total Bailout",
			wantTone:  "warn",
		},
		{
			name:      "F lost is Book Donation",
			letter:    "F",
			won:       false,
			wantKey:   "book_donation",
			wantLabel: "Book Donation",
			wantTone:  "bad",
		},
		{
			name:      "D lost is Book Donation",
			letter:    "D",
			won:       false,
			wantKey:   "book_donation",
			wantLabel: "Book Donation",
			wantTone:  "bad",
		},
		{
			name:      "C+ lost is Book Donation",
			letter:    " c+ ",
			won:       false,
			wantKey:   "book_donation",
			wantLabel: "Book Donation",
			wantTone:  "bad",
		},
		{name: "empty letter is zero", letter: "", won: true, wantKey: "", wantLabel: ""},
		{name: "whitespace letter is zero", letter: "   ", won: false, wantKey: "", wantLabel: ""},
		{name: "unknown letter is zero", letter: "X", won: false, wantKey: "", wantLabel: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := analyzer.ClassifyProcessOutcome(tt.letter, tt.won)
			assert.Equal(t, tt.wantKey, got.Key)
			assert.Equal(t, tt.wantLabel, got.Label)
			assert.Equal(t, tt.wantTone, got.Tone)
			if tt.wantKey != "" {
				assert.NotEmpty(t, got.Summary)
				assertFriendlyV2Tone(t, got.Label, got.Summary)
			} else {
				assert.Empty(t, got.Summary)
			}
		})
	}
}

func TestClassifyProcessResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		letter    string
		result    parlay.Result
		wantKey   string
		wantLabel string
	}{
		{name: "A won", letter: "A", result: parlay.Won, wantKey: "clean_hit", wantLabel: "Clean Hit"},
		{name: "B- lost", letter: "B-", result: parlay.Lost, wantKey: "tough_beat", wantLabel: "Tough Beat"},
		{name: "C won", letter: "C", result: parlay.Won, wantKey: "total_bailout", wantLabel: "Total Bailout"},
		{name: "F lost", letter: "F", result: parlay.Lost, wantKey: "book_donation", wantLabel: "Book Donation"},
		{name: "push returns zero", letter: "A", result: parlay.Push, wantKey: "", wantLabel: ""},
		{name: "bad grade push returns zero", letter: "F", result: parlay.Push, wantKey: "", wantLabel: ""},
		{name: "void returns zero", letter: "B", result: parlay.Void, wantKey: "", wantLabel: ""},
		{name: "pending returns zero", letter: "A", result: parlay.Pending, wantKey: "", wantLabel: ""},
		{name: "needs grading returns zero", letter: "C", result: parlay.NeedsGrading, wantKey: "", wantLabel: ""},
		{name: "ungraded won returns zero", letter: "", result: parlay.Won, wantKey: "", wantLabel: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := analyzer.ClassifyProcessResult(tt.letter, tt.result)
			assert.Equal(t, tt.wantKey, got.Key)
			assert.Equal(t, tt.wantLabel, got.Label)
		})
	}
}
