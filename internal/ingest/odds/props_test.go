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

// eventPropsQuery is the per-event prop query for one test event.
func eventPropsQuery() url.Values {
	query := apiKeyQuery()
	query.Set("regions", "us")
	query.Set("markets",
		"team_totals,player_pass_yds,player_pass_tds,player_rush_yds,"+
			"player_rush_attempts,player_reception_yds,player_receptions,"+
			"player_anytime_td,player_kicking_points")
	query.Set("oddsFormat", "american")
	query.Set("includeLinks", "true")
	query.Set("includeSids", "true")
	return query
}

func TestEventPropsMapsTheSavedOddsAPIResponse(t *testing.T) {
	t.Parallel()

	const eventID = "dc56d980f527ffeef89940133b211b13"
	credits := odds.Credits{Remaining: 489, Used: 11, Last: 1}
	server := fixtureServer(t, "odds_prop_links.json", "/events/"+eventID+"/odds",
		credits, eventPropsQuery())

	props, gotCredits, err := odds.NewClient(server.URL, testAPIKey, server.Client()).
		EventProps(t.Context(), eventID)
	require.NoError(t, err)
	assert.Equal(t, credits, gotCredits)
	require.Len(t, props, 48, "one row per priced outcome in the saved response")

	want := []odds.Prop{
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Joe Burrow",
			Side:       odds.SideOver,
			Line:       new(278.5),
			Price:      -112,
			Link:       "https://sportsbook.draftkings.com/?outcomes=0QA385017402%232346593166_13L88808Q1466938236Q20",
			Sid:        "0QA385017402#2346593166_13L88808Q1466938236Q20",
			EventSid:   "34118128",
		},
		{
			EventID:    eventID,
			Book:       "fanduel",
			MarketType: odds.MarketPassYds,
			PlayerName: "Joe Burrow",
			Side:       odds.SideUnder,
			Line:       new(276.5),
			Price:      -113,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.622015396&selectionId=20452293",
			Sid:        "20452293",
			EventSid:   "35603243",
		},
		{
			EventID:    eventID,
			Book:       "betonlineag",
			MarketType: odds.MarketPassYds,
			PlayerName: "Michael Penix Jr.",
			Side:       odds.SideOver,
			Line:       new(224.5),
			Price:      -111,
			EventSid:   "491166250",
		},
	}
	for _, expected := range want {
		assert.Contains(t, props, expected)
	}

	for _, prop := range props {
		assert.Equal(t, eventID, prop.EventID, "only the requested event's rows survive")
		assert.Equal(t, odds.MarketPassYds, prop.MarketType)
		assert.Empty(t, prop.Team, "player props carry PlayerName, not Team")
		assert.NotEmpty(t, prop.PlayerName)
		require.NotNil(t, prop.Line)
	}
}

