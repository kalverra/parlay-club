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
func addGameOdds(t *testing.T, store *db.Store, gameID string) {
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

// gamesHandler rebuilds the web routes for a different signed-in user, which
// is how the home-book test changes the profile after the fixture is built.

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

func (s *stubRefresher) KickPickLines(_ context.Context, _ int64) {}

func (s *stubRefresher) KickPickGame(_ context.Context, _ int64, _ string) {}

func (s *stubRefresher) ForcePickRefresh(_ context.Context, _ int64, _ string) error {
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

func TestGamePropsRendersPlayerPortraitsAndFallback(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	handler := gamesHandlerWithRefresher(t, fixture, fixture.user, &stubRefresher{})

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Josh Allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png",
			UpdatedAt:   now,
		},
	}))

	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "MIA",
			PlayerName: "Unknown Player",
			Side:       "OVER",
			Line:       new(35.5),
			Price:      -115,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games/"+game.ID+"/props", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `src="https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png"`)
	assert.Contains(t, body, `data-fallback-img`)
	assert.Contains(t, body, `>JA</span>`)
	assert.Contains(t, body, `>UP</span>`)
}
