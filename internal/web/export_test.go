package web_test

import (
	"bufio"
	"context"
	"fmt"
	"html"
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

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

// exportFetched is the odds fetch time the export card's note shows.
var exportFetched = time.Date(2026, time.November, 7, 15, 42, 0, 0, time.UTC)

// addExportLeg posts one leg with its full selection keys.
func addExportLeg(t *testing.T, fixture *slipFixture, form url.Values) {
	t.Helper()

	response := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/legs", fixture.slip.ID), form)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
}

// lockSlip flips the slip to LOCKED the way the last lock-in would.
func lockSlip(t *testing.T, fixture *slipFixture) {
	t.Helper()

	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
}

// exportRows writes the odds rows both books quote for the test legs.
func exportRows(t *testing.T, store *db.Store, gameID string) {
	t.Helper()

	row := func(book, market, team, side string, line *float64, price int, link, sid string) db.Odds {
		odds := db.Odds{
			GameID:     gameID,
			Book:       book,
			MarketType: market,
			Team:       team,
			Side:       side,
			Line:       line,
			Price:      price,
			FetchedAt:  exportFetched,
		}
		if link != "" {
			odds.Link = new(link)
			odds.Sid = new(sid)
		}
		return odds
	}

	rows := []db.Odds{
		row("draftkings", "ML", "BUF", "TEAM", nil, -150, "https://sportsbook.draftkings.com/?outcomes=dk-ml", "dk-ml"),
		row(
			"fanduel", "ML", "BUF", "TEAM", nil, -145,
			"https://sportsbook.fanduel.com/addToBetslip?marketId=1&selectionId=fd-ml", "fd-ml",
		),
		row("draftkings", "TOTAL", "", "OVER", new(47.5), -110, "", ""),
		row(
			"fanduel", "TOTAL", "", "OVER", new(47.5), -105,
			"https://sportsbook.fanduel.com/addToBetslip?marketId=2&selectionId=fd-over", "fd-over",
		),
		row(
			"draftkings", "SPREAD", "BUF", "TEAM", new(-3.0), -110,
			"https://sportsbook.draftkings.com/?outcomes=dk-spread", "dk-spread",
		),
	}
	changed, err := store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	require.True(t, changed)
}

// TestExportCardLinksSomeLegsAndListsTheRest checks the card at phone width:
// DraftKings per-leg taps, FanDuel's one-tap combined link, friendly notes for
// the rest, the prices note, and the copy text.
func TestExportCardLinksSomeLegsAndListsTheRest(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 4)
	game := addGame(t, fixture, gameKickoff())
	exportRows(t, fixture.store, game.ID)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"selection":   {"BUF ML"},
		"odds":        {"-110"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"TOTAL"},
		"side":        {"OVER"},
		"line":        {"47.5"},
		"selection":   {"Over 47.5"},
		"odds":        {"-110"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"SPREAD"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"line":        {"-3.5"},
		"selection":   {"BUF -3.5"},
		"odds":        {"-110"},
	})
	// A moved total must print 47.5, not +47.5, the way every other total does.
	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"TOTAL"},
		"side":        {"OVER"},
		"line":        {"48.5"},
		"selection":   {"Over 48.5"},
		"odds":        {"-110"},
	})
	lockSlip(t, fixture)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := html.UnescapeString(recorder.Body.String())

	assert.Contains(t, body, `id="slip-export"`)
	assert.Contains(t, body, fmt.Sprintf(`hx-get="/slips/%d/export"`, fixture.slip.ID))
	assert.Contains(t, body, `hx-trigger="slip-updated from:body"`)

	assert.Contains(t, body, "DraftKings")
	assert.Contains(t, body, "FanDuel")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/?outcomes=dk-ml")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/?outcomes=dk-spread")
	assert.Contains(t, body, "Open in FanDuel")
	assert.Contains(t, body, "https://sportsbook.fanduel.com/addToBetslip"+
		"?marketId%5B0%5D=1&selectionId%5B0%5D=fd-ml"+
		"&marketId%5B1%5D=2&selectionId%5B1%5D=fd-over"+
		"&marketId%5B2%5D=2&selectionId%5B2%5D=fd-over")
	assert.NotContains(t, body, "https://sportsbook.fanduel.com/addToBetslip?marketId=1&selectionId=fd-ml")
	assert.Contains(t, body, `target="_blank"`)
	assert.Contains(t, body, `rel="noopener"`)

	assert.Contains(t, body, "this pick didn't come with a DraftKings link")
	assert.Contains(t, body, "Book at -3 · slip -3.5")
	assert.Contains(t, body, "Over 48.5: Book at 47.5 · slip 48.5")
	assert.NotContains(t, body, "Book at +47.5")
	assert.Contains(t, body, "FanDuel has no line for this pick yet")

	assert.Contains(t, body, "Prices may have moved since")
	assert.Contains(t, body, exportFetched.Format("3:04 PM")+" UTC")
	assert.NotContains(t, body, "Links as of")
	assert.Contains(t, body, "Copy slip as text")
	assert.Contains(t, body, "Ada: BUF ML (-110)")
	assert.Contains(t, body, "Ada: Over 47.5 (-110)")
	assert.Contains(t, body, "Ada: BUF -3.5 (-110)")
	assert.Contains(t, body, "Ada: Over 48.5 (-110)")
}

