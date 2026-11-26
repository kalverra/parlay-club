package board_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
)

// exploreFixture builds a two-game slate with a graded A, B, and D row, a team
// total, an OTHER market, and a row whose game is missing from the cache.
func exploreFixture() ([]board.Row, map[string]analyzer.Game, map[string]analyzer.PlayerStats) {
	earlyKickoff := time.Date(2026, 9, 20, 17, 0, 0, 0, time.UTC)
	lateKickoff := time.Date(2026, 9, 20, 20, 25, 0, 0, time.UTC)

	games := map[string]analyzer.Game{
		"g1": {
			ID:   "g1",
			Home: "BUF",
			Away: "MIA",
			MarketScript: analyzer.MarketScript{
				HomeSpread:       -2.5,
				GameTotal:        49.5,
				HomeImpliedTotal: 26,
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
				DefPassEPAAllowedRank: 30,
				DefRushEPAAllowedRank: 29,
				DefPassEPAAllowed:     0.20,
			},
			Sharp: map[analyzer.SharpKey]analyzer.SharpQuote{
				{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}: {
					Line: new(235.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
				{MarketType: "REC_YDS", Team: "BUF", PlayerName: "khalil shakir", Side: "OVER"}: {
					Line: new(38.5), FairProb: 0.62, PowerFairProb: 0.62, Source: "Pinnacle", Tier: 1,
				},
			},
		},
		"g2": {
			ID:   "g2",
			Home: "KC",
			Away: "LV",
			MarketScript: analyzer.MarketScript{
				HomeSpread:       -3,
				GameTotal:        48.5,
				HomeImpliedTotal: 25.75,
				AwayImpliedTotal: 22.75,
				HasSpread:        true,
				HasTotal:         true,
			},
			HomeEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 12,
				DefRushEPAAllowedRank: 12,
			},
			AwayEPA: &analyzer.TeamDefenseProfile{
				DefPassEPAAllowedRank: 28,
				DefRushEPAAllowedRank: 28,
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
			Games:    map[string][]float64{"PASS_YDS": {290, 285, 280, 275, 270}},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6, PassYds: 285, PassAtt: 35, OffensePct: 0.98, ExpPassYds: 278,
			},
			Usage: &analyzer.PlayerUsage{Last3SnapAvg: 0.98, SeasonSnapAvg: 0.78},
		},
		"wr1": {
			Position: "WR",
			Role:     analyzer.RoleWR1,
			Games:    map[string][]float64{"REC_YDS": {95, 90, 88, 84, 82}},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6, RecYds: 96, Receptions: 7.5, TargetShare: 0.30, OffensePct: 0.95, ExpRecYds: 92,
			},
			Usage: &analyzer.PlayerUsage{
				Weeks:             []analyzer.PlayerWeekUsage{{TargetShare: 0.30, WOPR: 0.70}},
				Last3TargetShare:  0.30,
				SeasonTargetShare: 0.25,
				Last3SnapAvg:      0.90,
				SeasonSnapAvg:     0.80,
			},
		},
		"wr2": {
			Position: "WR",
			Role:     analyzer.RoleWR2,
			Games:    map[string][]float64{"REC_YDS": {50, 50, 50, 50, 50}},
		},
		"rb1": {
			Position: "RB",
			Role:     analyzer.RoleRB1,
			Games:    map[string][]float64{"RUSH_YDS": {90, 85, 82, 80, 78}},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6, RushYds: 92, Carries: 19, CarryShare: 0.70, OffensePct: 0.78, ExpRushYds: 86,
			},
		},
		"wr3": {
			Position: "WR",
			Role:     analyzer.RoleWR1,
			Games:    map[string][]float64{"REC_YDS": {95, 90, 88, 84, 82}},
			EWMA: &analyzer.PlayerEWMAUsage{
				GamesPlayed: 6, RecYds: 96, Receptions: 7.5, TargetShare: 0.30, OffensePct: 0.95, ExpRecYds: 92,
			},
		},
	}

	rows := []board.Row{
		{
			GameID: "g1", Market: "PASS_YDS", Team: "BUF", PlayerID: "qb1", PlayerName: "Josh Allen",
			Position: "QB", Side: "OVER", Line: new(235.5), Odds: -105, Category: board.CategoryPassing,
			Window: board.WindowSundayEarly, Kickoff: earlyKickoff, Text: "Allen PASS_YDS",
		},
		{
			GameID: "g1", Market: "REC_YDS", Team: "BUF", PlayerID: "wr1", PlayerName: "Khalil Shakir",
			Position: "WR", Side: "OVER", Line: new(38.5), Odds: -115, Category: board.CategoryReceiving,
			Window: board.WindowSundayEarly, Kickoff: earlyKickoff, Text: "Shakir REC_YDS",
		},
		{
			GameID: "g1", Market: "RUSH_YDS", Team: "BUF", PlayerID: "rb1", PlayerName: "James Cook",
			Position: "RB", Side: "OVER", Line: new(44.5), Odds: -108, Category: board.CategoryRushing,
			Window: board.WindowSundayEarly, Kickoff: earlyKickoff, Text: "Cook RUSH_YDS",
		},
		{
			GameID: "g1", Market: "REC_YDS", Team: "BUF", PlayerID: "wr2", PlayerName: "André Hollins",
			Position: "WR", Side: "OVER", Line: new(65.5), Odds: -110, Category: board.CategoryReceiving,
			Window: board.WindowSundayEarly, Kickoff: earlyKickoff, Text: "Hollins REC_YDS",
		},
		{
			GameID: "g1", Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-2.5), Odds: -110,
			Category: board.CategoryLines, Window: board.WindowSundayEarly, Kickoff: earlyKickoff,
			Text: "BUF SPREAD",
		},
		{
			GameID: "g1", Market: "TOTAL", Side: "OVER", Line: new(49.5), Odds: -110,
			Category: board.CategoryLines, Window: board.WindowSundayEarly, Kickoff: earlyKickoff,
			Text: "TOTAL",
		},
		{
			GameID: "g1", Market: "OTHER", Team: "BUF", PlayerID: "qb1", PlayerName: "Josh Allen",
			Side: "YES", Odds: 150, Category: board.CategoryLines, Window: board.WindowSundayEarly,
			Kickoff: earlyKickoff, Text: "OTHER",
		},
		{
			GameID: "g2", Market: "REC_YDS", Team: "KC", PlayerID: "wr3", PlayerName: "Xavier Worthy",
			Position: "WR", Side: "OVER", Line: new(38.5), Odds: -105, Category: board.CategoryReceiving,
			Window: board.WindowSundayLate, Kickoff: lateKickoff, Text: "Worthy REC_YDS",
		},
		{
			GameID: "g2", Market: "ML", Team: "KC", Side: "TEAM", Odds: -165, Category: board.CategoryLines,
			Window: board.WindowSundayLate, Kickoff: lateKickoff, Text: "KC ML",
		},
		{
			GameID: "g2", Market: "TEAM_TOTAL", Team: "KC", Side: "OVER", Line: new(27.5), Odds: -110,
			Category: board.CategoryTeamTotals, Window: board.WindowSundayLate, Kickoff: lateKickoff,
			Text: "KC TEAM_TOTAL",
		},
		{
			GameID: "gX", Market: "REC_YDS", Team: "SF", PlayerID: "x1", PlayerName: "Ghost Player",
			Position: "WR", Side: "OVER", Line: new(20.5), Odds: -110, Category: board.CategoryReceiving,
			Window: board.WindowSundayLate, Kickoff: lateKickoff, Text: "Ghost REC_YDS",
		},
	}
	return rows, games, stats
}