func TestEventPropsNormalizesPropMarketsAndSkipsIncompleteRows(t *testing.T) {
	t.Parallel()

	const eventID = "evt-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/events/"+eventID+"/odds", r.URL.Path)
		assert.Equal(t, eventPropsQuery(), r.URL.Query())
		w.Header().Set("X-Requests-Remaining", "489")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"evt-1",
			"bookmakers":[
				{"key":"draftkings","sid":"34118128","markets":[
					{"key":"player_anytime_td","outcomes":[
						{"name":"Yes","description":" Josh Allen ","price":150,
						 "link":"https://draftkings.test/td","sid":"td-1"},
						{"name":"No","description":"Josh Allen","price":-200},
						{"name":"Yes","description":"No Price"},
						{"name":"Yes","description":"   ","price":200},
						{"name":"Yes","description":"No Point Needed","price":-110,"point":0.5}
					]},
					{"key":"team_totals","outcomes":[
						{"name":"Over","description":"Buffalo Bills","price":-110,"point":24.5},
						{"name":"Under","description":"Buffalo Bills","price":-110},
						{"name":"Push","description":"Buffalo Bills","price":-110,"point":24.5}
					]},
					{"key":"player_pass_yds","outcomes":[
						{"name":"Over","description":"Josh Allen","price":-115,"point":275.5,
						 "link":"https://draftkings.test/over","sid":"over-1"},
						{"name":"Under","description":"Josh Allen","point":275.5},
						{"name":"Over","description":"Josh Allen","price":-115},
						{"name":"Under","description":"","price":-105,"point":275.5}
					]},
					{"key":"player_sacks","outcomes":[
						{"name":"Over","description":"Someone","price":200,"point":0.5}
					]}
				]},
				{"key":"","sid":"no-book","markets":[
					{"key":"player_pass_yds","outcomes":[
						{"name":"Over","description":"No Book","price":-110,"point":200}
					]}
				]}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	props, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).
		EventProps(t.Context(), eventID)
	require.NoError(t, err)

	want := []odds.Prop{
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketAnytimeTD,
			PlayerName: "Josh Allen",
			Side:       odds.SideYES,
			Price:      150,
			Link:       "https://draftkings.test/td",
			Sid:        "td-1",
			EventSid:   "34118128",
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketAnytimeTD,
			PlayerName: "No Point Needed",
			Side:       odds.SideYES,
			Price:      -110,
			EventSid:   "34118128",
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketTeamTotal,
			Team:       "Buffalo Bills",
			Side:       odds.SideOver,
			Line:       new(24.5),
			Price:      -110,
			EventSid:   "34118128",
		},
		{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Josh Allen",
			Side:       odds.SideOver,
			Line:       new(275.5),
			Price:      -115,
			Link:       "https://draftkings.test/over",
			Sid:        "over-1",
			EventSid:   "34118128",
		},
	}
	assert.Equal(t, want, props,
		"unpriced rows, missing points, empty subjects, unknown markets, and books without a key are skipped")
}

func TestEventPropsRequiresAnEventID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request sent without an event ID")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).
		EventProps(t.Context(), "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event ID")
}

func TestEventPropsReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	_, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).
		EventProps(t.Context(), "evt-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), strconv.Itoa(http.StatusServiceUnavailable))
}

func TestMergePropsEnrichesTheSavedParlayAPIProps(t *testing.T) {
	t.Parallel()

	const eventID = "dc56d980f527ffeef89940133b211b13"
	propsServer := fixtureServer(t, "props_sample.json", "/props",
		odds.Credits{Remaining: 988, Used: 12, Last: 3}, propsQuery(eventID))
	eventServer := fixtureServer(t, "odds_prop_links.json", "/events/"+eventID+"/odds",
		odds.Credits{Remaining: 489, Used: 11, Last: 1}, eventPropsQuery())

	primary, _, err := odds.NewClient(propsServer.URL, testAPIKey, propsServer.Client()).
		Props(t.Context(), eventID)
	require.NoError(t, err)
	secondary, _, err := odds.NewClient(eventServer.URL, testAPIKey, eventServer.Client()).
		EventProps(t.Context(), eventID)
	require.NoError(t, err)

	merged := odds.MergeProps(primary, secondary)
	require.Greater(t, len(merged), len(primary), "selections only The Odds API priced are appended")

	draftKingsOver := findProp(t, merged, "draftkings", odds.MarketPassYds, "Joe Burrow", odds.SideOver)
	assert.Equal(t, new(278.5), draftKingsOver.Line,
		"a newly linked DraftKings row adopts the line its deep link opens")
	assert.Equal(t, -112, draftKingsOver.Price)
	assert.Equal(t, "https://sportsbook.draftkings.com/?outcomes=0QA385017402%232346593166_13L88808Q1466938236Q20",
		draftKingsOver.Link)
	assert.Equal(t, "0QA385017402#2346593166_13L88808Q1466938236Q20", draftKingsOver.Sid)
	assert.Equal(t, "34118128", draftKingsOver.EventSid)

	draftKingsUnder := findProp(t, merged, "draftkings", odds.MarketPassYds, "Joe Burrow", odds.SideUnder)
	assert.Equal(t, new(278.5), draftKingsUnder.Line)
	assert.Equal(t, "0QA385017402#2346593167_13L88808Q1920164065Q20", draftKingsUnder.Sid)

	fanDuelOver := findProp(t, merged, "fanduel", odds.MarketPassYds, "Joe Burrow", odds.SideOver)
	assert.Equal(t, new(268.5), fanDuelOver.Line, "a linked primary row keeps its own number and price")
	assert.Equal(t, -115, fanDuelOver.Price)
	assert.Equal(t, "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=20101",
		fanDuelOver.Link)
	assert.Equal(t, "20101", fanDuelOver.Sid)
	assert.Equal(t, "35603243", fanDuelOver.EventSid, "FanDuel's event ID is backfilled")

	draftKingsKicker := findProp(t, merged, "draftkings", odds.MarketKickerPts, "Evan McPherson", odds.SideUnder)
	assert.Empty(t, draftKingsKicker.Link)
	assert.Equal(t, "34118128", draftKingsKicker.EventSid,
		"a book's event ID reaches rows the other provider did not price")

	penix := findProp(t, merged, "draftkings", odds.MarketPassYds, "Michael Penix Jr.", odds.SideOver)
	assert.Equal(t, new(224.5), penix.Line)
	assert.Equal(t, -113, penix.Price)
	assert.NotEmpty(t, penix.Link)
	assert.NotEmpty(t, penix.Sid)
	assert.Equal(t, "34118128", penix.EventSid)

	assertProp := findProp(t, merged, "fanduel", odds.MarketAnytimeTD, "Ja'Marr Chase", odds.SideYES)
	assert.Nil(t, assertProp.Line)
	assert.Equal(t, "https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189648&selectionId=20401",
		assertProp.Link)
	assert.Equal(t, "35603243", assertProp.EventSid)

	teeHiggins := findProp(t, merged, "prizepicks", odds.MarketRecYds, "Tee Higgins", odds.SideOver)
	assert.Equal(t, new(55.5), teeHiggins.Line, "primary rows with no secondary selection survive")
}

