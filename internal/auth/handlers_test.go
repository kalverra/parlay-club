package auth_test

import (
	"database/sql"
	"errors"
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

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// fixture wires the auth handlers over a real database and the embedded
// templates.
type fixture struct {
	mux      *http.ServeMux
	store    *db.Store
	database *sql.DB
	cookies  *auth.Cookies
	passcode *http.Cookie
	waits    []time.Duration
}

func newFixture(t *testing.T) *fixture {
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
		cookies:  auth.NewCookies(testSessionKey, testPasscode, false),
	}

	passcodeRecorder := httptest.NewRecorder()
	f.cookies.SetPasscode(passcodeRecorder)
	f.passcode = cookieFrom(passcodeRecorder, auth.PasscodeCookie)
	require.NotNil(t, f.passcode)

	handlers := auth.NewHandlers(
		zerolog.Nop(),
		f.store,
		f.cookies,
		renderer,
		func(delay time.Duration) { f.waits = append(f.waits, delay) },
	)
	handlers.Routes(f.mux)

	return f
}

// serve runs request through the auth routes the way a browser that already
// typed the passcode would.
func (f *fixture) serve(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	request.AddCookie(f.passcode)

	recorder := httptest.NewRecorder()
	f.mux.ServeHTTP(recorder, request)
	return recorder
}

// addUser seeds a user directly, without a cookie.
func (f *fixture) addUser(t *testing.T, name string) db.User {
	t.Helper()

	user, err := f.store.CreateUser(t.Context(), name)
	require.NoError(t, err)
	return user
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

// followUp builds a request carrying every cookie the response set.
func followUp(t *testing.T, recorder *httptest.ResponseRecorder) *http.Request {
	t.Helper()

	request := getRequest(t, "/")
	for _, cookie := range recorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
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

func TestLoginForm(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, getRequest(t, "/login"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	body := recorder.Body.String()
	assert.Contains(t, body, `action="/login"`)
	assert.Contains(t, body, `name="passcode"`)
	assert.Contains(t, body, `type="password"`)
	assert.Contains(t, body, "Enter the group passcode")
	assert.NotContains(t, body, `aria-label="Main"`, "auth pages hide the app tabs")
	assert.NotContains(t, body, `role="alert"`)
}

func TestLoginSetsPasscodeCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/login", url.Values{"passcode": {testPasscode}}))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/who", recorder.Header().Get("Location"))
	assert.Empty(t, f.waits)

	cookie := cookieFrom(recorder, auth.PasscodeCookie)
	require.NotNil(t, cookie)
	assert.True(t, f.cookies.HasPasscode(followUp(t, recorder)))
}

func TestLoginWrongPasscodeWaitsAndRejects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/login", url.Values{"passcode": {"nope"}}))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, []time.Duration{time.Second}, f.waits, "a wrong passcode waits a second")
	assert.Contains(t, recorder.Body.String(), "That passcode")
	assert.Nil(t, cookieFrom(recorder, auth.PasscodeCookie))
	assert.False(t, f.cookies.HasPasscode(followUp(t, recorder)))
}

func TestWhoListsActiveUsers(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "Ada Lovelace")
	gone := f.addUser(t, "Grace Hopper")
	_, err := f.database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", gone.ID)
	require.NoError(t, err)

	recorder := f.serve(t, getRequest(t, "/who"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))

	body := recorder.Body.String()
	assert.Contains(t, body, "Ada Lovelace")
	assert.Contains(t, body, ">AL</span", "avatar initials")
	assert.Contains(t, body, "bg-sky-500", "avatar color class")
	assert.NotContains(t, body, "Grace Hopper", "inactive users stay hidden")
	assert.Contains(t, body, `action="/who"`)
	assert.Contains(t, body, `value="`+itoa(ada.ID)+`"`)
	assert.Contains(t, body, `action="/users"`)
	assert.Contains(t, body, `maxlength="24"`)
	assert.NotContains(t, body, `aria-label="Main"`, "auth pages hide the app tabs")
}

func TestWhoWithNoUsersInvitesTheFirst(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, getRequest(t, "/who"))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "No names yet. Add the first one below.")
}

func TestWhoSetsUserCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "Ada")

	recorder := f.serve(t, postForm(t, "/who", url.Values{"user_id": {itoa(ada.ID)}}))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Location"))

	id, ok := f.cookies.UserID(followUp(t, recorder))
	require.True(t, ok)
	assert.Equal(t, ada.ID, id)
}

func TestWhoRejectsUnknownUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/who", url.Values{"user_id": {"404"}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "no longer on the list")
	assert.Nil(t, cookieFrom(recorder, auth.UserCookie))
}

func TestWhoRejectsInactiveUser(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "Ada")
	_, err := f.database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", ada.ID)
	require.NoError(t, err)

	recorder := f.serve(t, postForm(t, "/who", url.Values{"user_id": {itoa(ada.ID)}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "no longer active")
	assert.Nil(t, cookieFrom(recorder, auth.UserCookie))
}

func TestWhoRejectsBadUserID(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/who", url.Values{"user_id": {"ada"}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Pick a name from the list.")
}

func TestAddUserCreatesAndSelects(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/users", url.Values{"name": {"  Ada  "}}))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Location"))

	id, ok := f.cookies.UserID(followUp(t, recorder))
	require.True(t, ok)

	user, err := f.store.UserByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, "Ada", user.Name, "the name is trimmed")
	assert.Equal(t, db.AvatarColor("sky"), user.AvatarColor)
}

func TestAddUserRejectsBlankName(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	recorder := f.serve(t, postForm(t, "/users", url.Values{"name": {"   "}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Add a name first.")
	assert.Nil(t, cookieFrom(recorder, auth.UserCookie))

	users, err := f.store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestAddUserRejectsLongName(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	name := strings.Repeat("a", 25)

	recorder := f.serve(t, postForm(t, "/users", url.Values{"name": {name}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "at most 24 characters")
	assert.Contains(t, recorder.Body.String(), name, "the typed name comes back")

	users, err := f.store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestAddUserAcceptsMaxLengthName(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	name := strings.Repeat("a", 24)

	recorder := f.serve(t, postForm(t, "/users", url.Values{"name": {name}}))

	assert.Equal(t, http.StatusSeeOther, recorder.Code)

	users, err := f.store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, name, users[0].Name)
}

func TestAddUserRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	f.addUser(t, "Ada")

	recorder := f.serve(t, postForm(t, "/users", url.Values{"name": {"ada"}}))

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "already taken")
	assert.Contains(t, recorder.Body.String(), `value="ada"`, "the typed name comes back")
	assert.Nil(t, cookieFrom(recorder, auth.UserCookie))

	users, err := f.store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestLoginRenderErrorAnswers500(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	mux := http.NewServeMux()
	auth.NewHandlers(zerolog.Nop(), f.store, f.cookies, failingRenderer{}, nil).Routes(mux)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, getRequest(t, "/login"))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

// failingRenderer stands in for a template that cannot render.
type failingRenderer struct{}

func (failingRenderer) Render(string, any) ([]byte, error) {
	return nil, errors.New("broken template")
}

// itoa keeps form values readable.
func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
