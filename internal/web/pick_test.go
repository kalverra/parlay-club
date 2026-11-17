package web_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// addSecondGame writes another week game with its own teams.
func addSecondGame(t *testing.T, fixture *slipFixture) db.Game {
	t.Helper()

	return addSecondGameAt(
		t,
		fixture,
		"401772099",
		"CIN",
		"CLE",
		"Cincinnati Bengals",
		"Cleveland Browns",
		gameKickoff().Add(24*time.Hour),
	)
}

// addSecondGameAt writes another week game at a chosen kickoff.
func addSecondGameAt(
	t *testing.T,
	fixture *slipFixture,
	id, away, home, awayName, homeName string,
	kickoff time.Time,
) db.Game {
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
		Kickoff:    kickoff,
		Status:     db.GameScheduled,
		VenueID:    new("1"),
		VenueName:  new("A stadium"),
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)
	return game
}

// addPropOdds writes one prop row.
func addPropOdds(t *testing.T, store *db.Store, gameID, market, player string, line float64, price int) {
	t.Helper()

	_, err := store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     gameID,
		Book:       "draftkings",
		MarketType: market,
		Team:       "BUF",
		PlayerName: player,
		Side:       "OVER",
		Line:       new(line),
		Price:      price,
		FetchedAt:  time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC),
	}})
	require.NoError(t, err)
}

// addSpreadOdds gives a game one line so its card can offer to load props.
func addSpreadOdds(t *testing.T, store *db.Store, gameID, team string) {
	t.Helper()

	_, err := store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     gameID,
		Book:       "draftkings",
		MarketType: "SPREAD",
		Team:       team,
		Side:       "TEAM",
		Line:       new(-3.0),
		Price:      -110,
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

func pickDock(t *testing.T, fixture *slipFixture) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/dock", fixture.slip.ID), nil))
	return recorder
}

func TestPickDockShowsTheCombinedPriceOfYourLegs(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 3)
	game := addGame(t, fixture, gameKickoff())

	partner, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Grace", "grace@example.com", "google-grace", "")
	require.NoError(t, err)
	require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, partner.ID))

	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "MIA ML",
		Team:       new("MIA"),
		Odds:       130,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     partner.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -3",
		Team:       new("BUF"),
		Odds:       -110,
	})

	recorder := pickDock(t, fixture)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "2 of 3", "the dock counts only the viewer's legs")
	assert.Contains(t, body, "4.39x (+339)", "the combined price multiplies the viewer's legs")
	assert.Contains(t, body, "Slip", "the dock keeps the way into the tray")
}

func TestPickBoardShowsSearchChipsSortsAndDock(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	recorder := pickBoard(t, fixture, "game="+game.ID)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, fmt.Sprintf(`data-live-url="/slips/%d/events"`, fixture.slip.ID))
	assert.Equal(t, 1, strings.Count(body, "data-live-url"), "the board holds one stream open")
	assert.NotContains(t, body, `aria-current="page"`, "the pick board has no tab bar")

	assert.Contains(t, body, `placeholder="Search a team or player"`)
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/pick/results"`, fixture.slip.ID))
	assert.Contains(t, body, "Lines")
	assert.Contains(t, body, "Passing")
	assert.Contains(t, body, "Any time")
	assert.Contains(t, body, "Player", "the open game's sort chips start at player")
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, fmt.Sprintf(
		`hx-get="/slips/%d/pick/detail?cat=passing&game=%s&sort=player"`, fixture.slip.ID, game.ID))
	assert.Contains(t, body, `name="from" value="board"`)
	assert.Contains(t, body, `hx-target="closest .pick-line-group"`)
	assert.Contains(t, body, `id="pick-body"`)
	assert.Contains(t, body, `id="pick-detail"`)
	assert.Contains(t, body, `data-category="lines"`, "an open game starts on the Lines tab")
	assert.Contains(t, body, `data-sort="player"`)
	assert.Contains(t, body, `hx-trigger="board-updated from:body"`, "a fresh prop sync refreshes the board")
	assert.Contains(t, body, `hx-sync="this:drop"`, "board-updated refreshes drop duplicate in-flight requests")
	assert.Contains(t, body, `hx-disinherit="hx-include hx-sync"`, "pick-slate does not pass hx-sync to child controls")
	assert.Contains(t, body, `hx-disinherit="hx-sync"`, "pick-detail does not pass hx-sync to child controls")
	assert.Contains(t, body, `hx-sync="this:replace"`, "rapid search input replaces any in-flight search request")

	assert.Contains(t, body, `id="pick-dock"`, "the dock replaces the aside tray")
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/pick/dock"`, fixture.slip.ID))
	assert.Contains(t, body, `hx-trigger="tray-updated from:body, slip-updated from:body"`,
		"a new leg or a fresh slip refreshes the dock")
	assert.Contains(t, body, "Your picks")
	assert.Contains(t, body, "0 of 2")
	assert.Contains(t, body, `hx-target="#pick-tray-body"`, "the dock opens the tray sheet")
	assert.Contains(t, body, `id="pick-tray-sheet"`)
	assert.Contains(t, body, `data-close-dock`)
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/tray"`, fixture.slip.ID))
	assert.NotContains(t, body, "<aside", "the tray no longer sits beside the body")
	assert.NotContains(t, body, ">Done<", "the one action lives in the tray sheet, not on the page")
	assert.NotContains(t, body, "1.00x", "an empty slip shows no combined price")
}

func TestPickBoardOmitsLiveStreamWhenGraded(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addGame(t, fixture, gameKickoff())

	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
	locked := pickBoard(t, fixture, "")
	require.Equal(t, http.StatusOK, locked.Code, locked.Body.String())
	assert.Contains(t, locked.Body.String(), fmt.Sprintf(`data-live-url="/slips/%d/events"`, fixture.slip.ID))

	require.NoError(t, fixture.store.SetSlipGrade(t.Context(), fixture.slip.ID, parlay.Won, db.SlipGraded))
	graded := pickBoard(t, fixture, "")
	require.Equal(t, http.StatusOK, graded.Code, graded.Body.String())
	assert.NotContains(t, graded.Body.String(), "data-live-url")
}

func TestPickBoardLinksGameCardsWithHTMXAndHrefFallback(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	body := html.UnescapeString(pickBoard(t, fixture, "").Body.String())
	fullURL := fmt.Sprintf("/slips/%d/pick?game=%s", fixture.slip.ID, game.ID)
	resultsURL := fmt.Sprintf("/slips/%d/pick/results?game=%s", fixture.slip.ID, game.ID)

	assert.Contains(t, body, `href="`+fullURL+`"`, "the card works without JavaScript")
	assert.Contains(t, body, `hx-get="`+resultsURL+`"`)
	assert.Contains(t, body, `hx-target="#pick-body"`)
	assert.Contains(t, body, `hx-push-url="`+fullURL+`"`)
	assert.Contains(t, body, "Explore 1 prop")
	assert.Contains(t, body, "data-keep-scroll")
	assert.Contains(t, body, "max-w-6xl", "the board uses the wide column")
	assert.Contains(t, body, ">Spread<")
	assert.Contains(t, body, ">Total<")
	assert.Contains(t, body, ">Moneyline<")
	assert.Contains(t, body, " ET</time>", "the server renders an ET fallback before app.js localizes it")
}

func TestPickBoardFallsBackToTheSlateForAnUnknownGame(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	body := html.UnescapeString(pickBoard(t, fixture, "game=missing").Body.String())
	assert.Contains(t, body, "MIA @ BUF", "the slate still lists the week")
	assert.NotContains(t, body, `id="pick-detail"`, "an unknown game opens no detail")
}

func TestPickBackLinkKeepsTheSearchAndWindow(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	now := time.Now().In(loc)
	kickoff := time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, loc)
	for kickoff.Weekday() != time.Sunday || !kickoff.After(now) {
		kickoff = kickoff.Add(24 * time.Hour)
	}
	game := addSecondGameAt(t, fixture, "401772099", "CIN", "CLE", "Cincinnati Bengals", "Cleveland Browns", kickoff)
	addGameOdds(t, fixture.store, game.ID)

	target := fmt.Sprintf(
		"/slips/%d/pick?game=%s&cat=passing&q=Buffalo&win=sun_early",
		fixture.slip.ID,
		game.ID,
	)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, fmt.Sprintf(`href="/slips/%d/pick?q=Buffalo&win=sun_early"`, fixture.slip.ID))
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/pick/results?q=Buffalo&win=sun_early"`, fixture.slip.ID))
}

func TestPickBoardSaysWhenTheWeekHasNoGames(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := pickBoard(t, fixture, "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "No games are stored for this week yet.",
		"an empty slate is its own screen")
	assert.NotContains(t, body, "Nothing matches that search")
	assert.NotContains(t, body, "Load them")
}

func TestPickBoardSaysWhenTheWeekHasNoLines(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addGame(t, fixture, gameKickoff())

	recorder := pickBoard(t, fixture, "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "No lines for this week yet.",
		"a week with games but no odds says what is missing")
	assert.NotContains(t, body, "Nothing matches that search")
	assert.NotContains(t, body, "aren’t loaded yet",
		"one week-level message replaces a Load them prompt per game")
	assert.NotContains(t, body, "Load them")
}

func TestPickBoardSaysWhenEveryGameStarted(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addGame(t, fixture, time.Now().UTC().Add(-3*time.Hour))

	recorder := pickBoard(t, fixture, "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "Every game this week has kicked off. Picks are closed.")
	assert.NotContains(t, body, "Nothing matches that search")
	assert.NotContains(t, body, "already kicked off and is hidden",
		"the all-started screen does not also list each hidden game")
}

func TestPickResultsSearchFilterAndSort(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "Ja'Marr Chase", 72.5, -115)

	target := fmt.Sprintf(
		"/slips/%d/pick/results?game=%s&q=Chase&cat=receiving&sort=odds",
		fixture.slip.ID,
		game.ID,
	)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "Ja'Marr Chase")
	assert.Contains(t, body, "Over 72.5")
	assert.Contains(t, body, "Rec Yds", "the player card names the market above its line")
	assert.NotContains(t, body, "Josh Allen")
	assert.NotContains(t, body, "BUF -3")
	assert.Contains(t, body, `value="Chase"`)
	assert.Contains(t, body, `value="receiving"`)
	assert.Contains(t, body, `value="odds"`)
}

