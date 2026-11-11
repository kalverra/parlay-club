package web_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// slipFixture wires the slip handlers the way the server does, with stubs for
// the signed-in person and the active group.
type slipFixture struct {
	handler http.Handler
	store   *db.Store
	hub     *live.Hub
	user    db.User
	group   db.Group
	week    db.Week
	slip    db.Slip
}

func newSlipFixture(t *testing.T, legsPerMember int) *slipFixture {
	t.Helper()

	return newSlipFixtureAt(t, legsPerMember, nil)
}

// newSlipFixtureAt is newSlipFixture with an injected app clock.
func newSlipFixtureAt(t *testing.T, legsPerMember int, now func() time.Time) *slipFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(t.Context(), group.ID, week.ID, "Week 9", legsPerMember, []int64{user.ID})
	require.NoError(t, err)

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

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
		Now:    now,
	})

	return &slipFixture{
		handler: handler,
		store:   store,
		hub:     hub,
		user:    user,
		group:   group,
		week:    week,
		slip:    slip,
	}
}

func TestTwoSubscribersHearAnAddedLeg(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	server := httptest.NewServer(fixture.handler)
	t.Cleanup(server.Close)

	eventsURL := fmt.Sprintf("%s/slips/%d/events", server.URL, fixture.slip.ID)
	streams := make([]*http.Response, 2)
	for i := range streams {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, eventsURL, nil)
		require.NoError(t, err)
		response, err := server.Client().Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		streams[i] = response
		t.Cleanup(func() { assert.NoError(t, response.Body.Close()) })
	}

	form := url.Values{
		"market_type": {"ML"},
		"selection":   {"BUF ML"},
		"team":        {"BUF"},
		"odds":        {"-110"},
	}
	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("%s/slips/%d/legs", server.URL, fixture.slip.ID),
		strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")

	response, err := server.Client().Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode, "add leg response: %s", string(body))

	for i, stream := range streams {
		assert.Equal(t, "slip-updated", waitForEvent(t, stream.Body), "subscriber %d", i+1)
	}
}

func TestAddLegPastTheLimitAnswers422(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)

	first := addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	second := addLeg(t, fixture.handler, fixture.slip.ID, "MIA ML")
	assert.Equal(t, http.StatusUnprocessableEntity, second.Code)
	assert.Contains(t, second.Body.String(), "you've already added all your legs")
}

func TestProfilePageRendersTheHomeBookForm(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `<h1 class="text-xl font-semibold">Profile</h1>`)
	assert.Contains(t, body, `name="home_book"`)
	assert.Contains(t, body, fixture.user.Name)
}

func TestSlipPageAdvertisesTheLiveStream(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), fmt.Sprintf(`data-live-url="/slips/%d/events"`, fixture.slip.ID))
}

func TestSlipPageShowsTheSlip(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	response := addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "0/1 locked in")
	assert.Contains(t, body, "BUF ML")
	assert.Contains(t, body, "-110")
	assert.Contains(t, body, "PENDING")
	assert.Contains(t, body, "Reference", "a leg with no odds rows falls back to its own price")
	assert.Contains(t, body, "1.91x")
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/legs/new"`, fixture.slip.ID))
	assert.Contains(t, body, `id="leg-sheet"`)
	assert.Contains(t, body, `name="stake"`)
	assert.Contains(t, body, `hx-trigger="input changed delay:300ms, slip-updated from:body"`)
	assert.Contains(t, body, `<details`)
	assert.Contains(t, body, "Activity")
}

func TestLegFormIsReadyForTheBottomSheet(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/legs/new", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Add a leg")
	assert.Contains(t, body, `data-close-leg-sheet`)
	assert.Contains(t, body, `data-form-error`)
	assert.Contains(t, body, `name="selection"`)
	assert.Contains(t, body, `name="game_id"`)
}

func TestProfileKeepsInactiveMembersVisible(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	other, err := fixture.store.CreateUserWithIdentity(t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	_, err = fixture.store.JoinByToken(t.Context(), other.ID, fixture.group.InviteToken)
	require.NoError(t, err)
	_, err = fixture.store.SetUserActive(t.Context(), other.ID, false)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/me", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Sam")
	assert.Contains(t, body, "Inactive")
	assert.Contains(t, body, `name="active" value="1"`, "an inactive person can be turned back on")
}

