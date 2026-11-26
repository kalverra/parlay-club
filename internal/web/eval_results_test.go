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
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
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
	assert.Contains(t, body, "All groups")
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

// TestDocsEvalsServesResultsUnderTheDocsTab covers both URLs that land on the
// results screen. The docs alias keeps every filter, marks the Docs tab, and
// highlights the Eval Results switcher while forwarding a safe back link.
func TestDocsEvalsServesResultsUnderTheDocsTab(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	for _, path := range []string{
		"/docs/evals?source=market&season=2026&engine=v2&from=/slips/7",
		"/analyzer/results?source=market&season=2026&engine=v2&from=/slips/7",
	} {
		recorder := httptest.NewRecorder()
		f.handler.ServeHTTP(recorder, httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))

		require.Equal(t, http.StatusOK, recorder.Code, path)
		body := recorder.Body.String()

		assert.Contains(t, html.UnescapeString(body), "Market-wide · 2026 season · Engine v2", path)
		assert.Regexp(t, `<a\s+href="/docs"\s+class="flex flex-col[^>]*aria-current="page"`, body, path)
		assert.Contains(t, body, `aria-current="page">Eval Results</a`, path)
		assert.Contains(t, body, `href="/slips/7"`, path)
		assert.Contains(t, body, `href="/docs?from=%2Fslips%2F7"`, path)
		assert.Contains(t, body, `href="/docs/glossary?from=%2Fslips%2F7"`, path)
		assert.Contains(t, body, `href="/docs/grading?from=%2Fslips%2F7"`, path)
	}

	// The alias keeps the handler's 400 on an unreadable season.
	bad := httptest.NewRecorder()
	f.handler.ServeHTTP(bad, httptest.NewRequestWithContext(ctx, http.MethodGet, "/docs/evals?season=abc", nil))
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	// An unsafe back link is stripped, so nothing forwards it.
	unsafe := httptest.NewRecorder()
	f.handler.ServeHTTP(
		unsafe,
		httptest.NewRequestWithContext(
			ctx, http.MethodGet, "/docs/evals?from=https://evil.example/phish", nil),
	)
	require.Equal(t, http.StatusOK, unsafe.Code)
	assert.NotContains(t, unsafe.Body.String(), "evil.example")
	assert.NotContains(t, unsafe.Body.String(), "&larr; Back")
}

func TestEvalResultsGlobalAggregationAndFilters(t *testing.T) {
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

	// otherGroup has 3 settled legs in 2026 that are included in global results.
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

	// Default picks latest graded season (2026) and aggregates across all groups (1 WON + 3 LOST = 1-3-0).
	recDefault := httptest.NewRecorder()
	f.handler.ServeHTTP(recDefault, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results", nil))
	require.Equal(t, http.StatusOK, recDefault.Code)
	bodyDefault := html.UnescapeString(recDefault.Body.String())
	assert.Contains(t, bodyDefault, "All groups · 2026 season")
	assert.Contains(t, bodyDefault, "1-3-0")
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
	assert.Contains(t, body2025, "All groups · 2025 season")
	assert.Contains(t, body2025, "1-1-0")
	assert.Contains(t, body2025, "2025 REG Wk 1")
	assert.Contains(t, body2025, "2025 REG Wk 2")

	// ?season=all combines 2025 and 2026 across all groups (2-4-0).
	recAll := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recAll,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=all", nil),
	)
	require.Equal(t, http.StatusOK, recAll.Code)
	bodyAll := html.UnescapeString(recAll.Body.String())
	assert.Contains(t, bodyAll, "All groups · All seasons")
	assert.Contains(t, bodyAll, "2-4-0")
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

func TestEvalResultsDefaultsNilEvalCache(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return f.user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: f.group.ID, Name: f.group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return f.group, true
		},
		Store: f.store,
		Slips: f.service,
	})

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

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "1-0-0")
}

