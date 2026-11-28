package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

type researchTestRefresher struct {
	propsSynced []string
}

func (r *researchTestRefresher) SyncGameProps(_ context.Context, gameID string) error {
	r.propsSynced = append(r.propsSynced, gameID)
	return nil
}

func (r *researchTestRefresher) KickPickLines(_ context.Context, _ int64) {}

func (r *researchTestRefresher) KickPickGame(_ context.Context, _ int64, _ string) {}

func (r *researchTestRefresher) KickSlipExportEnrich(_ context.Context, _ int64) {}

func (r *researchTestRefresher) ForcePickRefresh(_ context.Context, _ int64, _ string) error {
	return nil
}

func (r *researchTestRefresher) ForceLiveRefresh(_ context.Context, _, _ int64) error {
	return nil
}

func researchHandlerWithRefresher(
	t *testing.T,
	fixture *slipFixture,
	user db.User,
	refresher webui.Refresher,
) http.Handler {
	t.Helper()

	renderer, err := webui.NewRenderer(assets.Templates, false)
	require.NoError(t, err)

	hub := &live.Hub{}
	return webui.NewWithDeps(webui.Dependencies{
		Logger:   zerolog.Nop(),
		Renderer: renderer,
		UserFrom: func(context.Context) (db.User, bool) {
			return user, true
		},
		GroupsFrom: func(context.Context) []webui.GroupOption {
			return []webui.GroupOption{{ID: fixture.group.ID, Name: fixture.group.Name, Current: true}}
		},
		GroupFrom: func(context.Context) (db.Group, bool) {
			return fixture.group, true
		},
		Store:   fixture.store,
		Slips:   slips.New(fixture.store, hub, nil),
		Stream:  live.NewStream(hub, func(int64) bool { return false }),
		Refresh: refresher,
	})
}

func TestResearchPage(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// Add roster players
	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}))

	// 1. GET /research with no query params renders selectors and placeholder
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Research")
	assert.Contains(t, body, "Find a leg across every cached line this week.")
	assert.Contains(t, body, "Bet type")
	assert.Contains(t, body, "Best grade")
	assert.Regexp(
		t,
		`<a\s+href="/research"\s+class="flex flex-col[^>]*aria-current="page"`,
		body,
		"the Research tab is the current destination",
	)
	assert.Contains(t, body, game.HomeTeam)
	assert.Contains(t, body, "Josh Allen")
	assert.Empty(t, refresher.propsSynced)

	// 2. GET /research?game_id={id} pre-loads game research
	reqGame := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?game_id="+game.ID, nil)
	recGame := httptest.NewRecorder()
	handler.ServeHTTP(recGame, reqGame)
	assert.Equal(t, http.StatusOK, recGame.Code)
	bodyGame := recGame.Body.String()
	assert.Contains(t, bodyGame, game.AwayTeam+" @ "+game.HomeTeam)
	assert.Contains(t, bodyGame, "Key Injuries")
	assert.Contains(t, bodyGame, "Player Matches")

	// 3. GET /research?player_id={espnID} pre-loads player research
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "3918298",
		GamesPlayed: 8,
		StatsJSON:   `{"PASS_YDS":[280.0, 310.0, 195.0, 240.0, 265.0]}`,
		UpdatedAt:   now,
	}))

	reqPlayer := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?player_id=3918298", nil)
	recPlayer := httptest.NewRecorder()
	handler.ServeHTTP(recPlayer, reqPlayer)
	assert.Equal(t, http.StatusOK, recPlayer.Code)
	bodyPlayer := recPlayer.Body.String()
	assert.Contains(t, bodyPlayer, "Josh Allen")
	assert.Contains(t, bodyPlayer, "8 GP")
	assert.Contains(t, bodyPlayer, "L5 Median")
	assert.Contains(t, bodyPlayer, "Season Mean")

	// 4. GET /research?player=Josh+Allen resolves athlete ID
	reqSearch := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?player=Josh+Allen", nil)
	recSearch := httptest.NewRecorder()
	handler.ServeHTTP(recSearch, reqSearch)
	assert.Equal(t, http.StatusOK, recSearch.Code)
	bodySearch := recSearch.Body.String()
	assert.Contains(t, bodySearch, "Josh Allen")
	assert.Contains(t, bodySearch, "8 GP")

	// 5. HTMX requests return partials
	reqHTMXGame := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?game_id="+game.ID, nil)
	reqHTMXGame.Header.Set("Hx-Request", "true")
	recHTMXGame := httptest.NewRecorder()
	handler.ServeHTTP(recHTMXGame, reqHTMXGame)
	assert.Equal(t, http.StatusOK, recHTMXGame.Code)
	bodyHTMXGame := recHTMXGame.Body.String()
	assert.NotContains(t, bodyHTMXGame, "<!doctype html>")
	assert.Contains(t, bodyHTMXGame, "Key Injuries")

	reqHTMXPlayer := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?player_id=3918298", nil)
	reqHTMXPlayer.Header.Set("Hx-Request", "true")
	recHTMXPlayer := httptest.NewRecorder()
	handler.ServeHTTP(recHTMXPlayer, reqHTMXPlayer)
	assert.Equal(t, http.StatusOK, recHTMXPlayer.Code)
	bodyHTMXPlayer := recHTMXPlayer.Body.String()
	assert.NotContains(t, bodyHTMXPlayer, "<!doctype html>")
	assert.Contains(t, bodyHTMXPlayer, "Josh Allen")
}

func TestResearchGameHandler(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// 1. Missing game returns 404
	req404 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/games/missing", nil)
	rec404 := httptest.NewRecorder()
	handler.ServeHTTP(rec404, req404)
	assert.Equal(t, http.StatusNotFound, rec404.Code)

	// 2. Set injuries JSON on game: includes key positions (QB, WR, CB) and non-key (K)
	injuriesJSON := `[
		{
			"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
			"injuries": [
				{
					"status": "Questionable",
					"athlete": {"id": "3918298", "fullName": "Josh Allen", "position": {"abbreviation": "QB"}},
					"details": {"type": "Ankle"}
				},
				{
					"status": "Out",
					"athlete": {"id": "99999", "fullName": "Tyler Bass", "position": {"abbreviation": "PK"}},
					"details": {"type": "Hip"}
				}
			]
		},
		{
			"team": {"id": "15", "displayName": "Miami Dolphins", "abbreviation": "MIA"},
			"injuries": [
				{
					"status": "Doubtful",
					"athlete": {"id": "4241479", "fullName": "Jaylen Waddle", "position": {"abbreviation": "WR"}},
					"details": {"type": "Hamstring"}
				}
			]
		}
	]`
	_, err := fixture.store.SetGameInjuries(t.Context(), game.ID, injuriesJSON)
	require.NoError(t, err)

	// Add roster players
	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
		{
			AthleteID:  "4241479",
			Team:       "MIA",
			ESPNTeamID: "15",
			FullName:   "Jaylen Waddle",
			Position:   "WR",
			UpdatedAt:  now,
		},
	}))

	// Add prop odds for one matched and one unmatched player
	oddsFetched := time.Date(2026, time.November, 9, 14, 0, 0, 0, time.UTC)
	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "MIA",
			PlayerName: "G. Davis",
			Side:       "OVER",
			Line:       new(45.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
	})
	require.NoError(t, err)

	// Fetch GET /research/games/{id}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/games/"+game.ID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Verify key injuries are present
	assert.Contains(t, body, "Josh Allen")
	assert.Contains(t, body, "QB")
	assert.Contains(t, body, "Questionable")
	assert.Contains(t, body, "Jaylen Waddle")
	assert.Contains(t, body, "WR")
	assert.Contains(t, body, "Doubtful")

	// Verify non-key injury (K / PK) was filtered out
	assert.NotContains(t, body, "Tyler Bass")

	// Verify player matches section
	assert.Contains(t, body, "Player Matches")
	assert.Contains(t, body, "Josh Allen")
	assert.Contains(t, body, "Auto")
	assert.Contains(t, body, "G. Davis")
	assert.Contains(t, body, "Unmatched")
	assert.Contains(t, body, "Manual match fix")
}

func TestResearchPlayerHandler(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// 1. Missing athlete returns 404
	req404 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/players/999999", nil)
	rec404 := httptest.NewRecorder()
	handler.ServeHTTP(rec404, req404)
	assert.Equal(t, http.StatusNotFound, rec404.Code)

	// Add roster player
	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}))

	// Add gamelog: last 5 games are [280, 310, 195, 240, 265]
	// L5 values: 280, 310, 195, 240, 265 -> sorted: 195, 240, 265, 280, 310 -> Median = 265
	// Mean of all 5: (280+310+195+240+265)/5 = 258.0
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "3918298",
		GamesPlayed: 5,
		StatsJSON:   `{"PASS_YDS":[280.0, 310.0, 195.0, 240.0, 265.0], "RUSH_YDS":[45.0, 30.0, 55.0, 20.0, 40.0]}`,
		UpdatedAt:   now,
	}))

	// 2. Test market & line from ?market=PASS_YDS&line=250.5
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/research/players/3918298?market=PASS_YDS&line=250.5",
		nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, "Josh Allen")
	assert.Contains(t, body, "5 GP")
	assert.Contains(t, body, "250.5") // Line
	assert.Contains(t, body, "265")   // Median
	assert.Contains(t, body, "258.0") // Mean
	assert.Contains(t, body, "OVER")  // Games with > 250.5
	assert.Contains(t, body, "UNDER") // Games with < 250.5

	// 3. Test comparison line resolved from ?leg_id=
	// Create a leg on slip with Line = 270.0
	leg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &game.ID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 270.0 Pass Yds",
		PlayerID:   new("3918298"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(270.0),
		Odds:       -110,
	})
	require.NoError(t, err)

	reqLeg := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/research/players/3918298?leg_id="+strconv.FormatInt(leg.ID, 10),
		nil,
	)
	recLeg := httptest.NewRecorder()
	handler.ServeHTTP(recLeg, reqLeg)
	assert.Equal(t, http.StatusOK, recLeg.Code)
	bodyLeg := recLeg.Body.String()
	assert.Contains(t, bodyLeg, `Target: <span class="font-mono font-semibold tabular-nums text-ink">270</span>`)

	// 4. Test comparison line resolved from latest prop in odds
	oddsFetched := time.Date(2026, time.November, 9, 14, 0, 0, 0, time.UTC)
	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
	})
	require.NoError(t, err)

	reqOdds := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		"/research/players/3918298",
		nil,
	)
	recOdds := httptest.NewRecorder()
	handler.ServeHTTP(recOdds, reqOdds)
	assert.Equal(t, http.StatusOK, recOdds.Code)
	bodyOdds := recOdds.Body.String()
	assert.Contains(t, bodyOdds, `Target: <span class="font-mono font-semibold tabular-nums text-ink">245.5</span>`)
}

