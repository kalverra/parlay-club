package web_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// gameKickoff is far enough ahead that the kickoff lock never fires.
func gameKickoff() time.Time {
	return time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
}

// addGame writes one game into the fixture's week.
func addGame(t *testing.T, fixture *slipFixture, kickoff time.Time) db.Game {
	t.Helper()

	game, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772001",
		WeekID:     fixture.week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    kickoff,
		Status:     db.GameScheduled,
		VenueID:    new("3915"),
		VenueName:  new("Highmark Stadium"),
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)
	return game
}

// addGameOdds writes three books with deliberately different lines and prices.
func addGameOdds(t *testing.T, store *db.Store, gameID string) time.Time {
	t.Helper()

	fetched := time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC)
	row := func(book, market, team, side string, line *float64, price int) db.Odds {
		return db.Odds{
			GameID:     gameID,
			Book:       book,
			MarketType: market,
			Team:       team,
			Side:       side,
			Line:       line,
			Price:      price,
			FetchedAt:  fetched,
		}
	}
	rows := []db.Odds{
		row("draftkings", "SPREAD", "BUF", "TEAM", new(-3.0), -110),
		row("draftkings", "SPREAD", "MIA", "TEAM", new(3.0), -110),
		row("draftkings", "ML", "BUF", "TEAM", nil, -150),
		row("draftkings", "ML", "MIA", "TEAM", nil, 130),
		row("draftkings", "TOTAL", "", "OVER", new(47.5), -110),
		row("draftkings", "TOTAL", "", "UNDER", new(47.5), -110),

		row("fanduel", "SPREAD", "BUF", "TEAM", new(-3.5), -105),
		row("fanduel", "SPREAD", "MIA", "TEAM", new(3.5), -115),
		row("fanduel", "ML", "BUF", "TEAM", nil, -145),
		row("fanduel", "ML", "MIA", "TEAM", nil, 125),
		row("fanduel", "TOTAL", "", "OVER", new(48.5), -110),
		row("fanduel", "TOTAL", "", "UNDER", new(48.5), -110),

		row("betmgm", "SPREAD", "BUF", "TEAM", new(-3.0), -115),
		row("betmgm", "SPREAD", "MIA", "TEAM", new(3.0), -105),
		row("betmgm", "ML", "BUF", "TEAM", nil, -160),
		row("betmgm", "ML", "MIA", "TEAM", nil, 140),
		row("betmgm", "TOTAL", "", "OVER", new(47.5), -105),
		row("betmgm", "TOTAL", "", "UNDER", new(47.5), -105),
	}

	changed, err := store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	require.True(t, changed)
	return fetched
}

func TestGamesPageShowsCardsLinesWeatherAndNote(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	kickoff := gameKickoff()
	game := addGame(t, fixture, kickoff)
	fetched := addGameOdds(t, fixture.store, game.ID)
	_, err := fixture.store.SetGameWeather(t.Context(), game.ID, 55, 22, 30, 0.2, time.Now().UTC())
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "MIA @ BUF")
	assert.Contains(t, body, fmt.Sprintf(`datetime="%s"`, kickoff.UTC().Format(time.RFC3339)))
	assert.Contains(t, body, "Upcoming")
	assert.Contains(t, body, "💨 22 mph")
	assert.Contains(t, body, "🌧")
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "MIA +3")
	assert.Contains(t, body, "BUF ML")
	assert.Contains(t, body, "MIA ML")
	assert.Contains(t, body, "Over 47.5")
	assert.Contains(t, body, "Under 47.5")
	assert.Contains(t, body, "Custom leg")
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/games/%s/props"`, game.ID))
	assert.Contains(t, body, `hx-trigger="toggle once"`)
	assert.Contains(t, body, fmt.Sprintf(`hx-target="#game-props-%s"`, game.ID))
	assert.Contains(t, body, "Lines as of")
	assert.Contains(t, body, fetched.Format("3:04 PM")+" UTC")
}

