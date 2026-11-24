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
	as      func(db.User)
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
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeFullWeek,
		legsPerMember,
		[]int64{user.ID},
	)
	require.NoError(t, err)

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	hub := &live.Hub{}
	service := slips.New(store, hub, nil)
	stream := live.NewStream(hub, func(int64) bool { return false })

	signedIn := user
	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return signedIn, true
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
		as:      func(u db.User) { signedIn = u },
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

func TestSlipPageOmitsLiveStreamWhenGradedAndKeepsItWhenLocked(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))

	lockedRecorder := httptest.NewRecorder()
	lockedReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(lockedRecorder, lockedReq)
	require.Equal(t, http.StatusOK, lockedRecorder.Code)
	assert.Contains(t, lockedRecorder.Body.String(), fmt.Sprintf(`data-live-url="/slips/%d/events"`, fixture.slip.ID))

	require.NoError(t, fixture.store.SetSlipGrade(t.Context(), fixture.slip.ID, parlay.Won, db.SlipGraded))

	gradedRecorder := httptest.NewRecorder()
	gradedReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(gradedRecorder, gradedReq)
	require.Equal(t, http.StatusOK, gradedRecorder.Code)
	assert.NotContains(t, gradedRecorder.Body.String(), "data-live-url")
}

func TestSlipEventsSaysByeWhenGraded(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	require.NoError(t, fixture.store.SetSlipGrade(t.Context(), fixture.slip.ID, parlay.Won, db.SlipGraded))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/events", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "event: bye\ndata: {}\n\n", recorder.Body.String())
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
	assert.NotContains(t, body, fmt.Sprintf(`hx-get="/slips/%d/legs/new"`, fixture.slip.ID))
	assert.Contains(t, body, `id="leg-sheet"`)
	assert.Contains(t, body, `name="stake"`)
	assert.Contains(t, body, `hx-trigger="input changed delay:300ms, slip-updated from:body"`)
	assert.Contains(t, body, `<details`)
	assert.Contains(t, body, "Activity")
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

func TestSlipReferenceOddsIncludeMovedMemberBookLine(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	now := time.Now().UTC()
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(246.0),
			Price:      -130,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	_, err = fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("draftkings"))
	require.NoError(t, err)
	sam, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam-moved@example.com", "google-sam-moved", "")
	require.NoError(t, err)
	require.NoError(t, fixture.store.AddSlipMember(t.Context(), fixture.slip.ID, sam.ID))
	_, err = fixture.store.SetUserHomeBook(t.Context(), sam.ID, new("fanduel"))
	require.NoError(t, err)

	form := url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"team":        {"BUF"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
		"line":        {"245.5"},
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
	// Mean of -110 (1.9091) and -130 (1.7692) is ~1.839x (-119), not 1.91x.
	assert.Contains(t, body, "1.84x")
	assert.Contains(t, body, "(-119)")
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
	assert.Contains(t, bodyEdit, `<option value="Dalton Kincaid"></option>`)
	assert.Contains(t, bodyEdit, `<option value="De'Von Achane"></option>`)
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

func TestSiblingSlipSwitcher(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	// 1. Single slip in week: no sibling switcher rendered.
	reqSingle := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	recSingle := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recSingle, reqSingle)
	assert.Equal(t, http.StatusOK, recSingle.Code)
	assert.NotContains(t, recSingle.Body.String(), "data-sibling-switcher")

	// 2. Add a second slip for Thursday Night in the same week.
	thuSlip, err := fixture.store.CreateSlip(
		t.Context(),
		fixture.group.ID,
		fixture.week.ID,
		nil,
		"Thursday Night",
		db.SlateScopeThursday,
		1,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)

	// Lock the Thursday night slip.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), thuSlip.ID, db.SlipLocked))

	// Add 1 leg to the main Sunday slip (legsPerMember = 2) for Ada.
	game := addGame(t, fixture, gameKickoff())
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &game.ID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 250.5",
		Odds:       -110,
	})
	require.NoError(t, err)

	// 3. View Thursday slip: shows sibling switcher with Thursday (Locked) and Sunday Main (1/2 Picks).
	reqThu := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", thuSlip.ID),
		nil,
	)
	recThu := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recThu, reqThu)
	assert.Equal(t, http.StatusOK, recThu.Code)
	bodyThu := recThu.Body.String()
	assert.Contains(t, bodyThu, "data-sibling-switcher")
	assert.Contains(t, bodyThu, "Thursday Night")
	assert.Contains(t, bodyThu, "(Locked)")
	assert.Contains(t, bodyThu, fixture.slip.Name)
	assert.Contains(t, bodyThu, "(1/2 Picks)")
	assert.Contains(t, bodyThu, fmt.Sprintf(`href="/slips/%d"`, fixture.slip.ID))
	assert.Contains(t, bodyThu, fmt.Sprintf(`href="/slips/%d"`, thuSlip.ID))
	// Thursday is current on reqThu, Sunday is not.
	assert.Contains(t, bodyThu, `aria-current="page"`)

	// 4. View Sunday slip: shows Thursday as sibling and Sunday as current.
	reqSun := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	recSun := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recSun, reqSun)
	assert.Equal(t, http.StatusOK, recSun.Code)
	bodySun := recSun.Body.String()
	assert.Contains(t, bodySun, "data-sibling-switcher")
	assert.Contains(t, bodySun, "(Locked)")
	assert.Contains(t, bodySun, "(1/2 Picks)")
	assert.Contains(t, bodySun, `aria-current="page"`)

	// 5. Check HTMX panel refresh also contains sibling switcher.
	reqPanel := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	reqPanel.Header.Set("Hx-Request", "true")
	recPanel := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recPanel, reqPanel)
	assert.Equal(t, http.StatusOK, recPanel.Code)
	assert.Contains(t, recPanel.Body.String(), "data-sibling-switcher")

	// 6. When member locks in on Sunday Main, switcher updates to (Locked).
	lockTime := time.Now().UTC()
	require.NoError(t, fixture.store.SetSlipMemberLock(t.Context(), fixture.slip.ID, fixture.user.ID, &lockTime))
	recLocked := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recLocked, reqSun)
	assert.Equal(t, http.StatusOK, recLocked.Code)
	bodyLocked := recLocked.Body.String()
	// Both slips should now show (Locked)
	assert.Equal(t, 2, strings.Count(bodyLocked, "(Locked)"))

	// 7. When slip is graded, switcher reflects the result (e.g. Won).
	require.NoError(t, fixture.store.SetSlipGrade(t.Context(), thuSlip.ID, parlay.Won, db.SlipGraded))
	recGraded := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recGraded, reqSun)
	assert.Equal(t, http.StatusOK, recGraded.Code)
	assert.Contains(t, recGraded.Body.String(), "(Won)")
}

