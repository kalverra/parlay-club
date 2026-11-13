package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/sandbox"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

const sandboxTestSessionKey = "test-session-key-long-enough-for-anything"

// sandboxFixtures is the repository root the fixture registry reads from.
func sandboxFixtures(t *testing.T) fs.FS {
	t.Helper()

	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	return os.DirFS(root)
}

// sandboxCookies returns a signer the runtime and the test share, so a reset's
// reissued cookies can be read back with the same key.
func sandboxCookies() *auth.Cookies {
	return auth.NewCookies(sandboxTestSessionKey, false)
}

// openSandbox boots the real runtime against a fresh isolated file.
func openSandbox(t *testing.T, hub *live.Hub, cookies *auth.Cookies) (
	*sql.DB,
	*db.Store,
	*sandboxRuntime,
) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	runtime, err := newSandboxRuntime(
		t.Context(), database, store, hub, sandboxFixtures(t), zerolog.Nop(),
	)
	require.NoError(t, err)
	runtime.cookies = cookies
	return database, store, runtime
}

// sandboxApp is the full server wiring the sandbox tests drive: the real
// templates, auth middleware, group scope, and the sandbox routes.
type sandboxApp struct {
	server  *httptest.Server
	base    *url.URL
	store   *db.Store
	runtime *sandboxRuntime
	cookies *auth.Cookies
}

// sandboxHTTPResponse is one request's status, headers, and body.
type sandboxHTTPResponse struct {
	status int
	header http.Header
	body   string
}

// openSandboxApp boots the sandbox and mounts it the way main does, with the
// real middleware and templates instead of a test stub.
func openSandboxApp(t *testing.T) *sandboxApp {
	t.Helper()

	_, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())
	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)
	runtime.renderer = renderer

	handlers := auth.NewHandlers(zerolog.Nop(), store, runtime.cookies, renderer, nil, true)
	scope := groups.Scope(zerolog.Nop(), store, auth.UserFrom, runtime.cookies)
	groupHandlers := groups.NewHandlers(
		zerolog.Nop(), store, auth.UserFrom, runtime.cookies, renderer)
	shell := webui.NewWithDeps(webui.Dependencies{
		Logger:     zerolog.Nop(),
		Renderer:   renderer,
		UserFrom:   auth.UserFrom,
		GroupsFrom: groups.Options,
		GroupFrom:  groups.From,
		Store:      store,
		Slips:      runtime.slips,
		Now:        runtime.scenario.Now,
	})
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		handlers,
		groupHandlers,
		scope,
		nil,
		runtime.MountRoutes,
	)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	base, err := url.Parse(server.URL)
	require.NoError(t, err)
	return &sandboxApp{
		server:  server,
		base:    base,
		store:   store,
		runtime: runtime,
		cookies: runtime.cookies,
	}
}

