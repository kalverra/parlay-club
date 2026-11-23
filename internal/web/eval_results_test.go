package web_test

import (
	"context"
	"html"
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

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

type evalResultsFixture struct {
	handler   http.Handler
	store     *db.Store
	service   *slips.Service
	evalCache *eval.Cache
	user      db.User
	group     db.Group
	week2026  db.Week
	signedIn  bool
	activeGrp db.Group
}

func newEvalResultsFixture(t *testing.T) *evalResultsFixture {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	user, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week2026, err := store.UpsertWeek(t.Context(), 2026, 2, 1)
	require.NoError(t, err)

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	hub := &live.Hub{}
	service := slips.New(store, hub, nil)
	evalCache := eval.NewCache()
	service.SetOnGradeChange(evalCache.InvalidateGroup)

	f := &evalResultsFixture{
		store:     store,
		service:   service,
		evalCache: evalCache,
		user:      user,
		group:     group,
		week2026:  week2026,
		signedIn:  true,
		activeGrp: group,
	}

	f.handler = webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return f.user, f.signedIn
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: f.activeGrp.ID, Name: f.activeGrp.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return f.activeGrp, true
		},
		Store:     store,
		Slips:     service,
		EvalCache: evalCache,
	})

	return f
}

func TestEvalResultsAuthEmptyStateAndInvalidSeason(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	// Unauthenticated request redirects to /login.
	f.signedIn = false
	unauth := httptest.NewRecorder()
	f.handler.ServeHTTP(unauth, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results", nil))
	assert.Equal(t, http.StatusSeeOther, unauth.Code)
	assert.Equal(t, "/login", unauth.Header().Get("Location"))

	// Authenticated empty state.
	f.signedIn = true
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?from=/slips/1", nil),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	body := html.UnescapeString(rec.Body.String())
	assert.Contains(t, body, "Grading Results")
	assert.Contains(t, body, "Sunday Parlay Club")
	assert.Contains(t, body, "No settled slips yet for All seasons.")
	assert.Contains(t, body, `href="/slips/1"`)
	assert.Contains(t, body, `href="/analyzer/guide?from=%2Fslips%2F1"`)

	// Unsafe ?from= is stripped.
	unsafe := httptest.NewRecorder()
	f.handler.ServeHTTP(
		unsafe,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?from=https://evil.example", nil),
	)
	require.Equal(t, http.StatusOK, unsafe.Code)
	assert.NotContains(t, unsafe.Body.String(), "evil.example")

	// Invalid ?season= returns 400.
	for _, raw := range []string{"abc", "-1"} {
		bad := httptest.NewRecorder()
		f.handler.ServeHTTP(
			bad,
			httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season="+raw, nil),
		)
		assert.Equal(t, http.StatusBadRequest, bad.Code)
	}

	// Guide page links to /analyzer/results.
	guide := httptest.NewRecorder()
	f.handler.ServeHTTP(
		guide,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/guide?from=/slips/1", nil),
	)
	require.Equal(t, http.StatusOK, guide.Code)
	assert.Contains(t, guide.Body.String(), `/analyzer/results?from=%2Fslips%2F1`)
}

func TestEvalResultsGroupIsolationAndFilters(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	bob, err := f.store.CreateUserWithIdentity(ctx, "Bob", "bob@example.com", "google-bob", "")
	require.NoError(t, err)
	otherGroup, err := f.store.Create(ctx, bob.ID, "Other Crew")
	require.NoError(t, err)

	// 2025 season in f.group: 2 weeks, 2 settled legs (1 WON, 1 LOST).
	wk2025_1, err := f.store.UpsertWeek(ctx, 2025, 2, 1)
	require.NoError(t, err)
	wk2025_2, err := f.store.UpsertWeek(ctx, 2025, 2, 2)
	require.NoError(t, err)

	s25_1, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		wk2025_1.ID,
		nil,
		"2025 Wk 1",
		db.SlateScopeFullWeek,
		1,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)
	l25_1, err := f.store.CreateLeg(
		ctx,
		db.Leg{SlipID: s25_1.ID, UserID: f.user.ID, MarketType: "ML", Selection: "BUF ML", Odds: -110},
	)
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, l25_1.ID, parlay.Won, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, s25_1.ID, parlay.Won, db.SlipGraded))

	s25_2, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		wk2025_2.ID,
		nil,
		"2025 Wk 2",
		db.SlateScopeFullWeek,
		1,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)
	l25_2, err := f.store.CreateLeg(
		ctx,
		db.Leg{SlipID: s25_2.ID, UserID: f.user.ID, MarketType: "ML", Selection: "KC ML", Odds: 110},
	)
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, l25_2.ID, parlay.Lost, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, s25_2.ID, parlay.Lost, db.SlipGraded))

	// 2026 season in f.group: 1 settled leg (WON).
	s26, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		f.week2026.ID,
		nil,
		"2026 Wk 1",
		db.SlateScopeFullWeek,
		1,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)
	l26, err := f.store.CreateLeg(
		ctx,
		db.Leg{SlipID: s26.ID, UserID: f.user.ID, MarketType: "ML", Selection: "DET ML", Odds: -120},
	)
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, l26.ID, parlay.Won, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, s26.ID, parlay.Won, db.SlipGraded))

	// otherGroup has 3 settled legs in 2026 that must never leak into f.group.
	otherSlip, err := f.store.CreateSlip(
		ctx,
		otherGroup.ID,
		f.week2026.ID,
		nil,
		"Other 2026",
		db.SlateScopeFullWeek,
		3,
		[]int64{bob.ID},
	)
	require.NoError(t, err)
	for range 3 {
		ol, err := f.store.CreateLeg(
			ctx,
			db.Leg{SlipID: otherSlip.ID, UserID: bob.ID, MarketType: "ML", Selection: "NYJ ML", Odds: 150},
		)
		require.NoError(t, err)
		require.NoError(t, f.store.SetLegResult(ctx, ol.ID, parlay.Lost, bob.Name))
	}
	require.NoError(t, f.store.SetSlipGrade(ctx, otherSlip.ID, parlay.Lost, db.SlipGraded))

	// Default picks latest graded season (2026) and isolates f.group (1 leg: 1-0-0, not 4 legs).
	recDefault := httptest.NewRecorder()
	f.handler.ServeHTTP(recDefault, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results", nil))
	require.Equal(t, http.StatusOK, recDefault.Code)
	bodyDefault := html.UnescapeString(recDefault.Body.String())
	assert.Contains(t, bodyDefault, "Sunday Parlay Club · 2026 season")
	assert.Contains(t, bodyDefault, "1-0-0")
	assert.NotContains(t, bodyDefault, "1-3-0")
	assert.Contains(t, bodyDefault, "2026 REG Wk 1")
	assert.NotContains(t, bodyDefault, "2025 REG Wk 1")

	// ?season=2025 filters to 2025.
	rec2025 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec2025,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2025&rubric=v1", nil),
	)
	require.Equal(t, http.StatusOK, rec2025.Code)
	body2025 := html.UnescapeString(rec2025.Body.String())
	assert.Contains(t, body2025, "Sunday Parlay Club · 2025 season")
	assert.Contains(t, body2025, "1-1-0")
	assert.Contains(t, body2025, "2025 REG Wk 1")
	assert.Contains(t, body2025, "2025 REG Wk 2")

	// ?season=all combines 2025 and 2026 for f.group (2-1-0).
	recAll := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recAll,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=all", nil),
	)
	require.Equal(t, http.StatusOK, recAll.Code)
	bodyAll := html.UnescapeString(recAll.Body.String())
	assert.Contains(t, bodyAll, "Sunday Parlay Club · All seasons")
	assert.Contains(t, bodyAll, "2-1-0")
}

