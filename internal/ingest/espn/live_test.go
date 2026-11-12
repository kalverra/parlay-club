package espn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

// TestLiveScoreboard hits ESPN's real scoreboard and checks that the parser
// still recognizes the response. It skips in -short runs and in the offseason,
// when there is no current slate to check.
func TestLiveScoreboard(t *testing.T) {
	if testing.Short() {
		t.Skip("live API test")
	}
	t.Parallel()

	board, err := espn.NewClient("", nil).Scoreboard(t.Context(), espn.ScoreboardParams{})
	require.NoError(t, err)

	if len(board.Games) == 0 {
		t.Skip("no games on the current scoreboard")
	}

	for _, game := range board.Games {
		assert.NotEmpty(t, game.ID)
		assert.False(t, game.Kickoff.IsZero())
		assert.NotEmpty(t, game.Home.ID)
		assert.NotEmpty(t, game.Away.ID)
		assert.NotEmpty(t, game.Home.Name)
		assert.NotEmpty(t, game.Away.Name)
		assert.Contains(t, []espn.Status{espn.Scheduled, espn.Live, espn.Final}, game.Status)
		assert.Contains(t, []string{"Thursday", "Sunday", "Monday"}, game.Kickoff.In(eastern(t)).Weekday().String(),
			"live scoreboard game %s is outside the slate", game.ID)
	}
}

// TestLiveRoster hits ESPN's real roster endpoint for the Buffalo Bills (team 2).
func TestLiveRoster(t *testing.T) {
	if testing.Short() {
		t.Skip("live API test")
	}
	t.Parallel()

	players, err := espn.NewClient("", nil).Roster(t.Context(), "2")
	require.NoError(t, err)
	assert.NotEmpty(t, players)

	for _, p := range players {
		assert.NotEmpty(t, p.ID)
		assert.NotEmpty(t, p.FullName)
	}
}

// TestLiveSummaryInjuries hits ESPN's live summary endpoint for a game and parses injuries.
func TestLiveSummaryInjuries(t *testing.T) {
	if testing.Short() {
		t.Skip("live API test")
	}
	t.Parallel()

	client := espn.NewClient("", nil)
	board, err := client.Scoreboard(t.Context(), espn.ScoreboardParams{})
	require.NoError(t, err)
	if len(board.Games) == 0 {
		t.Skip("no games on current scoreboard")
	}

	gameID := board.Games[0].ID
	raw, err := client.SummaryInjuries(t.Context(), gameID)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	teams, err := espn.ParseInjuries(raw)
	require.NoError(t, err)
	_ = teams
}

// TestLiveGameLog hits ESPN's live gamelog endpoint for Josh Allen (3918298).
func TestLiveGameLog(t *testing.T) {
	if testing.Short() {
		t.Skip("live API test")
	}
	t.Parallel()

	log, err := espn.NewClient("", nil).GameLog(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, "3918298", log.AthleteID)
	assert.GreaterOrEqual(t, log.GamesPlayed, 1)
	assert.Contains(t, log.Markets, "PASS_YDS")
	assert.Len(t, log.Markets["PASS_YDS"], log.GamesPlayed)
}