func TestCreateSlipFormSlateScopeSelection(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	// Default renders Sunday Slate as selected.
	reqDefault := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new", nil)
	recDefault := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recDefault, reqDefault)
	require.Equal(t, http.StatusOK, recDefault.Code)
	bodyDefault := recDefault.Body.String()
	assert.Contains(t, bodyDefault, `<select name="slate_scope"`)
	assert.Contains(t, bodyDefault, `<option value="sunday_main" selected>Sunday Slate</option>`)
	assert.Contains(t, bodyDefault, `<option value="thursday">Thursday Night</option>`)
	assert.Contains(t, bodyDefault, `<option value="full_week">Full Week</option>`)

	// Pre-select Thursday Night via ?scope=thursday.
	reqThu := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new?scope=thursday", nil)
	recThu := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recThu, reqThu)
	require.Equal(t, http.StatusOK, recThu.Code)
	bodyThu := recThu.Body.String()
	assert.Contains(t, bodyThu, `<option value="thursday" selected>Thursday Night</option>`)
	assert.Contains(t, bodyThu, `<option value="sunday_main">Sunday Slate</option>`)

	// Pre-select Full Week via ?slate_scope=full_week.
	reqFull := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new?slate_scope=full_week", nil)
	recFull := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recFull, reqFull)
	require.Equal(t, http.StatusOK, recFull.Code)
	bodyFull := recFull.Body.String()
	assert.Contains(t, bodyFull, `<option value="full_week" selected>Full Week</option>`)
}

