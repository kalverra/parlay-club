package web_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
