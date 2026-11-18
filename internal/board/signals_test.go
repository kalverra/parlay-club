package board_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/board"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

func TestApplySignalsBadges(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("g1", kickoff)
	roster := []db.RosterPlayer{
		{AthleteID: "wr1", FullName: "Khalil Shakir", Team: "BUF", Position: "WR"},
		{AthleteID: "wr2", FullName: "Keon Coleman", Team: "BUF", Position: "WR"},
		{AthleteID: "wr3", FullName: "Mack Hollins", Team: "BUF", Position: "WR"},
		{AthleteID: "rb1", FullName: "James Cook", Team: "BUF", Position: "RB"},
		{AthleteID: "te1", FullName: "Dalton Kincaid", Team: "BUF", Position: "TE"},
		{AthleteID: "qb1", FullName: "Josh Allen", Team: "BUF", Position: "QB"},
		{AthleteID: "opp_wr", FullName: "Tyreek Hill", Team: "MIA", Position: "WR"},
	}

	buildRow := func(market, team, player, side string, line float64) board.Row {
		rows := board.Build(
			[]db.Game{g},
			[]db.Odds{odds("g1", "dk", market, team, player, side, new(line), -110)},
			roster,
			"",
			nil,
			kickoff.Add(-time.Hour),
		)
		require.Len(t, rows, 1)
		return rows[0]
	}

	tests := []struct {
		name    string
		row     board.Row
		signals board.BoardSignals
		want    string
	}{
		{
			name:    "empty signals fail soft",
			row:     buildRow("REC_YDS", "BUF", "Khalil Shakir", "OVER", 55.5),
			signals: board.BoardSignals{},
			want:    "",
		},
		{
			name: "smash matchup WR over vs bottom-6 pass defense",
			row:  buildRow("REC_YDS", "BUF", "Khalil Shakir", "OVER", 55.5),
			signals: board.BoardSignals{
				TeamEPA: map[string]*analyzer.TeamDefenseProfile{
					"MIA": {DefPassEPAAllowedRank: 27},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr1": {
						SeasonTargetShare: 0.21,
						Weeks:             []analyzer.PlayerWeekUsage{{WeekNum: 1, TargetShare: 0.21}},
					},
				},
			},
			want: board.BadgeSmashMatchup,
		},
		{
			name: "smash matchup RB over vs bottom-6 rush defense",
			row:  buildRow("RUSH_YDS", "BUF", "James Cook", "OVER", 64.5),
			signals: board.BoardSignals{
				TeamEPA: map[string]*analyzer.TeamDefenseProfile{
					"MIA": {DefRushEPAAllowedRank: 30},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"rb1": {
						Weeks: []analyzer.PlayerWeekUsage{
							{WeekNum: 2, Carries: 14},
							{WeekNum: 1, Carries: 12},
						},
					},
				},
			},
			want: board.BadgeSmashMatchup,
		},
		{
			name: "rank 26 is not smash matchup",
			row:  buildRow("REC_YDS", "BUF", "Khalil Shakir", "OVER", 55.5),
			signals: board.BoardSignals{
				TeamEPA: map[string]*analyzer.TeamDefenseProfile{
					"MIA": {DefPassEPAAllowedRank: 26},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr1": {
						SeasonTargetShare: 0.21,
						Weeks:             []analyzer.PlayerWeekUsage{{WeekNum: 1, TargetShare: 0.21}},
					},
				},
			},
			want: "",
		},
		{
			name: "smash matchup ignored on under",
			row:  buildRow("REC_YDS", "BUF", "Khalil Shakir", "UNDER", 55.5),
			signals: board.BoardSignals{
				TeamEPA: map[string]*analyzer.TeamDefenseProfile{
					"MIA": {DefPassEPAAllowedRank: 32},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr1": {
						SeasonTargetShare: 0.28,
						Weeks:             []analyzer.PlayerWeekUsage{{WeekNum: 1, TargetShare: 0.28}},
					},
				},
			},
			want: "",
		},
		{
			name: "high volume WR by WOPR",
			row:  buildRow("REC_YDS", "BUF", "Khalil Shakir", "OVER", 55.5),
			signals: board.BoardSignals{
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr1": {
						Weeks: []analyzer.PlayerWeekUsage{
							{WeekNum: 3, WOPR: 0.60},
							{WeekNum: 2, WOPR: 0.56},
							{WeekNum: 1, WOPR: 0.55},
						},
					},
				},
			},
			want: board.BadgeHighVolume,
		},
		{
			name: "high volume TE by target share",
			row:  buildRow("REC_YDS", "BUF", "Dalton Kincaid", "OVER", 42.5),
			signals: board.BoardSignals{
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"te1": {
						Last3TargetShare: 0.19,
						Weeks:            []analyzer.PlayerWeekUsage{{WeekNum: 1, TargetShare: 0.19}},
					},
				},
			},
			want: board.BadgeHighVolume,
		},
		{
			name: "high volume RB by carries",
			row:  buildRow("RUSH_YDS", "BUF", "James Cook", "OVER", 64.5),
			signals: board.BoardSignals{
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"rb1": {
						Weeks: []analyzer.PlayerWeekUsage{
							{WeekNum: 2, Carries: 16},
							{WeekNum: 1, Carries: 15},
						},
					},
				},
			},
			want: board.BadgeHighVolume,
		},
		{
			name: "role bump when same-position starter out and prior snap spike >= 15pct",
			row:  buildRow("REC_YDS", "BUF", "Keon Coleman", "OVER", 38.5),
			signals: board.BoardSignals{
				Injuries: map[string][]espn.Injury{
					"BUF": {{AthleteID: "wr1", FullName: "Khalil Shakir", Position: "WR", Status: "Out"}},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr2": {
						SeasonSnapAvg: 0.58,
						Weeks: []analyzer.PlayerWeekUsage{
							{WeekNum: 3, OffensePct: 0.50},
							{WeekNum: 2, OffensePct: 0.78},
							{WeekNum: 1, OffensePct: 0.46},
						},
					},
				},
			},
			want: board.BadgeRoleBump,
		},
		{
			name: "questionable teammate does not trigger role bump",
			row:  buildRow("REC_YDS", "BUF", "Keon Coleman", "OVER", 38.5),
			signals: board.BoardSignals{
				Injuries: map[string][]espn.Injury{
					"BUF": {{AthleteID: "wr1", FullName: "Khalil Shakir", Position: "WR", Status: "Questionable"}},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr2": {
						SeasonSnapAvg: 0.58,
						Weeks: []analyzer.PlayerWeekUsage{
							{WeekNum: 2, OffensePct: 0.78},
							{WeekNum: 1, OffensePct: 0.38},
						},
					},
				},
			},
			want: "",
		},
		{
			name: "different position out does not trigger role bump",
			row:  buildRow("REC_YDS", "BUF", "Keon Coleman", "OVER", 38.5),
			signals: board.BoardSignals{
				Injuries: map[string][]espn.Injury{
					"BUF": {{AthleteID: "rb1", FullName: "James Cook", Position: "RB", Status: "Out"}},
				},
				PlayerUsage: map[string]*analyzer.PlayerUsage{
					"wr2": {
						SeasonSnapAvg: 0.58,
						Weeks: []analyzer.PlayerWeekUsage{
							{WeekNum: 2, OffensePct: 0.78},
							{WeekNum: 1, OffensePct: 0.38},
						},
					},
				},
			},
			want: "",
		},
		{
			name: "fits slip: same-team RB over pairs with -7.0 favorite spread",
			row:  buildRow("RUSH_YDS", "BUF", "James Cook", "OVER", 64.5),
			signals: board.BoardSignals{
				SlipLegs: []analyzer.Leg{
					{GameID: "g1", Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-7.0)},
				},
			},
			want: board.BadgeFitsSlip,
		},
		{
			name: "fits slip: opposing WR over pairs with -7.0 favorite spread",
			row:  buildRow("REC_YDS", "MIA", "Tyreek Hill", "OVER", 78.5),
			signals: board.BoardSignals{
				SlipLegs: []analyzer.Leg{
					{GameID: "g1", Market: "SPREAD", Team: "BUF", Side: "TEAM", Line: new(-7.0)},
				},
			},
			want: board.BadgeFitsSlip,
		},
		{
			name: "fits slip: same-team WR over pairs with QB PASS_YDS over",
			row:  buildRow("REC_YDS", "BUF", "Khalil Shakir", "OVER", 55.5),
			signals: board.BoardSignals{
				SlipLegs: []analyzer.Leg{
					{
						GameID:     "g1",
						Market:     "PASS_YDS",
						Team:       "BUF",
						PlayerID:   "qb1",
						PlayerName: "Josh Allen",
						Side:       "OVER",
						Line:       new(255.5),
					},
				},
			},
			want: board.BadgeFitsSlip,
		},
		{
			name: "fits slip: different game does not match",
			row:  buildRow("REC_YDS", "BUF", "Khalil Shakir", "OVER", 55.5),
			signals: board.BoardSignals{
				SlipLegs: []analyzer.Leg{
					{
						GameID:     "g2",
						Market:     "PASS_YDS",
						Team:       "BUF",
						PlayerID:   "qb1",
						PlayerName: "Josh Allen",
						Side:       "OVER",
						Line:       new(255.5),
					},
				},
			},
			want: "",
		},
		{
			name: "fits slip: skips already-picked selection even after line moves by 0.5",
			row:  buildRow("PASS_YDS", "BUF", "Josh Allen", "OVER", 256.0),
			signals: board.BoardSignals{
				SlipLegs: []analyzer.Leg{
					{
						GameID:     "g1",
						Market:     "PASS_YDS",
						Team:       "BUF",
						PlayerID:   "qb1",
						PlayerName: "Josh Allen",
						Side:       "OVER",
						Line:       new(255.5),
					},
				},
			},
			want: "",
		},
		{
			name: "fits slip: excludes same player across name suffix variants",
			row:  buildRow("REC_YDS", "BUF", "Ollie Gordon II", "OVER", 19.5),
			signals: board.BoardSignals{
				SlipLegs: []analyzer.Leg{
					{
						GameID:     "g1",
						Market:     "PASS_YDS",
						Team:       "BUF",
						PlayerName: "Ollie Gordon",
						Side:       "OVER",
						Line:       new(205.5),
					},
				},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := board.ApplySignals(g, []board.Row{tc.row}, tc.signals)
			require.Len(t, got, 1)
			assert.Equal(t, tc.want, got[0].Badge)
		})
	}
}