func TestPickBoardKeepsEachGamesProps(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	first := addGame(t, fixture, gameKickoff())
	second := addSecondGame(t, fixture)
	fetched := time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC)
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID: first.ID, Book: "draftkings", MarketType: "PASS_YDS",
			Team: "BUF", PlayerName: "Josh Allen", Side: "OVER",
			Line: new(245.5), Price: -110, FetchedAt: fetched,
		},
		{
			GameID: second.ID, Book: "draftkings", MarketType: "PASS_YDS",
			Team: "CIN", PlayerName: "Joe Burrow", Side: "OVER",
			Line: new(255.5), Price: -110, FetchedAt: fetched.Add(time.Hour),
		},
	})
	require.NoError(t, err)

	firstBody := html.UnescapeString(pickBoard(t, fixture, "game="+first.ID+"&cat=passing").Body.String())
	assert.Contains(t, firstBody, "Josh Allen",
		"a newer prop sync for one game leaves another game's props on the board")
	assert.Contains(t, firstBody, "Over 245.5")
	assert.NotContains(t, firstBody, "Joe Burrow")

	secondBody := html.UnescapeString(pickBoard(t, fixture, "game="+second.ID+"&cat=passing").Body.String())
	assert.Contains(t, secondBody, "Joe Burrow")
	assert.Contains(t, secondBody, "Over 255.5")
	assert.NotContains(t, secondBody, "Josh Allen")
	assert.NotContains(t, firstBody, "aren’t loaded yet")
	assert.NotContains(t, secondBody, "aren’t loaded yet")
}

func TestPickResultsNamesAPropGameThatIsNotLoaded(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	other := addSecondGame(t, fixture)
	addSpreadOdds(t, fixture.store, other.ID, "CIN")
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
	other := addSecondGame(t, fixture)
	addSpreadOdds(t, fixture.store, other.ID, "CIN")
	refresher := &stubRefresher{}
	handler := gamesHandlerWithRefresher(t, fixture, fixture.user, refresher)

	for _, path := range []string{
		fmt.Sprintf("/slips/%d/pick", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?q=Allen", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?cat=passing", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?win=sun_early", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?sort=odds", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/detail?game=%s", fixture.slip.ID, game.ID),
		fmt.Sprintf("/slips/%d/pick/detail?game=%s&cat=passing", fixture.slip.ID, game.ID),
		fmt.Sprintf("/slips/%d/pick/dock", fixture.slip.ID),
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

func TestPickLoadPropsPromptSitsOnTheCardAndTheDetail(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID) // lines only, no props yet
	loadHref := fmt.Sprintf(`hx-get="/games/%s/props?slip=%d"`, game.ID, fixture.slip.ID)

	slate := html.UnescapeString(pickBoard(t, fixture, "").Body.String())
	assert.Contains(t, slate, "Props for MIA @ BUF aren’t loaded yet.",
		"the prompt lives on the game card")
	assert.Contains(t, slate, loadHref)
	assert.Contains(t, slate, "Load them")

	detail := html.UnescapeString(pickBoard(t, fixture, "game="+game.ID).Body.String())
	assert.Contains(t, detail, "Props for this game aren’t loaded yet.",
		"the open detail offers the same prompt")
	assert.Equal(t, 2, strings.Count(detail, "Load them"),
		"the card and the detail each carry one button")
	assert.Equal(t, 2, strings.Count(detail, loadHref))
}

func TestPickGameWithNoLinesShowsTheWeekStateInsteadOfAPropPrompt(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addGame(t, fixture, gameKickoff())

	body := html.UnescapeString(pickBoard(t, fixture, "").Body.String())
	assert.Contains(t, body, "No lines for this week yet.")
	assert.NotContains(t, body, "Load them", "a game with no lines cannot load props")
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

func TestPickPairRowAddsOneSideAndUpdatesTheOther(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	form := url.Values{
		"quick":       {"1"},
		"from":        {"board"},
		"game_id":     {game.ID},
		"market_type": {"SPREAD"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"line":        {"-3"},
		"odds":        {"-110"},
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

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	assert.Contains(t, body, `class="pick-line-group"`, "the whole pair row comes back")
	assert.Contains(t, body, "grid-cols-2", "both sides of the line sit in one row")
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "MIA +3", "the opposite side comes back too")
	assert.Contains(t, body, fmt.Sprintf(`hx-delete="/legs/%d?from=board"`, legs[0].ID))
	assert.Contains(t, body, fmt.Sprintf(`hx-put="/legs/%d"`, legs[0].ID),
		"the opposite side updates the picked leg instead of adding another")
	assert.Contains(t, body, "tap to switch")
	assert.Contains(t, body, `hx-target="closest .pick-line-group"`)
	assert.Equal(t, "tray-updated", recorder.Header().Get("Hx-Trigger"))
}

func TestPickPairRowSwitchesSidesThroughPUT(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -3",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-3.0),
		Odds:       -110,
	})

	form := url.Values{
		"from":        {"board"},
		"game_id":     {game.ID},
		"market_type": {"SPREAD"},
		"team":        {"MIA"},
		"side":        {"TEAM"},
		"line":        {"3"},
		"odds":        {"-110"},
	}
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/legs/%d", leg.ID),
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, `class="pick-line-group"`)
	assert.Contains(t, body, "MIA +3")
	assert.Contains(t, body, "tap to remove", "the switched-to side is picked now")
	assert.Contains(t, body, "BUF -3", "the old side stays on the row")
	assert.Contains(t, body, fmt.Sprintf(`hx-put="/legs/%d"`, leg.ID))
	assert.Equal(t, "tray-updated", recorder.Header().Get("Hx-Trigger"))

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1, "switching sides updates the leg instead of adding one")
	assert.Equal(t, "MIA +3", legs[0].Selection)
}

func TestPickPairRowRefreshesTheBoardWhenTheLegMovesLines(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Odds:       -150,
	})

	form := url.Values{
		"from":        {"board"},
		"game_id":     {game.ID},
		"market_type": {"SPREAD"},
		"team":        {"MIA"},
		"side":        {"TEAM"},
		"line":        {"3"},
		"odds":        {"-110"},
	}
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/legs/%d", leg.ID),
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.Equal(t, "tray-updated, board-updated", recorder.Header().Get("Hx-Trigger"),
		"a leg that leaves its line needs the old row refreshed too")
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "MIA +3")
	assert.Contains(t, body, "tap to remove")
}

func TestPickPairRowClearsThePickOnDelete(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -3",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-3.0),
		Odds:       -110,
	})

	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		fmt.Sprintf("/legs/%d?from=board", leg.ID),
		nil,
	))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, `class="pick-line-group"`)
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "MIA +3", "the pair row survives the delete")
	assert.Equal(t, 2, strings.Count(body, "tap to add"), "both sides are open again")
	assert.NotContains(t, body, "hx-delete=")
	assert.NotContains(t, body, "hx-put=")
	assert.Equal(t, "tray-updated", recorder.Header().Get("Hx-Trigger"))

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	assert.Empty(t, legs)
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

	board := html.UnescapeString(pickBoard(t, fixture, "game="+game.ID).Body.String())
	assert.Contains(t, board, "swap in")
	assert.Contains(t, board, "/pick/swap?")

	swapSheet := url.Values{
		"game":        {game.ID},
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"MIA"},
		"side":        {"TEAM"},
		"odds":        {"130"},
		"text":        {"MIA ML"},
	}
	sheet := httptest.NewRecorder()
	fixture.handler.ServeHTTP(sheet, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/swap?%s", fixture.slip.ID, swapSheet.Encode()),
		nil,
	))
	require.Equal(t, http.StatusOK, sheet.Code, sheet.Body.String())
	sheetBody := html.UnescapeString(sheet.Body.String())
	assert.Contains(t, sheetBody, "Swap which pick?")
	assert.Contains(t, sheetBody, "Swap out BUF ML")
	assert.Contains(t, sheetBody, `hx-put="/legs/`)
	assert.Contains(t, sheetBody, `hx-target="#pick-line-`,
		"the swap switches that line's row instead of the whole board")

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	swap := url.Values{
		"from":        {"board"},
		"game":        {game.ID},
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
	results := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, results, "MIA ML", "the swapped-in pick's row comes back")
	assert.Contains(t, results, "tap to remove", "the swapped-in row is now a picked row")
	assert.Contains(t, results, `id="pick-line-`, "the swap answers with the refreshed pair row")

	stored, err := fixture.store.LegByID(t.Context(), legs[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "MIA ML", stored.Selection)

	activity, err := fixture.store.ActivityForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	messages := make([]string, 0, len(activity))
	for _, entry := range activity {
		messages = append(messages, entry.Message)
	}
	assert.Contains(t, messages, "Ada changed BUF ML to MIA ML",
		"a swap writes one changed entry")

	tray := httptest.NewRecorder()
	fixture.handler.ServeHTTP(tray, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/tray", fixture.slip.ID), nil))
	trayBody := html.UnescapeString(tray.Body.String())
	assert.Contains(t, trayBody, "1 of 1", "the tray keeps the progress after a swap")
	assert.Contains(t, trayBody, "MIA ML", "the tray shows the swapped-in pick")
	assert.Contains(t, trayBody, "Review and lock in", "the tray offers the next step at the limit")
}

func TestPickSwapRefusesALegWhoseGameKickedOff(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	upcoming := addGame(t, fixture, gameKickoff())
	past := addSecondGameAt(t, fixture, "401772098", "CIN", "CLE",
		"Cincinnati Bengals", "Cleveland Browns", gameKickoff().Add(-72*time.Hour))
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(past.ID),
		MarketType: "ML",
		Selection:  "CLE ML",
		Team:       new("CLE"),
		Side:       new("TEAM"),
		Odds:       120,
	})

	// The sheet leaves a kicked-off pick out of the choices.
	sheet := httptest.NewRecorder()
	fixture.handler.ServeHTTP(sheet, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf(
			"/slips/%d/pick/swap?game_id=%s&market_type=ML&team=BUF&side=TEAM&odds=-150&text=BUF+ML",
			fixture.slip.ID,
			upcoming.ID,
		),
		nil,
	))
	require.Equal(t, http.StatusOK, sheet.Code, sheet.Body.String())
	sheetBody := html.UnescapeString(sheet.Body.String())
	assert.NotContains(t, sheetBody, "Swap out CLE ML", "a kicked-off pick cannot be swapped out")
	assert.Contains(t, sheetBody, "Every one of your picks has kicked off.")

	form := url.Values{
		"from":        {"board"},
		"game_id":     {upcoming.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-150"},
	}
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/legs/%d", leg.ID),
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "that game already kicked off")

	stored, err := fixture.store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	assert.Equal(t, "CLE ML", stored.Selection, "a refused swap leaves the pick alone")
}

