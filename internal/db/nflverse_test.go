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
			Opponent:      "CLE",
			PlayerName:    "Ja'Marr Chase",
			Position:      "WR",
			OffenseSnaps:  62,
			OffensePct:    0.94,
			TargetShare:   0.29,
			AirYardsShare: 0.41,
			WOPR:          0.72,
			Carries:       0,
			Targets:       11,
			Receptions:    8,
			RecYds:        112,
			RecTDs:        1,
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
			Opponent:      "BAL",
			PlayerName:    "Ja'Marr Chase",
			Position:      "WR",
			OffenseSnaps:  65,
			OffensePct:    0.96,
			TargetShare:   0.33,
			AirYardsShare: 0.45,
			WOPR:          0.81,
			Carries:       1,
			RushYds:       6,
			Targets:       13,
			Receptions:    10,
			RecYds:        148,
			RecTDs:        2,
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

	batch, err := store.NFLversePlayerWeeksForPlayers(t.Context(), []string{"4362628", "missing"}, 2026, 2)
	require.NoError(t, err)
	assert.Equal(t, byPlayer, batch["4362628"])
	assert.Empty(t, batch["missing"])

	emptyBatch, err := store.NFLversePlayerWeeksForPlayers(t.Context(), nil, 2026, 2)
	require.NoError(t, err)
	assert.Empty(t, emptyBatch)

	// Multi-season-type queries merge regular and postseason rows in expected order.
	_, err = store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{
		{Season: 2026, SeasonType: 2, WeekNum: 18, Team: "CIN", OffPassEPA: 0.10, UpdatedAt: first},
		{Season: 2026, SeasonType: 3, WeekNum: 1, Team: "CIN", OffPassEPA: 0.25, UpdatedAt: second},
	})
	require.NoError(t, err)
	teamMerged, err := store.NFLverseTeamWeeksForSeasonTypes(t.Context(), 2026, 2, 3)
	require.NoError(t, err)
	require.Len(t, teamMerged, 2)
	assert.Equal(t, 2, teamMerged[0].SeasonType)
	assert.Equal(t, 3, teamMerged[1].SeasonType)

	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID:     "4362628",
			Season:     2026,
			SeasonType: 3,
			WeekNum:    1,
			Team:       "CIN",
			Opponent:   "BAL",
			PlayerName: "Ja'Marr Chase",
			Position:   "WR",
			OffensePct: 0.99,
			RecYds:     130,
			UpdatedAt:  second,
		},
	})
	require.NoError(t, err)
	playerMerged, err := store.NFLversePlayerWeeksForPlayersTypes(t.Context(), []string{"4362628"}, 2026, 2, 3)
	require.NoError(t, err)
	require.Len(t, playerMerged["4362628"], 3)
	assert.Equal(t, 3, playerMerged["4362628"][0].SeasonType, "postseason week comes first")
	assert.Equal(t, 8, playerMerged["4362628"][1].WeekNum)
	assert.Equal(t, 7, playerMerged["4362628"][2].WeekNum)

	// Opponent + position queries return matching rows across season types,
	// normalizing FB/HB to RB.
	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID:     "4429202",
			Season:     2026,
			SeasonType: 2,
			WeekNum:    6,
			Team:       "PIT",
			Opponent:   "BAL",
			PlayerName: "Back One",
			Position:   "HB",
			RushYds:    88,
			UpdatedAt:  first,
		},
		{
			ESPNID:     "3915511",
			Season:     2026,
			SeasonType: 2,
			WeekNum:    8,
			Team:       "CIN",
			Opponent:   "BAL",
			PlayerName: "Joe Burrow",
			Position:   "QB",
			PassAtt:    36,
			PassYds:    310,
			PassTDs:    3,
			UpdatedAt:  first,
		},
	})
	require.NoError(t, err)

	vsBALWR, err := store.NFLversePlayerWeeksAgainstOpponent(t.Context(), "BAL", "WR", 2026, 2, 3)
	require.NoError(t, err)
	require.Len(t, vsBALWR, 2)
	assert.Equal(t, 3, vsBALWR[0].SeasonType)
	assert.Equal(t, 130, vsBALWR[0].RecYds)
	assert.Equal(t, 2, vsBALWR[1].SeasonType)
	assert.Equal(t, 8, vsBALWR[1].WeekNum)

	vsBALRB, err := store.NFLversePlayerWeeksAgainstOpponent(t.Context(), "bal", "fb", 2026, 2)
	require.NoError(t, err)
	require.Len(t, vsBALRB, 1)
	assert.Equal(t, "RB", vsBALRB[0].Position)
	assert.Equal(t, 88, vsBALRB[0].RushYds)

	allWRs, err := store.NFLversePlayerWeeksForPositionTypes(t.Context(), "WR", 2026, 2, 3)
	require.NoError(t, err)
	require.Len(t, allWRs, 3)
	assert.Equal(t, 3, allWRs[0].SeasonType)
	assert.Equal(t, 8, allWRs[1].WeekNum)
	assert.Equal(t, 7, allWRs[2].WeekNum)

	emptyOpp, err := store.NFLversePlayerWeeksAgainstOpponent(t.Context(), "", "WR", 2026, 2)
	require.NoError(t, err)
	assert.Empty(t, emptyOpp)

	emptyPos, err := store.NFLversePlayerWeeksForPositionTypes(t.Context(), "WR", 2026)
	require.NoError(t, err)
	assert.Empty(t, emptyPos)
}