func TestPlayerTagsCappedAndPrioritized(t *testing.T) {
	t.Parallel()

	kickoff := at(t, 8, 13, 0)
	g := game("g1", kickoff)
	roster := []db.RosterPlayer{
		{AthleteID: "wr2", FullName: "Keon Coleman", Team: "BUF", Position: "WR"},
	}
	built := board.Build(
		[]db.Game{g},
		[]db.Odds{
			odds("g1", "dk", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(45.5), -110),
			odds("g1", "dk", "RECEPTIONS", "BUF", "Keon Coleman", "OVER", new(3.5), -120),
		},
		roster,
		"",
		nil,
		kickoff.Add(-time.Hour),
	)

	// Trigger all 4 badges simultaneously; Row.Badge takes Smash matchup,
	// PlayerGroup.Tags caps at 2 in priority order.
	signals := board.BoardSignals{
		TeamEPA: map[string]*analyzer.TeamDefenseProfile{
			"MIA": {DefPassEPAAllowedRank: 29},
		},
		PlayerUsage: map[string]*analyzer.PlayerUsage{
			"wr2": {
				SeasonSnapAvg:     0.60,
				SeasonTargetShare: 0.25,
				Weeks: []analyzer.PlayerWeekUsage{
					{WeekNum: 2, OffensePct: 0.85, TargetShare: 0.26, WOPR: 0.62},
					{WeekNum: 1, OffensePct: 0.55, TargetShare: 0.24, WOPR: 0.58},
				},
			},
		},
		Injuries: map[string][]espn.Injury{
			"BUF": {{AthleteID: "wr1", FullName: "Khalil Shakir", Position: "WR", Status: "Out"}},
		},
		SlipLegs: []analyzer.Leg{
			{GameID: "g1", Market: "PASS_YDS", Team: "BUF", PlayerName: "Josh Allen", Side: "OVER", Line: new(245.5)},
		},
	}

	detail := board.Detail(g, built, board.Query{
		Category: board.CategoryReceiving,
		Sort:     board.SortMatchup,
		Now:      kickoff.Add(-time.Hour),
		Signals:  signals,
	})
	require.Len(t, detail.Players, 1)
	player := detail.Players[0]
	assert.Equal(t, []string{board.BadgeSmashMatchup, board.BadgeFitsSlip}, player.Tags)
	for _, line := range player.Lines {
		assert.Equal(t, board.BadgeSmashMatchup, line.Options[0].Badge)
	}
}