func TestResearchMatchesHandler(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "4241479",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Gabriel Davis",
			Position:   "WR",
			UpdatedAt:  now,
		},
	}))
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Gabe Davis",
			Side:       "OVER",
			Line:       new(45.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	// 1. Missing required fields returns 400
	formBad := url.Values{
		"odds_name": {""},
		"team":      {"BUF"},
	}
	reqBad := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/research/matches",
		strings.NewReader(formBad.Encode()),
	)
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	handler.ServeHTTP(recBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, recBad.Code)

	// 2. Successful match fix saves manual=1 and redirects
	formGood := url.Values{
		"odds_name":  {"Gabe Davis"},
		"team":       {"BUF"},
		"athlete_id": {"4241479"},
		"game_id":    {game.ID},
	}
	reqGood := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/research/matches",
		strings.NewReader(formGood.Encode()),
	)
	reqGood.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recGood := httptest.NewRecorder()
	handler.ServeHTTP(recGood, reqGood)
	assert.Equal(t, http.StatusSeeOther, recGood.Code)
	assert.Equal(t, "/research?game_id="+game.ID, recGood.Header().Get("Location"))

	// Verify player match resolved
	aid, team, err := fixture.store.ResolvePlayerMatch(t.Context(), "Gabe Davis", "BUF", "MIA")
	require.NoError(t, err)
	assert.Equal(t, "4241479", aid)
	assert.Equal(t, "BUF", team)

	// 3. HTMX submission returns updated partial
	reqHTMX := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/research/matches",
		strings.NewReader(formGood.Encode()),
	)
	reqHTMX.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqHTMX.Header.Set("Hx-Request", "true")
	recHTMX := httptest.NewRecorder()
	handler.ServeHTTP(recHTMX, reqHTMX)
	assert.Equal(t, http.StatusOK, recHTMX.Code)
	bodyHTMX := recHTMX.Body.String()
	assert.NotContains(t, bodyHTMX, "<!doctype html>")
	assert.Contains(t, bodyHTMX, "Key Injuries")
	assert.Contains(t, bodyHTMX, "Manual")
}

func TestResearchLinksOnSlipLegs(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	upcomingGame := addGame(t, fixture, gameKickoff())
	startedGame, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:        "401772099",
		WeekID:    fixture.week.ID,
		HomeTeam:  "NYJ",
		AwayTeam:  "KC",
		Kickoff:   time.Now().UTC().Add(-2 * time.Hour),
		Status:    db.GameFinal,
		UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// Add upcoming player leg with PlayerID -> /research/bet
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &upcomingGame.ID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 265.5 Pass Yds",
		PlayerID:   new("3918298"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(265.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// Add upcoming game market leg -> /research/bet
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &upcomingGame.ID,
		MarketType: "SPREAD",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Selection:  "Bills -3.5",
		Odds:       -110,
	})
	require.NoError(t, err)

	// Add OTHER custom leg on upcoming game -> falls back to /research sheet
	otherLeg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &upcomingGame.ID,
		MarketType: "OTHER",
		Selection:  "Custom boost",
		Odds:       150,
	})
	require.NoError(t, err)

	// Add kicked-off player leg -> falls back to /research sheet
	startedLeg, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &startedGame.ID,
		MarketType: "RUSH_YDS",
		Selection:  "Isiah Pacheco Over 60.5 Rush Yds",
		PlayerID:   new("4361529"),
		PlayerName: new("Isiah Pacheco"),
		Side:       new("OVER"),
		Line:       new(60.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// Render slip
	slipIDText := strconv.FormatInt(fixture.slip.ID, 10)
	slipPath := "/slips/" + slipIDText
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, slipPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Upcoming non-OTHER legs link to /research/bet with from and slip_id.
	expectedPlayerHref := "/research/bet?" + url.Values{
		"from":        {slipPath},
		"game_id":     {upcomingGame.ID},
		"market":      {"PASS_YDS"},
		"player_id":   {"3918298"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
		"slip_id":     {slipIDText},
	}.Encode()
	expectedSpreadHref := "/research/bet?" + url.Values{
		"from":    {slipPath},
		"game_id": {upcomingGame.ID},
		"market":  {"SPREAD"},
		"side":    {"TEAM"},
		"slip_id": {slipIDText},
		"team":    {"BUF"},
	}.Encode()
	escapeAttrURL := func(u string) string {
		return strings.ReplaceAll(strings.ReplaceAll(u, "&", "&amp;"), "+", "&#43;")
	}
	assert.Contains(t, body, escapeAttrURL(expectedPlayerHref))
	assert.Contains(t, body, escapeAttrURL(expectedSpreadHref))
	// OTHER and kicked-off legs fall back to /research player or game sheets.
	assert.Contains(t, body, "/research?game_id="+upcomingGame.ID+"&amp;leg_id="+strconv.FormatInt(otherLeg.ID, 10))
	assert.Contains(
		t,
		body,
		"/research?leg_id="+strconv.FormatInt(
			startedLeg.ID,
			10,
		)+"&amp;line=60.5&amp;market=RUSH_YDS&amp;player_id=4361529",
	)
	assert.Contains(t, body, "Research &rarr;")
}

func TestResearchGameRendersLogosAndPortraits(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	now := time.Now().UTC()
	injuriesJSON := `[
		{
			"team": {
				"id": "2",
				"displayName": "Buffalo Bills",
				"abbreviation": "BUF",
				"logo": "https://a.espncdn.com/i/teamlogos/nfl/500/buf-inj.png"
			},
			"injuries": [
				{
					"status": "Questionable",
					"athlete": {
						"id": "4362628",
						"fullName": "Dalton Kincaid",
						"position": {"abbreviation": "TE"},
						"headshot": {"href": "https://a.espncdn.com/i/headshots/nfl/players/full/4362628.png"}
					},
					"details": {"type": "Knee", "detail": "Limited"}
				},
				{
					"status": "Out",
					"athlete": {
						"id": "9999999",
						"fullName": "Rookie Tackle",
						"position": {"abbreviation": "OT"}
					},
					"details": {"type": "Ankle", "detail": "DNP"}
				}
			]
		}
	]`

	game, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772099",
		WeekID:     fixture.week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"),
		AwayLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"),
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  now,
	})
	require.NoError(t, err)
	_, err = fixture.store.SetGameInjuries(t.Context(), game.ID, injuriesJSON)
	require.NoError(t, err)

	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Josh Allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png",
			UpdatedAt:   now,
		},
	}))

	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/games/"+game.ID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Matchup header logos
	assert.Contains(t, body, `src="https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"`)
	assert.Contains(t, body, `src="https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"`)
	// Injury team logo
	assert.Contains(t, body, `src="https://a.espncdn.com/i/teamlogos/nfl/500/buf-inj.png"`)
	// Injury row portrait + missing headshot initials fallback
	assert.Contains(t, body, `src="https://a.espncdn.com/i/headshots/nfl/players/full/4362628.png"`)
	assert.Contains(t, body, `>DK</span>`)
	assert.Contains(t, body, `>RT</span>`)
	// Matched player portrait
	assert.Contains(t, body, `src="https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png"`)
	assert.Contains(t, body, `>JA</span>`)
}

func TestResearchPlayerRendersPortraitAndTeamLogo(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	now := time.Now().UTC()

	_, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772088",
		WeekID:     fixture.week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"),
		AwayLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"),
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  now,
	})
	require.NoError(t, err)

	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Josh Allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png",
			UpdatedAt:   now,
		},
		{
			AthleteID:   "4361579",
			Team:        "MIA",
			ESPNTeamID:  "15",
			FullName:    "De'Von Achane",
			Position:    "RB",
			HeadshotURL: "",
			UpdatedAt:   now,
		},
	}))

	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/players/3918298", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `src="https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png"`)
	assert.Contains(t, body, `src="https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"`)
	assert.Contains(t, body, `>JA</span>`)

	reqNoImg := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/players/4361579", nil)
	recNoImg := httptest.NewRecorder()
	handler.ServeHTTP(recNoImg, reqNoImg)
	assert.Equal(t, http.StatusOK, recNoImg.Code)
	bodyNoImg := recNoImg.Body.String()
	assert.Contains(t, bodyNoImg, `src="https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"`)
	assert.Contains(t, bodyNoImg, `>DA</span>`)
	assert.NotContains(t, bodyNoImg, `src=""`)
}

