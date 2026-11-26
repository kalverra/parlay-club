package web_test

import (
	"bufio"
	"context"
	"encoding/json"
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
	"github.com/kalverra/parlay-club/internal/ingest/espn"
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

// TestCreateFormListsOnlyUpcomingWeeks checks /slips/new excludes past and
// completed weeks and omits the legacy past-week form.
func TestCreateFormListsOnlyUpcomingWeeks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return now })

	wk10, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 10)
	require.NoError(t, err)
	wk11, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 11)
	require.NoError(t, err)
	wk12, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 12)
	require.NoError(t, err)

	addGame := func(id string, weekID int64, kickoff time.Time, status db.GameStatus) {
		_, err := fixture.store.UpsertGame(t.Context(), db.Game{
			ID: id, WeekID: weekID, HomeTeam: "BUF", AwayTeam: "KC",
			HomeName: "Buffalo Bills", AwayName: "Kansas City Chiefs",
			HomeESPNID: "2", AwayESPNID: "12", Kickoff: kickoff, Status: status, UpdatedAt: now,
		})
		require.NoError(t, err)
	}
	addGame("g-10", wk10.ID, now.Add(-10*24*time.Hour), db.GameFinal)
	addGame("g-11", wk11.ID, now.Add(-1*time.Hour), db.GameLive)
	addGame("g-12", wk12.ID, now.Add(4*24*time.Hour), db.GameScheduled)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new", nil)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, fmt.Sprintf(`<option value="%d"`, wk10.ID))
	assert.NotContains(t, body, fmt.Sprintf(`<option value="%d"`, wk11.ID))
	assert.Contains(t, body, fmt.Sprintf(`<option value="%d"`, wk12.ID))
	assert.NotContains(t, body, "Or add a past week")
}