func TestMergePropsReturnsACopyOfAOneSidedBoard(t *testing.T) {
	t.Parallel()

	primary := []odds.Prop{{
		Book:       "fanduel",
		MarketType: odds.MarketPassYds,
		PlayerName: "Josh Allen",
		Side:       odds.SideOver,
	}}
	secondary := []odds.Prop{{
		Book:       "draftkings",
		MarketType: odds.MarketPassYds,
		PlayerName: "Josh Allen",
		Side:       odds.SideOver,
	}}

	onlyPrimary := odds.MergeProps(primary, nil)
	assert.Equal(t, primary, onlyPrimary)
	onlyPrimary[0].Book = "changed"
	assert.Equal(t, "fanduel", primary[0].Book, "the result does not alias the input")

	onlySecondary := odds.MergeProps(nil, secondary)
	assert.Equal(t, secondary, onlySecondary)
	onlySecondary[0].Book = "changed"
	assert.Equal(t, "draftkings", secondary[0].Book)
}

func TestMergePropsLinkRules(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		primary   odds.Prop
		secondary odds.Prop
		want      odds.Prop
		wantLen   int
	}{
		"same line keeps the primary price": {
			primary: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
			},
			secondary: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -125,
				Link: "https://draftkings.test/over", Sid: "over-1", EventSid: "34118128",
			},
			want: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
				Link: "https://draftkings.test/over", Sid: "over-1", EventSid: "34118128",
			},
		},
		"different line adopts the secondary line and price": {
			primary: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(268.5), Price: -110,
			},
			secondary: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(278.5), Price: -112,
				Link: "https://draftkings.test/over", Sid: "over-1", EventSid: "34118128",
			},
			want: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(278.5), Price: -112,
				Link: "https://draftkings.test/over", Sid: "over-1", EventSid: "34118128",
			},
		},
		"linked primary keeps its link and backfills a matching sid": {
			primary: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
				Link: "https://fanduel.test/primary",
			},
			secondary: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -125,
				Link: "https://fanduel.test/secondary", Sid: "sec-1", EventSid: "35603243",
			},
			want: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
				Link: "https://fanduel.test/primary", Sid: "sec-1", EventSid: "35603243",
			},
		},
		"linked primary does not borrow a sid from another line": {
			primary: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
				Link: "https://fanduel.test/primary",
			},
			secondary: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(276.5), Price: -125,
				Link: "https://fanduel.test/secondary", Sid: "sec-1", EventSid: "35603243",
			},
			want: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
				Link: "https://fanduel.test/primary", EventSid: "35603243",
			},
		},
		"secondary-only selection is appended": {
			primary: odds.Prop{
				Book: "fanduel", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(275.5), Price: -110,
			},
			secondary: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(276.5), Price: -112,
				Link: "https://draftkings.test/over", Sid: "over-1", EventSid: "34118128",
			},
			want: odds.Prop{
				Book: "draftkings", MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen", Side: odds.SideOver,
				Line: new(276.5), Price: -112,
				Link: "https://draftkings.test/over", Sid: "over-1", EventSid: "34118128",
			},
			wantLen: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			merged := odds.MergeProps([]odds.Prop{test.primary}, []odds.Prop{test.secondary})
			wantLen := test.wantLen
			if wantLen == 0 {
				wantLen = 1
			}
			require.Len(t, merged, wantLen)
			if wantLen == 1 {
				assert.Equal(t, test.want, merged[0])
				return
			}
			assert.Equal(t, test.primary, merged[0])
			assert.Equal(t, test.want, merged[1])
		})
	}
}