func TestPickSwapRefusesALockedInMember(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	game := addGame(t, fixture, gameKickoff())
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Odds:       -150,
	})

	lock := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/lock", fixture.slip.ID), nil)
	lock.Header.Set("Hx-Request", "true")
	lockRecorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(lockRecorder, lock)
	require.Equal(t, http.StatusOK, lockRecorder.Code, lockRecorder.Body.String())

	form := url.Values{
		"from":        {"board"},
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"MIA"},
		"side":        {"TEAM"},
		"odds":        {"130"},
	}
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/legs/%d", leg.ID),
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "you've locked in")

	stored, err := fixture.store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	assert.Equal(t, "BUF ML", stored.Selection, "a locked-in member cannot swap")
}

func TestPickTrayRemoveRefreshesTheDock(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
		Odds:       -110,
	})

	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodDelete,
		fmt.Sprintf("/legs/%d?from=tray", leg.ID),
		nil,
	))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "tray-updated, board-updated", recorder.Header().Get("Hx-Trigger"),
		"removing a pick in the sheet refreshes the dock and the board")
	body := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, body, "0 of 2")
	assert.Contains(t, body, "No picks yet.")
}

func TestPickTrayShowsOnlyHeadsUpsOnYourLegs(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	sam, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, sam.ID))

	// Sam's lone receiver leg trips the median trap. It names only his pick.
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "4430756",
		GamesPlayed: 5,
		StatsJSON:   `{"REC_YDS":[41.0,41.0,41.0,41.0,126.0]}`,
		UpdatedAt:   time.Now().UTC(),
	}))

	// The spread and the passing over together trip the correlation rules,
	// which name both owners.
	addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -7.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-7.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     sam.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 245.5 Pass Yds",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(245.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     sam.ID,
		GameID:     new(game.ID),
		MarketType: "REC_YDS",
		Selection:  "Dawson Knox Over 52.5 Rec Yds",
		Team:       new("BUF"),
		PlayerID:   new("4430756"),
		PlayerName: new("Dawson Knox"),
		Side:       new("OVER"),
		Line:       new(52.5),
		Odds:       -110,
	})

	tray := httptest.NewRecorder()
	fixture.handler.ServeHTTP(tray, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/tray", fixture.slip.ID), nil))
	require.Equal(t, http.StatusOK, tray.Code, tray.Body.String())
	body := html.UnescapeString(tray.Body.String())

	assert.Contains(t, body, `data-rule="hook"`,
		"a correlation heads-up that names the viewer's leg belongs in the tray")
	assert.Contains(t, body, "Ada's BUF -7.5")
	assert.Contains(t, body, "Sam's Josh Allen Over 245.5 Pass Yds",
		"a shared heads-up keeps the friend's side visible")
	assert.NotContains(t, body, "Dawson Knox's", "a heads-up on only a friend's pick stays out")
	assert.NotContains(t, body, `data-rule="median_trap"`)
	assert.Contains(t, body, "1 of 2")
}

func TestPickTrayMatchesTheViewerByIDNotName(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	// Two members share a display name. Names come from Google and can repeat.
	owner, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Chris", "chris-owner@example.com", "google-chris-owner", "")
	require.NoError(t, err)
	viewer, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Chris", "chris-viewer@example.com", "google-chris-viewer", "")
	require.NoError(t, err)
	for _, chris := range []db.User{owner, viewer} {
		_, err = fixture.store.JoinByToken(t.Context(), chris.ID, fixture.group.InviteToken)
		require.NoError(t, err)
		require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, chris.ID))
	}

	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "4430756",
		GamesPlayed: 5,
		StatsJSON:   `{"REC_YDS":[41.0,41.0,41.0,41.0,126.0]}`,
		UpdatedAt:   time.Now().UTC(),
	}))
	addStoredLeg(t, fixture, db.Leg{
		UserID:     owner.ID,
		GameID:     new(game.ID),
		MarketType: "REC_YDS",
		Selection:  "Dawson Knox Over 52.5 Rec Yds",
		Team:       new("BUF"),
		PlayerID:   new("4430756"),
		PlayerName: new("Dawson Knox"),
		Side:       new("OVER"),
		Line:       new(52.5),
		Odds:       -110,
	})
	addStoredLeg(t, fixture, db.Leg{
		UserID:     viewer.ID,
		GameID:     new(game.ID),
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
		Odds:       -110,
	})

	getTray := func() string {
		t.Helper()
		tray := httptest.NewRecorder()
		fixture.handler.ServeHTTP(tray, httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/tray", fixture.slip.ID), nil))
		require.Equal(t, http.StatusOK, tray.Code, tray.Body.String())
		return html.UnescapeString(tray.Body.String())
	}

	fixture.as(viewer)
	body := getTray()
	assert.NotContains(t, body, `data-rule="median_trap"`,
		"the other Chris's heads-up does not belong in this viewer's tray")
	assert.NotContains(t, body, "Dawson Knox")

	fixture.as(owner)
	assert.Contains(t, getTray(), `data-rule="median_trap"`,
		"the owner still sees the heads-up on their own leg")
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
	assert.Contains(t, body, "All games")
	assert.Contains(t, body, "hidden lg:block", "a phone drilldown hides the slate")
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/pick/results"`, fixture.slip.ID))
	assert.Contains(t, body, `hx-target="#pick-body"`)
	assert.Contains(
		t,
		body,
		fmt.Sprintf(`hx-get="/slips/%d/pick/detail?cat=lines&game=%s&sort=player"`, fixture.slip.ID, game.ID),
		"the detail's tab and sort links ask for the detail alone",
	)
}

// pickDetail fetches the detail partial for one game.
func pickDetail(t *testing.T, fixture *slipFixture, query string) *httptest.ResponseRecorder {
	t.Helper()

	target := fmt.Sprintf("/slips/%d/pick/detail", fixture.slip.ID)
	if query != "" {
		target += "?" + query
	}
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return recorder
}

// addTeamTotal writes one side of a team total.
func addTeamTotal(t *testing.T, store *db.Store, gameID, team, side string, line float64) {
	t.Helper()

	_, err := store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     gameID,
		Book:       "draftkings",
		MarketType: "TEAM_TOTAL",
		Team:       team,
		Side:       side,
		Line:       new(line),
		Price:      -110,
		FetchedAt:  time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC),
	}})
	require.NoError(t, err)
}

func TestPickDetailReturnsOnlyTheOpenGamesDetail(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)
	addPropOdds(t, fixture.store, game.ID, "PASS_TDS", "Josh Allen", 1.5, 120)
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "Ja'Marr Chase", 72.5, -115)

	recorder := pickDetail(t, fixture, "game="+game.ID+"&cat=passing")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.NotContains(t, body, `id="pick-body"`, "a tab change leaves the slate alone")
	assert.NotContains(t, body, `placeholder="Search a team or player"`)
	assert.Contains(t, body, `id="pick-detail"`)
	assert.Contains(t, body, `data-category="passing"`)
	assert.Contains(t, body, `data-sort="player"`)
	assert.Contains(t, body, `hx-target="#pick-detail"`)
	assert.Contains(t, body, "overflow-x-auto", "the tab bar scrolls sideways on a phone")

	lines := strings.Index(body, ">Lines")
	passing := strings.Index(body, ">Passing")
	receiving := strings.Index(body, ">Receiving")
	require.GreaterOrEqual(t, lines, 0)
	require.GreaterOrEqual(t, passing, 0)
	require.GreaterOrEqual(t, receiving, 0)
	assert.Less(t, lines, passing, "the tabs keep the board order")
	assert.Less(t, passing, receiving)

	assert.Equal(t, 1, strings.Count(body, ">Josh Allen</h3>"),
		"one player card clusters both of his passing markets")
	assert.Contains(t, body, "Pass TDs")
	assert.Contains(t, body, "Pass Yds")
	assert.Contains(t, body, "Over 1.5")
	assert.Contains(t, body, "Over 245.5")
	assert.NotContains(t, body, "Ja'Marr Chase", "a passing tab leaves receiving rows out")

	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/legs"`, fixture.slip.ID))
	assert.Contains(t, body, `hx-target="closest .pick-line-group"`, "add and remove keep working inside each row")
}

func TestPickDetailLinesTabShowsGameLinesThenTeamTotals(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addTeamTotal(t, fixture.store, game.ID, "BUF", "OVER", 24.5)
	addTeamTotal(t, fixture.store, game.ID, "BUF", "UNDER", 24.5)
	addTeamTotal(t, fixture.store, game.ID, "MIA", "OVER", 20.5)
	addTeamTotal(t, fixture.store, game.ID, "MIA", "UNDER", 20.5)

	recorder := pickDetail(t, fixture, "game="+game.ID)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, `data-category="lines"`, "team totals fold into Lines")
	spread := strings.Index(body, ">Spread</p>")
	moneyline := strings.Index(body, ">Moneyline</p>")
	total := strings.Index(body, ">Total</p>")
	buffalo := strings.Index(body, ">BUF team total</p>")
	miami := strings.Index(body, ">MIA team total</p>")
	require.GreaterOrEqual(t, spread, 0, "the Lines tab names its spread")
	require.GreaterOrEqual(t, moneyline, 0, "the Lines tab names its moneyline")
	require.GreaterOrEqual(t, total, 0, "the Lines tab names its total")
	require.GreaterOrEqual(t, buffalo, 0, "the team total keeps its team")
	require.GreaterOrEqual(t, miami, 0, "the team total keeps its team")
	assert.Less(t, spread, moneyline, "the game lines come first")
	assert.Less(t, moneyline, total)
	assert.Less(t, total, buffalo, "team totals come after the game lines")
	assert.Less(t, buffalo, miami)
	assert.Contains(t, body, "BUF Team Total Over 24.5")
	assert.Contains(t, body, "MIA Team Total Over 20.5")
	assert.Contains(t, body, "grid-cols-2", "both sides of a line sit in one row")
}

