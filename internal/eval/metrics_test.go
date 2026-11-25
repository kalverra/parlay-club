package eval

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func testGame() map[string]analyzer.Game {
	return map[string]analyzer.Game{
		"g1": {
			ID:     "g1",
			Home:   "BUF",
			Away:   "MIA",
			Indoor: true,
		},
	}
}

func testStats() map[string]analyzer.PlayerStats {
	return map[string]analyzer.PlayerStats{
		"p_hot": {
			Position: "WR",
			Games: map[string][]float64{
				"REC_YDS": {80, 75, 70, 65, 60},
			},
		},
		"p_cold": {
			Position: "WR",
			Games: map[string][]float64{
				"REC_YDS": {40, 45, 50, 42, 48},
			},
		},
	}
}

func bandByName(bands []BandStats, name string) (BandStats, bool) {
	for _, b := range bands {
		if b.Band == name {
			return b, true
		}
	}
	return BandStats{}, false
}

func factorByRule(factors []FactorStats, rule string) (FactorStats, bool) {
	for _, f := range factors {
		if f.Rule == rule {
			return f, true
		}
	}
	return FactorStats{}, false
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()

	hotLeg := func(id int64, odds int, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:       id,
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: "p_hot",
				Side:     "OVER",
				Line:     new(55.5),
				Odds:     odds,
			},
			Result:      res,
			DecimalOdds: parlay.AmericanToDecimal(odds),
		}
	}

	coldLeg := func(id int64, odds int, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:       id,
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: "p_cold",
				Side:     "OVER",
				Line:     new(55.5),
				Odds:     odds,
			},
			Result:      res,
			DecimalOdds: parlay.AmericanToDecimal(odds),
		}
	}

	ungradedLeg := func(id int64, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:     id,
				GameID: "g1",
				Market: "OTHER",
				Odds:   100,
			},
			Result:      res,
			DecimalOdds: 2.0,
		}
	}

	tests := []struct {
		name    string
		dataset ReplayDataset
		rubric  analyzer.Rubric
		check   func(t *testing.T, r Report)
	}{
		{
			name:    "empty dataset returns zeroed bands and monotonic true",
			dataset: ReplayDataset{},
			rubric:  analyzer.RubricV1,
			check: func(t *testing.T, r Report) {
				t.Helper()
				assert.Equal(t, "v1", r.RubricName)
				require.Len(t, r.LegBands, 6)
				require.Len(t, r.SlipBands, 6)
				assert.Empty(t, r.Factors)
				assert.True(t, r.LegMonotonic)
				assert.True(t, r.LegMonotonicWin)
				assert.True(t, r.LegMonotonicLift)
				for _, b := range r.LegBands {
					assert.Zero(t, b.Total)
					assert.Zero(t, b.Decided)
				}
			},
		},
		{
			name: "leg and slip calibration, lift, flat ROI, factor attribution, and push/void exclusion",
			dataset: ReplayDataset{
				Coverage: DataCoverage{TotalLegs: 6, WithGameLogs: 4},
				Weeks: []ReplayedWeek{
					{
						Season:  2025,
						WeekNum: 4,
						Games:   games,
						Stats:   stats,
						Slips: []ReplayedSlip{
							{
								SlipID:       1,
								Result:       parlay.Won,
								CombinedOdds: 4.0,
								Legs: []ReplayedLeg{
									hotLeg(1, 100, parlay.Won),  // Band A (85), dec=2.0, implied=0.5, units=+1
									hotLeg(2, 100, parlay.Won),  // Band A (85), dec=2.0, implied=0.5, units=+1
									hotLeg(3, 100, parlay.Push), // Band A (85), excluded from decided & factors
								},
							},
							{
								SlipID:       2,
								Result:       parlay.Lost,
								CombinedOdds: 2.0,
								Legs: []ReplayedLeg{
									coldLeg(5, 100, parlay.Lost), // Band C (60), dec=2.0, implied=0.5, units=-1
									coldLeg(6, 100, parlay.Lost), // Band C (60), dec=2.0, implied=0.5, units=-1
								},
							},
							{
								SlipID:       3,
								Result:       parlay.Void,
								CombinedOdds: 2.0,
								Legs: []ReplayedLeg{
									ungradedLeg(4, parlay.Void), // ungraded, excluded from decided
								},
							},
						},
					},
				},
			},
			rubric: analyzer.RubricV1,
			check: func(t *testing.T, r Report) {
				t.Helper()
				assert.Equal(t, DataCoverage{TotalLegs: 6, WithGameLogs: 4}, r.Coverage)

				bandA, ok := bandByName(r.LegBands, "A")
				require.True(t, ok)
				assert.Equal(t, 3, bandA.Total)
				assert.Equal(t, 2, bandA.Decided)
				assert.Equal(t, 2, bandA.Wins)
				assert.Equal(t, 0, bandA.Losses)
				assert.Equal(t, 1, bandA.Pushes)
				assert.InDelta(t, 1.0, bandA.WinRate, 1e-9)
				assert.InDelta(t, 0.5, bandA.AvgImpliedProb, 1e-9)
				assert.InDelta(t, 0.5, bandA.Lift, 1e-9)
				assert.InDelta(t, 1.0, bandA.FlatROI, 1e-9)
				assert.InDelta(t, 0.25, bandA.BrierScore, 1e-9)

				bandC, ok := bandByName(r.LegBands, "C")
				require.True(t, ok)
				assert.Equal(t, 2, bandC.Total)
				assert.Equal(t, 2, bandC.Decided)
				assert.Equal(t, 0, bandC.Wins)
				assert.Equal(t, 2, bandC.Losses)
				assert.InDelta(t, 0.0, bandC.WinRate, 1e-9)
				assert.InDelta(t, 0.5, bandC.AvgImpliedProb, 1e-9)
				assert.InDelta(t, -0.5, bandC.Lift, 1e-9)
				assert.InDelta(t, -1.0, bandC.FlatROI, 1e-9)
				assert.InDelta(t, 0.25, bandC.BrierScore, 1e-9)

				ung, ok := bandByName(r.LegBands, ungradedBand)
				require.True(t, ok)
				assert.Equal(t, 1, ung.Total)
				assert.Equal(t, 0, ung.Decided)
				assert.Equal(t, 1, ung.Pushes)

				// Slip 1 has 3 graded legs at 85 -> baseline 85, no warnings -> Band A, WON at 4.0 (+3u)
				slipA, ok := bandByName(r.SlipBands, "A")
				require.True(t, ok)
				assert.Equal(t, 1, slipA.Decided)
				assert.Equal(t, 1, slipA.Wins)
				assert.InDelta(t, 1.0, slipA.WinRate, 1e-9)
				assert.InDelta(t, 0.25, slipA.AvgImpliedProb, 1e-9)
				assert.InDelta(t, 0.75, slipA.Lift, 1e-9)
				assert.InDelta(t, 3.0, slipA.FlatROI, 1e-9)

				// Slip 2 has 2 graded legs at 60 -> baseline 60 -> Band C, LOST at 2.0 (-1u)
				slipC, ok := bandByName(r.SlipBands, "C")
				require.True(t, ok)
				assert.Equal(t, 1, slipC.Decided)
				assert.Equal(t, 1, slipC.Losses)
				assert.InDelta(t, 0.0, slipC.WinRate, 1e-9)
				assert.InDelta(t, -1.0, slipC.FlatROI, 1e-9)

				// Factor attribution: push leg must not be counted, and labels come from GradeRubric.
				hitRate, ok := factorByRule(r.Factors, "hit_rate")
				require.True(t, ok)
				assert.Equal(t, "Strong L5 hit rate", hitRate.Label)
				assert.Equal(t, 2, hitRate.Count)
				assert.Equal(t, 2, hitRate.Wins)
				assert.InDelta(t, 1.0, hitRate.WinRate, 1e-9)
				assert.InDelta(t, 0.5, hitRate.Lift, 1e-9)
				assert.InDelta(t, 10.0, hitRate.AvgDelta, 1e-9)

				medTrap, ok := factorByRule(r.Factors, "median_trap")
				require.True(t, ok)
				assert.Equal(t, "L5 median trap", medTrap.Label)
				assert.Equal(t, 2, medTrap.Count)
				assert.Equal(t, 2, medTrap.Losses)
				assert.InDelta(t, 0.0, medTrap.WinRate, 1e-9)
				assert.InDelta(t, -0.5, medTrap.Lift, 1e-9)
				assert.InDelta(t, -15.0, medTrap.AvgDelta, 1e-9)

				assert.True(t, r.LegMonotonic)
			},
		},
		{
			name: "missing-odds legs do not skew Lift in band or factor stats",
			dataset: ReplayDataset{
				Weeks: []ReplayedWeek{
					{
						Season:  2025,
						WeekNum: 4,
						Games:   games,
						Stats:   stats,
						Slips: []ReplayedSlip{
							{
								SlipID: 1,
								Result: parlay.Lost,
								Legs: []ReplayedLeg{
									hotLeg(1, 100, parlay.Won), // Band A, odds=2.0 (implied=0.5), Won
									hotLeg(2, 0, parlay.Lost),  // Band A, missing odds, Lost
								},
							},
						},
					},
				},
			},
			rubric: analyzer.RubricV1,
			check: func(t *testing.T, r Report) {
				t.Helper()
				bandA, ok := bandByName(r.LegBands, "A")
				require.True(t, ok)
				assert.Equal(t, 2, bandA.Decided)
				assert.Equal(t, 1, bandA.Wins)
				assert.InDelta(t, 0.5, bandA.WinRate, 1e-9)
				assert.InDelta(t, 0.5, bandA.AvgImpliedProb, 1e-9)
				// Lift is computed only over the 1 leg with valid odds (1/1 - 0.5 = 0.5).
				assert.InDelta(t, 0.5, bandA.Lift, 1e-9)

				hitRate, ok := factorByRule(r.Factors, "hit_rate")
				require.True(t, ok)
				assert.Equal(t, 2, hitRate.Count)
				assert.InDelta(t, 0.5, hitRate.WinRate, 1e-9)
				assert.InDelta(t, 0.5, hitRate.AvgImpliedProb, 1e-9)
				assert.InDelta(t, 0.5, hitRate.Lift, 1e-9)
			},
		},
		{
			name: "non-monotonic bands flagged when C outperforms A",
			dataset: ReplayDataset{
				Weeks: []ReplayedWeek{
					{
						Season:  2025,
						WeekNum: 4,
						Games:   games,
						Stats:   stats,
						Slips: []ReplayedSlip{
							{
								SlipID: 1,
								Result: parlay.Lost,
								Legs: []ReplayedLeg{
									hotLeg(1, 100, parlay.Lost),
									coldLeg(2, 100, parlay.Won),
								},
							},
						},
					},
				},
			},
			rubric: analyzer.RubricV1,
			check: func(t *testing.T, r Report) {
				t.Helper()
				assert.False(t, r.LegMonotonic)
				assert.False(t, r.LegMonotonicWin)
				assert.False(t, r.LegMonotonicLift)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Evaluate(tc.dataset, tc.rubric)
			tc.check(t, got)
		})
	}
}

