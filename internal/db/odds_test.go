package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestSameSelection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  db.SelectionKey
		right db.SelectionKey
		want  bool
	}{
		{
			name: "player prop matches by player ID across name and empty team",
			left: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-1", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "",
				PlayerName: "Ollie Gordon II (MIA)", PlayerID: "espn-1", Side: "OVER",
			},
			want: true,
		},
		{
			name: "player prop matches by normalized name and team when ID missing",
			left: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon II (MIA)", PlayerID: "espn-1", Side: "OVER",
			},
			want: true,
		},
		{
			name: "player prop allows empty team when normalized name matches",
			left: db.SelectionKey{
				GameID: "401", MarketType: "PASS_YDS", Team: "",
				PlayerName: "Bryce Young", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "PASS_YDS", Team: "CAR",
				PlayerName: "Bryce Young", Side: "OVER",
			},
			want: true,
		},
		{
			name: "player prop rejects differing player IDs",
			left: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-1", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-2", Side: "OVER",
			},
			want: false,
		},
		{
			name: "player prop rejects differing non-empty teams without IDs",
			left: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Josh Allen", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "BUF",
				PlayerName: "Josh Allen", Side: "OVER",
			},
			want: false,
		},
		{
			name: "player prop rejects differing sides",
			left: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-1", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-1", Side: "UNDER",
			},
			want: false,
		},
		{
			name: "player prop rejects differing markets",
			left: db.SelectionKey{
				GameID: "401", MarketType: "RUSH_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-1", Side: "OVER",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "REC_YDS", Team: "MIA",
				PlayerName: "Ollie Gordon", PlayerID: "espn-1", Side: "OVER",
			},
			want: false,
		},
		{
			name: "empty game ID never matches",
			left: db.SelectionKey{
				GameID: "", MarketType: "SPREAD", Team: "BUF", Side: "TEAM",
			},
			right: db.SelectionKey{
				GameID: "", MarketType: "SPREAD", Team: "BUF", Side: "TEAM",
			},
			want: false,
		},
		{
			name: "game market matches on game, market, team, side",
			left: db.SelectionKey{
				GameID: "401", MarketType: "SPREAD", Team: "BUF", Side: "TEAM",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "SPREAD", Team: "BUF", Side: "TEAM",
			},
			want: true,
		},
		{
			name: "game market rejects differing teams",
			left: db.SelectionKey{
				GameID: "401", MarketType: "SPREAD", Team: "BUF", Side: "TEAM",
			},
			right: db.SelectionKey{
				GameID: "401", MarketType: "SPREAD", Team: "MIA", Side: "TEAM",
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, db.SameSelection(tc.left, tc.right))
		})
	}
}

func TestOddsMatchesSelectionCanonicalAndMovedLine(t *testing.T) {
	t.Parallel()

	leg := db.Leg{
		GameID:     new("401"),
		MarketType: "RUSH_YDS",
		Team:       new("MIA"),
		PlayerName: new("Ollie Gordon"),
		Side:       new("OVER"),
		Line:       new(17.5),
	}
	row := db.Odds{
		GameID:     "401",
		Book:       "draftkings",
		MarketType: "RUSH_YDS",
		Team:       "MIA",
		PlayerName: "Ollie Gordon II",
		Side:       "OVER",
		Line:       new(19.5),
		Price:      -115,
	}
	assert.True(t, row.MatchesSelection(leg))

	otherSide := row
	otherSide.Side = "UNDER"
	assert.False(t, otherSide.MatchesSelection(leg))
}

func TestSamePlayer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  db.SelectionKey
		right db.SelectionKey
		want  bool
	}{
		{
			name:  "matches by player ID across name variants",
			left:  db.SelectionKey{PlayerID: "espn-1", PlayerName: "Ollie Gordon", Team: "MIA"},
			right: db.SelectionKey{PlayerID: "espn-1", PlayerName: "Ollie Gordon II", Team: ""},
			want:  true,
		},
		{
			name:  "rejects differing player IDs",
			left:  db.SelectionKey{PlayerID: "espn-1", PlayerName: "Ollie Gordon", Team: "MIA"},
			right: db.SelectionKey{PlayerID: "espn-2", PlayerName: "Ollie Gordon", Team: "MIA"},
			want:  false,
		},
		{
			name:  "matches by normalized name and tag when one ID is missing",
			left:  db.SelectionKey{PlayerName: "Ollie Gordon", Team: "MIA"},
			right: db.SelectionKey{PlayerID: "espn-1", PlayerName: "Ollie Gordon II (MIA)", Team: "MIA"},
			want:  true,
		},
		{
			name:  "rejects conflicting non-empty teams without IDs",
			left:  db.SelectionKey{PlayerName: "Josh Allen", Team: "BUF"},
			right: db.SelectionKey{PlayerName: "Josh Allen", Team: "JAX"},
			want:  false,
		},
		{
			name:  "empty player names do not match",
			left:  db.SelectionKey{Team: "BUF"},
			right: db.SelectionKey{Team: "BUF"},
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, db.SamePlayer(tc.left, tc.right))
		})
	}
}