func TestPickDetailKeepsTheOpenTabAndSortInEveryLink(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	recorder := pickDetail(t, fixture, "game="+game.ID+"&cat=passing&sort=line")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := html.UnescapeString(recorder.Body.String())

	base := fmt.Sprintf(`/slips/%d/pick`, fixture.slip.ID)
	assert.Contains(t, body, base+fmt.Sprintf("?cat=passing&game=%s&sort=line", game.ID),
		"the active tab link keeps the sort")
	assert.Contains(t, body, base+fmt.Sprintf("?cat=lines&game=%s&sort=line", game.ID),
		"another tab keeps the sort")
	assert.Contains(t, body, base+fmt.Sprintf("?cat=passing&game=%s&sort=odds", game.ID),
		"a sort chip keeps the tab")
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/pick/detail?cat=passing&game=%s&sort=odds"`,
		fixture.slip.ID, game.ID))
}

func TestPickDetailKeepsTheTabThroughALiveRefreshAndSearch(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "Ja'Marr Chase", 72.5, -115)

	query := "game=" + game.ID + "&cat=passing&sort=odds"
	boardBody := html.UnescapeString(pickBoard(t, fixture, query).Body.String())
	assert.Contains(t, boardBody, `data-category="passing"`)
	assert.Contains(t, boardBody, `data-sort="odds"`)
	assert.Contains(t, boardBody, `id="pick-cat" type="hidden" name="cat" value="passing"`,
		"the search form keeps the open tab")
	assert.Contains(t, boardBody, `id="pick-sort" type="hidden" name="sort" value="odds"`,
		"the search form keeps the open sort")
	assert.Contains(
		t,
		boardBody,
		fmt.Sprintf(`hx-get="/slips/%d/pick/detail?cat=passing&game=%s&sort=odds"`, fixture.slip.ID, game.ID),
		"the detail refreshes itself from the resolved tab and sort",
	)
	assert.Contains(t, boardBody, `hx-include="#pick-q, #pick-cat, #pick-win, #pick-sort, #pick-game"`,
		"the slate refresh sends the current filters")

	detailBody := html.UnescapeString(pickDetail(t, fixture, query).Body.String())
	assert.Contains(t, detailBody, `id="pick-cat" type="hidden" name="cat" value="passing" hx-swap-oob="true"`,
		"a tab swap syncs the search form out of band")
	assert.Contains(t, detailBody, `id="pick-sort" type="hidden" name="sort" value="odds" hx-swap-oob="true"`,
		"a sort swap syncs the search form out of band")

	slate := httptest.NewRecorder()
	fixture.handler.ServeHTTP(slate, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/slate?%s", fixture.slip.ID, query), nil))
	require.Equal(t, http.StatusOK, slate.Code, slate.Body.String())
	slateBody := html.UnescapeString(slate.Body.String())
	assert.NotContains(t, slateBody, `id="pick-detail"`, "a slate refresh leaves the detail alone")
	assert.Contains(t, slateBody, `id="pick-cat" type="hidden" name="cat" value="passing"`,
		"a slate refresh keeps the open tab")
	assert.Contains(t, slateBody, `id="pick-sort" type="hidden" name="sort" value="odds"`,
		"a slate refresh keeps the open sort")

	search := httptest.NewRecorder()
	fixture.handler.ServeHTTP(search, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/results?%s&q=Allen", fixture.slip.ID, query), nil))
	require.Equal(t, http.StatusOK, search.Code, search.Body.String())
	searchBody := html.UnescapeString(search.Body.String())
	assert.Contains(t, searchBody, `data-category="passing"`, "a search keeps the open tab")
	assert.Contains(t, searchBody, `data-sort="odds"`, "a search keeps the open sort")
	assert.Contains(t, searchBody, `id="pick-cat" type="hidden" name="cat" value="passing"`)
}

func TestPickSearchPushesTheFullPageURL(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	target := fmt.Sprintf(
		"/slips/%d/pick/results?q=Allen&game=%s&cat=passing&sort=odds",
		fixture.slip.ID,
		game.ID,
	)
	recorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.Equal(
		t,
		fmt.Sprintf("/slips/%d/pick?cat=passing&game=%s&q=Allen&sort=odds", fixture.slip.ID, game.ID),
		recorder.Header().Get("Hx-Push-Url"),
		"a search writes the full page URL into the address bar",
	)
}

func TestPickControlsMeetTheTouchTarget(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	body := html.UnescapeString(pickBoard(t, fixture, "game="+game.ID+"&cat=passing").Body.String())

	assert.Contains(t, body, "inline-flex min-h-12 items-center rounded-full px-2.5 py-1",
		"the window chips reach 48px")
	assert.Contains(t, body, "inline-flex min-h-12 shrink-0 items-center rounded-full px-2.5 py-1",
		"the tab chips reach 48px")
	assert.Contains(t, body, "inline-flex min-h-12 items-center rounded-full px-2 py-0.5",
		"the sort chips reach 48px")
	assert.Contains(t, body, "inline-flex min-h-12 items-center rounded-md border",
		"the back link reaches 48px")
	assert.Contains(t, body, "inline-flex min-h-12 shrink-0 items-center rounded-lg bg-sky-600 px-4 py-2",
		"the dock button reaches 48px")
}

func TestPickDetailSortsByOddsAndLine(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "Keon Coleman", 41.5, 150)
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "Zay Flowers", 55.5, -200)
	addPropOdds(t, fixture.store, game.ID, "REC_YDS", "Khalil Shakir", 72.5, 120)

	order := func(sort string) []int {
		t.Helper()

		body := html.UnescapeString(pickDetail(t, fixture,
			"game="+game.ID+"&cat=receiving&sort="+sort).Body.String())
		indexes := []int{
			strings.Index(body, "Keon Coleman"),
			strings.Index(body, "Zay Flowers"),
			strings.Index(body, "Khalil Shakir"),
		}
		for _, at := range indexes {
			require.GreaterOrEqual(t, at, 0, "every player card is on the tab")
		}
		return indexes
	}

	player := order("player")
	assert.Less(t, player[0], player[2], "the default sort is by player name")
	assert.Less(t, player[2], player[1])

	odds := order("odds")
	assert.Less(t, odds[1], odds[2], "the shortest price comes first")
	assert.Less(t, odds[2], odds[0])

	line := order("line")
	assert.Less(t, line[2], line[1], "the highest line comes first")
	assert.Less(t, line[1], line[0])
}

func TestPickDetailSearchStaysInsideTheOpenGame(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	bills := addGame(t, fixture, gameKickoff())
	addPropOdds(t, fixture.store, bills.ID, "PASS_YDS", "Josh Allen", 245.5, -110)
	bengals := addSecondGame(t, fixture)
	addPropOdds(t, fixture.store, bengals.ID, "PASS_YDS", "Joe Burrow", 255.5, -110)

	body := html.UnescapeString(pickDetail(t, fixture, "game="+bills.ID+"&q=pass").Body.String())
	assert.Contains(t, body, "Josh Allen", "the search matches the open game's rows")
	assert.NotContains(t, body, "Joe Burrow", "another game's rows stay out of the detail")

	empty := html.UnescapeString(pickDetail(t, fixture, "game="+bills.ID+"&q=burrow").Body.String())
	assert.Contains(t, empty, "Nothing matches that search.",
		"a player on another game does not leak into the open game")
}

func TestPickDetailFallsBackFromTeamTotalsAndUnknownTabs(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addTeamTotal(t, fixture.store, game.ID, "BUF", "OVER", 24.5)

	for _, query := range []string{
		"game=" + game.ID + "&cat=team_totals",
		"game=" + game.ID + "&cat=bogus",
	} {
		recorder := pickDetail(t, fixture, query)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		body := html.UnescapeString(recorder.Body.String())

		assert.Contains(t, body, `data-category="lines"`, query)
		assert.Contains(t, body, "BUF team total", query)
	}
}

func TestPickDetailShowsRemoveOnAPickedLine(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	leg := addStoredLeg(t, fixture, db.Leg{
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -3",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-3.0),
		Odds:       -110,
	})

	body := html.UnescapeString(pickDetail(t, fixture, "game="+game.ID).Body.String())
	assert.Contains(t, body, "tap to remove")
	assert.Contains(t, body, fmt.Sprintf(`hx-delete="/legs/%d?from=board"`, leg.ID))
}

func TestPickDetailSaysWhenTheGameHasNoLines(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	body := html.UnescapeString(pickDetail(t, fixture, "game="+game.ID).Body.String())
	assert.Contains(t, body, "No lines for this game yet.")
	assert.NotContains(t, body, "Nothing matches that search.")
}

func TestPickDetailSaysWhenTheSearchMatchesNothing(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	body := html.UnescapeString(pickDetail(t, fixture, "game="+game.ID+"&q=zzz").Body.String())
	assert.Contains(t, body, "Nothing matches that search.")
}

