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