// client returns a browser with its own cookie jar. Redirects are returned to
// the test instead of followed, so the middleware's responses stay visible.
func (a *sandboxApp) client(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// signInAsAda gives a browser the real session and group cookies for the first
// shared persona. Tests that need another view switch through the real route.
func (a *sandboxApp) signInAsAda(t *testing.T, client *http.Client) sandbox.Seed {
	t.Helper()

	seed := a.runtime.Seed()
	person, ok := seed.Person(sandbox.PersonaAda)
	require.True(t, ok, "persona %s is seeded", sandbox.PersonaAda)

	recorder := httptest.NewRecorder()
	a.cookies.SetSession(recorder, person.User.ID)
	a.cookies.SetGroup(recorder, person.GroupID)
	client.Jar.SetCookies(a.base, recorder.Result().Cookies())
	return seed
}

// request sends one form request. A nil form sends no body.
func (a *sandboxApp) request(
	t *testing.T,
	client *http.Client,
	method, path string,
	form url.Values,
	htmx bool,
) sandboxHTTPResponse {
	t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequestWithContext(
		t.Context(), method, a.server.URL+path, body)
	require.NoError(t, err)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if htmx {
		request.Header.Set("Hx-Request", "true")
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	content, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return sandboxHTTPResponse{
		status: response.StatusCode,
		header: response.Header,
		body:   string(content),
	}
}

// cookieIDs reads the signed session and group the browser currently carries.
func (a *sandboxApp) cookieIDs(t *testing.T, client *http.Client) (int64, int64) {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	for _, cookie := range client.Jar.Cookies(a.base) {
		request.AddCookie(cookie)
	}
	userID, ok := a.cookies.SessionID(request)
	require.True(t, ok, "the browser carries a signed session")
	groupID, ok := a.cookies.GroupID(request)
	require.True(t, ok, "the browser carries a signed group")
	return userID, groupID
}

func TestSandboxBootstrapReplaysTheMorningOffline(t *testing.T) {
	t.Parallel()

	database, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())

	assert.Equal(t, sandbox.Morning, runtime.scenario.Stage())
	assert.Equal(t, sandbox.MorningTime(), runtime.scenario.Now())
	assert.Equal(t, sandbox.Morning, runtime.transport.Stage())
	assert.Same(t, runtime.transport, runtime.client.Transport,
		"every sandbox client is on the replay, with no network fallback")
	assert.Equal(t, ingestTimeout, runtime.client.Timeout)

	week, err := store.LatestWeek(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2026, week.Season)
	assert.Equal(t, 9, week.WeekNum)
	games, err := store.GamesForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, games, 15, "the whole slate with Thursday, Sunday, and Monday is replayed")

	weatherGames := 0
	for _, game := range games {
		if game.Kickoff.Before(runtime.scenario.Now()) {
			continue
		}
		if game.Indoor {
			continue
		}
		require.NotNil(t, game.TempF, "outdoor game %s has a forecast", game.ID)
		weatherGames++
		if _, err := store.SyncState(t.Context(), refresh.WeatherSource(game.ID)); err != nil {
			t.Errorf("outdoor game %s has no weather sync state: %v", game.ID, err)
		}
	}
	assert.Equal(t, 10, weatherGames)

	linesState, err := store.SyncState(t.Context(), refresh.OddsLinesSource)
	require.NoError(t, err)
	require.NotNil(t, linesState.OddsCreditsRemaining)
	assert.Greater(t, *linesState.OddsCreditsRemaining, refresh.OddsLinesFloor)

	featured, err := store.GameByID(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	require.NotNil(t, featured.OddsEventID)
	assert.Equal(t, sandbox.FeaturedEventID, *featured.OddsEventID)
	rows, err := store.OddsForGame(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	assert.NotEmpty(t, rows, "the featured game has matched prices")

	_, hasProjection, err := store.NfeloForGame(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	assert.True(t, hasProjection, "nfelo projections landed for the slate")

	responses, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	assert.Empty(t, responses, "replayed calls never enter the response archive")

	saved, err := runtime.state.Load(t.Context())
	require.NoError(t, err)
	assert.Equal(t, sandbox.State{Stage: sandbox.Morning, Now: sandbox.MorningTime()}, saved)

	seed := runtime.Seed()
	assert.Len(t, seed.People, 4, "three shared personas and an outsider")
	assert.Len(t, seed.Legs, 6, "every shared persona has two named legs")
	slip, err := store.SlipByID(t.Context(), seed.Slip.ID)
	require.NoError(t, err)
	assert.Equal(t, db.SlipOpen, slip.Status, "the morning slip stays editable")
	assert.Equal(t, parlay.Pending, slip.Result)
	assert.Equal(t, len(seed.Legs), countRows(t, database, "legs"))
}

func TestSandboxFeaturedGameResearchReplaysWithoutAMiss(t *testing.T) {
	t.Parallel()

	_, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())

	// Opening the featured game's card syncs both rosters before the prop board.
	require.NoError(t, runtime.refresher.SyncGameProps(t.Context(), sandbox.FeaturedGameID))
	for _, team := range []string{"ATL", "CIN"} {
		roster, err := store.RosterForTeam(t.Context(), team)
		require.NoError(t, err, team)
		assert.NotEmpty(t, roster, team)
	}

	athleteID, team, err := store.ResolvePlayerMatch(t.Context(), "Joe Burrow", "ATL", "CIN")
	require.NoError(t, err)
	assert.Equal(t, "3915511", athleteID, "the props board's players resolve against the replayed roster")
	assert.Equal(t, "CIN", team)

	rows, err := store.OddsForGame(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	matched := 0
	for _, row := range rows {
		if row.PlayerName != "" && row.Team != "" {
			matched++
		}
	}
	assert.Positive(t, matched, "prop lines carry the team their player plays for")

	// The research page syncs the summary, props, and the player's game log.
	require.NoError(t, runtime.refresher.SyncGameResearch(t.Context(), sandbox.FeaturedGameID))
	game, err := store.GameByID(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	require.NotNil(t, game.InjuriesJSON)
	assert.NotEqual(t, "[]", *game.InjuriesJSON, "the research page has an injury report")

	require.NoError(t, runtime.refresher.SyncPlayerResearch(t.Context(), athleteID))
	log, err := store.PlayerGameLog(t.Context(), athleteID)
	require.NoError(t, err)
	assert.Positive(t, log.GamesPlayed, "player research has last-5 games")
}

func TestSandboxUncapturedGameResearchFallsBackToEmpty(t *testing.T) {
	t.Parallel()

	_, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())

	// Miami at Detroit is on the slate but is not the featured game, so the
	// scenario answers its optional research with labeled empty bodies.
	const gameID = "401873042"
	require.NoError(t, runtime.refresher.SyncGameProps(t.Context(), gameID))
	rows, err := store.OddsForGame(t.Context(), gameID)
	require.NoError(t, err)
	for _, row := range rows {
		assert.Empty(t, row.PlayerName, "a game without captured props shows no player props")
	}

	require.NoError(t, runtime.refresher.SyncGameResearch(t.Context(), gameID))
	game, err := store.GameByID(t.Context(), gameID)
	require.NoError(t, err)
	require.NotNil(t, game.InjuriesJSON)
	assert.Equal(t, "[]", *game.InjuriesJSON, "a game without a captured summary shows no injuries")
}

func TestSandboxRestartRestoresStageAndClock(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	fixtures := sandboxFixtures(t)

	first, err := newSandboxRuntime(
		t.Context(), database, store, &live.Hub{}, fixtures, zerolog.Nop(),
	)
	require.NoError(t, err)

	weather, err := store.SyncState(t.Context(), refresh.ScoreboardSource)
	require.NoError(t, err)
	marker, err := store.CreateUserWithIdentity(
		t.Context(), "Marker", "marker@example.com", "dev:marker@example.com", "")
	require.NoError(t, err)

	restored := sandbox.State{
		Stage: sandbox.Stage("all-clear"),
		Now:   sandbox.MorningTime().Add(4 * time.Hour),
	}
	require.NoError(t, first.state.Save(t.Context(), restored))

	second, err := newSandboxRuntime(
		t.Context(), database, store, &live.Hub{}, fixtures, zerolog.Nop(),
	)
	require.NoError(t, err)

	assert.Equal(t, restored, second.scenario.State(), "the stage and clock survive a restart")
	assert.Equal(t, restored.Stage, second.transport.Stage())

	after, err := store.SyncState(t.Context(), refresh.ScoreboardSource)
	require.NoError(t, err)
	assert.Equal(t, weather.FetchedAt, after.FetchedAt,
		"a restart does not rebuild or re-fetch the morning")

	stillThere, err := store.UserByID(t.Context(), marker.ID)
	require.NoError(t, err)
	assert.Equal(t, marker.Name, stillThere.Name, "manual edits and people survive a restart")
}

func TestSandboxAuthNeverBuildsGoogle(t *testing.T) {
	t.Parallel()

	provider, devLogin, err := authSettings(t.Context(), config.Config{
		AppEnv:             config.EnvDev,
		Sandbox:            true,
		GoogleClientID:     "real-looking-client-id",
		GoogleClientSecret: "real-looking-client-secret",
		BaseURL:            "https://parlay.example.com",
	})
	require.NoError(t, err)
	assert.Nil(t, provider, "sandbox startup never contacts the Google issuer")
	assert.True(t, devLogin, "the dev sign-in form is always available in the sandbox")

	provider, devLogin, err = authSettings(t.Context(), config.Config{AppEnv: config.EnvDev})
	require.NoError(t, err)
	assert.Nil(t, provider)
	assert.True(t, devLogin, "normal dev with no Google values keeps the dev form")
}

func TestSandboxResetRebuildsRepeatably(t *testing.T) {
	t.Parallel()

	database, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())
	first := runtime.Seed()
	require.Len(t, first.People, 4)
	require.Len(t, first.Legs, 6)

	// Leave manual edits behind: a person, a slip rename, a graded leg, and a
	// hand-written activity line. Reset must remove all of them.
	marker, err := store.CreateUserWithIdentity(
		t.Context(), "Marker", "marker@example.com", "dev:marker@example.com", "")
	require.NoError(t, err)
	_, err = runtime.slips.UpdateSlip(
		t.Context(), first.People[0].User.ID, first.Slip.ID, "Manual edit", 2)
	require.NoError(t, err)
	_, err = runtime.slips.SetLegResult(
		t.Context(), first.People[0].User.ID, first.Legs[sandbox.LegAdaBengalsML], parlay.Lost)
	require.NoError(t, err)
	_, err = store.AddActivity(t.Context(), first.Slip.ID, &marker.ID, "manual edit")
	require.NoError(t, err)

	one, err := runtime.Reset(t.Context())
	require.NoError(t, err)

	_, err = store.UserByEmail(t.Context(), "marker@example.com")
	require.ErrorIs(t, err, db.ErrUserNotFound, "reset removes a manual person")
	slip, err := store.SlipByID(t.Context(), one.Slip.ID)
	require.NoError(t, err)
	assert.Equal(t, first.Slip.Name, slip.Name, "reset removes a manual rename")
	assert.Equal(t, db.SlipOpen, slip.Status)
	assert.Equal(t, parlay.Pending, slip.Result)
	legs, err := store.LegsForSlip(t.Context(), one.Slip.ID)
	require.NoError(t, err)
	for _, leg := range legs {
		assert.Equal(t, parlay.Pending, leg.Result, "reset removes a manual grade from %s", leg.ID)
		assert.Nil(t, leg.GradedBy)
	}

	assert.Len(t, mustActiveUsers(t, store), 4)
	assert.Equal(t, 5, countRows(t, database, "groups"), "three solos, one shared, one outsider")
	assert.Equal(t, 7, countRows(t, database, "group_members"))
	assert.Equal(t, 1, countRows(t, database, "slips"))
	assert.Equal(t, 6, countRows(t, database, "legs"))
	assert.Equal(t, 6, countRows(t, database, "activity"))
	assert.Equal(t, 0, countRows(t, database, "api_responses"))
	assert.Equal(t, 0, countRows(t, database, "api_bodies"))
	assert.Equal(t, 1, countRows(t, database, "weeks"))
	assert.Equal(t, 15, countRows(t, database, "games"))
	assert.Equal(t, 1, countRows(t, database, "sandbox_state"))
	assertSandboxSync(t, database, store, one.Week)

	firstSnapshot := collectSandboxSnapshot(t, store, one)

	two, err := runtime.Reset(t.Context())
	require.NoError(t, err)
	assert.Equal(t, sandbox.State{Stage: sandbox.Morning, Now: sandbox.MorningTime()},
		runtime.scenario.State())
	assert.Equal(t, sandbox.Morning, runtime.transport.Stage())
	assert.Len(t, mustActiveUsers(t, store), 4)
	assert.Equal(t, 1, countRows(t, database, "slips"))
	assert.Equal(t, 0, countRows(t, database, "api_responses"))
	assertSandboxSync(t, database, store, two.Week)

	secondSnapshot := collectSandboxSnapshot(t, store, two)
	assert.Equal(t, firstSnapshot, secondSnapshot,
		"two resets leave the same people, memberships, slip, legs, activity, games, and cache state")
}

func TestSandboxResetLeavesTheNormalDevDatabaseAlone(t *testing.T) {
	t.Parallel()

	normalPath := filepath.Join(t.TempDir(), "parlay.db")
	sentinel := []byte("normal dev database sentinel")
	require.NoError(t, os.WriteFile(normalPath, sentinel, 0o600))

	_, _, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())
	_, err := runtime.Reset(t.Context())
	require.NoError(t, err)
	_, err = runtime.Reset(t.Context())
	require.NoError(t, err)

	//nolint:gosec // the test reads the file it just wrote in its own temp dir.
	unchanged, err := os.ReadFile(normalPath)
	require.NoError(t, err)
	assert.Equal(t, sentinel, unchanged, "sandbox startup and resets never open the normal dev file")
}

func TestSandboxResetClearsFailureBackoff(t *testing.T) {
	t.Parallel()

	database, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())
	first := runtime.Seed()

	// Make every source due and then fail, so the service remembers a pause far
	// beyond the morning clock.
	runtime.scenario.Set(sandbox.State{
		Stage: sandbox.Morning,
		Now:   sandbox.MorningTime().Add(24 * time.Hour),
	})
	runtime.transport.SetStage(sandbox.Stage("no-such-stage"))
	require.Error(t, runtime.refresher.RunOnce(t.Context()), "the replay stage is unknown")

	rebuilt, err := runtime.Reset(t.Context())
	require.NoError(t, err, "reset clears the old failure pauses before refreshing")
	assert.Equal(t, sandbox.MorningTime(), runtime.scenario.Now())
	assert.Equal(t, sandbox.Morning, runtime.transport.Stage())
	assert.Equal(t, first.SharedGroup.Name, rebuilt.SharedGroup.Name)
	assertSandboxSync(t, database, store, rebuilt.Week)
}

