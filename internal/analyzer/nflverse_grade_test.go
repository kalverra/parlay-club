package analyzer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
)

func epaGames(homeEPA, awayEPA *analyzer.TeamDefenseProfile) map[string]analyzer.Game {
	return map[string]analyzer.Game{
		"buf-mia": {
			ID:      "buf-mia",
			Home:    "BUF",
			Away:    "MIA",
			HomeEPA: homeEPA,
			AwayEPA: awayEPA,
		},
	}
}

func TestToughMatchup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		epa  *analyzer.TeamDefenseProfile
		want []want
	}{
		{
			name: "pass over against rank 5 pass defense warns",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
			},
			epa:  &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 5},
			want: []want{{rule: "tough_matchup", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "receiving over against rank 1 pass defense warns",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(72.5), -110),
			},
			epa:  &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 1},
			want: []want{{rule: "tough_matchup", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "pass over against rank 6 pass defense does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
			},
			epa: &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 6},
		},
		{
			name: "rush over against rank 5 rush defense warns",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "RUSH_YDS", "BUF", "James Cook", "OVER", new(68.5), -110),
			},
			epa:  &analyzer.TeamDefenseProfile{DefRushEPAAllowedRank: 5},
			want: []want{{rule: "tough_matchup", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "rush att over against rank 6 rush defense does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "RUSH_ATT", "BUF", "James Cook", "OVER", new(15.5), -110),
			},
			epa: &analyzer.TeamDefenseProfile{DefRushEPAAllowedRank: 6},
		},
		{
			name: "under does not trigger tough matchup",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "UNDER", new(265.5), -110),
			},
			epa: &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 1},
		},
		{
			name: "nil opponent EPA fails soft",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(265.5), -110),
			},
			epa: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, epaGames(nil, test.epa), nil), test.want)
		})
	}
}

func TestShrinkingRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		legs  []analyzer.Leg
		usage *analyzer.PlayerUsage
		want  []want
	}{
		{
			name: "snap share drop of exactly 15 percentage points warns",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Dalton Kincaid", "OVER", new(45.5), -110),
					"p1",
				),
			},
			usage: &analyzer.PlayerUsage{
				SeasonSnapAvg: 0.70,
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 8, OffensePct: 0.55},
					{WeekNum: 7, OffensePct: 0.55},
					{WeekNum: 6, OffensePct: 0.85},
				},
			},
			want: []want{{rule: "shrinking_role", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "snap share drop of 14 percentage points does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Dalton Kincaid", "OVER", new(45.5), -110),
					"p1",
				),
			},
			usage: &analyzer.PlayerUsage{
				SeasonSnapAvg: 0.70,
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 8, OffensePct: 0.56},
					{WeekNum: 7, OffensePct: 0.56},
					{WeekNum: 6, OffensePct: 0.85},
				},
			},
		},
		{
			name: "target share drop of exactly 8 percentage points warns",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "RECEPTIONS", "BUF", "Dalton Kincaid", "OVER", new(4.5), -110),
					"p1",
				),
			},
			usage: &analyzer.PlayerUsage{
				SeasonSnapAvg:     0.80,
				SeasonTargetShare: 0.24,
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 8, OffensePct: 0.80, TargetShare: 0.16},
					{WeekNum: 7, OffensePct: 0.80, TargetShare: 0.16},
					{WeekNum: 6, OffensePct: 0.80, TargetShare: 0.30},
				},
			},
			want: []want{{rule: "shrinking_role", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "fewer than 3 weeks fails soft",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Dalton Kincaid", "OVER", new(45.5), -110),
					"p1",
				),
			},
			usage: &analyzer.PlayerUsage{
				SeasonSnapAvg: 0.80,
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 8, OffensePct: 0.50},
					{WeekNum: 7, OffensePct: 0.50},
				},
			},
		},
		{
			name: "under does not trigger shrinking role",
			legs: []analyzer.Leg{
				withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Dalton Kincaid", "UNDER", new(45.5), -110),
					"p1",
				),
			},
			usage: &analyzer.PlayerUsage{
				SeasonSnapAvg: 0.80,
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 8, OffensePct: 0.50},
					{WeekNum: 7, OffensePct: 0.50},
					{WeekNum: 6, OffensePct: 0.90},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			stats := map[string]analyzer.PlayerStats{
				"p1": {Usage: test.usage},
			}
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), stats), test.want)
		})
	}
}

