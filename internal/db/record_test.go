package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestRecordSeasonQueriesIsolateGroupAndStatus(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	sam := createUser(t, store, "Sam")
	ben := createUser(t, store, "Ben")

	groupA := createTestGroup(t, store, ada)
	_, err := store.JoinByToken(t.Context(), sam.ID, groupA.InviteToken)
	require.NoError(t, err)
	groupB := createTestGroup(t, store, ben)

	wk2024 := createSeasonWeek(t, store, 2024, 10)
	wk2025 := createSeasonWeek(t, store, 2025, 1)
	wk2025W2 := createSeasonWeek(t, store, 2025, 2)
	wk2026 := createSeasonWeek(t, store, 2026, 1)

	// Group A: 2024 graded, 2025 graded (two weeks), 2026 open only.
	slip2024 := createTestSlip(t, store, groupA, wk2024, "2024 W10", 2, ada, sam)
	require.NoError(t, store.SetSlipGrade(t.Context(), slip2024.ID, parlay.Won, db.SlipGraded))

	slip2025W2 := createTestSlip(t, store, groupA, wk2025W2, "2025 W2", 2, ada, sam)
	require.NoError(t, store.SetSlipGrade(t.Context(), slip2025W2.ID, parlay.Lost, db.SlipGraded))

	slip2025W1 := createTestSlip(t, store, groupA, wk2025, "2025 W1", 2, ada, sam)
	require.NoError(t, store.SetSlipGrade(t.Context(), slip2025W1.ID, parlay.Push, db.SlipGraded))

	slip2025Open := createTestSlip(t, store, groupA, wk2025, "2025 W1 Open", 2, ada)
	require.NoError(t, store.SetSlipStatus(t.Context(), slip2025Open.ID, db.SlipLocked))

	_ = createTestSlip(t, store, groupA, wk2026, "2026 W1 Open", 2, ada)

	// Group B: 2026 graded (must not leak into Group A).
	slipB2026 := createTestSlip(t, store, groupB, wk2026, "B 2026", 2, ben)
	require.NoError(t, store.SetSlipGrade(t.Context(), slipB2026.ID, parlay.Won, db.SlipGraded))

	// Add legs across slips.
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip2025W1.ID, UserID: ada.ID, MarketType: "ML", Selection: "Chiefs ML", Odds: -120,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip2025W2.ID, UserID: sam.ID, MarketType: "SPREAD", Selection: "Bills -3.5", Odds: -110,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip2025Open.ID, UserID: ada.ID, MarketType: "TOTAL", Selection: "Over 45.5", Odds: -110,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID: slipB2026.ID, UserID: ben.ID, MarketType: "ML", Selection: "Lions ML", Odds: -150,
	})
	require.NoError(t, err)

	seasons, err := store.SeasonsForGroup(t.Context(), groupA.ID)
	require.NoError(t, err)
	assert.Equal(t, []int{2026, 2025, 2024}, seasons)

	gradedSeasons, err := store.GradedSeasonsForGroup(t.Context(), groupA.ID)
	require.NoError(t, err)
	assert.Equal(t, []int{2025, 2024}, gradedSeasons)

	gradedSlips, err := store.GradedSlipsForGroupSeason(t.Context(), groupA.ID, 2025)
	require.NoError(t, err)
	require.Len(t, gradedSlips, 2)
	// Week 1 must sort before Week 2 even though Week 2 was created first.
	assert.Equal(t, slip2025W1.ID, gradedSlips[0].Slip.ID)
	assert.Equal(t, 1, gradedSlips[0].Week.WeekNum)
	assert.Equal(t, slip2025W2.ID, gradedSlips[1].Slip.ID)
	assert.Equal(t, 2, gradedSlips[1].Week.WeekNum)

	gradedLegs, err := store.GradedLegsForGroupSeason(t.Context(), groupA.ID, 2025)
	require.NoError(t, err)
	require.Len(t, gradedLegs, 2)
	assert.Equal(t, slip2025W2.ID, gradedLegs[0].SlipID)
	assert.Equal(t, slip2025W1.ID, gradedLegs[1].SlipID)

	// Sam leaves group A, but still authored a leg on a 2025 graded slip.
	_, err = store.JoinByToken(t.Context(), sam.ID, groupB.InviteToken)
	require.NoError(t, err)
	require.NoError(t, store.Leave(t.Context(), groupA.ID, sam.ID))

	recordUsers, err := store.RecordUsersForGroupSeason(t.Context(), groupA.ID, 2025)
	require.NoError(t, err)
	require.Len(t, recordUsers, 2)
	assert.Equal(t, []int64{ada.ID, sam.ID}, []int64{recordUsers[0].ID, recordUsers[1].ID})
}

func createSeasonWeek(t *testing.T, store *db.Store, season, weekNum int) db.Week {
	t.Helper()
	week, err := store.UpsertWeek(t.Context(), season, 2, weekNum)
	require.NoError(t, err)
	return week
}
