package odds_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

// propsQuery is the /props query for one test event.
func propsQuery(eventID string) url.Values {
	query := apiKeyQuery()
	query.Set("eventId", eventID)
	query.Set("markets",
		"team_totals,player_pass_yds,player_pass_tds,player_rush_yds,"+
			"player_rush_attempts,player_reception_yds,player_receptions,"+
			"player_anytime_td,player_kicking_points")
	query.Set("oddsFormat", "american")
	query.Set("includeLinks", "true")
	query.Set("includeSids", "true")
	query.Set("limit", "10000")
	return query
}

func TestPropsMapsAliasesAndExpandsEachSide(t *testing.T) {
	t.Parallel()

	const eventID = "evt-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/props", r.URL.Path)
		assert.Equal(t, propsQuery(eventID), r.URL.Query())
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		w.Header().Set("X-Requests-Remaining", "990")
		w.Header().Set("X-Requests-Used", "10")
		w.Header().Set("X-Requests-Last", "3")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"event_id":"evt-1","bookmaker":"fanduel","player":"Josh Allen",
				"market_key":"player_passing_yards","period":"FULL","line":275.5,
				"over_price":-115,"under_price":-105,
				"over_link":"https://fanduel.test/over","under_link":"https://fanduel.test/under",
				"over_sid":"over-1","under_sid":"under-1"
			},
			{
				"event_id":"evt-1","bookmaker":"fanduel","player":"Josh Allen",
				"market_key":"player_anytime_touchdown_scorer","period":"FULL","line":0.5,
				"over_price":150,"over_link":"https://fanduel.test/td","over_sid":"td-1"
			},
			{
				"event_id":"evt-1","bookmaker":"fanduel","player":"Buffalo Bills",
				"market_key":"team_totals","period":"FULL","line":24.5,
				"over_price":-110,"under_price":-110
			},
			{
				"event_id":"evt-1","bookmaker":"prizepicks","player_name":"Stefon Diggs",
				"market_key":"player_receiving_yds","period":"FULL","line":68.5,
				"over_price":-120
			}
		]`))
	}))
	t.Cleanup(server.Close)

	props, credits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), eventID)
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 990, Used: 10, Last: 3}, credits)

	want := []odds.Prop{
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketPassYds,
			PlayerName: "Josh Allen",
			Side:       odds.SideOver,
			Line:       new(275.5),
			Price:      -115,
			Link:       "https://fanduel.test/over",
			Sid:        "over-1",
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketPassYds,
			PlayerName: "Josh Allen",
			Side:       odds.SideUnder,
			Line:       new(275.5),
			Price:      -105,
			Link:       "https://fanduel.test/under",
			Sid:        "under-1",
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketAnytimeTD,
			PlayerName: "Josh Allen",
			Side:       odds.SideYES,
			Line:       nil,
			Price:      150,
			Link:       "https://fanduel.test/td",
			Sid:        "td-1",
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketTeamTotal,
			Team:       "Buffalo Bills",
			Side:       odds.SideOver,
			Line:       new(24.5),
			Price:      -110,
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketTeamTotal,
			Team:       "Buffalo Bills",
			Side:       odds.SideUnder,
			Line:       new(24.5),
			Price:      -110,
		},
		{
			EventID:    eventID,
			Book:       "prizepicks",
			MarketType: odds.MarketRecYds,
			PlayerName: "Stefon Diggs",
			Side:       odds.SideOver,
			Line:       new(68.5),
			Price:      -120,
		},
	}
	assert.Equal(t, want, props)
}

func TestPropsSkipsOtherEventsOtherPeriodsAndUnpricedRows(t *testing.T) {
	t.Parallel()

	const eventID = "evt-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Requests-Remaining", "990")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"event_id":"evt-2","bookmaker":"fanduel","player":"Elsewhere",
			 "market_key":"player_pass_yds","period":"FULL","line":200,"over_price":-110},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"First Half",
			 "market_key":"player_pass_yds","period":"1H","line":150,"over_price":-110},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"Unknown Period",
			 "market_key":"player_pass_yds","period":"UNKNOWN","line":150,"over_price":-110},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"No Line",
			 "market_key":"player_pass_yds","period":"FULL","over_price":-110},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"No Price",
			 "market_key":"player_pass_yds","period":"FULL","line":200},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"No Over TD",
			 "market_key":"player_anytime_td","period":"FULL","under_price":-200},
			{"event_id":"evt-1","bookmaker":"","player":"No Book",
			 "market_key":"player_pass_yds","period":"FULL","line":200,"over_price":-110},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"  ",
			 "market_key":"player_pass_yds","period":"FULL","line":200,"over_price":-110},
			{"event_id":"evt-1","bookmaker":"fanduel","player":"Unknown Market",
			 "market_key":"player_sacks","period":"FULL","line":0.5,"over_price":200}
		]`))
	}))
	t.Cleanup(server.Close)

	props, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), eventID)
	require.NoError(t, err)
	assert.Empty(t, props)
}