func TestEvalResultsSlipFactors(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	_, err := f.store.UpsertGame(ctx, db.Game{
		ID:       "g1",
		WeekID:   f.week2026.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		HomeName: "Buffalo Bills",
		AwayName: "Miami Dolphins",
		Kickoff:  time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC),
		Status:   db.GameFinal,
		Indoor:   true,
	})
	require.NoError(t, err)

	slip, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		f.week2026.ID,
		nil,
		"4-Legger",
		db.SlateScopeFullWeek,
		4,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)

	for _, sel := range []string{"BUF ML", "KC ML", "DET ML", "PHI ML"} {
		leg, err := f.store.CreateLeg(
			ctx,
			db.Leg{
				SlipID:     slip.ID,
				UserID:     f.user.ID,
				GameID:     new("g1"),
				MarketType: "ML",
				Selection:  sel,
				Odds:       -250,
			},
		)
		require.NoError(t, err)
		require.NoError(t, f.store.SetLegResult(ctx, leg.ID, parlay.Won, f.user.Name))
	}
	require.NoError(t, f.store.SetSlipGrade(ctx, slip.ID, parlay.Won, db.SlipGraded))

	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := html.UnescapeString(rec.Body.String())
	assert.Contains(t, body, "Slip Factor Attribution")
	assert.Contains(t, body, "Parlay length drag")
	assert.Contains(t, body, "Extra chalk drag")
	assert.Contains(t, body, "1-0")
}

func TestEvalResultsSourceValidationAndEmptyMarket(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	// Invalid ?source= returns 400.
	for _, raw := range []string{"bogus", "all", "123"} {
		bad := httptest.NewRecorder()
		f.handler.ServeHTTP(
			bad,
			httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?source="+raw, nil),
		)
		assert.Equal(t, http.StatusBadRequest, bad.Code)
	}

	// Empty market state renders source switcher and market empty-state copy.
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?source=market", nil),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	body := html.UnescapeString(rec.Body.String())
	assert.Contains(t, body, "Market-wide · All seasons")
	assert.Contains(t, body, "User slips")
	assert.Contains(t, body, "Market-wide")
	assert.Contains(t, body, "No graded market-wide legs yet for All seasons.")
}