func TestSameLineGroup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  db.SelectionKey
		right db.SelectionKey
		want  bool
	}{
		{
			name:  "spread matches across teams",
			left:  db.SelectionKey{GameID: "g1", MarketType: "SPREAD", Team: "BUF", Side: "TEAM"},
			right: db.SelectionKey{GameID: "g1", MarketType: "SPREAD", Team: "MIA", Side: "TEAM"},
			want:  true,
		},
		{
			name:  "total matches across sides",
			left:  db.SelectionKey{GameID: "g1", MarketType: "TOTAL", Side: "OVER"},
			right: db.SelectionKey{GameID: "g1", MarketType: "TOTAL", Side: "UNDER"},
			want:  true,
		},
		{
			name:  "team total requires matching team",
			left:  db.SelectionKey{GameID: "g1", MarketType: "TEAM_TOTAL", Team: "BUF", Side: "OVER"},
			right: db.SelectionKey{GameID: "g1", MarketType: "TEAM_TOTAL", Team: "MIA", Side: "UNDER"},
			want:  false,
		},
		{
			name:  "team total matches same team across sides",
			left:  db.SelectionKey{GameID: "g1", MarketType: "TEAM_TOTAL", Team: "BUF", Side: "OVER"},
			right: db.SelectionKey{GameID: "g1", MarketType: "TEAM_TOTAL", Team: "BUF", Side: "UNDER"},
			want:  true,
		},
		{
			name: "player prop matches same player across sides and suffix",
			left: db.SelectionKey{
				GameID:     "g1",
				MarketType: "RUSH_YDS",
				Team:       "MIA",
				PlayerName: "Ollie Gordon",
				Side:       "OVER",
			},
			right: db.SelectionKey{
				GameID:     "g1",
				MarketType: "RUSH_YDS",
				Team:       "MIA",
				PlayerName: "Ollie Gordon II",
				Side:       "UNDER",
			},
			want: true,
		},
		{
			name: "player prop rejects different players",
			left: db.SelectionKey{
				GameID:     "g1",
				MarketType: "RUSH_YDS",
				Team:       "MIA",
				PlayerName: "Ollie Gordon",
				Side:       "OVER",
			},
			right: db.SelectionKey{
				GameID:     "g1",
				MarketType: "RUSH_YDS",
				Team:       "MIA",
				PlayerName: "De'Von Achane",
				Side:       "OVER",
			},
			want: false,
		},
		{
			name: "different markets do not match",
			left: db.SelectionKey{
				GameID:     "g1",
				MarketType: "RUSH_YDS",
				Team:       "MIA",
				PlayerName: "Ollie Gordon",
				Side:       "OVER",
			},
			right: db.SelectionKey{
				GameID:     "g1",
				MarketType: "REC_YDS",
				Team:       "MIA",
				PlayerName: "Ollie Gordon",
				Side:       "OVER",
			},
			want: false,
		},
		{
			name:  "empty game ID never matches",
			left:  db.SelectionKey{MarketType: "TOTAL", Side: "OVER"},
			right: db.SelectionKey{MarketType: "TOTAL", Side: "UNDER"},
			want:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, db.SameLineGroup(tc.left, tc.right))
		})
	}
}

func TestMatchBookOdds(t *testing.T) {
	t.Parallel()

	leg := db.Leg{
		GameID:     new("401"),
		MarketType: "RUSH_YDS",
		Team:       new("MIA"),
		PlayerName: new("Ollie Gordon"),
		Side:       new("OVER"),
		Line:       new(17.5),
	}

	moved := db.Odds{
		GameID: "401", Book: "draftkings", MarketType: "RUSH_YDS",
		Team: "MIA", PlayerName: "Ollie Gordon II", Side: "OVER",
		Line: new(18.0), Price: -120,
	}
	exact := db.Odds{
		GameID: "401", Book: "draftkings", MarketType: "RUSH_YDS",
		Team: "MIA", PlayerName: "Ollie Gordon II", Side: "OVER",
		Line: new(17.5), Price: -110,
	}
	otherBook := db.Odds{
		GameID: "401", Book: "fanduel", MarketType: "RUSH_YDS",
		Team: "MIA", PlayerName: "Ollie Gordon II", Side: "OVER",
		Line: new(17.5), Price: -105,
	}
	otherSide := db.Odds{
		GameID: "401", Book: "draftkings", MarketType: "RUSH_YDS",
		Team: "MIA", PlayerName: "Ollie Gordon II", Side: "UNDER",
		Line: new(17.5), Price: -115,
	}

	got, ok := db.MatchBookOdds(leg, []db.Odds{otherBook, otherSide, moved, exact}, "DraftKings")
	assert.True(t, ok)
	assert.Equal(t, exact, got)

	gotMoved, ok := db.MatchBookOdds(leg, []db.Odds{otherBook, otherSide, moved}, "draftkings")
	assert.True(t, ok)
	assert.Equal(t, moved, gotMoved)

	_, ok = db.MatchBookOdds(leg, []db.Odds{otherBook, otherSide}, "draftkings")
	assert.False(t, ok)

	_, ok = db.MatchBookOdds(leg, []db.Odds{exact}, "")
	assert.False(t, ok)
}

func TestMarketLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		market string
		want   string
	}{
		{"PASS_YDS", "Pass Yds"},
		{"PASS_TDS", "Pass TDs"},
		{"RUSH_YDS", "Rush Yds"},
		{"RUSH_ATT", "Rush Att"},
		{"REC_YDS", "Rec Yds"},
		{"RECEPTIONS", "Receptions"},
		{"ANYTIME_TD", "Anytime TD"},
		{"KICKER_PTS", "Kicking Pts"},
		{"SPREAD", ""},
		{"OTHER", ""},
		{"", ""},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.want, db.MarketLabel(tc.market), tc.market)
	}
}

func TestFormatSelection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		market string
		team   string
		player string
		side   string
		line   *float64
		want   string
	}{
		{
			name: "positive spread", market: "SPREAD", team: "MIA", line: new(3.5),
			want: "MIA +3.5",
		},
		{
			name: "negative spread", market: "SPREAD", team: "BUF", line: new(-3.5),
			want: "BUF -3.5",
		},
		{
			name: "spread without line omits line", market: "SPREAD", team: "BUF",
			want: "BUF",
		},
		{
			name: "moneyline", market: "ML", team: "BUF",
			want: "BUF ML",
		},
		{
			name: "total with line", market: "TOTAL", side: "OVER", line: new(48.5),
			want: "Over 48.5",
		},
		{
			name: "total without line", market: "TOTAL", side: "UNDER",
			want: "Under",
		},
		{
			name: "team total with line", market: "TEAM_TOTAL", team: "BUF", side: "OVER", line: new(24.5),
			want: "BUF Team Total Over 24.5",
		},
		{
			name: "team total without line", market: "TEAM_TOTAL", team: "BUF", side: "OVER",
			want: "BUF Team Total Over",
		},
		{
			name: "anytime td", market: "ANYTIME_TD", player: "Josh Allen",
			want: "Josh Allen Anytime TD",
		},
		{
			name: "player prop with line", market: "PASS_YDS", player: "Josh Allen", side: "OVER", line: new(245.5),
			want: "Josh Allen Over 245.5 Pass Yds",
		},
		{
			name: "player prop without line", market: "KICKER_PTS", player: "Tyler Bass", side: "UNDER",
			want: "Tyler Bass Under Kicking Pts",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, db.FormatSelection(tc.market, tc.team, tc.player, tc.side, tc.line))
		})
	}
}

func TestBuildLegSelection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		market string
		team   string
		player string
		side   string
		line   *float64
		want   string
	}{
		{
			name: "valid spread", market: "SPREAD", team: " BUF ", line: new(-3.5),
			want: "BUF -3.5",
		},
		{
			name: "spread missing line rejected", market: "SPREAD", team: "BUF",
			want: "",
		},
		{
			name: "spread missing team rejected", market: "SPREAD", line: new(-3.5),
			want: "",
		},
		{
			name: "valid moneyline", market: "ML", team: "BUF",
			want: "BUF ML",
		},
		{
			name: "moneyline missing team rejected", market: "ML",
			want: "",
		},
		{
			name: "valid total", market: "TOTAL", side: "over", line: new(48.5),
			want: "Over 48.5",
		},
		{
			name: "total invalid side rejected", market: "TOTAL", side: "TEAM", line: new(48.5),
			want: "",
		},
		{
			name: "valid team total", market: "TEAM_TOTAL", team: "BUF", side: "UNDER", line: new(24.5),
			want: "BUF Team Total Under 24.5",
		},
		{
			name: "team total missing line rejected", market: "TEAM_TOTAL", team: "BUF", side: "OVER",
			want: "",
		},
		{
			name: "valid anytime td", market: "ANYTIME_TD", player: "Josh Allen",
			want: "Josh Allen Anytime TD",
		},
		{
			name: "anytime td missing player rejected", market: "ANYTIME_TD",
			want: "",
		},
		{
			name:   "other market always rejected",
			market: "OTHER",
			team:   "BUF",
			player: "Josh Allen",
			side:   "OVER",
			line:   new(1.5),
			want:   "",
		},
		{
			name: "valid player prop", market: "PASS_YDS", player: "Josh Allen", side: "OVER", line: new(245.5),
			want: "Josh Allen Over 245.5 Pass Yds",
		},
		{
			name: "player prop missing line rejected", market: "PASS_YDS", player: "Josh Allen", side: "OVER",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, db.BuildLegSelection(tc.market, tc.team, tc.player, tc.side, tc.line))
		})
	}
}
