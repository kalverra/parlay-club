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
