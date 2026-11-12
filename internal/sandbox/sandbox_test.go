package sandbox_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nfelo"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/sandbox"
)

// morning is 8:00 AM EST on the week 9 Sunday. Every outdoor game is inside the
// weather lookahead and before kickoff.
var morning = time.Date(2026, time.November, 8, 13, 0, 0, 0, time.UTC)

func TestWeek9RegistryPreloadsEveryResponse(t *testing.T) {
	t.Parallel()

	fixtures := week9Registry(t).Fixtures()
	require.Len(t, fixtures, 88, "the morning set plus a full response set for each later stage")

	names := make(map[string]bool, len(fixtures))
	byStage := make(map[sandbox.Stage][]sandbox.Fixture, len(sandbox.Stages()))
	captured, derived := 0, 0
	for _, fixture := range fixtures {
		assert.NotEmpty(t, fixture.Name)
		assert.NotEmpty(t, fixture.URL)
		assert.NotEmpty(t, fixture.Note, fixture.Name)
		assert.False(t, names[fixture.Name], "fixture name %q is reused", fixture.Name)
		names[fixture.Name] = true
		byStage[fixture.Stage] = append(byStage[fixture.Stage], fixture)

		switch fixture.Provenance {
		case sandbox.Captured:
			captured++
		case sandbox.Derived:
			derived++
		default:
			t.Errorf("fixture %q has provenance %q", fixture.Name, fixture.Provenance)
		}
	}

	assert.Equal(t, 45, captured,
		"morning captures plus the reused events, lines, projections, Lumen forecast, and research in each stage")
	assert.Equal(t, 43, derived, "scripted scoreboards, the hand-built props board, and derived forecasts")
	for _, stage := range sandbox.Stages() {
		assert.Len(t, byStage[stage], 22, "stage %s has a complete response set", stage)
		scoreboards := 0
		for _, fixture := range byStage[stage] {
			assert.Equal(t, stage, fixture.Stage, fixture.Name)
			if strings.HasSuffix(fixture.URL, "/scoreboard") {
				scoreboards++
			}
		}
		assert.Equal(t, 1, scoreboards, "stage %s answers exactly one scoreboard request", stage)
	}
}

func TestWeek9StageScoreboardsReplayCoherentGames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stage          sandbox.Stage
		wantLive       int
		wantFinal      int
		wantScheduled  int
		featuredStatus espn.Status
		homeScore      int
		awayScore      int
	}{
		{
			stage: sandbox.EarlyLoss, wantLive: 1, wantFinal: 0, wantScheduled: 13,
			featuredStatus: espn.Live, homeScore: 17, awayScore: 10,
		},
		{
			stage: sandbox.AllClear, wantLive: 0, wantFinal: 13, wantScheduled: 1,
			featuredStatus: espn.Final, homeScore: 24, awayScore: 31,
		},
		{
			stage: sandbox.SundayComplete, wantLive: 0, wantFinal: 14, wantScheduled: 0,
			featuredStatus: espn.Final, homeScore: 24, awayScore: 31,
		},
	}

	for _, test := range tests {
		t.Run(string(test.stage), func(t *testing.T) {
			t.Parallel()

			client := replayClientAt(t, test.stage)
			board, err := espn.NewClient("", client).Scoreboard(t.Context(), espn.ScoreboardParams{})
			require.NoError(t, err)
			require.Len(t, board.Games, 14)

			counts := map[espn.Status]int{}
			for _, game := range board.Games {
				counts[game.Status]++
			}
			// The Thursday game is already final in the capture and is dropped
			// from the slate, so final counts here are Sunday and Monday games.
			assert.Equal(t, test.wantLive, counts[espn.Live], "live games")
			assert.Equal(t, test.wantFinal, counts[espn.Final], "final games")
			assert.Equal(t, test.wantScheduled, counts[espn.Scheduled], "scheduled games")

			featured := byID(board.Games, sandbox.FeaturedGameID)
			assert.Equal(t, test.featuredStatus, featured.Status)
			require.NotNil(t, featured.Home.Score)
			require.NotNil(t, featured.Away.Score)
			assert.Equal(t, test.homeScore, *featured.Home.Score)
			assert.Equal(t, test.awayScore, *featured.Away.Score)

			if test.stage == sandbox.SundayComplete {
				for _, game := range board.Games {
					assert.Equal(t, espn.Final, game.Status, game.ID)
					require.NotNil(t, game.Home.Score, game.ID)
					require.NotNil(t, game.Away.Score, game.ID)
				}
			}
			if test.stage == sandbox.AllClear {
				monday := byID(board.Games, "401873051")
				assert.Equal(t, espn.Scheduled, monday.Status,
					"the Monday night game stays upcoming at all-clear")
			}
		})
	}
}

