package web_test

import (
	"context"
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

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/slips"
	webui "github.com/kalverra/parlay-club/internal/web"
	assets "github.com/kalverra/parlay-club/web"
)

type researchTestRefresher struct {
	gameResearchSynced   []string
	playerResearchSynced []string
	propsSynced          []string
}

func (r *researchTestRefresher) SyncGameProps(_ context.Context, gameID string) error {
	r.propsSynced = append(r.propsSynced, gameID)
	return nil
}

func (r *researchTestRefresher) SyncGameResearch(_ context.Context, gameID string) error {
	r.gameResearchSynced = append(r.gameResearchSynced, gameID)
	return nil
}

func (r *researchTestRefresher) SyncPlayerResearch(_ context.Context, athleteID string) error {
	r.playerResearchSynced = append(r.playerResearchSynced, athleteID)
	return nil
}

func (r *researchTestRefresher) KickPickLines(_ context.Context, _ int64) {}

func (r *researchTestRefresher) KickPickGame(_ context.Context, _ int64, _ string) {}

func (r *researchTestRefresher) ForcePickRefresh(_ context.Context, _ int64, _ string) error {
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
	assert.Contains(t, body, "Player and game research shows up here.")
	assert.Contains(t, body, game.HomeTeam)
	assert.Contains(t, body, "Josh Allen")

	// 2. GET /research?game_id={id} pre-loads game research
	reqGame := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?game_id="+game.ID, nil)
	recGame := httptest.NewRecorder()
	handler.ServeHTTP(recGame, reqGame)
	assert.Equal(t, http.StatusOK, recGame.Code)
	bodyGame := recGame.Body.String()
	assert.Contains(t, bodyGame, game.AwayTeam+" @ "+game.HomeTeam)
	assert.Contains(t, bodyGame, "Key Injuries")
	assert.Contains(t, bodyGame, "Player Matches")
	assert.Contains(t, refresher.gameResearchSynced, game.ID)

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
	assert.Contains(t, refresher.playerResearchSynced, "3918298")

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
	assert.Contains(t, bodyLeg, "Target: <span class=\"font-semibold text-slate-200\">270</span>")

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
	assert.Contains(t, bodyOdds, "Target: <span class=\"font-semibold text-slate-200\">245.5</span>")
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
	game := addGame(t, fixture, gameKickoff())
	refresher := &researchTestRefresher{}
	handler := researchHandlerWithRefresher(t, fixture, fixture.user, refresher)

	// Add player leg with PlayerID
	_, err := fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &game.ID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 265.5 Pass Yds",
		PlayerID:   new("3918298"),
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(265.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	// Add game market leg with GameID only
	_, err = fixture.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     fixture.slip.ID,
		UserID:     fixture.user.ID,
		GameID:     &game.ID,
		MarketType: "SPREAD",
		Selection:  "Bills -3.5",
		Odds:       -110,
	})
	require.NoError(t, err)

	// Render slip
	slipPath := "/slips/" + strconv.FormatInt(fixture.slip.ID, 10)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, slipPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Verify research links on slip legs
	assert.Contains(t, body, "/research?leg_id=1&amp;line=265.5&amp;market=PASS_YDS&amp;player_id=3918298")
	assert.Contains(t, body, "/research?game_id="+game.ID+"&amp;leg_id=2")
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

	t.Run("fresh nflverse surfaces Snap %, Target share, WOPR, Opp Def Rank, and HTMX market swap", func(t *testing.T) {
		t.Parallel()
		fixture := newSlipFixture(t, 2)
		seedResearchNFLverse(t, fixture, time.Now().UTC())
		handler := researchHandlerWithRefresher(t, fixture, fixture.user, &researchTestRefresher{})

		// 1. HTMX partial swap via GET /research?player_id=3918298 (defaults to RUSH_YDS).
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/research?player_id=3918298", nil)
		req.Header.Set("Hx-Request", "true")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		body := rec.Body.String()
		assert.Contains(t, body, "Snap % (L3)")
		assert.Contains(t, body, "72%")
		assert.Contains(t, body, "Target share")
		assert.Contains(t, body, "14.0%")
		assert.Contains(t, body, "WOPR")
		assert.Contains(t, body, "0.38")
		assert.Contains(t, body, "Opp Rush Def")
		assert.Contains(t, body, "#3 vs MIA")
		// Mobile-width 2-column grid layout on phone screens.
		assert.Contains(t, body, "grid-cols-2 gap-2 sm:grid-cols-4")
		// Partial response should not include full page wrapper.
		assert.NotContains(t, body, `id="research-panel"`)

		// 2. HTMX market tab swap to REC_YDS switches opponent rank to Opp Pass Def.
		reqRec := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/research/players/3918298?market=REC_YDS", nil,
		)
		reqRec.Header.Set("Hx-Request", "true")
		recSwap := httptest.NewRecorder()
		handler.ServeHTTP(recSwap, reqRec)
		require.Equal(t, http.StatusOK, recSwap.Code)

		swapBody := recSwap.Body.String()
		assert.Contains(t, swapBody, "Opp Pass Def")
		assert.Contains(t, swapBody, "#2 vs MIA")
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