func TestSandboxResetPublishesAfterTheRebuild(t *testing.T) {
	t.Parallel()

	hub := &live.Hub{}
	_, store, runtime := openSandbox(t, hub, sandboxCookies())
	seed := runtime.Seed()

	weekSignals, unsubscribeWeek := hub.Subscribe(fmt.Sprintf("week:%d", seed.Week.ID))
	t.Cleanup(unsubscribeWeek)
	slipSignals, unsubscribeSlip := hub.Subscribe(fmt.Sprintf("slip:%d", seed.Slip.ID))
	t.Cleanup(unsubscribeSlip)

	rebuilt, err := runtime.Reset(t.Context())
	require.NoError(t, err)

	requireSignal(t, weekSignals, "week")
	requireSignal(t, slipSignals, "slip")

	games, err := store.GamesForWeek(t.Context(), rebuilt.Week.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, games, "the signal points at committed games")
	_, err = store.SlipByID(t.Context(), rebuilt.Slip.ID)
	require.NoError(t, err, "the signal points at a committed slip")
}

func TestSandboxResetRecoversTheCurrentSession(t *testing.T) {
	t.Parallel()

	hub := &live.Hub{}
	database, store, runtime := openSandbox(t, hub, sandboxCookies())
	cookies := runtime.cookies

	handlers := auth.NewHandlers(zerolog.Nop(), store, cookies, nil, nil, true)
	scope := groups.Scope(zerolog.Nop(), store, auth.UserFrom, cookies)

	var (
		activeGroup groups.Group
		hasGroup    bool
	)
	shell := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activeGroup, hasGroup = groups.From(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		handlers,
		nil,
		scope,
		nil,
		runtime.MountRoutes,
	)

	seed := runtime.Seed()
	ada, ok := seed.Person(sandbox.PersonaAda)
	require.True(t, ok)

	signed := httptest.NewRecorder()
	cookies.SetSession(signed, ada.User.ID)
	cookies.SetGroup(signed, ada.GroupID)
	sessionCookie := responseCookie(t, signed, auth.SessionCookie)
	groupCookie := responseCookie(t, signed, auth.GroupCookie)

	// The route obeys the normal guards: a plain POST is a cross-site refusal
	// and a signed-out HTMX request goes to the sign-in page.
	blocked := httptest.NewRecorder()
	blockedRequest := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/dev/sandbox/reset", nil)
	blockedRequest.AddCookie(sessionCookie)
	mux.ServeHTTP(blocked, blockedRequest)
	assert.Equal(t, http.StatusForbidden, blocked.Code)

	signedOut := httptest.NewRecorder()
	signedOutRequest := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/dev/sandbox/reset", nil)
	signedOutRequest.Header.Set("Hx-Request", "true")
	mux.ServeHTTP(signedOut, signedOutRequest)
	assert.Equal(t, http.StatusOK, signedOut.Code)
	assert.Equal(t, "/login", signedOut.Header().Get("Hx-Redirect"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/dev/sandbox/reset", nil)
	request.Header.Set("Hx-Request", "true")
	request.AddCookie(sessionCookie)
	request.AddCookie(groupCookie)
	mux.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "true", recorder.Header().Get("Hx-Refresh"))

	rebuilt := runtime.Seed()
	newAda, ok := rebuilt.Person(sandbox.PersonaAda)
	require.True(t, ok)
	recoveredSession := responseCookie(t, recorder, auth.SessionCookie)
	recoveredGroup := responseCookie(t, recorder, auth.GroupCookie)

	next := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	next.AddCookie(recoveredSession)
	next.AddCookie(recoveredGroup)
	id, ok := cookies.SessionID(next)
	require.True(t, ok)
	assert.Equal(t, newAda.User.ID, id, "the browser is signed in as the same persona after reset")
	groupID, ok := cookies.GroupID(next)
	require.True(t, ok)
	assert.Equal(t, newAda.GroupID, groupID, "the browser keeps the shared group after reset")

	page := httptest.NewRecorder()
	mux.ServeHTTP(page, next)
	assert.Equal(t, http.StatusNoContent, page.Code)
	require.True(t, hasGroup)
	assert.Equal(t, newAda.GroupID, activeGroup.ID, "the recovered session reaches the shared group")

	assert.Equal(t, 1, countRows(t, database, "slips"))
}

func TestSandboxControlsPageShowsScenarioAtPhoneWidth(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	client := app.client(t)
	app.signInAsAda(t, client)

	response := app.request(t, client, http.MethodGet, "/dev/sandbox", nil, false)
	require.Equal(t, http.StatusOK, response.status, response.body)
	assert.Equal(t, "no-store", response.header.Get("Cache-Control"))
	assert.Contains(t, response.header.Get("Content-Type"), "text/html")

	for _, want := range []string{
		"Sandbox controls",
		"morning",
		sandbox.MorningTime().UTC().Format(time.RFC3339),
		"Ada",
		"ada@sandbox.test",
		"Ben",
		"Cleo",
		"Dex",
		"dex@sandbox.test",
		sandbox.SharedGroupName,
		"Reset to Sunday morning",
		`hx-post="/dev/sandbox/reset"`,
		`hx-post="/dev/sandbox/persona"`,
		"Open shared invite",
		`<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">`,
		"grid-cols-1",
		"sm:grid-cols-2",
		"sm:flex-row",
	} {
		assert.Contains(t, response.body, want)
	}

	// The ordinary tabs share the same phone-first layout.
	for _, path := range []string{"/"} {
		page := app.request(t, client, http.MethodGet, path, nil, false)
		require.Equal(t, http.StatusOK, page.status, path)
		assert.Contains(t, page.body,
			`<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">`,
			"%s keeps the phone viewport", path)
	}

	// Games is a redirect now; the pick board owns the week.
	redirected := app.request(t, client, http.MethodGet, "/games", nil, false)
	require.Equal(t, http.StatusSeeOther, redirected.status)
	assert.Contains(t, redirected.header.Get("Location"), "/groups/")
}

func TestSandboxPersonaSwitchUsesRealSignInAndGroupScope(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	client := app.client(t)
	seed := app.signInAsAda(t, client)

	for _, id := range []sandbox.PersonaID{sandbox.PersonaBen, sandbox.PersonaCleo} {
		person, ok := seed.Person(id)
		require.True(t, ok)
		group, err := sandbox.PersonaGroup(t.Context(), app.store, person.Persona, person.User.ID)
		require.NoError(t, err)

		response := app.request(t, client, http.MethodPost, "/dev/sandbox/persona",
			url.Values{"persona": {string(id)}}, true)
		require.Equal(t, http.StatusOK, response.status, response.body)
		assert.Equal(t, "true", response.header.Get("Hx-Refresh"))

		userID, groupID := app.cookieIDs(t, client)
		assert.Equal(t, person.User.ID, userID, "the session cookie names the real signed-in person")
		assert.Equal(t, group.ID, groupID, "the group cookie names the shared group")
		assert.Equal(t, sandbox.SharedGroupName, group.Name)

		// The shared slip is visible from every shared persona's perspective.
		slip := app.request(t, client, http.MethodGet,
			fmt.Sprintf("/slips/%d", seed.Slip.ID), nil, false)
		require.Equal(t, http.StatusOK, slip.status, slip.body)
		assert.Contains(t, slip.body, seed.Slip.Name)

		profile := app.request(t, client, http.MethodGet, "/me", nil, false)
		require.Equal(t, http.StatusOK, profile.status, profile.body)
		assert.Contains(t, profile.body, person.User.Name)
		assert.Contains(t, profile.body, "Ada")
		assert.Contains(t, profile.body, "Ben")
		assert.Contains(t, profile.body, "Cleo")
	}
}

func TestSandboxActivityNamesTheActingPersona(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	client := app.client(t)
	seed := app.signInAsAda(t, client)

	response := app.request(t, client, http.MethodPost, "/dev/sandbox/persona",
		url.Values{"persona": {string(sandbox.PersonaBen)}}, true)
	require.Equal(t, http.StatusOK, response.status, response.body)

	ben, ok := seed.Person(sandbox.PersonaBen)
	require.True(t, ok)
	legID := seed.Legs[sandbox.LegBenEaglesML]
	graded := app.request(t, client, http.MethodPut,
		fmt.Sprintf("/legs/%d/result", legID),
		url.Values{"result": {string(parlay.Lost)}}, true)
	require.Equal(t, http.StatusOK, graded.status, graded.body)

	entries, err := app.store.ActivityForSlip(t.Context(), seed.Slip.ID)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	newest := entries[0]
	require.NotNil(t, newest.UserID)
	assert.Equal(t, ben.User.ID, *newest.UserID, "the activity line belongs to the acting persona")
	assert.Contains(t, newest.Message, "Ben set")

	leg, err := app.store.LegByID(t.Context(), legID)
	require.NoError(t, err)
	require.NotNil(t, leg.GradedBy)
	assert.Equal(t, "Ben", *leg.GradedBy)
}

func TestSandboxOutsiderIsolationAndInviteJoin(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	client := app.client(t)
	seed := app.signInAsAda(t, client)

	switchResponse := app.request(t, client, http.MethodPost, "/dev/sandbox/persona",
		url.Values{"persona": {string(sandbox.PersonaDex)}}, true)
	require.Equal(t, http.StatusOK, switchResponse.status, switchResponse.body)

	dex, ok := seed.Person(sandbox.PersonaDex)
	require.True(t, ok)
	userID, groupID := app.cookieIDs(t, client)
	assert.Equal(t, dex.User.ID, userID)
	assert.Equal(t, dex.GroupID, groupID, "the outsider keeps their own group")

	sharedSlip := fmt.Sprintf("/slips/%d", seed.Slip.ID)
	blocked := app.request(t, client, http.MethodGet, sharedSlip, nil, false)
	assert.Equal(t, http.StatusNotFound, blocked.status,
		"the outsider gets the normal 404 for a shared slip")

	blockedGrade := app.request(t, client, http.MethodPut,
		fmt.Sprintf("/legs/%d/result", seed.Legs[sandbox.LegAdaBengalsML]),
		url.Values{"result": {string(parlay.Won)}}, true)
	assert.Equal(t, http.StatusNotFound, blockedGrade.status)

	// A forged group cookie is only a preference: scope falls back to the
	// outsider's own group and the shared slip stays hidden.
	forged := httptest.NewRecorder()
	app.cookies.SetGroup(forged, seed.SharedGroup.ID)
	client.Jar.SetCookies(app.base, forged.Result().Cookies())
	stillBlocked := app.request(t, client, http.MethodGet, sharedSlip, nil, false)
	assert.Equal(t, http.StatusNotFound, stillBlocked.status,
		"the group cookie cannot grant membership")
	_, recoveredGroup := app.cookieIDs(t, client)
	assert.Equal(t, dex.GroupID, recoveredGroup, "scope rewrites the forged cookie to a real group")

	joinPath := "/join/" + seed.SharedGroup.InviteToken
	joinPage := app.request(t, client, http.MethodGet, joinPath, nil, false)
	require.Equal(t, http.StatusOK, joinPage.status, joinPage.body)
	assert.Contains(t, joinPage.body, sandbox.SharedGroupName)

	joined := app.request(t, client, http.MethodPost, joinPath, nil, true)
	require.Equal(t, http.StatusOK, joined.status, joined.body)
	assert.Equal(t, "/", joined.header.Get("Hx-Redirect"))

	_, joinedGroup := app.cookieIDs(t, client)
	assert.Equal(t, seed.SharedGroup.ID, joinedGroup, "the normal join sets the shared group active")

	visible := app.request(t, client, http.MethodGet, sharedSlip, nil, false)
	assert.Equal(t, http.StatusOK, visible.status, "the invite join makes shared state visible")
}

func TestSandboxControlsObeyCSRFAndAuth(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	seed := app.runtime.Seed()

	signedOut := app.client(t)
	page := app.request(t, signedOut, http.MethodGet, "/dev/sandbox", nil, false)
	assert.Equal(t, http.StatusFound, page.status)
	assert.Equal(t, "/login", page.header.Get("Location"))

	signedOutPOST := app.request(t, signedOut, http.MethodPost, "/dev/sandbox/persona",
		url.Values{"persona": {string(sandbox.PersonaBen)}}, true)
	assert.Equal(t, http.StatusOK, signedOutPOST.status)
	assert.Equal(t, "/login", signedOutPOST.header.Get("Hx-Redirect"))

	client := app.client(t)
	app.signInAsAda(t, client)
	ada, ok := seed.Person(sandbox.PersonaAda)
	require.True(t, ok)

	plainReset := app.request(t, client, http.MethodPost, "/dev/sandbox/reset", nil, false)
	assert.Equal(t, http.StatusForbidden, plainReset.status)

	plainSwitch := app.request(t, client, http.MethodPost, "/dev/sandbox/persona",
		url.Values{"persona": {string(sandbox.PersonaBen)}}, false)
	assert.Equal(t, http.StatusForbidden, plainSwitch.status)

	unknown := app.request(t, client, http.MethodPost, "/dev/sandbox/persona",
		url.Values{"persona": {"malory"}}, true)
	assert.Equal(t, http.StatusUnprocessableEntity, unknown.status)

	userID, groupID := app.cookieIDs(t, client)
	assert.Equal(t, ada.User.ID, userID, "a refused action changes no session")
	assert.Equal(t, ada.GroupID, groupID, "a refused action changes no group")
	assert.Equal(t, 1, countRows(t, app.runtime.database, "slips"),
		"a refused reset changes no domain rows")
}

func TestSandboxControlRoutesStayOffOutsideSandbox(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)

	user, err := store.CreateUserWithIdentity(
		t.Context(), "Ada", "ada@example.com", "dev:ada@example.com", "")
	require.NoError(t, err)
	_, err = store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	cookies := sandboxCookies()
	handlers := auth.NewHandlers(zerolog.Nop(), store, cookies, nil, nil, true)
	scope := groups.Scope(zerolog.Nop(), store, auth.UserFrom, cookies)
	shell := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/dev/sandbox") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newMux(
		shell,
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		http.NotFoundHandler(),
		handlers,
		nil,
		scope,
		nil,
		nil,
	)

	signed := httptest.NewRecorder()
	cookies.SetSession(signed, user.ID)
	sessionCookie := responseCookie(t, signed, auth.SessionCookie)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/dev/sandbox"},
		{method: http.MethodPost, path: "/dev/sandbox/reset"},
		{method: http.MethodPost, path: "/dev/sandbox/stage"},
		{method: http.MethodPost, path: "/dev/sandbox/persona"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), test.method, test.path, nil)
			request.Header.Set("Hx-Request", "true")
			request.AddCookie(sessionCookie)
			mux.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusNotFound, recorder.Code,
				"sandbox routes are mounted only when SUNDAY_SANDBOX=1")
		})
	}
}

