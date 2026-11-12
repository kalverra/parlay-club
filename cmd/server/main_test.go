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
	"github.com/kalverra/parlay-club/internal/groups"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/refresh"
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

	recorder := httptest.NewRecorder()
	newMux(
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		newTestHandlers(),
		nil,
		passthrough,
		nil,
		nil,
	).ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
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

func TestMuxServesScreenshotsWithoutAuth(t *testing.T) {
	t.Parallel()

	screenshots := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		screenshots,
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
	go func() { done <- serve(ctx, zerolog.Nop(), server, listener) }()

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
	service.KickContext(t.Context())

	weatherKey := refresh.WeatherSource("401")
	waitForSyncStates(t, store, refresh.ScoreboardSource, weatherKey, refresh.OddsLinesSource)

	rows, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 4, "one row per outside call")

	keyCounts := make(map[string]int, 3)
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
		case refresh.ScoreboardSource:
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
		refresh.OddsLinesSource:  2,
	}, keyCounts, "ESPN and weather once each, and the Odds API's free event list plus paid lines")

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

// fakeESPN serves one scheduled Sunday game at Highmark Stadium.
func fakeESPN(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/scoreboard", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
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