func TestPropsFiltersNonStandardDFSAndSelectsPrimaryLines(t *testing.T) {
	t.Parallel()

	const eventID = "401873059"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Requests-Remaining", "980")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"event_id":"401873059","bookmaker":"prizepicks","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","odds_type":"goblin","line":49.5,"over_price":100,"under_price":-100},
			{"event_id":"401873059","bookmaker":"prizepicks","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","odds_type":"goblin","line":59.5,"over_price":100,"under_price":-100},
			{"event_id":"401873059","bookmaker":"prizepicks","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","odds_type":"Standard","line":73.5,"over_price":100,"under_price":-100},
			{"event_id":"401873059","bookmaker":"prizepicks","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","odds_type":"demon","line":99.5,"over_price":100,"under_price":-100},
			{"event_id":"401873059","bookmaker":"prizepicks","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","odds_type":"demon","line":124.5,"over_price":100,"under_price":-100},
			{"event_id":"401873059","bookmaker":"prizepicks","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","odds_type":"demon","line":152.5,"over_price":100,"under_price":-100},

			{"event_id":"401873059","bookmaker":"underdog","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","projection_type":"goblin","line":54.5,"over_price":-110},
			{"event_id":"401873059","bookmaker":"underdog","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","projection_type":"standard","line":73.5,"over_price":-110,"under_price":-110},
			{"event_id":"401873059","bookmaker":"underdog","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","projection_type":"demon","line":109.5,"over_price":-110},

			{"event_id":"401873059","bookmaker":"novig","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":59.5,"over_price":-260,"under_price":195},
			{"event_id":"401873059","bookmaker":"novig","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":64.5,"over_price":-190,"under_price":145},
			{"event_id":"401873059","bookmaker":"novig","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":69.5,"over_price":-140,"under_price":110},
			{"event_id":"401873059","bookmaker":"novig","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":73.5,"over_price":-108,"under_price":-112},
			{"event_id":"401873059","bookmaker":"novig","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":79.5,"over_price":140,"under_price":-175},
			{"event_id":"401873059","bookmaker":"novig","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":89.5,"over_price":240,"under_price":-320},

			{"event_id":"401873059","bookmaker":"bovada","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":55.5,"over_price":-300,"under_price":220},
			{"event_id":"401873059","bookmaker":"bovada","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":65.5,"over_price":-165,"under_price":125},
			{"event_id":"401873059","bookmaker":"bovada","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":75.5,"over_price":-115,"under_price":-115},
			{"event_id":"401873059","bookmaker":"bovada","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":85.5,"over_price":135,"under_price":-175},
			{"event_id":"401873059","bookmaker":"bovada","player":"Ollie Gordon","market_key":"player_rush_yds","period":"FULL","line":95.5,"over_price":250,"under_price":-350}
		]`))
	}))
	t.Cleanup(server.Close)

	props, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), eventID)
	require.NoError(t, err)
	require.Len(t, props, 8, "one standard OVER + UNDER pair per book")

	ppOver := findProp(t, props, "prizepicks", odds.MarketRushYds, "Ollie Gordon", odds.SideOver)
	assert.Equal(t, new(73.5), ppOver.Line)
	ppUnder := findProp(t, props, "prizepicks", odds.MarketRushYds, "Ollie Gordon", odds.SideUnder)
	assert.Equal(t, new(73.5), ppUnder.Line)

	udOver := findProp(t, props, "underdog", odds.MarketRushYds, "Ollie Gordon", odds.SideOver)
	assert.Equal(t, new(73.5), udOver.Line)
	udUnder := findProp(t, props, "underdog", odds.MarketRushYds, "Ollie Gordon", odds.SideUnder)
	assert.Equal(t, new(73.5), udUnder.Line)

	novigOver := findProp(t, props, "novig", odds.MarketRushYds, "Ollie Gordon", odds.SideOver)
	assert.Equal(t, new(73.5), novigOver.Line)
	assert.Equal(t, -108, novigOver.Price)
	novigUnder := findProp(t, props, "novig", odds.MarketRushYds, "Ollie Gordon", odds.SideUnder)
	assert.Equal(t, new(73.5), novigUnder.Line)
	assert.Equal(t, -112, novigUnder.Price)

	bovadaOver := findProp(t, props, "bovada", odds.MarketRushYds, "Ollie Gordon", odds.SideOver)
	assert.Equal(t, new(75.5), bovadaOver.Line)
	assert.Equal(t, -115, bovadaOver.Price)
	bovadaUnder := findProp(t, props, "bovada", odds.MarketRushYds, "Ollie Gordon", odds.SideUnder)
	assert.Equal(t, new(75.5), bovadaUnder.Line)
	assert.Equal(t, -115, bovadaUnder.Price)
}

func TestPropsPrimaryLineSelectionEdgeCases(t *testing.T) {
	t.Parallel()

	const eventID = "evt-edge"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Requests-Remaining", "980")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"event_id":"evt-edge","bookmaker":"book-two-vs-one","player":"Player A","market_key":"player_rush_yds","period":"FULL","line":60.5,"over_price":-110},
			{"event_id":"evt-edge","bookmaker":"book-two-vs-one","player":"Player A","market_key":"player_rush_yds","period":"FULL","line":72.5,"over_price":-120,"under_price":-105},

			{"event_id":"evt-edge","bookmaker":"book-tiebreak","player":"Player B","market_key":"player_rush_yds","period":"FULL","line":70.5,"over_price":100,"under_price":100},
			{"event_id":"evt-edge","bookmaker":"book-tiebreak","player":"Player B","market_key":"player_rush_yds","period":"FULL","line":73.5,"over_price":-110,"under_price":-110},

			{"event_id":"evt-edge","bookmaker":"book-onesided","player":"Player C","market_key":"player_rush_yds","period":"FULL","line":50.5,"over_price":200},
			{"event_id":"evt-edge","bookmaker":"book-onesided","player":"Player C","market_key":"player_rush_yds","period":"FULL","line":68.5,"over_price":-115},

			{"event_id":"evt-edge","bookmaker":"book-split","player":"Player D","market_key":"player_rush_yds","period":"FULL","line":65.5,"over_price":-110},
			{"event_id":"evt-edge","bookmaker":"book-split","player":"Player D","market_key":"player_rush_yds","period":"FULL","line":74.5,"over_price":-115},
			{"event_id":"evt-edge","bookmaker":"book-split","player":"Player D","market_key":"player_rush_yds","period":"FULL","line":74.5,"under_price":-105},

			{"event_id":"evt-edge","bookmaker":"book-td","player":"Player E","market_key":"player_anytime_td","period":"FULL","odds_type":"demon","line":0.5,"over_price":500},
			{"event_id":"evt-edge","bookmaker":"book-td","player":"Player E","market_key":"player_anytime_td","period":"FULL","odds_type":"standard","line":0.5,"over_price":140}
		]`))
	}))
	t.Cleanup(server.Close)

	props, _, err := odds.NewClient(server.URL, testAPIKey, server.Client()).Props(t.Context(), eventID)
	require.NoError(t, err)

	pA := findProp(t, props, "book-two-vs-one", odds.MarketRushYds, "Player A", odds.SideOver)
	assert.Equal(t, new(72.5), pA.Line, "two-sided quote beats one-sided quote")

	pB := findProp(t, props, "book-tiebreak", odds.MarketRushYds, "Player B", odds.SideOver)
	assert.Equal(t, new(73.5), pB.Line, "tie in implied-prob diff breaks toward -110")

	pC := findProp(t, props, "book-onesided", odds.MarketRushYds, "Player C", odds.SideOver)
	assert.Equal(t, new(68.5), pC.Line, "one-sided quotes pick price closest to -110")

	pDOver := findProp(t, props, "book-split", odds.MarketRushYds, "Player D", odds.SideOver)
	assert.Equal(t, new(74.5), pDOver.Line, "split over/under at same line combine into two-sided quote")
	pDUnder := findProp(t, props, "book-split", odds.MarketRushYds, "Player D", odds.SideUnder)
	assert.Equal(t, new(74.5), pDUnder.Line)

	pE := findProp(t, props, "book-td", odds.MarketAnytimeTD, "Player E", odds.SideYES)
	assert.Equal(t, 140, pE.Price, "non-standard anytime TD is dropped")
}

