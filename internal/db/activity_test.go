package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestAddActivityWritesOneEntry(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	entry, err := store.AddActivity(t.Context(), slip.ID, &ada.ID, "  Ada added BUF -3.5  ")
	require.NoError(t, err)
	assert.Positive(t, entry.ID)
	assert.Equal(t, slip.ID, entry.SlipID)
	require.NotNil(t, entry.UserID)
	assert.Equal(t, ada.ID, *entry.UserID)
	assert.Equal(t, "Ada added BUF -3.5", entry.Message, "the message is trimmed")
	assert.False(t, entry.CreatedAt.IsZero())

	system, err := store.AddActivity(t.Context(), slip.ID, nil, "Slip locked")
	require.NoError(t, err)
	assert.Nil(t, system.UserID, "a nil user records a system entry")
}

func TestAddActivityRejectsABlankMessage(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	_, err := store.AddActivity(t.Context(), slip.ID, &ada.ID, "   ")
	require.ErrorIs(t, err, db.ErrActivityMessageRequired)
}

func TestActivityForSlipIsNewestFirst(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	firstSlip := createTestSlip(t, store, group, week, "Week 9", 2, ada)
	secondSlip := createTestSlip(t, store, group, week, "Week 9 #2", 2, ada)

	first, err := store.AddActivity(t.Context(), firstSlip.ID, &ada.ID, "Ada added BUF -3.5")
	require.NoError(t, err)
	second, err := store.AddActivity(t.Context(), firstSlip.ID, &bob.ID, "Bob locked in")
	require.NoError(t, err)
	_, err = store.AddActivity(t.Context(), secondSlip.ID, nil, "Another slip")
	require.NoError(t, err)

	entries, err := store.ActivityForSlip(t.Context(), firstSlip.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Activity{second, first}, entries)
}