func TestLegFormListsOnlyGamesBeforeKickoff(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	upcoming := addGame(t, fixture, gameKickoff())

	started, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772002",
		WeekID:     fixture.week.ID,
		HomeTeam:   "NYJ",
		AwayTeam:   "NE",
		HomeName:   "New York Jets",
		AwayName:   "New England Patriots",
		HomeESPNID: "20",
		AwayESPNID: "17",
		Kickoff:    time.Now().UTC().Add(-time.Hour).Truncate(time.Second),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/legs/new", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `name="game_id"`)
	assert.Contains(t, body, upcoming.ID)
	assert.Contains(t, body, "MIA @ BUF")
	assert.NotContains(t, body, started.ID, "a kicked-off game is not offered")
	assert.Contains(t, body, "required", "a current week asks for a game")
}

func TestLegFormPickerUsesTheScenarioClock(t *testing.T) {
	t.Parallel()

	scenarioNow := time.Now().UTC().Add(48*time.Hour + time.Minute)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return scenarioNow })
	started := addGame(t, fixture, gameKickoff())
	later, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772099",
		WeekID:     fixture.week.ID,
		HomeTeam:   "SEA",
		AwayTeam:   "SF",
		HomeName:   "Seattle Seahawks",
		AwayName:   "San Francisco 49ers",
		HomeESPNID: "26",
		AwayESPNID: "25",
		Kickoff:    scenarioNow.Add(2 * time.Hour),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/legs/new", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, later.ID, "a game after scenario time is still pickable")
	assert.NotContains(t, body, started.ID, "the real wall clock is not the picker's clock")
	assert.Contains(t, body, "required")
}

func TestLegFormListsKickedOffGamesForAPastWeek(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, time.Now().UTC().Add(-3*time.Hour).Truncate(time.Second))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/legs/new", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, game.ID, "a past week can still enter history")
	assert.Contains(t, body, "No game")
	assert.NotContains(t, body, "required")
}

func TestAddLegForAKickedOffGameAnswers422(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addGame(t, fixture, gameKickoff())
	started, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772003",
		WeekID:     fixture.week.ID,
		HomeTeam:   "NYJ",
		AwayTeam:   "NE",
		HomeName:   "New York Jets",
		AwayName:   "New England Patriots",
		HomeESPNID: "20",
		AwayESPNID: "17",
		Kickoff:    time.Now().UTC().Add(-time.Hour).Truncate(time.Second),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	response := postForm(t,
		fixture.handler,
		fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
		url.Values{
			"game_id":     {started.ID},
			"market_type": {"ML"},
			"team":        {"NE"},
			"side":        {"TEAM"},
			"odds":        {"-110"},
		},
	)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Contains(t, response.Body.String(), "that game already kicked off")
}

func TestSlipReferenceOddsUseMemberHomeBooks(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)

	_, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("draftkings"))
	require.NoError(t, err)
	sam, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, sam.ID))
	_, err = fixture.store.SetUserHomeBook(t.Context(), sam.ID, new("fanduel"))
	require.NoError(t, err)

	form := url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-110"},
	}
	response := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), form)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/payout?stake=10&boost=0", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Combined")
	assert.Contains(t, body, "1.91x", "the leg's own price still sets the combined odds")
	assert.Contains(t, body, "Reference")
	assert.Contains(t, body, "1.68x", "the median of the members' home-book prices")
	assert.Contains(t, body, "(-147)")
}

// TestLegResultMenuGradesALeg covers the chip menu on a leg and the PUT behind
// it.
func TestLegResultMenuGradesALeg(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	response := addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, fmt.Sprintf(`hx-put="/legs/%d/result"`, legs[0].ID))
	for _, result := range []string{"PENDING", "WON", "LOST", "PUSH", "VOID"} {
		assert.Contains(t, body, fmt.Sprintf(`name="result" value="%s"`, result))
	}

	graded := putForm(t,
		fixture.handler,
		fmt.Sprintf("/legs/%d/result", legs[0].ID),
		url.Values{"result": {"WON"}},
	)

	require.Equal(t, http.StatusOK, graded.Code, graded.Body.String())
	assert.Contains(t, graded.Body.String(), "GRADED")
	stored, err := fixture.store.LegByID(t.Context(), legs[0].ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, stored.Result)
	require.NotNil(t, stored.GradedBy)
	assert.Equal(t, fixture.user.Name, *stored.GradedBy)

	slip, err := fixture.store.SlipByID(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, slip.Result)
	assert.Equal(t, db.SlipGraded, slip.Status)
}

