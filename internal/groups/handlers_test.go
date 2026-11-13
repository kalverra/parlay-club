package groups_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/groups"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// joinFixture wires the invite screens the way the server does: RequireUser and
// Scope around an app mux that holds the shell and the join routes.
type joinFixture struct {
	mux     http.Handler
	store   *db.Store
	cookies *auth.Cookies
	group   groups.Group
	member  db.User
	shell   *shellProbe
}

// shellProbe records the group the shell ran in.
type shellProbe struct {
	called bool
	group  groups.Group
}

func (s *shellProbe) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s.called = true
	w.WriteHeader(http.StatusNoContent)
}

func newJoinFixture(t *testing.T) *joinFixture {
	t.Helper()

	return newJoinFixtureWithBaseURL(t, "")
}

// newJoinFixtureWithBaseURL wires the invite screens with a configured public
// origin, as cmd/server does from cfg.BaseURL.
func newJoinFixtureWithBaseURL(t *testing.T, baseURL string) *joinFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)
	cookies := auth.NewCookies("test-session-key-long-enough-for-anything", false)

	ada, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	authHandlers := auth.NewHandlers(zerolog.Nop(), store, cookies, renderer, nil, true)
	groupHandlers := groups.NewHandlers(zerolog.Nop(), store, auth.UserFrom, cookies, renderer, baseURL)
	scope := groups.Scope(zerolog.Nop(), store, auth.UserFrom, cookies)

	shell := &shellProbe{}
	mux := http.NewServeMux()
	authHandlers.Routes(mux)

	app := http.NewServeMux()
	app.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		shell.group, _ = groups.From(r.Context())
		shell.ServeHTTP(w, r)
	})
	groupHandlers.Routes(app)
	mux.Handle("/", authHandlers.RequireUser(scope(app)))

	return &joinFixture{
		mux:     authHandlers.CSRF(mux),
		store:   store,
		cookies: cookies,
		group:   group,
		member:  ada,
		shell:   shell,
	}
}

// serve runs request with cookies and returns the response.
func (f *joinFixture) serve(t *testing.T, request *http.Request, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	f.mux.ServeHTTP(recorder, request)
	return recorder
}

// request builds a request with no cookies.
func request(t *testing.T, method, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), method, target, nil)
}