func TestEvalResultsCacheInvalidatesAfterSettlement(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	slip, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		f.week2026.ID,
		nil,
		"Week 1",
		db.SlateScopeFullWeek,
		1,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)
	leg, err := f.store.CreateLeg(
		ctx,
		db.Leg{SlipID: slip.ID, UserID: f.user.ID, MarketType: "ML", Selection: "BUF ML", Odds: -110},
	)
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, leg.ID, parlay.Won, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip.ID, parlay.Won, db.SlipGraded))

	// First request computes and caches 1-0-0.
	rec1 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec1, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026", nil))
	require.Equal(t, http.StatusOK, rec1.Code)
	assert.Contains(t, rec1.Body.String(), "1-0-0")

	// Re-grade the leg via HTTP PUT /legs/{id}/result, which calls slips.Service.SetLegResult
	// and fires evalCache.InvalidateGroup(f.group.ID).
	form := url.Values{"result": {string(parlay.Lost)}}
	putReq := httptest.NewRequestWithContext(
		ctx,
		http.MethodPut,
		"/legs/"+strconv.FormatInt(leg.ID, 10)+"/result",
		strings.NewReader(form.Encode()),
	)
	putReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	putRec := httptest.NewRecorder()
	f.handler.ServeHTTP(putRec, putReq)
	require.Equal(t, http.StatusOK, putRec.Code)

	// Subsequent request must see the updated 0-1-0 outcome, not the stale 1-0-0 cache.
	rec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec2, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	body2 := rec2.Body.String()
	assert.Contains(t, body2, "0-1-0")
	assert.NotContains(t, body2, "1-0-0")
}
