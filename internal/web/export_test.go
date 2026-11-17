package web_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
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
	assert.Contains(t, body, "DraftKings has no line for this pick yet")
	assert.Contains(t, body, "FanDuel has no line for this pick yet")
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