func TestEvalResultsMarketModeAndFilterPreservation(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	// Seed a graded slip in 2025 so Seasons() != FinalSeasons().
	wk2025, err := f.store.UpsertWeek(ctx, 2025, 2, 1)
	require.NoError(t, err)
	slip2025, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		wk2025.ID,
		nil,
		"2025 Slip",
		db.SlateScopeFullWeek,
		1,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)
	leg2025, err := f.store.CreateLeg(
		ctx,
		db.Leg{SlipID: slip2025.ID, UserID: f.user.ID, MarketType: "ML", Selection: "BUF ML", Odds: -110},
	)
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, leg2025.ID, parlay.Won, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip2025.ID, parlay.Won, db.SlipGraded))

	// Seed a FINAL game in 2026 with pre-kickoff archived odds (including pinnacle for sharp benchmarks).
	kickoff := time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC)
	homeScore, awayScore := 27, 20
	_, err = f.store.UpsertGame(ctx, db.Game{
		ID:          "g-mkt-2026",
		WeekID:      f.week2026.ID,
		OddsEventID: new("evt-mkt-2026"),
		HomeTeam:    "BUF",
		AwayTeam:    "MIA",
		HomeName:    "Buffalo Bills",
		AwayName:    "Miami Dolphins",
		Kickoff:     kickoff,
		Status:      db.GameFinal,
		HomeScore:   &homeScore,
		AwayScore:   &awayScore,
	})
	require.NoError(t, err)

	linesBody := `[
		{
			"id": "evt-mkt-2026",
			"commence_time": "2026-09-13T17:00:00Z",
			"home_team": "Buffalo Bills",
			"away_team": "Miami Dolphins",
			"bookmakers": [
				{
					"key": "pinnacle",
					"markets": [{
						"key": "spreads",
						"outcomes": [
							{"name": "Buffalo Bills", "price": -110, "point": -3.5},
							{"name": "Miami Dolphins", "price": -110, "point": 3.5}
						]
					}]
				},
				{
					"key": "fanduel",
					"markets": [{
						"key": "spreads",
						"outcomes": [
							{"name": "Buffalo Bills", "price": -110, "point": -3.5},
							{"name": "Miami Dolphins", "price": -110, "point": 3.5}
						]
					}]
				}
			]
		}
	]`
	require.NoError(t, f.store.SaveAPIResponse(ctx, archive.Record{
		Source:    "odds",
		Key:       db.ArchiveOddsLinesKey,
		Method:    "GET",
		URL:       "https://example.test/odds",
		Status:    200,
		FetchedAt: kickoff.Add(-30 * time.Minute),
		Body:      []byte(linesBody),
	}))

	// ?source=market resolves default season from FINAL games (2026), not graded slips (2025).
	recMkt := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recMkt,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?source=market", nil),
	)
	require.Equal(t, http.StatusOK, recMkt.Code)
	bodyMkt := html.UnescapeString(recMkt.Body.String())

	assert.Contains(t, bodyMkt, "Market-wide · 2026 season")
	assert.Contains(t, bodyMkt, "Leg Calibration (A–F)")
	assert.Contains(t, bodyMkt, "2 decided")
	assert.Contains(t, bodyMkt, "Sharp benchmarks")
	assert.Contains(t, bodyMkt, "nflverse L5 fallback")

	// Slip-only sections are hidden and replaced by a note linking to User slips.
	assert.NotContains(t, bodyMkt, "Slip Calibration (A–F)")
	assert.NotContains(t, bodyMkt, "Slip Factor Attribution")
	assert.Contains(t, bodyMkt, "Parlay-level slip calibration and slip factor rules live under")

	// Filter URLs preserve ?source=market.
	assert.Contains(t, bodyMkt, "season=all&source=market")
	assert.Contains(t, bodyMkt, "season=2026&source=market")
	assert.Contains(t, bodyMkt, "season=2025&source=market")

	// Default ?source=slips still shows Slip Calibration and resolves 2025.
	recSlips := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recSlips,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?source=slips", nil),
	)
	require.Equal(t, http.StatusOK, recSlips.Code)
	bodySlips := html.UnescapeString(recSlips.Body.String())
	assert.Contains(t, bodySlips, "All groups · 2025 season")
	assert.Contains(t, bodySlips, "Slip Calibration (A–F)")
	assert.Contains(t, bodySlips, "season=2025&source=slips")
}