func TestPickDetailFallsBackToAPlaceholderWithoutAnOpenGame(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	body := html.UnescapeString(pickDetail(t, fixture, "game=missing").Body.String())
	assert.Contains(t, body, `id="pick-detail"`)
	assert.Contains(t, body, "Pick a game to see its lines and props.")
	assert.NotContains(t, body, "data-category=", "no game means no tab bar")
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

func TestPickBoardFiltersSlateBySlipSlateScope(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	now := time.Date(2026, 11, 4, 12, 0, 0, 0, loc) // Wednesday noon ET
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return now })

	// Insert Thursday, Sunday, and Monday games.
	thuGame := addSecondGameAt(
		t, fixture, "401873037", "BAL", "JAX", "Baltimore Ravens", "Jacksonville Jaguars",
		time.Date(2026, 11, 5, 20, 15, 0, 0, loc),
	)
	sunGame := addSecondGameAt(
		t, fixture, "401873042", "MIA", "DET", "Miami Dolphins", "Detroit Lions",
		time.Date(2026, 11, 8, 13, 0, 0, 0, loc),
	)
	monGame := addSecondGameAt(
		t, fixture, "401873051", "MIN", "BUF", "Minnesota Vikings", "Buffalo Bills",
		time.Date(2026, 11, 9, 20, 15, 0, 0, loc),
	)

	addGameOdds(t, fixture.store, thuGame.ID)
	addGameOdds(t, fixture.store, sunGame.ID)
	addGameOdds(t, fixture.store, monGame.ID)

	thuSlip, err := fixture.store.CreateSlip(
		t.Context(), fixture.group.ID, fixture.week.ID, nil, "Thursday Slip",
		db.SlateScopeThursday, 2, []int64{fixture.user.ID},
	)
	require.NoError(t, err)

	sunSlip, err := fixture.store.CreateSlip(
		t.Context(), fixture.group.ID, fixture.week.ID, nil, "Sunday Slip",
		db.SlateScopeSundayMain, 2, []int64{fixture.user.ID},
	)
	require.NoError(t, err)

	// 1. Thursday slip pick board includes only the Thursday game.
	recThu := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recThu, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", thuSlip.ID), nil,
	))
	require.Equal(t, http.StatusOK, recThu.Code)
	bodyThu := recThu.Body.String()
	assert.Contains(t, bodyThu, "BAL @ JAX")
	assert.NotContains(t, bodyThu, "MIA @ DET")
	assert.NotContains(t, bodyThu, "MIN @ BUF")
	assert.Contains(t, bodyThu, "win=thursday")
	assert.NotContains(t, bodyThu, "win=sun_early")
	assert.NotContains(t, bodyThu, "win=monday")

	// Attempting to open out-of-scope Sunday game falls back without opening it.
	recThuDetail := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recThuDetail, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick?game=%s", thuSlip.ID, sunGame.ID), nil,
	))
	require.Equal(t, http.StatusOK, recThuDetail.Code)
	bodyThuDetail := recThuDetail.Body.String()
	assert.NotContains(t, bodyThuDetail, "MIA @ DET")

	// Partial slate endpoint also respects slate scope.
	recThuSlate := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recThuSlate, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/slate", thuSlip.ID), nil,
	))
	require.Equal(t, http.StatusOK, recThuSlate.Code)
	bodyThuSlate := recThuSlate.Body.String()
	assert.Contains(t, bodyThuSlate, "BAL @ JAX")
	assert.NotContains(t, bodyThuSlate, "MIA @ DET")

	// 2. Sunday Main slip pick board includes Sunday and Monday games, but excludes Thursday.
	recSun := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recSun, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", sunSlip.ID), nil,
	))
	require.Equal(t, http.StatusOK, recSun.Code)
	bodySun := recSun.Body.String()
	assert.NotContains(t, bodySun, "BAL @ JAX")
	assert.Contains(t, bodySun, "MIA @ DET")
	assert.Contains(t, bodySun, "MIN @ BUF")
	assert.NotContains(t, bodySun, "win=thursday")
	assert.Contains(t, bodySun, "win=sun_early")
	assert.Contains(t, bodySun, "win=monday")

	// 3. Full Week slip (fixture.slip) includes all games.
	recFull := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recFull, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, recFull.Code)
	bodyFull := recFull.Body.String()
	assert.Contains(t, bodyFull, "BAL @ JAX")
	assert.Contains(t, bodyFull, "MIA @ DET")
	assert.Contains(t, bodyFull, "MIN @ BUF")
	assert.Contains(t, bodyFull, "win=thursday")
	assert.Contains(t, bodyFull, "win=sun_early")
	assert.Contains(t, bodyFull, "win=monday")
}

func TestPickBoardRendersLogosAndPortraits(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixtureAt(t, 3, func() time.Time {
		return gameKickoff().Add(-24 * time.Hour)
	})
	_, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772001",
		WeekID:     fixture.week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		HomeLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"),
		AwayLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"),
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	addSpreadOdds(t, fixture.store, "401772001", "BUF")
	addPropOdds(t, fixture.store, "401772001", "PASS_YDS", "Josh Allen", 255.5, -110)
	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{{
		GameID:     "401772001",
		Book:       "draftkings",
		MarketType: "PASS_YDS",
		Team:       "MIA",
		PlayerName: "Tua Tagovailoa",
		Side:       "OVER",
		Line:       new(245.5),
		Price:      -110,
		FetchedAt:  time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC),
	}})
	require.NoError(t, err)
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Josh Allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png",
			UpdatedAt:   time.Now().UTC(),
		},
		{
			AthleteID:   "4241479",
			Team:        "MIA",
			ESPNTeamID:  "15",
			FullName:    "Tua Tagovailoa",
			Position:    "QB",
			HeadshotURL: "",
			UpdatedAt:   time.Now().UTC(),
		},
	}))

	// 1. pick-slate renders matchup logos.
	slateRec := httptest.NewRecorder()
	fixture.handler.ServeHTTP(slateRec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/slate", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, slateRec.Code)
	slateBody := slateRec.Body.String()
	assert.Contains(t, slateBody, `src="https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"`)
	assert.Contains(t, slateBody, `src="https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"`)

	// 2. pick-detail header renders matchup logos and PickPlayerView renders
	//    player portrait + team logo, or initials fallback when headshot is empty.
	detailRec := pickBoard(t, fixture, "game=401772001&cat=passing")
	require.Equal(t, http.StatusOK, detailRec.Code)
	detailBody := detailRec.Body.String()
	assert.Contains(t, detailBody, `src="https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"`)
	assert.Contains(t, detailBody, `src="https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"`)
	assert.Contains(t, detailBody, `src="https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png"`)
	assert.Contains(t, detailBody, `data-fallback-badge`)
	assert.Contains(t, detailBody, `>JA</span>`)
	assert.Contains(t, detailBody, `>TT</span>`)
}

type pickSyncCall struct {
	weekID int64
	gameID string
}

type pickTestRefresher struct {
	linesCalls []int64
	gameCalls  []pickSyncCall
	forceCalls []pickSyncCall
}

func (r *pickTestRefresher) SyncGameProps(_ context.Context, _ string) error {
	return nil
}

func (r *pickTestRefresher) SyncGameResearch(_ context.Context, _ string) error {
	return nil
}

func (r *pickTestRefresher) SyncPlayerResearch(_ context.Context, _ string) error {
	return nil
}

func (r *pickTestRefresher) KickPickLines(_ context.Context, weekID int64) {
	r.linesCalls = append(r.linesCalls, weekID)
}

func (r *pickTestRefresher) KickPickGame(_ context.Context, weekID int64, gameID string) {
	r.gameCalls = append(r.gameCalls, pickSyncCall{weekID: weekID, gameID: gameID})
}

func (r *pickTestRefresher) ForcePickRefresh(_ context.Context, weekID int64, gameID string) error {
	r.forceCalls = append(r.forceCalls, pickSyncCall{weekID: weekID, gameID: gameID})
	return nil
}

func (r *pickTestRefresher) totalCalls() int {
	return len(r.linesCalls) + len(r.gameCalls) + len(r.forceCalls)
}

func pickHandlerWithGroupRefresher(
	t *testing.T,
	fixture *slipFixture,
	user db.User,
	group db.Group,
	refresher webui.Refresher,
	now func() time.Time,
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
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store:   fixture.store,
		Slips:   slips.New(fixture.store, hub, nil),
		Stream:  live.NewStream(hub, func(int64) bool { return false }),
		Refresh: refresher,
		Now:     now,
	})
}

func pickHandlerWithRefresher(
	t *testing.T,
	fixture *slipFixture,
	user db.User,
	refresher webui.Refresher,
	now func() time.Time,
) http.Handler {
	t.Helper()
	return pickHandlerWithGroupRefresher(t, fixture, user, fixture.group, refresher, now)
}

func TestPickBoardAndPartialsSyncRules(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	upcoming := addGame(t, fixture, gameKickoff())
	started := addSecondGameAt(
		t, fixture, "401772099", "CIN", "CLE", "Cincinnati Bengals", "Cleveland Browns",
		gameKickoff().Add(-2*time.Hour),
	)
	now := gameKickoff().Add(-1 * time.Hour)
	refresher := &pickTestRefresher{}
	handler := pickHandlerWithRefresher(t, fixture, fixture.user, refresher, func() time.Time { return now })

	refreshAction := fmt.Sprintf(`hx-post="/slips/%d/pick/refresh"`, fixture.slip.ID)

	// 1. Open slip, pickBoard with no game selected: calls SyncPickLines and renders Refresh button.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []int64{fixture.slip.WeekID}, refresher.linesCalls)
	assert.Empty(t, refresher.gameCalls)
	assert.Contains(t, rec.Body.String(), refreshAction)

	// 2. pickBoard with upcoming game selected: calls SyncPickGame and renders Refresh button in detail too.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick?game=%s", fixture.slip.ID, upcoming.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []pickSyncCall{{weekID: fixture.slip.WeekID, gameID: upcoming.ID}}, refresher.gameCalls)
	assert.Equal(t, 2, strings.Count(rec.Body.String(), refreshAction))

	// 3. pickResults without game selected: 0 extra sync calls.
	before := refresher.totalCalls()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/results?q=allen", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, before, refresher.totalCalls())

	// 4. pickResults with upcoming game selected: calls SyncPickGame.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/results?game=%s&q=allen", fixture.slip.ID, upcoming.ID),
		nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, refresher.gameCalls, 2)

	// 5. pickResults with kicked-off game selected: 0 extra sync calls.
	before = refresher.totalCalls()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/results?game=%s", fixture.slip.ID, started.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, before, refresher.totalCalls())

	// 6. pickDetail with upcoming game selected: calls SyncPickGame.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/detail?game=%s", fixture.slip.ID, upcoming.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, refresher.gameCalls, 3)
	assert.Contains(t, rec.Body.String(), refreshAction)

	// 7. pickSlate is strictly read-only.
	before = refresher.totalCalls()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/slate", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, before, refresher.totalCalls())

	// 8. Locked slip: 0 sync calls across pickBoard, pickResults, pickDetail, and no Refresh button.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
	before = refresher.totalCalls()
	for _, target := range []string{
		fmt.Sprintf("/slips/%d/pick", fixture.slip.ID),
		fmt.Sprintf("/slips/%d/pick/results?game=%s", fixture.slip.ID, upcoming.ID),
		fmt.Sprintf("/slips/%d/pick/detail?game=%s", fixture.slip.ID, upcoming.ID),
	} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.NotContains(t, rec.Body.String(), refreshAction)
	}
	assert.Equal(t, before, refresher.totalCalls())

	// 9. Open slip with all games kicked off: 0 sync calls and no Refresh button.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipOpen))
	now = gameKickoff().Add(time.Minute)
	before = refresher.totalCalls()
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, before, refresher.totalCalls())
	assert.NotContains(t, rec.Body.String(), refreshAction)
}