// exploreSignals returns the nflverse, injury, and open-slip context that gives
// the fixture's Shakir row a smash matchup, a slip fit, and high volume.
func exploreSignals() board.BoardSignals {
	_, games, _ := exploreFixture()
	return board.BoardSignals{
		TeamEPA: map[string]*analyzer.TeamDefenseProfile{
			"BUF": games["g1"].HomeEPA,
			"MIA": games["g1"].AwayEPA,
		},
		PlayerUsage: map[string]*analyzer.PlayerUsage{
			"wr1": {
				Weeks:             []analyzer.PlayerWeekUsage{{TargetShare: 0.30, WOPR: 0.70}},
				Last3TargetShare:  0.30,
				SeasonTargetShare: 0.25,
				Last3SnapAvg:      0.90,
				SeasonSnapAvg:     0.80,
			},
		},
		SlipLegs: []analyzer.Leg{{
			ID:       7,
			GameID:   "g1",
			Market:   "PASS_YDS",
			Team:     "BUF",
			PlayerID: "qb1",
			Side:     "OVER",
			Line:     new(235.5),
			Odds:     -105,
		}},
	}
}

// exploreTexts lists the matched rows' fixture labels in result order.
func exploreTexts(rows []board.ScoredRow) []string {
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = row.Row.Text
	}
	return texts
}

