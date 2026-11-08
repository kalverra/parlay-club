package odds

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readEventFixture decodes one saved per-event odds response.
func readEventFixture(t *testing.T, name string) eventResponse {
	t.Helper()

	//nolint:gosec // the fixture name is a test constant.
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)

	var event eventResponse
	require.NoError(t, json.Unmarshal(raw, &event))
	return event
}

// TestParseLinesMapsOutcomeLinksFromTheSavedFixture checks that the saved
// DraftKings and FanDuel game-line outcomes carry their link and sid onto the
// matching Line, in fixture order.
func TestParseLinesMapsOutcomeLinksFromTheSavedFixture(t *testing.T) {
	t.Parallel()

	event := readEventFixture(t, "odds_game_links.json")
	lines, err := parseLines([]eventResponse{event})
	require.NoError(t, err)

	for _, book := range []string{"draftkings", "fanduel"} {
		t.Run(book, func(t *testing.T) {
			t.Parallel()

			var want []outcomeResponse
			for _, bookmaker := range event.Bookmakers {
				if bookmaker.Key != book {
					continue
				}
				for _, market := range bookmaker.Markets {
					want = append(want, market.Outcomes...)
				}
			}
			require.NotEmpty(t, want, "the fixture has no %s outcomes", book)

			var got []Line
			for _, line := range lines {
				if line.Book == book {
					got = append(got, line)
				}
			}
			require.Len(t, got, len(want))
			for i, outcome := range want {
				assert.NotEmpty(t, outcome.Link)
				assert.NotEmpty(t, outcome.Sid)
				assert.Equal(t, outcome.Link, got[i].Link, outcome.Name)
				assert.Equal(t, outcome.Sid, got[i].Sid, outcome.Name)
			}
		})
	}
}

// TestPropFixtureCarriesOutcomeLinks checks that the prop response, which the
// game-line parser ignores for now, still decodes both fields onto each
// DraftKings and FanDuel outcome. The props client will reuse this struct.
func TestPropFixtureCarriesOutcomeLinks(t *testing.T) {
	t.Parallel()

	event := readEventFixture(t, "odds_prop_links.json")

	for _, book := range []string{"draftkings", "fanduel"} {
		t.Run(book, func(t *testing.T) {
			t.Parallel()

			found := false
			for _, bookmaker := range event.Bookmakers {
				if bookmaker.Key != book {
					continue
				}
				for _, market := range bookmaker.Markets {
					if market.Key != "player_pass_yds" {
						continue
					}
					require.NotEmpty(t, market.Outcomes)
					for _, outcome := range market.Outcomes {
						found = true
						assert.NotEmpty(t, outcome.Link, outcome.Name)
						assert.NotEmpty(t, outcome.Sid, outcome.Name)
					}
				}
			}
			assert.True(t, found, "the fixture has no %s player_pass_yds outcomes", book)
		})
	}
}
