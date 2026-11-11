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

	return addSecondGameAt(t, fixture, id, away, home, awayName, homeName, gameKickoff().Add(24*time.Hour))
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

func TestPickBoardKeepsEachGamesProps(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	first := addGame(t, fixture, gameKickoff())
	second := addSecondGame(t, fixture, "401772099", "CIN", "CLE", "Cincinnati Bengals", "Cleveland Browns")
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

	body := html.UnescapeString(pickBoard(t, fixture, "q=pass").Body.String())
	assert.Contains(t, body, "Josh Allen Over 245.5 Pass Yds",
		"a newer prop sync for one game leaves another game's props on the board")
	assert.Contains(t, body, "Joe Burrow Over 255.5 Pass Yds")
	assert.NotContains(t, body, "aren’t loaded yet")
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

	swapSheet := url.Values{
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
	results := html.UnescapeString(recorder.Body.String())
	assert.Contains(t, results, "MIA ML", "the swapped-in pick's row comes back")
	assert.Contains(t, results, "tap to remove", "the swapped-in row is now a picked row")

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