func TestExploreTargetedFilters(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()

	tests := []struct {
		name  string
		query board.ExploreQuery
		want  []string
	}{
		{
			name:  "player search ignores case",
			query: board.ExploreQuery{Q: "ALLEN"},
			want:  []string{"Allen PASS_YDS"},
		},
		{
			name:  "player search ignores accents",
			query: board.ExploreQuery{Q: "andre"},
			want:  []string{"Hollins REC_YDS"},
		},
		{
			name:  "team search matches part of the matchup",
			query: board.ExploreQuery{Q: "mia"},
			want: []string{
				"Allen PASS_YDS", "Cook RUSH_YDS", "Shakir REC_YDS", "BUF SPREAD", "TOTAL", "Hollins REC_YDS",
			},
		},
		{
			name:  "matchup search matches the whole string",
			query: board.ExploreQuery{Q: "MIA @ BUF"},
			want: []string{
				"Allen PASS_YDS", "Cook RUSH_YDS", "Shakir REC_YDS", "BUF SPREAD", "TOTAL", "Hollins REC_YDS",
			},
		},
		{
			name:  "market search matches the market label",
			query: board.ExploreQuery{Q: "rec yds"},
			want:  []string{"Worthy REC_YDS", "Shakir REC_YDS", "Hollins REC_YDS", "Ghost REC_YDS"},
		},
		{
			name:  "game filter keeps one game",
			query: board.ExploreQuery{GameID: "g2"},
			want:  []string{"Worthy REC_YDS", "KC ML", "KC TEAM_TOTAL"},
		},
		{
			name:  "player filter keeps one player",
			query: board.ExploreQuery{PlayerID: "wr1"},
			want:  []string{"Shakir REC_YDS"},
		},
		{
			name:  "targeted filters combine",
			query: board.ExploreQuery{GameID: "g1", PlayerID: "qb1"},
			want:  []string{"Allen PASS_YDS"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := board.Explore(rows, games, stats, tc.query)
			assert.Equal(t, tc.want, exploreTexts(got.Rows))
			assert.Equal(t, len(tc.want), got.TotalMatched)
		})
	}
}

func TestExploreOpenFilters(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()

	tests := []struct {
		name  string
		query board.ExploreQuery
		want  []string
	}{
		{
			name:  "category keeps the bet type",
			query: board.ExploreQuery{Category: board.CategoryReceiving},
			want:  []string{"Worthy REC_YDS", "Shakir REC_YDS", "Hollins REC_YDS", "Ghost REC_YDS"},
		},
		{
			name:  "single grade",
			query: board.ExploreQuery{Grades: []string{"A"}},
			want:  []string{"Allen PASS_YDS", "Cook RUSH_YDS", "Worthy REC_YDS"},
		},
		{
			name:  "grade range accepts lowercase and blanks",
			query: board.ExploreQuery{Grades: []string{"a", " B "}},
			want: []string{
				"Allen PASS_YDS", "Cook RUSH_YDS", "Worthy REC_YDS", "Shakir REC_YDS",
				"KC ML", "BUF SPREAD", "TOTAL",
			},
		},
		{
			name:  "window keeps one kickoff window",
			query: board.ExploreQuery{Window: board.WindowSundayLate},
			want:  []string{"Worthy REC_YDS", "KC ML", "KC TEAM_TOTAL", "Ghost REC_YDS"},
		},
		{
			name:  "unknown category and window never empty the list",
			query: board.ExploreQuery{Category: "bogus", Window: "bogus"},
			want: []string{
				"Allen PASS_YDS", "Cook RUSH_YDS", "Worthy REC_YDS", "Shakir REC_YDS",
				"KC ML", "BUF SPREAD", "TOTAL", "KC TEAM_TOTAL", "Hollins REC_YDS", "Ghost REC_YDS",
			},
		},
		{
			name: "facets combine",
			query: board.ExploreQuery{
				Category: board.CategoryReceiving,
				Grades:   []string{"A"},
				Window:   board.WindowSundayLate,
			},
			want: []string{"Worthy REC_YDS"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := board.Explore(rows, games, stats, tc.query)
			assert.Equal(t, tc.want, exploreTexts(got.Rows))
			assert.Equal(t, len(tc.want), got.TotalMatched)
		})
	}
}

