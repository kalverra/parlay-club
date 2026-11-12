package web_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

// landingFixture wires the routing screens with a fixed active group and a
// record of every group switch the handlers asked for.
type landingFixture struct {
	handler    http.Handler
	store      *db.Store
	user       db.User
	first      db.Group
	second     db.Group
	week       db.Week
	selections []int64
	clock      func() time.Time
}

// newLandingFixture builds a person in two groups with one slip in the first.
func newLandingFixture(t *testing.T) *landingFixture {
	t.Helper()

	return newLandingFixtureWithWeek(t, true)
}

// newLandingFixtureWithWeek builds the landing fixture with or without a
// stored week, so the create form's no-week state can be tested.
func newLandingFixtureWithWeek(t *testing.T, withWeek bool) *landingFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	first, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	second, err := store.Create(t.Context(), user.ID, "Lake House")
	require.NoError(t, err)

	fixture := &landingFixture{
		store:  store,
		user:   user,
		first:  first,
		second: second,
	}
	if withWeek {
		week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
		require.NoError(t, err)
		fixture.week = week
		scheds, _ := store.GroupSlipSchedules(t.Context(), first.ID)
		var sID *int64
		if len(scheds) > 0 {
			sID = &scheds[0].ID
		}
		_, err = store.CreateSlip(
			t.Context(),
			first.ID,
			week.ID,
			sID,
			"Week 9",
			db.SlateScopeSundayMain,
			2,
			[]int64{user.ID},
		)
		require.NoError(t, err)
	}

	fixture.handler = fixture.build()
	return fixture
}

// build returns the app's routes with the first group active.
func (f *landingFixture) build() http.Handler {
	renderer, err := webui.NewRenderer(assets.Templates, false)
	if err != nil {
		panic(err)
	}
	hub := &live.Hub{}

	return webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return f.user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{
				{ID: f.first.ID, Name: f.first.Name, Current: true},
				{ID: f.second.ID, Name: f.second.Name},
			}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return f.first, true
		},
		SetGroup: func(_ http.ResponseWriter, id int64) {
			f.selections = append(f.selections, id)
		},
		Store:  f.store,
		Slips:  slips.New(f.store, hub, nil),
		Stream: live.NewStream(hub, func(int64) bool { return false }),
		Now:    f.clock,
	})
}

// get serves one GET and returns the recorder.
func (f *landingFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return recorder
}

// getHTMX serves one GET with the HTMX header.
func (f *landingFixture) getHTMX(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	request.Header.Set("Hx-Request", "true")
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)
	return recorder
}

func TestLandingListsEveryGroup(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)

	recorder := fixture.get(t, "/")
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	assert.Contains(t, body, "Sunday Parlay Club")
	assert.Contains(t, body, "Lake House")
	assert.Contains(t, body, fmt.Sprintf(`href="/groups/%d/week"`, fixture.first.ID))
	assert.Contains(t, body, fmt.Sprintf(`href="/groups/%d/week"`, fixture.second.ID))
	assert.Contains(t, body, "Pick 2 more", "the card shows the viewer's next step")
	assert.Contains(t, body, "No slip yet", "a group with no slip says so")
	assert.Contains(t, body, "data-refresh-on-visible", "the list refetches when the tab is visible")
	assert.NotContains(t, body, "data-live-url", "the group list holds no stream open")
	assert.Empty(t, fixture.selections, "landing on the list chooses no group")
}

func TestLandingSendsOneGroupStraightToThisWeek(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	require.NoError(t, fixture.store.Leave(t.Context(), fixture.second.ID, fixture.user.ID))

	recorder := fixture.get(t, "/")
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, fmt.Sprintf("/groups/%d/week", fixture.first.ID), recorder.Header().Get("Location"))
}

func TestLandingRefetchesTheListForHTMX(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	require.NoError(t, fixture.store.Leave(t.Context(), fixture.second.ID, fixture.user.ID))

	recorder := fixture.getHTMX(t, "/")
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	assert.Contains(t, body, `id="group-list"`)
	assert.Contains(t, body, "Sunday Parlay Club")
	assert.NotContains(t, body, "<title>", "an HTMX refresh returns the list, not the page")
}

func TestGroupWeekSwitchesTheGroupAndOpensTheNewestSlip(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	scheds, err := fixture.store.GroupSlipSchedules(t.Context(), fixture.second.ID)
	require.NoError(t, err)
	var sID *int64
	if len(scheds) > 0 {
		sID = &scheds[0].ID
	}
	created, err := fixture.store.CreateSlip(
		t.Context(), fixture.second.ID, fixture.week.ID, sID, "Lake House Week 9", db.SlateScopeSundayMain, 2,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)

	recorder := fixture.get(t, fmt.Sprintf("/groups/%d/week", fixture.second.ID))
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, fmt.Sprintf("/slips/%d", created.ID), recorder.Header().Get("Location"))
	assert.Equal(t, []int64{fixture.second.ID}, fixture.selections)
}