// htmx marks request as an HTMX request and gives it a form body.
func htmx(request *http.Request) *http.Request {
	request.Header.Set("Hx-Request", "true")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

// cookie returns the last named cookie the response set, or nil. A handler that
// changes the active group writes after Scope, so its cookie wins in a browser.
func cookie(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	var found *http.Cookie
	for _, candidate := range recorder.Result().Cookies() {
		if candidate.Name == name {
			found = candidate
		}
	}
	return found
}

// lastGroupCookie returns the pc_group cookie a browser would keep.
func lastGroupCookie(recorder *httptest.ResponseRecorder) *http.Cookie {
	return cookie(recorder, auth.GroupCookie)
}

func TestJoinFormShowsTheGroupAndMembers(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	newcomer := f.addUser(t)

	recorder := f.serve(t,
		request(t, http.MethodGet, "/join/"+f.group.InviteToken),
		f.sessionCookie(t, newcomer.ID),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	body := recorder.Body.String()
	assert.Contains(t, body, "Join Sunday Parlay Club?")
	assert.Contains(t, body, "Ada")
	assert.Contains(t, body, `hx-post="/join/`+f.group.InviteToken+`"`)
	assert.NotContains(t, body, "already in this group")
}

func TestJoinFormShowsWhenAlreadyAMember(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)

	recorder := f.serve(t,
		request(t, http.MethodGet, "/join/"+f.group.InviteToken),
		f.sessionCookie(t, f.member.ID),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "You're already in this group.")
	assert.NotContains(t, body, `hx-post="/join/`)
}

func TestJoinFormIsNotFoundForAnUnknownToken(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)

	recorder := f.serve(t,
		request(t, http.MethodGet, "/join/no-such-token"),
		f.sessionCookie(t, f.member.ID),
	)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestJoinAddsTheMemberAndSwitchesToTheGroup(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	newcomer := f.addUser(t)
	session := f.sessionCookie(t, newcomer.ID)

	recorder := f.serve(t,
		htmx(request(t, http.MethodPost, "/join/"+f.group.InviteToken)),
		session,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/", recorder.Header().Get("Hx-Redirect"))

	groupCookie := lastGroupCookie(recorder)
	require.NotNil(t, groupCookie)
	id, ok := f.cookies.GroupID(requestWithGroupCookie(t, groupCookie))
	require.True(t, ok)
	assert.Equal(t, f.group.ID, id, "the new group becomes active")

	member, err := f.store.IsMember(t.Context(), f.group.ID, newcomer.ID)
	require.NoError(t, err)
	assert.True(t, member)
}

func TestJoinTwiceChangesNothing(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	newcomer := f.addUser(t)
	session := f.sessionCookie(t, newcomer.ID)

	for range 2 {
		recorder := f.serve(t,
			htmx(request(t, http.MethodPost, "/join/"+f.group.InviteToken)),
			session,
		)
		require.Equal(t, http.StatusOK, recorder.Code)
	}

	groups, err := f.store.ForUser(t.Context(), newcomer.ID)
	require.NoError(t, err)
	assert.Len(t, groups, 2, "the solo group plus the joined one")

	members, err := f.store.Members(t.Context(), f.group.ID)
	require.NoError(t, err)
	count := 0
	for _, member := range members {
		if member.ID == newcomer.ID {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestJoinAfterResetUsesTheNewLink(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	newcomer := f.addUser(t)
	session := f.sessionCookie(t, newcomer.ID)

	token, err := f.store.ResetInvite(t.Context(), f.group.ID)
	require.NoError(t, err)

	old := f.serve(t, request(t, http.MethodGet, "/join/"+f.group.InviteToken), session)
	assert.Equal(t, http.StatusNotFound, old.Code, "the old link is dead")

	page := f.serve(t, request(t, http.MethodGet, "/join/"+token), session)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "Join Sunday Parlay Club?")

	joined := f.serve(t, htmx(request(t, http.MethodPost, "/join/"+token)), session)
	require.Equal(t, http.StatusOK, joined.Code)
	assert.Equal(t, "/", joined.Header().Get("Hx-Redirect"))
}

func TestSignedOutVisitorSignsInAndJoins(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	invite := "/join/" + f.group.InviteToken

	// Following the invite signed out sends the visitor to sign in and keeps
	// the path.
	guest := f.serve(t, request(t, http.MethodGet, invite))
	require.Equal(t, http.StatusFound, guest.Code)
	assert.Equal(t, "/login", guest.Header().Get("Location"))
	returnCookie := cookie(guest, auth.OAuthCookie)
	require.NotNil(t, returnCookie, "the invite path is kept for after sign-in")

	// Signing in lands back on the invite.
	login := f.serve(t, htmxPostForm(t, "/dev/login", url.Values{
		"email": {"bob@example.com"},
		"name":  {"Bob"},
	}), returnCookie)
	require.Equal(t, http.StatusSeeOther, login.Code)
	assert.Equal(t, invite, login.Header().Get("Location"))

	session := cookie(login, auth.SessionCookie)
	require.NotNil(t, session)

	// The join page loads for the new person.
	page := f.serve(t, request(t, http.MethodGet, invite), session)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "Join Sunday Parlay Club?")

	// Joining switches them to the group and sends them to the slips.
	joined := f.serve(t, htmx(request(t, http.MethodPost, invite)), session)
	require.Equal(t, http.StatusOK, joined.Code)
	assert.Equal(t, "/", joined.Header().Get("Hx-Redirect"))

	// The home page now runs in the joined group, so it sees that group's
	// slips.
	groupCookie := lastGroupCookie(joined)
	require.NotNil(t, groupCookie)
	home := f.serve(t, request(t, http.MethodGet, "/"), session, groupCookie)
	require.Equal(t, http.StatusNoContent, home.Code)
	assert.True(t, f.shell.called)
	assert.Equal(t, f.group, f.shell.group)
}

// groupPath builds the path for one group action.
func groupPath(id int64, action string) string {
	return "/groups/" + strconv.FormatInt(id, 10) + "/" + action
}

func TestGroupsPageListsGroupsAndInviteLinks(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	second, err := f.store.Create(t.Context(), f.member.ID, "Lake House")
	require.NoError(t, err)

	recorder := f.serve(t, request(t, http.MethodGet, "/groups"), f.sessionCookie(t, f.member.ID))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Sunday Parlay Club")
	assert.Contains(t, body, "Lake House")
	assert.Contains(t, body, f.group.InviteToken)
	assert.Contains(t, body, second.InviteToken)
	assert.Contains(t, body, `hx-post="/groups"`)
	assert.Contains(t, body, groupPath(f.group.ID, "reset-invite"))
	assert.Contains(t, body, groupPath(f.group.ID, "leave"))
	assert.Equal(t, 2, strings.Count(body, ">Active</span"),
		"one Active badge on the card, one in the top-bar switcher")
	assert.Contains(t, body, "data-group-switcher", "two groups show the top-bar switcher")
}

// proxyHost is the Cloud Run hostname a rewriting proxy sends as Host, which
// is not the public URL people should paste.
const proxyHost = "parlay-club-xyz.us-central1.run.app"

func TestGroupsPageBuildsInviteLinksFromTheConfiguredBaseURL(t *testing.T) {
	t.Parallel()

	f := newJoinFixtureWithBaseURL(t, "https://parlay.example.com/")
	req := request(t, http.MethodGet, "/groups")
	req.Host = proxyHost

	recorder := f.serve(t, req, f.sessionCookie(t, f.member.ID))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "https://parlay.example.com/join/"+f.group.InviteToken)
	assert.NotContains(t, body, proxyHost, "the proxy's Host never reaches the invite link")
}

func TestGroupsPageFallsBackToTheRequestHostWithoutABaseURL(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	req := request(t, http.MethodGet, "/groups")
	req.Host = proxyHost
	req.Header.Set("X-Forwarded-Proto", "https")

	recorder := f.serve(t, req, f.sessionCookie(t, f.member.ID))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "https://"+proxyHost+"/join/"+f.group.InviteToken)
}

func TestGroupsPageRefusesToLeaveTheOnlyGroup(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)

	recorder := f.serve(t, request(t, http.MethodGet, "/groups"), f.sessionCookie(t, f.member.ID))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "You can't leave your only group.")
	assert.NotContains(t, body, groupPath(f.group.ID, "leave"))
}