func TestGamesPageShowsLiveAndFinalStatus(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	kickoff := gameKickoff()
	base := db.Game{
		WeekID:     fixture.week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    kickoff,
		VenueID:    new("3915"),
		VenueName:  new("Highmark Stadium"),
		Indoor:     true,
		UpdatedAt:  time.Now().UTC(),
	}
	live := base
	live.ID = "401700001"
	live.Status = db.GameLive
	live.HomeScore = new(14)
	live.AwayScore = new(7)
	_, err := fixture.store.UpsertGame(t.Context(), live)
	require.NoError(t, err)

	final := base
	final.ID = "401700002"
	final.Status = db.GameFinal
	final.HomeScore = new(24)
	final.AwayScore = new(17)
	_, err = fixture.store.UpsertGame(t.Context(), final)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "Live")
	assert.Contains(t, body, "Final")
	assert.Contains(t, body, "MIA 7 - BUF 14")
	assert.Contains(t, body, "MIA 17 - BUF 24")
	assert.Contains(t, body, "🏠 Indoor")
}

func TestGamesPageIgnoresRowsFromAnOlderSync(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	fetched := addGameOdds(t, fixture.store, game.ID)

	stale := fetched.Add(-time.Hour)
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     game.ID,
		Book:       "betrivers",
		MarketType: "ML",
		Team:       "BUF",
		Side:       "TEAM",
		Price:      1000,
		FetchedAt:  stale,
	}})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "-150", "the current median moneyline still shows")
	assert.NotContains(t, body, "+1000", "a price from an older sync is left off the board")
}

func TestGamesPageCountsMoneylineOnlyBoardsAsHavingLines(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	fetched := time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC)
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     game.ID,
		Book:       "draftkings",
		MarketType: "ML",
		Team:       "BUF",
		Side:       "TEAM",
		Price:      -150,
		FetchedAt:  fetched,
	}})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "BUF ML")
	assert.NotContains(t, body, "No lines yet.")
}

func TestGamesPageUsesTheHomeBookFirst(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	user, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("fanduel"))
	require.NoError(t, err)
	handler := gamesHandler(t, fixture, user)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "BUF -3.5")
	assert.Contains(t, body, "MIA +3.5")
	assert.Contains(t, body, "Over 48.5")
	assert.Contains(t, body, "Under 48.5")
	assert.Contains(t, body, "-105")
	assert.Contains(t, body, "-115")
}

func TestQuickAddCreatesTheSelectedLeg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		market    string
		team      string
		side      string
		line      string
		odds      string
		selection string
		wantLine  *float64
		wantTeam  *string
		wantSide  string
	}{
		{
			name: "home spread", market: "SPREAD", team: "BUF", side: "TEAM", line: "-3.5", odds: "-105",
			selection: "BUF -3.5", wantLine: new(-3.5), wantTeam: new("BUF"), wantSide: "TEAM",
		},
		{
			name: "away spread", market: "SPREAD", team: "MIA", side: "TEAM", line: "3.5", odds: "-115",
			selection: "MIA +3.5", wantLine: new(3.5), wantTeam: new("MIA"), wantSide: "TEAM",
		},
		{
			name: "home moneyline", market: "ML", team: "BUF", side: "TEAM", odds: "-145",
			selection: "BUF ML", wantTeam: new("BUF"), wantSide: "TEAM",
		},
		{
			name: "away moneyline", market: "ML", team: "MIA", side: "TEAM", odds: "125",
			selection: "MIA ML", wantTeam: new("MIA"), wantSide: "TEAM",
		},
		{
			name: "over", market: "TOTAL", side: "OVER", line: "47.5", odds: "-110",
			selection: "Over 47.5", wantLine: new(47.5), wantSide: "OVER",
		},
		{
			name: "under", market: "TOTAL", side: "UNDER", line: "47.5", odds: "-105",
			selection: "Under 47.5", wantLine: new(47.5), wantSide: "UNDER",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fixture := newSlipFixture(t, 2)
			game := addGame(t, fixture, gameKickoff())

			form := url.Values{
				"quick":       {"1"},
				"game_id":     {game.ID},
				"market_type": {test.market},
				"team":        {test.team},
				"side":        {test.side},
				"odds":        {test.odds},
			}
			if test.line != "" {
				form.Set("line", test.line)
			}
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
				strings.NewReader(form.Encode()),
			)
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Hx-Request", "true")

			recorder := httptest.NewRecorder()
			fixture.handler.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Contains(t, html.UnescapeString(recorder.Body.String()), "Added "+test.selection)
			assert.Equal(t, "leg-saved", recorder.Header().Get("Hx-Trigger"))

			legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
			require.NoError(t, err)
			require.Len(t, legs, 1)
			leg := legs[0]
			assert.Equal(t, game.ID, *leg.GameID)
			assert.Equal(t, test.market, leg.MarketType)
			assert.Equal(t, test.selection, leg.Selection)
			assert.Equal(t, test.wantLine, leg.Line)
			assert.Equal(t, test.wantTeam, leg.Team)
			assert.Equal(t, test.wantSide, *leg.Side)
			assert.Equal(t, test.odds, strconv.Itoa(leg.Odds))
		})
	}
}