func TestEvalResultsMarketCacheInvalidation(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	kickoff := time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC)
	homeScore, awayScore := 27, 20
	_, err := f.store.UpsertGame(ctx, db.Game{
		ID:          "g-cache-1",
		WeekID:      f.week2026.ID,
		OddsEventID: new("evt-cache-1"),
		HomeTeam:    "BUF",
		AwayTeam:    "MIA",
		HomeName:    "Buffalo Bills",
		AwayName:    "Miami Dolphins",
		Kickoff:     kickoff,
		Status:      db.GameFinal,
		HomeScore:   &homeScore,
		AwayScore:   &awayScore,
	})
	require.NoError(t, err)

	linesBody := `[
		{
			"id": "evt-cache-1",
			"commence_time": "2026-09-13T17:00:00Z",
			"home_team": "Buffalo Bills",
			"away_team": "Miami Dolphins",
			"bookmakers": [{
				"key": "pinnacle",
				"markets": [{
					"key": "spreads",
					"outcomes": [
						{"name": "Buffalo Bills", "price": -110, "point": -3.5},
						{"name": "Miami Dolphins", "price": -110, "point": 3.5}
					]
				}]
			}]
		}
	]`
	require.NoError(t, f.store.SaveAPIResponse(ctx, archive.Record{
		Source:    "odds",
		Key:       db.ArchiveOddsLinesKey,
		Method:    "GET",
		URL:       "https://example.test/odds",
		Status:    200,
		FetchedAt: kickoff.Add(-30 * time.Minute),
		Body:      []byte(linesBody),
	}))

	// First request caches 2 market legs (2 decided).
	rec1 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec1,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?source=market&season=2026", nil),
	)
	require.Equal(t, http.StatusOK, rec1.Code)
	assert.Contains(t, rec1.Body.String(), "2 decided")

	// Add a second FINAL game and invalidate market cache.
	_, err = f.store.UpsertGame(ctx, db.Game{
		ID:          "g-cache-2",
		WeekID:      f.week2026.ID,
		OddsEventID: new("evt-cache-2"),
		HomeTeam:    "KC",
		AwayTeam:    "BAL",
		HomeName:    "Kansas City Chiefs",
		AwayName:    "Baltimore Ravens",
		Kickoff:     kickoff,
		Status:      db.GameFinal,
		HomeScore:   &homeScore,
		AwayScore:   &awayScore,
	})
	require.NoError(t, err)
	linesBody2 := `[
		{
			"id": "evt-cache-1",
			"commence_time": "2026-09-13T17:00:00Z",
			"home_team": "Buffalo Bills",
			"away_team": "Miami Dolphins",
			"bookmakers": [{
				"key": "pinnacle",
				"markets": [{
					"key": "spreads",
					"outcomes": [
						{"name": "Buffalo Bills", "price": -110, "point": -3.5},
						{"name": "Miami Dolphins", "price": -110, "point": 3.5}
					]
				}]
			}]
		},
		{
			"id": "evt-cache-2",
			"commence_time": "2026-09-13T17:00:00Z",
			"home_team": "Kansas City Chiefs",
			"away_team": "Baltimore Ravens",
			"bookmakers": [{
				"key": "pinnacle",
				"markets": [{
					"key": "spreads",
					"outcomes": [
						{"name": "Kansas City Chiefs", "price": -110, "point": -2.5},
						{"name": "Baltimore Ravens", "price": -110, "point": 2.5}
					]
				}]
			}]
		}
	]`
	require.NoError(t, f.store.SaveAPIResponse(ctx, archive.Record{
		Source:    "odds",
		Key:       db.ArchiveOddsLinesKey,
		Method:    "GET",
		URL:       "https://example.test/odds",
		Status:    200,
		FetchedAt: kickoff.Add(-20 * time.Minute),
		Body:      []byte(linesBody2),
	}))
	f.evalCache.InvalidateMarket()

	rec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		rec2,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?source=market&season=2026", nil),
	)
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "4 decided")
}