func TestResearchPlayerSurfacesNFLverseMetricsAndHTMX(t *testing.T) {
	t.Parallel()

	seedResearchNFLverse := func(t *testing.T, fixture *slipFixture, syncedAt time.Time) {
		t.Helper()
		addGame(t, fixture, gameKickoff())
		require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
			{AthleteID: "3918298", FullName: "James Cook", Team: "BUF", Position: "RB", UpdatedAt: syncedAt},
		}))
		statsJSON := `{"RUSH_YDS":[95,82,74,110,68],"REC_YDS":[25,18,32,12,20]}`
		require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
			AthleteID: "3918298", GamesPlayed: 5, StatsJSON: statsJSON, UpdatedAt: syncedAt,
		}))

		teams := []string{"BUF", "MIA", "BAL", "SF"}
		teamWeeks := make([]db.NFLverseTeamWeek, 0, len(teams))
		for i, abbr := range teams {
			teamWeeks = append(teamWeeks, db.NFLverseTeamWeek{
				Season:            2026,
				SeasonType:        2,
				WeekNum:           1,
				Team:              abbr,
				DefPassEPAAllowed: float64(i) * 0.05,            // MIA is #2 pass def
				DefRushEPAAllowed: float64(len(teams)-i) * 0.05, // MIA is #3 rush def
				UpdatedAt:         syncedAt,
			})
		}
		_, err := fixture.store.UpsertNFLverseTeamWeeks(t.Context(), teamWeeks)
		require.NoError(t, err)
		require.NoError(t, fixture.store.RecordSync(t.Context(), "nflverse:season:2026", syncedAt, nil))

		var playerWeeks []db.NFLversePlayerWeek
		for wk := 1; wk <= 3; wk++ {
			playerWeeks = append(playerWeeks, db.NFLversePlayerWeek{
				Season:      2026,
				SeasonType:  2,
				WeekNum:     wk,
				ESPNID:      "3918298",
				Team:        "BUF",
				OffensePct:  0.72,
				TargetShare: 0.14,
				WOPR:        0.38,
				UpdatedAt:   syncedAt,
			})
		}
		_, err = fixture.store.UpsertNFLversePlayerWeeks(t.Context(), playerWeeks)
		require.NoError(t, err)
	}

	t.Run("fresh nflverse surfaces position usage tiles and HTMX market swap", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		seedResearchNFLverse(t, fixture, time.Now().UTC())
		handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

		// 1. HTMX partial swap via GET /research?player_id=3918298 (RB defaults to RUSH_YDS).
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?player_id=3918298", nil)
		req.Header.Set("Hx-Request", "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := rec.Body.String()
		assert.Contains(t, body, "Snap % (L3)")
		assert.Contains(t, body, "72%")
		assert.Contains(t, body, "Carries/g (L3)")
		assert.Contains(t, body, "Targets/g (L3)")
		assert.Contains(t, body, "RZ Opps/g")
		assert.Contains(t, body, "Last 5 Games · Rush Yds")
		// Mobile-width 2-column grid layout on phone screens.
		assert.Contains(t, body, "grid-cols-2 gap-2 sm:grid-cols-4")
		// Partial response should not include full page wrapper.
		assert.NotContains(t, body, `id="research-panel"`)

		// 2. HTMX market tab swap to REC_YDS switches active market chart.
		reqRec := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/research/players/3918298?market=REC_YDS", nil,
		)
		reqRec.Header.Set("Hx-Request", "true")
		recSwap := httptest.NewRecorder()
		handler.ServeHTTP(recSwap, reqRec)
		require.Equal(t, http.StatusOK, recSwap.Code)

		swapBody := recSwap.Body.String()
		assert.Contains(t, swapBody, "Last 5 Games · Rec Yds")
	})

	t.Run("stale nflverse fails soft without usage tiles", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		seedResearchNFLverse(t, fixture, time.Now().UTC().Add(-8*24*time.Hour))
		handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/players/3918298", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := rec.Body.String()
		assert.Contains(t, body, "James Cook")
		assert.NotContains(t, body, "Snap % (L3)")
		assert.NotContains(t, body, "Target share")
		assert.NotContains(t, body, "WOPR")
		assert.NotContains(t, body, "Opp Rush Def")
	})
}

func TestResearchGameResolvesTeamHintInMemory(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:   "5001",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Ollie Gordon",
			Position:    "RB",
			HeadshotURL: "https://example.com/buf-ollie.png",
			UpdatedAt:   now,
		},
		{
			AthleteID:   "5002",
			Team:        "MIA",
			ESPNTeamID:  "15",
			FullName:    "Ollie Gordon II",
			Position:    "RB",
			HeadshotURL: "https://example.com/mia-ollie.png",
			UpdatedAt:   now,
		},
		{
			AthleteID:  "6001",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Chris Jones",
			Position:   "DT",
			UpdatedAt:  now,
		},
		{
			AthleteID:  "6002",
			Team:       "MIA",
			ESPNTeamID: "15",
			FullName:   "Chris Jones",
			Position:   "DT",
			UpdatedAt:  now,
		},
	}))

	oddsFetched := time.Date(2026, time.November, 9, 14, 0, 0, 0, time.UTC)
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "",
			PlayerName: "Ollie Gordon II (MIA)",
			Side:       "OVER",
			Line:       new(42.5),
			Price:      -110,
			FetchedAt:  oddsFetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "ANYTIME_TD",
			Team:       "",
			PlayerName: "Chris Jones",
			Side:       "YES",
			Price:      +500,
			FetchedAt:  oddsFetched,
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/games/"+game.ID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "Ollie Gordon II (MIA)")
	assert.Contains(t, body, "Ollie Gordon II")
	assert.Contains(t, body, "https://example.com/mia-ollie.png")
	assert.Contains(t, body, "Auto")
	assert.Contains(t, body, "Chris Jones")
	assert.Contains(t, body, "Unmatched")
}

func TestResearchGameDropsStalePropPlayers(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	staleFetch := time.Date(2026, time.November, 9, 10, 0, 0, 0, time.UTC)
	freshFetch := time.Date(2026, time.November, 9, 14, 0, 0, 0, time.UTC)
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			PlayerName: "Stale Runner",
			Side:       "OVER",
			Line:       new(30.5),
			Price:      -110,
			FetchedAt:  staleFetch,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(250.5),
			Price:      -110,
			FetchedAt:  freshFetch,
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/games/"+game.ID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "Josh Allen")
	assert.NotContains(t, body, "Stale Runner")
}

func TestResearchGameIncludesTruncatedRosterInjuries(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	injuriesJSON := `[
		{
			"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
			"injuries": [
				{
					"status": "Out",
					"athlete": {"id": "999", "fullName": "Curtis Samuel", "position": {"abbreviation": "WR"}},
					"details": {"type": "Foot"}
				}
			]
		}
	]`
	_, err := fixture.store.SetGameInjuries(t.Context(), game.ID, injuriesJSON)
	require.NoError(t, err)

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:    "4362887",
			Team:         "BUF",
			ESPNTeamID:   "2",
			FullName:     "Dalton Kincaid",
			Position:     "TE",
			HeadshotURL:  "https://a.espncdn.com/i/headshots/nfl/players/full/4362887.png",
			InjuryStatus: "Out",
			InjuryDate:   "2026-11-20T17:00Z",
			UpdatedAt:    now,
		},
		{
			AthleteID:    "5550001",
			Team:         "BUF",
			ESPNTeamID:   "2",
			FullName:     "Depth Linebacker",
			Position:     "LB",
			InjuryStatus: "Out",
			InjuryDate:   "2026-11-20T17:00Z",
			UpdatedAt:    now,
		},
	}))

	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/games/"+game.ID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "Key Injuries")
	assert.Contains(t, body, "Curtis Samuel")
	assert.Contains(t, body, "Dalton Kincaid")
	assert.NotContains(t, body, "Depth Linebacker")
}

func TestResearchPlayerMarketTabResolvesOwnLine(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID: "3918298",
			Team:      "BUF",
			FullName:  "Josh Allen",
			Position:  "QB",
			UpdatedAt: now,
		},
	}))
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "3918298",
		GamesPlayed: 5,
		StatsJSON:   `{"PASS_YDS":[280,310,195,240,265],"RUSH_YDS":[45,30,55,20,40]}`,
		UpdatedAt:   now,
	}))

	fetched := time.Date(2026, time.November, 9, 14, 0, 0, 0, time.UTC)
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(245.5),
			Price:      -110,
			FetchedAt:  fetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(42.5),
			Price:      -110,
			FetchedAt:  fetched,
		},
	})
	require.NoError(t, err)

	// Pass Yards tab: resolves 245.5 and tab links omit &line=.
	reqPass := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/players/3918298?market=PASS_YDS", nil,
	)
	recPass := httptest.NewRecorder()
	handler.ServeHTTP(recPass, reqPass)
	require.Equal(t, http.StatusOK, recPass.Code)
	passBody := recPass.Body.String()
	assert.Contains(t, passBody, `Target: <span class="font-mono font-semibold tabular-nums text-ink">245.5</span>`)
	assert.Contains(t, passBody, `href="/research/players/3918298?market=RUSH_YDS"`)
	assert.NotContains(t, passBody, `market=RUSH_YDS&amp;line=`)
	assert.NotContains(t, passBody, `market=RUSH_YDS&line=`)

	// Rush Yards tab: resolves its own 42.5 line instead of carrying 245.5.
	reqRush := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/players/3918298?market=RUSH_YDS", nil,
	)
	recRush := httptest.NewRecorder()
	handler.ServeHTTP(recRush, reqRush)
	require.Equal(t, http.StatusOK, recRush.Code)
	assert.Contains(
		t,
		recRush.Body.String(),
		`Target: <span class="font-mono font-semibold tabular-nums text-ink">42.5</span>`,
	)

	// Explicit ?line= still overrides.
	reqOverride := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/players/3918298?market=RUSH_YDS&line=50.5", nil,
	)
	recOverride := httptest.NewRecorder()
	handler.ServeHTTP(recOverride, reqOverride)
	require.Equal(t, http.StatusOK, recOverride.Code)
	assert.Contains(
		t,
		recOverride.Body.String(),
		`Target: <span class="font-mono font-semibold tabular-nums text-ink">50.5</span>`,
	)
}

