package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
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

// userCookie returns the signed cookie for id.
func (f *fixture) userCookie(t *testing.T, id int64) *http.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	f.cookies.SetUser(recorder, id)

	cookie := cookieFrom(recorder, auth.UserCookie)
	require.NotNil(t, cookie)
	return cookie
}

// tamperedCopy returns a copy of cookie with a broken signature.
func tamperedCopy(cookie *http.Cookie) *http.Cookie {
	//nolint:gosec // the handler set the real attributes; only the value changes here.
	broken := *cookie
	broken.Value = tampered(broken.Value)
	return &broken
}

func TestRequirePasscodeRedirects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	tests := map[string]*http.Cookie{
		"missing":  nil,
		"tampered": tamperedCopy(f.passcode),
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
				f.handlers.RequirePasscode(next),
				guardedRequest(t, http.MethodGet, "/games", cookies...),
			)

			assert.Equal(t, http.StatusFound, recorder.Code)
			assert.Equal(t, "/login", recorder.Header().Get("Location"))
			assert.False(t, next.called)
		})
	}
}

func TestRequirePasscodeAllowsValidCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	next := &probe{}

	recorder := serveProbe(
		f.handlers.RequirePasscode(next),
		guardedRequest(t, http.MethodGet, "/games", f.passcode),
	)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.True(t, next.called)
}

func TestRequirePasscodeUsesHXRedirect(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	next := &probe{}
	request := guardedRequest(t, http.MethodGet, "/games")
	request.Header.Set("Hx-Request", "true")

	recorder := serveProbe(f.handlers.RequirePasscode(next), request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Hx-Redirect"))
	assert.Empty(t, recorder.Body.String())
	assert.False(t, next.called)
}

func TestRequireUserRedirects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	active := f.addUser(t, "Ada")
	inactive := f.addUser(t, "Grace")
	_, err := f.database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", inactive.ID)
	require.NoError(t, err)

	tests := map[string]*http.Cookie{
		"missing":       nil,
		"tampered":      tamperedCopy(f.userCookie(t, active.ID)),
		"unknown user":  f.userCookie(t, 404),
		"inactive user": f.userCookie(t, inactive.ID),
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
			assert.Equal(t, "/who", recorder.Header().Get("Location"))
			assert.False(t, next.called)
		})
	}
}

func TestRequireUserAllowsActiveUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "Ada")
	next := &probe{}

	recorder := serveProbe(
		f.handlers.RequireUser(next),
		guardedRequest(t, http.MethodGet, "/", f.userCookie(t, ada.ID)),
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
	assert.Equal(t, "/who", recorder.Header().Get("Hx-Redirect"))
	assert.Empty(t, recorder.Body.String())
	assert.False(t, next.called)
}

func TestRequireUserHXRedirectForInactiveUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	gone := f.addUser(t, "Grace")
	_, err := f.database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", gone.ID)
	require.NoError(t, err)

	next := &probe{}
	request := guardedRequest(t, http.MethodGet, "/", f.userCookie(t, gone.ID))
	request.Header.Set("Hx-Request", "true")

	recorder := serveProbe(f.handlers.RequireUser(next), request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/who", recorder.Header().Get("Hx-Redirect"))
	assert.False(t, next.called)
}

func TestCSRFAllowsSafeAndExemptRequests(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	tests := map[string]struct{ method, target string }{
		"get page":      {http.MethodGet, "/games"},
		"post login":    {http.MethodPost, "/login"},
		"post who":      {http.MethodPost, "/who"},
		"post add name": {http.MethodPost, "/users"},
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
		"exempt path with another verb":   {http.MethodPut, "/login"},
		"exempt path with trailing slash": {http.MethodPost, "/users/"},
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
