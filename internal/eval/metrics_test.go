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
