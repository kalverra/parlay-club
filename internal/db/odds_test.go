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
