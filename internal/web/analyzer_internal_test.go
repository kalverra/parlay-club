package web

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
)

func TestPostseasonNFLverseMergesRegularAndPostseason(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	now := time.Date(2027, 1, 16, 12, 0, 0, 0, time.UTC)
	require.NoError(t, store.RecordSync(t.Context(), nflverseSource(2026), now, nil))

	// Seed regular-season rows for all 32 teams, plus 1 postseason week for 2 teams.
	teamRows := make([]db.NFLverseTeamWeek, 0, 34)
	for i := range 32 {
		team := fmt.Sprintf("T%02d", i+1)
		teamRows = append(teamRows, db.NFLverseTeamWeek{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypeRegular,
			WeekNum:           18,
			Team:              team,
			OffPassEPA:        float64(i) * 0.01,
			DefPassEPAAllowed: float64(i) * 0.01,
			PlaysPerGame:      60 + float64(i)*0.2,
			PassRate:          0.50 + float64(i)*0.005,
			UpdatedAt:         now,
		})
	}
	teamRows = append(teamRows,
		db.NFLverseTeamWeek{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypePostseason,
			WeekNum:           1,
			Team:              "T01",
			DefPassEPAAllowed: -0.05,
			UpdatedAt:         now,
		},
		db.NFLverseTeamWeek{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypePostseason,
			WeekNum:           1,
			Team:              "T32",
			DefPassEPAAllowed: 0.40,
			UpdatedAt:         now,
		},
	)
	_, err = store.UpsertNFLverseTeamWeeks(t.Context(), teamRows)
	require.NoError(t, err)

	postWeek := db.Week{
		ID:         db.WeekID(2026, nflverse.SeasonTypePostseason, 2),
		Season:     2026,
		SeasonType: nflverse.SeasonTypePostseason,
		WeekNum:    2,
	}
	profiles, fresh := loadFreshNFLverseTeamProfiles(t.Context(), store, now, postWeek)
	require.True(t, fresh)
	require.Len(t, profiles, 32)

	seenRanks := make(map[int]bool, 32)
	for _, prof := range profiles {
		seenRanks[prof.DefPassEPAAllowedRank] = true
	}
	for r := 1; r <= 32; r++ {
		assert.True(t, seenRanks[r], "expected rank %d to be present across 32 teams", r)
	}

	// Player usage in postseason merges regular-season and postseason weeks,
	// with postseason weeks ordered first for L3/L2 recency.
	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 16, Team: "T01", OffensePct: 0.80, TargetShare: 0.20, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 17, Team: "T01", OffensePct: 0.80, TargetShare: 0.20, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 18, Team: "T01", OffensePct: 0.80, TargetShare: 0.20, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypePostseason,
			WeekNum: 1, Team: "T01", OffensePct: 0.95, TargetShare: 0.35, UpdatedAt: now,
		},
	})
	require.NoError(t, err)

	usage := nflverseUsageForPlayers(t.Context(), store, []string{"p1"}, 2026, nflverse.SeasonTypePostseason)
	require.NotNil(t, usage["p1"])
	require.Len(t, usage["p1"].Weeks, 4)
	assert.InDelta(t, 0.95, usage["p1"].Weeks[0].OffensePct, 1e-9, "postseason week should be most recent")
	assert.InDelta(t, 0.85, usage["p1"].Last3SnapAvg, 1e-9)
}