func TestEvalResultsEngineSelectionAndCalibration(t *testing.T) {
	t.Parallel()

	f := newEvalResultsFixture(t)
	ctx := t.Context()

	kickoff := time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC)
	homeScore, awayScore := 28, 21
	_, err := f.store.UpsertGame(ctx, db.Game{
		ID:          "g-sgp-1",
		WeekID:      f.week2026.ID,
		OddsEventID: new("evt-sgp-1"),
		HomeTeam:    "BUF",
		AwayTeam:    "MIA",
		HomeName:    "Buffalo Bills",
		AwayName:    "Miami Dolphins",
		Kickoff:     kickoff,
		Status:      db.GameFinal,
		Indoor:      true,
		HomeScore:   &homeScore,
		AwayScore:   &awayScore,
	})
	require.NoError(t, err)

	linesBody := `[
		{
			"id": "evt-sgp-1",
			"commence_time": "2026-09-13T17:00:00Z",
			"home_team": "Buffalo Bills",
			"away_team": "Miami Dolphins",
			"bookmakers": [
				{
					"key": "pinnacle",
					"markets": [
						{
							"key": "spreads",
							"outcomes": [
								{"name": "Buffalo Bills", "price": -108, "point": -3.5},
								{"name": "Miami Dolphins", "price": -108, "point": 3.5}
							]
						},
						{
							"key": "totals",
							"outcomes": [
								{"name": "Over", "price": -108, "point": 45.5},
								{"name": "Under", "price": -108, "point": 45.5}
							]
						}
					]
				}
			]
		}
	]`
	require.NoError(t, f.store.SaveAPIResponse(ctx, archive.Record{
		Source:    "odds",
		Key:       db.ArchiveOddsLinesKey,
		Method:    "GET",
		URL:       "https://example.test/odds",
		Status:    200,
		FetchedAt: kickoff.Add(-30 * time.Minute),
		Body:      []byte(linesBody),
	}))

	slip, err := f.store.CreateSlip(
		ctx,
		f.group.ID,
		f.week2026.ID,
		nil,
		"BUF SGP",
		db.SlateScopeFullWeek,
		2,
		[]int64{f.user.ID},
	)
	require.NoError(t, err)

	leg1, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID:     slip.ID,
		UserID:     f.user.ID,
		GameID:     new("g-sgp-1"),
		MarketType: "SPREAD",
		Selection:  "BUF -3.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-3.5),
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, leg1.ID, parlay.Won, f.user.Name))

	leg2, err := f.store.CreateLeg(ctx, db.Leg{
		SlipID:     slip.ID,
		UserID:     f.user.ID,
		GameID:     new("g-sgp-1"),
		MarketType: "TOTAL",
		Selection:  "BUF vs MIA Over 45.5",
		Side:       new("OVER"),
		Line:       new(45.5),
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, f.store.SetLegResult(ctx, leg2.ID, parlay.Won, f.user.Name))
	require.NoError(t, f.store.SetSlipGrade(ctx, slip.ID, parlay.Won, db.SlipGraded))

	// Default view selects active engine (v2) and shows v1 vs. v2 switcher and comparison.
	recDefault := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recDefault,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026", nil),
	)
	require.Equal(t, http.StatusOK, recDefault.Code)
	bodyDefault := html.UnescapeString(recDefault.Body.String())
	assert.Contains(t, bodyDefault, "All groups · 2026 season · Engine v2")
	assert.Contains(t, bodyDefault, `aria-label="Grading engine"`)
	assert.Contains(t, bodyDefault, "v2 (active)")
	assert.Contains(t, bodyDefault, "Probability Calibration (v2)")
	assert.Contains(t, bodyDefault, "Market Log Loss")
	assert.Contains(t, bodyDefault, "Engine Comparison")
	assert.Contains(t, bodyDefault, "v1 → v2 (Leg Bands)")
	assert.Contains(t, bodyDefault, "engine=v1&season=2026&source=slips")

	// ?engine=v1 renders the v1 A–F points baseline view alongside the market baseline.
	recV1 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recV1,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026&engine=v1", nil),
	)
	require.Equal(t, http.StatusOK, recV1.Code)
	bodyV1 := html.UnescapeString(recV1.Body.String())
	assert.Contains(t, bodyV1, "All groups · 2026 season · Engine v1")
	assert.Contains(t, bodyV1, "Probability Calibration (v1)")
	assert.Contains(t, bodyV1, "A–F points only")
	assert.Contains(t, bodyV1, "engine=v2&season=2026&source=slips")

	// ?engine=v2 renders v2 calibrated tiles, reliability bins, and SGP correlation lift.
	recV2 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recV2,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026&engine=v2", nil),
	)
	require.Equal(t, http.StatusOK, recV2.Code)
	bodyV2 := html.UnescapeString(recV2.Body.String())
	assert.Contains(t, bodyV2, "All groups · 2026 season · Engine v2")
	assert.Contains(t, bodyV2, "Probability Calibration (v2)")
	assert.Contains(t, bodyV2, "Log Loss")
	assert.Contains(t, bodyV2, "Brier Score")
	assert.Contains(t, bodyV2, "Classwise ECE")
	assert.Contains(t, bodyV2, "Bin Spread")
	assert.Contains(t, bodyV2, "SGP Correlation Lift")
	assert.Contains(t, bodyV2, "1 settled SGP")
	assert.Contains(t, bodyV2, "Copula LL")
	assert.Contains(t, bodyV2, "Naive LL")
	assert.Contains(t, bodyV2, "Lift (LL Δ)")
	assert.Contains(t, bodyV2, "Reliability Table (10% Bins)")
	assert.Contains(t, bodyV2, "0–10%")
	assert.Contains(t, bodyV2, "50–60%")
	assert.Contains(t, bodyV2, "90–100%")
	assert.Contains(t, bodyV2, "engine=v2&season=all&source=slips")
	assert.Contains(t, bodyV2, "engine=v2&season=2026&source=market")

	// Backwards-compatible ?rubric=v2 selects v2 as well.
	recLegacy := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recLegacy,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/analyzer/results?season=2026&rubric=v2", nil),
	)
	require.Equal(t, http.StatusOK, recLegacy.Code)
	assert.Contains(t, html.UnescapeString(recLegacy.Body.String()), "Probability Calibration (v2)")

	// Market mode with ?engine=v2 renders reliability table and hides SGP Correlation Lift.
	recMktV2 := httptest.NewRecorder()
	f.handler.ServeHTTP(
		recMktV2,
		httptest.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"/analyzer/results?source=market&season=2026&engine=v2",
			nil,
		),
	)
	require.Equal(t, http.StatusOK, recMktV2.Code)
	bodyMktV2 := html.UnescapeString(recMktV2.Body.String())
	assert.Contains(t, bodyMktV2, "Market-wide · 2026 season · Engine v2")
	assert.Contains(t, bodyMktV2, "Probability Calibration (v2)")
	assert.Contains(t, bodyMktV2, "Reliability Table (10% Bins)")
	assert.NotContains(t, bodyMktV2, "SGP Correlation Lift")
}