func TestCreateSlipWithSlateScope(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)

	// 1. Explicit slate_scope: "thursday"
	resThu := postForm(t, fixture.handler, "/slips", url.Values{
		"week_id":         {strconv.FormatInt(fixture.week.ID, 10)},
		"name":            {"Thursday One-Off"},
		"slate_scope":     {"thursday"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
	})
	require.Equal(t, http.StatusOK, resThu.Code, resThu.Body.String())
	locThu := resThu.Header().Get("Hx-Redirect")
	require.NotEmpty(t, locThu)
	idThu, err := strconv.ParseInt(strings.TrimPrefix(locThu, "/slips/"), 10, 64)
	require.NoError(t, err)
	slipThu, err := fixture.store.SlipByID(t.Context(), idThu)
	require.NoError(t, err)
	assert.Equal(t, db.SlateScopeThursday, slipThu.SlateScope)

	// 2. Default slate_scope (omitted) defaults to Sunday Main
	resDefault := postForm(t, fixture.handler, "/slips", url.Values{
		"week_id":         {strconv.FormatInt(fixture.week.ID, 10)},
		"name":            {"Default Sunday One-Off"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
	})
	require.Equal(t, http.StatusOK, resDefault.Code, resDefault.Body.String())
	locDefault := resDefault.Header().Get("Hx-Redirect")
	require.NotEmpty(t, locDefault)
	idDefault, err := strconv.ParseInt(strings.TrimPrefix(locDefault, "/slips/"), 10, 64)
	require.NoError(t, err)
	slipDefault, err := fixture.store.SlipByID(t.Context(), idDefault)
	require.NoError(t, err)
	assert.Equal(t, db.SlateScopeSundayMain, slipDefault.SlateScope)

	// 3. Invalid slate_scope triggers 422 error and renders form with error message
	resInvalid := postForm(t, fixture.handler, "/slips", url.Values{
		"week_id":         {strconv.FormatInt(fixture.week.ID, 10)},
		"name":            {"Invalid Scope Slip"},
		"slate_scope":     {"bogus_scope"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
	})
	require.Equal(t, http.StatusOK, resInvalid.Code)
	bodyInvalid := resInvalid.Body.String()
	assert.Contains(t, bodyInvalid, "invalid slate scope")
	assert.Contains(t, bodyInvalid, "Invalid Scope Slip")
}

func TestSlipPanelRendersLegLogosAndPortraits(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 5)
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
		Kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
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
		{
			AthleteID:   "9999999",
			Team:        "KC",
			ESPNTeamID:  "12",
			FullName:    "Rookie Player",
			Position:    "WR",
			HeadshotURL: "",
			UpdatedAt:   time.Now().UTC(),
		},
	}))

	// 1. Matched player leg with headshot.
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     new("401772001"),
		MarketType: "PASS_YARDS",
		Selection:  "Josh Allen Over 255.5 Passing Yards",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		PlayerID:   new("3918298"),
		Side:       new("OVER"),
		Line:       new(255.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// 2. Matched player leg without headshot, falls back to team logo.
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     new("401772001"),
		MarketType: "PASS_YARDS",
		Selection:  "Tua Tagovailoa Over 245.5 Passing Yards",
		Team:       new("MIA"),
		PlayerName: new("Tua Tagovailoa"),
		PlayerID:   new("4241479"),
		Side:       new("OVER"),
		Line:       new(245.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// 3. Matched player leg without headshot or team logo, falls back to initials.
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		MarketType: "REC_YARDS",
		Selection:  "Rookie Player Over 45.5 Receiving Yards",
		Team:       new("KC"),
		PlayerName: new("Rookie Player"),
		PlayerID:   new("9999999"),
		Side:       new("OVER"),
		Line:       new(45.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// 4. Team-scoped spread leg.
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     new("401772001"),
		MarketType: "SPREAD",
		Selection:  "Bills -3.5",
		Team:       new("BUF"),
		Odds:       -110,
	})
	require.NoError(t, err)

	// 5. Game TOTAL leg (GameID set, Team empty).
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     new("401772001"),
		MarketType: "TOTAL",
		Selection:  "MIA @ BUF Over 47.5",
		Side:       new("OVER"),
		Line:       new(47.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// 6. Custom/unlinked leg without game, team, or player.
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		MarketType: "OTHER",
		Selection:  "Custom Backyard Prop",
		Odds:       150,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	fixture.handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/panel", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, `src="https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png"`)
	assert.Contains(t, body, `>JA</span>`)
	assert.Contains(t, body, `src="https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"`)
	assert.Contains(t, body, `src="https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"`)
	assert.Contains(t, body, `>RP</span>`)
	assert.Contains(t, body, "Custom Backyard Prop")
	// Custom leg must not render a data-media placeholder next to its selection.
	customIdx := strings.Index(body, "Custom Backyard Prop")
	require.Positive(t, customIdx)
	liStart := strings.LastIndex(body[:customIdx], "<li ")
	require.Positive(t, liStart)
	assert.NotContains(t, body[liStart:customIdx], "data-media")
}

// TestSlipPanelRendersGradedByAndNeedsGradingAmberChip checks that the slip
// panel surfaces Auto vs manual grader names on graded legs, styles
// NEEDS_GRADING legs in amber with "Needs grading", and keeps the five
// override options in the menu.
func TestSlipPanelRendersGradedByAndNeedsGradingAmberChip(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 3)

	autoLeg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		MarketType: "ML",
		Selection:  "BUF ML",
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, fixture.store.SetLegResult(t.Context(), autoLeg.ID, parlay.Won, slips.AutoGraderName))

	manualLeg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		MarketType: "SPREAD",
		Selection:  "KC -3.5",
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, fixture.store.SetLegResult(t.Context(), manualLeg.ID, parlay.Lost, fixture.user.Name))

	needsLeg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		MarketType: "OTHER",
		Selection:  "Gatorade Color Orange",
		Odds:       250,
	})
	require.NoError(t, err)
	require.NoError(t, fixture.store.SetLegResult(t.Context(), needsLeg.ID, parlay.NeedsGrading, slips.AutoGraderName))

	rec := httptest.NewRecorder()
	fixture.handler.ServeHTTP(rec, httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/panel", fixture.slip.ID), nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, "WON · Auto")
	assert.Contains(t, body, "LOST · "+fixture.user.Name)
	assert.Contains(t, body, "badge-push")
	assert.Contains(t, body, "Needs grading")
	for _, result := range []string{"PENDING", "WON", "LOST", "PUSH", "VOID"} {
		assert.Contains(t, body, fmt.Sprintf(`name="result" value="%s"`, result))
	}
	assert.NotContains(t, body, `name="result" value="NEEDS_GRADING"`)

	// Overriding a NEEDS_GRADING leg manually updates both chip and attribution.
	override := putForm(t,
		fixture.handler,
		fmt.Sprintf("/legs/%d/result", needsLeg.ID),
		url.Values{"result": {"WON"}},
	)
	require.Equal(t, http.StatusOK, override.Code, override.Body.String())
	overrideBody := override.Body.String()
	assert.Contains(t, overrideBody, "WON · "+fixture.user.Name)
	assert.NotContains(t, overrideBody, "Needs grading")
	assert.NotContains(t, overrideBody, "badge-push")
}