// TestLegResultRefusesAnUnknownResult answers 422 with the short message.
func TestLegResultRefusesAnUnknownResult(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	require.Equal(t, http.StatusOK, addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML").Code)
	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)

	response := putForm(t,
		fixture.handler,
		fmt.Sprintf("/legs/%d/result", legs[0].ID),
		url.Values{"result": {"MAYBE"}},
	)

	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Contains(t, response.Body.String(), "that result isn't valid")
}

// TestCreateFormListsPastWeeks checks the week picker offers earlier weeks and
// tells seasons apart.
func TestCreateFormListsPastWeeks(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	past, err := fixture.store.UpsertWeek(t.Context(), 2025, 2, 7)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, fmt.Sprintf(`<option value="%d"`, past.ID))
	assert.Contains(t, body, "2025 Week 7")
	assert.Contains(t, body, "Or add a past week")
}

// TestCreateSlipAcceptsAPastWeekWithFreeTextOtherLegs creates a slip for a week
// the slate never stored, then adds a free-text OTHER leg with no game.
func TestCreateSlipAcceptsAPastWeekWithFreeTextOtherLegs(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	response := postForm(t, fixture.handler, "/slips", url.Values{
		"week_id":         {strconv.FormatInt(fixture.week.ID, 10)},
		"season":          {"2025"},
		"season_type":     {"2"},
		"week":            {"7"},
		"name":            {"Week 7"},
		"legs_per_member": {"1"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	location := response.Header().Get("Hx-Redirect")
	require.NotEmpty(t, location)
	slipID, err := strconv.ParseInt(strings.TrimPrefix(location, "/slips/"), 10, 64)
	require.NoError(t, err)

	slip, err := fixture.store.SlipByID(t.Context(), slipID)
	require.NoError(t, err)
	assert.Equal(t, db.WeekID(2025, 2, 7), slip.WeekID, "the typed week wins over the picker")

	leg := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", slipID), url.Values{
		"market_type": {"OTHER"},
		"selection":   {"BUF -3.5"},
		"odds":        {"-110"},
	})
	require.Equal(t, http.StatusOK, leg.Code, leg.Body.String())
	assert.Contains(t, leg.Body.String(), "BUF -3.5")

	legs, err := fixture.store.LegsForSlip(t.Context(), slipID)
	require.NoError(t, err)
	require.Len(t, legs, 1)
	assert.Equal(t, "OTHER", legs[0].MarketType)
	assert.Equal(t, "BUF -3.5", legs[0].Selection)
	assert.Nil(t, legs[0].GameID, "a past-week OTHER leg needs no game")
}

// TestCreateFormRefusesAFutureWeek keeps a typed week from running ahead of the
// slate.
func TestCreateFormRefusesAFutureWeek(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	response := postForm(t, fixture.handler, "/slips", url.Values{
		"week_id":         {strconv.FormatInt(fixture.week.ID, 10)},
		"season":          {"2026"},
		"season_type":     {"2"},
		"week":            {"10"},
		"name":            {"Week 10"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
	})

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "started yet")
	_, err := fixture.store.WeekByID(t.Context(), db.WeekID(2026, 2, 10))
	require.ErrorIs(t, err, db.ErrWeekNotFound, "a refused week is not stored")
}

// postForm sends one already-encoded form as the browser's HTMX layer would.
func postForm(t *testing.T, handler http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	return sendForm(t, handler, http.MethodPost, path, form)
}

// putForm sends one already-encoded form as HTMX sends a grade.
func putForm(t *testing.T, handler http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	return sendForm(t, handler, http.MethodPut, path, form)
}

// sendForm sends one form with the HTMX headers the CSRF layer wants.
func sendForm(
	t *testing.T,
	handler http.Handler,
	method, path string,
	form url.Values,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(
		t.Context(),
		method,
		path,
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// addLeg posts one ML leg through the regular handler.
func addLeg(t *testing.T, handler http.Handler, slipID int64, selection string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		"market_type": {"ML"},
		"selection":   {selection},
		"team":        {strings.Split(selection, " ")[0]},
		"odds":        {"-110"},
	}
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("/slips/%d/legs", slipID),
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Hx-Request", "true")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// waitForEvent reads one Server-Sent Event name from body.
func waitForEvent(t *testing.T, body io.Reader) string {
	t.Helper()

	type result struct {
		event string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			line := scanner.Text()
			if event, ok := strings.CutPrefix(line, "event: "); ok {
				done <- result{event: event}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			done <- result{err: err}
			return
		}
		done <- result{err: errors.New("stream ended before an event")}
	}()

	select {
	case got := <-done:
		require.NoError(t, got.err)
		return got.event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for an SSE event")
		return ""
	}
}

func TestLegFormPlayerAutocompleteDatalist(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	now := time.Now().UTC()
	game := addGame(t, fixture, gameKickoff())

	// Add roster players
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
		{
			AthleteID:  "4429013",
			Team:       "MIA",
			ESPNTeamID: "15",
			FullName:   "De'Von Achane",
			Position:   "RB",
			UpdatedAt:  now,
		},
	}))

	// Add prop odds player
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Dalton Kincaid",
			Side:       "OVER",
			Line:       new(45.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	// GET /slips/{id}/legs/new
	reqNew := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/legs/new", fixture.slip.ID),
		nil,
	)
	recNew := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recNew, reqNew)
	assert.Equal(t, http.StatusOK, recNew.Code)
	bodyNew := html.UnescapeString(recNew.Body.String())

	assert.Contains(t, bodyNew, `list="player-names"`)
	assert.Contains(t, bodyNew, `<datalist id="player-names">`)
	assert.Contains(t, bodyNew, `<option value="Dalton Kincaid"></option>`)
	assert.Contains(t, bodyNew, `<option value="De'Von Achane"></option>`)
	assert.Contains(t, bodyNew, `<option value="Josh Allen"></option>`)

	// Create a leg to test edit
	leg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &game.ID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 265.5 Pass Yds",
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(265.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// GET /legs/{id}/edit
	reqEdit := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/legs/%d/edit", leg.ID),
		nil,
	)
	recEdit := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recEdit, reqEdit)
	assert.Equal(t, http.StatusOK, recEdit.Code)
	bodyEdit := html.UnescapeString(recEdit.Body.String())

	assert.Contains(t, bodyEdit, `list="player-names"`)
	assert.Contains(t, bodyEdit, `<datalist id="player-names">`)
	assert.Contains(t, bodyEdit, `<option value="Josh Allen"></option>`)
}