func TestExploreSignalFilters(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()
	signals := exploreSignals()

	tests := []struct {
		name   string
		signal string
		want   []string
	}{
		{name: "smash", signal: board.SignalSmash, want: []string{"Shakir REC_YDS"}},
		{name: "volume", signal: board.SignalVolume, want: []string{"Shakir REC_YDS"}},
		{
			name:   "fits slip",
			signal: board.SignalFitsSlip,
			want:   []string{"Shakir REC_YDS", "TOTAL", "Hollins REC_YDS"},
		},
		{name: "role bump has no match", signal: board.SignalRoleBump, want: []string{}},
		{name: "unknown signal is ignored", signal: "bogus", want: exploreTexts(
			board.Explore(rows, games, stats, board.ExploreQuery{}).Rows,
		)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := board.Explore(rows, games, stats, board.ExploreQuery{Signal: tc.signal, Signals: signals})
			assert.Equal(t, tc.want, exploreTexts(got.Rows))
		})
	}

	t.Run("secondary signals count even when another badge is shown", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{Signals: signals})
		require.Len(t, got.Rows, 10)
		assert.Equal(t, board.BadgeSmashMatchup, got.Rows[3].Row.Badge, "Shakir shows his highest-priority badge")
		assert.Equal(t, 1, got.SignalCounts[board.SignalSmash])
		assert.Equal(t, 1, got.SignalCounts[board.SignalVolume])
		assert.Equal(t, 3, got.SignalCounts[board.SignalFitsSlip], "TOTAL and Hollins fit the slip leg too")
		assert.Equal(t, 0, got.SignalCounts[board.SignalRoleBump])
	})

	t.Run("falls back to an applied badge without signals", func(t *testing.T) {
		t.Parallel()
		withBadge := make([]board.Row, len(rows))
		copy(withBadge, rows)
		for i := range withBadge {
			if withBadge[i].Text == "Shakir REC_YDS" {
				withBadge[i].Badge = board.BadgeHighVolume
			}
		}
		got := board.Explore(withBadge, games, stats, board.ExploreQuery{Signal: board.SignalVolume})
		assert.Equal(t, []string{"Shakir REC_YDS"}, exploreTexts(got.Rows))
	})

	t.Run("signal labels read like the badges", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, board.BadgeSmashMatchup, board.SignalLabel(board.SignalSmash))
		assert.Equal(t, board.BadgeHighVolume, board.SignalLabel(board.SignalVolume))
		assert.Equal(t, board.BadgeRoleBump, board.SignalLabel(board.SignalRoleBump))
		assert.Equal(t, board.BadgeFitsSlip, board.SignalLabel(board.SignalFitsSlip))
		assert.Equal(t, "bogus", board.SignalLabel("bogus"))
	})
}