func TestResearchPlayerRichChartsAndMatchupContext(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff()) // BUF (home) vs MIA (away)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:    "4241479",
			Team:         "BUF",
			FullName:     "Khalil Shakir",
			Position:     "WR",
			InjuryStatus: "Questionable",
			UpdatedAt:    now,
		},
	}))

	// 6 games most-recent first: 5 of 6 over 74.5; L5 is [90, 80, 110, 50, 85] -> 4/5 hit, range 50–110.
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "4241479",
		GamesPlayed: 6,
		StatsJSON:   `{"REC_YDS":[90,80,110,50,85,100]}`,
		GamesJSON: `[
			{"eventId":"6","week":6,"opponent":"KC","atVs":"@","result":"W","score":"24-20"},
			{"eventId":"5","week":5,"opponent":"BAL","atVs":"vs","result":"L","score":"20-27"},
			{"eventId":"4","week":4,"opponent":"NYJ","atVs":"@","result":"W","score":"30-10"},
			{"eventId":"3","week":3,"opponent":"NE","atVs":"vs","result":"W","score":"28-14"},
			{"eventId":"2","week":2,"opponent":"CIN","atVs":"@","result":"L","score":"17-24"}
		]`,
		UpdatedAt: now,
	}))

	// Cached game weather + spread/total + player prop line.
	_, err := fixture.store.SetGameWeather(t.Context(), game.ID, 42.0, 14.0, 0, 0, now)
	require.NoError(t, err)
	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "HOME",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TOTAL",
			Side:       "OVER",
			Line:       new(48.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Khalil Shakir",
			Side:       "OVER",
			Line:       new(74.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	// Fresh nflverse team + player weeks for overall/pos defense ranks and Opp Defense L5.
	require.NoError(t, fixture.store.RecordSync(t.Context(), "nflverse:season:2026", now, nil))
	_, err = fixture.store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "KC",
			DefPassEPAAllowed: 0.05,
			DefRushEPAAllowed: 0.05,
			UpdatedAt:         now,
		},
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "MIA",
			DefPassEPAAllowed: 0.25,
			DefRushEPAAllowed: 0.25,
			UpdatedAt:         now,
		},
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "BAL",
			DefPassEPAAllowed: 0.10,
			DefRushEPAAllowed: 0.10,
			UpdatedAt:         now,
		},
	})
	require.NoError(t, err)
	_, err = fixture.store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			Season:        2026,
			SeasonType:    2,
			WeekNum:       1,
			ESPNID:        "4241479",
			PlayerName:    "Khalil Shakir",
			Team:          "BUF",
			Opponent:      "KC",
			Position:      "WR",
			OffensePct:    0.82,
			TargetShare:   0.26,
			AirYardsShare: 0.31,
			WOPR:          0.62,
			RecYds:        90,
			UpdatedAt:     now,
		},
		{
			Season:     2026,
			SeasonType: 2,
			WeekNum:    1,
			ESPNID:     "4362628",
			PlayerName: "Ja'Marr Chase",
			Team:       "CIN",
			Opponent:   "MIA",
			Position:   "WR",
			RecYds:     112,
			UpdatedAt:  now,
		},
		{
			Season:     2026,
			SeasonType: 2,
			WeekNum:    2,
			ESPNID:     "4047646",
			PlayerName: "Garrett Wilson",
			Team:       "NYJ",
			Opponent:   "MIA",
			Position:   "WR",
			RecYds:     68,
			UpdatedAt:  now,
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research/players/4241479", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()

	// Injury badge & phone-first matchup strip.
	assert.Contains(t, body, "Questionable")
	assert.Contains(t, body, "vs MIA")
	assert.Contains(t, body, `Spread <span class="font-mono font-semibold tabular-nums text-ink">-3.5</span>`)
	assert.Contains(t, body, `O/U <span class="font-mono font-semibold tabular-nums text-ink">48.5</span>`)
	assert.Contains(t, body, "42°F · 14 mph")

	// Hit-rate & L5 range tiles.
	assert.Contains(t, body, "L5 Hit")
	assert.Contains(t, body, "4/5")
	assert.Contains(t, body, "Season Hit")
	assert.Contains(t, body, "5/6")
	assert.Contains(t, body, "L5 Range")
	assert.Contains(t, body, "50–110")

	// WR position-tailored usage tiles.
	assert.Contains(t, body, "Snap % (L3)")
	assert.Contains(t, body, "82%")
	assert.Contains(t, body, "Target share")
	assert.Contains(t, body, "26.0%")
	assert.Contains(t, body, "Air Yards %")
	assert.Contains(t, body, "31.0%")
	assert.Contains(t, body, "WOPR")
	assert.Contains(t, body, "0.62")

	// Dotted O/U reference line appears on both charts (player L5 + opp defense L5).
	assert.Equal(t, 2, strings.Count(body, "border-dashed border-warning"))
	assert.Equal(t, 2, strings.Count(body, "O/U 74.5"))

	// Player L5 bar opponent labels and defense-rank badges.
	assert.Contains(t, body, "@KC")
	assert.Contains(t, body, "vs BAL")
	assert.Contains(t, body, "Def #1")
	assert.Contains(t, body, "vs WR #1")
	assert.Contains(t, body, "bar-hit")
	assert.Contains(t, body, "bar-miss")
	assert.Contains(t, body, "badge-lost")

	// Opposing defense L5 chart.
	assert.Contains(t, body, "MIA Defense · Last 5 vs WR1 · Rec Yds")
	assert.Contains(t, body, "Def #3")
	assert.Contains(t, body, "vs WR #2")
	assert.Contains(t, body, "J. Chase")
	assert.Contains(t, body, "CIN · W1")
	assert.Contains(t, body, "G. Wilson")
	assert.Contains(t, body, "NYJ · W2")
}

func TestResearchPlayerNFLverseFallbackForWR(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})
	ctx := t.Context()
	now := time.Now().UTC()

	week, err := fixture.store.UpsertWeek(ctx, 2026, 2, 10)
	require.NoError(t, err)
	_, err = fixture.store.UpsertGame(ctx, db.Game{
		ID:       "401999101",
		WeekID:   week.ID,
		HomeTeam: "PHI",
		AwayTeam: "DAL",
		HomeName: "Philadelphia Eagles",
		AwayName: "Dallas Cowboys",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)
	require.NoError(t, fixture.store.UpsertRosterPlayers(ctx, []db.RosterPlayer{
		{
			AthleteID: "5083754",
			Team:      "DAL",
			FullName:  "Ryan Flournoy",
			Position:  "WR",
			UpdatedAt: now,
		},
	}))
	_, err = fixture.store.UpsertOdds(ctx, []db.Odds{
		{
			GameID:     "401999101",
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "DAL",
			PlayerName: "Ryan Flournoy",
			Side:       "OVER",
			Line:       new(32.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)
	// Intentionally no player_gamelogs row — only nflverse_player_weeks.
	_, err = fixture.store.UpsertNFLversePlayerWeeks(ctx, []db.NFLversePlayerWeek{
		{
			ESPNID:     "5083754",
			Season:     2026,
			SeasonType: 2,
			WeekNum:    9,
			Team:       "DAL",
			Opponent:   "IND",
			PlayerName: "Ryan Flournoy",
			Position:   "WR",
			RecYds:     53,
			Receptions: 4,
			OffensePct: 0.72,
			UpdatedAt:  now,
		},
		{
			ESPNID:     "5083754",
			Season:     2026,
			SeasonType: 2,
			WeekNum:    8,
			Team:       "DAL",
			Opponent:   "NYG",
			PlayerName: "Ryan Flournoy",
			Position:   "WR",
			RecYds:     28,
			Receptions: 2,
			OffensePct: 0.65,
			UpdatedAt:  now,
		},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(
		rec,
		httptest.NewRequestWithContext(ctx, http.MethodGet, "/research/players/5083754", nil),
	)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "Ryan Flournoy")
	assert.Contains(t, body, "Last 5 Games · Rec Yds")
	assert.NotContains(t, body, "No game log stats available for this market.")
	assert.Contains(t, body, "53")
	assert.Contains(t, body, "28")
	assert.Contains(t, body, "IND")
	assert.Contains(t, body, "NYG")
	assert.Contains(t, body, "2 GP")
}

// seedResearchExplorer writes one upcoming game, a rostered player, and a mix
// of game and prop odds the explorer can read from cache.
func seedResearchExplorer(t *testing.T, fixture *slipFixture, game db.Game) {
	t.Helper()

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}))
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(42.5),
			Price:      +120,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(42.5),
			Price:      +120,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)
}

func TestResearchExplorerBrowsesCachedBetsWithoutRefresh(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedResearchExplorer(t, fixture, game)
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "Find a leg across every cached line this week.")
	assert.Contains(t, body, "Bet type")
	assert.Contains(t, body, "Best grade")
	assert.Contains(t, body, "3 bets match")
	assert.Contains(t, body, "MIA @ BUF")
	assert.Contains(t, body, "Josh Allen Over 265.5 Pass Yds")
	assert.Contains(t, body, "Josh Allen Over 42.5 Rush Yds")
	assert.Contains(t, body, "Research bet &rarr;")
	assert.Contains(t, body, "/research/bet?")
	assert.Contains(t, body, "from=%2Fresearch")

	assert.Empty(t, refresher.propsSynced, "browsing must not fetch props")
}