// TestExportCardHidesWhenTheSlipUnlocks checks the partial the SSE refresh
// fetches: it carries the card while locked and only its refresh wrapper after
// an unlock.
func TestExportCardHidesWhenTheSlipUnlocks(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")
	lockSlip(t, fixture)

	locked := getExport(t, fixture.handler, fixture.slip.ID)
	assert.Equal(t, http.StatusOK, locked.Code)
	assert.Contains(t, locked.Body.String(), "Copy slip as text")
	assert.Contains(t, locked.Body.String(), `hx-trigger="slip-updated from:body"`)

	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipOpen))

	unlocked := getExport(t, fixture.handler, fixture.slip.ID)
	assert.Equal(t, http.StatusOK, unlocked.Code)
	assert.Contains(t, unlocked.Body.String(), `id="slip-export"`)
	assert.NotContains(t, unlocked.Body.String(), "Copy slip as text")
	assert.NotContains(t, unlocked.Body.String(), "DraftKings")
}

// TestExportCardOnlyRendersOnALockedSlip checks that a page loaded before the
// slip locks carries no card, only the wrapper that later refreshes.
func TestExportCardOnlyRendersOnALockedSlip(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d", fixture.slip.ID),
		nil,
	)
	fixture.handler.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `id="slip-export"`)
	assert.NotContains(t, body, "Copy slip as text")
	assert.NotContains(t, body, "Open in DraftKings")
}

// TestExportCardEscapesFreeText checks that OTHER legs, which are typed by
// hand, cannot inject markup into the card or the copy text.
func TestExportCardEscapesFreeText(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	addLeg(t, fixture.handler, fixture.slip.ID, `<script>alert("pick")</script>`)
	lockSlip(t, fixture)

	recorder := getExport(t, fixture.handler, fixture.slip.ID)
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	assert.Contains(t, body, `&lt;script&gt;`)
	assert.NotContains(t, body, `<script>alert`)
}