func TestWeek9ScoreboardReplaysThroughESPN(t *testing.T) {
	t.Parallel()

	board := week9Board(t, replayClient(t))

	assert.Equal(t, espn.Season{Year: 2026, Type: 2}, board.Season)
	assert.Equal(t, espn.Week{Number: 9}, board.Week)
	require.Len(t, board.Games, 14, "the Thursday game and any other weekday game are dropped")

	first := board.Games[0]
	assert.Equal(t, "401873038", first.ID)
	assert.Equal(t, "ATL", first.Home.Abbreviation)
	assert.Equal(t, "CIN", first.Away.Abbreviation)
	assert.Equal(t, espn.Scheduled, first.Status)
	assert.Equal(t, "Santiago Bernabéu", first.Venue.Name)

	last := board.Games[len(board.Games)-1]
	assert.Equal(t, "401873051", last.ID)
	assert.Equal(t, "MIN", last.Home.Abbreviation)
	assert.Equal(t, "BUF", last.Away.Abbreviation)
	assert.Equal(t, espn.Scheduled, last.Status)

	for _, game := range board.Games {
		assert.Equal(t, espn.Scheduled, game.Status, game.ID)
	}
}

func TestWeek9LinesReplayThroughParlayAPI(t *testing.T) {
	t.Parallel()

	client := replayClient(t)
	board := week9Board(t, client)
	games := matchGames(board)

	oddsClient := odds.NewClient("", "synthetic-sandbox-key", client)
	events, eventCredits, err := oddsClient.Events(t.Context())
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 620, Used: 0, Last: 0}, eventCredits)

	matches, unmatched := odds.MatchEvents(events, games)
	assert.Len(t, matches, 14)
	assert.NotEmpty(t, unmatched, "week 9's Thursday game and week 10 events do not match")
	assert.Equal(t, sandbox.FeaturedEventID, matches[sandbox.FeaturedGameID])

	lines, credits, err := oddsClient.Lines(t.Context())
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 617, Used: 3, Last: 3}, credits)
	assert.Greater(t, credits.Remaining, refresh.OddsLinesFloor)
	require.NotEmpty(t, lines)

	byEvent := make(map[string]espn.Game, len(matches))
	for gameID, eventID := range matches {
		byEvent[eventID] = byID(board.Games, gameID)
	}

	seen := make(map[string]int, len(matches))
	for _, line := range lines {
		game, ok := byEvent[line.EventID]
		if !ok {
			continue // another week's event
		}
		seen[line.EventID]++
		assert.NotZero(t, line.Price)

		switch line.MarketType {
		case odds.MarketML, odds.MarketSpread:
			assert.Contains(t, []string{game.Home.Name, game.Away.Name}, line.Team)
		case odds.MarketTotal:
			assert.Empty(t, line.Team)
			assert.NotNil(t, line.Line)
		default:
			t.Errorf("event %s has unexpected market %q", line.EventID, line.MarketType)
		}
	}
	assert.Len(t, seen, 14, "every slate game has at least one priced line")
}

