package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/groups"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/refresh"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// passthrough stands in for the group scope in tests that do not need one.
func passthrough(next http.Handler) http.Handler { return next }

func TestKickOnPageRunsOnlyForFullPageGETs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		want    int
	}{
		{name: "slip page", method: http.MethodGet, path: "/", want: 1},
		{name: "games page", method: http.MethodGet, path: "/games", want: 1},
		{
			name:   "htmx partial",
			method: http.MethodGet, path: "/slips/7/panel",
			headers: map[string]string{"HX-Request": "true"},
			want:    0,
		},
		{
			name:   "sse stream",
			method: http.MethodGet, path: "/slips/7/events",
			headers: map[string]string{"Accept": "text/event-stream"},
			want:    0,
		},
		{
			name:   "sse path without an accept header",
			method: http.MethodGet, path: "/slips/7/events",
			want: 0,
		},
		{name: "form post", method: http.MethodPost, path: "/groups", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			handler := kickOnPage(func() { calls++ })(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}))

			request := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			for key, value := range tt.headers {
				request.Header.Set(key, value)
			}
			handler.ServeHTTP(httptest.NewRecorder(), request)

			assert.Equal(t, tt.want, calls)
		})
	}
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	mux := newMux(
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		newHealthzHandler(database, zerolog.Nop()),
		nil,
		nil,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	healthy := httptest.NewRecorder()
	mux.ServeHTTP(healthy, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, healthy.Code)
	assert.Equal(t, "ok", healthy.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", healthy.Header().Get("Content-Type"))

	require.NoError(t, database.Close())

	unhealthy := httptest.NewRecorder()
	mux.ServeHTTP(
		unhealthy,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil),
	)

	assert.Equal(t, http.StatusServiceUnavailable, unhealthy.Code)
	assert.Equal(t, "unavailable", unhealthy.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", unhealthy.Header().Get("Content-Type"))
	assert.NotContains(t, unhealthy.Body.String(), "database is closed",
		"the probe must not leak the SQLite error")
}

func TestMuxProtectsShell(t *testing.T) {
	t.Parallel()

	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		nil,
		nil,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil))

	assert.Equal(t, http.StatusFound, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Location"))
}

func TestMuxServesStaticWithoutAuth(t *testing.T) {
	t.Parallel()

	static := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		http.NotFoundHandler(),
		static,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		nil,
		nil,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/static/app.css",
		nil,
	))

	assert.Equal(t, http.StatusNoContent, recorder.Code, "static files load on the login page")
}

func TestMuxServesPWAWithoutAuth(t *testing.T) {
	t.Parallel()

	mux := newMux(
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		nil,
		nil,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	tests := []struct {
		path        string
		contentType string
	}{
		{path: "/manifest.webmanifest", contentType: "application/manifest+json; charset=utf-8"},
		{path: "/sw.js", contentType: "text/javascript; charset=utf-8"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))

			assert.Equal(t, http.StatusOK, recorder.Code, "the browser fetches this before sign-in")
			assert.Equal(t, test.contentType, recorder.Header().Get("Content-Type"))
			assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
			assert.Empty(t, recorder.Header().Get("Hx-Redirect"), "an expired session never redirects the worker fetch")
			assert.Empty(t, recorder.Header().Get("Set-Cookie"), "no session is needed or issued")
			assert.NotEmpty(t, recorder.Body.String())
		})
	}
}

func TestMuxServesScreenshotsWithoutAuth(t *testing.T) {
	t.Parallel()

	screenshots := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		screenshots,
		http.NotFoundHandler(),
		nil,
		nil,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/screenshots/0123456789abcdef0123456789abcdef.jpg",
		nil,
	))

	assert.Equal(t, http.StatusNoContent, recorder.Code, "capability screenshot URLs are public")
}

// newTestHandlers builds auth handlers that need no database or templates.
func newTestHandlers() *auth.Handlers {
	cookies := auth.NewCookies("test-session-key-long-enough-for-anything", false)
	return auth.NewHandlers(zerolog.Nop(), nil, cookies, nil, nil, false)
}

