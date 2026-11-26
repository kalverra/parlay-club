package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
)

// TestMuxProtectsPushRoutes drives the push routes through the real CSRF and
// RequireUser middleware. The shell stands in for the app's handlers and
// answers 204, so reaching it proves the request was let through.
func TestMuxProtectsPushRoutes(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "", "")
	require.NoError(t, err)

	cookies := auth.NewCookies("test-session-key-long-enough-for-anything", false)
	handlers := auth.NewHandlers(zerolog.Nop(), store, cookies, nil, nil, false)
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
		handlers,
		nil,
		passthrough,
		nil,
		nil,
	)

	signed := httptest.NewRecorder()
	cookies.SetSession(signed, user.ID)
	sessionCookie := responseCookie(t, signed, auth.SessionCookie)

	expired := httptest.NewRecorder()
	cookies.SetSession(expired, user.ID+1000)
	expiredCookie := responseCookie(t, expired, auth.SessionCookie)

	routes := []struct {
		name   string
		method string
		path   string
	}{
		{name: "subscribe", method: http.MethodPost, path: "/push/subscription"},
		{name: "unsubscribe", method: http.MethodDelete, path: "/push/subscription"},
		{name: "test", method: http.MethodPost, path: "/push/test"},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			t.Parallel()

			t.Run("rejects a non-HTMX request", func(t *testing.T) {
				t.Parallel()

				recorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, nil)
				mux.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusForbidden, recorder.Code)
			})

			t.Run("redirects a signed-out person", func(t *testing.T) {
				t.Parallel()

				recorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, nil)
				request.Header.Set("Hx-Request", "true")
				mux.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusOK, recorder.Code)
				assert.Equal(t, "/login", recorder.Header().Get("Hx-Redirect"))
			})

			t.Run("redirects an expired session", func(t *testing.T) {
				t.Parallel()

				recorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, nil)
				request.Header.Set("Hx-Request", "true")
				request.AddCookie(expiredCookie)
				mux.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusOK, recorder.Code)
				assert.Equal(t, "/login", recorder.Header().Get("Hx-Redirect"))
			})

			t.Run("lets a signed-in person through", func(t *testing.T) {
				t.Parallel()

				recorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(t.Context(), route.method, route.path, nil)
				request.Header.Set("Hx-Request", "true")
				request.AddCookie(sessionCookie)
				mux.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusNoContent, recorder.Code)
				assert.Empty(t, recorder.Header().Get("Hx-Redirect"))
			})
		})
	}
}