func TestCreateGroupAddsItAndSwitchesToIt(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	session := f.sessionCookie(t, f.member.ID)

	recorder := f.serve(t, htmxPostForm(t, "/groups", url.Values{"name": {"Lake House"}}), session)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/groups", recorder.Header().Get("Hx-Redirect"))

	memberships, err := f.store.ForUser(t.Context(), f.member.ID)
	require.NoError(t, err)
	require.Len(t, memberships, 2)

	created := memberships[1]
	assert.Equal(t, "Lake House", created.Name)

	groupCookie := lastGroupCookie(recorder)
	require.NotNil(t, groupCookie)
	id, ok := f.cookies.GroupID(requestWithGroupCookie(t, groupCookie))
	require.True(t, ok)
	assert.Equal(t, created.ID, id, "the new group becomes active")
}

func TestCreateGroupWithABlankNameIsRefused(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	session := f.sessionCookie(t, f.member.ID)

	recorder := f.serve(t, htmxPostForm(t, "/groups", url.Values{"name": {"  "}}), session)

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

	memberships, err := f.store.ForUser(t.Context(), f.member.ID)
	require.NoError(t, err)
	assert.Len(t, memberships, 1, "no group was created")
}

func TestResetInviteKillsTheOldLink(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	session := f.sessionCookie(t, f.member.ID)
	oldToken := f.group.InviteToken

	recorder := f.serve(
		t,
		htmxPostForm(t, groupPath(f.group.ID, "reset-invite"), url.Values{}),
		session,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "true", recorder.Header().Get("Hx-Refresh"))

	dead := f.serve(t, request(t, http.MethodGet, "/join/"+oldToken), session)
	assert.Equal(t, http.StatusNotFound, dead.Code, "the old link stops working")

	memberships, err := f.store.ForUser(t.Context(), f.member.ID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.NotEqual(t, oldToken, memberships[0].InviteToken)

	live := f.serve(t, request(t, http.MethodGet, "/join/"+memberships[0].InviteToken), session)
	assert.Equal(t, http.StatusOK, live.Code)
}

func TestLeaveRemovesTheMembershipAndPicksAnotherGroup(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	second, err := f.store.Create(t.Context(), f.member.ID, "Lake House")
	require.NoError(t, err)
	session := f.sessionCookie(t, f.member.ID)

	recorder := f.serve(
		t,
		htmxPostForm(t, groupPath(f.group.ID, "leave"), url.Values{}),
		session,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "/groups", recorder.Header().Get("Hx-Redirect"))

	member, err := f.store.IsMember(t.Context(), f.group.ID, f.member.ID)
	require.NoError(t, err)
	assert.False(t, member)

	groupCookie := lastGroupCookie(recorder)
	require.NotNil(t, groupCookie)
	id, ok := f.cookies.GroupID(requestWithGroupCookie(t, groupCookie))
	require.True(t, ok)
	assert.Equal(t, second.ID, id, "the remaining group becomes active")
}

func TestLeaveTheOnlyGroupIsRefused(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)

	recorder := f.serve(
		t,
		htmxPostForm(t, groupPath(f.group.ID, "leave"), url.Values{}),
		f.sessionCookie(t, f.member.ID),
	)

	assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)

	member, err := f.store.IsMember(t.Context(), f.group.ID, f.member.ID)
	require.NoError(t, err)
	assert.True(t, member)
}