// TestExportCardIgnoresStaleOddsRows checks that a row left behind by an
// earlier sync cannot offer a dead link after the book stopped quoting it.
func TestExportCardIgnoresStaleOddsRows(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	exportRows(t, fixture.store, game.ID)

	// A newer prop sync at exportFetched carried a different prop, so the books stopped
	// quoting PASS_YDS and the older PASS_YDS rows are left behind.
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       "OVER",
			Line:       new(65.5),
			Price:      -110,
			FetchedAt:  exportFetched,
		},
	})
	require.NoError(t, err)

	stale := exportFetched.Add(-time.Hour)
	link := func() *string {
		value := "https://sportsbook.draftkings.com/?outcomes=stale"
		return &value
	}
	rows := []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -110,
			Link:       link(),
			Sid:        new("stale"),
			FetchedAt:  stale,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -110,
			Link:       link(),
			Sid:        new("stale"),
			FetchedAt:  stale,
		},
	}
	changed, err := fixture.store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	require.True(t, changed)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"team":        {"BUF"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
		"line":        {"245.5"},
		"selection":   {"Josh Allen Over 245.5"},
		"odds":        {"-110"},
	})
	lockSlip(t, fixture)

	body := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	assert.NotContains(t, body, "outcomes=stale")
	assert.Contains(
		t,
		body,
		"DraftKings isn't offering this prop right now, so add it by hand if it appears on their board.",
	)
	assert.Contains(
		t,
		body,
		"FanDuel isn't offering this prop right now, so add it by hand if it appears on their board.",
	)
}

func TestExportUsesTheScenarioClockForKickoff(t *testing.T) {
	t.Parallel()

	scenarioNow := time.Now().UTC().Add(49 * time.Hour)
	fixture := newSlipFixtureAt(t, 2, func() time.Time { return scenarioNow })
	game := addGame(t, fixture, gameKickoff())
	exportRows(t, fixture.store, game.ID)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"selection":   {"BUF ML"},
		"odds":        {"-150"},
	})
	lockSlip(t, fixture)

	body := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	assert.Contains(t, body, "this game already started, so add it by hand",
		"the export reads scenario time, not the wall clock")
}

// TestLockAndUnlockRefreshTheExportCard checks that the person who acts gets
// the export card refreshed with their own response, not only over SSE.
func TestLockAndUnlockRefreshTheExportCard(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 1)
	addLeg(t, fixture.handler, fixture.slip.ID, "BUF ML")

	locked := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/lock", fixture.slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, locked.Code)
	assert.Equal(t, "slip-updated", locked.Header().Get("Hx-Trigger"))

	unlocked := postForm(t, fixture.handler, fmt.Sprintf("/slips/%d/unlock", fixture.slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, unlocked.Code)
	assert.Equal(t, "slip-updated", unlocked.Header().Get("Hx-Trigger"))
}

// TestExportCardLinksMovedPropAndNameVariant checks that a prop picked under
// a suffix-free player name or an older line still links to the sportsbook and
// surfaces the line-drift callout on the export card.
func TestExportCardLinksMovedPropAndNameVariant(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	rows := []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "MIA",
			PlayerName: "Ollie Gordon II",
			Side:       "OVER",
			Line:       new(17.5),
			Price:      -110,
			Link:       new("https://sportsbook.draftkings.com/event/123?outcomes=dk-gordon"),
			Sid:        new("0ML#gordon"),
			FetchedAt:  exportFetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "CAR",
			PlayerName: "Bryce Young",
			Side:       "OVER",
			Line:       new(254.5),
			Price:      -115,
			Link:       new("https://sportsbook.draftkings.com/event/123?outcomes=dk-bryce"),
			Sid:        new("0ML#bryce"),
			FetchedAt:  exportFetched,
		},
	}
	changed, err := fixture.store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	require.True(t, changed)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"RUSH_YDS"},
		"team":        {"MIA"},
		"player_name": {"Ollie Gordon"},
		"side":        {"OVER"},
		"line":        {"17.5"},
		"selection":   {"Ollie Gordon Over 17.5 Rush Yds"},
		"odds":        {"-110"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"team":        {"CAR"},
		"player_name": {"Bryce Young"},
		"side":        {"OVER"},
		"line":        {"248.5"},
		"selection":   {"Bryce Young Over 248.5 Pass Yds"},
		"odds":        {"-110"},
	})
	lockSlip(t, fixture)

	body := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/123?outcomes=dk-gordon")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/123?outcomes=dk-bryce")
	assert.Contains(t, body, "Book at 254.5 · slip 248.5")
	assert.NotContains(t, body, "DraftKings has no line for this pick yet")
}

