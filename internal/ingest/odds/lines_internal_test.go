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

func TestParseLinesRejectsAMalformedMarket(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		market marketResponse
		want   string
	}{
		"spread without a point": {
			market: marketResponse{
				Key:      marketSpreads,
				Outcomes: []outcomeResponse{{Name: "Bills", Price: new(-110)}},
			},
			want: "point is missing",
		},
		"moneyline without a price": {
			market: marketResponse{
				Key:      marketH2H,
				Outcomes: []outcomeResponse{{Name: "Bills"}},
			},
			want: "price is missing",
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
