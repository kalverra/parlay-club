package auth_test

import (
	"context"
	"database/sql"
	"errors"
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

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// fixture wires the auth handlers over a real database and the embedded
// templates.
type fixture struct {
	mux      *http.ServeMux
	handlers *auth.Handlers
	store    *db.Store
	database *sql.DB
	cookies  *auth.Cookies
	provider *fakeProvider
	gate     auth.Gate
}

// fakeProvider stands in for Google, recording what the handlers asked for.
type fakeProvider struct {
	authURL   string
	state     string
	nonce     string
	challenge string
	code      string
	verifier  string
	gotNonce  string
	exchanges int
	identity  auth.Identity
	err       error
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{
		authURL: "https://accounts.example/auth",
		identity: auth.Identity{
			Sub:     "google-ada",
			Email:   "ada@example.com",
			Name:    "Ada",
			Picture: "ada.png",
		},
	}
}

func (p *fakeProvider) AuthURL(state, nonce, challenge string) string {
	p.state, p.nonce, p.challenge = state, nonce, challenge
	return p.authURL + "?state=" + url.QueryEscape(state)
}

func (p *fakeProvider) Exchange(
	_ context.Context,
	code, verifier, nonce string,
) (auth.Identity, error) {
	p.code, p.verifier, p.gotNonce = code, verifier, nonce
	p.exchanges++
	return p.identity, p.err
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixtureWith(t, newFakeProvider(), auth.NewGate(nil), true)
}

func newFixtureWith(
	t *testing.T,
	provider auth.Provider,
	gate auth.Gate,
	devLogin bool,
) *fixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	f := &fixture{
		mux:      http.NewServeMux(),
		store:    db.NewStore(database),
		database: database,
		cookies:  auth.NewCookies(testSessionKey, false),
		gate:     gate,
	}
	if fake, ok := provider.(*fakeProvider); ok {
		f.provider = fake
	}

	f.handlers = auth.NewHandlers(zerolog.Nop(), f.store, f.cookies, gate, renderer, provider, devLogin)
	f.handlers.Routes(f.mux)

	return f
}

// serve runs request through the auth routes.
func (f *fixture) serve(t *testing.T, request *http.Request, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	f.mux.ServeHTTP(recorder, request)
	return recorder
}

// addUser seeds a user with no email.
func (f *fixture) addUser(t *testing.T, name string) db.User {
	t.Helper()
	return f.addUserWithEmail(t, name, "")
}

// addUserWithEmail seeds a user with an email.
func (f *fixture) addUserWithEmail(t *testing.T, name, email string) db.User {
	t.Helper()

	user, err := f.store.CreateUserWithIdentity(t.Context(), name, email, "", "")
	require.NoError(t, err)
	return user
}

// sessionCookie returns a signed session cookie for id.
func (f *fixture) sessionCookie(t *testing.T, id int64) *http.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	f.cookies.SetSession(recorder, id)

	cookie := cookieFrom(recorder, auth.SessionCookie)
	require.NotNil(t, cookie)
	return cookie
}

// start begins the Google round trip and returns the cookie the response set.
func (f *fixture) start(t *testing.T) *http.Cookie {
	t.Helper()

	recorder := f.serve(t, getRequest(t, "/auth/google/start"))
	require.Equal(t, http.StatusFound, recorder.Code)

	cookie := cookieFrom(recorder, auth.OAuthCookie)
	require.NotNil(t, cookie)
	return cookie
}

// callbackRequest builds the callback for state with the round-trip cookie.
func callbackRequest(t *testing.T, state string, cookie *http.Cookie) *http.Request {
	t.Helper()

	target := "/auth/google/callback?state=" + url.QueryEscape(state) + "&code=code-1"
	if cookie == nil {
		return getRequest(t, target)
	}
	request := getRequest(t, target)
	request.AddCookie(cookie)
	return request
}

func getRequest(t *testing.T, target string) *http.Request {
	t.Helper()

	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
}