func TestPaceClash(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		legs []analyzer.Leg
		home *analyzer.TeamDefenseProfile
		away *analyzer.TeamDefenseProfile
		want []want
	}{
		{
			name: "total over with both teams bottom 8 pace and bottom 10 pass rate warns",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(45.5), -110),
			},
			home: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 25, PassRateRank: 23},
			away: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 30, PassRateRank: 28},
			want: []want{{rule: "pace_clash", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "total over with one team rank 24 pace does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(45.5), -110),
			},
			home: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 24, PassRateRank: 23},
			away: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 30, PassRateRank: 28},
		},
		{
			name: "total under with both teams top 8 pace and top 8 off pass EPA warns",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(49.5), -110),
			},
			home: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 8, OffPassEPARank: 8},
			away: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 3, OffPassEPARank: 2},
			want: []want{{rule: "pace_clash", severity: analyzer.Warn, ids: []int64{1}}},
		},
		{
			name: "total under with one team rank 9 off pass EPA does not trigger",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(49.5), -110),
			},
			home: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 4, OffPassEPARank: 9},
			away: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 3, OffPassEPARank: 2},
		},
		{
			name: "nil away EPA fails soft",
			legs: []analyzer.Leg{
				leg(1, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(45.5), -110),
			},
			home: &analyzer.TeamDefenseProfile{PlaysPerGameRank: 30, PassRateRank: 30},
			away: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, epaGames(test.home, test.away), nil), test.want)
		})
	}
}