// getExport fetches the export partial the way HTMX does.
func getExport(t *testing.T, handler http.Handler, slipID int64) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("/slips/%d/export", slipID),
		nil,
	)
	request.Header.Set("Hx-Request", "true")
	handler.ServeHTTP(recorder, request)
	return recorder
}

type exportSpyRefresher struct {
	stubRefresher
	enrichCalls []int64
}

func (r *exportSpyRefresher) KickSlipExportEnrich(_ context.Context, slipID int64) {
	r.enrichCalls = append(r.enrichCalls, slipID)
}

type exportFakeFallbackProps struct {
	events []odds.Event
	props  map[string][]odds.Prop
}

func (f *exportFakeFallbackProps) Events(_ context.Context) ([]odds.Event, odds.Credits, error) {
	return f.events, odds.Credits{Remaining: 300}, nil
}

func (f *exportFakeFallbackProps) EventProps(_ context.Context, eventID string) ([]odds.Prop, odds.Credits, error) {
	return f.props[eventID], odds.Credits{Remaining: 299}, nil
}

// TestLockInKicksExportEnrichOnlyWhenSlipTransitionsToLocked checks that
// partial lock-ins on a multi-member slip do not kick export enrichment, and
// the final lock-in that transitions the slip to LOCKED does.
func TestLockInKicksExportEnrichOnlyWhenSlipTransitionsToLocked(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	ada, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	sam, err := store.CreateUserWithIdentity(t.Context(), "Sam", "sam@example.com", "google-sam", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	_, err = store.JoinByToken(t.Context(), sam.ID, group.InviteToken)
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeFullWeek,
		1,
		[]int64{ada.ID, sam.ID},
	)
	require.NoError(t, err)

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	spy := &exportSpyRefresher{}
	signedIn := ada
	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return signedIn, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store:   store,
		Slips:   slips.New(store, &live.Hub{}, nil),
		Refresh: spy,
	})

	fixture := &slipFixture{handler: handler, store: store, week: week, slip: slip}
	require.Equal(t, http.StatusOK, addLeg(t, handler, slip.ID, "BUF ML").Code)
	signedIn = sam
	require.Equal(t, http.StatusOK, addLeg(t, handler, slip.ID, "MIA ML").Code)

	// First member locks in: slip is still OPEN, so enrichment is not kicked.
	signedIn = ada
	firstLock := postForm(t, handler, fmt.Sprintf("/slips/%d/lock", slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, firstLock.Code)
	assert.Empty(t, spy.enrichCalls)

	// Second member locks in: slip transitions to LOCKED and kicks enrichment.
	signedIn = sam
	secondLock := postForm(t, handler, fmt.Sprintf("/slips/%d/lock", slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, secondLock.Code)
	assert.Equal(t, []int64{slip.ID}, spy.enrichCalls)
	_ = fixture
}

// TestLockInEnrichesUnlinkedPropInBackgroundAndPushesSSE checks that locking a
// slip with an unlinked Parlay-only prop leg runs background enrichment and
// pushes slip-updated over SSE once the deep links land.
func TestLockInEnrichesUnlinkedPropInBackgroundAndPushesSSE(t *testing.T) {
	t.Parallel()

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
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeFullWeek,
		1,
		[]int64{user.ID},
	)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	kickoff := now.Add(4 * time.Hour)
	game, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "401772099",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeName:  "Buffalo Bills",
		AwayName:  "Miami Dolphins",
		Kickoff:   kickoff,
		Status:    db.GameScheduled,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	// Seed Parlay-only prop row (no link/sid) and odds:props:{gameID}.
	require.NoError(t, store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.UpsertOdds(t.Context(), []db.Odds{{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Josh Allen",
			Side:       odds.SideOver,
			Line:       new(255.5),
			Price:      -110,
			FetchedAt:  now,
		}})
		if err != nil {
			return err
		}
		return tx.RecordSync(t.Context(), refresh.OddsPropsSource(game.ID), now, nil)
	}))

	hub := &live.Hub{}
	svc := refresh.New(refresh.Deps{
		Store: store,
		FallbackProps: &exportFakeFallbackProps{
			events: []odds.Event{{
				ID:           "toa-99",
				CommenceTime: kickoff,
				HomeTeam:     "Buffalo Bills",
				AwayTeam:     "Miami Dolphins",
			}},
			props: map[string][]odds.Prop{
				"toa-99": {{
					EventID:    "toa-99",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(255.5),
					Price:      -110,
					Link:       "https://sportsbook.draftkings.com/event/dk-evt-99?outcomes=dk-sid-99",
					Sid:        "dk-sid-99",
					EventSid:   "dk-evt-99",
				}},
			},
		},
		Publisher: hub,
		Now:       func() time.Time { return now },
		Logger:    zerolog.Nop(),
	})
	t.Cleanup(func() { assert.NoError(t, svc.Shutdown(context.Background())) })

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	handler := webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: group.ID, Name: group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return group, true
		},
		Store:   store,
		Slips:   slips.New(store, hub, nil),
		Stream:  live.NewStream(hub, func(int64) bool { return false }),
		Refresh: svc,
		Now:     func() time.Time { return now },
	})

	fixture := &slipFixture{handler: handler, store: store, week: week, slip: slip}
	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {odds.MarketPassYds},
		"player_name": {"Josh Allen"},
		"side":        {odds.SideOver},
		"line":        {"255.5"},
		"selection":   {"Josh Allen Over 255.5 Pass Yds"},
		"odds":        {"-110"},
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	streamReq, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		fmt.Sprintf("%s/slips/%d/events", server.URL, slip.ID),
		nil,
	)
	require.NoError(t, err)
	streamResp, err := server.Client().Do(streamReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, streamResp.StatusCode)
	t.Cleanup(func() { assert.NoError(t, streamResp.Body.Close()) })

	lockResp := postForm(t, handler, fmt.Sprintf("/slips/%d/lock", slip.ID), url.Values{})
	require.Equal(t, http.StatusOK, lockResp.Code)

	// LockIn publishes slip-updated immediately, and background enrichment
	// publishes board-updated + slip-updated once the enriched DraftKings link is written.
	// Read with one scanner so buffered bytes are not lost across events.
	scanner := bufio.NewScanner(streamResp.Body)
	nextEvent := func() string {
		done := make(chan string, 1)
		go func() {
			for scanner.Scan() {
				if ev, ok := strings.CutPrefix(scanner.Text(), "event: "); ok {
					done <- ev
					return
				}
			}
			done <- ""
		}()
		select {
		case ev := <-done:
			require.NoError(t, scanner.Err())
			require.NotEmpty(t, ev, "stream ended before expected event")
			return ev
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for an SSE event")
			return ""
		}
	}

	assert.Equal(t, "slip-updated", nextEvent())
	seenSlipUpdated := false
	for range 2 {
		switch ev := nextEvent(); ev {
		case "slip-updated":
			seenSlipUpdated = true
		case "board-updated":
		default:
			t.Fatalf("unexpected SSE event %q", ev)
		}
	}
	assert.True(t, seenSlipUpdated, "expected slip-updated after background enrichment")

	body := html.UnescapeString(getExport(t, handler, slip.ID).Body.String())
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/dk-evt-99?outcomes=dk-sid-99")
}