func TestExploreFacetCounts(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()
	signals := exploreSignals()

	t.Run("unfiltered slate counts every facet", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{Signals: signals})
		assert.Equal(t, map[board.Category]int{
			board.CategoryLines:      3,
			board.CategoryTeamTotals: 1,
			board.CategoryPassing:    1,
			board.CategoryRushing:    1,
			board.CategoryReceiving:  4,
			board.CategoryTDs:        0,
			board.CategoryKicking:    0,
		}, got.CategoryCounts)
		assert.Equal(t, map[string]int{"A": 3, "B": 4, "C": 0, "D": 2, "F": 0}, got.GradeCounts)
	})

	t.Run("a facet keeps counting rows its own filter would hide", func(t *testing.T) {
		t.Parallel()
		unfiltered := board.Explore(rows, games, stats, board.ExploreQuery{Signals: signals})
		byCategory := board.Explore(rows, games, stats, board.ExploreQuery{
			Category: board.CategoryReceiving,
			Signals:  signals,
		})
		assert.Equal(t, unfiltered.CategoryCounts, byCategory.CategoryCounts,
			"category chips still show every bet type while one is selected")
		assert.Equal(t, map[string]int{"A": 1, "B": 1, "C": 0, "D": 1, "F": 0}, byCategory.GradeCounts,
			"grade chips narrow to the receiving rows")
		assert.Equal(t, 1, byCategory.SignalCounts[board.SignalSmash])
	})

	t.Run("other facets narrow the grade counts", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{Grades: []string{"A"}, Signals: signals})
		assert.Equal(t, map[string]int{"A": 3, "B": 4, "C": 0, "D": 2, "F": 0}, got.GradeCounts,
			"grade chips still show every grade while one is selected")
		assert.Equal(t, map[board.Category]int{
			board.CategoryLines:      0,
			board.CategoryTeamTotals: 0,
			board.CategoryPassing:    1,
			board.CategoryRushing:    1,
			board.CategoryReceiving:  1,
			board.CategoryTDs:        0,
			board.CategoryKicking:    0,
		}, got.CategoryCounts)
		assert.Equal(t, 0, got.SignalCounts[board.SignalSmash], "the only signal row is a B grade")
	})

	t.Run("window narrows both remaining facets", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{
			Window:  board.WindowSundayLate,
			Signals: signals,
		})
		assert.Equal(t, map[string]int{"A": 1, "B": 1, "C": 0, "D": 1, "F": 0}, got.GradeCounts)
		assert.Equal(t, map[board.Category]int{
			board.CategoryLines:      1,
			board.CategoryTeamTotals: 1,
			board.CategoryPassing:    0,
			board.CategoryRushing:    0,
			board.CategoryReceiving:  2,
			board.CategoryTDs:        0,
			board.CategoryKicking:    0,
		}, got.CategoryCounts)
	})

	t.Run("ungraded rows never land in a grade bucket", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{PlayerID: "x1"})
		require.Len(t, got.Rows, 1)
		assert.Empty(t, got.Rows[0].Grade.Letter)
		assert.Equal(t, map[string]int{"A": 0, "B": 0, "C": 0, "D": 0, "F": 0}, got.GradeCounts)
		assert.Equal(t, 1, got.CategoryCounts[board.CategoryReceiving])
	})
}

