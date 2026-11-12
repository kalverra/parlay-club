package odds_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

func TestMatchEventsFindsEveryGameInTheSavedSlate(t *testing.T) {
	t.Parallel()

	board := readScoreboardFixture(t, "scoreboard_week.json")
	require.Len(t, board.Games, 15)

	eventsServer := fixtureServer(t, "events_nfl.json", "/events",
		odds.Credits{Remaining: 500}, apiKeyQuery())
	events, _, err := odds.NewClient(eventsServer.URL, testAPIKey, eventsServer.Client()).Events(t.Context())
	require.NoError(t, err)
	require.Len(t, events, 27)

	matches, unmatched := odds.MatchEvents(events, matchGames(board.Games))

	require.Len(t, matches, len(board.Games)-1, "every upcoming slate game matches an event (Thursday already played)")
	for _, game := range board.Games {
		if game.ID == "401873037" {
			assert.Empty(t, matches[game.ID], "Thursday game was already played")
			continue
		}
		assert.NotEmpty(t, matches[game.ID], "game %s %s vs %s has no event",
			game.ID, game.Away.Name, game.Home.Name)
	}

	assert.Equal(t, "dc56d980f527ffeef89940133b211b13", matches["401873038"], "the Madrid game")
	assert.Equal(t, "17ad7eff2f7875e20a8109abe40470e3", matches["401873051"], "the Monday night game")

	assert.Len(t, unmatched, 13, "the next week's events have no game on this slate")
	for _, event := range unmatched {
		assert.False(t, event.CommenceTime.Before(time.Date(2026, time.November, 9, 2, 0, 0, 0, time.UTC)),
			"unmatched event %s is outside the saved week", event.ID)
	}
}

func TestMatchEventsNeedsTheSameTeamsAndASimilarKickoff(t *testing.T) {
	t.Parallel()

	kickoff := time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC)
	event := odds.Event{
		ID:           "evt-1",
		CommenceTime: kickoff,
		HomeTeam:     "Buffalo Bills",
		AwayTeam:     "New York Jets",
	}
	game := odds.MatchGame{
		ID:       "401873000",
		HomeName: "Buffalo Bills",
		AwayName: "New York Jets",
		Kickoff:  kickoff,
	}

	tests := map[string]struct {
		event odds.Event
		match bool
	}{
		"same teams and kickoff": {
			event: event,
			match: true,
		},
		"kickoff moves a few hours": {
			event: withCommence(event, kickoff.Add(5*time.Hour)),
			match: true,
		},
		"kickoff moves exactly the window": {
			event: withCommence(event, kickoff.Add(odds.MatchWindow)),
			match: true,
		},
		"kickoff moves past the window": {
			event: withCommence(event, kickoff.Add(odds.MatchWindow+time.Minute)),
			match: false,
		},
		"team names differ": {
			event: withNames(event, "Buffalo", "New York Jets"),
			match: false,
		},
		"sides are swapped": {
			event: withNames(event, "New York Jets", "Buffalo Bills"),
			match: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			matches, unmatched := odds.MatchEvents([]odds.Event{test.event}, []odds.MatchGame{game})
			if test.match {
				assert.Equal(t, map[string]string{game.ID: event.ID}, matches)
				assert.Empty(t, unmatched)
				return
			}
			assert.Empty(t, matches)
			assert.Equal(t, []odds.Event{test.event}, unmatched)
		})
	}
}

// matchGames adapts ESPN games to the shape odds matching takes.
func matchGames(games []espn.Game) []odds.MatchGame {
	adapted := make([]odds.MatchGame, 0, len(games))
	for _, game := range games {
		adapted = append(adapted, odds.MatchGame{
			ID:       game.ID,
			HomeName: game.Home.Name,
			AwayName: game.Away.Name,
			Kickoff:  game.Kickoff,
		})
	}
	return adapted
}

// readScoreboardFixture serves the saved scoreboard and returns the parsed week.
func readScoreboardFixture(t *testing.T, fixture string) espn.Scoreboard {
	t.Helper()

	//nolint:gosec // the fixture name is a test constant.
	raw, err := os.ReadFile(filepath.Join("..", "espn", "testdata", fixture))
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	board, err := espn.NewClient(server.URL, server.Client()).Scoreboard(t.Context(), espn.ScoreboardParams{
		Season:     2026,
		SeasonType: 2,
		Week:       9,
	})
	require.NoError(t, err)
	return board
}

// withCommence returns event with a different commence time.
func withCommence(event odds.Event, commence time.Time) odds.Event {
	event.CommenceTime = commence
	return event
}

// withNames returns event with different team names.
func withNames(event odds.Event, home, away string) odds.Event {
	event.HomeTeam = home
	event.AwayTeam = away
	return event
}
