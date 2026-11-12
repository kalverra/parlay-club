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
