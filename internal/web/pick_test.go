package web_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// addSecondGame writes another week game with its own teams.
func addSecondGame(t *testing.T, fixture *slipFixture, id, away, home, awayName, homeName string) db.Game {
	t.Helper()

	game, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         id,
		WeekID:     fixture.week.ID,
		HomeTeam:   home,
		AwayTeam:   away,
		HomeName:   homeName,
		AwayName:   awayName,
		HomeESPNID: "4",
		AwayESPNID: "5",
		Kickoff:    gameKickoff().Add(24 * time.Hour),
		Status:     db.GameScheduled,
		VenueID:    new("1"),
		VenueName:  new("A stadium"),
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)
	return game
}

// addPropOdds writes one prop row.
func addPropOdds(t *testing.T, store *db.Store, gameID, market, team, player, side string, line float64, price int) {
	t.Helper()

	_, err := store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     gameID,
		Book:       "draftkings",
		MarketType: market,
		Team:       team,
		PlayerName: player,
		Side:       side,
		Line:       new(line),
		Price:      price,
		FetchedAt:  time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC),
	}})
	require.NoError(t, err)
}

func pickBoard(t *testing.T, fixture *slipFixture, query string) *httptest.ResponseRecorder {
	t.Helper()

	target := fmt.Sprintf("/slips/%d/pick", fixture.slip.ID)
	if query != "" {
		target += "?" + query
	}
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return recorder
}

func TestPickBoardShowsSearchChipsSortsAndTray(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "BUF", "Josh Allen", "OVER", 245.5, -110)

	recorder := pickBoard(t, fixture, "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, fmt.Sprintf(`data-live-url="/slips/%d/events"`, fixture.slip.ID))
	assert.Equal(t, 1, strings.Count(body, "data-live-url"), "the board holds one stream open")
	assert.NotContains(t, body, `aria-current="page"`, "the pick board has no tab bar")

	assert.Contains(t, body, `placeholder="Search a player, team, or market"`)
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/pick/results"`, fixture.slip.ID))
	assert.Contains(t, body, "Lines")
	assert.Contains(t, body, "Passing")
	assert.Contains(t, body, "Any time")
	assert.Contains(t, body, "Kickoff")
	assert.Contains(t, body, "Your picks")
	assert.Contains(t, body, "0 of 2")
	assert.Contains(t, body, "Done")
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "Josh Allen Over 245.5 Pass Yds")
	assert.Contains(t, body, `name="from" value="board"`)
	assert.Contains(t, body, `hx-target="closest .pick-row"`)
}

func TestPickResultsSearchFilterAndSort(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "BUF", "Josh Allen", "OVER", 245.5, -110)
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "BUF", "Ja'Marr Chase", "OVER", 72.5, -115)

	target := fmt.Sprintf("/slips/%d/pick/results?q=Chase&cat=receiving&sort=odds", fixture.slip.ID)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "Ja'Marr Chase Over 72.5 Rec Yds")
	assert.NotContains(t, body, "Josh Allen")
	assert.NotContains(t, body, "BUF -3")
	assert.Contains(t, body, `value="Chase"`)
	assert.Contains(t, body, `value="receiving"`)
	assert.Contains(t, body, `value="odds"`)
}

func TestPickResultsNamesAPropGameThatIsNotLoaded(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	other := addSecondGame(t, fixture, "401772099", "CIN", "CLE", "Cincinnati Bengals", "Cleveland Browns")
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{{
		AthleteID: "99",
		Team:      "CIN",
		FullName:  "Ja'Marr Chase",
		UpdatedAt: time.Now().UTC(),
	}}))

	target := fmt.Sprintf("/slips/%d/pick/results?q=Chase", fixture.slip.ID)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "Props for CIN @ CLE aren’t loaded yet.",
		"the prompt names the game that matches the roster")
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/games/%s/props?slip=%d"`, other.ID, fixture.slip.ID))
	assert.Contains(t, body, "Load them")
}

func TestPickBoardNeverSyncsPropsOnItsOwn(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	refresher := &stubRefresher{}
	handler := gamesHandlerWithRefresher(t, fixture, fixture.user, refresher)

	for _, path := range []string{
		fmt.Sprintf("/slips/%d/pick", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?q=Allen", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?cat=passing", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?win=sun_early", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?sort=odds", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/tray", fixture.slip.ID),
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, path)
	}
	assert.Empty(t, refresher.propsSynced,
		"opening, searching, filtering, sorting, and the tray spend no prop credits")

	loadURL := fmt.Sprintf("/games/%s/props?slip=%d", game.ID, fixture.slip.ID)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, loadURL, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{game.ID}, refresher.propsSynced,
		"Load them asks for that one game's props")
	assert.Equal(t, "board-updated", recorder.Header().Get("Hx-Trigger"))
}

func TestPickRowTogglesAddAndRemove(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	form := url.Values{
		"quick":       {"1"},
		"from":        {"board"},
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-150"},
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
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "BUF ML")
	assert.Contains(t, body, "tap to remove")
	assert.Contains(t, body, "hx-delete=\"/legs/")
	assert.Equal(t, "tray-updated", recorder.Header().Get("Hx-Trigger"))

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	tray := httptest.NewRecorder()
	fixture.handler.ServeHTTP(tray, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/tray", fixture.slip.ID), nil))
	trayBody := html.UnescapeString(tray.Body.String())
	assert.Contains(t, trayBody, "1 of 2")
	assert.Contains(t, trayBody, "BUF ML")
	assert.Contains(t, trayBody, "Remove")

	remove := httptest.NewRecorder()
	fixture.handler.ServeHTTP(remove, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		fmt.Sprintf("/legs/%d?from=board", legs[0].ID),
		nil,
	))
	require.Equal(t, http.StatusOK, remove.Code, remove.Body.String())
	removedBody := html.UnescapeString(remove.Body.String())
	assert.Contains(t, removedBody, "BUF ML")
	assert.Contains(t, removedBody, "tap to add")
	assert.NotContains(t, removedBody, "hx-delete=")
	assert.Equal(t, "tray-updated", remove.Header().Get("Hx-Trigger"))
}

func TestPickBoardSwapsAtTheLimit(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	first := url.Values{
		"quick":       {"1"},
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-150"},
	}
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
		strings.NewReader(first.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	fixture.handler.ServeHTTP(httptest.NewRecorder(), request)

	board := html.UnescapeString(pickBoard(t, fixture, "").Body.String())
	assert.Contains(t, board, "swap in")
	assert.Contains(t, board, "/pick/swap?")

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	swap := url.Values{
		"from":        {"board"},
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"MIA"},
		"side":        {"TEAM"},
		"odds":        {"130"},
	}
	request = httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/legs/%d", legs[0].ID),
		strings.NewReader(swap.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "tray-updated", recorder.Header().Get("Hx-Trigger"))

	stored, err := fixture.store.LegByID(t.Context(), legs[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "MIA ML", stored.Selection)
}

func TestPickOneGameViewPairsBothSides(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	target := fmt.Sprintf("/slips/%d/pick?game=%s", fixture.slip.ID, game.ID)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "MIA @ BUF")
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "MIA +3")
	assert.Contains(t, body, "grid-cols-2", "both sides of a line sit in one row")
	assert.Contains(t, body, "Show the whole week")
}

func TestGamesRedirectsToTheActiveGroupWeek(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/games", nil))

	require.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, fmt.Sprintf("/groups/%d/week", fixture.group.ID), recorder.Header().Get("Location"))
}