func TestWeek9WeatherReplaysEveryMorningForecast(t *testing.T) {
	t.Parallel()

	client := replayClient(t)
	board := week9Board(t, client)
	weatherClient := weather.NewClient("", client)

	forecasts := make(map[string]weather.Forecast, len(board.Games))
	for _, game := range board.Games {
		if game.Venue.Indoor || !morning.Before(game.Kickoff) || morning.Before(game.Kickoff.Add(-48*time.Hour)) {
			continue
		}
		venue, ok := weather.VenueByID(game.Venue.ID)
		require.True(t, ok, "venue %s for game %s", game.Venue.ID, game.ID)

		forecast, err := weatherClient.Forecast(t.Context(), venue, game.Kickoff)
		require.NoError(t, err, "game %s", game.ID)
		forecasts[game.ID] = forecast
	}

	require.Len(t, forecasts, 10, "every outdoor Sunday and Monday game has a forecast")
	assert.Equal(t, weather.Forecast{
		TempF:       53.6,
		WindMPH:     11.3,
		WindGustMPH: 19.5,
		PrecipIN:    0.028,
	}, forecasts["401873048"], "the captured Lumen Field forecast")
	assert.Equal(t, weather.Forecast{
		TempF:       58.2,
		WindMPH:     11.2,
		WindGustMPH: 19.5,
	}, forecasts["401873045"], "the derived Arrowhead forecast")
}

func TestWeek9ProjectionsReplayThroughNfelo(t *testing.T) {
	t.Parallel()

	client := replayClient(t)
	board := week9Board(t, client)

	projections, err := nfelo.NewClient("", client).Projections(t.Context())
	require.NoError(t, err)

	type projectionKey struct {
		home string
		away string
	}
	week9 := make(map[projectionKey]nfelo.Projection)
	for _, projection := range projections {
		if projection.Season != 2026 || projection.Week != 9 {
			continue
		}
		week9[projectionKey{home: projection.Home, away: projection.Away}] = projection
	}
	require.Len(t, week9, 15, "the CSV's whole week 9 is readable, including the Thursday game")

	for _, game := range board.Games {
		key := projectionKey{home: game.Home.Abbreviation, away: game.Away.Abbreviation}
		assert.Contains(t, week9, key, game.ID)
	}
}

func TestWeek9FeaturedPropsReplayThroughParlayAPI(t *testing.T) {
	t.Parallel()

	props, credits, err := odds.NewClient("", "synthetic-sandbox-key", replayClient(t)).
		Props(t.Context(), sandbox.FeaturedEventID)
	require.NoError(t, err)
	assert.Equal(t, odds.Credits{Remaining: 614, Used: 6, Last: 3}, credits)
	assert.Greater(t, credits.Remaining, refresh.OddsPropsFloor)
	require.NotEmpty(t, props)

	markets := make(map[string]bool)
	for _, prop := range props {
		assert.Equal(t, sandbox.FeaturedEventID, prop.EventID)
		assert.NotEmpty(t, prop.Book)
		assert.NotEmpty(t, prop.MarketType)
		assert.NotEmpty(t, prop.Side)
		assert.NotEmpty(t, prop.PlayerName+prop.Team)
		markets[prop.MarketType] = true
	}
	assert.Contains(t, markets, odds.MarketPassYds)
	assert.Contains(t, markets, odds.MarketTeamTotal)
}