func TestMuxScopesShellToActiveGroup(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	user, err := store.CreateUserWithIdentity(
		t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	cookies := auth.NewCookies("test-session-key-long-enough-for-anything", false)
	handlers := auth.NewHandlers(zerolog.Nop(), store, cookies, nil, nil, false)
	scope := groups.Scope(zerolog.Nop(), store, auth.UserFrom, cookies)

	var (
		got    groups.Group
		hasGot bool
	)
	shell := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, hasGot = groups.From(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	kicks := 0
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		nil,
		nil,
		handlers,
		nil,
		scope,
		func() { kicks++ },
		nil,
	)

	signed := httptest.NewRecorder()
	cookies.SetSession(signed, user.ID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	request.AddCookie(responseCookie(t, signed, auth.SessionCookie))
	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, 1, kicks, "a signed-in full-page GET starts a refresh pass")
	assert.True(t, hasGot, "the shell runs after the group scope")
	assert.Equal(t, group, got)

	chosen := responseCookie(t, recorder, auth.GroupCookie)
	chosenRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	chosenRequest.AddCookie(chosen)
	id, ok := cookies.GroupID(chosenRequest)
	require.True(t, ok)
	assert.Equal(t, group.ID, id, "the fallback group sticks in the cookie")
}

// responseCookie returns the named cookie a recorder set, failing the test when
// it is absent.
func responseCookie(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	name string,
) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	require.FailNow(t, "response set no cookie", "cookie %q missing", name)
	return nil
}

func TestLoggerWritesJSONInProd(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	logger := newLogger(config.Config{AppEnv: config.EnvProd, LogLevel: zerolog.InfoLevel}, &out)
	logger.Info().Msg("hello")

	assert.True(t, json.Valid(out.Bytes()), "prod logs must be JSON: %q", out.String())
	assert.Contains(t, out.String(), `"message":"hello"`)
}

func TestLoggerWritesConsoleInDev(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	logger := newLogger(config.Config{AppEnv: config.EnvDev, LogLevel: zerolog.DebugLevel}, &out)
	logger.Info().Msg("hello")

	assert.Contains(t, out.String(), "hello")
	assert.False(t, json.Valid(out.Bytes()), "dev logs should be readable, not JSON: %q", out.String())
}

func TestServeDrainsInflightRequestOnShutdown(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, zerolog.Nop(), server, listener, shutdownHooks{}) }()

	url := fmt.Sprintf("http://%s/slow", listener.Addr())
	response := make(chan error, 1)
	go func() {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		if err != nil {
			response <- err
			return
		}
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			response <- err
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			response <- fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
			return
		}
		response <- nil
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		t.Fatalf("serve returned before the in-flight request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Shutdown is still waiting on the handler, as it should.
	}

	close(release)
	require.NoError(t, <-response)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the in-flight request finished")
	}
}

// TestServeDrainsOpenStreamAndInflightRequestsOnShutdown holds one SSE stream,
// one slow request, and one refresh sync open, then cancels the serve context.
// The streams drain first without a bye, the slow request holds the HTTP drain,
// and the refresh drain cancels the sync only after the HTTP drain finishes.
func TestServeDrainsOpenStreamAndInflightRequestsOnShutdown(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	sync := &blockingScoreboard{entered: make(chan struct{}), left: make(chan struct{})}
	refresher := refresh.New(refresh.Deps{
		Store:      store,
		Scoreboard: sync,
		Logger:     zerolog.Nop(),
	})
	refresher.KickContext(t.Context())
	<-sync.entered

	var hub live.Hub
	stream := live.NewStream(&hub, nil)

	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		stream.ServeSlip(w, r, 7, 2026209)
	})
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, zerolog.Nop(), server, listener, shutdownHooks{
			beforeHTTP: stream.Shutdown,
			afterHTTP:  refresher.Shutdown,
		})
	}()

	baseURL := fmt.Sprintf("http://%s", listener.Addr())

	streamRequest, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/events", nil)
	require.NoError(t, err)
	streamResponse, err := http.DefaultClient.Do(streamRequest)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := streamResponse.Body.Close(); err != nil {
			t.Logf("close stream body: %v", err)
		}
	})
	require.Equal(t, http.StatusOK, streamResponse.StatusCode)
	assert.Equal(t, "text/event-stream", streamResponse.Header.Get("Content-Type"))

	response := make(chan error, 1)
	go func() {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/slow", nil)
		if err != nil {
			response <- err
			return
		}
		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			response <- err
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			response <- fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
			return
		}
		response <- nil
	}()

	<-started
	cancel()

	body, err := io.ReadAll(streamResponse.Body)
	require.NoError(t, err)
	assert.Empty(t, string(body), "shutdown must end the stream without a bye event")

	select {
	case err := <-done:
		t.Fatalf("serve returned before the slow request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
		// The drain still waits on the slow handler, as it should.
	}
	select {
	case <-sync.left:
		t.Fatal("the refresh drain ran before the HTTP drain finished")
	default:
		// The sync is still running, as it should be.
	}

	close(release)
	require.NoError(t, <-response)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the slow request finished")
	}
	select {
	case <-sync.left:
	default:
		t.Fatal("serve returned before the refresh drain finished")
	}
}