func TestSandboxStagesApplyDirectly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stage sandbox.Stage
		check func(t *testing.T, store *db.Store, seed sandbox.Seed)
	}{
		{
			stage: sandbox.EarlyLoss,
			check: func(t *testing.T, store *db.Store, seed sandbox.Seed) {
				t.Helper()

				featured, err := store.GameByID(t.Context(), sandbox.FeaturedGameID)
				require.NoError(t, err)
				assert.Equal(t, db.GameLive, featured.Status)
				require.NotNil(t, featured.HomeScore)
				require.NotNil(t, featured.AwayScore)
				assert.Equal(t, 17, *featured.HomeScore)
				assert.Equal(t, 10, *featured.AwayScore)

				slip, err := store.SlipByID(t.Context(), seed.Slip.ID)
				require.NoError(t, err)
				assert.Equal(t, db.SlipGraded, slip.Status)
				assert.Equal(t, parlay.Lost, slip.Result,
					"one lost leg settles the slip even while other legs are pending")

				legs, err := store.LegsForSlip(t.Context(), seed.Slip.ID)
				require.NoError(t, err)
				lost, pending := 0, 0
				for _, leg := range legs {
					switch leg.Result {
					case parlay.Lost:
						lost++
					case parlay.Pending:
						pending++
					}
				}
				assert.Equal(t, 1, lost)
				assert.Equal(t, 5, pending, "the other five picks stay unresolved")
			},
		},
		{
			stage: sandbox.AllClear,
			check: func(t *testing.T, store *db.Store, seed sandbox.Seed) {
				t.Helper()

				games, err := store.GamesForWeek(t.Context(), seed.Week.ID)
				require.NoError(t, err)
				final, scheduled := 0, 0
				for _, game := range games {
					switch game.Status {
					case db.GameFinal:
						final++
						require.NotNil(t, game.HomeScore, game.ID)
						require.NotNil(t, game.AwayScore, game.ID)
					case db.GameScheduled:
						scheduled++
					}
				}
				assert.Equal(t, 14, final, "Thursday and Sunday games are final")
				assert.Equal(t, 1, scheduled, "the Monday night game is still upcoming")

				slip, err := store.SlipByID(t.Context(), seed.Slip.ID)
				require.NoError(t, err)
				assert.Equal(t, db.SlipGraded, slip.Status)
				assert.Equal(t, parlay.Won, slip.Result)
			},
		},
		{
			stage: sandbox.SundayComplete,
			check: func(t *testing.T, store *db.Store, seed sandbox.Seed) {
				t.Helper()

				games, err := store.GamesForWeek(t.Context(), seed.Week.ID)
				require.NoError(t, err)
				require.Len(t, games, 15)
				for _, game := range games {
					assert.Equal(t, db.GameFinal, game.Status, game.ID)
					require.NotNil(t, game.HomeScore, game.ID)
					require.NotNil(t, game.AwayScore, game.ID)
				}

				slip, err := store.SlipByID(t.Context(), seed.Slip.ID)
				require.NoError(t, err)
				assert.Equal(t, parlay.Won, slip.Result)
			},
		},
	}

	for _, test := range tests {
		t.Run(string(test.stage), func(t *testing.T) {
			t.Parallel()

			_, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())
			seed, err := runtime.Stage(t.Context(), test.stage)
			require.NoError(t, err, "selecting a stage directly from a fresh morning")
			assertStageApplied(t, store, runtime, seed, test.stage)
			test.check(t, store, seed)
		})
	}
}