func TestResearchExploreFiltersSortsAndPushesURL(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedResearchExplorer(t, fixture, game)
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// Passing only, sorted by odds. The partial swaps in place and pushes the
	// matching full-page URL.
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore?cat=passing&sort=odds", nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/research?cat=passing&sort=odds", rec.Header().Get("Hx-Push-Url"))

	body := rec.Body.String()
	assert.NotContains(t, body, "<!doctype html>")
	assert.Contains(t, body, "Josh Allen Over 265.5 Pass Yds")
	assert.NotContains(t, body, "Josh Allen Over 42.5 Rush Yds", "the bet-type chip filters rushing out")
	assert.Contains(t, body, ">Passing<span", "the Passing chip is current")

	// Odds sort puts the shorter price first.
	reqOdds := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore?q=allen&sort=odds", nil,
	)
	recOdds := httptest.NewRecorder()
	handler.ServeHTTP(recOdds, reqOdds)
	require.Equal(t, http.StatusOK, recOdds.Code)
	oddsBody := recOdds.Body.String()
	passAt := strings.Index(oddsBody, "Josh Allen Over 265.5 Pass Yds")
	rushAt := strings.Index(oddsBody, "Josh Allen Over 42.5 Rush Yds")
	require.Positive(t, passAt)
	require.Positive(t, rushAt)
	assert.Less(t, passAt, rushAt, "-110 sorts before +120")

	// A search with no match gets a friendly empty state.
	reqEmpty := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore?q=nobody", nil,
	)
	recEmpty := httptest.NewRecorder()
	handler.ServeHTTP(recEmpty, reqEmpty)
	require.Equal(t, http.StatusOK, recEmpty.Code)
	assert.Contains(t, recEmpty.Body.String(), "Nothing matches those filters.")

	// Targeting a game through the explorer stays on cached data.
	reqGame := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore?game_id="+game.ID, nil,
	)
	recGame := httptest.NewRecorder()
	handler.ServeHTTP(recGame, reqGame)
	require.Equal(t, http.StatusOK, recGame.Code)
	assert.Equal(t, "/research?game_id="+game.ID, recGame.Header().Get("Hx-Push-Url"))

	assert.Empty(t, refresher.propsSynced, "exploring must not fetch props")
}

func TestResearchExploreReplacesOpenSheet(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedResearchExplorer(t, fixture, game)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	// Targeting a game loads its sheet into the out-of-band panel.
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore?game_id="+game.ID, nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `id="research-panel" hx-swap-oob="true"`)
	assert.Contains(t, body, "Game sheet")
	assert.Contains(t, body, "Key Injuries")

	// Switching the targeted game replaces the sheet rather than leaving the
	// old one above a list that now names another game.
	other, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:         "401772002",
		WeekID:     fixture.week.ID,
		HomeTeam:   "DAL",
		AwayTeam:   "PHI",
		HomeName:   "Dallas Cowboys",
		AwayName:   "Philadelphia Eagles",
		HomeESPNID: "6",
		AwayESPNID: "21",
		Kickoff:    gameKickoff(),
		Status:     db.GameScheduled,
		UpdatedAt:  time.Now().UTC(),
	})
	require.NoError(t, err)

	reqOther := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore?game_id="+other.ID, nil,
	)
	recOther := httptest.NewRecorder()
	handler.ServeHTTP(recOther, reqOther)
	require.Equal(t, http.StatusOK, recOther.Code)
	otherBody := recOther.Body.String()
	assert.Contains(t, otherBody, ">PHI @ DAL</h2>")
	assert.Contains(t, otherBody, "Game sheet")
	assert.NotContains(t, otherBody, ">MIA @ BUF</h2>", "the previous game's sheet is gone")

	// Clearing the target empties the panel, so no sheet lingers at all.
	reqClear := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/explore", nil,
	)
	recClear := httptest.NewRecorder()
	handler.ServeHTTP(recClear, reqClear)
	require.Equal(t, http.StatusOK, recClear.Code)
	clearBody := recClear.Body.String()
	assert.Contains(t, clearBody, `id="research-panel" hx-swap-oob="true"`)
	assert.NotContains(t, clearBody, ">Game sheet</p>")
	assert.NotContains(t, clearBody, ">Player sheet</p>")
}

func TestResearchTargetedGameOffersPropsButton(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	now := time.Now().UTC()
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TOTAL",
			Side:       "OVER",
			Line:       new(47.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "TOTAL",
			Side:       "OVER",
			Line:       new(47.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// A targeted game with lines but no props offers the explicit load button.
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research?game_id="+game.ID, nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Props for MIA @ BUF aren&rsquo;t loaded yet.")
	assert.Contains(t, body, "Load props")
	assert.Contains(t, body, "Key Injuries", "the targeted game sheet still renders")

	// The button is the one path that fetches props.
	form := url.Values{"game_id": {game.ID}}
	reqProps := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/research/games/"+game.ID+"/props",
		strings.NewReader(form.Encode()),
	)
	reqProps.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqProps.Header.Set("Hx-Request", "true")
	recProps := httptest.NewRecorder()
	handler.ServeHTTP(recProps, reqProps)
	require.Equal(t, http.StatusOK, recProps.Code)
	assert.Equal(t, []string{game.ID}, refresher.propsSynced)
	assert.NotContains(t, recProps.Body.String(), "<!doctype html>")

	// Once cached props exist, the button drops away.
	_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	reqWithProps := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research?game_id="+game.ID, nil,
	)
	recWithProps := httptest.NewRecorder()
	handler.ServeHTTP(recWithProps, reqWithProps)
	require.Equal(t, http.StatusOK, recWithProps.Code)
	withPropsBody := recWithProps.Body.String()
	assert.NotContains(t, withPropsBody, "Load props")
	assert.Contains(t, withPropsBody, "Josh Allen Over 265.5 Pass Yds")
}

// seedBetResearchPlayer writes a rostered passer with a game log and three
// books on both sides of one prop line.
func seedBetResearchPlayer(t *testing.T, fixture *slipFixture, game db.Game) {
	t.Helper()

	now := time.Now().UTC()
	require.NoError(t, fixture.store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:  "3918298",
			Team:       "BUF",
			ESPNTeamID: "2",
			FullName:   "Josh Allen",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}))
	require.NoError(t, fixture.store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "3918298",
		GamesPlayed: 5,
		StatsJSON:   `{"PASS_YDS":[280,310,195,240,265]}`,
		UpdatedAt:   now,
	}))

	row := func(book, side string, price int) db.Odds {
		return db.Odds{
			GameID:     game.ID,
			Book:       book,
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       side,
			Line:       new(265.5),
			Price:      price,
			FetchedAt:  now,
		}
	}
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		row("betmgm", "OVER", -115),
		row("betmgm", "UNDER", -105),
		row("draftkings", "OVER", -110),
		row("draftkings", "UNDER", -110),
		row("fanduel", "OVER", -105),
		row("fanduel", "UNDER", -115),
	})
	require.NoError(t, err)
}

func TestResearchBetPlayerPropRendersPlayerResearchAndPrices(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedBetResearchPlayer(t, fixture, game)

	homeBook := "fanduel"
	updated, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, &homeBook)
	require.NoError(t, err)

	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, updated, refresher)

	values := url.Values{
		"game_id":   {game.ID},
		"market":    {"PASS_YDS"},
		"side":      {"OVER"},
		"team":      {"BUF"},
		"player_id": {"3918298"},
		"from":      {"/research?cat=passing"},
	}
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/bet?"+values.Encode(), nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "Josh Allen Over 265.5 Pass Yds")
	assert.Contains(t, body, "MIA @ BUF")
	assert.Contains(t, body, "Book prices")
	assert.Contains(t, body, "Best price")
	assert.Contains(t, body, "Your book")
	assert.Contains(t, body, "De-vig fair")
	assert.Contains(t, body, ">fanduel<")
	assert.Contains(t, body, "Grade &amp; analyzer")
	assert.Contains(t, body, "data-grade-scope")
	assert.Contains(t, body, "data-build-descriptor")
	assert.Contains(t, body, "data-grade-drawer")
	assert.Contains(t, body, "data-ground-rule-banner")
	assert.Contains(t, body, "data-market-reality")
	assert.Contains(t, body, "data-slip-diagnostics")
	assert.Contains(t, body, `data-provenance="Vegas Implied"`)
	assert.Contains(t, body, `data-provenance="Fair No-Vig Est."`)
	assert.Contains(t, body, `data-provenance="Model Context"`)
	assert.Contains(t, body, "~1 in 2")
	assert.Contains(t, body, "Coin Flip")
	assert.Contains(t, body, "Lottery Ticket")
	assert.Contains(t, body, "data-layer3-why")
	assert.Contains(t, body, `data-why-popover="implied_prob"`)
	assert.Contains(t, body, `data-why-popover="vig_hold"`)
	assert.Contains(t, body, `data-why-popover="corr_tax"`)
	assert.Contains(t, body, `data-why-popover="already_priced_in"`)
	assert.Contains(t, body, "data-model-card")
	assert.NotContains(t, body, "+EV")
	assert.Contains(t, body, "Last 5 Games · Pass Yds")
	assert.Contains(t, body, "L5 Hit")
	assert.Contains(t, body, "Full player sheet")
	assert.NotContains(t, body, "Team context")
	assert.Contains(t, body, `href="/research?cat=passing"`)
	assert.Contains(t, body, "formaction=\"/slips/"+strconv.FormatInt(fixture.slip.ID, 10)+"/legs\"")

	assert.Empty(t, refresher.propsSynced, "bet research must not fetch props")
}