func TestPickRefreshHandlerAndGuards(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	upcoming := addGame(t, fixture, gameKickoff())
	now := gameKickoff().Add(-1 * time.Hour)
	refresher := &pickTestRefresher{}
	handler := pickHandlerWithRefresher(t, fixture, fixture.user, refresher, func() time.Time { return now })

	// 1. Open slip with no game selected: ForcePickRefresh(weekID, "") and returns #pick-body.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID), nil,
	)
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []pickSyncCall{{weekID: fixture.slip.WeekID, gameID: ""}}, refresher.forceCalls)
	assert.Contains(t, rec.Body.String(), `id="pick-body"`)

	// 2. Open slip with game form param: ForcePickRefresh(weekID, gameID).
	form := url.Values{"game": {upcoming.ID}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID),
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []pickSyncCall{
		{weekID: fixture.slip.WeekID, gameID: ""},
		{weekID: fixture.slip.WeekID, gameID: upcoming.ID},
	}, refresher.forceCalls)
	assert.Contains(t, rec.Body.String(), `id="pick-detail"`)

	// 3. Non-member of the slip's group gets 404 and triggers 0 refreshes.
	outsider, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Outsider", "outsider@example.com", "sub-outsider", "",
	)
	require.NoError(t, err)
	otherGroup := db.Group{ID: fixture.group.ID + 1, Name: "Other Group"}
	outsiderHandler := pickHandlerWithGroupRefresher(
		t, fixture, outsider, otherGroup, refresher, func() time.Time { return now },
	)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID), nil,
	)
	outsiderHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Len(t, refresher.forceCalls, 2)

	// 4. Locked slip returns 422 and triggers 0 refreshes.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID), nil,
	)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Len(t, refresher.forceCalls, 2)

	// 5. Open slip with all games kicked off: 0 ForcePickRefresh calls.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipOpen))
	now = gameKickoff().Add(time.Minute)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID), nil,
	)
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, refresher.forceCalls, 2)
}

type fakePickLinesClient struct {
	calls atomic.Int32
	event odds.Event
	lines []odds.Line
}

func (c *fakePickLinesClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return []odds.Event{c.event}, odds.Credits{Remaining: 400}, nil
}

func (c *fakePickLinesClient) Lines(context.Context) ([]odds.Line, odds.Credits, error) {
	c.calls.Add(1)
	return c.lines, odds.Credits{Remaining: 397}, nil
}

type fakePickPropsClient struct {
	calls atomic.Int32
	event odds.Event
	props []odds.Prop
}

func (c *fakePickPropsClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return []odds.Event{c.event}, odds.Credits{Remaining: 800}, nil
}

func (c *fakePickPropsClient) Props(context.Context, string) ([]odds.Prop, odds.Credits, error) {
	c.calls.Add(1)
	return c.props, odds.Credits{Remaining: 797}, nil
}

func TestPickRefreshCooldownAndRepeatedSearchWithService(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	var clockMu sync.Mutex
	clockNow := gameKickoff().Add(-1 * time.Hour)
	now := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return clockNow
	}

	event := odds.Event{
		ID:           "evt-pick-1",
		HomeTeam:     game.HomeName,
		AwayTeam:     game.AwayName,
		CommenceTime: game.Kickoff,
	}
	linesClient := &fakePickLinesClient{
		event: event,
		lines: []odds.Line{{
			EventID:    event.ID,
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       game.AwayName,
			Line:       new(-2.5),
			Price:      -110,
		}},
	}
	propsClient := &fakePickPropsClient{
		event: event,
		props: []odds.Prop{{
			EventID:    event.ID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Josh Allen",
			Side:       odds.SideOver,
			Line:       new(265.5),
			Price:      -115,
		}},
	}
	svc := refresh.New(refresh.Deps{
		Store:  fixture.store,
		Lines:  linesClient,
		Props:  propsClient,
		Now:    now,
		Logger: zerolog.Nop(),
	})
	t.Cleanup(func() {
		assert.NoError(t, svc.Shutdown(context.Background()))
	})
	handler := pickHandlerWithRefresher(t, fixture, fixture.user, svc, now)

	// 1. Opening pickDetail/pickResults for a game triggers an async refresh once.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick/detail?game=%s", fixture.slip.ID, game.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Eventually(t, func() bool {
		return linesClient.calls.Load() == 1 && propsClient.calls.Load() == 1
	}, 2*time.Second, 5*time.Millisecond)

	// 2. Repeated search queries on that game spend 0 extra API calls.
	for _, q := range []string{"j", "jo", "jos", "josh"} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d/pick/results?game=%s&q=%s", fixture.slip.ID, game.ID, q),
			nil,
		))
		require.Equal(t, http.StatusOK, rec.Code)
	}
	require.NoError(t, svc.Shutdown(t.Context()))
	assert.Equal(t, int32(1), linesClient.calls.Load())
	assert.Equal(t, int32(1), propsClient.calls.Load())

	// 3. Immediate POST /pick/refresh within 2m cooldown is blocked (0 extra calls).
	form := url.Values{"game": {game.ID}}
	rec = httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID),
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(1), linesClient.calls.Load())
	assert.Equal(t, int32(1), propsClient.calls.Load())

	// 4. After 2m cooldown expires, POST /pick/refresh refreshes both lines and props.
	clockMu.Lock()
	clockNow = clockNow.Add(2 * time.Minute)
	clockMu.Unlock()
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, fmt.Sprintf("/slips/%d/pick/refresh", fixture.slip.ID),
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(2), linesClient.calls.Load())
	assert.Equal(t, int32(2), propsClient.calls.Load())
}

func TestPickBoardWiresSignalsBadgesSortAndHTMXSwaps(t *testing.T) {
	t.Parallel()

	seedBoardSignals := func(t *testing.T, fixture *slipFixture, syncedAt time.Time) db.Game {
		t.Helper()
		injuries := `[{"team":"Buffalo Bills","abbreviation":"BUF","injuries":[{"athlete_id":"999","full_name":"Curtis Samuel","position":"WR","status":"Out"}]}]`
		game := addGame(t, fixture, gameKickoff())
		game.InjuriesJSON = &injuries
		_, err := fixture.store.UpsertGame(t.Context(), game)
		require.NoError(t, err)

		require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
			{AthleteID: "wr-smash", FullName: "Khalil Shakir", Team: "BUF", Position: "WR", UpdatedAt: syncedAt},
			{AthleteID: "wr-plain", FullName: "Keon Coleman", Team: "BUF", Position: "WR", UpdatedAt: syncedAt},
		}))

		// Seed 32 teams so MIA (the away opponent in addGame) sits in the bottom 6 pass defenses (rank >= 27).
		teams := []string{
			"BUF", "BAL", "SF", "DET", "PHI", "MIN", "GB", "HOU",
			"CIN", "LAC", "DEN", "PIT", "TB", "ATL", "LAR", "SEA",
			"WAS", "ARI", "CHI", "IND", "DAL", "KC", "NYJ", "CLE",
			"NO", "LV", "TEN", "NYG", "NE", "CAR", "JAX", "MIA",
		}
		teamWeeks := make([]db.NFLverseTeamWeek, 0, len(teams))
		for i, abbr := range teams {
			teamWeeks = append(teamWeeks, db.NFLverseTeamWeek{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              abbr,
				OffPassEPA:        0.10,
				DefPassEPAAllowed: float64(i) * 0.02, // KC is highest (rank 32)
				DefRushEPAAllowed: 0.0,
				PlaysPerGame:      62,
				PassRate:          0.56,
				UpdatedAt:         syncedAt,
			})
		}
		_, err = fixture.store.UpsertNFLverseTeamWeeks(t.Context(), teamWeeks)
		require.NoError(t, err)
		require.NoError(t, fixture.store.RecordSync(t.Context(), "nflverse:season:2026", syncedAt, nil))

		var playerWeeks []db.NFLversePlayerWeek
		for wk := 1; wk <= 3; wk++ {
			playerWeeks = append(playerWeeks, db.NFLversePlayerWeek{
				Season:        2026,
				SeasonType:    2,
				WeekNum:       wk,
				ESPNID:        "wr-smash",
				Team:          "BUF",
				OffensePct:    0.85,
				TargetShare:   0.26,
				AirYardsShare: 0.32,
				WOPR:          0.62,
				Targets:       9,
				UpdatedAt:     syncedAt,
			})
		}
		_, err = fixture.store.UpsertNFLversePlayerWeeks(t.Context(), playerWeeks)
		require.NoError(t, err)

		_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
			{
				GameID: game.ID, Book: "fanduel", MarketType: "REC_YDS", Team: "BUF",
				PlayerName: "Khalil Shakir", Side: "OVER", Line: new(62.5), Price: -110, FetchedAt: syncedAt,
			},
			{
				GameID: game.ID, Book: "fanduel", MarketType: "REC_YDS", Team: "BUF",
				PlayerName: "Khalil Shakir", Side: "UNDER", Line: new(62.5), Price: -110, FetchedAt: syncedAt,
			},
			{
				GameID: game.ID, Book: "fanduel", MarketType: "REC_YDS", Team: "BUF",
				PlayerName: "Keon Coleman", Side: "OVER", Line: new(44.5), Price: -110, FetchedAt: syncedAt,
			},
			{
				GameID: game.ID, Book: "fanduel", MarketType: "REC_YDS", Team: "BUF",
				PlayerName: "Keon Coleman", Side: "UNDER", Line: new(44.5), Price: -110, FetchedAt: syncedAt,
			},
		})
		require.NoError(t, err)
		return game
	}

	t.Run("fresh signals render badges, Matchup sort, mobile wrap, and HTMX line swap", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 4)
		now := gameKickoff().Add(-2 * time.Hour)
		game := seedBoardSignals(t, fixture, now)
		handler := pickHandlerWithRefresher(
			t, fixture, fixture.user, &pickTestRefresher{}, func() time.Time { return now },
		)

		// Add Josh Allen PASS_YDS OVER so Khalil Shakir also qualifies for Fits slip.
		_, err := fixture.store.CreateLeg(t.Context(), db.Leg{
			SlipID:     fixture.slip.ID,
			UserID:     fixture.user.ID,
			GameID:     new(game.ID),
			MarketType: "PASS_YDS",
			Team:       new("BUF"),
			PlayerName: new("Josh Allen"),
			PlayerID:   new("qb-allen"),
			Side:       new("OVER"),
			Line:       new(255.5),
			Odds:       -110,
			Selection:  "Josh Allen Over 255.5 pass yds",
		})
		require.NoError(t, err)

		// 1. HTMX detail request with sort=matchup renders badges & Matchup chip.
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d/pick/detail?game=%s&cat=receiving&sort=matchup", fixture.slip.ID, game.ID),
			nil,
		)
		req.Header.Set("Hx-Request", "true")
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := html.UnescapeString(rec.Body.String())
		assert.Contains(t, body, `id="pick-detail"`)
		assert.Contains(t, body, `data-sort="matchup"`)
		assert.Contains(t, body, `Matchup`)
		assert.Contains(t, body, `Smash matchup`)
		assert.Contains(t, body, `Fits slip`)
		// Mobile-width layout guards: flex-wrap for tags/sorts and 2-column grid for options.
		assert.Contains(t, body, `flex-wrap gap-1.5`)
		assert.Contains(t, body, `grid-cols-2`)
		// Under SortMatchup, Khalil Shakir (Smash matchup) ranks before Keon Coleman.
		assert.Less(t, strings.Index(body, "Khalil Shakir"), strings.Index(body, "Keon Coleman"))

		// 2. HTMX POST /slips/{id}/legs from board returns pick-line-group with badge preserved.
		form := url.Values{
			"quick":       {"1"},
			"from":        {"board"},
			"game_id":     {game.ID},
			"market_type": {"REC_YDS"},
			"team":        {"BUF"},
			"player_name": {"Khalil Shakir"},
			"player_id":   {"wr-smash"},
			"side":        {"OVER"},
			"line":        {"62.5"},
			"odds":        {"-110"},
		}
		postRec := httptest.NewRecorder()
		postReq := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
			strings.NewReader(form.Encode()),
		)
		postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		postReq.Header.Set("Hx-Request", "true")
		handler.ServeHTTP(postRec, postReq)
		require.Equal(t, http.StatusOK, postRec.Code)

		swapBody := html.UnescapeString(postRec.Body.String())
		assert.Contains(t, swapBody, `class="pick-line-group"`)
		assert.Contains(t, swapBody, `tap to remove`)
		assert.Contains(t, swapBody, `Smash matchup`)
	})

	t.Run("stale nflverse fails soft without nflverse badges", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 4)
		stale := time.Now().UTC().Add(-8 * 24 * time.Hour)
		game := seedBoardSignals(t, fixture, stale)

		rec := pickDetail(t, fixture, fmt.Sprintf("game=%s&cat=receiving&sort=matchup", game.ID))
		require.Equal(t, http.StatusOK, rec.Code)

		body := html.UnescapeString(rec.Body.String())
		assert.Contains(t, body, "Khalil Shakir")
		assert.NotContains(t, body, "Smash matchup")
		assert.NotContains(t, body, "Role bump")
	})

	t.Run("friend leg on open slip triggers Fits slip", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 4)
		now := gameKickoff().Add(-2 * time.Hour)
		game := seedBoardSignals(t, fixture, now)
		handler := pickHandlerWithRefresher(
			t, fixture, fixture.user, &pickTestRefresher{}, func() time.Time { return now },
		)

		friend, err := fixture.store.CreateUserWithIdentity(
			t.Context(), "Friend", "friend@example.com", "google-friend", "",
		)
		require.NoError(t, err)
		_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
			SlipID:     fixture.slip.ID,
			UserID:     friend.ID,
			GameID:     new(game.ID),
			MarketType: "PASS_YDS",
			Team:       new("BUF"),
			PlayerName: new("Josh Allen"),
			PlayerID:   new("qb-allen"),
			Side:       new("OVER"),
			Line:       new(255.5),
			Odds:       -110,
			Selection:  "Josh Allen Over 255.5 pass yds",
		})
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d/pick/detail?game=%s&cat=receiving&sort=matchup", fixture.slip.ID, game.ID),
			nil,
		)
		req.Header.Set("Hx-Request", "true")
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := html.UnescapeString(rec.Body.String())
		assert.Contains(t, body, `Fits slip`)
	})
}