// TestServeReportsTheRefreshDrainError checks that a drain which outlives its
// deadline fails the serve, so a sync still writing is never mistaken for a
// clean stop.
func TestServeReportsTheRefreshDrainError(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, zerolog.Nop(), server, listener, shutdownHooks{
			afterHTTP: func(context.Context) error {
				return context.DeadlineExceeded
			},
		})
	}()

	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return")
	}
}

// blockingScoreboard holds its fetch until its context ends, so a test can see
// when the refresh drain cancels an in-flight sync.
type blockingScoreboard struct {
	entered chan struct{}
	left    chan struct{}
}

// Scoreboard signals that it started, waits for cancellation, and signals that
// it returned.
func (b *blockingScoreboard) Scoreboard(
	ctx context.Context,
	_ espn.ScoreboardParams,
) (espn.Scoreboard, error) {
	close(b.entered)
	<-ctx.Done()
	close(b.left)
	return espn.Scoreboard{}, ctx.Err()
}

// The fixed instant and kickoff the ingest wiring test works against: the
// Saturday before a Sunday game, inside both the weather and odds windows.
var (
	ingestKickoff = time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC)
	ingestNow     = time.Date(2026, time.November, 7, 16, 0, 0, 0, time.UTC)
)

// archiveTestAPIKey is the Odds API key the wiring test hands the client. It
// must never reach the archive.
const archiveTestAPIKey = "archive-test-key"

// TestRefreshArchivesEveryIngestCall drives one refresh pass through real
// ingest clients over fake sources. It proves the cmd/server client archives
// each call under the refresh source key without changing the parsed rows or
// the sync state the app already wrote.
func TestRefreshArchivesEveryIngestCall(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	client := newIngestClient(store)
	require.IsType(t, &archive.Transport{}, client.Transport, "the ingest client archives every call")
	assert.Equal(t, ingestTimeout, client.Timeout)

	service := refresh.New(refresh.Deps{
		Store:      store,
		Scoreboard: espn.NewClient(fakeESPN(t).URL, client),
		Forecast:   weather.NewClient(fakeWeather(t).URL, client),
		Lines:      odds.NewClient(fakeOdds(t).URL, archiveTestAPIKey, client),
		Now:        func() time.Time { return ingestNow },
		Logger:     zerolog.Nop(),
	})
	require.NoError(t, service.RunOnce(t.Context()))

	weatherKey := refresh.WeatherSource("401")
	summaryKey := refresh.SummarySource("401")
	homeRosterKey := refresh.RosterSource("2")
	awayRosterKey := refresh.RosterSource("15")
	waitForSyncStates(t, store,
		refresh.ScoreboardSource, weatherKey, summaryKey, homeRosterKey, awayRosterKey, refresh.OddsLinesSource)

	rows, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 7, "one row per outside call")

	keyCounts := make(map[string]int, 6)
	var eventsRow, linesRow db.APIResponse
	for _, row := range rows {
		keyCounts[row.Key]++
		assert.Equal(t, http.MethodGet, row.Method)
		assert.Equal(t, http.StatusOK, row.Status)
		assert.WithinDuration(t, time.Now(), row.FetchedAt, time.Minute, "the archive stamps real time")
		assert.NotContains(t, row.URL, archiveTestAPIKey, "the Odds API key never reaches the archive")

		body, err := store.APIResponseBody(t.Context(), row.BodySHA256)
		require.NoError(t, err)
		assert.NotEmpty(t, body)

		switch row.Key {
		case refresh.ScoreboardSource, summaryKey, homeRosterKey, awayRosterKey:
			assert.Equal(t, archive.SourceESPN, row.Source)
			assert.Nil(t, row.CreditsRemaining)
			assert.Nil(t, row.CreditsUsed)
		case weatherKey:
			assert.Equal(t, archive.SourceWeather, row.Source)
			assert.Nil(t, row.CreditsRemaining)
			assert.Nil(t, row.CreditsUsed)
		case refresh.OddsLinesSource:
			assert.Equal(t, archive.SourceOdds, row.Source)
			assert.Contains(t, row.URL, "apiKey=REDACTED")
			if strings.Contains(row.URL, "/events") {
				eventsRow = row
			} else {
				linesRow = row
			}
		}
	}
	assert.Equal(t, map[string]int{
		refresh.ScoreboardSource: 1,
		weatherKey:               1,
		summaryKey:               1,
		homeRosterKey:            1,
		awayRosterKey:            1,
		refresh.OddsLinesSource:  2,
	}, keyCounts, "ESPN scoreboard/summary/rosters and weather once each, and the Odds API's free event list plus paid lines")

	require.NotEmpty(t, eventsRow.BodySHA256, "the free event list is archived")
	require.NotNil(t, eventsRow.CreditsRemaining)
	assert.Equal(t, 490, *eventsRow.CreditsRemaining)
	require.NotNil(t, eventsRow.CreditsUsed)
	assert.Equal(t, 0, *eventsRow.CreditsUsed)

	require.NotEmpty(t, linesRow.BodySHA256, "the paid lines call is archived")
	require.NotNil(t, linesRow.CreditsRemaining)
	assert.Equal(t, 489, *linesRow.CreditsRemaining)
	require.NotNil(t, linesRow.CreditsUsed)
	assert.Equal(t, 1, *linesRow.CreditsUsed)

	game, err := store.GameByID(t.Context(), "401")
	require.NoError(t, err)
	assert.Equal(t, "BUF", game.HomeTeam)
	assert.Equal(t, "MIA", game.AwayTeam)
	require.NotNil(t, game.TempF)
	assert.InDelta(t, 41.6, *game.TempF, 1e-9)
	require.NotNil(t, game.OddsEventID)
	assert.Equal(t, "evt-1", *game.OddsEventID)

	parsedOdds, err := store.OddsForGame(t.Context(), "401")
	require.NoError(t, err)
	assert.Len(t, parsedOdds, 2, "the archive does not duplicate or drop parsed odds rows")

	for _, key := range []string{refresh.ScoreboardSource, weatherKey, refresh.OddsLinesSource} {
		state, err := store.SyncState(t.Context(), key)
		require.NoError(t, err)
		assert.Equal(t, ingestNow, state.FetchedAt, "the archive leaves %s's sync state alone", key)
	}
	oddsState, err := store.SyncState(t.Context(), refresh.OddsLinesSource)
	require.NoError(t, err)
	require.NotNil(t, oddsState.OddsCreditsRemaining)
	assert.Equal(t, 489, *oddsState.OddsCreditsRemaining)
}

