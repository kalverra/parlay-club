package groups_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/groups"
)

// fixture wires the group scope over a real database and real cookies.
type fixture struct {
	store   *db.Store
	cookies *auth.Cookies
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return &fixture{
		store:   store,
		cookies: auth.NewCookies("test-session-key-long-enough-for-anything", false),
	}
}

// addUser seeds a user with a unique email.
func (f *fixture) addUser(t *testing.T, name string) db.User {
	t.Helper()

	user, err := f.store.CreateUserWithIdentity(
		t.Context(), name, name+"@example.com", "google-"+name, "")
	require.NoError(t, err)
	return user
}

// createGroup makes a group with user as its first member.
func (f *fixture) createGroup(t *testing.T, user db.User, name string) groups.Group {
	t.Helper()

	group, err := f.store.Create(t.Context(), user.ID, name)
	require.NoError(t, err)
	return group
}

// groupCookie signs a pc_group cookie for id.
func (f *fixture) groupCookie(t *testing.T, id int64) *http.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	f.cookies.SetGroup(recorder, id)

	cookie := groupCookieFrom(recorder)
	require.NotNil(t, cookie)
	return cookie
}

// scope builds the middleware under test for user.
func (f *fixture) scope(user db.User) func(http.Handler) http.Handler {
	return groups.Scope(zerolog.Nop(), f.store, func(context.Context) (db.User, bool) {
		return user, true
	}, f.cookies)
}

// probe stands in for the handler the scope guards.
type probe struct {
	called   bool
	group    groups.Group
	hasGroup bool
}

func (p *probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.called = true
	p.group, p.hasGroup = groups.From(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// serve runs one request through handler.
func serve(t *testing.T, handler http.Handler, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// groupCookieFrom returns the pc_group cookie a recorder set, or nil.
func groupCookieFrom(recorder *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == auth.GroupCookie {
			return cookie
		}
	}
	return nil
}

func TestScopeFallsBackToFirstGroup(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.addUser(t, "Ada")
	first := f.createGroup(t, user, "First")
	f.createGroup(t, user, "Second")
	next := &probe{}

	recorder := serve(t, f.scope(user)(next))

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, next.called)
	assert.True(t, next.hasGroup)
	assert.Equal(t, first, next.group)

	cookie := groupCookieFrom(recorder)
	require.NotNil(t, cookie, "the fallback group is written to the cookie")
	_, ok := f.cookies.GroupID(requestWithCookie(t, cookie))
	assert.True(t, ok)
}

func TestScopeHonoursAChosenMemberGroup(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.addUser(t, "Ada")
	f.createGroup(t, user, "First")
	second := f.createGroup(t, user, "Second")
	next := &probe{}

	recorder := serve(t, f.scope(user)(next), f.groupCookie(t, second.ID))

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, next.called)
	assert.Equal(t, second, next.group)
	assert.Nil(t, groupCookieFrom(recorder), "a valid choice is not rewritten")
}

func TestScopeFallsBackOnAForgedCookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.addUser(t, "Ada")
	first := f.createGroup(t, user, "First")
	second := f.createGroup(t, user, "Second")
	//nolint:gosec // the signer set these attributes; the test tampers with the value.
	cookie := f.groupCookie(t, second.ID)
	cookie.Value = tampered(cookie.Value)
	next := &probe{}

	recorder := serve(t, f.scope(user)(next), cookie)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, next.called)
	assert.Equal(t, first, next.group)
	assert.NotNil(t, groupCookieFrom(recorder), "the fallback is written")
}

func TestScopeFallsBackOnAnotherUsersGroup(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ada := f.addUser(t, "Ada")
	adaGroup := f.createGroup(t, ada, "Ada's")
	grace := f.addUser(t, "Grace")
	graceGroup := f.createGroup(t, grace, "Grace's")
	next := &probe{}

	recorder := serve(t, f.scope(ada)(next), f.groupCookie(t, graceGroup.ID))

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, next.called)
	assert.Equal(t, adaGroup, next.group, "a group Ada does not belong to is ignored")
	assert.NotNil(t, groupCookieFrom(recorder), "the fallback is written")
}

func TestScopeAnswers500WhenTheUserHasNoGroup(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	user := f.addUser(t, "Ada")
	next := &probe{}

	recorder := serve(t, f.scope(user)(next))

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, next.called)
}

func TestScopeAnswers500WhenTheStoreFails(t *testing.T) {
	t.Parallel()

	next := &probe{}
	handler := groups.Scope(
		zerolog.Nop(),
		failingStore{},
		func(context.Context) (db.User, bool) { return db.User{ID: 1}, true },
		&stubCookie{},
	)(next)

	recorder := serve(t, handler)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, next.called)
}

// failingStore fails every membership lookup.
type failingStore struct{}

func (failingStore) Create(context.Context, int64, string) (groups.Group, error) {
	return groups.Group{}, errStore
}

func (failingStore) ForUser(context.Context, int64) ([]groups.Group, error) {
	return nil, errStore
}

func (failingStore) IsMember(context.Context, int64, int64) (bool, error) {
	return false, errStore
}

func (failingStore) JoinByToken(context.Context, int64, string) (groups.Group, error) {
	return groups.Group{}, errStore
}

func (failingStore) ResetInvite(context.Context, int64) (string, error) {
	return "", errStore
}

func (failingStore) Leave(context.Context, int64, int64) error {
	return errStore
}

// stubCookie reports no active group and records writes.
type stubCookie struct{}

func (*stubCookie) GroupID(*http.Request) (int64, bool) { return 0, false }

func (*stubCookie) SetGroup(http.ResponseWriter, int64) {}

var errStore = errors.New("store unavailable")

// requestWithCookie builds a GET request carrying cookie.
func requestWithCookie(t *testing.T, cookie *http.Cookie) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	return request
}

// tampered flips the first character of a signed value without shortening it.
func tampered(value string) string {
	if value[0] == '0' {
		return "1" + value[1:]
	}
	return "0" + value[1:]
}
