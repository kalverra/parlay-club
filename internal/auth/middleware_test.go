package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
)

// probe stands in for the handler a middleware guards.
type probe struct {
	called  bool
	user    db.User
	hasUser bool
}

func (p *probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.called = true
	p.user, p.hasUser = auth.UserFrom(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// guardedRequest builds a request carrying cookies.
func guardedRequest(t *testing.T, method, target string, cookies ...*http.Cookie) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	return request
}

// serveProbe runs one request through handler and returns its response.
func serveProbe(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// tamperedCopy returns a copy of cookie with a broken signature.
func tamperedCopy(cookie *http.Cookie) *http.Cookie {
	//nolint:gosec // the handler set the real attributes; only the value changes here.
	broken := *cookie
	broken.Value = tampered(broken.Value)
	return &broken
}

func TestRequireUserRedirects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	active := f.addUserWithEmail(t, "Ada", "ada@example.com")
	inactive := f.addUserWithEmail(t, "Grace", "grace@example.com")
	_, err := f.database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", inactive.ID)
	require.NoError(t, err)

	oldUserCookie := &http.Cookie{
		Name:     "pc_user",
		Value:    "1.signature",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	expired := &http.Cookie{
		Name:     auth.SessionCookie,
		Value:    sessionValue(t, active.ID, time.Now().Add(-time.Minute).Unix()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	tests := map[string]*http.Cookie{
		"missing":       nil,
		"tampered":      tamperedCopy(f.sessionCookie(t, active.ID)),
		"expired":       expired,
		"old pc_user":   oldUserCookie,
		"unknown user":  f.sessionCookie(t, 404),
		"inactive user": f.sessionCookie(t, inactive.ID),
	}

	for name, cookie := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			next := &probe{}
			var cookies []*http.Cookie
			if cookie != nil {
				cookies = append(cookies, cookie)
			}

			recorder := serveProbe(
				f.handlers.RequireUser(next),
				guardedRequest(t, http.MethodGet, "/", cookies...),
			)

			assert.Equal(t, http.StatusFound, recorder.Code)
			assert.Equal(t, "/login", recorder.Header().Get("Location"))
			assert.False(t, next.called)
		})
	}
}

func TestRequireUserChecksTheGateEveryRequest(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, newFakeProvider(), auth.NewGate([]config.AllowedEmail{
		{Email: "ada@example.com"},
	}), true)
	ada := f.addUserWithEmail(t, "Ada", "ada@example.com")

	next := &probe{}
	allowed := serveProbe(
		f.handlers.RequireUser(next),
		guardedRequest(t, http.MethodGet, "/", f.sessionCookie(t, ada.ID)),
	)
	require.Equal(t, http.StatusNoContent, allowed.Code)
	require.True(t, next.called)

	// The list loses Ada, and her existing cookie stops working on the next load.
	f = newFixtureWith(t, newFakeProvider(), auth.NewGate([]config.AllowedEmail{
		{Email: "friend@example.com"},
	}), true)
	ada = f.addUserWithEmail(t, "Ada", "ada@example.com")

	next = &probe{}
	denied := serveProbe(
		f.handlers.RequireUser(next),
		guardedRequest(t, http.MethodGet, "/", f.sessionCookie(t, ada.ID)),
	)

	assert.Equal(t, http.StatusFound, denied.Code)
	assert.Equal(t, "/login", denied.Header().Get("Location"))
	assert.False(t, next.called)
}

func TestRequireUserAllowsActiveUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUserWithEmail(t, "Ada", "ada@example.com")
	next := &probe{}

	recorder := serveProbe(
		f.handlers.RequireUser(next),
		guardedRequest(t, http.MethodGet, "/", f.sessionCookie(t, ada.ID)),
	)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.True(t, next.called)
	require.True(t, next.hasUser)
	assert.Equal(t, ada, next.user)
}

func TestRequireUserUsesHXRedirect(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	next := &probe{}
	request := guardedRequest(t, http.MethodGet, "/")
	request.Header.Set("Hx-Request", "true")

	recorder := serveProbe(f.handlers.RequireUser(next), request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Hx-Redirect"))
	assert.Empty(t, recorder.Body.String())
	assert.False(t, next.called)
}

func TestRequireUserHXRedirectForInactiveUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	gone := f.addUserWithEmail(t, "Grace", "grace@example.com")
	_, err := f.database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", gone.ID)
	require.NoError(t, err)

	next := &probe{}
	request := guardedRequest(t, http.MethodGet, "/", f.sessionCookie(t, gone.ID))
	request.Header.Set("Hx-Request", "true")

	recorder := serveProbe(f.handlers.RequireUser(next), request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Hx-Redirect"))
	assert.False(t, next.called)
}

func TestCSRFAllowsSafeAndExemptRequests(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	tests := map[string]struct{ method, target string }{
		"get page":       {http.MethodGet, "/games"},
		"google start":   {http.MethodGet, "/auth/google/start"},
		"google return":  {http.MethodGet, "/auth/google/callback"},
		"post logout":    {http.MethodPost, "/logout"},
		"post dev login": {http.MethodPost, "/dev/login"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			next := &probe{}
			recorder := serveProbe(
				f.handlers.CSRF(next),
				guardedRequest(t, test.method, test.target),
			)

			assert.Equal(t, http.StatusNoContent, recorder.Code)
			assert.True(t, next.called)
		})
	}
}

func TestCSRFRejectsUnmarkedWrites(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	tests := map[string]struct{ method, target string }{
		"post":                            {http.MethodPost, "/games"},
		"put":                             {http.MethodPut, "/slips/1"},
		"patch":                           {http.MethodPatch, "/slips/1"},
		"delete":                          {http.MethodDelete, "/legs/1"},
		"exempt path with another verb":   {http.MethodPut, "/logout"},
		"exempt path with trailing slash": {http.MethodPost, "/dev/login/"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			next := &probe{}
			recorder := serveProbe(
				f.handlers.CSRF(next),
				guardedRequest(t, test.method, test.target),
			)

			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.False(t, next.called)
		})
	}
}

func TestCSRFAllowsHTMXWrites(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	next := &probe{}
	request := guardedRequest(t, http.MethodPost, "/slips/1/legs")
	request.Header.Set("Hx-Request", "true")

	recorder := serveProbe(f.handlers.CSRF(next), request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.True(t, next.called)
}

func TestCSRFRejectsFalseHXHeader(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	next := &probe{}
	request := guardedRequest(t, http.MethodPost, "/games")
	request.Header.Set("Hx-Request", "false")

	recorder := serveProbe(f.handlers.CSRF(next), request)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.False(t, next.called)
}