func TestCompare(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()

	dataset := ReplayDataset{
		Weeks: []ReplayedWeek{
			{
				Season:  2025,
				WeekNum: 4,
				Games:   games,
				Stats:   stats,
				Slips: []ReplayedSlip{
					{
						SlipID:       1,
						Result:       parlay.Won,
						CombinedOdds: 2.0,
						Legs: []ReplayedLeg{
							{
								Leg: analyzer.Leg{
									ID: 1, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_hot", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Won,
								DecimalOdds: 2.0,
							},
							{
								Leg: analyzer.Leg{
									ID: 2, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_cold", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Lost,
								DecimalOdds: 2.0,
							},
							{
								Leg: analyzer.Leg{
									ID: 3, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_cold", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Push,
								DecimalOdds: 2.0,
							},
						},
					},
				},
			},
		},
	}

	baselineRubric := analyzer.RubricV1
	candidateRubric := analyzer.RubricV1
	candidateRubric.Name = "v2-test"
	candidateRubric.HitRate = 0    // p_hot drops from 85 (A) to 75 (B)
	candidateRubric.MedianTrap = 0 // p_cold rises from 60 (C) to 75 (B)

	baseReport := Evaluate(dataset, baselineRubric)
	candReport := Evaluate(dataset, candidateRubric)
	diff := Compare(baseReport, candReport)

	assert.Equal(t, "v1", diff.BaselineName)
	assert.Equal(t, "v2-test", diff.CandidateName)
	require.Len(t, diff.LegBandDiffs, 6)
	require.Len(t, diff.SlipBandDiffs, 6)

	// Check LegBandDiff for B: baseline had 0 decided, candidate has 2 decided (1W, 1L -> 0.5 win rate, 0.0 lift, 0.0 ROI).
	var diffB BandDiff
	for _, d := range diff.LegBandDiffs {
		if d.Band == "B" {
			diffB = d
			break
		}
	}
	assert.Equal(t, 0, diffB.BaselineDecided)
	assert.Equal(t, 2, diffB.CandidateDecided)
	assert.InDelta(t, 0.5, diffB.WinRateDelta, 1e-9)

	// Migration matrix: A->B (1 decided, 1 win), C->B (2 total, 1 decided, 0 wins, 1 loss).
	require.Len(t, diff.Migrations, 2)
	assert.Equal(t, MigrationCell{
		FromBand: "A",
		ToBand:   "B",
		Total:    1,
		Decided:  1,
		Wins:     1,
		Losses:   0,
		WinRate:  1.0,
	}, diff.Migrations[0])
	assert.Equal(t, MigrationCell{
		FromBand: "C",
		ToBand:   "B",
		Total:    2,
		Decided:  1,
		Wins:     0,
		Losses:   1,
		WinRate:  0.0,
	}, diff.Migrations[1])

	// Sanity check no NaN values in any BandStats.
	for _, b := range candReport.LegBands {
		assert.False(t, math.IsNaN(b.WinRate))
		assert.False(t, math.IsNaN(b.Lift))
		assert.False(t, math.IsNaN(b.FlatROI))
		assert.False(t, math.IsNaN(b.BrierScore))
	}
}

func TestEvaluateWeekly(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()

	hotLeg := func(id int64, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:       id,
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: "p_hot",
				Side:     "OVER",
				Line:     new(55.5),
				Odds:     100,
			},
			Result:      res,
			DecimalOdds: 2.0,
		}
	}
	coldLeg := func(id int64, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:       id,
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: "p_cold",
				Side:     "OVER",
				Line:     new(55.5),
				Odds:     100,
			},
			Result:      res,
			DecimalOdds: 2.0,
		}
	}

	// Intentionally out of chronological order.
	dataset := ReplayDataset{
		Weeks: []ReplayedWeek{
			{
				Season:   2025,
				WeekNum:  4,
				WeekType: "REG",
				Games:    games,
				Stats:    stats,
				Slips: []ReplayedSlip{
					{
						SlipID: 4,
						Result: parlay.Lost,
						Legs:   []ReplayedLeg{hotLeg(1, parlay.Lost), coldLeg(2, parlay.Lost)},
					},
				},
			},
			{
				Season:   2025,
				WeekNum:  1,
				WeekType: "POST",
				Games:    games,
				Stats:    stats,
				Slips: []ReplayedSlip{
					{
						SlipID: 3,
						Result: parlay.Won,
						Legs:   []ReplayedLeg{hotLeg(3, parlay.Won), hotLeg(4, parlay.Push)},
					},
				},
			},
			{
				Season:   2024,
				WeekNum:  18,
				WeekType: "REG",
				Games:    games,
				Stats:    stats,
				Slips: []ReplayedSlip{
					{
						SlipID: 1,
						Result: parlay.Won,
						Legs:   []ReplayedLeg{hotLeg(5, parlay.Won)},
					},
				},
			},
			{
				Season:   2025,
				WeekNum:  1,
				WeekType: "REG",
				Games:    games,
				Stats:    stats,
				Slips: []ReplayedSlip{
					{
						SlipID: 2,
						Result: parlay.Lost,
						Legs:   []ReplayedLeg{hotLeg(6, parlay.Won), coldLeg(7, parlay.Lost)},
					},
				},
			},
		},
	}

	report := Evaluate(dataset, analyzer.RubricV1)
	require.Len(t, report.Weekly, 4)

	// Chronological order: 2024 W18 REG, 2025 W1 REG, 2025 W4 REG, 2025 W1 POST.
	assert.Equal(t, 2024, report.Weekly[0].Season)
	assert.Equal(t, 18, report.Weekly[0].WeekNum)
	assert.Equal(t, "REG", report.Weekly[0].WeekType)
	assert.Equal(t, 1, report.Weekly[0].DecidedLegs)
	assert.InDelta(t, 1.0, report.Weekly[0].LegWinRate, 1e-9)
	assert.InDelta(t, 0.5, report.Weekly[0].LegLift, 1e-9)
	assert.InDelta(t, 1.0, report.Weekly[0].LegFlatROI, 1e-9)

	assert.Equal(t, 2025, report.Weekly[1].Season)
	assert.Equal(t, 1, report.Weekly[1].WeekNum)
	assert.Equal(t, "REG", report.Weekly[1].WeekType)
	assert.Equal(t, 2, report.Weekly[1].DecidedLegs)
	assert.InDelta(t, 0.5, report.Weekly[1].LegWinRate, 1e-9)
	assert.InDelta(t, 0.0, report.Weekly[1].LegLift, 1e-9)
	assert.InDelta(t, 0.0, report.Weekly[1].LegFlatROI, 1e-9)

	assert.Equal(t, 2025, report.Weekly[2].Season)
	assert.Equal(t, 4, report.Weekly[2].WeekNum)
	assert.Equal(t, "REG", report.Weekly[2].WeekType)
	assert.Equal(t, 2, report.Weekly[2].DecidedLegs)
	assert.InDelta(t, 0.0, report.Weekly[2].LegWinRate, 1e-9)
	assert.InDelta(t, -0.5, report.Weekly[2].LegLift, 1e-9)
	assert.InDelta(t, -1.0, report.Weekly[2].LegFlatROI, 1e-9)

	assert.Equal(t, 2025, report.Weekly[3].Season)
	assert.Equal(t, 1, report.Weekly[3].WeekNum)
	assert.Equal(t, "POST", report.Weekly[3].WeekType)
	assert.Equal(t, 1, report.Weekly[3].DecidedLegs) // push excluded
	assert.InDelta(t, 1.0, report.Weekly[3].LegWinRate, 1e-9)

	// Weekly band counts must sum to overall report band counts.
	for i, overall := range report.LegBands {
		var total, decided, wins, losses, pushes int
		for _, w := range report.Weekly {
			total += w.LegBands[i].Total
			decided += w.LegBands[i].Decided
			wins += w.LegBands[i].Wins
			losses += w.LegBands[i].Losses
			pushes += w.LegBands[i].Pushes
		}
		assert.Equal(t, overall.Total, total)
		assert.Equal(t, overall.Decided, decided)
		assert.Equal(t, overall.Wins, wins)
		assert.Equal(t, overall.Losses, losses)
		assert.Equal(t, overall.Pushes, pushes)
	}
	for i, overall := range report.SlipBands {
		var total, decided, wins, losses, pushes int
		for _, w := range report.Weekly {
			total += w.SlipBands[i].Total
			decided += w.SlipBands[i].Decided
			wins += w.SlipBands[i].Wins
			losses += w.SlipBands[i].Losses
			pushes += w.SlipBands[i].Pushes
		}
		assert.Equal(t, overall.Total, total)
		assert.Equal(t, overall.Decided, decided)
		assert.Equal(t, overall.Wins, wins)
		assert.Equal(t, overall.Losses, losses)
		assert.Equal(t, overall.Pushes, pushes)
	}
}