func TestSwitchChangesTheActiveGroup(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	second, err := f.store.Create(t.Context(), f.member.ID, "Lake House")
	require.NoError(t, err)

	recorder := f.serve(
		t,
		htmxPostForm(t, "/groups/switch", url.Values{
			"group_id": {strconv.FormatInt(second.ID, 10)},
		}),
		f.sessionCookie(t, f.member.ID),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "true", recorder.Header().Get("Hx-Refresh"))

	groupCookie := lastGroupCookie(recorder)
	require.NotNil(t, groupCookie)
	id, ok := f.cookies.GroupID(requestWithGroupCookie(t, groupCookie))
	require.True(t, ok)
	assert.Equal(t, second.ID, id)
}

func TestSwitchToAGroupYouAreNotInIsNotFound(t *testing.T) {
	t.Parallel()

	f := newJoinFixture(t)
	other := f.addUser(t)
	memberships, err := f.store.ForUser(t.Context(), other.ID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)

	recorder := f.serve(
		t,
		htmxPostForm(t, "/groups/switch", url.Values{
			"group_id": {strconv.FormatInt(memberships[0].ID, 10)},
		}),
		f.sessionCookie(t, f.member.ID),
	)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// sessionCookie returns a signed session cookie for id.
func (f *joinFixture) sessionCookie(t *testing.T, id int64) *http.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	f.cookies.SetSession(recorder, id)

	session := cookie(recorder, auth.SessionCookie)
	require.NotNil(t, session)
	return session
}

// addUser seeds a user with a solo group, as sign-in does.
func (f *joinFixture) addUser(t *testing.T) db.User {
	t.Helper()

	user, err := f.store.CreateUserWithIdentity(
		t.Context(), "Bob", "bob@example.com", "google-bob", "")
	require.NoError(t, err)
	_, err = f.store.Create(t.Context(), user.ID, "Just me")
	require.NoError(t, err)
	return user
}

// htmxPostForm builds an HTMX form post.
func htmxPostForm(t *testing.T, target string, values url.Values) *http.Request {
	t.Helper()

	request := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, target, strings.NewReader(values.Encode()))
	return htmx(request)
}

// requestWithGroupCookie builds a GET request carrying a group cookie.
func requestWithGroupCookie(t *testing.T, groupCookie *http.Cookie) *http.Request {
	t.Helper()

	request := request(t, http.MethodGet, "/")
	request.AddCookie(groupCookie)
	return request
}
