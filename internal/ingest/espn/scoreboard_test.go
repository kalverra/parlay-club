package espn_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

func TestScoreboardParsesAFullWeek(t *testing.T) {
	t.Parallel()

	board := serveScoreboard(t, "scoreboard_week.json", espn.ScoreboardParams{
		Season:     2026,
		SeasonType: 2,
		Week:       9,
	}, weekQuery(9))

	assert.Equal(t, espn.Season{Year: 2026, Type: 2}, board.Season)
	assert.Equal(t, espn.Week{Number: 9}, board.Week)
	require.Len(t, board.Games, 14, "the Thursday game is outside the slate")

	location := eastern(t)
	for _, game := range board.Games {
		day := game.Kickoff.In(location).Weekday()
		assert.Contains(t, []time.Weekday{time.Sunday, time.Monday}, day, game.ID)
	}

	thursday := "401873037"
	assert.NotContains(t, gameIDs(board.Games), thursday)

	madrid := gameByID(t, board.Games, "401873038")
	assert.Equal(t, espn.Scheduled, madrid.Status)
	assert.Equal(t, time.Date(2026, time.November, 8, 14, 30, 0, 0, time.UTC), madrid.Kickoff)
	assert.Equal(t, espn.Team{
		ID:           "1",
		Abbreviation: "ATL",
		Name:         "Atlanta Falcons",
	}, madrid.Home)
	assert.Equal(t, espn.Team{
		ID:           "4",
		Abbreviation: "CIN",
		Name:         "Cincinnati Bengals",
	}, madrid.Away)
	assert.Equal(t, espn.Venue{ID: "1353", Name: "Santiago Bernabéu", Indoor: true}, madrid.Venue)

	monday := gameByID(t, board.Games, "401873051")
	assert.Equal(t, time.Date(2026, time.November, 10, 1, 15, 0, 0, time.UTC), monday.Kickoff)
	assert.Equal(t, time.Monday, monday.Kickoff.In(location).Weekday(), "Tuesday in UTC is Monday night in ET")
	assert.Equal(t, "MIN", monday.Home.Abbreviation)
	assert.Equal(t, "BUF", monday.Away.Abbreviation)
}

func TestScoreboardParsesLiveAndFinalGames(t *testing.T) {
	t.Parallel()

	board := serveScoreboard(t, "scoreboard_live.json", espn.ScoreboardParams{
		Season:     2026,
		SeasonType: 2,
		Week:       4,
	}, weekQuery(4))

	assert.Equal(t, espn.Season{Year: 2026, Type: 2}, board.Season)
	assert.Equal(t, espn.Week{Number: 4}, board.Week)
	require.Len(t, board.Games, 15, "the Thursday game is outside the slate")

	counts := map[espn.Status]int{}
	for _, game := range board.Games {
		counts[game.Status]++
	}
	assert.Equal(t, map[espn.Status]int{
		espn.Scheduled: 6,
		espn.Live:      8,
		espn.Final:     1,
	}, counts)

	live := gameByID(t, board.Games, "401872971")
	assert.Equal(t, espn.Live, live.Status)
	assert.Equal(t, "BUF", live.Home.Abbreviation)
	require.NotNil(t, live.Home.Score)
	assert.Equal(t, 17, *live.Home.Score)
	assert.Equal(t, "NE", live.Away.Abbreviation)
	require.NotNil(t, live.Away.Score)
	assert.Equal(t, 21, *live.Away.Score)

	final := gameByID(t, board.Games, "401872965")
	assert.Equal(t, espn.Final, final.Status)
	assert.Equal(t, espn.Venue{ID: "5534", Name: "Tottenham Hotspur Stadium"}, final.Venue)
	require.NotNil(t, final.Home.Score)
	assert.Equal(t, 13, *final.Home.Score)
	require.NotNil(t, final.Away.Score)
	assert.Equal(t, 30, *final.Away.Score)

	scheduled := gameByID(t, board.Games, "401872978")
	assert.Equal(t, espn.Scheduled, scheduled.Status)
	assert.Nil(t, scheduled.Home.Score, "a game that has not kicked off has no score")
	assert.Nil(t, scheduled.Away.Score)
}

func TestScoreboardParsesAnInternationalGame(t *testing.T) {
	t.Parallel()

	board := serveScoreboard(t, "scoreboard_international.json", espn.ScoreboardParams{
		Season:     2026,
		SeasonType: 2,
		Week:       5,
	}, weekQuery(5))

	assert.Equal(t, espn.Season{Year: 2026, Type: 2}, board.Season)
	assert.Equal(t, espn.Week{Number: 5}, board.Week)
	require.Len(t, board.Games, 1)

	game := board.Games[0]
	assert.Equal(t, "401872981", game.ID)
	assert.Equal(t, espn.Final, game.Status)
	assert.Equal(t, time.Date(2026, time.October, 11, 13, 30, 0, 0, time.UTC), game.Kickoff)
	assert.Equal(t, time.Sunday, game.Kickoff.In(eastern(t)).Weekday(), "9:30 AM ET kickoff stays on the slate")
	assert.Equal(t, "JAX", game.Home.Abbreviation)
	assert.Equal(t, "PHI", game.Away.Abbreviation)
	assert.Equal(t, espn.Venue{ID: "5534", Name: "Tottenham Hotspur Stadium"}, game.Venue)
	require.NotNil(t, game.Home.Score)
	assert.Equal(t, 19, *game.Home.Score)
	require.NotNil(t, game.Away.Score)
	assert.Equal(t, 17, *game.Away.Score)
}

func TestScoreboardReportsAnHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	_, err := espn.NewClient(server.URL, server.Client()).Scoreboard(t.Context(), espn.ScoreboardParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503 Service Unavailable")
}

// serveScoreboard points a client at fixture and returns the parsed board.
func serveScoreboard(
	t *testing.T,
	fixture string,
	params espn.ScoreboardParams,
	wantQuery url.Values,
) espn.Scoreboard {
	t.Helper()

	//nolint:gosec // the fixture name is a test constant.
	raw, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)

	queries := make(chan url.Values, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/scoreboard", r.URL.Path)
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		queries <- r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	board, err := espn.NewClient(server.URL, server.Client()).Scoreboard(t.Context(), params)
	require.NoError(t, err)
	assert.Equal(t, wantQuery, <-queries)
	return board
}

// weekQuery is the query a named week should put on the wire.
func weekQuery(week int) url.Values {
	query := url.Values{}
	query.Set("dates", "2026")
	query.Set("seasontype", "2")
	query.Set("week", strconv.Itoa(week))
	return query
}

// eastern loads the zone the slate filter uses.
func eastern(t *testing.T) *time.Location {
	t.Helper()

	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	return location
}

// gameByID finds one game or fails the test.
func gameByID(t *testing.T, games []espn.Game, id string) espn.Game {
	t.Helper()

	for _, game := range games {
		if game.ID == id {
			return game
		}
	}
	t.Fatalf("game %s not found", id)
	return espn.Game{}
}

// gameIDs returns the IDs of games in order.
func gameIDs(games []espn.Game) []string {
	ids := make([]string, 0, len(games))
	for _, game := range games {
		ids = append(ids, game.ID)
	}
	return ids
}