func TestSandboxStagesApplyInSequenceAndReset(t *testing.T) {
	t.Parallel()

	_, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())
	seed := runtime.Seed()

	for _, stage := range []sandbox.Stage{
		sandbox.EarlyLoss,
		sandbox.AllClear,
		sandbox.SundayComplete,
	} {
		updated, err := runtime.Stage(t.Context(), stage)
		require.NoError(t, err, stage)
		assertStageApplied(t, store, runtime, updated, stage)
		seed = updated
	}

	// Reset removes every manual edit and returns to the open morning slip.
	morning, err := runtime.Reset(t.Context())
	require.NoError(t, err)
	assertStageApplied(t, store, runtime, morning, sandbox.Morning)

	slip, err := store.SlipByID(t.Context(), morning.Slip.ID)
	require.NoError(t, err)
	assert.Equal(t, db.SlipOpen, slip.Status)
	assert.Equal(t, parlay.Pending, slip.Result)
	legs, err := store.LegsForSlip(t.Context(), morning.Slip.ID)
	require.NoError(t, err)
	for _, leg := range legs {
		assert.Equal(t, parlay.Pending, leg.Result, leg.ID)
	}
	assert.Equal(t, seed.Slip.Name, slip.Name)
}

func TestSandboxStagePublishesCommittedSignals(t *testing.T) {
	t.Parallel()

	hub := &live.Hub{}
	_, store, runtime := openSandbox(t, hub, sandboxCookies())
	seed := runtime.Seed()

	weekSignals, unsubscribeWeek := hub.Subscribe(fmt.Sprintf("week:%d", seed.Week.ID))
	t.Cleanup(unsubscribeWeek)
	slipSignals, unsubscribeSlip := hub.Subscribe(fmt.Sprintf("slip:%d", seed.Slip.ID))
	t.Cleanup(unsubscribeSlip)

	staged, err := runtime.Stage(t.Context(), sandbox.AllClear)
	require.NoError(t, err)
	requireSignal(t, weekSignals, "week")
	requireSignal(t, slipSignals, "slip")

	// The signals name committed rows a subscriber can refetch.
	slip, err := store.SlipByID(t.Context(), staged.Slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, slip.Result)
	games, err := store.GamesForWeek(t.Context(), staged.Week.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, games)
}