func TestAnalyzerResultsTemplateRendersCalibrationAndPhoneWidthLayout(t *testing.T) {
	t.Parallel()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	page := webui.EvalResultsPage{
		Title:          "Grading Results",
		GuideURL:       "/analyzer/guide",
		ScopeLabel:     "All groups",
		Source:         "slips",
		SlipsURL:       "/analyzer/results?source=slips",
		HasData:        true,
		SeasonLabel:    "2026 season",
		SelectedEngine: "v2",
		Engines: []webui.EvalEngineOption{
			{Name: "v1", Active: true, URL: "/analyzer/results?engine=v1"},
			{Name: "v2", Current: true, URL: "/analyzer/results?engine=v2"},
		},
		ShowEngineDiffs: true,
		EngineComparisons: []webui.EvalEngineComparisonView{
			{
				Name:         "v1",
				Active:       true,
				URL:          "/analyzer/results?engine=v1",
				LogLoss:      "–",
				BrierScore:   "–",
				ClasswiseECE: "–",
				BinSpread:    "–",
				SGPLogLoss:   "–",
				SGPLift:      "–",
				Monotonic:    true,
			},
			{
				Name:          "v2",
				Current:       true,
				URL:           "/analyzer/results?engine=v2",
				HasModelProbs: true,
				LogLoss:       "0.6120",
				BrierScore:    "0.2104",
				ClasswiseECE:  "0.0241",
				BinSpread:     "8/10",
				BinSpreadOK:   true,
				HasSGP:        true,
				SGPCount:      3,
				SGPLogLoss:    "0.5110",
				NaiveSGPLL:    "0.5522",
				SGPLift:       "+0.0412",
				SGPLiftPos:    true,
				Monotonic:     true,
			},
		},
		Calibration: webui.EvalCalibrationView{
			HasData:           true,
			HasModelProbs:     true,
			LogLoss:           "0.6120",
			BrierScore:        "0.2104",
			ClasswiseECE:      "0.0241",
			BinSpread:         "8/10",
			BinSpreadOK:       true,
			HasMarket:         true,
			MarketLogLoss:     "0.6380",
			MarketBrier:       "0.2230",
			BeatsMarketLL:     true,
			BeatsMarketBrier:  true,
			HasSGP:            true,
			SGPCount:          3,
			SGPLabel:          "3 settled SGPs",
			SGPLogLoss:        "0.5110",
			NaiveIndepLogLoss: "0.5522",
			SGPLift:           "+0.0412",
			SGPLiftPositive:   true,
			Reliability: []webui.EvalReliabilityBinView{
				{
					Label:      "0–10%",
					Count:      0,
					CountLabel: "0 legs",
					Empty:      true,
					MeanPred:   "–",
					Empirical:  "–",
					Gap:        "–",
				},
				{
					Label:      "50–60%",
					Count:      14,
					CountLabel: "14 legs",
					MeanPred:   "54.2%",
					Empirical:  "57.1%",
					Gap:        "+2.9 pp",
				},
			},
		},
		Monotonic:       true,
		MonotonicStatus: "A-grade legs are outperforming lower bands.",
		LegBands: []webui.EvalBandView{
			{
				Band:         "A",
				Class:        "grade-a",
				Record:       "8-2-0",
				Decided:      10,
				HitPct:       "80.0%",
				ImpliedPct:   "60.0%",
				Lift:         "+20.0%",
				LiftPositive: true,
				FlatROI:      "+28.0%",
				ROIPositive:  true,
			},
		},
		Diffs: []webui.EvalDiffView{
			{
				BaselineName:  "v1",
				CandidateName: "v2",
				LegBandDiffs: []webui.EvalBandDiffView{
					{
						Band:         "A",
						Class:        "grade-a",
						SampleChange: "6 → 10",
						WinRateDelta: "+5.0 pp",
						WinPositive:  true,
						LiftDelta:    "+4.2 pp",
						LiftPositive: true,
						FlatROIDelta: "+8.0 pp",
						ROIPositive:  true,
					},
				},
				Migrations: []webui.EvalMigrationView{
					{
						FromBand:  "B",
						FromClass: "grade-b",
						ToBand:    "A",
						ToClass:   "grade-a",
						Total:     4,
						Record:    "4-0",
						HitPct:    "100.0%",
					},
				},
			},
		},
	}

	out, err := renderer.Render("analyzer-results", page)
	require.NoError(t, err)
	body := html.UnescapeString(string(out))

	assert.Contains(t, body, "Bin spread 8/10 (≥80%)")
	assert.Contains(t, body, "Meets ≥80% target")
	assert.Contains(t, body, "0.6120")
	assert.Contains(t, body, "vs. Market 0.6380")
	assert.Contains(t, body, "0.2104")
	assert.Contains(t, body, "0.0241")
	assert.Contains(t, body, "3 settled SGPs")
	assert.Contains(t, body, "0.5110")
	assert.Contains(t, body, "0.5522")
	assert.Contains(t, body, "+0.0412")
	assert.Contains(t, body, "Copula joint probability beats naive leg multiplication on settled SGPs.")
	assert.Contains(t, body, "50–60%")
	assert.Contains(t, body, "14 legs")
	assert.Contains(t, body, "54.2%")
	assert.Contains(t, body, "57.1%")
	assert.Contains(t, body, "+2.9 pp")
	assert.Contains(t, body, "0–10%")
	assert.Contains(t, body, "0 legs")
	assert.Contains(t, body, "Notable band migrations")
	assert.Contains(t, body, "sm:hidden")
	assert.Contains(t, body, "hidden sm:block")
}
