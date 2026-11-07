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
		assert.Contains(t, []string{"Sunday", "Monday"}, game.Kickoff.In(eastern(t)).Weekday().String(),
			"live scoreboard game %s is outside the slate", game.ID)
	}
}