func TestPickSlateSkipsDockAndDetail(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	addPropOdds(t, fixture.store, game.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	_, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(245.5),
		Odds:       -110,
		Selection:  "Josh Allen Over 245.5 pass yds",
	})
	require.NoError(t, err)

	reqOpen := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/slate?game=%s&cat=passing&sort=line", fixture.slip.ID, game.ID),
		nil,
	)
	recOpen := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recOpen, reqOpen)
	require.Equal(t, http.StatusOK, recOpen.Code, recOpen.Body.String())

	openBody := html.UnescapeString(recOpen.Body.String())
	assert.Contains(t, openBody, `id="pick-slate"`)
	assert.Contains(t, openBody, `hidden lg:block`)
	assert.Contains(t, openBody, `id="pick-cat" type="hidden" name="cat" value="passing"`)
	assert.Contains(t, openBody, `id="pick-sort" type="hidden" name="sort" value="line"`)
	assert.Contains(t, openBody, "MIA @ BUF")
	assert.NotContains(t, openBody, `id="pick-detail"`)
	assert.NotContains(t, openBody, `id="pick-dock"`)

	reqClosed := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/slate", fixture.slip.ID),
		nil,
	)
	recClosed := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recClosed, reqClosed)
	require.Equal(t, http.StatusOK, recClosed.Code, recClosed.Body.String())

	closedBody := html.UnescapeString(recClosed.Body.String())
	assert.Contains(t, closedBody, `id="pick-slate"`)
	assert.NotContains(t, closedBody, `hidden lg:block`)
	assert.Contains(t, closedBody, "MIA @ BUF")
}

func TestPickDetailScopesToOpenGameAndFiltersScope(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game1 := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game1.ID)
	addPropOdds(t, fixture.store, game1.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	game2, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:       "game-kc-cin",
		WeekID:   fixture.week.ID,
		HomeTeam: "KC",
		AwayTeam: "CIN",
		Kickoff:  gameKickoff().Add(3 * time.Hour),
		Status:   "SCHEDULED",
	})
	require.NoError(t, err)
	addGameOdds(t, fixture.store, game2.ID)
	addPropOdds(t, fixture.store, game2.ID, "PASS_YDS", "Patrick Mahomes", 275.5, -110)

	rec := pickDetail(t, fixture, "game="+game1.ID+"&cat=passing")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := html.UnescapeString(rec.Body.String())
	assert.Contains(t, body, "Josh Allen")
	assert.NotContains(t, body, "Patrick Mahomes")

	// A kicked-off game falls back to the empty detail placeholder.
	started, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:       "game-started",
		WeekID:   fixture.week.ID,
		HomeTeam: "DET",
		AwayTeam: "GB",
		Kickoff:  time.Now().UTC().Add(-1 * time.Hour),
		Status:   "SCHEDULED",
	})
	require.NoError(t, err)
	addGameOdds(t, fixture.store, started.ID)

	recStarted := pickDetail(t, fixture, "game="+started.ID)
	require.Equal(t, http.StatusOK, recStarted.Code, recStarted.Body.String())
	assert.Contains(t, html.UnescapeString(recStarted.Body.String()), "Pick a game to see its lines and props.")

	// A game from another week also falls back to the empty detail placeholder.
	otherWeek, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 11)
	require.NoError(t, err)
	otherGame, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:       "game-other-week",
		WeekID:   otherWeek.ID,
		HomeTeam: "DAL",
		AwayTeam: "PHI",
		Kickoff:  gameKickoff(),
		Status:   "SCHEDULED",
	})
	require.NoError(t, err)

	recOther := pickDetail(t, fixture, "game="+otherGame.ID)
	require.Equal(t, http.StatusOK, recOther.Code, recOther.Body.String())
	assert.Contains(t, html.UnescapeString(recOther.Body.String()), "Pick a game to see its lines and props.")
}

func TestPickPageDataSingleSlatesPassWhenSearchEmpty(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game1 := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game1.ID)
	addPropOdds(t, fixture.store, game1.ID, "PASS_YDS", "Josh Allen", 245.5, -110)

	game2, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:       "game-kc-cin",
		WeekID:   fixture.week.ID,
		HomeTeam: "KC",
		AwayTeam: "CIN",
		Kickoff:  gameKickoff().Add(3 * time.Hour),
		Status:   "SCHEDULED",
	})
	require.NoError(t, err)
	addGameOdds(t, fixture.store, game2.ID)
	addPropOdds(t, fixture.store, game2.ID, "PASS_YDS", "Patrick Mahomes", 275.5, -110)
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{{
		Team:     "KC",
		FullName: "Patrick Mahomes",
		NormName: "patrick mahomes",
		Position: "QB",
	}}))

	// Empty search: both games appear on the slate and game1 detail is open.
	recEmpty := pickBoard(t, fixture, "game="+game1.ID+"&cat=passing")
	require.Equal(t, http.StatusOK, recEmpty.Code, recEmpty.Body.String())
	emptyBody := html.UnescapeString(recEmpty.Body.String())
	assert.Contains(t, emptyBody, "MIA @ BUF")
	assert.Contains(t, emptyBody, "CIN @ KC")
	assert.Contains(t, emptyBody, "Josh Allen")

	// Non-empty search: slate filters to game2, while game1's detail stays open.
	reqSearch := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/results?game=%s&q=Patrick+Mahomes", fixture.slip.ID, game1.ID),
		nil,
	)
	recSearch := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recSearch, reqSearch)
	require.Equal(t, http.StatusOK, recSearch.Code, recSearch.Body.String())
	searchBody := html.UnescapeString(recSearch.Body.String())
	slateHTML, detailHTML, foundDetail := strings.Cut(searchBody, `id="pick-detail"`)
	require.True(t, foundDetail)
	assert.Contains(t, slateHTML, "CIN @ KC")
	assert.NotContains(t, slateHTML, "MIA @ BUF")
	assert.Contains(t, detailHTML, "Nothing matches that search.")
}

type blockingPickLinesClient struct {
	event   odds.Event
	lines   []odds.Line
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingPickLinesClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return []odds.Event{c.event}, odds.Credits{Remaining: 400}, nil
}

func (c *blockingPickLinesClient) Lines(ctx context.Context) ([]odds.Line, odds.Credits, error) {
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.release:
	case <-ctx.Done():
		return nil, odds.Credits{}, ctx.Err()
	}
	return c.lines, odds.Credits{Remaining: 397}, nil
}

type blockingPickPropsClient struct {
	event   odds.Event
	props   []odds.Prop
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingPickPropsClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return []odds.Event{c.event}, odds.Credits{Remaining: 800}, nil
}

func (c *blockingPickPropsClient) Props(ctx context.Context, _ string) ([]odds.Prop, odds.Credits, error) {
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.release:
	case <-ctx.Done():
		return nil, odds.Credits{}, ctx.Err()
	}
	return c.props, odds.Credits{Remaining: 797}, nil
}