func TestExportCardDraftKingsMultiGameWithCombinedGroup(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 4)
	gameA := addGame(t, fixture, gameKickoff()) // MIA @ BUF
	gameB, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772002",
		WeekID:     fixture.week.ID,
		HomeTeam:   "KC",
		AwayTeam:   "LAC",
		HomeName:   "Kansas City Chiefs",
		AwayName:   "Los Angeles Chargers",
		HomeESPNID: "12",
		AwayESPNID: "24",
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     gameA.ID,
			Book:       "draftkings",
			MarketType: "ML",
			Team:       "BUF",
			Side:       "TEAM",
			Price:      -150,
			Link:       new("https://sportsbook.draftkings.com/event/dk-evt-a?outcomes=dk-a1"),
			Sid:        new("dk-a1"),
			EventSid:   new("dk-evt-a"),
			FetchedAt:  exportFetched,
		},
		{
			GameID:     gameA.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.0),
			Price:      -110,
			Link:       new("https://sportsbook.draftkings.com/event/dk-evt-a?outcomes=dk-a2"),
			Sid:        new("dk-a2"),
			EventSid:   new("dk-evt-a"),
			FetchedAt:  exportFetched,
		},
		{
			GameID:     gameB.ID,
			Book:       "draftkings",
			MarketType: "ML",
			Team:       "KC",
			Side:       "TEAM",
			Price:      -180,
			Link:       new("https://sportsbook.draftkings.com/event/dk-evt-b?outcomes=dk-b1"),
			Sid:        new("dk-b1"),
			EventSid:   new("dk-evt-b"),
			FetchedAt:  exportFetched,
		},
	})
	require.NoError(t, err)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {gameA.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"selection":   {"BUF ML"},
		"odds":        {"-150"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {gameA.ID},
		"market_type": {"SPREAD"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"line":        {"-3.5"},
		"selection":   {"BUF -3.5"},
		"odds":        {"-110"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {gameB.ID},
		"market_type": {"ML"},
		"team":        {"KC"},
		"side":        {"TEAM"},
		"selection":   {"KC ML"},
		"odds":        {"-180"},
	})
	lockSlip(t, fixture)

	body := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	assert.Contains(t, body, "DraftKings links load one game at a time — tap each game below to add its legs.")
	assert.Contains(t, body, "Open MIA @ BUF (2 legs)")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/dk-evt-a?outcomes=dk-a1,dk-a2")
	assert.Contains(t, body, "BUF ML")
	assert.Contains(t, body, "BUF -3.5")
	assert.Contains(t, body, "Book at -3 · slip -3.5")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/dk-evt-b?outcomes=dk-b1")
	assert.Contains(t, body, "KC ML")
}

func TestExportCardDraftKingsMultiGameAllSingleLegs(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 4)
	gameA := addGame(t, fixture, gameKickoff())
	gameB, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772002",
		WeekID:     fixture.week.ID,
		HomeTeam:   "KC",
		AwayTeam:   "LAC",
		HomeName:   "Kansas City Chiefs",
		AwayName:   "Los Angeles Chargers",
		HomeESPNID: "12",
		AwayESPNID: "24",
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     gameA.ID,
			Book:       "draftkings",
			MarketType: "ML",
			Team:       "BUF",
			Side:       "TEAM",
			Price:      -150,
			Link:       new("https://sportsbook.draftkings.com/event/dk-evt-a?outcomes=dk-a1"),
			Sid:        new("dk-a1"),
			EventSid:   new("dk-evt-a"),
			FetchedAt:  exportFetched,
		},
		{
			GameID:     gameB.ID,
			Book:       "draftkings",
			MarketType: "ML",
			Team:       "KC",
			Side:       "TEAM",
			Price:      -180,
			Link:       new("https://sportsbook.draftkings.com/event/dk-evt-b?outcomes=dk-b1"),
			Sid:        new("dk-b1"),
			EventSid:   new("dk-evt-b"),
			FetchedAt:  exportFetched,
		},
	})
	require.NoError(t, err)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {gameA.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"selection":   {"BUF ML"},
		"odds":        {"-150"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {gameB.ID},
		"market_type": {"ML"},
		"team":        {"KC"},
		"side":        {"TEAM"},
		"selection":   {"KC ML"},
		"odds":        {"-180"},
	})
	lockSlip(t, fixture)

	body := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	assert.Contains(t, body, "DraftKings links load one game at a time — tap each game below to add its legs.")
	assert.NotContains(t, body, "Open MIA @ BUF")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/dk-evt-a?outcomes=dk-a1")
	assert.Contains(t, body, "https://sportsbook.draftkings.com/event/dk-evt-b?outcomes=dk-b1")
	assert.Contains(t, body, "1.")
	assert.Contains(t, body, "2.")
}

