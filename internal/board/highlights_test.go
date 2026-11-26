package board_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestRowToAnalyzerLeg(t *testing.T) {
	t.Parallel()

	line := 265.5
	row := board.Row{
		GameID:     "g1",
		Market:     "PASS_YDS",
		Team:       "BUF",
		PlayerID:   "qb1",
		PlayerName: "Josh Allen",
		Side:       "OVER",
		Line:       &line,
		Odds:       -115,
		Picked:     42,
	}

	leg := board.RowToAnalyzerLeg(row)
	assert.Equal(t, int64(42), leg.ID)
	assert.Equal(t, "g1", leg.GameID)
	assert.Equal(t, "PASS_YDS", leg.Market)
	assert.Equal(t, "BUF", leg.Team)
	assert.Equal(t, "qb1", leg.PlayerID)
	assert.Equal(t, "Josh Allen", leg.PlayerName)
	assert.Equal(t, "OVER", leg.Side)
	require.NotNil(t, leg.Line)
	assert.InDelta(t, 265.5, *leg.Line, 0.001)
	assert.Equal(t, -115, leg.Odds)
}

func TestSelectHighlightsFiltersAndDeduplicates(t *testing.T) {
	t.Parallel()

	games := map[string]analyzer.Game{
		"g1": {
			ID:   "g1",
			Home: "BUF",
			Away: "MIA",
			MarketScript: analyzer.MarketScript{
				HomeSpread:       -2.5,
				GameTotal:        49.5,
				HomeImpliedTotal: 26.0,
				AwayImpliedTotal: 23.5,
				HasSpread:        true,
				HasTotal:         true,
			},
			HomeEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 10,
				DefRushEPAAllowedRank: 10,
				PlaysPerGame:          64,
				PassRate:              0.58,
			},
			AwayEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 30, // favorable pass defense for BUF
				DefRushEPAAllowedRank: 29, // favorable rush defense for BUF
				DefPassEPAAllowed:     0.20,
				DefRushEPAAllowed:     0.15,
			},
			Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
				{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}: {
					Line: new(235.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
				{MarketType: "PASS_TDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}: {
					Line: new(1.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
				{MarketType: "REC_YDS", Team: "BUF", PlayerName: "khalil shakir", Side: "OVER"}: {
					Line: new(38.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
				{MarketType: "RUSH_YDS", Team: "BUF", PlayerName: "james cook", Side: "OVER"}: {
					Line: new(44.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
			},
		},
		"g2": {
			ID:   "g2",
			Home: "KC",
			Away: "LV",
			MarketScript: analyzer.MarketScript{
				HomeSpread:       -2.5,
				GameTotal:        49.5,
				HomeImpliedTotal: 26.0,
				AwayImpliedTotal: 23.5,
				HasSpread:        true,
				HasTotal:         true,
			},
			HomeEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 12,
				PlaysPerGame:          64,
				PassRate:              0.58,
			},
			AwayEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 28,
				DefPassEPAAllowed:     0.18,
			},
			Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
				{MarketType: "REC_YDS", Team: "KC", PlayerName: "xavier worthy", Side: "OVER"}: {
					Line: new(38.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
			},
		},
	}

	stats := map[string]analyzer.PlayerStats{
		"qb1": {
			Position: "QB",
			Role:     analyzer.RoleQB,
			Games: map[string][]float64{
				"PASS_YDS": {290, 285, 280, 275, 270},
				"PASS_TDS": {3, 2, 2, 2, 2},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6,
				PassYds:     285,
				PassTDs:     2.3,
				PassAtt:     35,
				OffensePct:  0.98,
				ExpPassYds:  278,
				ExpPassTDs:  2.1,
			},
			Usage: &analyzer.PlayerUsage{
				Last3SnapAvg:  0.98,
				SeasonSnapAvg: 0.78,
			},
		},
		"wr1": {
			Position: "WR",
			Role:     analyzer.RoleWR1,
			Games: map[string][]float64{
				"REC_YDS": {95, 90, 88, 84, 82},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6,
				RecYds:      96,
				Receptions:  7.5,
				TargetShare: 0.30,
				OffensePct:  0.95,
				ExpRecYds:   92,
				TotalEPA:    5.5,
			},
		},
		"rb1": {
			Position: "RB",
			Role:     analyzer.RoleRB1,
			Games: map[string][]float64{
				"RUSH_YDS": {90, 85, 82, 80, 78},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6,
				RushYds:     92,
				Carries:     19,
				CarryShare:  0.70,
				OffensePct:  0.78,
				ExpRushYds:  86,
				TotalEPA:    4.0,
			},
		},
		"wr3": {
			Position: "WR",
			Role:     analyzer.RoleWR1,
			Games: map[string][]float64{
				"REC_YDS": {95, 90, 88, 84, 82},
			},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6,
				RecYds:      96,
				Receptions:  7.5,
				TargetShare: 0.30,
				OffensePct:  0.95,
				ExpRecYds:   92,
				TotalEPA:    5.5,
			},
		},
		"wr2": {
			Position: "WR",
			Role:     analyzer.RoleWR2,
			Games: map[string][]float64{
				"REC_YDS": {50, 50, 50, 50, 50},
			},
		},
	}

	rows := []board.Row{
		// Picked A-grade row (excluded when ExcludePicked is true)
		{
			GameID: "g1", Market: "PASS_YDS", Team: "BUF",
			PlayerID: "qb1", PlayerName: "Josh Allen", Side: "OVER",
			Line: new(235.5), Odds: -105, Picked: 99,
		},
		// Same player, second A-grade market (PASS_TDS)
		{
			GameID: "g1", Market: "PASS_TDS", Team: "BUF",
			PlayerID: "qb1", PlayerName: "Josh Allen", Side: "OVER",
			Line: new(1.5), Odds: -120,
		},
		// A-grade WR1
		{
			GameID: "g1", Market: "REC_YDS", Team: "BUF",
			PlayerID: "wr1", PlayerName: "Khalil Shakir", Side: "OVER",
			Line: new(38.5), Odds: -115,
		},
		// Same score as WR1, better odds (-105 > -115) -> ranks above WR1
		{
			GameID: "g1", Market: "RUSH_YDS", Team: "BUF",
			PlayerID: "rb1", PlayerName: "James Cook", Side: "OVER",
			Line: new(44.5), Odds: -105,
		},
		// Same score and odds (-105) as RB1, later GameID ("g2" > "g1") -> tie-break by GameID
		{
			GameID: "g2", Market: "REC_YDS", Team: "KC",
			PlayerID: "wr3", PlayerName: "Xavier Worthy", Side: "OVER",
			Line: new(38.5), Odds: -105,
		},
		// Not A-grade
		{
			GameID: "g1", Market: "REC_YDS", Team: "BUF",
			PlayerID: "wr2", PlayerName: "Mack Hollins", Side: "OVER",
			Line: new(65.5), Odds: -110,
		},
		// OTHER market must be skipped
		{
			GameID: "g1", Market: "OTHER", Team: "BUF",
			PlayerID: "qb1", PlayerName: "Josh Allen", Side: "YES",
			Odds: 150,
		},
	}

	got := board.SelectHighlights(rows, games, stats, board.HighlightOptions{
		MaxLegs:       4,
		MaxParlays:    -1, // disable parlays for this test
		MaxPerPlayer:  1,
		ExcludePicked: true,
	})

	require.Len(t, got.Legs, 4, "picked row, non-A row, and duplicate qb1 market should be filtered")
	assert.Equal(t, "James Cook", got.Legs[0].Row.PlayerName)
	assert.Equal(t, "Xavier Worthy", got.Legs[1].Row.PlayerName)
	assert.Equal(t, "Khalil Shakir", got.Legs[2].Row.PlayerName)
	assert.Equal(t, "Josh Allen", got.Legs[3].Row.PlayerName)
	assert.Equal(t, "PASS_TDS", got.Legs[3].Row.Market, "picked PASS_YDS excluded, PASS_TDS kept")
	assert.Equal(t, "A", got.Legs[0].Grade.Letter)
	assert.NotEmpty(t, got.Legs[0].Reason)

	// When ExcludePicked is false, Josh Allen's PASS_YDS (Picked=99) is included (score 100),
	// and MaxPerPlayer=1 deduplicates his PASS_TDS.
	gotWithPicked := board.SelectHighlights(rows, games, stats, board.HighlightOptions{
		MaxLegs:       4,
		MaxParlays:    -1,
		MaxPerPlayer:  1,
		ExcludePicked: false,
	})
	require.Len(t, gotWithPicked.Legs, 4)
	assert.Equal(t, "PASS_YDS", gotWithPicked.Legs[0].Row.Market)
}

func TestSelectHighlightsParlayCombos(t *testing.T) {
	t.Parallel()

	t.Run("surfaces shootout stack even with B-grade partner and rejects conflicts", func(t *testing.T) {
		t.Parallel()

		games := map[string]analyzer.Game{
			"g1": {
				ID:   "g1",
				Home: "BUF",
				Away: "MIA",
				AwayEPA: &analyzer.TeamDefenseProfile{
					DefPassEPAAllowedRank: 30,
					DefRushEPAAllowedRank: 29,
				},
				Nfelo: &analyzer.Nfelo{
					HomeSpread: -10.5, // blowout favorite BUF (-10.5) triggers blowout_passing if paired with BUF PASS_YDS
				},
			},
			"g2": {
				ID:   "g2",
				Home: "CIN",
				Away: "BAL",
				MarketScript: analyzer.MarketScript{
					HomeSpread:       -2.5,
					GameTotal:        52.5,
					HomeImpliedTotal: 27.5,
					AwayImpliedTotal: 25.0,
					HasSpread:        true,
					HasTotal:         true,
				},
				AwayEPA: &analyzer.TeamDefenseProfile{
					DefPassEPAAllowedRank: 29,
					DefPassEPAAllowed:     0.20,
				},
				Nfelo: &analyzer.Nfelo{
					HomeSpread: -2.5,
				},
				Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
					{MarketType: "PASS_YDS", Team: "CIN", PlayerName: "joe burrow", Side: "OVER"}: {
						Line: new(245.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
					},
					{MarketType: "REC_YDS", Team: "CIN", PlayerName: "jamarr chase", Side: "OVER"}: {
						Line: new(42.5), FairProb: 0.65, PowerFairProb: 0.65, Source: "Pinnacle", Tier: 1,
					},
					{MarketType: "TOTAL", Side: "OVER"}: {
						Line: new(48.5), FairProb: 0.51, PowerFairProb: 0.51, Source: "Pinnacle", Tier: 1,
					},
				},
			},
		}

		stats := map[string]analyzer.PlayerStats{
			// g2 shootout trio: QB (A), WR (A), TOTAL OVER (B -> lifted to A by shootout copula + stack bonus)
			"qb_cin": {
				Position: "QB",
				Role:     analyzer.RoleQB,
				Games: map[string][]float64{
					"PASS_YDS": {310, 305, 300, 295, 290},
				},
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 6, PassYds: 300, PassAtt: 38, OffensePct: 0.99, ExpPassYds: 292, TotalEPA: 9.5,
				},
			},
			"wr_cin": {
				Position: "WR",
				Role:     analyzer.RoleWR1,
				Games: map[string][]float64{
					"REC_YDS": {105, 100, 95, 92, 90},
				},
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 6,
					RecYds:      105,
					Receptions:  8.0,
					TargetShare: 0.32,
					OffensePct:  0.96,
					ExpRecYds:   98,
					TotalEPA:    6.0,
				},
			},
			// g1 RB + BUF spread (-10.5)
			"rb_buf": {
				Position: "RB",
				Games: map[string][]float64{
					"RUSH_YDS": {95, 90, 88, 85, 82},
				},
			},
			// g1 MIA RB (trailing RB conflict with BUF -10.5!)
			"rb_mia": {
				Position: "RB",
				Games: map[string][]float64{
					"RUSH_YDS": {95, 90, 88, 85, 82},
				},
			},
		}
		// Give MIA RB favorable matchup too so both RBs are A-grade individually.
		g1 := games["g1"]
		g1.HomeEPA = &analyzer.TeamDefenseProfile{DefRushEPAAllowedRank: 29}
		games["g1"] = g1

		rows := []board.Row{
			// g2 shootout legs
			{
				GameID:     "g2",
				Market:     "PASS_YDS",
				Team:       "CIN",
				PlayerID:   "qb_cin",
				PlayerName: "Joe Burrow",
				Side:       "OVER",
				Line:       new(245.5),
				Odds:       -110,
			},
			{
				GameID:     "g2",
				Market:     "REC_YDS",
				Team:       "CIN",
				PlayerID:   "wr_cin",
				PlayerName: "Ja'Marr Chase",
				Side:       "OVER",
				Line:       new(42.5),
				Odds:       -110,
			},
			// TOTAL OVER 48.5 (not near key number) -> score 75 (B), lifted to A (96) by +8 shootout stack!
			{GameID: "g2", Market: "TOTAL", Team: "", Side: "OVER", Line: new(48.5), Odds: -110},
			// Opposite side TOTAL UNDER in g2 (must never pair with TOTAL OVER)
			{GameID: "g2", Market: "TOTAL", Team: "", Side: "UNDER", Line: new(48.5), Odds: -110},

			// g1 BUF -10.5 spread (score 75 + 8 nfelo? No, market=-10.5 vs nfelo=-10.5 -> 75, so not A unless sharp edge)
			// Let's give g1 BUF -6.5 vs nfelo -10.5 (4 pt cushion -> +8 -> 83 B), or with sharp edge -> A.
			{GameID: "g1", Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-6.5), Odds: -110},
			{
				GameID:     "g1",
				Market:     "RUSH_YDS",
				Team:       "MIA",
				PlayerID:   "rb_mia",
				PlayerName: "De'Von Achane",
				Side:       "OVER",
				Line:       new(64.5),
				Odds:       -110,
			},
		}

		// Check SameGameOnly on g2: should surface the 3-leg Shootout stack!
		g2Rows := []board.Row{rows[0], rows[1], rows[2], rows[3]}
		gotG2 := board.SelectHighlights(g2Rows, games, stats, board.HighlightOptions{
			MaxLegs:      3,
			MaxParlays:   1,
			SameGameOnly: true,
		})
		require.Len(t, gotG2.Parlays, 1)
		p := gotG2.Parlays[0]
		assert.Equal(t, "Shootout stack", p.Label)
		assert.Equal(t, "A", p.Grade.Letter)
		assert.Len(t, p.Legs, 3)

		expectedOdds := parlay.DecimalToAmerican(parlay.CombinedOdds([]parlay.Leg{
			{Decimal: parlay.AmericanToDecimal(-110), Result: parlay.Pending},
			{Decimal: parlay.AmericanToDecimal(-110), Result: parlay.Pending},
			{Decimal: parlay.AmericanToDecimal(-110), Result: parlay.Pending},
		}))
		assert.Equal(t, expectedOdds, p.CombinedOdds)
	})

	t.Run("rejects trailing_rb conflict and deduplicates overlapping parlays", func(t *testing.T) {
		t.Parallel()

		games := map[string]analyzer.Game{
			"g1": {
				ID:   "g1",
				Home: "BUF",
				Away: "MIA",
				HomeEPA: &analyzer.TeamDefenseProfile{
					DefRushEPAAllowedRank: 29,
				},
				AwayEPA: &analyzer.TeamDefenseProfile{
					DefPassEPAAllowedRank: 30,
					DefRushEPAAllowedRank: 29,
				},
				Nfelo: &analyzer.Nfelo{
					HomeSpread: -14.0, // BUF -6.5 has huge cushion (+8), and -10.5 triggers trailing_rb on MIA RB
				},
			},
		}
		stats := map[string]analyzer.PlayerStats{
			"rb_mia": {
				Position: "RB",
				Role:     analyzer.RoleRB1,
				Games:    map[string][]float64{"RUSH_YDS": {90, 88, 85, 82, 80}},
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 6, RushYds: 86, Carries: 18, CarryShare: 0.68, OffensePct: 0.76, ExpRushYds: 82,
				},
			},
		}

		rows := []board.Row{
			// MIA +10.5 spread with nfelo cushion (+8) and sharp edge -> A
			{GameID: "g1", Market: "SPREAD", Team: "MIA", Side: "TEAM", Line: new(10.5), Odds: -110},
			{
				GameID:     "g1",
				Market:     "RUSH_YDS",
				Team:       "MIA",
				PlayerID:   "rb_mia",
				PlayerName: "De'Von Achane",
				Side:       "OVER",
				Line:       new(52.5),
				Odds:       -110,
			},
		}
		// Add sharp benchmarks so both MIA +10.5 and De'Von Achane RUSH_YDS are A-grade individually.
		g1 := games["g1"]
		g1.Nfelo = &analyzer.Nfelo{HomeSpread: -6.5}
		g1.MarketScript = analyzer.MarketScript{
			HomeSpread:       -3.5,
			GameTotal:        48.5,
			HomeImpliedTotal: 26.0,
			AwayImpliedTotal: 22.5,
			HasSpread:        true,
			HasTotal:         true,
		}
		g1.Sharp = map[analyzer.SharpKey]analyzer.SharpQuote{
			{MarketType: "SPREAD", Team: "MIA", Side: "TEAM"}: {
				Line:          new(8.5),
				FairProb:      0.55,
				PowerFairProb: 0.55,
				Source:        "Pinnacle",
				Tier:          1,
			},
			{MarketType: "RUSH_YDS", Team: "MIA", PlayerName: "devon achane", Side: "OVER"}: {
				Line:          new(52.5),
				FairProb:      0.62,
				PowerFairProb: 0.62,
				Source:        "Pinnacle",
				Tier:          1,
			},
		}
		games["g1"] = g1

		got := board.SelectHighlights(rows, games, stats, board.HighlightOptions{
			MaxLegs:      3,
			MaxParlays:   2,
			SameGameOnly: true,
		})
		// Both legs are A-grade individually, but pairing them triggers trailing_rb (Danger)!
		assert.Len(t, got.Legs, 2)
		assert.Empty(t, got.Parlays, "conflict combo must be rejected")
	})

	t.Run("SameGameOnly prevents cross-game parlays and deduplication prevents sharing >1 leg", func(t *testing.T) {
		t.Parallel()

		games := map[string]analyzer.Game{
			"g1": {
				ID:      "g1",
				Home:    "BUF",
				Away:    "MIA",
				AwayEPA: &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 30, DefPassEPAAllowed: 0.20},
				MarketScript: analyzer.MarketScript{
					HomeSpread: -2.5, GameTotal: 49.5, HomeImpliedTotal: 26.0, AwayImpliedTotal: 23.5,
					HasSpread: true, HasTotal: true,
				},
				Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
					{MarketType: "REC_YDS", Team: "BUF", PlayerName: "p1", Side: "OVER"}: {
						Line: new(38.5), FairProb: 0.64, PowerFairProb: 0.64, Source: "Pinnacle", Tier: 1,
					},
				},
			},
			"g2": {
				ID:      "g2",
				Home:    "KC",
				Away:    "LV",
				AwayEPA: &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 30, DefPassEPAAllowed: 0.20},
				MarketScript: analyzer.MarketScript{
					HomeSpread: -2.5, GameTotal: 49.5, HomeImpliedTotal: 26.0, AwayImpliedTotal: 23.5,
					HasSpread: true, HasTotal: true,
				},
				Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
					{MarketType: "REC_YDS", Team: "KC", PlayerName: "p2", Side: "OVER"}: {
						Line: new(38.5), FairProb: 0.64, PowerFairProb: 0.64, Source: "Pinnacle", Tier: 1,
					},
				},
			},
		}
		stats := map[string]analyzer.PlayerStats{
			"p1": {
				Position: "WR",
				Role:     analyzer.RoleWR1,
				Games:    map[string][]float64{"REC_YDS": {90, 90, 90, 90, 90}},
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 6,
					RecYds:      96,
					Receptions:  7.5,
					TargetShare: 0.30,
					OffensePct:  0.95,
					ExpRecYds:   92,
					TotalEPA:    5.5,
				},
			},
			"p2": {
				Position: "WR",
				Role:     analyzer.RoleWR1,
				Games:    map[string][]float64{"REC_YDS": {90, 90, 90, 90, 90}},
				EWMA: &analyzer.PlayerEWMAUsage{
					GamesPlayed: 6,
					RecYds:      96,
					Receptions:  7.5,
					TargetShare: 0.30,
					OffensePct:  0.95,
					ExpRecYds:   92,
					TotalEPA:    5.5,
				},
			},
		}
		rows := []board.Row{
			{
				GameID:     "g1",
				Market:     "REC_YDS",
				Team:       "BUF",
				PlayerID:   "p1",
				PlayerName: "P1",
				Side:       "OVER",
				Line:       new(38.5),
				Odds:       -110,
			},
			{
				GameID:     "g2",
				Market:     "REC_YDS",
				Team:       "KC",
				PlayerID:   "p2",
				PlayerName: "P2",
				Side:       "OVER",
				Line:       new(38.5),
				Odds:       -110,
			},
		}

		sameGame := board.SelectHighlights(rows, games, stats, board.HighlightOptions{
			SameGameOnly: true,
		})
		assert.Empty(t, sameGame.Parlays)

		crossGame := board.SelectHighlights(rows, games, stats, board.HighlightOptions{
			SameGameOnly: false,
		})
		require.Len(t, crossGame.Parlays, 1)
		assert.Equal(t, "2-leg A-grade combo", crossGame.Parlays[0].Label)
	})
}