func TestPickBoardAndDetailReturnCachedRowsWhileAsyncRefreshPublishes(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	clockNow := gameKickoff().Add(-1 * time.Hour)
	now := func() time.Time { return clockNow }

	// Seed cached spread and prop rows before the stale refresh runs.
	require.NoError(t, fixture.store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.UpsertOdds(t.Context(), []db.Odds{
			{
				GameID:     game.ID,
				Book:       "draftkings",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -110,
				FetchedAt:  clockNow.Add(-2 * time.Hour),
			},
			{
				GameID:     game.ID,
				Book:       "draftkings",
				MarketType: "PASS_YDS",
				PlayerName: "Josh Allen",
				Side:       "OVER",
				Line:       new(265.5),
				Price:      -115,
				FetchedAt:  clockNow.Add(-2 * time.Hour),
			},
		})
		return err
	}))

	event := odds.Event{
		ID:           "evt-async-1",
		HomeTeam:     game.HomeName,
		AwayTeam:     game.AwayName,
		CommenceTime: game.Kickoff,
	}
	linesClient := &blockingPickLinesClient{
		event: event,
		lines: []odds.Line{{
			EventID:    event.ID,
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       game.HomeName,
			Line:       new(-4.5),
			Price:      -110,
		}},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	propsClient := &blockingPickPropsClient{
		event: event,
		props: []odds.Prop{{
			EventID:    event.ID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Josh Allen",
			Side:       odds.SideOver,
			Line:       new(275.5),
			Price:      -115,
		}},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	hub := &live.Hub{}
	slipCh, unsubSlip := hub.Subscribe(fmt.Sprintf("slip:%d", fixture.slip.ID))
	defer unsubSlip()
	weekCh, unsubWeek := hub.Subscribe(fmt.Sprintf("week:%d", fixture.slip.WeekID))
	defer unsubWeek()

	svc := refresh.New(refresh.Deps{
		Store:     fixture.store,
		Lines:     linesClient,
		Props:     propsClient,
		Publisher: hub,
		Now:       now,
		Logger:    zerolog.Nop(),
	})
	t.Cleanup(func() {
		assert.NoError(t, svc.Shutdown(context.Background()))
	})

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)
	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return fixture.user, true
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
		Refresh: svc,
		Now:     now,
	})

	// 1. pickBoard returns cached rows immediately while lines fetch is blocked.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "-3.5")

	<-linesClient.started
	close(linesClient.release)

	select {
	case <-slipCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slip board-updated signal after async lines refresh")
	}
	select {
	case <-weekCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for week board-updated signal after async lines refresh")
	}

	// 2. pickDetail returns cached props immediately while props fetch is blocked.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/detail?game=%s&cat=passing", fixture.slip.ID, game.ID),
		nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "265.5")

	<-propsClient.started
	close(propsClient.release)

	select {
	case <-slipCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slip board-updated signal after async props refresh")
	}
	select {
	case <-weekCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for week board-updated signal after async props refresh")
	}

	// 3. Subsequent pickDetail sees the updated prop line persisted by the async refresh.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/pick/detail?game=%s&cat=passing", fixture.slip.ID, game.ID),
		nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "275.5")
}

func newProdScalePickFixture(b *testing.B) (*slipFixture, string) {
	b.Helper()

	database, err := db.Open(filepath.Join(b.TempDir(), "parlay.db"))
	require.NoError(b, err)
	require.NoError(b, db.Migrate(b.Context(), database))

	store := db.NewStore(database)
	b.Cleanup(func() {
		_ = store.Close()
		_ = database.Close()
	})

	user, err := store.CreateUserWithIdentity(b.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(b, err)
	group, err := store.Create(b.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(b, err)
	week, err := store.UpsertWeek(b.Context(), 2026, 2, 9)
	require.NoError(b, err)
	slip, err := store.CreateSlip(
		b.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeFullWeek,
		2,
		[]int64{user.ID},
	)
	require.NoError(b, err)

	kickoff := gameKickoff()
	const (
		numGames       = 14
		playersPerTeam = 75
		totalOdds      = 15500
	)

	teams := make([]string, numGames*2)
	for i := range teams {
		teams[i] = fmt.Sprintf("T%02d", i)
	}

	positions := []string{"QB", "RB", "WR", "TE", "K"}
	roster := make([]db.RosterPlayer, 0, len(teams)*playersPerTeam)
	for tIdx, team := range teams {
		for p := range playersPerTeam {
			id := strconv.Itoa(tIdx*100 + p + 1)
			fullName := fmt.Sprintf("Player %s %02d Jr.", team, p)
			roster = append(roster, db.RosterPlayer{
				AthleteID:   id,
				Team:        team,
				FullName:    fullName,
				NormName:    db.NormalizePlayerName(fullName),
				Position:    positions[p%len(positions)],
				HeadshotURL: "https://a.espncdn.com/" + id + ".png",
				UpdatedAt:   kickoff,
			})
		}
	}
	require.NoError(b, store.UpsertRosterPlayers(b.Context(), roster))

	fetchedAt := kickoff.Add(-2 * time.Hour)
	books := []string{"draftkings", "fanduel"}
	markets := []string{"PASS_YDS", "RUSH_YDS", "REC_YDS", "RECEPTIONS", "ANYTIME_TD", "PASS_TDS"}
	perGame := totalOdds / numGames
	remainder := totalOdds % numGames

	var firstGameID string
	for i := range numGames {
		away := teams[i*2]
		home := teams[i*2+1]
		game, err := store.UpsertGame(b.Context(), db.Game{
			ID:         fmt.Sprintf("40167%04d", i),
			WeekID:     week.ID,
			HomeTeam:   home,
			AwayTeam:   away,
			HomeName:   "Home " + home,
			AwayName:   "Away " + away,
			HomeESPNID: strconv.Itoa(i*2 + 2),
			AwayESPNID: strconv.Itoa(i*2 + 1),
			Kickoff:    kickoff.Add(time.Duration(i%3) * 3 * time.Hour),
			Status:     db.GameScheduled,
			VenueID:    new("1"),
			VenueName:  new("A stadium"),
			UpdatedAt:  kickoff,
		})
		require.NoError(b, err)
		if i == 0 {
			firstGameID = game.ID
		}

		want := perGame
		if i < remainder {
			want++
		}
		gameOdds := make([]db.Odds, 0, want)
		for _, book := range books {
			gameOdds = append(gameOdds,
				db.Odds{
					GameID: game.ID, Book: book, MarketType: "ML", Team: away,
					Side: "TEAM", Price: 120, FetchedAt: fetchedAt,
				},
				db.Odds{
					GameID: game.ID, Book: book, MarketType: "ML", Team: home,
					Side: "TEAM", Price: -140, FetchedAt: fetchedAt,
				},
				db.Odds{
					GameID: game.ID, Book: book, MarketType: "SPREAD", Team: away,
					Side: "TEAM", Line: new(2.5), Price: -110, FetchedAt: fetchedAt,
				},
				db.Odds{
					GameID: game.ID, Book: book, MarketType: "SPREAD", Team: home,
					Side: "TEAM", Line: new(-2.5), Price: -110, FetchedAt: fetchedAt,
				},
				db.Odds{
					GameID: game.ID, Book: book, MarketType: "TOTAL", Team: "",
					Side: "OVER", Line: new(45.5), Price: -110, FetchedAt: fetchedAt,
				},
				db.Odds{
					GameID: game.ID, Book: book, MarketType: "TOTAL", Team: "",
					Side: "UNDER", Line: new(45.5), Price: -110, FetchedAt: fetchedAt,
				},
			)
		}
	fillGameOdds:
		for p := range playersPerTeam {
			team := away
			if p%2 == 1 {
				team = home
			}
			player := fmt.Sprintf("Player %s %02d Jr.", team, p)
			for _, market := range markets {
				for _, book := range books {
					if market == "ANYTIME_TD" {
						gameOdds = append(gameOdds, db.Odds{
							GameID: game.ID, Book: book, MarketType: market, Team: team,
							PlayerName: player, Side: "YES", Price: 150, FetchedAt: fetchedAt,
						})
					} else {
						line := new(45.5)
						gameOdds = append(gameOdds,
							db.Odds{
								GameID: game.ID, Book: book, MarketType: market, Team: team,
								PlayerName: player, Side: "OVER", Line: line, Price: -110, FetchedAt: fetchedAt,
							},
							db.Odds{
								GameID: game.ID, Book: book, MarketType: market, Team: team,
								PlayerName: player, Side: "UNDER", Line: line, Price: -110, FetchedAt: fetchedAt,
							},
						)
					}
					if len(gameOdds) >= want {
						gameOdds = gameOdds[:want]
						break fillGameOdds
					}
				}
			}
		}
		_, err = store.UpsertOdds(b.Context(), gameOdds)
		require.NoError(b, err)
	}

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(b, err)

	hub := &live.Hub{}
	service := slips.New(store, hub, nil)
	stream := live.NewStream(hub, func(int64) bool { return false })

	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store:  store,
		Slips:  service,
		Stream: stream,
		Now:    func() time.Time { return kickoff.Add(-24 * time.Hour) },
	})

	return &slipFixture{
		handler: handler,
		store:   store,
		hub:     hub,
		user:    user,
		group:   group,
		week:    week,
		slip:    slip,
	}, firstGameID
}

func benchmarkPickRequest(b *testing.B, fixture *slipFixture, target string) {
	b.Helper()

	b.ReportAllocs()
	for b.Loop() {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, target, nil)
		fixture.handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			b.Fatalf("GET %s: status %d: %s", target, recorder.Code, recorder.Body.String())
		}
	}
}

func BenchmarkPickBoard(b *testing.B) {
	fixture, _ := newProdScalePickFixture(b)
	benchmarkPickRequest(b, fixture, fmt.Sprintf("/slips/%d/pick", fixture.slip.ID))
}

func BenchmarkPickDetail(b *testing.B) {
	fixture, gameID := newProdScalePickFixture(b)
	benchmarkPickRequest(b, fixture, fmt.Sprintf("/slips/%d/pick/detail?game=%s", fixture.slip.ID, gameID))
}

func BenchmarkPickSlate(b *testing.B) {
	fixture, _ := newProdScalePickFixture(b)
	benchmarkPickRequest(b, fixture, fmt.Sprintf("/slips/%d/pick/slate", fixture.slip.ID))
}