func postForm(t *testing.T, target string, values url.Values) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		target,
		strings.NewReader(values.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

// cookieFrom returns the named cookie the response set, or nil.
func cookieFrom(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// sessionUser returns the user the response's session cookie signs in.
func (f *fixture) sessionUser(t *testing.T, recorder *httptest.ResponseRecorder) (db.User, bool) {
	t.Helper()

	cookie := cookieFrom(recorder, auth.SessionCookie)
	if cookie == nil {
		return db.User{}, false
	}
	request := getRequest(t, "/")
	request.AddCookie(cookie)
	id, ok := f.cookies.SessionID(request)
	if !ok {
		return db.User{}, false
	}
	user, err := f.store.UserByID(t.Context(), id)
	require.NoError(t, err)
	return user, true
}

func TestLoginForm(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, getRequest(t, "/login"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	body := recorder.Body.String()
	assert.Contains(t, body, `href="/auth/google/start"`)
	assert.Contains(t, body, "Sign in with Google")
	assert.Contains(t, body, `href="/dev/login"`)
	assert.NotContains(t, body, `name="passcode"`)
	assert.NotContains(t, body, `aria-label="Main"`, "auth pages hide the app tabs")
	assert.NotContains(t, body, `role="alert"`)
}

func TestLoginFormWithoutGoogleOffersDevOnly(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, nil, auth.NewGate(nil), true)

	recorder := f.serve(t, getRequest(t, "/login"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.NotContains(t, body, "/auth/google/start")
	assert.Contains(t, body, `href="/dev/login"`)
}

func TestGoogleRoutesNeedAProvider(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, nil, auth.NewGate(nil), false)

	recorder := f.serve(t, getRequest(t, "/auth/google/start"))
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestGoogleStartRedirectsAndSetsOAuthCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, getRequest(t, "/auth/google/start"))

	assert.Equal(t, http.StatusFound, recorder.Code)
	assert.True(t, strings.HasPrefix(recorder.Header().Get("Location"), f.provider.authURL))

	cookie := cookieFrom(recorder, auth.OAuthCookie)
	require.NotNil(t, cookie)
	assert.Equal(t, "/auth", cookie.Path)
	assert.True(t, cookie.HttpOnly)

	saved, ok := f.cookies.OAuthState(requestWithCookie(t, cookie))
	require.True(t, ok)
	assert.Equal(t, f.provider.state, saved.State)
	assert.Equal(t, f.provider.nonce, saved.Nonce)
	assert.NotEmpty(t, saved.Verifier)
	assert.NotEmpty(t, f.provider.challenge)
	assert.NotEqual(t, saved.Verifier, f.provider.challenge, "the challenge is hashed")
}

func TestGoogleCallbackSignsIn(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	oauth := f.start(t)

	recorder := f.serve(t, callbackRequest(t, f.provider.state, oauth))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Location"))

	user, ok := f.sessionUser(t, recorder)
	require.True(t, ok)
	assert.Equal(t, "ada@example.com", user.Email)
	assert.Equal(t, "Ada", user.Name)
	assert.Equal(t, "ada.png", user.PictureURL)

	assert.Equal(t, "code-1", f.provider.code)
	assert.NotEmpty(t, f.provider.verifier)
	assert.Equal(t, f.provider.nonce, f.provider.gotNonce, "the nonce reaches the provider")

	cleared := cookieFrom(recorder, auth.OAuthCookie)
	require.NotNil(t, cleared)
	assert.Empty(t, cleared.Value, "the round-trip cookie is cleared")
}

func TestGoogleCallbackNotInvited(t *testing.T) {
	t.Parallel()

	provider := newFakeProvider()
	provider.identity.Email = "stranger@example.com"
	f := newFixtureWith(t, provider, auth.NewGate([]config.AllowedEmail{
		{Email: "friend@example.com"},
	}), true)
	oauth := f.start(t)

	recorder := f.serve(t, callbackRequest(t, provider.state, oauth))

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(),
		"That Google account isn&#39;t on the list yet. Ask whoever runs the club to add you.")
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie), "a stranger gets no cookie")

	users, err := f.store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Empty(t, users, "nothing is written for a stranger")
}

func TestGoogleCallbackRejectsWrongState(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	oauth := f.start(t)

	recorder := f.serve(t, callbackRequest(t, "another-state", oauth))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Sign-in didn")
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie))
	assert.Zero(t, f.provider.exchanges, "a mismatched state never reaches the provider")
}

func TestGoogleCallbackRejectsMissingOAuthCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, callbackRequest(t, "some-state", nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie))
	assert.Zero(t, f.provider.exchanges)
}