func TestSandboxStageRouteReissuesSessionAndShowsTheState(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	client := app.client(t)
	app.signInAsAda(t, client)

	response := app.request(t, client, http.MethodPost, "/dev/sandbox/stage",
		url.Values{"stage": {"all-clear"}}, true)
	require.Equal(t, http.StatusOK, response.status, response.body)
	assert.Equal(t, "true", response.header.Get("Hx-Refresh"))

	staged := app.runtime.Seed()
	ada, ok := staged.Person(sandbox.PersonaAda)
	require.True(t, ok)
	userID, groupID := app.cookieIDs(t, client)
	assert.Equal(t, ada.User.ID, userID, "the stage reissues the same persona's real session")
	assert.Equal(t, ada.GroupID, groupID)

	controls := app.request(t, client, http.MethodGet, "/dev/sandbox", nil, false)
	require.Equal(t, http.StatusOK, controls.status, controls.body)
	assert.Contains(t, controls.body, "all-clear")
	for _, want := range []string{
		"Sunday morning",
		"Early loss",
		"All clear",
		"Sunday complete",
		`hx-post="/dev/sandbox/stage"`,
		`name="stage" value="early-loss"`,
		"scripted manual",
	} {
		assert.Contains(t, controls.body, want)
	}

	games := app.request(t, client, http.MethodGet, "/games", nil, false)
	require.Equal(t, http.StatusSeeOther, games.status, games.body)
	assert.Contains(t, games.header.Get("Location"), "/groups/",
		"the old Games link lands on the active group's week")

	picks := app.request(t, client, http.MethodGet,
		fmt.Sprintf("/slips/%d/pick", staged.Slip.ID), nil, false)
	require.Equal(t, http.StatusOK, picks.status, picks.body)
	assert.Contains(t, picks.body, "already kicked off",
		"the pick board hides games that kicked off at the scenario clock")

	slip := app.request(t, client, http.MethodGet,
		fmt.Sprintf("/slips/%d", staged.Slip.ID), nil, false)
	require.Equal(t, http.StatusOK, slip.status, slip.body)
	assert.Contains(t, slip.body, "WON")
}

