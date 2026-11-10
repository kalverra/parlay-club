package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestCurrentOddsKeepsBothSyncGroups(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 11, 7, 14, 0, 0, 0, time.UTC)

	rows := []db.Odds{
		// Stale game line
		{MarketType: "ML", FetchedAt: t1},
		// Current game line
		{MarketType: "ML", FetchedAt: t2},
		// Stale prop
		{MarketType: "PASS_YDS", PlayerName: "Josh Allen", FetchedAt: t1},
		// Current prop (newer than game line)
		{MarketType: "PASS_YDS", PlayerName: "Josh Allen", FetchedAt: t3},
		// Current team total prop
		{MarketType: "TEAM_TOTAL", FetchedAt: t3},
	}

	result := currentOdds(rows)
	require.Len(t, result, 3)
	assert.Equal(t, t2, result[0].FetchedAt)
	assert.Equal(t, "ML", result[0].MarketType)
	assert.Equal(t, t3, result[1].FetchedAt)
	assert.Equal(t, "PASS_YDS", result[1].MarketType)
	assert.Equal(t, t3, result[2].FetchedAt)
	assert.Equal(t, "TEAM_TOTAL", result[2].MarketType)
}