func TestMergePropsDeduplicatesPrimaryWhenSecondaryEmpty(t *testing.T) {
	t.Parallel()

	primary := []odds.Prop{
		{
			Book:       "fanduel",
			MarketType: odds.MarketRushYds,
			Team:       "MIA",
			PlayerName: "Ollie Gordon",
			Side:       odds.SideOver,
			Line:       new(73.5),
			Price:      -135,
		},
		{
			Book:       "fanduel",
			MarketType: odds.MarketRushYds,
			Team:       "MIA",
			PlayerName: "Ollie Gordon",
			Side:       odds.SideOver,
			Line:       new(74.5),
			Price:      -112,
		},
	}

	merged := odds.MergeProps(primary, nil)
	require.Len(t, merged, 1)
	assert.Equal(t, new(74.5), merged[0].Line, "keeps primary collision closest to -110")
	assert.Equal(t, -112, merged[0].Price)
}

func TestMergePropsMatchesNormalizedPlayerNames(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		primaryName   string
		secondaryName string
		market        string
		side          string
	}{
		"generational suffix": {
			primaryName:   "Kenneth Walker III",
			secondaryName: "Kenneth Walker",
			market:        odds.MarketRushYds,
			side:          odds.SideOver,
		},
		"initials with periods": {
			primaryName:   "D.J. Moore",
			secondaryName: "DJ Moore",
			market:        odds.MarketRecYds,
			side:          odds.SideOver,
		},
		"apostrophe and case": {
			primaryName:   "Ja'Marr Chase",
			secondaryName: "jamarr chase",
			market:        odds.MarketAnytimeTD,
			side:          odds.SideYES,
		},
		"jr suffix with period": {
			primaryName:   "Michael Penix Jr.",
			secondaryName: "Michael Penix",
			market:        odds.MarketPassYds,
			side:          odds.SideUnder,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var line *float64
			var secLine *float64
			wantPrice := -110
			if test.side != odds.SideYES {
				line = new(65.5)
				secLine = new(68.5)
				wantPrice = -115
			}

			primary := []odds.Prop{{
				EventID:    "evt-1",
				Book:       "draftkings",
				MarketType: test.market,
				PlayerName: test.primaryName,
				Side:       test.side,
				Line:       line,
				Price:      -110,
			}}
			secondary := []odds.Prop{{
				EventID:    "evt-1",
				Book:       "draftkings",
				MarketType: test.market,
				PlayerName: test.secondaryName,
				Side:       test.side,
				Line:       secLine,
				Price:      -115,
				Link:       "https://draftkings.test/prop",
				Sid:        "sid-1",
				EventSid:   "evt-sid-1",
			}}

			merged := odds.MergeProps(primary, secondary)
			require.Len(t, merged, 1, "variant player names should merge instead of duplicating")
			assert.Equal(t, test.primaryName, merged[0].PlayerName)
			assert.Equal(t, "https://draftkings.test/prop", merged[0].Link)
			assert.Equal(t, "sid-1", merged[0].Sid)
			assert.Equal(t, "evt-sid-1", merged[0].EventSid)
			assert.Equal(t, secLine, merged[0].Line)
			assert.Equal(t, wantPrice, merged[0].Price)
		})
	}
}