// TestCreateSlipRejectsPastAndCompletedWeeks verifies POST /slips refuses
// completed weeks, legacy season/week fields, and empty game selections with 422.
func TestCreateSlipRejectsPastAndCompletedWeeks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return now })

	wk11, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 11)
	require.NoError(t, err)
	wk12, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 12)
	require.NoError(t, err)

	_, err = fixture.store.UpsertGame(t.Context(), db.Game{
		ID: "g-11", WeekID: wk11.ID, HomeTeam: "BUF", AwayTeam: "KC",
		HomeName: "Buffalo Bills", AwayName: "Kansas City Chiefs",
		HomeESPNID: "2", AwayESPNID: "12", Kickoff: now.Add(-2 * time.Hour), Status: db.GameFinal, UpdatedAt: now,
	})
	require.NoError(t, err)
	_, err = fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "g-12",
		WeekID:     wk12.ID,
		HomeTeam:   "DET",
		AwayTeam:   "GB",
		HomeName:   "Detroit Lions",
		AwayName:   "Green Bay Packers",
		HomeESPNID: "8",
		AwayESPNID: "9",
		Kickoff:    now.Add(4 * 24 * time.Hour),
		Status:     db.GameScheduled,
		UpdatedAt:  now,
	})
	require.NoError(t, err)

	// Completed Week 11 rejected with 422.
	recPast := postForm(t, fixture.handler, "/slips", url.Values{
		"mode":            {"one_off"},
		"week_id":         {strconv.FormatInt(wk11.ID, 10)},
		"name":            {"Week 11"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
		"game_id":         {"g-11"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, recPast.Code)

	// Legacy season/week fields rejected with 422.
	recLegacy := postForm(t, fixture.handler, "/slips", url.Values{
		"week_id":         {strconv.FormatInt(wk12.ID, 10)},
		"season":          {"2025"},
		"season_type":     {"2"},
		"week":            {"7"},
		"name":            {"Week 7"},
		"legs_per_member": {"1"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
		"game_id":         {"g-12"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, recLegacy.Code)

	// Zero games selected returns 422 (ErrSlipGamesRequired).
	recEmpty := postForm(t, fixture.handler, "/slips", url.Values{
		"mode":            {"one_off"},
		"week_id":         {strconv.FormatInt(wk12.ID, 10)},
		"name":            {"Week 12 Empty"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, recEmpty.Code)
	assert.Contains(t, recEmpty.Body.String(), db.ErrSlipGamesRequired.Error())
}

// TestCreateFormStates verifies existing-slip copy, empty-week copy, and zero-upcoming state.
func TestCreateFormStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return now })

	// With zero upcoming games, /slips/new shows empty state and hides the form.
	recNone := httptest.NewRecorder()
	reqNone := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new", nil)
	fixture.handler.ServeHTTP(recNone, reqNone)
	require.Equal(t, http.StatusOK, recNone.Code)
	assert.Contains(t, recNone.Body.String(), "No upcoming games are loaded yet.")
	assert.NotContains(t, recNone.Body.String(), "data-slip-create-form")

	// Add upcoming games to Week 9 (has existing slip) and Week 10 (no slip yet).
	wk10, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 10)
	require.NoError(t, err)
	for _, tc := range []struct {
		id     string
		weekID int64
		kick   time.Time
	}{
		{"g-9", fixture.week.ID, now.Add(72 * time.Hour)},
		{"g-10", wk10.ID, now.Add(10 * 24 * time.Hour)},
	} {
		_, err := fixture.store.UpsertGame(t.Context(), db.Game{
			ID: tc.id, WeekID: tc.weekID, HomeTeam: "BUF", AwayTeam: "KC",
			HomeName: "Buffalo Bills", AwayName: "Kansas City Chiefs",
			HomeESPNID: "2", AwayESPNID: "12", Kickoff: tc.kick, Status: db.GameScheduled, UpdatedAt: now,
		})
		require.NoError(t, err)
	}

	recExisting := httptest.NewRecorder()
	reqExisting := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new", nil)
	fixture.handler.ServeHTTP(recExisting, reqExisting)
	require.Equal(t, http.StatusOK, recExisting.Code)
	bodyExisting := html.UnescapeString(recExisting.Body.String())
	assert.Contains(t, bodyExisting, "Add a slip for Week 9")
	assert.NotContains(t, bodyExisting, "No slip for Week 9 yet")
	assert.NotContains(t, bodyExisting, "data-past-week")

	recEmpty := httptest.NewRecorder()
	reqEmpty := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/new?week=%d", wk10.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recEmpty, reqEmpty)
	require.Equal(t, http.StatusOK, recEmpty.Code)
	bodyEmpty := html.UnescapeString(recEmpty.Body.String())
	assert.Contains(t, bodyEmpty, "Start Week 10’s slip")
	assert.Contains(t, bodyEmpty, "No slip for Week 10 yet.")
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

	// 1. Single slip in week: no sibling switcher rendered, scope badge and + New slip shown.
	reqSingle := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	recSingle := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recSingle, reqSingle)
	assert.Equal(t, http.StatusOK, recSingle.Code)
	bodySingle := recSingle.Body.String()
	assert.NotContains(t, bodySingle, "data-sibling-switcher")
	assert.NotContains(t, bodySingle, `id="slip-switcher"`)
	assert.Contains(t, bodySingle, "Full Week")
	assert.Contains(t, bodySingle, fmt.Sprintf(`href="/slips/new?week=%d"`, fixture.week.ID))
	assert.Contains(t, bodySingle, "+ New slip")

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
	assert.NotContains(t, bodyThu, `id="slip-switcher"`)
	assert.Contains(t, bodyThu, "data-sibling-switcher")
	assert.Contains(t, bodyThu, "Thursday Slate")
	assert.Contains(t, bodyThu, "Thursday Night")
	assert.Contains(t, bodyThu, "(Locked)")
	assert.Contains(t, bodyThu, fixture.slip.Name)
	assert.Contains(t, bodyThu, "(1/2 Picks)")
	assert.Contains(t, bodyThu, fmt.Sprintf(`href="/slips/%d"`, fixture.slip.ID))
	assert.Contains(t, bodyThu, fmt.Sprintf(`href="/slips/%d"`, thuSlip.ID))
	assert.Contains(t, bodyThu, fmt.Sprintf(`href="/slips/new?week=%d"`, fixture.week.ID))
	assert.Contains(t, bodyThu, "+ New slip")
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
	assert.NotContains(t, bodySun, `id="slip-switcher"`)
	assert.Contains(t, bodySun, "data-sibling-switcher")
	assert.Contains(t, bodySun, "Full Week")
	assert.Contains(t, bodySun, "(Locked)")
	assert.Contains(t, bodySun, "(1/2 Picks)")
	assert.Contains(t, bodySun, fmt.Sprintf(`href="/slips/new?week=%d"`, fixture.week.ID))
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

	// Thanksgiving week: Thu, Fri, Sun, Mon upcoming games.
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return now })

	for _, tc := range []struct {
		id   string
		kick time.Time
	}{
		{"g-thu", time.Date(2026, 11, 26, 17, 30, 0, 0, time.UTC)},
		{"g-fri", time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)},
		{"g-sun", time.Date(2026, 11, 29, 18, 0, 0, 0, time.UTC)},
		{"g-mon", time.Date(2026, 12, 1, 1, 15, 0, 0, time.UTC)},
	} {
		_, err := fixture.store.UpsertGame(t.Context(), db.Game{
			ID: tc.id, WeekID: fixture.week.ID, HomeTeam: "BUF", AwayTeam: "KC",
			HomeName: "Buffalo Bills", AwayName: "Kansas City Chiefs",
			HomeESPNID: "2", AwayESPNID: "12", Kickoff: tc.kick, Status: db.GameScheduled, UpdatedAt: now,
		})
		require.NoError(t, err)
	}

	// Default one-off mode renders dynamic weekday shortcuts and pre-selects Sunday games.
	reqDefault := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/new", nil)
	recDefault := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recDefault, reqDefault)
	require.Equal(t, http.StatusOK, recDefault.Code)
	bodyDefault := recDefault.Body.String()
	assert.Contains(t, bodyDefault, `data-slate-shortcut="thursday"`)
	assert.Contains(t, bodyDefault, `data-slate-shortcut="friday"`)
	assert.Contains(t, bodyDefault, `data-slate-shortcut="sunday"`)
	assert.Contains(t, bodyDefault, `data-slate-shortcut="monday"`)
	assert.NotContains(t, bodyDefault, `data-slate-shortcut="saturday"`)
	assert.Contains(t, bodyDefault, `data-slate-shortcut="all"`)
	assert.Contains(t, bodyDefault, `data-slate-shortcut="clear"`)
	assert.Contains(t, bodyDefault, "1 game selected")

	// ?mode=recurring&scope=thursday pre-selects recurring mode and Thursday Slate.
	reqThu := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/slips/new?mode=recurring&scope=thursday", nil,
	)
	recThu := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recThu, reqThu)
	require.Equal(t, http.StatusOK, recThu.Code)
	bodyThu := recThu.Body.String()
	assert.Contains(t, bodyThu, `value="recurring" data-slip-mode-input`)
	assert.Contains(t, bodyThu, `<option value="thursday" selected>Thursday Slate</option>`)

	// GET /slips/new/games returns partial for the selected week.
	reqGames := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, fmt.Sprintf("/slips/new/games?week_id=%d", fixture.week.ID), nil,
	)
	recGames := httptest.NewRecorder()
	fixture.handler.ServeHTTP(recGames, reqGames)
	require.Equal(t, http.StatusOK, recGames.Code)
	assert.Contains(t, recGames.Body.String(), "All Thursday")
	assert.Contains(t, recGames.Body.String(), "g-thu")
}