func TestGroupWeekRendersMultiSlipViewWhenMultipleSlipsExist(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	fixture.clock = func() time.Time { return now }
	fixture.handler = fixture.build()

	thuKick := time.Date(2026, 11, 5, 20, 15, 0, 0, time.UTC)
	sunKick := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	_, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401873037",
		WeekID:     fixture.week.ID,
		HomeTeam:   "BAL",
		AwayTeam:   "CIN",
		HomeName:   "Baltimore Ravens",
		AwayName:   "Cincinnati Bengals",
		HomeESPNID: "33",
		AwayESPNID: "4",
		Kickoff:    thuKick,
		Status:     db.GameScheduled,
		UpdatedAt:  thuKick.Add(-24 * time.Hour),
	})
	require.NoError(t, err)
	_, err = fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401873038",
		WeekID:     fixture.week.ID,
		HomeTeam:   "KC",
		AwayTeam:   "BUF",
		HomeName:   "Kansas City Chiefs",
		AwayName:   "Buffalo Bills",
		HomeESPNID: "12",
		AwayESPNID: "2",
		Kickoff:    sunKick,
		Status:     db.GameScheduled,
		UpdatedAt:  sunKick.Add(-24 * time.Hour),
	})
	require.NoError(t, err)

	thuSlip, err := fixture.store.CreateSlip(
		t.Context(), fixture.second.ID, fixture.week.ID, nil, "Thursday Night", db.SlateScopeThursday, 2,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)

	sunSlip, err := fixture.store.CreateSlip(
		t.Context(), fixture.second.ID, fixture.week.ID, nil, "Sunday Main", db.SlateScopeSundayMain, 2,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)

	recorder := fixture.get(t, fmt.Sprintf("/groups/%d/week", fixture.second.ID))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "This week's slips")
	assert.Contains(t, body, "Thursday Night")
	assert.Contains(t, body, "Sunday Main")
	assert.Contains(t, body, "Sunday Slate")
	assert.Contains(t, body, fmt.Sprintf(`/slips/%d`, thuSlip.ID))
	assert.Contains(t, body, fmt.Sprintf(`/slips/%d`, sunSlip.ID))
	assert.Contains(t, body, "+ Add one-off slip")
	assert.Contains(t, body, fmt.Sprintf(`/slips/new?week=%d`, fixture.week.ID))
	assert.Contains(t, body, "Pick 2 more")
	assert.Contains(t, body, "1 member · 0/2 picks")
	assert.Contains(t, body, thuKick.Format(time.RFC3339))
	assert.Equal(t, []int64{fixture.second.ID}, fixture.selections)
}

func TestGroupWeekAutoCreatesMultipleScheduledSlipsAndRendersView(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	_, err := fixture.store.CreateGroupSlipSchedule(
		t.Context(),
		fixture.second.ID,
		"Sunday Main",
		db.SlateScopeSundayMain,
		2,
	)
	require.NoError(t, err)
	_, err = fixture.store.CreateGroupSlipSchedule(
		t.Context(),
		fixture.second.ID,
		"Thursday Night",
		db.SlateScopeThursday,
		2,
	)
	require.NoError(t, err)

	recorder := fixture.get(t, fmt.Sprintf("/groups/%d/week", fixture.second.ID))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "This week's slips")
	assert.Contains(t, body, "Thursday Night")
	assert.Contains(t, body, "Sunday Main")
	assert.Contains(t, body, "Sunday Slate")
	assert.Contains(t, body, "+ Add one-off slip")
	assert.Contains(t, body, "Recurring schedules")
	assert.Contains(t, body, fmt.Sprintf(`/groups/%d/schedules`, fixture.second.ID))
	assert.Contains(t, body, "Save schedule")
	assert.Equal(t, []int64{fixture.second.ID}, fixture.selections)
}