func TestTDSpikeRisk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		legs  []analyzer.Leg
		stats map[string]analyzer.PlayerStats
		want  []want
	}{
		{
			name: "anytime TD with 3 TDs in last 3 games on low target share and carries",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "Dawson Knox", "YES", nil, 240), "p1"),
			},
			stats: map[string]analyzer.PlayerStats{
				"p1": {
					Games: map[string][]float64{"ANYTIME_TD": {1, 1, 1, 0}},
					Usage: &analyzer.PlayerUsage{
						Last3TargetShare: 0.11,
						Weeks: []analyzer.PlayerWeekUsage{
							{Carries: 0}, {Carries: 1}, {Carries: 0},
						},
					},
				},
			},
			want: []want{{rule: "td_spike_risk", severity: analyzer.Info, ids: []int64{1}}},
		},
		{
			name: "anytime TD at 15% target share boundary does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "Dawson Knox", "YES", nil, 240), "p1"),
			},
			stats: map[string]analyzer.PlayerStats{
				"p1": {
					Games: map[string][]float64{"ANYTIME_TD": {1, 1, 1}},
					Usage: &analyzer.PlayerUsage{
						Last3TargetShare: 0.15,
						Weeks: []analyzer.PlayerWeekUsage{
							{Carries: 0}, {Carries: 0}, {Carries: 0},
						},
					},
				},
			},
		},
		{
			name: "anytime TD at 8 carries/game boundary does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "ANYTIME_TD", "BUF", "Ray Davis", "YES", nil, 150), "p1"),
			},
			stats: map[string]analyzer.PlayerStats{
				"p1": {
					Games: map[string][]float64{"ANYTIME_TD": {1, 1, 1}},
					Usage: &analyzer.PlayerUsage{
						Last3TargetShare: 0.05,
						Weeks: []analyzer.PlayerWeekUsage{
							{Carries: 8}, {Carries: 8}, {Carries: 8},
						},
					},
				},
			},
		},
		{
			name: "pass TDs over with 6 TDs and non-positive EPA triggers info",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "PASS_TDS", "BUF", "Josh Allen", "OVER", new(1.5), -120), "p1"),
			},
			stats: map[string]analyzer.PlayerStats{
				"p1": {
					Games: map[string][]float64{"PASS_TDS": {2, 2, 2}},
					Usage: &analyzer.PlayerUsage{
						Weeks: []analyzer.PlayerWeekUsage{
							{TotalEPA: -1.0}, {TotalEPA: 0.0}, {TotalEPA: -0.5},
						},
					},
				},
			},
			want: []want{{rule: "td_spike_risk", severity: analyzer.Info, ids: []int64{1}}},
		},
		{
			name: "pass TDs over with positive EPA does not trigger",
			legs: []analyzer.Leg{
				withPlayerID(leg(1, "Adam", "buf-mia", "PASS_TDS", "BUF", "Josh Allen", "OVER", new(1.5), -120), "p1"),
			},
			stats: map[string]analyzer.PlayerStats{
				"p1": {
					Games: map[string][]float64{"PASS_TDS": {2, 2, 2}},
					Usage: &analyzer.PlayerUsage{
						Weeks: []analyzer.PlayerWeekUsage{
							{TotalEPA: 5.0}, {TotalEPA: 4.0}, {TotalEPA: 6.0},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertWarnings(t, analyzer.AnalyzeSlip(test.legs, games(), test.stats), test.want)
		})
	}
}

func TestScoreLeg(t *testing.T) {
	t.Parallel()

	t.Run("skipped legs return empty letter and zero score", func(t *testing.T) {
		t.Parallel()
		skipped := []analyzer.Leg{
			leg(1, "Adam", "buf-mia", "OTHER", "BUF", "", "", nil, -110),
			leg(2, "Adam", "", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110),
			leg(3, "Adam", "missing", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110),
		}
		for _, l := range skipped {
			g := analyzer.ScoreLeg(l, games(), nil)
			assert.Equal(t, l.ID, g.LegID)
			assert.Empty(t, g.Letter)
			assert.Zero(t, g.Score)
			assert.Empty(t, g.Factors)
		}
	})

	t.Run("neutral leg starts at 75 (B) with no factors", func(t *testing.T) {
		t.Parallel()
		g := analyzer.ScoreLeg(leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.0), -110), games(), nil)
		assert.Equal(t, "B", g.Letter)
		assert.Equal(t, 75, g.Score)
		assert.Empty(t, g.Factors)
	})

	t.Run("A-grade leg with L5 hit rate, rising usage, and weak pass defense", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(60.5), -110), "p1")
		stats := map[string]analyzer.PlayerStats{
			"p1": {
				Games: map[string][]float64{"REC_YDS": {80, 75, 65, 90, 50}},
				Usage: &analyzer.PlayerUsage{
					Last3SnapAvg:  0.88,
					SeasonSnapAvg: 0.78,
					Weeks: []analyzer.PlayerWeekUsage{
						{OffensePct: 0.90}, {OffensePct: 0.88}, {OffensePct: 0.86},
					},
				},
			},
		}
		g := analyzer.ScoreLeg(l, epaGames(nil, &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 28}), stats)
		assert.Equal(t, "A", g.Letter)
		assert.Equal(t, 100, g.Score)
		assertSumEqualsScore(t, g)
	})

	t.Run("hook softened by nfelo cushion and adds cushion bonus", func(t *testing.T) {
		t.Parallel()
		l := leg(1, "Adam", "buf-mia", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110)
		// In nfeloGames, BUF is Away and MIA is Home, so HomeSpread: +5.5 means BUF -5.5.
		g := analyzer.ScoreLeg(l, nfeloGames(&analyzer.Nfelo{HomeSpread: 5.5}), nil)
		// 75 - 4 (softened hook) + 8 (nfelo_cushion) = 79 (B)
		assert.Equal(t, "B", g.Letter)
		assert.Equal(t, 79, g.Score)
		assertSumEqualsScore(t, g)
	})

	t.Run("chalk scaling and category caps preserve factor sum invariant", func(t *testing.T) {
		t.Parallel()
		l := withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(260.5), -500), "p1")
		gs := map[string]analyzer.Game{
			"buf-mia": {
				ID:      "buf-mia",
				Home:    "BUF",
				Away:    "MIA",
				Weather: &analyzer.Weather{WindMPH: 22},
				Nfelo:   &analyzer.Nfelo{HomeQBAdj: -40},
				AwayEPA: &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 2},
			},
		}
		stats := map[string]analyzer.PlayerStats{
			"p1": {
				Games: map[string][]float64{"PASS_YDS": {220, 210, 200}},
				Usage: &analyzer.PlayerUsage{
					SeasonSnapAvg: 0.90,
					Weeks: []analyzer.PlayerWeekUsage{
						{OffensePct: 0.70}, {OffensePct: 0.70}, {OffensePct: 0.95},
					},
				},
			},
		}
		g := analyzer.ScoreLeg(l, gs, stats)
		// Form & usage capped at -20 (median_trap -15 + shrinking_role -15 -> -20)
		// Matchup & env capped at -20 (tough_matchup -12 + wind -18 + qb_drag -8 -> -20)
		// Line & price: chalk -500 -> -18
		// Total: 75 - 20 - 20 - 18 = 17 (F)
		assert.Equal(t, "F", g.Letter)
		assert.Equal(t, 17, g.Score)
		assertSumEqualsScore(t, g)
	})

	t.Run("grade band boundaries A through F", func(t *testing.T) {
		t.Parallel()
		hitStats := map[string]analyzer.PlayerStats{
			"p1": {Games: map[string][]float64{"REC_YDS": {80, 75, 65, 90, 70}}},
		}
		tests := []struct {
			name  string
			leg   analyzer.Leg
			games map[string]analyzer.Game
			stats map[string]analyzer.PlayerStats
			score int
			grade string
		}{
			{
				name: "85 is A",
				leg: withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(60.5), -110),
					"p1",
				),
				games: games(),
				stats: hitStats,
				score: 85,
				grade: "A",
			},
			{
				name: "84 is B",
				leg: withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(60.5), -264),
					"p1",
				),
				games: epaGames(nil, &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 28}),
				stats: hitStats,
				score: 84,
				grade: "B",
			},
			{
				name: "73 is B",
				leg: withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(60.5), -110),
					"p1",
				),
				games: epaGames(nil, &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 3}),
				stats: hitStats,
				score: 73,
				grade: "B",
			},
			{
				name:  "72 is C",
				leg:   leg(1, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
				games: weatherGames(&analyzer.Weather{PrecipIn: 0.05, WindMPH: 5}, false),
				score: 72,
				grade: "C",
			},
			{
				name: "60 is C",
				leg: withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(85.5), -110),
					"p1",
				),
				games: games(),
				stats: hitStats,
				score: 60,
				grade: "C",
			},
			{
				name: "59 is D",
				leg:  leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110),
				games: map[string]analyzer.Game{
					"buf-mia": {
						ID:      "buf-mia",
						Home:    "BUF",
						Away:    "MIA",
						Weather: &analyzer.Weather{WindMPH: 16},
						Nfelo:   &analyzer.Nfelo{HomeQBAdj: -35},
					},
				},
				score: 59,
				grade: "D",
			},
			{
				name: "48 is D",
				leg: withPlayerID(
					leg(1, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(85.5), -110),
					"p1",
				),
				games: epaGames(nil, &analyzer.TeamDefenseProfile{DefPassEPAAllowedRank: 3}),
				stats: hitStats,
				score: 48,
				grade: "D",
			},
			{
				name:  "47 is F",
				leg:   leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -230),
				games: weatherGames(&analyzer.Weather{WindMPH: 22}, false),
				score: 47,
				grade: "F",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				g := analyzer.ScoreLeg(tc.leg, tc.games, tc.stats)
				assert.Equal(t, tc.score, g.Score)
				assert.Equal(t, tc.grade, g.Letter)
				assertSumEqualsScore(t, g)
			})
		}
	})
}