func TestExploreSorts(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()

	// Every expectation lists the fixture labels in the order the sort must
	// return them, including the fixed tiebreakers.
	tests := []struct {
		sort board.ExploreSort
		want []string
	}{
		{
			sort: board.ExploreSortGrade,
			want: []string{
				"Allen PASS_YDS", "Cook RUSH_YDS", "Worthy REC_YDS", "Shakir REC_YDS",
				"KC ML", "BUF SPREAD", "TOTAL", "KC TEAM_TOTAL", "Hollins REC_YDS", "Ghost REC_YDS",
			},
		},
		{
			sort: board.ExploreSortMatchup,
			want: []string{
				"Allen PASS_YDS", "Shakir REC_YDS", "Hollins REC_YDS", "Cook RUSH_YDS",
				"Worthy REC_YDS", "KC ML", "BUF SPREAD", "TOTAL", "KC TEAM_TOTAL", "Ghost REC_YDS",
			},
		},
		{
			sort: board.ExploreSortOdds,
			want: []string{
				"KC ML", "Shakir REC_YDS", "Hollins REC_YDS", "BUF SPREAD", "TOTAL",
				"KC TEAM_TOTAL", "Ghost REC_YDS", "Cook RUSH_YDS", "Allen PASS_YDS", "Worthy REC_YDS",
			},
		},
		{
			sort: board.ExploreSortLine,
			want: []string{
				"Allen PASS_YDS", "Hollins REC_YDS", "TOTAL", "Cook RUSH_YDS", "Shakir REC_YDS",
				"Worthy REC_YDS", "KC TEAM_TOTAL", "Ghost REC_YDS", "BUF SPREAD", "KC ML",
			},
		},
		{
			sort: board.ExploreSortKickoff,
			want: []string{
				"Allen PASS_YDS", "Hollins REC_YDS", "Shakir REC_YDS", "Cook RUSH_YDS", "BUF SPREAD", "TOTAL",
				"KC ML", "Worthy REC_YDS", "KC TEAM_TOTAL", "Ghost REC_YDS",
			},
		},
		{
			sort: board.ExploreSortPlayer,
			want: []string{
				"Hollins REC_YDS", "BUF SPREAD", "Ghost REC_YDS", "Cook RUSH_YDS", "Allen PASS_YDS",
				"KC ML", "KC TEAM_TOTAL", "Shakir REC_YDS", "Worthy REC_YDS", "TOTAL",
			},
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.sort), func(t *testing.T) {
			t.Parallel()
			got := board.Explore(rows, games, stats, board.ExploreQuery{Sort: tc.sort})
			assert.Equal(t, tc.want, exploreTexts(got.Rows))
		})
	}

	t.Run("default sort is best grade", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{})
		graded := board.Explore(rows, games, stats, board.ExploreQuery{Sort: board.ExploreSortGrade})
		assert.Equal(t, exploreTexts(graded.Rows), exploreTexts(got.Rows))
	})

	t.Run("matchup prefers signals even over a higher grade", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{
			Sort:    board.ExploreSortMatchup,
			Signals: exploreSignals(),
		})
		assert.Equal(t, "Shakir REC_YDS", got.Rows[0].Row.Text)
	})

	t.Run("every sort is deterministic", func(t *testing.T) {
		t.Parallel()
		shuffled := make([]board.Row, len(rows))
		for i := range rows {
			shuffled[i] = rows[len(rows)-1-i]
		}
		for _, sortMode := range board.ExploreSorts {
			first := board.Explore(rows, games, stats, board.ExploreQuery{Sort: sortMode})
			second := board.Explore(shuffled, games, stats, board.ExploreQuery{Sort: sortMode})
			assert.Equal(t, exploreTexts(first.Rows), exploreTexts(second.Rows), "sort %s", sortMode)
		}
	})
}

func TestExplorePagesResults(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()

	t.Run("limit slices the page and flags more", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{Limit: 3})
		require.Len(t, got.Rows, 3)
		assert.Equal(t, 10, got.TotalMatched)
		assert.True(t, got.HasMore)
	})

	t.Run("exact fit has no more", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{Limit: 10})
		assert.Len(t, got.Rows, 10)
		assert.False(t, got.HasMore)
	})

	t.Run("large limit returns everything", func(t *testing.T) {
		t.Parallel()
		got := board.Explore(rows, games, stats, board.ExploreQuery{Limit: 500})
		assert.Len(t, got.Rows, 10)
		assert.False(t, got.HasMore)
	})

	t.Run("zero and negative limits use the phone default", func(t *testing.T) {
		t.Parallel()
		many := make([]board.Row, 0, 60)
		for i := range 60 {
			many = append(many, board.Row{
				GameID:   "g1",
				Market:   "REC_YDS",
				Team:     "BUF",
				PlayerID: fmt.Sprintf("p%d", i),
				Side:     "OVER",
				Line:     new(10.5),
				Odds:     -110,
				Category: board.CategoryReceiving,
				Text:     fmt.Sprintf("player %d", i),
			})
		}
		for _, limit := range []int{0, -5} {
			got := board.Explore(many, games, stats, board.ExploreQuery{Limit: limit})
			assert.Len(t, got.Rows, board.DefaultExploreLimit)
			assert.Equal(t, 60, got.TotalMatched)
			assert.True(t, got.HasMore)
		}
	})
}