func TestAutomaticPlayerResolutionOnSave(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 5)
	now := time.Now().UTC()
	game := addGame(t, fixture, gameKickoff())

	// Add roster players: Josh Allen on BUF
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

	// 1. Post a new leg without player_id or team
	form := url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
		"line":        {"265.5"},
		"odds":        {"-110"},
	}
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	rec := httptest.NewRecorder()
	fixture.handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)
	assert.Equal(t, "3918298", *legs[0].PlayerID)
	assert.Equal(t, "BUF", *legs[0].Team)

	// 2. Post a leg where team is explicitly provided
	form2 := url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"player_name": {"Josh Allen"},
		"team":        {"MIA"}, // override team
		"side":        {"OVER"},
		"line":        {"265.5"},
		"odds":        {"-110"},
	}
	req2 := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
		strings.NewReader(form2.Encode()),
	)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Hx-Request", "true")
	rec2 := httptest.NewRecorder()
	fixture.handler.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)

	legs, err = fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	assert.Equal(t, "MIA", *legs[1].Team)
	assert.Equal(t, "3918298", *legs[1].PlayerID)

	// 3. Update an existing leg via PUT
	updateForm := url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"player_name": {"Josh Allen"},
		"side":        {"UNDER"},
		"line":        {"270.5"},
		"odds":        {"-105"},
	}
	reqUpdate := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPut,
		fmt.Sprintf("/legs/%d", legs[0].ID),
		strings.NewReader(updateForm.Encode()),
	)
	reqUpdate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqUpdate.Header.Set("Hx-Request", "true")
	recUpdate := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recUpdate, reqUpdate)
	assert.Equal(t, http.StatusOK, recUpdate.Code)

	updatedLeg, err := fixture.store.LegByID(t.Context(), legs[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "3918298", *updatedLeg.PlayerID)
	assert.Equal(t, "BUF", *updatedLeg.Team)
	assert.Equal(t, "UNDER", *updatedLeg.Side)

	// 4. Post an unmatched player leaves player_id nil
	formUnknown := url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"player_name": {"Unknown Quarterback"},
		"side":        {"OVER"},
		"line":        {"200.5"},
		"odds":        {"-110"},
	}
	reqUnknown := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("/slips/%d/legs", fixture.slip.ID),
		strings.NewReader(formUnknown.Encode()),
	)
	reqUnknown.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqUnknown.Header.Set("Hx-Request", "true")
	recUnknown := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recUnknown, reqUnknown)
	assert.Equal(t, http.StatusOK, recUnknown.Code)

	legs3, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs3, 3)
	assert.Nil(t, legs3[2].PlayerID)
}