func TestFanDuelMarketPrefixAndPreference(t *testing.T) {
	t.Parallel()

	prefixTests := map[string]struct {
		link       string
		wantPrefix string
		wantOK     bool
	}{
		"US 42 prefix": {
			link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=101",
			wantPrefix: "42",
			wantOK:     true,
		},
		"non-42 734 prefix": {
			link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=734.199156488&selectionId=102",
			wantPrefix: "734",
			wantOK:     true,
		},
		"non-42 717 prefix": {
			link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=717.199156499&selectionId=103",
			wantPrefix: "717",
			wantOK:     true,
		},
		"missing dot": {
			link:   "https://sportsbook.fanduel.com/addToBetslip?marketId=42&selectionId=104",
			wantOK: false,
		},
		"non-numeric prefix": {
			link:   "https://sportsbook.fanduel.com/addToBetslip?marketId=us.12345&selectionId=105",
			wantOK: false,
		},
		"empty marketId suffix": {
			link:   "https://sportsbook.fanduel.com/addToBetslip?marketId=42.&selectionId=106",
			wantOK: false,
		},
		"no marketId param": {
			link:   "https://sportsbook.fanduel.com/buf",
			wantOK: false,
		},
	}

	for name, tc := range prefixTests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := odds.FanDuelMarketPrefix(tc.link)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantPrefix, got)
		})
	}

	fd42 := "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=101"
	fd734 := "https://sportsbook.fanduel.com/addToBetslip?marketId=734.199156488&selectionId=102"
	fd717 := "https://sportsbook.fanduel.com/addToBetslip?marketId=717.199156499&selectionId=103"

	assert.True(t, odds.PreferFanDuelLink("fanduel", fd734, fd42))
	assert.True(t, odds.PreferFanDuelLink("fanduel", fd717, fd42))
	assert.False(t, odds.PreferFanDuelLink("fanduel", fd42, fd734))
	assert.False(t, odds.PreferFanDuelLink("fanduel", fd42, fd42))
	assert.False(t, odds.PreferFanDuelLink("fanduel", fd734, fd717))
	assert.False(t, odds.PreferFanDuelLink("draftkings", fd734, fd42))
}