func TestGoogleCallbackRejectsExpiredOAuthCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	expired := &http.Cookie{
		Name:     auth.OAuthCookie,
		Value:    oauthValue(t, "state-1", "nonce-1", "verifier-1", time.Now().Add(-11*time.Minute)),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	recorder := f.serve(t, callbackRequest(t, "state-1", expired))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie))
	assert.Zero(t, f.provider.exchanges)
}

func TestGoogleCallbackRejectsReplay(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	oauth := f.start(t)

	first := f.serve(t, callbackRequest(t, f.provider.state, oauth))
	require.Equal(t, http.StatusSeeOther, first.Code)

	// The browser no longer has the round-trip cookie after a successful sign-in.
	cleared := cookieFrom(first, auth.OAuthCookie)
	require.NotNil(t, cleared)

	replay := f.serve(t, callbackRequest(t, f.provider.state, cleared))

	assert.Equal(t, http.StatusBadRequest, replay.Code)
	assert.Nil(t, cookieFrom(replay, auth.SessionCookie))
	assert.Equal(t, 1, f.provider.exchanges, "the second callback never reaches the provider")
}

func TestGoogleCallbackRejectsUnverifiedEmail(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.provider.err = auth.ErrEmailNotVerified
	oauth := f.start(t)

	recorder := f.serve(t, callbackRequest(t, f.provider.state, oauth))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Sign-in didn")
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie))
}

func TestGoogleCallbackRenderErrorAnswers500(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	mux := http.NewServeMux()
	f.handlers = auth.NewHandlers(
		zerolog.Nop(), f.store, f.cookies, f.gate, failingRenderer{}, f.provider, true)
	f.handlers.Routes(mux)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, getRequest(t, "/login"))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestLogoutClearsSession(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "Ada")

	recorder := f.serve(t, postForm(t, "/logout", nil), f.sessionCookie(t, ada.ID))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/login", recorder.Header().Get("Location"))

	cookie := cookieFrom(recorder, auth.SessionCookie)
	require.NotNil(t, cookie)
	_, ok := f.cookies.SessionID(requestWithCookie(t, cookie))
	assert.False(t, ok)
}

func TestDevLoginForm(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, getRequest(t, "/dev/login"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	body := recorder.Body.String()
	assert.Contains(t, body, `action="/dev/login"`)
	assert.Contains(t, body, `name="email"`)
	assert.Contains(t, body, `name="name"`)
	assert.NotContains(t, body, `aria-label="Main"`, "auth pages hide the app tabs")
}

func TestDevLoginSignsIn(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/dev/login", url.Values{
		"email": {"new@example.com"},
		"name":  {"New Person"},
	}))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Location"))

	user, ok := f.sessionUser(t, recorder)
	require.True(t, ok)
	assert.Equal(t, "new@example.com", user.Email)
	assert.Equal(t, "New Person", user.Name)

	linked, err := f.store.UserByGoogleSub(t.Context(), "dev:new@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, linked.ID)
}

func TestDevLoginRejectsStranger(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, newFakeProvider(), auth.NewGate([]config.AllowedEmail{
		{Email: "friend@example.com"},
	}), true)

	recorder := f.serve(t, postForm(t, "/dev/login", url.Values{
		"email": {"stranger@example.com"},
		"name":  {"Stranger"},
	}))

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "isn&#39;t on the list yet")
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie))
}

func TestDevLoginRequiresEmail(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/dev/login", url.Values{"name": {"Ada"}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Add an email first.")
	assert.Nil(t, cookieFrom(recorder, auth.SessionCookie))
}

func TestDevLoginIsAbsentInProd(t *testing.T) {
	t.Parallel()

	f := newFixtureWith(t, newFakeProvider(), auth.NewGate([]config.AllowedEmail{
		{Email: "ada@example.com"},
	}), false)

	assert.Equal(t, http.StatusNotFound, f.serve(t, getRequest(t, "/dev/login")).Code)
	assert.Equal(t, http.StatusNotFound, f.serve(t, postForm(t, "/dev/login", url.Values{
		"email": {"ada@example.com"},
	})).Code)
}

// failingRenderer stands in for a template that cannot render.
type failingRenderer struct{}

func (failingRenderer) Render(string, any) ([]byte, error) {
	return nil, errors.New("broken template")
}