func TestAddLegsBatchInsertsAtomicallyAndPublishesSlipUpdated(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	server := httptest.NewServer(fixture.handler)
	t.Cleanup(server.Close)

	eventsURL := fmt.Sprintf("%s/slips/%d/events", server.URL, fixture.slip.ID)
	streamReq, err := http.NewRequestWithContext(t.Context(), http.MethodGet, eventsURL, nil)
	require.NoError(t, err)
	streamResp, err := server.Client().Do(streamReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, streamResp.StatusCode)
	t.Cleanup(func() { assert.NoError(t, streamResp.Body.Close()) })

	form := url.Values{
		"legs[0][game_id]":     {game.ID},
		"legs[0][market_type]": {"PASS_YDS"},
		"legs[0][team]":        {"BUF"},
		"legs[0][player_name]": {"Josh Allen"},
		"legs[0][side]":        {"OVER"},
		"legs[0][line]":        {"245.5"},
		"legs[0][odds]":        {"-110"},
		"legs[1][game_id]":     {game.ID},
		"legs[1][market_type]": {"TOTAL"},
		"legs[1][side]":        {"OVER"},
		"legs[1][line]":        {"47.5"},
		"legs[1][odds]":        {"-105"},
	}
	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		fmt.Sprintf("%s/slips/%d/legs/batch", server.URL, fixture.slip.ID),
		strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")

	resp, err := server.Client().Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	assert.Equal(t, "slip-updated", resp.Header.Get("Hx-Trigger"))
	assert.Contains(t, string(body), "Josh Allen")
	assert.Contains(t, string(body), "Over 47.5")

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 2)

	assert.Equal(t, "slip-updated", waitForEvent(t, streamResp.Body))
}