// TestGoogleSignInLeavesTheArchiveEmpty guards the invariant that Google
// sign-in keeps its own HTTP client. The ingest client proves the archive is
// live first, so an OAuth request reaching it would show up as a second row.
func TestGoogleSignInLeavesTheArchiveEmpty(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"events":[]}`))
	}))
	t.Cleanup(source.Close)

	client := newIngestClient(store)
	request, err := http.NewRequestWithContext(
		archive.WithKey(t.Context(), refresh.ScoreboardSource),
		http.MethodGet,
		source.URL+"/scoreboard",
		nil,
	)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	rows, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1, "the ingest client's archive is live")
	require.Equal(t, refresh.ScoreboardSource, rows[0].Key)

	server := fakeOIDC(t)
	provider, err := newProviderForIssuer(t.Context(), config.Config{
		GoogleClientID:     "client-id",
		GoogleClientSecret: "secret",
		BaseURL:            "http://localhost:8080",
	}, server.URL)
	require.NoError(t, err)

	_, err = provider.Exchange(t.Context(), "code-1", "verifier-1", "nonce-1")
	require.Error(t, err, "the fake issuer returns an invalid ID token")

	rows, err = store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1, "OAuth traffic must never reach the archive")
	assert.Equal(t, refresh.ScoreboardSource, rows[0].Key)
}

// fakeOIDC serves the discovery document, an empty key set, and a token
// response whose ID token cannot verify. The calls still cross the network,
// which is what the archive guard needs.
func fakeOIDC(t *testing.T) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                server.URL,
			"authorization_endpoint":                server.URL + "/auth",
			"token_endpoint":                        server.URL + "/token",
			"jwks_uri":                              server.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     "not-a-signed-token",
		})
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// waitForSyncStates blocks until every source has recorded a fetch. Refresh
// runs in the background, so the test waits for the pass before reading.
func waitForSyncStates(t *testing.T, store *db.Store, keys ...string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		missing := false
		for _, key := range keys {
			_, err := store.SyncState(t.Context(), key)
			if errors.Is(err, db.ErrSyncStateNotFound) {
				missing = true
				break
			}
			require.NoError(t, err)
		}
		if !missing {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("refresh did not record every source: %v", keys)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// fakeESPN serves one scheduled Sunday game at Highmark Stadium, along with
// its summary and team rosters.
func fakeESPN(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/scoreboard":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"season": map[string]any{"year": 2026, "type": 2},
				"week":   map[string]any{"number": 9},
				"events": []any{map[string]any{
					"id":     "401",
					"date":   ingestKickoff.Format(time.RFC3339),
					"status": map[string]any{"type": map[string]any{"state": "pre"}},
					"competitions": []any{map[string]any{
						"competitors": []any{
							map[string]any{
								"homeAway": "home",
								"team": map[string]any{
									"id": "2", "abbreviation": "BUF", "displayName": "Buffalo Bills",
								},
							},
							map[string]any{
								"homeAway": "away",
								"team": map[string]any{
									"id": "15", "abbreviation": "MIA", "displayName": "Miami Dolphins",
								},
							},
						},
						"venue": map[string]any{
							"id": "11938", "fullName": "Highmark Stadium", "indoor": false,
						},
					}},
				}},
			})
		case r.URL.Path == "/summary":
			_ = json.NewEncoder(w).Encode(map[string]any{"injuries": []any{}})
		case strings.HasPrefix(r.URL.Path, "/teams/") && strings.HasSuffix(r.URL.Path, "/roster"):
			_ = json.NewEncoder(w).Encode(map[string]any{"athletes": []any{}})
		default:
			t.Errorf("unexpected ESPN path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// fakeWeather serves the three-hour kickoff window Open-Meteo would return for
// Highmark Stadium.
func fakeWeather(t *testing.T) *httptest.Server {
	t.Helper()

	start := ingestKickoff.UTC().Truncate(time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/forecast", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hourly": map[string]any{
				"time": []string{
					start.Format("2006-01-02T15:04"),
					start.Add(time.Hour).Format("2006-01-02T15:04"),
					start.Add(2 * time.Hour).Format("2006-01-02T15:04"),
				},
				"temperature_2m": []float64{41.6, 42.0, 42.7},
				"precipitation":  []float64{0, 0, 0},
				"wind_speed_10m": []float64{6.3, 5.0, 4.0},
				"wind_gusts_10m": []float64{18.3, 15.0, 12.0},
			},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// fakeOdds serves the free event list and the paid game lines, with the credit
// headers each call would carry.
func fakeOdds(t *testing.T) *httptest.Server {
	t.Helper()

	event := map[string]any{
		"id":            "evt-1",
		"commence_time": ingestKickoff.Format(time.RFC3339),
		"home_team":     "Buffalo Bills",
		"away_team":     "Miami Dolphins",
	}
	lines := map[string]any{
		"id":            "evt-1",
		"commence_time": ingestKickoff.Format(time.RFC3339),
		"home_team":     "Buffalo Bills",
		"away_team":     "Miami Dolphins",
		"bookmakers": []any{map[string]any{
			"key": "draftkings",
			"markets": []any{map[string]any{
				"key": "h2h",
				"outcomes": []any{
					map[string]any{"name": "Buffalo Bills", "price": -155},
					map[string]any{"name": "Miami Dolphins", "price": 135},
				},
			}},
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, archiveTestAPIKey, r.URL.Query().Get("apiKey"))
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/events":
			w.Header().Set("X-Requests-Remaining", "490")
			w.Header().Set("X-Requests-Last", "0")
			_ = json.NewEncoder(w).Encode([]any{event})
		case "/odds":
			w.Header().Set("X-Requests-Remaining", "489")
			w.Header().Set("X-Requests-Last", "1")
			_ = json.NewEncoder(w).Encode([]any{lines})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestMuxProtectsBugReports(t *testing.T) {
	t.Parallel()

	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		nil,
		nil,
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	)

	t.Run("rejects non-HTMX POST", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/bug-reports", nil)
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("redirects unauthenticated HTMX POST to login", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/bug-reports", nil)
		req.Header.Set("Hx-Request", "true")
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "/login", rec.Header().Get("Hx-Redirect"))
	})
}

type stubPrefillRunner struct {
	run func(ctx context.Context) (refresh.PrefillReport, error)
}

func (s stubPrefillRunner) RunPrefill(ctx context.Context) (refresh.PrefillReport, error) {
	return s.run(ctx)
}

func TestPrefillEndpoint(t *testing.T) {
	t.Parallel()

	newTestMux := func(stopCtx context.Context, token string, sandbox bool, runner prefillRunner) http.Handler {
		prefill := newPrefillHandler(stopCtx, token, sandbox, runner, zerolog.Nop())
		return newMux(
			http.NotFoundHandler(),
			http.NotFoundHandler(),
			http.NotFoundHandler(),
			http.NotFoundHandler(),
			prefill,
			nil,
			newTestHandlers(),
			nil,
			passthrough,
			nil,
			nil,
		)
	}

	t.Run("404 when PREFILL_TOKEN is empty", func(t *testing.T) {
		t.Parallel()

		called := false
		mux := newTestMux(t.Context(), "", false, stubPrefillRunner{
			run: func(_ context.Context) (refresh.PrefillReport, error) {
				called = true
				return refresh.PrefillReport{}, nil
			},
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/prefill", nil)
		req.Header.Set("Authorization", "Bearer anything")
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.False(t, called)
	})

	t.Run("401 on missing or invalid bearer token", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			auth string
		}{
			{name: "missing header"},
			{name: "wrong scheme", auth: "Basic dXNlcjpwYXNz"},
			{name: "empty bearer", auth: "Bearer "},
			{name: "wrong token", auth: "Bearer wrong-secret"},
			{name: "prefix match only", auth: "Bearer super-secret-extra"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				called := false
				mux := newTestMux(t.Context(), "super-secret", false, stubPrefillRunner{
					run: func(_ context.Context) (refresh.PrefillReport, error) {
						called = true
						return refresh.PrefillReport{}, nil
					},
				})

				rec := httptest.NewRecorder()
				req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/prefill", nil)
				if tc.auth != "" {
					req.Header.Set("Authorization", tc.auth)
				}
				mux.ServeHTTP(rec, req)

				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
				assert.False(t, called)
			})
		}
	})

	t.Run("503 in sandbox mode", func(t *testing.T) {
		t.Parallel()

		called := false
		mux := newTestMux(t.Context(), "super-secret", true, stubPrefillRunner{
			run: func(_ context.Context) (refresh.PrefillReport, error) {
				called = true
				return refresh.PrefillReport{}, nil
			},
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/prefill", nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.False(t, called)
	})

	t.Run("200 JSON summary on valid bearer token without cookies or CSRF header", func(t *testing.T) {
		t.Parallel()

		remaining := 420
		finished := false
		mux := newTestMux(t.Context(), "super-secret", false, stubPrefillRunner{
			run: func(ctx context.Context) (refresh.PrefillReport, error) {
				deadline, ok := ctx.Deadline()
				assert.True(t, ok, "prefill context should have a deadline")
				assert.LessOrEqual(t, time.Until(deadline), prefillTimeout)
				finished = true
				return refresh.PrefillReport{
					WeekID:          11,
					UpcomingGames:   13,
					LinesSynced:     true,
					PropsSynced:     12,
					PropsSkipped:    1,
					FreeSourcesRun:  30,
					ParlayRemaining: &remaining,
				}, nil
			},
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/prefill", nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		mux.ServeHTTP(rec, req)

		assert.True(t, finished, "handler must run synchronously before returning")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

		var got refresh.PrefillReport
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, int64(11), got.WeekID)
		assert.Equal(t, 13, got.UpcomingGames)
		assert.True(t, got.LinesSynced)
		assert.Equal(t, 12, got.PropsSynced)
		assert.Equal(t, 1, got.PropsSkipped)
		assert.Equal(t, 30, got.FreeSourcesRun)
		require.NotNil(t, got.ParlayRemaining)
		assert.Equal(t, 420, *got.ParlayRemaining)
	})

	t.Run("500 JSON when RunPrefill fails", func(t *testing.T) {
		t.Parallel()

		mux := newTestMux(t.Context(), "super-secret", false, stubPrefillRunner{
			run: func(_ context.Context) (refresh.PrefillReport, error) {
				return refresh.PrefillReport{WeekID: 11, PropsFailed: 2}, errors.New("boom")
			},
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/prefill", nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, rec.Body.String(), `"boom"`)
	})

	t.Run("stopCtx cancellation cancels RunPrefill context", func(t *testing.T) {
		t.Parallel()

		stopCtx, stop := context.WithCancel(t.Context())
		mux := newTestMux(stopCtx, "super-secret", false, stubPrefillRunner{
			run: func(ctx context.Context) (refresh.PrefillReport, error) {
				stop()
				<-ctx.Done()
				return refresh.PrefillReport{}, ctx.Err()
			},
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/internal/prefill", nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		mux.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestMuxKickBoundary(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	user, err := store.CreateUserWithIdentity(
		t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	_, err = store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	cookies := auth.NewCookies("test-session-key-long-enough-for-anything", false)
	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	handlers := auth.NewHandlers(zerolog.Nop(), store, cookies, renderer, nil, false)
	scope := groups.Scope(zerolog.Nop(), store, auth.UserFrom, cookies)
	healthz := newHealthzHandler(database, zerolog.Nop())
	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	kicks := 0
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		healthz,
		nil,
		nil,
		handlers,
		nil,
		scope,
		func() { kicks++ },
		nil,
	)

	signed := httptest.NewRecorder()
	cookies.SetSession(signed, user.ID)
	sessionCookie := responseCookie(t, signed, auth.SessionCookie)

	tests := []struct {
		name       string
		path       string
		auth       bool
		headers    map[string]string
		wantStatus int
		wantKicks  int
	}{
		{
			name:       "unauthenticated GET /",
			path:       "/",
			wantStatus: http.StatusFound,
			wantKicks:  0,
		},
		{
			name:       "unauthenticated GET /games",
			path:       "/games",
			wantStatus: http.StatusFound,
			wantKicks:  0,
		},
		{
			name:       "GET /healthz",
			path:       "/healthz",
			wantStatus: http.StatusOK,
			wantKicks:  0,
		},
		{
			name:       "GET /readyz",
			path:       "/readyz",
			wantStatus: http.StatusOK,
			wantKicks:  0,
		},
		{
			name:       "unauthenticated GET /login",
			path:       "/login",
			wantStatus: http.StatusOK,
			wantKicks:  0,
		},
		{
			name:       "authenticated GET /login",
			path:       "/login",
			auth:       true,
			wantStatus: http.StatusOK,
			wantKicks:  0,
		},
		{
			name:       "authenticated HTMX partial GET /",
			path:       "/",
			auth:       true,
			headers:    map[string]string{"HX-Request": "true"},
			wantStatus: http.StatusNoContent,
			wantKicks:  0,
		},
		{
			name:       "authenticated HTMX partial GET /slips/7/panel",
			path:       "/slips/7/panel",
			auth:       true,
			headers:    map[string]string{"HX-Request": "true"},
			wantStatus: http.StatusNoContent,
			wantKicks:  0,
		},
		{
			name:       "authenticated SSE stream GET /events",
			path:       "/events",
			auth:       true,
			headers:    map[string]string{"Accept": "text/event-stream"},
			wantStatus: http.StatusNoContent,
			wantKicks:  0,
		},
		{
			name:       "authenticated SSE stream GET /slips/7/events",
			path:       "/slips/7/events",
			auth:       true,
			wantStatus: http.StatusNoContent,
			wantKicks:  0,
		},
		{
			name:       "authenticated full-page GET /",
			path:       "/",
			auth:       true,
			wantStatus: http.StatusNoContent,
			wantKicks:  1,
		},
		{
			name:       "second authenticated full-page GET /",
			path:       "/",
			auth:       true,
			wantStatus: http.StatusNoContent,
			wantKicks:  1,
		},
	}

	for _, tt := range tests {
		kicks = 0
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil)
		if tt.auth {
			req.AddCookie(sessionCookie)
		}
		for key, value := range tt.headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		assert.Equal(t, tt.wantStatus, rec.Code, tt.name)
		assert.Equal(t, tt.wantKicks, kicks, tt.name)
	}
}

func TestRequestTimingMiddleware(t *testing.T) {
	t.Parallel()

	type logEntry struct {
		Level      string  `json:"level"`
		Message    string  `json:"message"`
		Method     string  `json:"method"`
		Path       string  `json:"path"`
		Status     int     `json:"status"`
		DurationMS float64 `json:"duration_ms"`
		DBMS       float64 `json:"db_ms"`
		BoardMS    float64 `json:"board_ms"`
		RenderMS   float64 `json:"render_ms"`
	}

	t.Run("slow request logs phase timings and emits Server-Timing in non-prod", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		logger := zerolog.New(&logs).Level(zerolog.InfoLevel)

		handler := withRequestTimingThreshold(logger, true, 5*time.Millisecond)(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				webui.AddDBTiming(r.Context(), 12*time.Millisecond)
				webui.AddBoardTiming(r.Context(), 7*time.Millisecond)
				webui.AddRenderTiming(r.Context(), 3*time.Millisecond)
				time.Sleep(10 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			},
		))

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/2/pick", nil)
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		serverTiming := rec.Header().Get("Server-Timing")
		assert.Contains(t, serverTiming, "db;dur=12.00")
		assert.Contains(t, serverTiming, "board;dur=7.00")
		assert.Contains(t, serverTiming, "render;dur=3.00")
		assert.Contains(t, serverTiming, "total;dur=")

		var entry logEntry
		require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry))
		assert.Equal(t, "warn", entry.Level)
		assert.Equal(t, "http request", entry.Message)
		assert.Equal(t, http.MethodGet, entry.Method)
		assert.Equal(t, "/slips/2/pick", entry.Path)
		assert.Equal(t, http.StatusOK, entry.Status)
		assert.GreaterOrEqual(t, entry.DurationMS, 5.0)
		assert.InDelta(t, 12.0, entry.DBMS, 0.01)
		assert.InDelta(t, 7.0, entry.BoardMS, 0.01)
		assert.InDelta(t, 3.0, entry.RenderMS, 0.01)
	})

	t.Run("prod omits Server-Timing and fast requests log at debug", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		logger := zerolog.New(&logs).Level(zerolog.DebugLevel)

		handler := withRequestTimingThreshold(logger, false, 500*time.Millisecond)(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
		))

		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/slips/2", nil)
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Header().Get("Server-Timing"))

		var entry logEntry
		require.NoError(t, json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry))
		assert.Equal(t, "debug", entry.Level)
		assert.Equal(t, http.StatusNoContent, entry.Status)
	})

	t.Run("skips SSE streams", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		logger := zerolog.New(&logs).Level(zerolog.DebugLevel)

		handler := withRequestTimingThreshold(logger, true, 0)(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		))

		for _, path := range []string{"/slips/2/events", "/events"} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			if path == "/events" {
				req.Header.Set("Accept", "text/event-stream")
			}
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("Server-Timing"))
		}
		assert.Empty(t, logs.String())
	})
}

func TestPprofRoutes(t *testing.T) {
	t.Parallel()

	fallback := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	t.Run("enabled in non-prod", func(t *testing.T) {
		t.Parallel()

		handler := withPprof(fallback, false)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/debug/pprof/", nil))
		assert.Equal(t, http.StatusOK, rec.Code)

		cmdline := httptest.NewRecorder()
		handler.ServeHTTP(
			cmdline,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/debug/pprof/cmdline", nil),
		)
		assert.Equal(t, http.StatusOK, cmdline.Code)
	})

	t.Run("returns 404 in prod", func(t *testing.T) {
		t.Parallel()

		handler := withPprof(fallback, true)

		for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/cmdline"} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
			assert.Equal(t, http.StatusNotFound, rec.Code, path)
		}
	})
}

type offlineRoundTripper struct{}

func (offlineRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Body:       http.NoBody,
	}, nil
}

func TestProdServicesShareSlipsInstanceAndInvalidateEvalCache(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	evalCache := eval.NewCache()
	ingestClient := &http.Client{Transport: offlineRoundTripper{}}
	refresher, slipService := newProdServices(
		config.Config{},
		store,
		&live.Hub{},
		evalCache,
		ingestClient,
		zerolog.Nop(),
	)
	require.NotNil(t, slipService)
	require.NotNil(t, refresher)
	t.Cleanup(func() { assert.NoError(t, refresher.Shutdown(context.Background())) })

	ctx := t.Context()
	user, err := store.CreateUserWithIdentity(ctx, "Ada", "ada@example.com", "google", "google-ada")
	require.NoError(t, err)
	group, err := store.Create(ctx, user.ID, "Sunday Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(ctx, 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(ctx, group.ID, week.ID, nil, "Week 9", db.SlateScopeFullWeek, 4, []int64{user.ID})
	require.NoError(t, err)

	game, err := store.UpsertGame(ctx, db.Game{
		ID:        "prod-grade-test",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "KC",
		Kickoff:   time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC),
		Status:    db.GameFinal,
		HomeScore: new(27),
		AwayScore: new(20),
	})
	require.NoError(t, err)

	leg, err := store.CreateLeg(ctx, db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &game.ID,
		MarketType: "ML",
		Selection:  "BUF ML",
		Team:       new("BUF"),
	})
	require.NoError(t, err)

	reportBefore, err := evalCache.GetOrCompute(ctx, store, group.ID, week.Season)
	require.NoError(t, err)
	assert.Equal(t, 0, reportBefore.Coverage.TotalLegs)

	_ = refresher.RunOnce(ctx)

	gotLeg, err := store.LegByID(ctx, leg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotLeg.Result)

	reportAfter, err := evalCache.GetOrCompute(ctx, store, group.ID, week.Season)
	require.NoError(t, err)
	assert.Equal(t, 1, reportAfter.Coverage.TotalLegs)
}