func TestMergePropsPrefersFanDuel42OverNon42Week12(t *testing.T) {
	t.Parallel()

	// Week 12 mixed-prefix FanDuel payload: Parlay API returns 734.* / 717.*
	// marketIds while The Odds API carries 42.* marketIds.
	primary := []odds.Prop{
		{
			EventID:    "evt-wk12",
			Book:       "fanduel",
			MarketType: odds.MarketRushYds,
			PlayerName: "Jahmyr Gibbs",
			Side:       odds.SideOver,
			Line:       new(74.5),
			Price:      -114,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=734.199156488&selectionId=73401",
			Sid:        "73401",
		},
		{
			EventID:    "evt-wk12",
			Book:       "fanduel",
			MarketType: odds.MarketRecYds,
			PlayerName: "Amon-Ra St. Brown",
			Side:       odds.SideOver,
			Line:       new(81.5),
			Price:      -115,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=717.199156499&selectionId=71701",
			Sid:        "71701",
		},
		{
			EventID:    "evt-wk12",
			Book:       "fanduel",
			MarketType: odds.MarketAnytimeTD,
			PlayerName: "David Montgomery",
			Side:       odds.SideYES,
			Price:      -125,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958600&selectionId=42000",
			Sid:        "42000",
		},
	}

	secondary := []odds.Prop{
		{
			EventID:    "evt-wk12",
			Book:       "fanduel",
			MarketType: odds.MarketRushYds,
			PlayerName: "Jahmyr Gibbs",
			Side:       odds.SideOver,
			Line:       new(74.5),
			Price:      -112,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=42001",
			Sid:        "42001",
			EventSid:   "35609999",
		},
		{
			EventID:    "evt-wk12",
			Book:       "fanduel",
			MarketType: odds.MarketRecYds,
			PlayerName: "Amon-Ra St. Brown",
			Side:       odds.SideOver,
			Line:       new(82.5),
			Price:      -110,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958633&selectionId=42002",
			Sid:        "42002",
			EventSid:   "35609999",
		},
		{
			EventID:    "evt-wk12",
			Book:       "fanduel",
			MarketType: odds.MarketAnytimeTD,
			PlayerName: "David Montgomery",
			Side:       odds.SideYES,
			Price:      -130,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=734.199156511&selectionId=73409",
			Sid:        "73409",
			EventSid:   "35609999",
		},
	}

	merged := odds.MergeProps(primary, secondary)
	require.Len(t, merged, 3)

	gibbs := findProp(t, merged, "fanduel", odds.MarketRushYds, "Jahmyr Gibbs", odds.SideOver)
	assert.Equal(t, "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=42001", gibbs.Link)
	assert.Equal(t, "42001", gibbs.Sid)
	assert.Equal(t, "35609999", gibbs.EventSid)
	assert.Equal(t, new(74.5), gibbs.Line)
	assert.Equal(t, -114, gibbs.Price)

	stBrown := findProp(t, merged, "fanduel", odds.MarketRecYds, "Amon-Ra St. Brown", odds.SideOver)
	assert.Equal(t, "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958633&selectionId=42002", stBrown.Link)
	assert.Equal(t, "42002", stBrown.Sid)
	assert.Equal(t, "35609999", stBrown.EventSid)
	assert.Equal(t, new(82.5), stBrown.Line, "adopts secondary line when replacing non-42 link on moved line")
	assert.Equal(t, -110, stBrown.Price)

	montgomery := findProp(t, merged, "fanduel", odds.MarketAnytimeTD, "David Montgomery", odds.SideYES)
	assert.Equal(
		t,
		"https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958600&selectionId=42000",
		montgomery.Link,
		"existing 42.* link is never replaced by a non-42 link",
	)
	assert.Equal(t, "42000", montgomery.Sid)
	assert.Equal(t, "35609999", montgomery.EventSid)
	assert.Equal(t, -125, montgomery.Price)

	// Duplicate primary rows for the same selection prefer 42.* over 734.*.
	deduped := odds.MergeProps([]odds.Prop{primary[0], secondary[0]}, nil)
	require.Len(t, deduped, 1)
	assert.Equal(t, "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=42001",
		deduped[0].Link)
}