func TestAddLegsBatchSkipsDuplicatesAndChecksCapacity(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	// Seed 1 existing leg on the viewer's slip (1 of 2 slots used).
	first := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-110"},
	})
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	// Over-capacity: 2 new legs when only 1 slot remains -> 422, 0 legs added.
	overCap := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs/batch", fixture.slip.ID), url.Values{
		"legs[0][game_id]":     {game.ID},
		"legs[0][market_type]": {"SPREAD"},
		"legs[0][team]":        {"BUF"},
		"legs[0][side]":        {"TEAM"},
		"legs[0][line]":        {"-3.5"},
		"legs[0][odds]":        {"-110"},
		"legs[1][game_id]":     {game.ID},
		"legs[1][market_type]": {"TOTAL"},
		"legs[1][side]":        {"OVER"},
		"legs[1][line]":        {"47.5"},
		"legs[1][odds]":        {"-110"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, overCap.Code)
	assert.Contains(t, overCap.Body.String(), "you've already added all your legs")

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1, "atomic failure must not insert partial legs")

	// Duplicate + 1 new leg: duplicate BUF ML is skipped, so 1 new leg fits in remaining slot.
	withDup := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs/batch", fixture.slip.ID), url.Values{
		"legs[0][game_id]":     {game.ID},
		"legs[0][market_type]": {"ML"},
		"legs[0][team]":        {"BUF"},
		"legs[0][side]":        {"TEAM"},
		"legs[0][odds]":        {"-120"},
		"legs[1][game_id]":     {game.ID},
		"legs[1][market_type]": {"TOTAL"},
		"legs[1][side]":        {"OVER"},
		"legs[1][line]":        {"47.5"},
		"legs[1][odds]":        {"-110"},
	})
	require.Equal(t, http.StatusOK, withDup.Code, withDup.Body.String())

	legs, err = fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 2)
}

func TestAddLegsBatchAtomicRollbackOnKickoffAndValidation(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	upcoming := addGame(t, fixture, gameKickoff())
	started, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "started-batch-game",
		WeekID:     fixture.week.ID,
		HomeTeam:   "NE",
		AwayTeam:   "NYJ",
		HomeESPNID: "17",
		AwayESPNID: "20",
		Kickoff:    time.Now().UTC().Add(-time.Hour).Truncate(time.Second),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	// One valid upcoming leg + one kicked-off leg -> 422, neither inserted.
	resp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs/batch", fixture.slip.ID), url.Values{
		"legs[0][game_id]":     {upcoming.ID},
		"legs[0][market_type]": {"ML"},
		"legs[0][team]":        {"BUF"},
		"legs[0][side]":        {"TEAM"},
		"legs[0][odds]":        {"-110"},
		"legs[1][game_id]":     {started.ID},
		"legs[1][market_type]": {"ML"},
		"legs[1][team]":        {"NE"},
		"legs[1][side]":        {"TEAM"},
		"legs[1][odds]":        {"-110"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, resp.Body.String(), "that game already kicked off")

	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	assert.Empty(t, legs)

	// Empty batch -> 422.
	empty := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs/batch", fixture.slip.ID), url.Values{})
	assert.Equal(t, http.StatusUnprocessableEntity, empty.Code)
}

func TestAddLegsBatchRefusesLockedMemberAndSupportsBoardPartial(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	// Board partial mode returns #pick-body and triggers tray-updated, slip-updated.
	boardResp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs/batch", fixture.slip.ID), url.Values{
		"from":                 {"board"},
		"legs[0][game_id]":     {game.ID},
		"legs[0][market_type]": {"ML"},
		"legs[0][team]":        {"BUF"},
		"legs[0][side]":        {"TEAM"},
		"legs[0][odds]":        {"-110"},
		"legs[1][game_id]":     {game.ID},
		"legs[1][market_type]": {"TOTAL"},
		"legs[1][side]":        {"OVER"},
		"legs[1][line]":        {"47.5"},
		"legs[1][odds]":        {"-110"},
	})
	require.Equal(t, http.StatusOK, boardResp.Code, boardResp.Body.String())
	assert.Equal(t, "tray-updated, slip-updated", boardResp.Header().Get("Hx-Trigger"))
	assert.Contains(t, boardResp.Body.String(), `id="pick-body"`)

	// Lock the member in; subsequent batch add is refused with 422.
	lockResp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/lock", fixture.slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, lockResp.Code, lockResp.Body.String())

	lockedResp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs/batch", fixture.slip.ID), url.Values{
		"legs[0][game_id]":     {game.ID},
		"legs[0][market_type]": {"SPREAD"},
		"legs[0][team]":        {"BUF"},
		"legs[0][side]":        {"TEAM"},
		"legs[0][line]":        {"-3.5"},
		"legs[0][odds]":        {"-110"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, lockedResp.Code)
	assert.Contains(t, lockedResp.Body.String(), "the slip is locked")
}