func TestCreateSlipWithSlateScope(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return now })

	for _, tc := range []struct {
		id, away, home string
		kick           time.Time
	}{
		{"g-sun-1", "KC", "BUF", time.Date(2026, 11, 29, 18, 0, 0, 0, time.UTC)},
		{"g-sun-2", "GB", "DET", time.Date(2026, 11, 29, 18, 0, 0, 0, time.UTC)},
		{"g-sun-3", "PHI", "DAL", time.Date(2026, 11, 29, 21, 25, 0, 0, time.UTC)},
	} {
		_, err := fixture.store.UpsertGame(t.Context(), db.Game{
			ID: tc.id, WeekID: fixture.week.ID, HomeTeam: tc.home, AwayTeam: tc.away,
			HomeName: tc.home, AwayName: tc.away,
			HomeESPNID: "1", AwayESPNID: "2", Kickoff: tc.kick, Status: db.GameScheduled, UpdatedAt: now,
		})
		require.NoError(t, err)
	}

	// 1. One-off slip with 2 selected Sunday games creates SlateScopeCustom.
	resOneOff := postForm(t, fixture.handler, "/slips", url.Values{
		"mode":            {"one_off"},
		"week_id":         {strconv.FormatInt(fixture.week.ID, 10)},
		"name":            {"Two Sunday Games"},
		"legs_per_member": {"2"},
		"member_id":       {strconv.FormatInt(fixture.user.ID, 10)},
		"game_id":         {"g-sun-1", "g-sun-2"},
	})
	require.Equal(t, http.StatusOK, resOneOff.Code, resOneOff.Body.String())
	locOneOff := resOneOff.Header().Get("Hx-Redirect")
	require.NotEmpty(t, locOneOff)
	idOneOff, err := strconv.ParseInt(strings.TrimPrefix(locOneOff, "/slips/"), 10, 64)
	require.NoError(t, err)
	customSlip, err := fixture.store.SlipByID(t.Context(), idOneOff)
	require.NoError(t, err)
	assert.Equal(t, db.SlateScopeCustom, customSlip.SlateScope)
	assert.Equal(t, []string{"g-sun-1", "g-sun-2"}, customSlip.GameIDs)

	// Pick board shows only the 2 selected games, and leg for 3rd game is rejected (422).
	recPick := httptest.NewRecorder()
	reqPick := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d/pick", idOneOff), nil)
	fixture.handler.ServeHTTP(recPick, reqPick)
	require.Equal(t, http.StatusOK, recPick.Code)
	assert.Contains(t, recPick.Body.String(), "KC @ BUF")
	assert.Contains(t, recPick.Body.String(), "GB @ DET")
	assert.NotContains(t, recPick.Body.String(), "PHI @ DAL")

	recOutside := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", idOneOff), url.Values{
		"game_id":     {"g-sun-3"},
		"market_type": {"ML"},
		"selection":   {"DAL ML"},
		"odds":        {"-110"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, recOutside.Code)

	// 2. Recurring mode creates GroupSlipSchedule and redirects to /groups/{id}/week
	// when no matching Thursday game is upcoming.
	resRec := postForm(t, fixture.handler, "/slips", url.Values{
		"mode":            {"recurring"},
		"name":            {"Thursday Night"},
		"slate_scope":     {"thursday"},
		"legs_per_member": {"2"},
	})
	require.Equal(t, http.StatusOK, resRec.Code, resRec.Body.String())
	assert.Equal(t, fmt.Sprintf("/groups/%d/week", fixture.group.ID), resRec.Header().Get("Hx-Redirect"))

	// 3. Invalid slate_scope in recurring mode triggers 422.
	resInvalid := postForm(t, fixture.handler, "/slips", url.Values{
		"mode":            {"recurring"},
		"name":            {"Invalid Scope Slip"},
		"slate_scope":     {"bogus_scope"},
		"legs_per_member": {"2"},
	})
	require.Equal(t, http.StatusUnprocessableEntity, resInvalid.Code)
	assert.Contains(t, resInvalid.Body.String(), db.ErrSlateScopeInvalid.Error())
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
	require.NoError(t, fixture.store.RecordSync(t.Context(), "nflverse:season:2026", now, nil))
	var nflRows []db.NFLversePlayerWeek
	for w := 1; w <= 5; w++ {
		nflRows = append(nflRows,
			db.NFLversePlayerWeek{
				ESPNID: "p-allen", Season: 2026, SeasonType: 2, WeekNum: w,
				Team: "BUF", Opponent: "MIA", PlayerName: "Josh Allen", Position: "QB",
				PassAtt: 34, PassYds: 295, PassTDs: 2, ExpPassYds: 285, OffensePct: 0.98, TotalEPA: 9.0,
				UpdatedAt: now,
			},
			db.NFLversePlayerWeek{
				ESPNID: "p-cook", Season: 2026, SeasonType: 2, WeekNum: w,
				Team: "BUF", Opponent: "MIA", PlayerName: "James Cook", Position: "RB",
				Carries: 18, RushYds: 92, RushTDs: 1, CarryShare: 0.68, ExpRushYds: 86, OffensePct: 0.72, TotalEPA: 3.5,
				UpdatedAt: now,
			},
		)
	}
	_, err := fixture.store.UpsertNFLversePlayerWeeks(t.Context(), nflRows)
	require.NoError(t, err)

	existingOdds, err := fixture.store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	hasGameLines := false
	for _, row := range existingOdds {
		if !db.IsPropRow(row) {
			hasGameLines = true
			break
		}
	}

	var oddsRows []db.Odds
	if !hasGameLines {
		oddsRows = append(oddsRows,
			db.Odds{
				GameID:     game.ID,
				Book:       "pinnacle",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-4.5),
				Price:      -110,
				FetchedAt:  now,
			},
			db.Odds{
				GameID:     game.ID,
				Book:       "pinnacle",
				MarketType: "SPREAD",
				Team:       "MIA",
				Side:       "TEAM",
				Line:       new(4.5),
				Price:      -110,
				FetchedAt:  now,
			},
			db.Odds{
				GameID:     game.ID,
				Book:       "pinnacle",
				MarketType: "TOTAL",
				Side:       "OVER",
				Line:       new(49.5),
				Price:      -110,
				FetchedAt:  now,
			},
			db.Odds{
				GameID:     game.ID,
				Book:       "pinnacle",
				MarketType: "TOTAL",
				Side:       "UNDER",
				Line:       new(49.5),
				Price:      -110,
				FetchedAt:  now,
			},
		)
	}
	oddsRows = append(oddsRows, []db.Odds{
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -155,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(245.5),
			Price:      125,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "OVER",
			Line:       new(65.5),
			Price:      -155,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "pinnacle",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "UNDER",
			Line:       new(65.5),
			Price:      125,
			FetchedAt:  now,
		},
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
	}...)
	_, err = fixture.store.UpsertOdds(t.Context(), oddsRows)
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

