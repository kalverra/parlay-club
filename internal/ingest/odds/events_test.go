package odds_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

func TestEventsParsesTheEventList(t *testing.T) {
	t.Parallel()

	server := fixtureServer(t, "events_nfl.json", "/events",
		odds.Credits{Remaining: 500, Used: 0, Last: 0}, apiKeyQuery())

	events, credits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Events(t.Context())
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 500, Used: 0, Last: 0}, credits)

	require.Len(t, events, 27)
	assert.Equal(t, odds.Event{
		ID:           "dc56d980f527ffeef89940133b211b13",
		CommenceTime: time.Date(2026, time.November, 8, 14, 30, 0, 0, time.UTC),
		HomeTeam:     "Atlanta Falcons",
		AwayTeam:     "Cincinnati Bengals",
	}, events[0])
	assert.Equal(t, odds.Event{
		ID:           "35e3bdff558b5d1506251e814986f164",
		CommenceTime: time.Date(2026, time.November, 17, 1, 15, 0, 0, time.UTC),
		HomeTeam:     "Baltimore Ravens",
		AwayTeam:     "Los Angeles Chargers",
	}, events[len(events)-1])
}

func TestEventsSkipsFixturesWithoutACommenceTime(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/events", r.URL.Path)
		assert.Equal(t, apiKeyQuery(), r.URL.Query())
		w.Header().Set("X-Requests-Remaining", "500")
		w.Header().Set("X-Requests-Used", "0")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"ready","commence_time":"2026-11-08T14:30:00Z","home_team":"Atlanta Falcons","away_team":"Cincinnati Bengals"},
			{"id":"unreported","commence_time":null,"home_team":"Buffalo Bills","away_team":"New York Jets"},
			{"id":"blank","commence_time":"","home_team":"Dallas Cowboys","away_team":"Philadelphia Eagles"}
		]`))
	}))
	t.Cleanup(server.Close)

	events, credits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Events(t.Context())
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 500}, credits)
	require.Len(t, events, 1)
	assert.Equal(t, odds.Event{
		ID:           "ready",
		CommenceTime: time.Date(2026, time.November, 8, 14, 30, 0, 0, time.UTC),
		HomeTeam:     "Atlanta Falcons",
		AwayTeam:     "Cincinnati Bengals",
	}, events[0])
}

func TestEventsReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Events(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401 Unauthorized")
}

func TestClientRequiresAnAPIKey(t *testing.T) {
	t.Parallel()

	_, _, err := odds.NewClient("", "", nil).Events(t.Context())
	require.ErrorIs(t, err, odds.ErrNoAPIKey)
}