func seedAGradeBoardForHubTest(t *testing.T, fixture *slipFixture, game db.Game) {
	t.Helper()

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{AthleteID: "p-allen", FullName: "Josh Allen", Team: "BUF", Position: "QB"},
		{AthleteID: "p-cook", FullName: "James Cook", Team: "BUF", Position: "RB"},
	}))
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "p-allen",
		GamesPlayed: 5,
		StatsJSON:   `{"PASS_YDS":[280.0, 290.0, 300.0, 295.0, 310.0]}`,
		UpdatedAt:   now,
	}))
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "p-cook",
		GamesPlayed: 5,
		StatsJSON:   `{"RUSH_YDS":[85.0, 90.0, 95.0, 100.0, 88.0]}`,
		UpdatedAt:   now,
	}))

	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -105,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(245.5),
			Price:      -115,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "OVER",
			Line:       new(65.5),
			Price:      -105,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "UNDER",
			Line:       new(65.5),
			Price:      -115,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)
}

func TestSlipHubRendersTopAGradeIdeasDuringPickStep(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		fixture.handler.ServeHTTP(rec, req)
		return rec
	}

	// Before any odds are cached, the strip stays hidden.
	emptyPage := get(fmt.Sprintf("/slips/%d", fixture.slip.ID))
	require.Equal(t, http.StatusOK, emptyPage.Code)
	assert.NotContains(t, emptyPage.Body.String(), "Top A-grade ideas")

	seedAGradeBoardForHubTest(t, fixture, game)

	pageResp := get(fmt.Sprintf("/slips/%d", fixture.slip.ID))
	require.Equal(t, http.StatusOK, pageResp.Code)
	body := pageResp.Body.String()
	assert.Contains(t, body, "Top A-grade ideas")
	assert.Contains(t, body, "data-highlight-leg")
	assert.Contains(t, body, "data-highlight-parlay")
	assert.Contains(t, body, "grade-a")
	assert.Contains(t, body, "Josh Allen")
	assert.Contains(t, body, "James Cook")
	assert.Contains(t, body, "Add all 2")
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/legs/batch"`, fixture.slip.ID))
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/legs"`, fixture.slip.ID))

	// Panel partial also renders the strip.
	panelResp := get(fmt.Sprintf("/slips/%d/panel", fixture.slip.ID))
	require.Equal(t, http.StatusOK, panelResp.Code)
	assert.Contains(t, panelResp.Body.String(), "Top A-grade ideas")
}

func TestSlipHubHighlightsHideAndDisableWhenSlotsFullOrLocked(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedAGradeBoardForHubTest(t, fixture, game)

	// Fill 1 of 2 slots: 1 slot remains, so 2-leg combo disables bulk add.
	addOne := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-110"},
	})
	require.Equal(t, http.StatusOK, addOne.Code, addOne.Body.String())
	partialBody := addOne.Body.String()
	assert.Contains(t, partialBody, "Top A-grade ideas")
	assert.Contains(t, partialBody, "Needs 2 open slots")
	assert.NotContains(t, partialBody, "Add all 2")

	// Fill the 2nd slot -> StepLockIn -> highlights strip disappears.
	addTwo := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
		"game_id":     {game.ID},
		"market_type": {"TOTAL"},
		"side":        {"OVER"},
		"line":        {"47.5"},
		"odds":        {"-110"},
	})
	require.Equal(t, http.StatusOK, addTwo.Code, addTwo.Body.String())
	assert.NotContains(t, addTwo.Body.String(), "Top A-grade ideas")

	// Lock in -> StepExport -> highlights strip stays hidden.
	lockResp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/lock", fixture.slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, lockResp.Code, lockResp.Body.String())
	assert.NotContains(t, lockResp.Body.String(), "Top A-grade ideas")
}
