package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/groups"
)

// passthrough stands in for the group scope in tests that do not need one.
func passthrough(next http.Handler) http.Handler { return next }

func TestHealthz(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	newMux(http.NotFoundHandler(), http.NotFoundHandler(), newTestHandlers(), nil, passthrough).
		ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ok", recorder.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func TestMuxProtectsShell(t *testing.T) {
	t.Parallel()

	shell := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(shell, http.NotFoundHandler(), newTestHandlers(), nil, passthrough)

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
	mux := newMux(http.NotFoundHandler(), static, newTestHandlers(), nil, passthrough)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/static/app.css",
		nil,
	))

	assert.Equal(t, http.StatusNoContent, recorder.Code, "static files load on the login page")
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
	mux := newMux(shell, http.NotFoundHandler(), handlers, nil, scope)

	signed := httptest.NewRecorder()
	cookies.SetSession(signed, user.ID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/games", nil)
	request.AddCookie(responseCookie(t, signed, auth.SessionCookie))
	mux.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
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