func TestWeek9FeaturedResearchReplaysRostersSummaryAndLogs(t *testing.T) {
	t.Parallel()

	client := espn.NewClient("", replayClient(t))

	rosters := []struct{ teamID, abbreviation, player string }{
		{teamID: sandbox.FeaturedHomeESPNID, abbreviation: "ATL", player: "Matthew Bergeron"},
		{teamID: sandbox.FeaturedAwayESPNID, abbreviation: "CIN", player: "Joe Burrow"},
	}
	for _, team := range rosters {
		roster, err := client.Roster(t.Context(), team.teamID)
		require.NoError(t, err, team.abbreviation)
		assert.NotEmpty(t, roster, team.abbreviation)

		names := make([]string, 0, len(roster))
		for _, player := range roster {
			names = append(names, player.FullName)
		}
		assert.Contains(t, names, team.player, team.abbreviation)
	}

	// The props sync reads both rosters before it matches player names, so the
	// featured game's away roster has to resolve the prop board's players.
	rawInjuries, err := client.SummaryInjuries(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	teams, err := espn.ParseInjuries(rawInjuries)
	require.NoError(t, err)
	require.Len(t, teams, 2, "the summary reports both teams")
	reported := 0
	for _, team := range teams {
		reported += len(team.Injuries)
	}
	assert.Positive(t, reported, "the research page has key-position injuries to show")

	logs := []struct{ name, athleteID string }{
		{name: "Joe Burrow", athleteID: "3915511"},
		{name: "Ja'Marr Chase", athleteID: "4362628"},
		{name: "Tee Higgins", athleteID: "4239993"},
		{name: "Evan McPherson", athleteID: "4360234"},
	}
	for _, log := range logs {
		got, err := client.GameLog(t.Context(), log.athleteID)
		require.NoError(t, err, log.name)
		assert.Equal(t, log.athleteID, got.AthleteID)
		assert.Positive(t, got.GamesPlayed, log.name)
		assert.NotEmpty(t, got.Markets, log.name)
	}
}

func TestWeek9EmptyFallbacksAnswerUncapturedResearch(t *testing.T) {
	t.Parallel()

	client := replayClient(t)

	props, _, err := odds.NewClient("", "synthetic-sandbox-key", client).
		Props(t.Context(), "44d8c37868b700e90f816db7c5df5cdd")
	require.NoError(t, err)
	assert.Empty(t, props, "a game without captured props shows an empty board")

	espnClient := espn.NewClient("", client)
	roster, err := espnClient.Roster(t.Context(), "15")
	require.NoError(t, err)
	assert.Empty(t, roster, "a team without a captured roster shows no players")

	rawInjuries, err := espnClient.SummaryInjuries(t.Context(), "401873042")
	require.NoError(t, err)
	assert.Equal(t, "[]", rawInjuries, "a game without a captured summary shows no injuries")

	log, err := espnClient.GameLog(t.Context(), "9999999")
	require.NoError(t, err)
	assert.Zero(t, log.GamesPlayed, "an athlete without a captured log shows no games")

	// The transport labels the synthesized body so a debugging session can
	// tell it apart from a capture.
	transport := sandbox.NewTransport(week9Registry(t), sandbox.Morning)
	req, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, odds.DefaultBaseURL+"/props?eventId=another-game", nil,
	)
	require.NoError(t, err)
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, sandbox.FallbackEmpty, resp.Header.Get(sandbox.FallbackHeader))
}