func TestCustomLegFormTargetsTheGameCard(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/legs/new?game_id=%s&quick=1", fixture.slip.ID, game.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, fmt.Sprintf(`hx-target="#quick-add-%s"`, game.ID))
	assert.Contains(t, body, `name="quick" value="1"`)
	assert.Contains(t, body, fmt.Sprintf(`<option value="%s" selected>`, game.ID))
	assert.Contains(t, body, `name="game_id"`)
}

// gamesHandler rebuilds the web routes for a different signed-in user, which
// is how the home-book test changes the profile after the fixture is built.
func gamesHandler(t *testing.T, fixture *slipFixture, user db.User) http.Handler {
	t.Helper()
	return gamesHandlerWithRefresher(t, fixture, user, nil)
}

func gamesHandlerWithRefresher(
	t *testing.T,
	fixture *slipFixture,
	user db.User,
	refresher webui.Refresher,
) http.Handler {
	t.Helper()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	hub := &live.Hub{}
	return webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: fixture.group.ID, Name: fixture.group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return fixture.group, true
		},
		Store:   fixture.store,
		Slips:   slips.New(fixture.store, hub, nil),
		Stream:  live.NewStream(hub, func(int64) bool { return false }),
		Refresh: refresher,
	})
}

type stubRefresher struct {
	propsSynced []string
}

func (s *stubRefresher) SyncGameProps(_ context.Context, gameID string) error {
	s.propsSynced = append(s.propsSynced, gameID)
	return nil
}

func (s *stubRefresher) SyncGameResearch(_ context.Context, _ string) error {
	return nil
}

func (s *stubRefresher) SyncPlayerResearch(_ context.Context, _ string) error {
	return nil
}