func TestResearchBetGameMarketRendersTeamContext(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	now := time.Now().UTC()

	row := func(book, market, team, side string, line *float64, price int) db.Odds {
		return db.Odds{
			GameID:     game.ID,
			Book:       book,
			MarketType: market,
			Team:       team,
			Side:       side,
			Line:       line,
			Price:      price,
			FetchedAt:  now,
		}
	}
	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		row("draftkings", "SPREAD", "BUF", "TEAM", new(-3.5), -110),
		row("draftkings", "SPREAD", "MIA", "TEAM", new(3.5), -110),
		row("fanduel", "SPREAD", "BUF", "TEAM", new(-3.0), -105),
		row("fanduel", "SPREAD", "MIA", "TEAM", new(3.0), -115),
		row("betmgm", "SPREAD", "BUF", "TEAM", new(-3.0), -115),
		row("betmgm", "SPREAD", "MIA", "TEAM", new(3.0), -105),
		row("draftkings", "TOTAL", "", "OVER", new(47.5), -110),
		row("fanduel", "TOTAL", "", "OVER", new(47.5), -110),
		row("draftkings", "TOTAL", "", "UNDER", new(47.5), -110),
		row("fanduel", "TOTAL", "", "UNDER", new(47.5), -110),
	})
	require.NoError(t, err)

	injuriesJSON := `[
		{
			"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
			"injuries": [
				{
					"status": "Questionable",
					"athlete": {"id": "3918298", "fullName": "Josh Allen", "position": {"abbreviation": "QB"}},
					"details": {"type": "Ankle"}
				}
			]
		}
	]`
	_, err = fixture.store.SetGameInjuries(t.Context(), game.ID, injuriesJSON)
	require.NoError(t, err)

	require.NoError(t, fixture.store.RecordSync(t.Context(), analyzer.NfeloSource, now, nil))
	_, err = fixture.store.UpsertNfeloGames(t.Context(), []db.NfeloGame{
		{GameID: game.ID, HomeSpread: -2.5, HomeWinProb: 0.58, UpdatedAt: now},
	})
	require.NoError(t, err)

	require.NoError(t, fixture.store.RecordSync(t.Context(), "nflverse:season:2026", now, nil))
	_, err = fixture.store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "BUF",
			OffPassEPA:        0.12,
			OffRushEPA:        0.04,
			DefPassEPAAllowed: 0.03,
			DefRushEPAAllowed: 0.08,
			UpdatedAt:         now,
		},
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "MIA",
			OffPassEPA:        0.09,
			OffRushEPA:        0.01,
			DefPassEPAAllowed: 0.14,
			DefRushEPAAllowed: 0.11,
			UpdatedAt:         now,
		},
	})
	require.NoError(t, err)

	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	values := url.Values{
		"game_id": {game.ID},
		"market":  {"SPREAD"},
		"side":    {"TEAM"},
		"team":    {"BUF"},
	}
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/research/bet?"+values.Encode(), nil,
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, "BUF -3")
	assert.Contains(t, body, "Team context")
	assert.Contains(t, body, "Model spread")
	assert.Contains(t, body, "BUF -2.5")
	assert.Contains(t, body, "BUF 58%")
	assert.Contains(t, body, "MIA 42%")
	assert.Contains(t, body, "EPA ranks")
	assert.Contains(t, body, "Off pass")
	assert.Contains(t, body, "Def overall")
	assert.Contains(t, body, "BUF")
	assert.Contains(t, body, "MIA")
	assert.Contains(t, body, "Key injuries")
	assert.Contains(t, body, "Josh Allen")
	assert.Contains(t, body, "Questionable")
	assert.Contains(t, body, "Full game sheet")
	assert.Contains(t, body, "De-vig fair")
	assert.Contains(t, body, "More in this game", "the same game's other markets stay reachable")
	assert.NotContains(t, body, "Player research")
	assert.Empty(t, refresher.propsSynced)
}

func TestResearchBetMissingSelectionReturnsNotFound(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	cases := []string{
		"/research/bet",
		"/research/bet?game_id=missing&market=SPREAD&side=TEAM&team=BUF",
		"/research/bet?game_id=" + game.ID + "&market=SPREAD&side=OVER&team=BUF",
	}
	for _, path := range cases {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
		assert.Contains(t, rec.Body.String(), "Bet not found", path)
		assert.Contains(t, rec.Body.String(), `href="/research"`, path)
	}
}

func TestResearchBetAddToSlipOnlyWhenOpenSlipIsPicking(t *testing.T) {
	t.Parallel()

	t.Run("open slip in the pick step offers the add", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		game := addGame(t, fixture, gameKickoff())
		seedBetResearchPlayer(t, fixture, game)
		handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

		values := url.Values{
			"game_id":   {game.ID},
			"market":    {"PASS_YDS"},
			"side":      {"OVER"},
			"team":      {"BUF"},
			"player_id": {"3918298"},
		}
		betURL := "/research/bet?" + values.Encode()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, betURL, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := rec.Body.String()
		assert.Contains(t, body, "Add to slip")
		assert.Contains(t, body, "+ Week 9")
		slipPath := "/slips/" + strconv.FormatInt(fixture.slip.ID, 10) + "/legs"
		assert.Contains(t, body, "formaction=\""+slipPath+"\"")
		assert.Contains(t, body, "hx-post=\""+slipPath+"\"")
		assert.Contains(t, body, `hx-include="closest form"`)

		// The form posts the existing add-leg route and returns to this bet.
		form := url.Values{
			"quick":       {"1"},
			"from":        {"research"},
			"return":      {betURL},
			"game_id":     {game.ID},
			"market_type": {"PASS_YDS"},
			"team":        {"BUF"},
			"player_name": {"Josh Allen"},
			"player_id":   {"3918298"},
			"side":        {"OVER"},
			"line":        {"265.5"},
			"odds":        {"-105"},
		}
		reqAdd := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/slips/"+strconv.FormatInt(fixture.slip.ID, 10)+"/legs",
			strings.NewReader(form.Encode()),
		)
		reqAdd.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recAdd := httptest.NewRecorder()
		handler.ServeHTTP(recAdd, reqAdd)
		assert.Equal(t, http.StatusSeeOther, recAdd.Code)
		assert.Equal(t, betURL, recAdd.Header().Get("Location"))
	})

	t.Run("a game outside the slip's week offers nothing", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		otherWeek, err := fixture.store.UpsertWeek(t.Context(), 2026, 2, 10)
		require.NoError(t, err)
		game, err := fixture.store.UpsertGame(t.Context(), db.Game{
			ID:         "401772099",
			WeekID:     otherWeek.ID,
			HomeTeam:   "BUF",
			AwayTeam:   "MIA",
			HomeESPNID: "2",
			AwayESPNID: "15",
			Kickoff:    gameKickoff(),
			Status:     db.GameScheduled,
			UpdatedAt:  time.Now().UTC(),
		})
		require.NoError(t, err)
		_, err = fixture.store.UpsertOdds(t.Context(), []db.Odds{
			{
				GameID:     game.ID,
				Book:       "draftkings",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -110,
				FetchedAt:  time.Now().UTC(),
			},
			{
				GameID:     game.ID,
				Book:       "fanduel",
				MarketType: "SPREAD",
				Team:       "BUF",
				Side:       "TEAM",
				Line:       new(-3.5),
				Price:      -110,
				FetchedAt:  time.Now().UTC(),
			},
		})
		require.NoError(t, err)

		handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})
		values := url.Values{
			"game_id": {game.ID},
			"market":  {"SPREAD"},
			"side":    {"TEAM"},
			"team":    {"BUF"},
		}
		req := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/research/bet?"+values.Encode(), nil,
		)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := rec.Body.String()
		assert.Contains(t, body, "BUF -3.5")
		assert.NotContains(t, body, "Add to slip")
	})

	t.Run("a slip that is no longer open offers nothing", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		game := addGame(t, fixture, gameKickoff())
		seedBetResearchPlayer(t, fixture, game)
		require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
		handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

		values := url.Values{
			"game_id":   {game.ID},
			"market":    {"PASS_YDS"},
			"side":      {"OVER"},
			"team":      {"BUF"},
			"player_id": {"3918298"},
		}
		req := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/research/bet?"+values.Encode(), nil,
		)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := rec.Body.String()
		assert.Contains(t, body, "Josh Allen Over 265.5 Pass Yds")
		assert.NotContains(t, body, "Add to slip")
		assert.NotContains(t, body, "formaction=\"/slips/")
	})
}

// The app puts auth.CSRF in front of every non-GET route. The bet page's slip
// buttons must carry the HTMX header, or the production guard rejects the add.
func TestResearchBetAddToSlipPassesCSRF(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedBetResearchPlayer(t, fixture, game)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})
	guard := auth.NewHandlers(zerolog.Nop(), nil, nil, nil, nil, false).CSRF(handler)

	values := url.Values{
		"game_id":   {game.ID},
		"market":    {"PASS_YDS"},
		"side":      {"OVER"},
		"team":      {"BUF"},
		"player_id": {"3918298"},
	}
	betURL := "/research/bet?" + values.Encode()
	form := url.Values{
		"quick":       {"1"},
		"from":        {"research"},
		"return":      {betURL},
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"team":        {"BUF"},
		"player_name": {"Josh Allen"},
		"player_id":   {"3918298"},
		"side":        {"OVER"},
		"line":        {"265.5"},
		"odds":        {"-105"},
	}
	path := "/slips/" + strconv.FormatInt(fixture.slip.ID, 10) + "/legs"

	// A plain form post, which is what the page would send without htmx, is
	// rejected before the handler runs.
	reqPlain := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()),
	)
	reqPlain.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recPlain := httptest.NewRecorder()
	guard.ServeHTTP(recPlain, reqPlain)
	assert.Equal(t, http.StatusForbidden, recPlain.Code)

	// An HTMX post passes the guard and asks htmx to navigate back to the bet.
	reqHTMX := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()),
	)
	reqHTMX.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqHTMX.Header.Set("Hx-Request", "true")
	recHTMX := httptest.NewRecorder()
	guard.ServeHTTP(recHTMX, reqHTMX)
	require.Equal(t, http.StatusOK, recHTMX.Code)
	assert.Equal(t, betURL, recHTMX.Header().Get("Hx-Redirect"))
}