func TestSandboxUnknownStageChangesNothing(t *testing.T) {
	t.Parallel()

	app := openSandboxApp(t)
	client := app.client(t)
	seed := app.signInAsAda(t, client)
	beforeUser, beforeGroup := app.cookieIDs(t, client)
	beforeState := app.runtime.scenario.State()
	beforeSlips := countRows(t, app.runtime.database, "slips")
	beforeLegs := countRows(t, app.runtime.database, "legs")
	beforeActivity := countRows(t, app.runtime.database, "activity")

	response := app.request(t, client, http.MethodPost, "/dev/sandbox/stage",
		url.Values{"stage": {"playoffs"}}, true)
	assert.Equal(t, http.StatusUnprocessableEntity, response.status)

	assert.Equal(t, beforeState, app.runtime.scenario.State(), "an unknown stage changes no clock or stage")
	assert.Equal(t, sandbox.Morning, app.runtime.transport.Stage())
	afterUser, afterGroup := app.cookieIDs(t, client)
	assert.Equal(t, beforeUser, afterUser, "an unknown stage changes no session")
	assert.Equal(t, beforeGroup, afterGroup, "an unknown stage changes no group")
	assert.Equal(t, beforeSlips, countRows(t, app.runtime.database, "slips"))
	assert.Equal(t, beforeLegs, countRows(t, app.runtime.database, "legs"))
	assert.Equal(t, beforeActivity, countRows(t, app.runtime.database, "activity"))
	assert.Equal(t, seed.Slip.ID, app.runtime.Seed().Slip.ID)

	errResponse := app.request(t, client, http.MethodPost, "/dev/sandbox/stage",
		url.Values{"stage": {""}}, true)
	assert.Equal(t, http.StatusUnprocessableEntity, errResponse.status)

	_, err := app.runtime.Stage(t.Context(), sandbox.Stage("playoffs"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown stage "playoffs"`)
	assert.Equal(t, beforeState, app.runtime.scenario.State())
}

func TestSandboxStageChangeWinsOverPageLoadKick(t *testing.T) {
	t.Parallel()

	_, store, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())

	// Hold the transition lock while a page-load kick and a stage change queue
	// behind it. Whichever acquires it first, the chosen stage is the state a
	// later reader sees; an old morning refresh cannot overwrite it.
	runtime.mu.Lock()
	started := make(chan struct{}, 2)
	done := make(chan error, 1)
	go func() {
		runtime.Kick()
		started <- struct{}{}
	}()
	go func() {
		started <- struct{}{}
		_, err := runtime.Stage(t.Context(), sandbox.SundayComplete)
		done <- err
	}()
	<-started
	<-started
	runtime.mu.Unlock()

	require.NoError(t, <-done)
	require.Eventually(t, func() bool { return !runtime.kickPending.Load() },
		2*time.Second, 5*time.Millisecond, "the page-load pass finishes")

	assertStageApplied(t, store, runtime, runtime.Seed(), sandbox.SundayComplete)
	games, err := store.GamesForWeek(t.Context(), runtime.Seed().Week.ID)
	require.NoError(t, err)
	for _, game := range games {
		assert.Equal(t, db.GameFinal, game.Status,
			"a late page-load pass cannot put a scheduled morning game back")
	}
}

// TestSandboxShutdownCancelsPageLoadKick proves a stopped runtime refuses new
// page-load passes and that a second shutdown is safe.
func TestSandboxShutdownCancelsPageLoadKick(t *testing.T) {
	t.Parallel()

	_, _, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())

	require.NoError(t, runtime.Shutdown(t.Context()))
	runtime.Kick()
	assert.False(t, runtime.kickPending.Load(), "a stopped runtime starts no page-load pass")
	require.NoError(t, runtime.Shutdown(t.Context()), "a second shutdown is safe")
	assert.False(t, runtime.kickPending.Load(), "the kick is still refused")
}

// TestSandboxShutdownReportsTheDrainDeadline holds a pass that never finishes,
// so Shutdown hands the caller its deadline error instead of blocking the stop
// past Cloud Run's kill window.
func TestSandboxShutdownReportsTheDrainDeadline(t *testing.T) {
	t.Parallel()

	_, _, runtime := openSandbox(t, &live.Hub{}, sandboxCookies())

	runtime.kickWg.Add(1) // a pass that never finishes
	defer runtime.kickWg.Done()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, runtime.Shutdown(ctx), context.DeadlineExceeded)
}

// assertStageApplied checks the clock, replay stage, stage row, slip lock
// state, and every scripted result for one stage.
func assertStageApplied(
	t *testing.T,
	store *db.Store,
	runtime *sandboxRuntime,
	seed sandbox.Seed,
	stage sandbox.Stage,
) {
	t.Helper()

	plan, ok := sandbox.PlanFor(stage)
	require.True(t, ok, stage)
	want := sandbox.State{Stage: stage, Now: plan.Now}
	assert.Equal(t, want, runtime.scenario.State())
	assert.Equal(t, stage, runtime.transport.Stage())

	saved, err := runtime.state.Load(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, saved)

	wantResults := make(map[sandbox.LegID]parlay.Result, len(seed.Legs))
	for id := range seed.Legs {
		wantResults[id] = parlay.Pending
	}
	for _, grade := range plan.Grades {
		wantResults[grade.Leg] = grade.Result
	}
	for id, legID := range seed.Legs {
		leg, err := store.LegByID(t.Context(), legID)
		require.NoError(t, err)
		assert.Equal(t, wantResults[id], leg.Result, "%s %s", stage, id)
		if wantResults[id] == parlay.Pending {
			assert.Nil(t, leg.GradedBy, "%s %s", stage, id)
		} else {
			require.NotNil(t, leg.GradedBy, "%s %s", stage, id)
		}
	}

	members, err := store.SlipMembers(t.Context(), seed.Slip.ID)
	require.NoError(t, err)
	for _, member := range members {
		if plan.Lock {
			assert.NotNil(t, member.LockedAt, "%s locks %s", stage, member.User.Name)
		} else {
			assert.Nil(t, member.LockedAt, "%s leaves the slip editable", stage)
		}
	}
}

// sandboxSnapshot is the domain state a reset should reproduce, with numeric
// IDs and wall-clock metadata left out.
type sandboxSnapshot struct {
	People        []sandboxPersonSnapshot
	SharedGroup   sandboxGroupSnapshot
	OutsiderGroup sandboxGroupSnapshot
	Slip          sandboxSlipSnapshot
	Legs          []sandboxLegSnapshot
	Activity      []string
	Games         []string
	Sync          map[string]string
}

type sandboxPersonSnapshot struct {
	ID     sandbox.PersonaID
	Name   string
	Email  string
	Active bool
}

type sandboxGroupSnapshot struct {
	Name    string
	Members []string
}

type sandboxSlipSnapshot struct {
	Name          string
	Status        db.SlipStatus
	Result        parlay.Result
	LegsPerMember int
	Members       []string
}

type sandboxLegSnapshot struct {
	ID        sandbox.LegID
	Owner     string
	GameID    string
	Market    string
	Selection string
	Team      string
	Side      string
	Line      string
	Odds      int
	Result    parlay.Result
}

func collectSandboxSnapshot(t *testing.T, store *db.Store, seed sandbox.Seed) sandboxSnapshot {
	t.Helper()

	emails := make(map[int64]string, len(seed.People))
	people := make([]sandboxPersonSnapshot, 0, len(seed.People))
	for _, seeded := range seed.People {
		user, err := store.UserByID(t.Context(), seeded.User.ID)
		require.NoError(t, err)
		emails[user.ID] = user.Email
		people = append(people, sandboxPersonSnapshot{
			ID:     seeded.Persona.ID,
			Name:   user.Name,
			Email:  user.Email,
			Active: user.Active,
		})
	}

	slip, err := store.SlipByID(t.Context(), seed.Slip.ID)
	require.NoError(t, err)
	members, err := store.SlipMembers(t.Context(), seed.Slip.ID)
	require.NoError(t, err)
	memberEmails := make([]string, 0, len(members))
	for _, member := range members {
		memberEmails = append(memberEmails, member.User.Email)
	}
	sort.Strings(memberEmails)

	legs := make([]sandboxLegSnapshot, 0, len(seed.Legs))
	for id, legID := range seed.Legs {
		leg, err := store.LegByID(t.Context(), legID)
		require.NoError(t, err)
		legs = append(legs, sandboxLegSnapshot{
			ID:        id,
			Owner:     emails[leg.UserID],
			GameID:    pointerValue(leg.GameID),
			Market:    leg.MarketType,
			Selection: leg.Selection,
			Team:      pointerValue(leg.Team),
			Side:      pointerValue(leg.Side),
			Line:      lineValue(leg.Line),
			Odds:      leg.Odds,
			Result:    leg.Result,
		})
	}
	sort.Slice(legs, func(i, j int) bool { return legs[i].ID < legs[j].ID })

	entries, err := store.ActivityForSlip(t.Context(), seed.Slip.ID)
	require.NoError(t, err)
	activity := make([]string, 0, len(entries))
	for _, entry := range entries {
		activity = append(activity, entry.Message)
	}

	games, err := store.GamesForWeek(t.Context(), seed.Week.ID)
	require.NoError(t, err)
	gameStates := make([]string, 0, len(games))
	for _, game := range games {
		gameStates = append(gameStates, fmt.Sprintf(
			"%s %s %s @ %s %d-%d %s",
			game.ID,
			game.Status,
			game.AwayTeam,
			game.HomeTeam,
			pointerInt(game.AwayScore),
			pointerInt(game.HomeScore),
			game.Kickoff.UTC().Format(time.RFC3339),
		))
	}

	return sandboxSnapshot{
		People:        people,
		SharedGroup:   groupSnapshot(t, store, seed.SharedGroup),
		OutsiderGroup: groupSnapshot(t, store, seed.OutsiderGroup),
		Slip: sandboxSlipSnapshot{
			Name:          slip.Name,
			Status:        slip.Status,
			Result:        slip.Result,
			LegsPerMember: slip.LegsPerMember,
			Members:       memberEmails,
		},
		Legs:     legs,
		Activity: activity,
		Games:    gameStates,
		Sync:     syncSnapshot(t, store, seed.Week),
	}
}

func groupSnapshot(t *testing.T, store *db.Store, group db.Group) sandboxGroupSnapshot {
	t.Helper()

	members, err := store.Members(t.Context(), group.ID)
	require.NoError(t, err)
	emails := make([]string, 0, len(members))
	for _, member := range members {
		emails = append(emails, member.Email)
	}
	sort.Strings(emails)
	return sandboxGroupSnapshot{Name: group.Name, Members: emails}
}

func syncSnapshot(t *testing.T, store *db.Store, week db.Week) map[string]string {
	t.Helper()

	keys := syncKeys(t, store, week)
	snapshot := make(map[string]string, len(keys))
	for _, key := range keys {
		state, err := store.SyncState(t.Context(), key)
		require.NoError(t, err, key)
		snapshot[key] = state.FetchedAt.UTC().Format(time.RFC3339)
	}
	return snapshot
}

func assertSandboxSync(t *testing.T, database *sql.DB, store *db.Store, week db.Week) {
	t.Helper()

	keys := syncKeys(t, store, week)
	assert.Equal(t, len(keys), countRows(t, database, "sync_state"),
		"no stale or duplicate source rows survive a reset")
	for _, key := range keys {
		state, err := store.SyncState(t.Context(), key)
		require.NoError(t, err, key)
		assert.Equal(t, sandbox.MorningTime(), state.FetchedAt, key)
	}
}

func syncKeys(t *testing.T, store *db.Store, week db.Week) []string {
	t.Helper()

	keys := []string{
		refresh.ScoreboardSource,
		refresh.OddsLinesSource,
		refresh.NfeloProjectionsSource,
		refresh.OddsPropsSource(sandbox.FeaturedGameID),
		refresh.RosterSource(sandbox.FeaturedHomeESPNID),
		refresh.RosterSource(sandbox.FeaturedAwayESPNID),
	}
	games, err := store.GamesForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	for _, game := range games {
		if !game.Indoor && sandbox.MorningTime().Before(game.Kickoff) {
			keys = append(keys, refresh.WeatherSource(game.ID))
		}
	}
	sort.Strings(keys)
	return keys
}

func mustActiveUsers(t *testing.T, store *db.Store) []db.User {
	t.Helper()

	users, err := store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	return users
}

func countRows(t *testing.T, database *sql.DB, table string) int {
	t.Helper()

	var count int
	require.NoError(t, database.QueryRowContext(
		t.Context(), "SELECT count(*) FROM "+table).Scan(&count))
	return count
}

func requireSignal(t *testing.T, signals <-chan struct{}, topic string) {
	t.Helper()

	select {
	case <-signals:
	case <-time.After(2 * time.Second):
		t.Fatalf("no %s signal after reset", topic)
	}
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func lineValue(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%g", *value)
}

func pointerInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