func TestEvaluateAll(t *testing.T) {
	t.Parallel()

	dataset := ReplayDataset{
		Coverage: DataCoverage{TotalLegs: 2, WithGameLogs: 2},
		Weeks: []ReplayedWeek{
			{
				Season:  2025,
				WeekNum: 4,
				Games:   testGame(),
				Stats:   testStats(),
				Slips: []ReplayedSlip{
					{
						SlipID: 1,
						Result: parlay.Won,
						Legs: []ReplayedLeg{
							{
								Leg: analyzer.Leg{
									ID: 1, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_hot", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Won,
								DecimalOdds: 2.0,
							},
							{
								Leg: analyzer.Leg{
									ID: 2, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_cold", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Lost,
								DecimalOdds: 2.0,
							},
						},
					},
				},
			},
		},
	}

	single := EvaluateAll(dataset, []analyzer.Rubric{analyzer.RubricV1})
	assert.Equal(t, analyzer.ActiveRubric.Name, single.ActiveRubric)
	assert.Equal(t, dataset.Coverage, single.Coverage)
	require.Len(t, single.Reports, 1)
	assert.Equal(t, "v1", single.Reports[0].RubricName)
	assert.Empty(t, single.Diffs)

	v2 := analyzer.RubricV1
	v2.Name = "v2-test"
	v2.HitRate = 0
	multi := EvaluateAll(dataset, []analyzer.Rubric{analyzer.RubricV1, v2})
	require.Len(t, multi.Reports, 2)
	require.Len(t, multi.Diffs, 1)
	assert.Equal(t, "v1", multi.Diffs[0].BaselineName)
	assert.Equal(t, "v2-test", multi.Diffs[0].CandidateName)
	assert.Equal(t, Compare(multi.Reports[0], multi.Reports[1]), multi.Diffs[0])
}

func TestEvaluateEarlyBustedSlip(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()
	makeLeg := func(id int64, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:       id,
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: "p_hot",
				Side:     "OVER",
				Line:     new(55.5),
				Odds:     100,
			},
			Result:      res,
			DecimalOdds: 2.0,
		}
	}

	legs := []ReplayedLeg{
		makeLeg(1, parlay.Lost),
		makeLeg(2, parlay.Pending),
		makeLeg(3, parlay.Pending),
		makeLeg(4, parlay.NeedsGrading),
	}

	analyzerLegs := make([]analyzer.Leg, len(legs))
	for i, rl := range legs {
		analyzerLegs[i] = rl.Leg
	}
	fullGrade := analyzer.ScoreSlipWithRubric(analyzerLegs, games, stats, analyzer.RubricV1)
	singleGrade := analyzer.ScoreSlipWithRubric(analyzerLegs[:1], games, stats, analyzer.RubricV1)
	require.NotEqual(t, singleGrade.Letter, fullGrade.Letter, "4-leg parlay drag must change slip grade band")

	dataset := ReplayDataset{
		Coverage: DataCoverage{TotalLegs: 4, WithGameLogs: 4},
		Weeks: []ReplayedWeek{
			{
				Season:  2025,
				WeekNum: 4,
				Games:   games,
				Stats:   stats,
				Slips: []ReplayedSlip{
					{
						SlipID: 10,
						Result: parlay.Lost,
						Legs:   legs,
					},
				},
			},
		},
	}

	report := Evaluate(dataset, analyzer.RubricV1)

	// Slip is scored using all 4 pre-kickoff legs (including unsettled legs).
	slipBand, ok := bandByName(report.SlipBands, normalizeBand(fullGrade.Letter))
	require.True(t, ok)
	assert.Equal(t, 1, slipBand.Total)
	assert.Equal(t, 1, slipBand.Decided)
	assert.Equal(t, 1, slipBand.Losses)
	assert.InDelta(t, 0.5, slipBand.AvgImpliedProb, 1e-9)

	// Only the 1 settled leg is accumulated into leg bands, factors, weekly summary, and legOutcomes.
	bandA, ok := bandByName(report.LegBands, "A")
	require.True(t, ok)
	assert.Equal(t, 1, bandA.Total)
	assert.Equal(t, 1, bandA.Decided)
	assert.Equal(t, 1, bandA.Losses)
	assert.Equal(t, 0, bandA.Wins)

	require.Len(t, report.Weekly, 1)
	assert.Equal(t, 1, report.Weekly[0].DecidedLegs)

	hitRate, ok := factorByRule(report.Factors, "hit_rate")
	require.True(t, ok)
	assert.Equal(t, 1, hitRate.Count)
	assert.Equal(t, 1, hitRate.Losses)
	assert.Len(t, report.legOutcomes, 1)

	parlayDrag, ok := factorByRule(report.SlipFactors, "parlay_drag")
	require.True(t, ok)
	assert.Equal(t, 1, parlayDrag.Count)
	assert.Equal(t, 1, parlayDrag.Losses)
}