func TestGroupWeekShowsTheCreateFormWithNoSlip(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	schedules, err := fixture.store.GroupSlipSchedules(t.Context(), fixture.second.ID)
	require.NoError(t, err)
	for _, s := range schedules {
		_, err := fixture.store.SetGroupSlipScheduleActive(t.Context(), s.ID, false)
		require.NoError(t, err)
	}

	recorder := fixture.get(t, fmt.Sprintf("/groups/%d/week", fixture.second.ID))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "Start Week 9’s slip", "the create screen names the week")
	assert.Contains(t, body, "No slip for Week 9 yet.", "the empty state explains why the form is here")
	assert.Contains(t, body, "Create slip")
	assert.Contains(t, body, ">Lake House</span>", "the top bar shows the group the page acts in")
	assert.Contains(t, body, fmt.Sprintf(`href="/groups/%d/week"`, fixture.second.ID))
	assert.Equal(t, []int64{fixture.second.ID}, fixture.selections)
}

func TestGroupWeekCreateFormWelcomesTheOnlyMember(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	schedules, err := fixture.store.GroupSlipSchedules(t.Context(), fixture.second.ID)
	require.NoError(t, err)
	for _, s := range schedules {
		_, err := fixture.store.SetGroupSlipScheduleActive(t.Context(), s.ID, false)
		require.NoError(t, err)
	}

	body := html.UnescapeString(
		fixture.get(t, fmt.Sprintf("/groups/%d/week", fixture.second.ID)).Body.String())

	assert.Contains(t, body, "It’s just you for now.",
		"a first run in a solo group says who is picking")
	assert.Contains(t, body, "Invite friends from Groups whenever you like.")
}

func TestGroupWeekCreateFormWithoutAWeekSaysSo(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixtureWithWeek(t, false)

	recorder := fixture.get(t, fmt.Sprintf("/groups/%d/week", fixture.second.ID))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, "Start a slip")
	assert.Contains(t, body, "No week is loaded yet. Add a past week below to start a slip.")
	assert.NotContains(t, body, `name="week_id"`, "an empty week picker is not offered")
	assert.Contains(t, body, "Or add a past week", "the typed-week fields still start a slip")
}

func TestGroupWeekRefusesANonMember(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	stranger, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	other, err := fixture.store.Create(t.Context(), stranger.ID, "Strangers")
	require.NoError(t, err)

	recorder := fixture.get(t, fmt.Sprintf("/groups/%d/week", other.ID))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Empty(t, fixture.selections, "a refused group is never made active")
}

func TestOpeningASlipFromAnotherGroupSwitchesTheActiveGroup(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	scheds, err := fixture.store.GroupSlipSchedules(t.Context(), fixture.second.ID)
	require.NoError(t, err)
	var sID *int64
	if len(scheds) > 0 {
		sID = &scheds[0].ID
	}
	created, err := fixture.store.CreateSlip(
		t.Context(), fixture.second.ID, fixture.week.ID, sID, "Lake House Week 9", db.SlateScopeSundayMain, 2,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)

	recorder := fixture.get(t, fmt.Sprintf("/slips/%d", created.ID))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	assert.Contains(t, body, "Lake House Week 9")
	assert.Equal(t, []int64{fixture.second.ID}, fixture.selections)
	assert.Contains(t, body, fmt.Sprintf(`href="/groups/%d/week"`, fixture.second.ID))
	assert.Contains(t, body, fmt.Sprintf(`href="/slips/%d/pick"`, created.ID), "the hub keeps the pick board reachable")
}

func TestOpeningASlipOutsideEveryGroupIs404(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	stranger, err := fixture.store.CreateUserWithIdentity(
		t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	other, err := fixture.store.Create(t.Context(), stranger.ID, "Strangers")
	require.NoError(t, err)
	created, err := fixture.store.CreateSlip(
		t.Context(), other.ID, fixture.week.ID, nil, "Strangers Week 9", db.SlateScopeSundayMain, 2,
		[]int64{stranger.ID},
	)
	require.NoError(t, err)

	recorder := fixture.get(t, fmt.Sprintf("/slips/%d", created.ID))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Empty(t, fixture.selections, "a slip outside every group switches nothing")
}

func TestHubKeepsTheThreeTabBar(t *testing.T) {
	t.Parallel()

	fixture := newLandingFixture(t)
	groupSlips, err := fixture.store.SlipsForWeek(t.Context(), fixture.first.ID, fixture.week.ID)
	require.NoError(t, err)
	require.NotEmpty(t, groupSlips)

	recorder := fixture.get(t, fmt.Sprintf("/slips/%d", groupSlips[0].ID))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	for _, label := range []string{"Groups", "This week", "Record Book"} {
		assert.Contains(t, body, ">"+label+"</a")
	}
	assert.Contains(t, body, fmt.Sprintf(`href="/groups/%d/week"`, fixture.first.ID))
	assert.Equal(t, 1, strings.Count(body, `aria-current="page"`))
}