func TestExportCardUnquotedPropVsUnsyncedGame(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 4)
	syncedGame := addGame(t, fixture, gameKickoff())
	unsyncedGame, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772002",
		WeekID:     fixture.week.ID,
		HomeTeam:   "KC",
		AwayTeam:   "LAC",
		HomeName:   "Kansas City Chiefs",
		AwayName:   "Los Angeles Chargers",
		HomeESPNID: "12",
		AwayESPNID: "24",
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     syncedGame.ID,
			Book:       "draftkings",
			MarketType: odds.MarketRushYds,
			Team:       "BUF",
			PlayerName: "James Cook",
			Side:       odds.SideOver,
			Line:       new(65.5),
			Price:      -110,
			FetchedAt:  exportFetched,
		},
	})
	require.NoError(t, err)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {syncedGame.ID},
		"market_type": {odds.MarketPassYds},
		"team":        {"BUF"},
		"player_name": {"Josh Allen"},
		"side":        {odds.SideOver},
		"line":        {"245.5"},
		"selection":   {"Josh Allen Over 245.5 Pass Yds"},
		"odds":        {"-110"},
	})
	addExportLeg(t, fixture, url.Values{
		"game_id":     {unsyncedGame.ID},
		"market_type": {odds.MarketPassYds},
		"team":        {"KC"},
		"player_name": {"Patrick Mahomes"},
		"side":        {odds.SideOver},
		"line":        {"265.5"},
		"selection":   {"Patrick Mahomes Over 265.5 Pass Yds"},
		"odds":        {"-110"},
	})
	lockSlip(t, fixture)

	body := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	assert.Contains(
		t,
		body,
		"Heads up: FanDuel isn't offering this prop right now, so add it by hand if it appears on their board.",
	)
	assert.Contains(t, body, "Heads up: FanDuel has no line for this pick yet, so add it by hand.")
}