func TestScoreSlip(t *testing.T) {
	t.Parallel()

	t.Run("empty or ungradable slip returns empty grade", func(t *testing.T) {
		t.Parallel()
		got := analyzer.ScoreSlip([]analyzer.Leg{leg(1, "Adam", "", "OTHER", "BUF", "", "", nil, -110)}, games(), nil)
		assert.Empty(t, got.Letter)
		assert.Zero(t, got.Score)
		require.Len(t, got.LegGrades, 1)
		assert.Empty(t, got.LegGrades[0].Letter)
	})

	t.Run("shootout stack boosts slip grade", func(t *testing.T) {
		t.Parallel()
		legs := []analyzer.Leg{
			withPlayerID(leg(1, "Adam", "buf-mia", "PASS_YDS", "BUF", "Josh Allen", "OVER", new(255.5), -110), "qb"),
			withPlayerID(leg(2, "Adam", "buf-mia", "REC_YDS", "BUF", "Khalil Shakir", "OVER", new(75.5), -110), "wr"),
			leg(3, "Adam", "buf-mia", "TOTAL", "", "", "OVER", new(48.5), -110),
		}
		got := analyzer.ScoreSlip(legs, games(), nil)
		// Each leg baseline 75 -> avg 75 + 8 (stack_shootout) = 83 (B)
		assert.Equal(t, "B", got.Letter)
		assert.Equal(t, 83, got.Score)
		require.Len(t, got.Factors, 1)
		assert.Equal(t, "stack_shootout", got.Factors[0].Rule)
		assert.Equal(t, 8, got.Factors[0].Delta)
		assert.NotEmpty(t, got.Summary)
	})

	t.Run("danger conflict, parlay drag, and extra chalk drag reduce slip grade", func(t *testing.T) {
		t.Parallel()
		legs := []analyzer.Leg{
			leg(1, "Adam", "buf-mia", "SPREAD", "MIA", "", "TEAM", new(5.5), -300),
			withPlayerID(leg(2, "Adam", "buf-mia", "RUSH_YDS", "MIA", "De'Von Achane", "OVER", new(60.5), -300), "rb"),
			leg(3, "Adam", "buf-mia", "ML", "BUF", "", "TEAM", nil, -110),
			leg(4, "Adam", "buf-mia", "TOTAL", "", "", "UNDER", new(45.5), -110),
		}
		got := analyzer.ScoreSlip(legs, games(), nil)
		// Legs 1 & 2 have chalk -300 (-12 each -> 63), legs 3 & 4 are 75 -> avg 69
		// Slip factors: trailing_rb (-20), parlay_drag (-3 for 4th leg), chalk_drag (-5 for 2nd chalk)
		// Total: 69 - 20 - 3 - 5 = 41 (F)
		assert.Equal(t, "F", got.Letter)
		assert.Equal(t, 41, got.Score)
		assert.Equal(t, got, analyzer.ScoreSlip(legs, games(), nil), "ScoreSlip must be deterministic")
	})
}