func TestEvaluateSlipFactors(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()

	makeLeg := func(id int64, odds int, res parlay.Result) ReplayedLeg {
		return ReplayedLeg{
			Leg: analyzer.Leg{
				ID:       id,
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: "p_hot",
				Side:     "OVER",
				Line:     new(55.5),
				Odds:     odds,
			},
			Result:      res,
			DecimalOdds: parlay.AmericanToDecimal(odds),
		}
	}

	// Custom rubric to verify GradeRubricFor(rubric) labels and deltas are used.
	rubric := analyzer.RubricV1
	rubric.Name = "custom-slip-factors"
	rubric.ParlayDragPerLeg = -5
	rubric.ChalkDragPerExtraLeg = -8

	dataset := ReplayDataset{
		Weeks: []ReplayedWeek{
			{
				Season:  2025,
				WeekNum: 5,
				Games:   games,
				Stats:   stats,
				Slips: []ReplayedSlip{
					{
						// 4-leg Won slip -> triggers parlay_drag (2 * -5 = -10), 4.0 decimal odds (implied 0.25)
						SlipID:       1,
						Result:       parlay.Won,
						CombinedOdds: 4.0,
						Legs: []ReplayedLeg{
							makeLeg(1, 100, parlay.Won),
							makeLeg(2, 100, parlay.Won),
							makeLeg(3, 100, parlay.Won),
							makeLeg(4, 100, parlay.Won),
						},
					},
					{
						// 4-leg Lost slip with 2 heavy chalk legs (-600) -> triggers parlay_drag (-10) and chalk_drag (-8), 2.0 decimal odds (implied 0.50)
						SlipID:       2,
						Result:       parlay.Lost,
						CombinedOdds: 2.0,
						Legs: []ReplayedLeg{
							makeLeg(5, -600, parlay.Lost),
							makeLeg(6, -600, parlay.Won),
							makeLeg(7, 100, parlay.Won),
							makeLeg(8, 100, parlay.Won),
						},
					},
					{
						// Push slip -> must be excluded from SlipFactors even though it has 4 legs
						SlipID:       3,
						Result:       parlay.Push,
						CombinedOdds: 4.0,
						Legs: []ReplayedLeg{
							makeLeg(9, 100, parlay.Push),
							makeLeg(10, 100, parlay.Push),
							makeLeg(11, 100, parlay.Push),
							makeLeg(12, 100, parlay.Push),
						},
					},
				},
			},
		},
	}

	report := Evaluate(dataset, rubric)
	doc := analyzer.GradeRubricFor(rubric)
	expectedLabels := make(map[string]string, len(doc.SlipFactors))
	for _, f := range doc.SlipFactors {
		expectedLabels[f.Rule] = f.Label
	}

	parlayDrag, ok := factorByRule(report.SlipFactors, "parlay_drag")
	require.True(t, ok)
	assert.Equal(t, expectedLabels["parlay_drag"], parlayDrag.Label)
	assert.Equal(t, 2, parlayDrag.Count)
	assert.Equal(t, 1, parlayDrag.Wins)
	assert.Equal(t, 1, parlayDrag.Losses)
	assert.InDelta(t, 0.5, parlayDrag.WinRate, 1e-9)
	assert.InDelta(t, 0.375, parlayDrag.AvgImpliedProb, 1e-9)
	assert.InDelta(t, 0.125, parlayDrag.Lift, 1e-9)
	assert.InDelta(t, -5.0, parlayDrag.AvgDelta, 1e-9)

	chalkDrag, ok := factorByRule(report.SlipFactors, "chalk_drag")
	require.True(t, ok)
	assert.Equal(t, expectedLabels["chalk_drag"], chalkDrag.Label)
	assert.Equal(t, 1, chalkDrag.Count)
	assert.Equal(t, 0, chalkDrag.Wins)
	assert.Equal(t, 1, chalkDrag.Losses)
	assert.InDelta(t, 0.0, chalkDrag.WinRate, 1e-9)
	assert.InDelta(t, 0.5, chalkDrag.AvgImpliedProb, 1e-9)
	assert.InDelta(t, -0.5, chalkDrag.Lift, 1e-9)
	assert.InDelta(t, -8.0, chalkDrag.AvgDelta, 1e-9)
}

