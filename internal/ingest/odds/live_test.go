package odds_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

// TestLiveEvents hits Parlay API's free event list and checks that the parser
// still recognizes the response. It skips in -short runs and without an API
// key, and it never spends credits on game lines.
func TestLiveEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("live API test")
	}

	apiKey := os.Getenv("ODDS_API_KEY")
	if apiKey == "" {
		t.Skip("ODDS_API_KEY is not set")
	}
	t.Parallel()

	events, credits, err := odds.NewClient("", apiKey, nil).Events(t.Context())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, credits.Remaining, 0)

	for _, event := range events {
		assert.NotEmpty(t, event.ID)
		assert.False(t, event.CommenceTime.IsZero())
		assert.NotEmpty(t, event.HomeTeam)
		assert.NotEmpty(t, event.AwayTeam)
		assert.NotEqual(t, event.HomeTeam, event.AwayTeam)
	}
}