func TestRuleDocsCoverageAndGradeRubric(t *testing.T) {
	t.Parallel()
	activeIDs := analyzer.RuleIDs()
	docs := analyzer.RuleDocs()

	require.Len(t, docs, len(activeIDs), "RuleDocs must match RuleIDs 1:1")

	docByID := make(map[string]analyzer.RuleDoc, len(docs))
	for _, doc := range docs {
		assert.NotEmpty(t, doc.ID)
		assert.NotEmpty(t, doc.Title)
		assert.NotEmpty(t, doc.Category)
		assert.NotEmpty(t, doc.Threshold)
		assert.NotEmpty(t, doc.Rationale)
		assert.NotEmpty(t, doc.Example)
		_, dup := docByID[doc.ID]
		assert.False(t, dup, "duplicate RuleDoc for %s", doc.ID)
		docByID[doc.ID] = doc
	}

	for _, id := range activeIDs {
		_, ok := docByID[id]
		assert.True(t, ok, "missing RuleDoc for active rule %s", id)
	}

	rubric := analyzer.GradeRubric()
	assert.Equal(t, 75, rubric.BaselineScore)
	require.Len(t, rubric.Bands, 5)
	assert.Equal(t, []string{"A", "B", "C", "D", "F"}, []string{
		rubric.Bands[0].Letter,
		rubric.Bands[1].Letter,
		rubric.Bands[2].Letter,
		rubric.Bands[3].Letter,
		rubric.Bands[4].Letter,
	})
	assert.NotEmpty(t, rubric.LegFactors)
	assert.NotEmpty(t, rubric.SlipFactors)
}

func assertSumEqualsScore(t *testing.T, g analyzer.LegGrade) {
	t.Helper()
	sum := 75
	for _, f := range g.Factors {
		sum += f.Delta
	}
	assert.Equal(t, sum, g.Score, "baseline 75 + sum(Factors.Delta) should equal Score")
}