func TestSlipHubRendersImpliedChancesAndOmitsEmptyOrVoided(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	addGameOdds(t, fixture.store, game.ID)
	_, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("draftkings"))
	require.NoError(t, err)

	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		fixture.handler.ServeHTTP(rec, req)
		return rec
	}

	// Empty slip omits implied chances.
	emptyPayout := get(fmt.Sprintf("/slips/%d/payout?stake=10&boost=0", fixture.slip.ID))
	require.Equal(t, http.StatusOK, emptyPayout.Code)
	assert.Contains(t, emptyPayout.Body.String(), "Add a leg to see a payout.")
	assert.NotContains(t, emptyPayout.Body.String(), "Implied chance from combined odds")
	assert.NotContains(t, emptyPayout.Body.String(), "Implied chance from reference odds")

	// Add two legs: -110 (52%) and +150 (40%).
	resp1 := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"odds":        {"-110"},
	})
	require.Equal(t, http.StatusOK, resp1.Code, resp1.Body.String())

	resp2 := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
		"game_id":     {game.ID},
		"market_type": {"TOTAL"},
		"side":        {"OVER"},
		"line":        {"47.5"},
		"odds":        {"+150"},
	})
	require.Equal(t, http.StatusOK, resp2.Code, resp2.Body.String())

	hubResp := get(fmt.Sprintf("/slips/%d", fixture.slip.ID))
	require.Equal(t, http.StatusOK, hubResp.Code)
	hubBody := hubResp.Body.String()
	assert.Contains(t, hubBody, "(52%)")
	assert.Contains(t, hubBody, "(40%)")
	assert.Contains(t, hubBody, "21% chance")

	// Payout endpoint renders both combined and reference implied chances.
	payoutResp := get(fmt.Sprintf("/slips/%d/payout?stake=10&boost=0", fixture.slip.ID))
	require.Equal(t, http.StatusOK, payoutResp.Code)
	payoutBody := payoutResp.Body.String()
	assert.Contains(t, payoutBody, "Implied chance from combined odds")
	assert.Contains(t, payoutBody, "21% chance")
	assert.Contains(t, payoutBody, "Implied chance from reference odds")

	// Voiding all legs drops CombinedOdds to 1.00x and omits payout implied chances.
	legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	for _, leg := range legs {
		voidResp := putForm(t, fixture.handler, fmt.Sprintf("/legs/%d/result", leg.ID), url.Values{
			"result": {"VOID"},
		})
		require.Equal(t, http.StatusOK, voidResp.Code, voidResp.Body.String())
	}

	voidPayout := get(fmt.Sprintf("/slips/%d/payout?stake=10&boost=0", fixture.slip.ID))
	require.Equal(t, http.StatusOK, voidPayout.Code)
	voidBody := voidPayout.Body.String()
	assert.Contains(t, voidBody, "1.00x")
	assert.NotContains(t, voidBody, "Implied chance from combined odds")
	assert.NotContains(t, voidBody, "Implied chance from reference odds")
	assert.NotContains(t, voidBody, "chance")
}