func TestReplayTransportFailsClosed(t *testing.T) {
	t.Parallel()

	client := replayClient(t)

	_, err := espn.NewClient("", client).Scoreboard(t.Context(), espn.ScoreboardParams{
		Season:     2026,
		SeasonType: 2,
		Week:       10,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, sandbox.ErrNoResponse)
	assert.Contains(t, err.Error(), "week=10")
	assert.Contains(t, err.Error(), `stage "morning"`)

	// The Odds API fallback is not registered. A request to it must fail rather
	// than reach the network or invent a second provider.
	_, _, err = odds.NewClient(odds.TheOddsAPIBaseURL, "synthetic-sandbox-key", client).Events(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, sandbox.ErrNoResponse)
	assert.Contains(t, err.Error(), "api.the-odds-api.com")
}

func TestReplayTransportIgnoresCredentials(t *testing.T) {
	t.Parallel()

	transport := sandbox.NewTransport(week9Registry(t), sandbox.Morning)
	target := odds.DefaultBaseURL + "/events?apiKey=real-looking-provider-key"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Atlanta Falcons")

	transport.SetStage("playoffs")
	unknown, err := transport.RoundTrip(req)
	require.Error(t, err)
	require.ErrorIs(t, err, sandbox.ErrNoResponse)
	assert.Contains(t, err.Error(), `stage "playoffs"`)
	if unknown != nil {
		require.NoError(t, unknown.Body.Close())
	}
}

func TestRegistryRejectsMissingFixture(t *testing.T) {
	t.Parallel()

	_, err := sandbox.NewRegistry(fstest.MapFS{}, []sandbox.Fixture{{
		Name:       "missing-response",
		Stage:      sandbox.Morning,
		Method:     http.MethodGet,
		URL:        "https://example.test/scoreboard",
		File:       "does-not-exist.json",
		Provenance: sandbox.Captured,
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing-response")
	assert.Contains(t, err.Error(), "does-not-exist.json")
}

func TestRegistryRejectsDuplicateResponse(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{"saved.json": {Data: []byte(`{"ok":true}`)}}
	fixture := sandbox.Fixture{
		Stage:      sandbox.Morning,
		Method:     http.MethodGet,
		URL:        "https://example.test/scoreboard",
		File:       "saved.json",
		Provenance: sandbox.Captured,
		Note:       "A test fixture.",
	}
	first := fixture
	first.Name = "first"
	second := fixture
	second.Name = "second"

	_, err := sandbox.NewRegistry(files, []sandbox.Fixture{first, second})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "second")
	assert.Contains(t, err.Error(), "another response")
}

func TestRegistryRejectsBadFallbackPattern(t *testing.T) {
	t.Parallel()

	files := fstest.MapFS{"empty.json": {Data: []byte(`[]`)}}
	fixture := sandbox.Fixture{
		Name:       "saved",
		Stage:      sandbox.Morning,
		Method:     http.MethodGet,
		URL:        "https://example.test/scoreboard",
		File:       "empty.json",
		Provenance: sandbox.Captured,
		Note:       "A test fixture.",
	}

	_, err := sandbox.NewRegistryWithFallbacks(files, []sandbox.Fixture{fixture}, []sandbox.Fallback{{
		Name:   "bad-pattern",
		Stage:  sandbox.Morning,
		Method: http.MethodGet,
		URL:    "https://example.test/teams/[bad/roster",
		File:   "empty.json",
		Note:   "A test fallback.",
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a match pattern")
}

// week9Registry preloads the real week 9 registry from the repository root.
func week9Registry(t *testing.T) *sandbox.Registry {
	t.Helper()

	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	registry, err := sandbox.Week9(os.DirFS(root))
	require.NoError(t, err)
	return registry
}

// replayClient builds an ordinary HTTP client whose transport is the replay.
func replayClient(t *testing.T) *http.Client {
	t.Helper()

	return replayClientAt(t, sandbox.Morning)
}

// replayClientAt builds one HTTP client replaying a named stage.
func replayClientAt(t *testing.T, stage sandbox.Stage) *http.Client {
	t.Helper()

	return &http.Client{Transport: sandbox.NewTransport(week9Registry(t), stage)}
}

// week9Board reads the replayed ESPN slate.
func week9Board(t *testing.T, client *http.Client) espn.Scoreboard {
	t.Helper()

	board, err := espn.NewClient("", client).Scoreboard(t.Context(), espn.ScoreboardParams{})
	require.NoError(t, err)
	return board
}

// matchGames adapts the ESPN slate to the odds matcher's shape.
func matchGames(board espn.Scoreboard) []odds.MatchGame {
	games := make([]odds.MatchGame, 0, len(board.Games))
	for _, game := range board.Games {
		games = append(games, odds.MatchGame{
			ID:       game.ID,
			HomeName: game.Home.Name,
			AwayName: game.Away.Name,
			Kickoff:  game.Kickoff,
		})
	}
	return games
}

// byID finds one game by ID. The caller only asks for IDs that came from the
// same slate, so a miss is a test bug and returns the zero game.
func byID(games []espn.Game, id string) espn.Game {
	for _, game := range games {
		if game.ID == id {
			return game
		}
	}
	return espn.Game{}
}
