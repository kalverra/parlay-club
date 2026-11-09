package odds

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLinesIgnoresMarketsItDoesNotRead(t *testing.T) {
	t.Parallel()

	lines, err := parseLines([]eventResponse{{
		ID: "evt-1",
		Bookmakers: []bookmakerResponse{{
			Key: "draftkings",
			Markets: []marketResponse{{
				Key: "player_pass_yds",
				Outcomes: []outcomeResponse{{
					Name:  "Over",
					Price: new(105),
					Point: new(275.5),
				}},
			}},
		}},
	}})
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestParseLinesSkipsUnpricedAndOffTeamOutcomes(t *testing.T) {
	t.Parallel()

	lines, err := parseLines([]eventResponse{{
		ID:       "evt-1",
		HomeTeam: "Bills",
		AwayTeam: "Jets",
		Bookmakers: []bookmakerResponse{{
			Key: "fanduel",
			Sid: "34118128",
			Markets: []marketResponse{
				{
					Key: marketH2H,
					Outcomes: []outcomeResponse{
						{Name: "Bills", Price: new(120)},
						{Name: "Draw", Price: new(900)},
						{Name: "Jets"},
						{Name: "Jets", Price: new(-140)},
					},
				},
				{
					Key: marketSpreads,
					Outcomes: []outcomeResponse{
						{Name: "Bills", Price: new(-110)},
						{Name: "Jets", Price: new(-110), Point: new(2.5)},
					},
				},
				{
					Key: marketTotals,
					Outcomes: []outcomeResponse{
						{Name: "Over", Price: new(-108)},
						{Name: "Under", Point: new(44.5)},
					},
				},
			},
		}},
	}})
	require.NoError(t, err)

	want := []Line{
		{
			EventID:    "evt-1",
			Book:       "fanduel",
			MarketType: MarketML,
			Team:       "Bills",
			Side:       SideTeam,
			Price:      120,
			EventSid:   "34118128",
		},
		{
			EventID:    "evt-1",
			Book:       "fanduel",
			MarketType: MarketML,
			Team:       "Jets",
			Side:       SideTeam,
			Price:      -140,
			EventSid:   "34118128",
		},
		{
			EventID:    "evt-1",
			Book:       "fanduel",
			MarketType: MarketSpread,
			Team:       "Jets",
			Side:       SideTeam,
			Line:       new(2.5),
			Price:      -110,
			EventSid:   "34118128",
		},
	}
	assert.Equal(t, want, lines)
}

func TestParseLinesRejectsAnUnreadableMarket(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		market marketResponse
		want   string
	}{
		"outcome without a name": {
			market: marketResponse{
				Key:      marketH2H,
				Outcomes: []outcomeResponse{{Price: new(-110)}},
			},
			want: "has no name",
		},
		"total without a recognized side": {
			market: marketResponse{
				Key:      marketTotals,
				Outcomes: []outcomeResponse{{Name: "Push", Price: new(-110), Point: new(47.5)}},
			},
			want: "unknown total outcome",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := parseLines([]eventResponse{{
				ID: "evt-1",
				Bookmakers: []bookmakerResponse{{
					Key:     "draftkings",
					Markets: []marketResponse{test.market},
				}},
			}})
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}