func TestDetailSortMatchup(t *testing.T) {
	t.Parallel()

	assert.Contains(t, board.DetailSorts, board.SortMatchup)
	assert.Equal(t, "Matchup", board.SortMatchup.Label())

	kickoff := at(t, 8, 13, 0)
	g := game("g1", kickoff)
	roster := []db.RosterPlayer{
		{AthleteID: "wr_plain", FullName: "Aaron Alpha", Team: "BUF", Position: "WR"},
		{AthleteID: "wr_vol", FullName: "Mack Hollins", Team: "BUF", Position: "WR"},
		{AthleteID: "wr_fit", FullName: "Keon Coleman", Team: "BUF", Position: "WR"},
		{AthleteID: "wr_smash", FullName: "Zay Flowers", Team: "BUF", Position: "WR"},
	}
	built := board.Build(
		[]db.Game{g},
		[]db.Odds{
			odds("g1", "dk", "REC_YDS", "BUF", "Aaron Alpha", "OVER", new(30.5), -110),
			odds("g1", "dk", "REC_YDS", "BUF", "Mack Hollins", "OVER", new(35.5), -110),
			odds("g1", "dk", "REC_YDS", "BUF", "Keon Coleman", "OVER", new(45.5), -110),
			odds("g1", "dk", "REC_YDS", "BUF", "Zay Flowers", "OVER", new(65.5), -110),
			odds("g1", "dk", "TOTAL", "", "", "OVER", new(47.5), -110),
			odds("g1", "dk", "TOTAL", "", "", "UNDER", new(47.5), -110),
			odds("g1", "dk", "SPREAD", "BUF", "", "TEAM", new(-3.5), -110),
		},
		roster,
		"",
		nil,
		kickoff.Add(-time.Hour),
	)

	signals := board.BoardSignals{
		TeamEPA: map[string]*analyzer.TeamDefenseProfile{
			"MIA": {DefPassEPAAllowedRank: 28},
		},
		PlayerUsage: map[string]*analyzer.PlayerUsage{
			"wr_smash": {
				SeasonTargetShare: 0.25,
				Weeks:             []analyzer.PlayerWeekUsage{{WeekNum: 1, TargetShare: 0.25}},
			},
			"wr_vol": {
				SeasonSnapAvg: 0.88,
				Weeks:         []analyzer.PlayerWeekUsage{{WeekNum: 1, OffensePct: 0.88, WOPR: 0.60}},
			},
		},
		SlipLegs: []analyzer.Leg{
			{GameID: "g1", Market: "PASS_YDS", Team: "BUF", PlayerName: "Josh Allen", Side: "OVER", Line: new(245.5)},
		},
	}

	// Note: wr_fit AND wr_smash AND wr_vol AND wr_plain are all BUF WRs so all
	// match QB PASS_YDS OVER ("Fits slip"), except:
	// - wr_smash has Smash matchup (+40) + Fits slip (+30) + High volume (+20) + rank 28 = 118
	// - wr_vol has Fits slip (+30) + High volume (+20) + rank 28 = 78
	// - wr_fit has Fits slip (+30) + rank 28 = 58
	// - wr_plain has Fits slip (+30) + rank 28 = 58 (alphabetical tiebreak: Aaron Alpha < Keon Coleman)
	detail := board.Detail(g, built, board.Query{
		Category: board.CategoryReceiving,
		Sort:     board.SortMatchup,
		Now:      kickoff.Add(-time.Hour),
		Signals:  signals,
	})
	require.Len(t, detail.Players, 4)
	names := make([]string, len(detail.Players))
	for i, p := range detail.Players {
		names[i] = p.Name
	}
	assert.Equal(t, []string{"Zay Flowers", "Mack Hollins", "Aaron Alpha", "Keon Coleman"}, names)

	// On the Lines tab, TOTAL OVER gets Fits slip from QB PASS_YDS OVER so it
	// sorts ahead of SPREAD under SortMatchup.
	linesDetail := board.Detail(g, built, board.Query{
		Category: board.CategoryLines,
		Sort:     board.SortMatchup,
		Now:      kickoff.Add(-time.Hour),
		Signals:  signals,
	})
	require.Len(t, linesDetail.Lines, 2)
	assert.Equal(t, "TOTAL", linesDetail.Lines[0].Market)
	assert.Equal(t, board.BadgeFitsSlip, linesDetail.Lines[0].Options[0].Badge)
}