func TestCompareRubricsAndIdentityMigrations(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()

	dataset := ReplayDataset{
		Weeks: []ReplayedWeek{
			{
				Season:  2025,
				WeekNum: 4,
				Games:   games,
				Stats:   stats,
				Slips: []ReplayedSlip{
					{
						SlipID:       1,
						Result:       parlay.Won,
						CombinedOdds: 2.0,
						Legs: []ReplayedLeg{
							{
								Leg: analyzer.Leg{
									ID: 101, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_hot", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Won,
								DecimalOdds: 2.0,
							},
							{
								Leg: analyzer.Leg{
									ID: 102, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p_cold", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Lost,
								DecimalOdds: 2.0,
							},
						},
					},
				},
			},
		},
	}

	cand := analyzer.RubricV1
	cand.Name = "v2-test"
	cand.HitRate = 0
	cand.MedianTrap = 0

	baseReport, candReport, diff := CompareRubrics(dataset, analyzer.RubricV1, cand)
	assert.Equal(t, Compare(baseReport, candReport), diff)
	require.Len(t, diff.Migrations, 2)

	// Reordering candidate legOutcomes must produce the exact same migrations.
	reorderedCand := candReport
	reorderedCand.legOutcomes = []legOutcome{candReport.legOutcomes[1], candReport.legOutcomes[0]}
	assert.Equal(t, diff.Migrations, Compare(baseReport, reorderedCand).Migrations)

	// Mismatched / subset legOutcomes must only migrate matching (SlipID, LegID) keys.
	subsetCand := candReport
	subsetCand.legOutcomes = []legOutcome{
		{slipID: 999, legID: 999, band: "F", decided: true, won: false},
		candReport.legOutcomes[0],
	}
	subsetDiff := Compare(baseReport, subsetCand)
	require.Len(t, subsetDiff.Migrations, 1)
	assert.Equal(t, MigrationCell{
		FromBand: "A",
		ToBand:   "B",
		Total:    1,
		Decided:  1,
		Wins:     1,
		Losses:   0,
		WinRate:  1.0,
	}, subsetDiff.Migrations[0])

	// Empty legOutcomes (e.g., reports decoded from JSON) must return empty migrations cleanly.
	emptyCand := candReport
	emptyCand.legOutcomes = nil
	assert.Empty(t, Compare(baseReport, emptyCand).Migrations)

	// Zero-ID fixtures fall back to (SlipID, legIdx).
	zeroIDBase := Report{
		RubricName: "base",
		legOutcomes: []legOutcome{
			{slipID: 7, legID: 0, legIdx: 0, band: "A", decided: true, won: true},
			{slipID: 7, legID: 0, legIdx: 1, band: "C", decided: true, won: false},
		},
	}
	zeroIDCand := Report{
		RubricName: "cand",
		legOutcomes: []legOutcome{
			{slipID: 7, legID: 0, legIdx: 1, band: "B", decided: true, won: false},
			{slipID: 7, legID: 0, legIdx: 0, band: "B", decided: true, won: true},
		},
	}
	zeroIDDiff := Compare(zeroIDBase, zeroIDCand)
	require.Len(t, zeroIDDiff.Migrations, 2)
	assert.Equal(t, "A", zeroIDDiff.Migrations[0].FromBand)
	assert.Equal(t, "B", zeroIDDiff.Migrations[0].ToBand)
	assert.Equal(t, 1, zeroIDDiff.Migrations[0].Wins)
	assert.Equal(t, "C", zeroIDDiff.Migrations[1].FromBand)
	assert.Equal(t, "B", zeroIDDiff.Migrations[1].ToBand)
	assert.Equal(t, 1, zeroIDDiff.Migrations[1].Losses)
}

func TestEvaluateMarketLegsAndMigrations(t *testing.T) {
	t.Parallel()

	games := testGame()
	stats := testStats()

	dataset := ReplayDataset{
		Weeks: []ReplayedWeek{
			{
				Season:   2025,
				WeekNum:  4,
				WeekType: "REG",
				Games:    games,
				Stats:    stats,
				MarketLegs: []ReplayedLeg{
					{
						Leg: analyzer.Leg{
							GameID: "g1", Market: "REC_YDS", Team: "BUF",
							PlayerID: "p_hot", Side: "OVER", Line: new(55.5), Odds: 100,
						},
						Result:      parlay.Won,
						DecimalOdds: 2.0,
					},
					{
						Leg: analyzer.Leg{
							GameID: "g1", Market: "REC_YDS", Team: "BUF",
							PlayerID: "p_cold", Side: "OVER", Line: new(55.5), Odds: -110,
						},
						Result:      parlay.Lost,
						DecimalOdds: parlay.AmericanToDecimal(-110),
					},
				},
			},
			{
				Season:   2025,
				WeekNum:  5,
				WeekType: "REG",
				Games:    games,
				Stats:    stats,
				MarketLegs: []ReplayedLeg{
					{
						Leg: analyzer.Leg{
							GameID: "g1", Market: "REC_YDS", Team: "BUF",
							PlayerID: "p_hot", Side: "OVER", Line: new(55.5), Odds: 100,
						},
						Result:      parlay.Push,
						DecimalOdds: 2.0,
					},
					{
						Leg: analyzer.Leg{
							GameID: "g1", Market: "REC_YDS", Team: "BUF",
							PlayerID: "p_cold", Side: "OVER", Line: new(55.5), Odds: 100,
						},
						Result:      parlay.Won,
						DecimalOdds: 2.0,
					},
				},
			},
		},
	}

	report := Evaluate(dataset, analyzer.RubricV1)

	// Slip sections must remain completely empty in market-only datasets.
	for _, b := range report.SlipBands {
		assert.Zero(t, b.Total, "slip band %s should be empty", b.Band)
		assert.Zero(t, b.Decided, "slip band %s decided should be empty", b.Band)
	}
	assert.Empty(t, report.SlipFactors)
	require.Len(t, report.Weekly, 2)
	for _, ws := range report.Weekly {
		for _, b := range ws.SlipBands {
			assert.Zero(t, b.Total)
		}
	}

	// Band A: 1 Won (at 2.0) + 1 Push (at 2.0) -> Total=2, Decided=1, Wins=1, Pushes=1.
	bandA, ok := bandByName(report.LegBands, "A")
	require.True(t, ok)
	assert.Equal(t, 2, bandA.Total)
	assert.Equal(t, 1, bandA.Decided)
	assert.Equal(t, 1, bandA.Wins)
	assert.Equal(t, 1, bandA.Pushes)
	assert.InDelta(t, 1.0, bandA.WinRate, 1e-9)
	assert.InDelta(t, 0.5, bandA.AvgImpliedProb, 1e-9)
	assert.InDelta(t, 0.5, bandA.Lift, 1e-9)
	assert.InDelta(t, 1.0, bandA.FlatROI, 1e-9)
	assert.InDelta(t, 0.25, bandA.BrierScore, 1e-9)

	// Factor attribution must include hit_rate and median_trap from MarketLegs.
	hitRate, ok := factorByRule(report.Factors, "hit_rate")
	require.True(t, ok)
	assert.Equal(t, 1, hitRate.Count)
	assert.Equal(t, 1, hitRate.Wins)

	medianTrap, ok := factorByRule(report.Factors, "median_trap")
	require.True(t, ok)
	assert.Equal(t, 2, medianTrap.Count)
	assert.Equal(t, 1, medianTrap.Wins)
	assert.Equal(t, 1, medianTrap.Losses)

	// Rubric comparison must match each market leg across weeks without key collisions.
	cand := analyzer.RubricV1
	cand.Name = "v2-market"
	cand.HitRate = 0
	cand.MedianTrap = 0

	_, _, diff := CompareRubrics(dataset, analyzer.RubricV1, cand)
	totalMigrated := 0
	for _, cell := range diff.Migrations {
		totalMigrated += cell.Total
	}
	assert.Equal(t, 4, totalMigrated)
}
