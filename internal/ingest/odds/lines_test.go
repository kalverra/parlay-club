package odds_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

func TestLinesMapsEverySelectionForEveryBook(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t, "odds_nfl.json", "/odds",
		odds.Credits{Remaining: 497, Used: 3, Last: 3}, linesQuery())

	lines, credits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Lines(t.Context())
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 497, Used: 3, Last: 3}, credits)
	assert.Len(t, lines, 1162, "one row per priced selection across the saved slate")
}

func TestLinesMapsMoneylineSpreadsAndTotals(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t, "odds_nfl.json", "/odds",
		odds.Credits{Remaining: 497, Used: 3, Last: 3}, linesQuery())

	lines, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Lines(t.Context())
	require.NoError(t, err)

	const eventID = "dc56d980f527ffeef89940133b211b13"
	want := []odds.Line{
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Atlanta Falcons",
			Side:       odds.SideTeam,
			Price:      145,
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Cincinnati Bengals",
			Side:       odds.SideTeam,
			Price:      -175,
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Team:       "Atlanta Falcons",
			Side:       odds.SideTeam,
			Line:       new(3.0),
			Price:      100,
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Team:       "Cincinnati Bengals",
			Side:       odds.SideTeam,
			Line:       new(-3.0),
			Price:      -120,
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketTotal,
			Side:       odds.SideOver,
			Line:       new(52.5),
			Price:      -108,
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketTotal,
			Side:       odds.SideUnder,
			Line:       new(52.5),
			Price:      -112,
		},
	}
	assert.Equal(t, want, onlyBook(lines, eventID, "draftkings"))
}

func TestLinesReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Lines(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503 Service Unavailable")
}

func TestLinesRequiresCreditHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Lines(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Requests-Remaining")
}
