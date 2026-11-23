package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
)

func TestRubricRegistry(t *testing.T) {
	t.Parallel()

	r, ok := analyzer.RubricByName("v1")
	require.True(t, ok)
	assert.Equal(t, analyzer.RubricV1, r)

	_, ok = analyzer.RubricByName("nonexistent")
	assert.False(t, ok)

	names := analyzer.RubricNames()
	assert.Contains(t, names, "v1")

	rubrics := analyzer.BuiltInRubrics()
	require.Len(t, rubrics, len(names))
	assert.Equal(t, analyzer.RubricV1, rubrics[0])

	// Mutating the returned slice must not affect the registry.
	rubrics[0].Name = "mutated"
	fresh := analyzer.BuiltInRubrics()
	assert.Equal(t, "v1", fresh[0].Name)
}

func TestRubricV1Equivalence(t *testing.T) {
	t.Parallel()

	games := map[string]analyzer.Game{
		"buf-mia": {
			ID:     "buf-mia",
			Home:   "BUF",
			Away:   "MIA",
			Indoor: false,
			Weather: &analyzer.Weather{
				WindMPH: 22,
			},
			Nfelo: &analyzer.Nfelo{
				HomeSpread: 5.5,
				HomeQBAdj:  -35,
			},
			Injuries: map[string][]analyzer.Injury{
				"BUF": {
					{AthleteID: "wr2", FullName: "Gabe Davis", Position: "WR", Status: "Out"},
				},
			},
		},
	}

	stats := map[string]analyzer.PlayerStats{
		"allen": {
			Position: "QB",
			Games: map[string][]float64{
				"PASS_YDS": {280, 275, 290, 270, 260},
			},
			Usage: &analyzer.PlayerUsage{
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 1}, {WeekNum: 2}, {WeekNum: 3}, {WeekNum: 4},
				},
				SeasonSnapAvg:     0.85,
				Last3SnapAvg:      0.95,
				SeasonTargetShare: 0,
				Last3TargetShare:  0,
			},
		},
		"diggs": {
			Position: "WR",
			Games: map[string][]float64{
				"REC_YDS": {90, 85, 70, 60, 50},
			},
		},
	}

	legs := []analyzer.Leg{
		{
			ID: 1, Owner: "Adam", GameID: "buf-mia", Market: "SPREAD",
			Team: "BUF", Side: "TEAM", Line: new(-3.5), Odds: -110,
		},
		{
			ID: 2, Owner: "Adam", GameID: "buf-mia", Market: "PASS_YDS",
			Team: "BUF", PlayerID: "allen", PlayerName: "Josh Allen",
			Side: "OVER", Line: new(265.5), Odds: -110,
		},
		{
			ID: 3, Owner: "Adam", GameID: "buf-mia", Market: "REC_YDS",
			Team: "BUF", PlayerID: "diggs", PlayerName: "Stefon Diggs",
			Side: "OVER", Line: new(75.5), Odds: -300,
		},
		{
			ID: 4, Owner: "Adam", GameID: "buf-mia", Market: "TOTAL",
			Side: "OVER", Line: new(44.0), Odds: -350,
		},
		{
			ID: 5, Owner: "Adam", Market: "OTHER", Team: "BUF", Odds: -110,
		},
	}

	for _, leg := range legs {
		want := analyzer.ScoreLeg(leg, games, stats)
		got := analyzer.ScoreLegWithRubric(leg, games, stats, analyzer.RubricV1)
		assert.Equal(t, want, got, "leg %d mismatch", leg.ID)
	}

	wantSlip := analyzer.ScoreSlip(legs, games, stats)
	gotSlip := analyzer.ScoreSlipWithRubric(legs, games, stats, analyzer.RubricV1)
	assert.Equal(t, wantSlip, gotSlip)
}

func TestRubricOverrideIsolation(t *testing.T) {
	t.Parallel()

	games := map[string]analyzer.Game{
		"buf-mia": {
			ID:   "buf-mia",
			Home: "BUF",
			Away: "MIA",
			Nfelo: &analyzer.Nfelo{
				HomeSpread: -6.0,
			},
		},
	}
	leg := analyzer.Leg{
		ID: 1, Owner: "Adam", GameID: "buf-mia", Market: "SPREAD",
		Team: "BUF", Side: "TEAM", Line: new(-3.5), Odds: -110,
	}

	baseGrade := analyzer.ScoreLeg(leg, games, nil)

	custom := analyzer.RubricV1
	custom.Name = "custom"
	custom.NfeloCushion = 15
	custom.HookSoftened = -1
	custom.BandA = 80

	customGrade := analyzer.ScoreLegWithRubric(leg, games, nil, custom)
	assert.NotEqual(t, baseGrade.Score, customGrade.Score)
	assert.Equal(t, "A", customGrade.Letter)

	// ActiveRubric and subsequent ScoreLeg calls must remain unaffected.
	assert.Equal(t, analyzer.RubricV1, analyzer.ActiveRubric)
	assert.Equal(t, baseGrade, analyzer.ScoreLeg(leg, games, nil))
}

func TestRubricOverrideSlipFactors(t *testing.T) {
	t.Parallel()

	games := map[string]analyzer.Game{
		"buf-mia": {ID: "buf-mia", Home: "BUF", Away: "MIA"},
	}
	legs := []analyzer.Leg{
		{
			ID:         1,
			Owner:      "Adam",
			GameID:     "buf-mia",
			Market:     "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Odds:       -110,
		},
		{
			ID:         2,
			Owner:      "Adam",
			GameID:     "buf-mia",
			Market:     "REC_YDS",
			Team:       "BUF",
			PlayerName: "Stefon Diggs",
			Side:       "OVER",
			Line:       new(75.5),
			Odds:       -110,
		},
		{ID: 3, Owner: "Adam", GameID: "buf-mia", Market: "TOTAL", Side: "OVER", Line: new(49.5), Odds: -110},
		{ID: 4, Owner: "Adam", GameID: "buf-mia", Market: "MONEYLINE", Team: "MIA", Side: "TEAM", Odds: -300},
	}

	baseSlip := analyzer.ScoreSlip(legs, games, nil)

	custom := analyzer.RubricV1
	custom.StackBonus = 12
	custom.ParlayDragPerLeg = -6
	customSlip := analyzer.ScoreSlipWithRubric(legs, games, nil, custom)

	assert.NotEqual(t, baseSlip.Score, customSlip.Score)
	assert.Equal(t, baseSlip, analyzer.ScoreSlip(legs, games, nil))
}