func TestSlipHubHighlightsRenderImpliedChances(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedAGradeBoardForHubTest(t, fixture, game)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/slips/%d", fixture.slip.ID), nil)
	fixture.handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "data-highlight-leg")
	assert.Contains(t, body, "data-highlight-parlay")
	// Single-leg highlights and combo leg items average Pinnacle (-155) and DraftKings (-105) -> -125 (56%).
	assert.Contains(t, body, "(56%)")
	// 2-leg combo of -125 and -125 is +224 -> (31%).
	assert.Contains(t, body, "(31%)")
}

func TestSlipHubLiveProgressStates(t *testing.T) {
	t.Parallel()

	t.Run("scheduled", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		game := addGame(t, fixture, gameKickoff())
		resp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
			"game_id":     {game.ID},
			"market_type": {"ML"},
			"team":        {"BUF"},
			"side":        {"TEAM"},
			"odds":        {"-110"},
		})
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.NotContains(t, body, "data-slip-live-progress")
		assert.NotContains(t, body, "data-leg-live-progress")
	})

	t.Run("live", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 3)
		liveGame := addGame(t, fixture, gameKickoff())
		schedGame, err := fixture.store.UpsertGame(t.Context(), db.Game{
			ID:         "sched-game-1",
			WeekID:     fixture.week.ID,
			HomeTeam:   "DAL",
			AwayTeam:   "PHI",
			HomeName:   "Dallas Cowboys",
			AwayName:   "Philadelphia Eagles",
			HomeESPNID: "6",
			AwayESPNID: "21",
			Kickoff:    gameKickoff(),
			Status:     db.GameScheduled,
		})
		require.NoError(t, err)

		// Add legs before the game kicks off.
		for _, form := range []url.Values{
			{
				"game_id":     {liveGame.ID},
				"market_type": {"SPREAD"},
				"team":        {"BUF"},
				"side":        {"TEAM"},
				"line":        {"-3.5"},
				"odds":        {"-110"},
			},
			{
				"game_id":     {liveGame.ID},
				"market_type": {"PASS_YDS"},
				"team":        {"BUF"},
				"player_id":   {"3918298"},
				"player_name": {"Josh Allen"},
				"side":        {"OVER"},
				"line":        {"245.5"},
				"odds":        {"-110"},
			},
			{
				"game_id":     {schedGame.ID},
				"market_type": {"ML"},
				"team":        {"DAL"},
				"side":        {"TEAM"},
				"odds":        {"+120"},
			},
		} {
			resp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), form)
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		}

		// Transition game to LIVE with score, clock, and box score.
		liveGame.Status = db.GameLive
		liveGame.HomeScore = new(21)
		liveGame.AwayScore = new(17)
		liveGame.Period = new(3)
		liveGame.Clock = new("8:42")
		_, err = fixture.store.UpsertGame(t.Context(), liveGame)
		require.NoError(t, err)

		boxBytes, err := json.Marshal(espn.BoxScore{
			HasStats: true,
			Players: []espn.PlayerBoxStats{
				{
					PlayerID:    "3918298",
					DisplayName: "Josh Allen",
					TeamAbbrev:  "BUF",
					Passing:     espn.PassingStats{PassingYards: 210},
				},
			},
		})
		require.NoError(t, err)
		_, err = fixture.store.SetGameSummary(t.Context(), liveGame.ID, "", new(string(boxBytes)))
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()

		assert.Contains(t, body, `data-slip-live-progress data-live-state="LIVE"`)
		assert.Contains(t, body, `role="progressbar"`)
		assert.Contains(t, body, "0/3 hit · 2 live")
		assert.Contains(t, body, "Live chance ")
		assert.Contains(t, body, "Pre-game ")
		assert.Equal(t, 2, strings.Count(body, "data-leg-live-progress"))
		assert.Contains(t, body, "BUF 21-17 (-3.5) · Q3 8:42")
		assert.Contains(t, body, "210 / 245.5 yds · Q3 8:42")
	})

	t.Run("cashed", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		game := addGame(t, fixture, gameKickoff())
		for _, form := range []url.Values{
			{"game_id": {game.ID}, "market_type": {"ML"}, "team": {"BUF"}, "side": {"TEAM"}, "odds": {"-110"}},
			{"game_id": {game.ID}, "market_type": {"TOTAL"}, "side": {"OVER"}, "line": {"47.5"}, "odds": {"-110"}},
		} {
			resp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), form)
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		}
		legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
		require.NoError(t, err)
		require.Len(t, legs, 2)

		require.Equal(
			t,
			http.StatusOK,
			putForm(t, fixture.handler, fmt.Sprintf("/legs/%d/result", legs[0].ID), url.Values{"result": {"WON"}}).Code,
		)
		require.Equal(
			t,
			http.StatusOK,
			putForm(
				t,
				fixture.handler,
				fmt.Sprintf("/legs/%d/result", legs[1].ID),
				url.Values{"result": {"PUSH"}},
			).Code,
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()

		assert.Contains(t, body, `data-slip-live-progress data-live-state="CASHED"`)
		assert.Contains(t, body, "1/1 hit")
		assert.Contains(t, body, "Live chance 100%")
		assert.Contains(t, body, `aria-valuenow="100"`)
	})

	t.Run("busted", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		game := addGame(t, fixture, gameKickoff())
		for _, form := range []url.Values{
			{"game_id": {game.ID}, "market_type": {"ML"}, "team": {"BUF"}, "side": {"TEAM"}, "odds": {"-110"}},
			{"game_id": {game.ID}, "market_type": {"TOTAL"}, "side": {"OVER"}, "line": {"47.5"}, "odds": {"-110"}},
		} {
			resp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), form)
			require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		}
		legs, err := fixture.store.LegsForSlip(t.Context(), fixture.slip.ID)
		require.NoError(t, err)
		require.Len(t, legs, 2)

		require.Equal(
			t,
			http.StatusOK,
			putForm(t, fixture.handler, fmt.Sprintf("/legs/%d/result", legs[0].ID), url.Values{"result": {"WON"}}).Code,
		)
		require.Equal(
			t,
			http.StatusOK,
			putForm(
				t,
				fixture.handler,
				fmt.Sprintf("/legs/%d/result", legs[1].ID),
				url.Values{"result": {"LOST"}},
			).Code,
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			fmt.Sprintf("/slips/%d", fixture.slip.ID),
			nil,
		)
		fixture.handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()

		assert.Contains(t, body, `data-slip-live-progress data-live-state="BUSTED"`)
		assert.Contains(t, body, "1/2 hit · 1 lost")
		assert.Contains(t, body, "Live chance 0%")
	})
}