func TestExportCardOrdersBooksWithHomeBookFirst(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	exportRows(t, fixture.store, game.ID)

	addExportLeg(t, fixture, url.Values{
		"game_id":     {game.ID},
		"market_type": {"ML"},
		"team":        {"BUF"},
		"side":        {"TEAM"},
		"selection":   {"BUF ML"},
		"odds":        {"-110"},
	})
	lockSlip(t, fixture)

	// FanDuel homeBook puts FanDuel before DraftKings.
	fdUser, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("fanduel"))
	require.NoError(t, err)
	fixture.as(fdUser)

	fdBody := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	fdIdx := strings.Index(fdBody, ">FanDuel</h3>")
	dkIdx := strings.Index(fdBody, ">DraftKings</h3>")
	require.NotEqual(t, -1, fdIdx)
	require.NotEqual(t, -1, dkIdx)
	assert.Less(t, fdIdx, dkIdx)

	// DraftKings homeBook (or unset) puts DraftKings before FanDuel.
	dkUser, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("draftkings"))
	require.NoError(t, err)
	fixture.as(dkUser)

	dkBody := html.UnescapeString(getExport(t, fixture.handler, fixture.slip.ID).Body.String())
	fdIdx = strings.Index(dkBody, ">FanDuel</h3>")
	dkIdx = strings.Index(dkBody, ">DraftKings</h3>")
	require.NotEqual(t, -1, fdIdx)
	require.NotEqual(t, -1, dkIdx)
	assert.Less(t, dkIdx, fdIdx)
}
