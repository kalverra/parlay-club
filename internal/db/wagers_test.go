package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestUpsertAndGetWager(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	sam := createUser(t, store, "Sam")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada, sam)

	_, found, err := store.WagerForUser(t.Context(), slip.ID, ada.ID)
	require.NoError(t, err)
	assert.False(t, found)

	book := "DraftKings"
	inserted, err := store.UpsertWager(t.Context(), db.Wager{
		SlipID:   slip.ID,
		UserID:   ada.ID,
		Book:     &book,
		Stake:    25,
		Odds:     340,
		BoostPct: 25,
	})
	require.NoError(t, err)
	require.NotNil(t, inserted.Book)
	assert.Equal(t, "DraftKings", *inserted.Book)

	got, found, err := store.WagerForUser(t.Context(), slip.ID, ada.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, inserted, got)

	blank := "   "
	updated, err := store.UpsertWager(t.Context(), db.Wager{
		SlipID:   slip.ID,
		UserID:   ada.ID,
		Book:     &blank,
		Stake:    50,
		Odds:     -115,
		BoostPct: 0,
	})
	require.NoError(t, err)
	assert.Nil(t, updated.Book)

	got, found, err = store.WagerForUser(t.Context(), slip.ID, ada.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, updated, got)
}

func TestDeleteWagerAndWagersForSlip(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	sam := createUser(t, store, "Sam")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada, sam)

	_, err := store.UpsertWager(t.Context(), db.Wager{
		SlipID: slip.ID, UserID: sam.ID, Stake: 10, Odds: 200,
	})
	require.NoError(t, err)
	_, err = store.UpsertWager(t.Context(), db.Wager{
		SlipID: slip.ID, UserID: ada.ID, Stake: 20, Odds: 250,
	})
	require.NoError(t, err)

	list, err := store.WagersForSlip(t.Context(), slip.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, ada.ID, list[0].UserID)
	assert.Equal(t, sam.ID, list[1].UserID)

	require.NoError(t, store.DeleteWager(t.Context(), slip.ID, ada.ID))
	require.NoError(t, store.DeleteWager(t.Context(), slip.ID, ada.ID))

	list, err = store.WagersForSlip(t.Context(), slip.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, sam.ID, list[0].UserID)
}

func TestWagersForGroupSeasonIsolatesGroupAndSeason(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	groupA := createTestGroup(t, store, ada)
	groupB := createTestGroup(t, store, ada)

	week2026, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	week2025, err := store.UpsertWeek(t.Context(), 2025, 2, 9)
	require.NoError(t, err)

	slipA2026 := createTestSlip(t, store, groupA, week2026, "A 2026", 2, ada)
	slipA2025 := createTestSlip(t, store, groupA, week2025, "A 2025", 2, ada)
	slipB2026 := createTestSlip(t, store, groupB, week2026, "B 2026", 2, ada)

	_, err = store.UpsertWager(t.Context(), db.Wager{SlipID: slipA2026.ID, UserID: ada.ID, Stake: 15, Odds: 300})
	require.NoError(t, err)
	_, err = store.UpsertWager(t.Context(), db.Wager{SlipID: slipA2025.ID, UserID: ada.ID, Stake: 25, Odds: 300})
	require.NoError(t, err)
	_, err = store.UpsertWager(t.Context(), db.Wager{SlipID: slipB2026.ID, UserID: ada.ID, Stake: 35, Odds: 300})
	require.NoError(t, err)

	got, err := store.WagersForGroupSeason(t.Context(), groupA.ID, 2026)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, slipA2026.ID, got[0].SlipID)
	assert.Equal(t, float64(15), got[0].Stake)
}