func TestExploreScoresRows(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()
	got := board.Explore(rows, games, stats, board.ExploreQuery{})
	require.Len(t, got.Rows, 10)

	byText := make(map[string]board.ScoredRow, len(got.Rows))
	for _, row := range got.Rows {
		byText[row.Row.Text] = row
	}

	allen := byText["Allen PASS_YDS"]
	assert.Equal(t, "A", allen.Grade.Letter)
	assert.Positive(t, allen.Grade.Score)
	assert.Equal(t, "Sharp price edge", allen.Reason)
	assert.Equal(t, 51, allen.ImpliedPct, "-105 implies 51%")

	shakir := byText["Shakir REC_YDS"]
	assert.Equal(t, "B", shakir.Grade.Letter)
	assert.Equal(t, 53, shakir.ImpliedPct, "-115 implies 53%")

	hollins := byText["Hollins REC_YDS"]
	assert.Equal(t, "D", hollins.Grade.Letter)
	assert.Equal(t, "Tail risk", hollins.Reason, "the largest-magnitude factor wins")

	ghost := byText["Ghost REC_YDS"]
	assert.Empty(t, ghost.Grade.Letter, "a row whose game is missing cannot be graded")
	assert.Equal(t, "Balanced profile", ghost.Reason)

	for _, row := range got.Rows {
		assert.NotEmpty(t, row.Reason, row.Row.Text)
	}
}

func TestExploreImpliedPct(t *testing.T) {
	t.Parallel()

	_, games, stats := exploreFixture()
	rows := []board.Row{
		{GameID: "g1", Market: "ML", Team: "BUF", Side: "TEAM", Odds: -110, Text: "minus110"},
		{GameID: "g1", Market: "ML", Team: "BUF", Side: "TEAM", Odds: 150, Text: "plus150"},
		{GameID: "g1", Market: "ML", Team: "BUF", Side: "TEAM", Odds: -400, Text: "minus400"},
		{GameID: "g1", Market: "ML", Team: "BUF", Side: "TEAM", Odds: 0, Text: "unpriced"},
	}
	got := board.Explore(rows, games, stats, board.ExploreQuery{})
	want := map[string]int{
		"minus110": 52,
		"plus150":  40,
		"minus400": 80,
		"unpriced": 0,
	}
	for _, row := range got.Rows {
		assert.Equal(t, want[row.Row.Text], row.ImpliedPct, row.Row.Text)
	}
}

func TestExploreLeavesOutOtherMarket(t *testing.T) {
	t.Parallel()

	rows, games, stats := exploreFixture()
	got := board.Explore(rows, games, stats, board.ExploreQuery{Q: "Josh Allen"})
	assert.Equal(t, []string{"Allen PASS_YDS"}, exploreTexts(got.Rows),
		"the OTHER row shares the player name but must never surface")
}

func TestExploreEmptyInput(t *testing.T) {
	t.Parallel()

	got := board.Explore(nil, nil, nil, board.ExploreQuery{})
	assert.Empty(t, got.Rows)
	assert.Equal(t, 0, got.TotalMatched)
	assert.False(t, got.HasMore)
	assert.Equal(t, 0, got.CategoryCounts[board.CategoryLines])
	assert.Equal(t, 0, got.GradeCounts["A"])
	assert.Equal(t, 0, got.SignalCounts[board.SignalSmash])
}

func TestExploreSortLabels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Best grade", board.ExploreSortGrade.Label())
	assert.Equal(t, "Matchup", board.ExploreSortMatchup.Label())
	assert.Equal(t, "Odds", board.ExploreSortOdds.Label())
	assert.Equal(t, "Line", board.ExploreSortLine.Label())
	assert.Equal(t, "Kickoff", board.ExploreSortKickoff.Label())
	assert.Equal(t, "Player", board.ExploreSortPlayer.Label())
	assert.Equal(t, "bogus", board.ExploreSort("bogus").Label())
	assert.Len(t, board.ExploreSorts, 6)
	assert.Len(t, board.ExploreSignals, 4)
}