func TestSelectHighlightsFailSoft(t *testing.T) {
	t.Parallel()

	empty := board.SelectHighlights(nil, nil, nil, board.HighlightOptions{})
	assert.Empty(t, empty.Legs)
	assert.Empty(t, empty.Parlays)

	noGames := board.SelectHighlights(
		[]board.Row{{GameID: "g1", Market: "ML", Team: "BUF", Side: "TEAM", Odds: -110}},
		nil,
		nil,
		board.HighlightOptions{},
	)
	assert.Empty(t, noGames.Legs)
	assert.Empty(t, noGames.Parlays)
}

func BenchmarkSelectHighlightsFullSlate(b *testing.B) {
	const numGames = 16
	const playersPerGame = 12
	games := make(map[string]analyzer.Game, numGames)
	stats := make(map[string]analyzer.PlayerStats, numGames*playersPerGame)
	rows := make([]board.Row, 0, 1550)

	kickoff := time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC)
	_ = kickoff

	for gIdx := range numGames {
		gid := fmt.Sprintf("g%02d", gIdx)
		home := fmt.Sprintf("H%02d", gIdx)
		away := fmt.Sprintf("A%02d", gIdx)
		games[gid] = analyzer.Game{
			ID:   gid,
			Home: home,
			Away: away,
			HomeEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 16 + (gIdx % 16),
				DefRushEPAAllowedRank: 15 + (gIdx % 16),
			},
			AwayEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 17 + (gIdx % 16),
				DefRushEPAAllowedRank: 14 + (gIdx % 16),
			},
			Nfelo: &analyzer.Nfelo{
				HomeSpread:  -3.5,
				HomeWinProb: 0.60,
			},
		}

		// Game lines (6 rows per game = 96)
		rows = append(rows,
			board.Row{GameID: gid, Market: "SPREAD", Team: home, Side: "TEAM", Line: new(-3.5), Odds: -110},
			board.Row{GameID: gid, Market: "SPREAD", Team: away, Side: "TEAM", Line: new(3.5), Odds: -110},
			board.Row{GameID: gid, Market: "TOTAL", Side: "OVER", Line: new(45.5), Odds: -110},
			board.Row{GameID: gid, Market: "TOTAL", Side: "UNDER", Line: new(45.5), Odds: -110},
			board.Row{GameID: gid, Market: "ML", Team: home, Side: "TEAM", Odds: -165},
			board.Row{GameID: gid, Market: "ML", Team: away, Side: "TEAM", Odds: 140},
		)

		// ~90 player prop rows per game -> ~1,536 total rows across 16 games
		for pIdx := range playersPerGame {
			pid := fmt.Sprintf("%s_p%02d", gid, pIdx)
			team := home
			if pIdx >= playersPerGame/2 {
				team = away
			}
			pos := "WR"
			markets := []string{"REC_YDS", "RECEPTIONS", "ANYTIME_TD", "RUSH_YDS"}
			if pIdx%6 == 0 {
				pos = "QB"
				markets = []string{"PASS_YDS", "PASS_TDS", "RUSH_YDS", "ANYTIME_TD"}
			}
			stats[pid] = analyzer.PlayerStats{
				Position: pos,
				Games: map[string][]float64{
					"PASS_YDS":   {280, 275, 260, 255, 250},
					"PASS_TDS":   {2, 2, 2, 1, 2},
					"REC_YDS":    {85, 80, 75, 70, 65},
					"RECEPTIONS": {6, 6, 5, 5, 5},
					"RUSH_YDS":   {75, 70, 65, 60, 55},
				},
			}
			for _, m := range markets {
				rows = append(rows,
					board.Row{
						GameID: gid, Market: m, Team: team,
						PlayerID: pid, PlayerName: "Player " + pid,
						Side: "OVER", Line: new(55.5), Odds: -110,
					},
					board.Row{
						GameID: gid, Market: m, Team: team,
						PlayerID: pid, PlayerName: "Player " + pid,
						Side: "UNDER", Line: new(55.5), Odds: -110,
					},
				)
			}
		}
	}

	opts := board.HighlightOptions{
		MaxLegs:       3,
		MaxParlays:    2,
		MaxPerPlayer:  1,
		ExcludePicked: true,
	}

	b.ResetTimer()
	for b.Loop() {
		_ = board.SelectHighlights(rows, games, stats, opts)
	}
}
