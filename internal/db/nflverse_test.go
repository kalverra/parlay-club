package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestUpsertNFLversePlayerIDsIdempotentAndUpdate(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	second := first.Add(24 * time.Hour)

	row := db.NFLversePlayerID{
		ESPNID:    "4362628",
		GSISID:    "00-0036900",
		PFRID:     "ChasJa00",
		UpdatedAt: first,
	}
	changed, err := store.UpsertNFLversePlayerIDs(t.Context(), []db.NFLversePlayerID{row})
	require.NoError(t, err)
	assert.True(t, changed)

	stored, ok, err := store.NFLversePlayerIDByESPN(t.Context(), "4362628")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, row, stored)

	// Rerunning with identical values at a later timestamp leaves updated_at untouched.
	again := row
	again.UpdatedAt = second
	changed, err = store.UpsertNFLversePlayerIDs(t.Context(), []db.NFLversePlayerID{again})
	require.NoError(t, err)
	assert.False(t, changed)

	stored, ok, err = store.NFLversePlayerIDByESPN(t.Context(), "4362628")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, first, stored.UpdatedAt)

	// Changing pfr_id updates the row and bumps updated_at.
	again.PFRID = "ChasJa01"
	changed, err = store.UpsertNFLversePlayerIDs(t.Context(), []db.NFLversePlayerID{again})
	require.NoError(t, err)
	assert.True(t, changed)

	stored, ok, err = store.NFLversePlayerIDByESPN(t.Context(), "4362628")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, again, stored)

	_, ok, err = store.NFLversePlayerIDByESPN(t.Context(), "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestUpsertNFLverseTeamWeeksIdempotentAndQuery(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	second := first.Add(6 * time.Hour)

	rows := []db.NFLverseTeamWeek{
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           8,
			Team:              "BUF",
			OffPassEPA:        12.4,
			OffRushEPA:        3.1,
			DefPassEPAAllowed: -4.2,
			DefRushEPAAllowed: -1.5,
			PlaysPerGame:      64,
			PassRate:          0.5625,
			UpdatedAt:         first,
		},
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           7,
			Team:              "ATL",
			OffPassEPA:        5.0,
			OffRushEPA:        2.0,
			DefPassEPAAllowed: 1.5,
			DefRushEPAAllowed: -0.5,
			PlaysPerGame:      60,
			PassRate:          0.5,
			UpdatedAt:         first,
		},
	}

	changed, err := store.UpsertNFLverseTeamWeeks(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	got, err := store.NFLverseTeamWeeksForSeason(t.Context(), 2026, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, rows[1], got[0], "ordered by week_num ASC")
	assert.Equal(t, rows[0], got[1])

	// Idempotent rerun does not bump updated_at.
	rows[0].UpdatedAt = second
	rows[1].UpdatedAt = second
	changed, err = store.UpsertNFLverseTeamWeeks(t.Context(), rows)
	require.NoError(t, err)
	assert.False(t, changed)

	got, err = store.NFLverseTeamWeeksForSeason(t.Context(), 2026, 2)
	require.NoError(t, err)
	assert.Equal(t, first, got[0].UpdatedAt)
	assert.Equal(t, first, got[1].UpdatedAt)

	// Changing a metric updates the row.
	rows[0].OffPassEPA = 14.0
	changed, err = store.UpsertNFLverseTeamWeeks(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	got, err = store.NFLverseTeamWeeksForSeason(t.Context(), 2026, 2)
	require.NoError(t, err)
	assert.InDelta(t, 14.0, got[1].OffPassEPA, 1e-9)
	assert.Equal(t, second, got[1].UpdatedAt)
}

func TestUpsertNFLversePlayerWeeksIdempotentAndQuery(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := time.Date(2026, time.November, 8, 12, 0, 0, 0, time.UTC)
	second := first.Add(6 * time.Hour)

	rows := []db.NFLversePlayerWeek{
		{
			ESPNID:        "4362628",
			Season:        2026,
			SeasonType:    2,
			WeekNum:       7,
			Team:          "CIN",
			OffenseSnaps:  62,
			OffensePct:    0.94,
			TargetShare:   0.29,
			AirYardsShare: 0.41,
			WOPR:          0.72,
			Carries:       0,
			Targets:       11,
			TotalEPA:      8.4,
			RZOpps:        2,
			UpdatedAt:     first,
		},
		{
			ESPNID:        "4362628",
			Season:        2026,
			SeasonType:    2,
			WeekNum:       8,
			Team:          "CIN",
			OffenseSnaps:  65,
			OffensePct:    0.96,
			TargetShare:   0.33,
			AirYardsShare: 0.45,
			WOPR:          0.81,
			Carries:       1,
			Targets:       13,
			TotalEPA:      11.2,
			RZOpps:        3,
			UpdatedAt:     first,
		},
	}

	changed, err := store.UpsertNFLversePlayerWeeks(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	byPlayer, err := store.NFLversePlayerWeeksForPlayer(t.Context(), "4362628", 2026, 2)
	require.NoError(t, err)
	require.Len(t, byPlayer, 2)
	assert.Equal(t, 8, byPlayer[0].WeekNum, "ordered most recent week first")
	assert.Equal(t, 7, byPlayer[1].WeekNum)

	bySeason, err := store.NFLversePlayerWeeksForSeason(t.Context(), 2026, 2)
	require.NoError(t, err)
	assert.Equal(t, byPlayer, bySeason)

	// Idempotent rerun reports unchanged.
	rows[0].UpdatedAt = second
	rows[1].UpdatedAt = second
	changed, err = store.UpsertNFLversePlayerWeeks(t.Context(), rows)
	require.NoError(t, err)
	assert.False(t, changed)

	// Changing snap pct updates only that row.
	rows[1].OffensePct = 0.98
	changed, err = store.UpsertNFLversePlayerWeeks(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	byPlayer, err = store.NFLversePlayerWeeksForPlayer(t.Context(), "4362628", 2026, 2)
	require.NoError(t, err)
	assert.InDelta(t, 0.98, byPlayer[0].OffensePct, 1e-9)
	assert.Equal(t, second, byPlayer[0].UpdatedAt)
	assert.Equal(t, first, byPlayer[1].UpdatedAt)
}