func TestResearchExploreAndBetRenderSplitLinesWithHomeBookFirst(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	now := time.Now().UTC()

	_, err := fixture.store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "MIA",
			Side:       "TEAM",
			Line:       new(3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.0),
			Price:      -115,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "SPREAD",
			Team:       "MIA",
			Side:       "TEAM",
			Line:       new(3.0),
			Price:      -105,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TOTAL",
			Side:       "OVER",
			Line:       new(47.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "TOTAL",
			Side:       "OVER",
			Line:       new(47.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TOTAL",
			Side:       "UNDER",
			Line:       new(47.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "TOTAL",
			Side:       "UNDER",
			Line:       new(47.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(263.5),
			Price:      -115,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(265.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(263.5),
			Price:      -105,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	betURL := "/research/bet?" + url.Values{
		"game_id":     {game.ID},
		"market":      {"PASS_YDS"},
		"side":        {"OVER"},
		"team":        {"BUF"},
		"player_name": {"Josh Allen"},
	}.Encode()

	// DraftKings viewer sees DK first on explorer cards and bet detail.
	dkUser, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("draftkings"))
	require.NoError(t, err)
	dkHandler := researchHandlerWithRefresher(t, fixture, dkUser, &researchTestRefresher{})

	exploreDK := httptest.NewRecorder()
	dkHandler.ServeHTTP(exploreDK, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research", nil))
	require.Equal(t, http.StatusOK, exploreDK.Code)
	exploreDKBody := exploreDK.Body.String()
	assert.Contains(t, exploreDKBody, "DK 265.5 (-110) · FD 263.5 (-115)")
	assert.Contains(t, exploreDKBody, "DK -3.5 (-110) · FD -3 (-115)")
	assert.NotContains(t, exploreDKBody, "DK 47.5")

	betDK := httptest.NewRecorder()
	dkHandler.ServeHTTP(betDK, httptest.NewRequestWithContext(t.Context(), http.MethodGet, betURL, nil))
	require.Equal(t, http.StatusOK, betDK.Code)
	betDKBody := betDK.Body.String()
	assert.Contains(t, betDKBody, "DK 265.5 (-110) · FD 263.5 (-115)")

	// FanDuel viewer sees FD first on explorer cards and bet detail.
	fdUser, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("fanduel"))
	require.NoError(t, err)
	fdHandler := researchHandlerWithRefresher(t, fixture, fdUser, &researchTestRefresher{})

	exploreFD := httptest.NewRecorder()
	fdHandler.ServeHTTP(exploreFD, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research", nil))
	require.Equal(t, http.StatusOK, exploreFD.Code)
	exploreFDBody := exploreFD.Body.String()
	assert.Contains(t, exploreFDBody, "FD 263.5 (-115) · DK 265.5 (-110)")
	assert.Contains(t, exploreFDBody, "FD -3 (-115) · DK -3.5 (-110)")

	betFD := httptest.NewRecorder()
	fdHandler.ServeHTTP(betFD, httptest.NewRequestWithContext(t.Context(), http.MethodGet, betURL, nil))
	require.Equal(t, http.StatusOK, betFD.Code)
	betFDBody := betFD.Body.String()
	assert.Contains(t, betFDBody, "FD 263.5 (-115) · DK 265.5 (-110)")
}

func TestResearchBetVisualHierarchyAndFDDKSnapshot(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedBetResearchPlayer(t, fixture, game)

	betURL := "/research/bet?" + url.Values{
		"game_id":   {game.ID},
		"market":    {"PASS_YDS"},
		"side":      {"OVER"},
		"team":      {"BUF"},
		"player_id": {"3918298"},
	}.Encode()

	// DraftKings homeBook: DK card appears before FD card in the snapshot.
	dkUser, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("draftkings"))
	require.NoError(t, err)
	dkHandler := researchHandlerWithRefresher(t, fixture, dkUser, &researchTestRefresher{})

	recDK := httptest.NewRecorder()
	dkHandler.ServeHTTP(recDK, httptest.NewRequestWithContext(t.Context(), http.MethodGet, betURL, nil))
	require.Equal(t, http.StatusOK, recDK.Code)
	bodyDK := recDK.Body.String()

	assert.Contains(t, bodyDK, `data-side-badge="OVER"`)
	assert.Contains(t, bodyDK, "data-research-pillars")
	assert.Contains(t, bodyDK, "Compare all 3 books")

	snapshotIdx := strings.Index(bodyDK, "data-fdk-snapshot")
	pillarsIdx := strings.Index(bodyDK, "data-research-pillars")
	playerIdx := strings.Index(bodyDK, `id="research-panel"`)
	breakdownIdx := strings.Index(bodyDK, "data-grade-breakdown")
	allBooksIdx := strings.Index(bodyDK, "data-all-books")

	require.Greater(t, snapshotIdx, -1)
	require.Greater(t, pillarsIdx, snapshotIdx)
	require.Greater(t, playerIdx, pillarsIdx)
	require.Greater(t, breakdownIdx, playerIdx)
	require.Greater(t, allBooksIdx, breakdownIdx)

	// Collapsed <details> must not start open.
	assert.NotContains(t, bodyDK[allBooksIdx:allBooksIdx+40], " open")

	// Snapshot contains DK before FD, and excludes betmgm.
	snapshotBlock := bodyDK[snapshotIdx:pillarsIdx]
	dkCardIdx := strings.Index(snapshotBlock, `data-snapshot-book="draftkings"`)
	fdCardIdx := strings.Index(snapshotBlock, `data-snapshot-book="fanduel"`)
	require.Greater(t, dkCardIdx, -1)
	require.Greater(t, fdCardIdx, dkCardIdx)
	assert.NotContains(t, snapshotBlock, "betmgm")
	assert.Contains(t, bodyDK[allBooksIdx:], ">betmgm<")

	// FanDuel homeBook: FD card appears before DK card in the snapshot.
	fdUser, err := fixture.store.SetUserHomeBook(t.Context(), fixture.user.ID, new("fanduel"))
	require.NoError(t, err)
	fdHandler := researchHandlerWithRefresher(t, fixture, fdUser, &researchTestRefresher{})

	recFD := httptest.NewRecorder()
	fdHandler.ServeHTTP(recFD, httptest.NewRequestWithContext(t.Context(), http.MethodGet, betURL, nil))
	require.Equal(t, http.StatusOK, recFD.Code)
	bodyFD := recFD.Body.String()

	fdSnapStart := strings.Index(bodyFD, "data-fdk-snapshot")
	fdSnapEnd := strings.Index(bodyFD, "data-research-pillars")
	require.Greater(t, fdSnapStart, -1)
	require.Greater(t, fdSnapEnd, fdSnapStart)
	fdSnapshotBlock := bodyFD[fdSnapStart:fdSnapEnd]
	assert.Less(t,
		strings.Index(fdSnapshotBlock, `data-snapshot-book="fanduel"`),
		strings.Index(fdSnapshotBlock, `data-snapshot-book="draftkings"`),
	)
}

func TestBetResearchHrefSharedAcrossSurfaces(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedResearchExplorer(t, fixture, game)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	slipIDText := strconv.FormatInt(fixture.slip.ID, 10)

	// 1. Pick board options (Lines SPREAD and Passing O/U PASS_YDS).
	pickPath := "/slips/" + slipIDText + "/pick?game=" + game.ID
	pickRec := httptest.NewRecorder()
	handler.ServeHTTP(pickRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, pickPath, nil))
	require.Equal(t, http.StatusOK, pickRec.Code)
	pickBody := pickRec.Body.String()

	expectedSpread := "/research/bet?" + url.Values{
		"from":    {pickPath},
		"game_id": {game.ID},
		"market":  {"SPREAD"},
		"side":    {"TEAM"},
		"slip_id": {slipIDText},
		"team":    {"BUF"},
	}.Encode()
	assert.Contains(t, pickBody, strings.ReplaceAll(expectedSpread, "&", "&amp;"))
	assert.Contains(t, pickBody, "Research &rarr;")

	passingRec := httptest.NewRecorder()
	handler.ServeHTTP(
		passingRec,
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, pickPath+"&cat=passing", nil),
	)
	require.Equal(t, http.StatusOK, passingRec.Code)
	passingBody := passingRec.Body.String()

	expectedPassOver := "/research/bet?" + url.Values{
		"from":        {pickPath},
		"game_id":     {game.ID},
		"market":      {"PASS_YDS"},
		"player_id":   {"3918298"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
		"slip_id":     {slipIDText},
		"team":        {"BUF"},
	}.Encode()
	assert.Contains(
		t,
		passingBody,
		strings.ReplaceAll(strings.ReplaceAll(expectedPassOver, "&", "&amp;"), "+", "&#43;"),
	)
	assert.Contains(t, passingBody, "Research &rarr;")

	// 2. Slip highlights strip (single legs and parlay legs).
	slipPath := "/slips/" + slipIDText
	slipRec := httptest.NewRecorder()
	handler.ServeHTTP(slipRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, slipPath, nil))
	require.Equal(t, http.StatusOK, slipRec.Code)
	slipBody := slipRec.Body.String()

	if idx := strings.Index(slipBody, "data-highlights"); idx != -1 {
		highlightsBlock := slipBody[idx:]
		assert.Contains(t, highlightsBlock, "/research/bet?")
		assert.Contains(t, highlightsBlock, "slip_id="+slipIDText)
		assert.Contains(t, highlightsBlock, "Research &rarr;")
	}

	// 3. Related bets on /research/bet preserve slip_id and include Research -> link.
	betURL := "/research/bet?" + url.Values{
		"from":        {slipPath},
		"game_id":     {game.ID},
		"market":      {"PASS_YDS"},
		"player_id":   {"3918298"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
		"slip_id":     {slipIDText},
	}.Encode()
	betRec := httptest.NewRecorder()
	handler.ServeHTTP(betRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, betURL, nil))
	require.Equal(t, http.StatusOK, betRec.Code)
	betBody := betRec.Body.String()

	relatedIdx := strings.Index(betBody, "More in this game")
	require.Greater(t, relatedIdx, -1)
	relatedBlock := betBody[relatedIdx:]
	assert.Contains(t, relatedBlock, "/research/bet?")
	assert.Contains(t, relatedBlock, "slip_id="+slipIDText)
	assert.Contains(t, relatedBlock, "Research &rarr;")
}

func TestBetAddSheetListsEligibleSlipsAndEmptyState(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedResearchExplorer(t, fixture, game)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	// 1. Missing or invalid params return 404.
	for _, path := range []string{
		"/bets/add-sheet",
		"/bets/add-sheet?game_id=" + game.ID,
		"/bets/add-sheet?game_id=missing&market=SPREAD",
		"/bets/add-sheet?game_id=" + game.ID + "&market=OTHER",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
	}

	// 2. Create additional slips: one out-of-scope (CUSTOM on a different game),
	// one full (locked-in step), and one eligible open slip.
	eligibleSlip, err := fixture.store.CreateSlip(
		t.Context(),
		fixture.group.ID,
		fixture.week.ID,
		nil,
		"Eligible Sunday",
		db.SlateScopeFullWeek,
		2,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)

	otherGame, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:        "401772002",
		WeekID:    fixture.week.ID,
		HomeTeam:  "KC",
		AwayTeam:  "LV",
		Kickoff:   gameKickoff().Add(3 * time.Hour),
		Status:    db.GameScheduled,
		UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	_, err = fixture.store.CreateSlipWithGames(
		t.Context(),
		fixture.group.ID,
		fixture.week.ID,
		nil,
		"Out of Scope",
		db.SlateScopeCustom,
		2,
		[]int64{fixture.user.ID},
		[]string{otherGame.ID},
	)
	require.NoError(t, err)

	fullSlip, err := fixture.store.CreateSlip(
		t.Context(),
		fixture.group.ID,
		fixture.week.ID,
		nil,
		"Full Slip",
		db.SlateScopeFullWeek,
		1,
		[]int64{fixture.user.ID},
	)
	require.NoError(t, err)
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fullSlip.ID,
		UserID:     fixture.user.ID,
		GameID:     &game.ID,
		MarketType: "ML",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Selection:  "BUF ML",
		Odds:       -150,
	})
	require.NoError(t, err)

	sheetURL := "/bets/add-sheet?" + url.Values{
		"game_id":     {game.ID},
		"market":      {"PASS_YDS"},
		"side":        {"OVER"},
		"team":        {"BUF"},
		"player_id":   {"3918298"},
		"player_name": {"Josh Allen"},
		"line":        {"265.5"},
		"odds":        {"-110"},
		"return":      {"/research"},
	}.Encode()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, sheetURL, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, "Add to slip")
	assert.Contains(t, body, "data-close-bet-add-dialog")
	assert.Contains(t, body, fixture.slip.Name)
	assert.Contains(t, body, "Eligible Sunday")
	assert.NotContains(t, body, "Out of Scope")
	assert.NotContains(t, body, "Full Slip")
	assert.Contains(t, body, fmt.Sprintf(`action="/slips/%d/legs"`, eligibleSlip.ID))
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/slips/%d/legs"`, eligibleSlip.ID))
	assert.Contains(t, body, `name="quick" value="1"`)
	assert.Contains(t, body, `name="from" value="research"`)

	// 3. Submitting to Eligible Sunday adds the leg and redirects to /research.
	addRec := postForm(t, handler, fmt.Sprintf("/slips/%d/legs", eligibleSlip.ID), url.Values{
		"quick":       {"1"},
		"from":        {"research"},
		"return":      {"/research"},
		"game_id":     {game.ID},
		"market_type": {"PASS_YDS"},
		"team":        {"BUF"},
		"side":        {"OVER"},
		"player_id":   {"3918298"},
		"player_name": {"Josh Allen"},
		"line":        {"265.5"},
		"odds":        {"-110"},
	})
	assert.Equal(t, http.StatusOK, addRec.Code)
	assert.Equal(t, "/research", addRec.Header().Get("Hx-Redirect"))
	legs, err := fixture.store.LegsForSlip(t.Context(), eligibleSlip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)
	assert.Equal(t, "PASS_YDS", legs[0].MarketType)

	// 4. Empty state when no eligible open slip remains.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), eligibleSlip.ID, db.SlipLocked))

	emptyRec := httptest.NewRecorder()
	handler.ServeHTTP(emptyRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, sheetURL, nil))
	require.Equal(t, http.StatusOK, emptyRec.Code)
	emptyBody := emptyRec.Body.String()
	assert.Contains(t, emptyBody, "No open slips in your pick step have room for this game.")
	assert.Contains(t, emptyBody, fmt.Sprintf(`/slips/new?week=%d`, fixture.week.ID))

	// 5. Kicked-off game shows closed-lines empty state.
	startedGame, err := fixture.store.UpsertGame(t.Context(), db.Game{
		ID:        "401772098",
		WeekID:    fixture.week.ID,
		HomeTeam:  "NYJ",
		AwayTeam:  "KC",
		Kickoff:   time.Now().UTC().Add(-2 * time.Hour),
		Status:    db.GameFinal,
		UpdatedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	startedSheet := "/bets/add-sheet?game_id=" + startedGame.ID + "&market=SPREAD&side=TEAM&team=KC"
	startedRec := httptest.NewRecorder()
	handler.ServeHTTP(startedRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, startedSheet, nil))
	require.Equal(t, http.StatusOK, startedRec.Code)
	assert.Contains(t, startedRec.Body.String(), "This game has kicked off, so its lines are closed.")
}

