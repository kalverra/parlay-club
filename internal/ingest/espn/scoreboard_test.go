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
	require.Len(t, board.Games, 15, "the Thursday game is on the slate")

	location := eastern(t)
	for _, game := range board.Games {
		day := game.Kickoff.In(location).Weekday()
		assert.Contains(t, []time.Weekday{time.Thursday, time.Sunday, time.Monday}, day, game.ID)
	}

	thursdayID := "401873037"
	assert.Contains(t, gameIDs(board.Games), thursdayID)
	thursday := gameByID(t, board.Games, thursdayID)
	assert.Equal(t, espn.Final, thursday.Status)
	assert.Equal(t, time.Date(2026, time.November, 6, 1, 15, 0, 0, time.UTC), thursday.Kickoff)
	assert.Equal(t, time.Thursday, thursday.Kickoff.In(location).Weekday(), "Friday in UTC is Thursday night in ET")
	assert.Equal(t, "BAL", thursday.Home.Abbreviation)
	require.NotNil(t, thursday.Home.Score)
	assert.Equal(t, 10, *thursday.Home.Score)
	assert.Equal(t, "JAX", thursday.Away.Abbreviation)
	require.NotNil(t, thursday.Away.Score)
	assert.Equal(t, 33, *thursday.Away.Score)
	assert.Equal(t, espn.Venue{ID: "3814", Name: "M&T Bank Stadium", Indoor: false}, thursday.Venue)

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
	require.Len(t, board.Games, 16, "the Thursday game is on the slate")

	counts := map[espn.Status]int{}
	for _, game := range board.Games {
		counts[game.Status]++
	}
	assert.Equal(t, map[espn.Status]int{
		espn.Scheduled: 6,
		espn.Live:      8,
		espn.Final:     2,
	}, counts)

	thursday := gameByID(t, board.Games, "401872964")
	assert.Equal(t, espn.Final, thursday.Status)
	assert.Equal(t, "CLE", thursday.Home.Abbreviation)
	require.NotNil(t, thursday.Home.Score)
	assert.Equal(t, 27, *thursday.Home.Score)
	assert.Equal(t, "PIT", thursday.Away.Abbreviation)
	require.NotNil(t, thursday.Away.Score)
	assert.Equal(t, 24, *thursday.Away.Score)

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

func TestInSlate(t *testing.T) {
	t.Parallel()

	location := eastern(t)
	// 2026-11-05 is Thursday, 11-06 is Friday, 11-07 is Saturday, 11-08 is Sunday,
	// 11-09 is Monday, 11-10 is Tuesday, 11-11 is Wednesday in New York.
	tests := []struct {
		name    string
		kickoff time.Time
		want    bool
	}{
		{name: "Thursday", kickoff: time.Date(2026, time.November, 5, 20, 15, 0, 0, location), want: true},
		{name: "Friday", kickoff: time.Date(2026, time.November, 6, 20, 0, 0, 0, location), want: false},
		{name: "Saturday", kickoff: time.Date(2026, time.November, 7, 13, 0, 0, 0, location), want: false},
		{name: "Sunday", kickoff: time.Date(2026, time.November, 8, 13, 0, 0, 0, location), want: true},
		{name: "Monday", kickoff: time.Date(2026, time.November, 9, 20, 15, 0, 0, location), want: true},
		{name: "Tuesday", kickoff: time.Date(2026, time.November, 10, 20, 0, 0, 0, location), want: false},
		{name: "Wednesday", kickoff: time.Date(2026, time.November, 11, 20, 0, 0, 0, location), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, espn.InSlate(tc.kickoff, location))
		})
	}
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