func TestSlipPanelSSERefreshesLiveProgress(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	game := addGame(t, fixture, gameKickoff())
	resp := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), url.Values{
		"game_id":     {game.ID},
		"market_type": {"TOTAL"},
		"side":        {"OVER"},
		"line":        {"47.5"},
		"odds":        {"-110"},
	})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	server := httptest.NewServer(fixture.handler)
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("%s/slips/%d/events", server.URL, fixture.slip.ID),
		nil,
	)
	require.NoError(t, err)
	stream, err := server.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, stream.StatusCode)
	t.Cleanup(func() { assert.NoError(t, stream.Body.Close()) })

	// Simulate a live score update crossing the total line and publishing slip-updated.
	game.Status = db.GameLive
	game.HomeScore = new(28)
	game.AwayScore = new(24)
	game.Period = new(4)
	game.Clock = new("2:10")
	_, err = fixture.store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	fixture.hub.Publish(fmt.Sprintf("slip:%d", fixture.slip.ID))

	assert.Equal(t, "slip-updated", waitForEvent(t, stream.Body))

	// Fetch the refreshed panel partial that HTMX requests on slip-updated.
	panelRec := httptest.NewRecorder()
	panelReq := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/panel", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(panelRec, panelReq)
	require.Equal(t, http.StatusOK, panelRec.Code)
	panelBody := panelRec.Body.String()

	assert.Contains(t, panelBody, `data-slip-live-progress data-live-state="CASHED"`)
	assert.Contains(t, panelBody, "52 / 47.5 pts · Q4 2:10")
	assert.Contains(t, panelBody, "Live chance 100%")
}