func TestAddButtonsOnResearchCardsAndRelatedBets(t *testing.T) {
	t.Parallel()

	fixture := newSlipFixture(t, 2)
	game := addGame(t, fixture, gameKickoff())
	seedResearchExplorer(t, fixture, game)
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

	// 1. Research Explorer mounts #bet-add-dialog and wires + Add on cards to /bets/add-sheet.
	resRec := httptest.NewRecorder()
	handler.ServeHTTP(resRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research", nil))
	require.Equal(t, http.StatusOK, resRec.Code)
	resBody := resRec.Body.String()
	assert.Contains(t, resBody, `<dialog id="bet-add-dialog" class="leg-sheet"`)
	assert.Contains(t, resBody, `id="bet-add-dialog-body"`)
	assert.Contains(t, resBody, `hx-get="/bets/add-sheet?`)
	assert.Contains(t, resBody, `hx-target="#bet-add-dialog-body"`)
	assert.Contains(t, resBody, "+ Add")

	// 2. /research/bet without slip_id wires Related bets + Add to /bets/add-sheet.
	baseBetURL := "/research/bet?" + url.Values{
		"game_id":     {game.ID},
		"market":      {"PASS_YDS"},
		"player_id":   {"3918298"},
		"player_name": {"Josh Allen"},
		"side":        {"OVER"},
	}.Encode()
	noSlipRec := httptest.NewRecorder()
	handler.ServeHTTP(noSlipRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, baseBetURL, nil))
	require.Equal(t, http.StatusOK, noSlipRec.Code)
	noSlipBody := noSlipRec.Body.String()
	relatedStart := strings.Index(noSlipBody, "More in this game")
	require.Greater(t, relatedStart, -1)
	relatedNoSlip := noSlipBody[relatedStart:]
	assert.Contains(t, relatedNoSlip, `hx-get="/bets/add-sheet?`)
	assert.Contains(t, relatedNoSlip, `hx-target="#bet-add-dialog-body"`)
	assert.Contains(t, relatedNoSlip, "+ Add")

	// 3. /research/bet with eligible slip_id wires Related bets + Add directly to /slips/{id}/legs.
	slipIDText := strconv.FormatInt(fixture.slip.ID, 10)
	withSlipURL := baseBetURL + "&slip_id=" + slipIDText
	withSlipRec := httptest.NewRecorder()
	handler.ServeHTTP(withSlipRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, withSlipURL, nil))
	require.Equal(t, http.StatusOK, withSlipRec.Code)
	withSlipBody := withSlipRec.Body.String()
	relatedWithSlipStart := strings.Index(withSlipBody, "More in this game")
	require.Greater(t, relatedWithSlipStart, -1)
	relatedWithSlip := withSlipBody[relatedWithSlipStart:]
	assert.Contains(t, relatedWithSlip, fmt.Sprintf(`hx-post="/slips/%d/legs"`, fixture.slip.ID))
	assert.NotContains(t, relatedWithSlip, `hx-get="/bets/add-sheet?`)
	assert.Contains(t, relatedWithSlip, "+ Add")

	// 4. When slip_id is locked (ineligible), Related bets fall back to /bets/add-sheet.
	require.NoError(t, fixture.store.SetSlipStatus(t.Context(), fixture.slip.ID, db.SlipLocked))
	lockedRec := httptest.NewRecorder()
	handler.ServeHTTP(lockedRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, withSlipURL, nil))
	require.Equal(t, http.StatusOK, lockedRec.Code)
	lockedBody := lockedRec.Body.String()
	relatedLockedStart := strings.Index(lockedBody, "More in this game")
	require.Greater(t, relatedLockedStart, -1)
	relatedLocked := lockedBody[relatedLockedStart:]
	assert.Contains(t, relatedLocked, `hx-get="/bets/add-sheet?`)
	assert.Contains(t, relatedLocked, `hx-target="#bet-add-dialog-body"`)
}