func TestGamePropsHandler(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	refresher := &stubRefresher{}
	handler := gamesHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// 1. Unknown game ID returns 404
	req404 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games/missing-game/props", nil)
	rec404 := httptest.NewRecorder()
	handler.ServeHTTP(rec404, req404)
	assert.Equal(t, http.StatusNotFound, rec404.Code)

	// 2. Empty props state returns empty message and research link
	reqEmpty := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/games/"+game.ID+"/props",
		nil,
	)
	recEmpty := httptest.NewRecorder()
	handler.ServeHTTP(recEmpty, reqEmpty)
	assert.Equal(t, http.StatusOK, recEmpty.Code)
	bodyEmpty := recEmpty.Body.String()
	assert.Contains(t, bodyEmpty, "/research?game_id="+game.ID)
	assert.Contains(t, bodyEmpty, "No prop lines available yet.")
	assert.Contains(t, refresher.propsSynced, game.ID)

	// Add roster player for BUF
	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}))

	// Add prop odds for the game
	oddsFetched := time.Date(2026, time.November, 9, 14, 0, 0, 0, time.UTC)
	propOdds := []db.Odds{
		// Team totals
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TEAM_TOTAL",
			Team:       "BUF",
			Side:       "OVER",
			Line:       new(24.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "TEAM_TOTAL",
			Team:       "BUF",
			Side:       "OVER",
			Line:       new(24.5),
			Price:      -115,
			FetchedAt:  oddsFetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TEAM_TOTAL",
			Team:       "BUF",
			Side:       "UNDER",
			Line:       new(24.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
		// Player props for matched player (Josh Allen)
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "ANYTIME_TD",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "YES",
			Line:       nil,
			Price:      150,
			FetchedAt:  oddsFetched,
		},
		// Player prop for unmatched player
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "",
			PlayerName: "Unknown Player",
			Side:       "OVER",
			Line:       new(35.5),
			Price:      -115,
			FetchedAt:  oddsFetched,
		},
	}
	_, err := fixture.store.UpsertOdds(t.Context(), propOdds)
	require.NoError(t, err)

	// Request props
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/games/%s/props", game.ID), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Game research link
	assert.Contains(t, body, "/research?game_id="+game.ID)
	// Matched player research link
	assert.Contains(t, body, `/research?player_id=3918298`)
	// Unmatched player has pills but no player research link
	assert.Contains(t, body, "Unknown Player")
	assert.NotContains(t, body, `/research?player_id="`)

	// Team totals
	assert.Contains(t, body, "BUF Over 24.5")
	assert.Contains(t, body, "BUF Under 24.5")

	// Player props
	assert.Contains(t, body, "Over 265.5 Pass Yds")
	assert.Contains(t, body, "Anytime TD")
	assert.Contains(t, body, "Over 35.5 Rec Yds")

	// Quick add form attributes
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/legs"`, fixture.slip.ID))
	assert.Contains(t, body, fmt.Sprintf(`hx-target="#quick-add-%s"`, game.ID))
	assert.Contains(t, body, `name="player_name" value="Josh Allen"`)
	assert.Contains(t, body, `name="player_id" value="3918298"`)

	// 3. User with HomeBook fanduel gets FanDuel price (-115 for BUF Over 24.5)
	fbUser := fixture.user
	fbUser.HomeBook = new("fanduel")
	fbHandler := gamesHandlerWithRefresher(t, fixture, fbUser, refresher)

	recFB := httptest.NewRecorder()
	fbHandler.ServeHTTP(recFB, req)
	assert.Contains(t, recFB.Body.String(), "-115")

	// 4. Locked slip renders non-interactive pills (no form)
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))

	recLocked := httptest.NewRecorder()
	handler.ServeHTTP(recLocked, req)
	bodyLocked := recLocked.Body.String()
	assert.NotContains(t, bodyLocked, `<form hx-post=`)
	assert.Contains(t, bodyLocked, "BUF Over 24.5")
	assert.Contains(t, bodyLocked, "Over 265.5 Pass Yds")
}

func TestCurrentOddsIndependentSyncGroupsAndNewestOdds(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	gameLineTime := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	propTime := time.Date(2026, time.November, 7, 14, 0, 0, 0, time.UTC)

	// Game lines fetched at 12:00
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "ML",
			Team:       "BUF",
			Side:       "TEAM",
			Price:      -150,
			FetchedAt:  gameLineTime,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.0),
			Price:      -110,
			FetchedAt:  gameLineTime,
		},
	})
	require.NoError(t, err)

	// Newer prop sync fetched at 14:00 (after gameLineTime)
	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -110,
			FetchedAt:  propTime,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	// Newer prop sync does NOT drop game lines; card still has lines
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "BUF ML")
	assert.NotContains(t, body, "No lines yet.")

	// Footer timestamp comes strictly from game-line rows (12:00 UTC), not prop sync (14:00 UTC)
	assert.Contains(t, body, "datetime=\"2026-11-07T12:00:00Z\"")
	assert.NotContains(t, body, "datetime=\"2026-11-07T14:00:00Z\"")
}