func TestPropsMapsTheSavedParlayAPIResponse(t *testing.T) {
	t.Parallel()

	const eventID = "dc56d980f527ffeef89940133b211b13"
	server := fixtureServer(t, "props_sample.json", "/props",
		odds.Credits{Remaining: 988, Used: 12, Last: 3}, propsQuery(eventID))

	props, credits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), eventID)
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 988, Used: 12, Last: 3}, credits)
	require.Len(t, props, 16, "one row per priced side on the full-game board")

	want := []odds.Prop{
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketPassYds,
			PlayerName: "Joe Burrow",
			Side:       odds.SideOver,
			Line:       new(268.5),
			Price:      -115,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=20101",
			Sid:        "20101",
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketPassYds,
			PlayerName: "Joe Burrow",
			Side:       odds.SideUnder,
			Line:       new(268.5),
			Price:      -105,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=20102",
			Sid:        "20102",
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Joe Burrow",
			Side:       odds.SideOver,
			Line:       new(268.5),
			Price:      -110,
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketPassTds,
			PlayerName: "Joe Burrow",
			Side:       odds.SideOver,
			Line:       new(1.5),
			Price:      120,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189646&selectionId=20201",
			Sid:        "20201",
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketRecYds,
			PlayerName: "Ja'Marr Chase",
			Side:       odds.SideUnder,
			Line:       new(78.5),
			Price:      100,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189647&selectionId=20302",
			Sid:        "20302",
		},
		{
			EventID:    eventID,
			Book:       "betmgm",
			MarketType: odds.MarketRushAtt,
			PlayerName: "Joe Mixon",
			Side:       odds.SideOver,
			Line:       new(16.5),
			Price:      -110,
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketAnytimeTD,
			PlayerName: "Ja'Marr Chase",
			Side:       odds.SideYES,
			Line:       nil,
			Price:      145,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189648&selectionId=20401",
			Sid:        "20401",
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketKickerPts,
			PlayerName: "Evan McPherson",
			Side:       odds.SideUnder,
			Line:       new(7.5),
			Price:      -115,
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketTeamTotal,
			Team:       "Cincinnati Bengals",
			Side:       odds.SideOver,
			Line:       new(23.5),
			Price:      -110,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189649&selectionId=20501",
			Sid:        "20501",
		},
		{
			EventID:    eventID,
			Book:       "prizepicks",
			MarketType: odds.MarketRecYds,
			PlayerName: "Tee Higgins",
			Side:       odds.SideOver,
			Line:       new(55.5),
			Price:      -125,
		},
	}
	for _, expected := range want {
		assert.Contains(t, props, expected)
	}

	for _, prop := range props {
		assert.Equal(t, eventID, prop.EventID, "only the requested event's rows survive")
		if prop.MarketType == odds.MarketTeamTotal {
			assert.Empty(t, prop.PlayerName, "team totals carry Team, not PlayerName")
			assert.NotEmpty(t, prop.Team)
			continue
		}
		assert.Empty(t, prop.Team)
		assert.NotEmpty(t, prop.PlayerName)
	}
}

func TestPropsRequiresAnEventID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request sent without an event ID")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event ID")
}

func TestPropsReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), "evt-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(http.StatusServiceUnavailable))
}
