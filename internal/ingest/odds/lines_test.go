package odds_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestLinesMapsDeepLinksFromTheSavedEvent(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "odds_game_links.json"))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/odds", r.URL.Path)
		assert.Equal(t, linesQuery(), r.URL.Query())
		w.Header().Set("X-Requests-Remaining", "491")
		w.Header().Set("X-Requests-Used", "6")
		w.Header().Set("X-Requests-Last", "3")
		w.Header().Set("Content-Type", "application/json")
		// The live all-events call returns an array; the saved fixture is one event.
		_, _ = w.Write([]byte("[" + string(raw) + "]"))
	}))
	t.Cleanup(server.Close)

	lines, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Lines(t.Context())
	require.NoError(t, err)

	const eventID = "dc56d980f527ffeef89940133b211b13"
	want := map[string]struct{ link, sid, eventSid string }{
		"draftkings": {
			link:     "https://sportsbook.draftkings.com/?outcomes=0ML84695561_1",
			sid:      "0ML84695561_1",
			eventSid: "34118128",
		},
		"fanduel": {
			link:     "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=50199",
			sid:      "50199",
			eventSid: "35603243",
		},
	}
	for book, expected := range want {
		found := false
		for _, line := range lines {
			if line.EventID != eventID || line.Book != book ||
				line.MarketType != odds.MarketML || line.Team != "Atlanta Falcons" {
				continue
			}
			found = true
			assert.Equal(t, expected.link, line.Link)
			assert.Equal(t, expected.sid, line.Sid)
			assert.Equal(t, expected.eventSid, line.EventSid)
		}
		assert.True(t, found, "no %s moneyline row for Atlanta", book)
	}
}

func TestLinesSkipsUnpricedOutcomesAndNonTeamMoneylines(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/odds", r.URL.Path)
		assert.Equal(t, linesQuery(), r.URL.Query())
		w.Header().Set("X-Requests-Remaining", "497")
		w.Header().Set("X-Requests-Used", "3")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id":"evt-1",
			"commence_time":"2026-11-08T14:30:00Z",
			"home_team":"Bills",
			"away_team":"Jets",
			"bookmakers":[{
				"key":"fanduel",
				"sid":"34118128",
				"markets":[
					{"key":"h2h","outcomes":[
						{"name":"Bills","price":120,"link":"https://fanduel.test/ml","sid":"1"},
						{"name":"Draw","price":900},
						{"name":"Jets"}
					]},
					{"key":"spreads","outcomes":[
						{"name":"Bills","price":-110},
						{"name":"Jets","price":-110,"point":2.5}
					]},
					{"key":"totals","outcomes":[
						{"name":"Over","price":-108},
						{"name":"Under","price":-112,"point":44.5}
					]}
				]
			}]
		}]`))
	}))
	t.Cleanup(server.Close)

	lines, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Lines(t.Context())
	require.NoError(t, err)

	want := []odds.Line{
		{
			EventID:    "evt-1",
			Book:       "fanduel",
			MarketType: odds.MarketML,
			Team:       "Bills",
			Side:       odds.SideTeam,
			Price:      120,
			Link:       "https://fanduel.test/ml",
			Sid:        "1",
			EventSid:   "34118128",
		},
		{
			EventID:    "evt-1",
			Book:       "fanduel",
			MarketType: odds.MarketSpread,
			Team:       "Jets",
			Side:       odds.SideTeam,
			Line:       new(2.5),
			Price:      -110,
			EventSid:   "34118128",
		},
		{
			EventID:    "evt-1",
			Book:       "fanduel",
			MarketType: odds.MarketTotal,
			Side:       odds.SideUnder,
			Line:       new(44.5),
			Price:      -112,
			EventSid:   "34118128",
		},
	}
	assert.Equal(t, want, lines)
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

func TestLinesForMarketsSendsScopedMarketsAndBookmakers(t *testing.T) {
	t.Parallel()

	wantQuery := apiKeyQuery()
	wantQuery.Set("bookmakers", "draftkings,fanduel")
	wantQuery.Set("markets", "spreads,totals")
	wantQuery.Set("oddsFormat", "american")
	wantQuery.Set("includeLinks", "true")
	wantQuery.Set("includeSids", "true")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/odds", r.URL.Path)
		assert.Equal(t, wantQuery, r.URL.Query())
		assert.False(t, r.URL.Query().Has("regions"))
		w.Header().Set("X-Requests-Remaining", "498")
		w.Header().Set("X-Requests-Used", "2")
		w.Header().Set("X-Requests-Last", "2")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id":"evt-1",
			"commence_time":"2026-11-08T14:30:00Z",
			"home_team":"Bills",
			"away_team":"Jets",
			"bookmakers":[{
				"key":"draftkings",
				"sid":"34118128",
				"markets":[
					{"key":"spreads","outcomes":[
						{"name":"Bills","price":-110,"point":-2.5,"link":"https://dk.test/spread","sid":"s1"}
					]},
					{"key":"totals","outcomes":[
						{"name":"Over","price":-108,"point":44.5,"link":"https://dk.test/total","sid":"t1"}
					]}
				]
			}]
		}]`))
	}))
	t.Cleanup(server.Close)

	lines, credits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).
		LinesForMarkets(t.Context(), []string{"spreads", "totals", "spreads"}, []string{"draftkings", "fanduel"})
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 498, Used: 2, Last: 2}, credits)
	assert.Equal(t, []odds.Line{
		{
			EventID:    "evt-1",
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Team:       "Bills",
			Side:       odds.SideTeam,
			Line:       new(-2.5),
			Price:      -110,
			Link:       "https://dk.test/spread",
			Sid:        "s1",
			EventSid:   "34118128",
		},
		{
			EventID:    "evt-1",
			Book:       "draftkings",
			MarketType: odds.MarketTotal,
			Side:       odds.SideOver,
			Line:       new(44.5),
			Price:      -108,
			Link:       "https://dk.test/total",
			Sid:        "t1",
			EventSid:   "34118128",
		},
	}, lines)
}

func TestLinesForMarketsFallsBackToRegionsWhenBookmakersEmpty(t *testing.T) {
	t.Parallel()

	wantQuery := apiKeyQuery()
	wantQuery.Set("regions", "us")
	wantQuery.Set("markets", "h2h")
	wantQuery.Set("oddsFormat", "american")
	wantQuery.Set("includeLinks", "true")
	wantQuery.Set("includeSids", "true")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/odds", r.URL.Path)
		assert.Equal(t, wantQuery, r.URL.Query())
		assert.False(t, r.URL.Query().Has("bookmakers"))
		w.Header().Set("X-Requests-Remaining", "499")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(server.Close)

	lines, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).
		LinesForMarkets(t.Context(), []string{"h2h"}, nil)
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestLinesForMarketsRequiresNonEmptyMarkets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request sent without markets")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := odds.NewClient(server.URL, testAPIKey, server.Client())
	for _, markets := range [][]string{nil, {}, {"  ", ""}} {
		_, _, err := client.LinesForMarkets(t.Context(), markets, []string{"draftkings"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "markets")
	}
}