// findProp returns the one merged row matching a selection, failing the test
// when it is missing or duplicated.
func findProp(
	t *testing.T,
	props []odds.Prop,
	book, marketType, subject, side string,
) odds.Prop {
	t.Helper()

	var found []odds.Prop
	for _, prop := range props {
		if prop.Book != book || prop.MarketType != marketType || prop.Side != side {
			continue
		}
		if prop.PlayerName != subject && prop.Team != subject {
			continue
		}
		found = append(found, prop)
	}
	require.Len(t, found, 1, "%s %s %s %s", book, marketType, subject, side)
	return found[0]
}

func TestHasPreferredFanDuelPrefix(t *testing.T) {
	t.Parallel()

	assert.True(t, odds.HasPreferredFanDuelPrefix(
		"https://sportsbook.fanduel.com/addToBetslip?marketId=42.578189645&selectionId=50199",
	))
	assert.False(t, odds.HasPreferredFanDuelPrefix(
		"https://sportsbook.fanduel.com/addToBetslip?marketId=734.199156488&selectionId=7017917",
	))
	assert.False(t, odds.HasPreferredFanDuelPrefix(
		"https://sportsbook.fanduel.com/addToBetslip?marketId=717.881234567&selectionId=991122",
	))
	assert.False(t, odds.HasPreferredFanDuelPrefix(
		"https://sportsbook.fanduel.com/addToBetslip?marketId=578189648&selectionId=7017917",
	))
	assert.False(t, odds.HasPreferredFanDuelPrefix("https://%zz"))
}
