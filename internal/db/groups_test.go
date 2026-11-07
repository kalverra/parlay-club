package db_test

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestCreateGroupMakesCreatorItsFirstMember(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	group, err := store.Create(t.Context(), ada.ID, "  Sunday Parlay Club  ")
	require.NoError(t, err)

	assert.Positive(t, group.ID)
	assert.Equal(t, "Sunday Parlay Club", group.Name)
	assert.Len(t, group.InviteToken, 32, "a token is 16 bytes of hex")
	_, err = hex.DecodeString(group.InviteToken)
	require.NoError(t, err, "the token is hex")

	member, err := store.IsMember(t.Context(), group.ID, ada.ID)
	require.NoError(t, err)
	assert.True(t, member, "the creator joins the group")

	groups, err := store.ForUser(t.Context(), ada.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Group{group}, groups)
}

func TestCreateGroupMintsAUniqueToken(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	first, err := store.Create(t.Context(), ada.ID, "First")
	require.NoError(t, err)
	second, err := store.Create(t.Context(), ada.ID, "Second")
	require.NoError(t, err)

	assert.NotEqual(t, first.InviteToken, second.InviteToken)
}

func TestCreateGroupRejectsABlankName(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	_, err := store.Create(t.Context(), ada.ID, "   ")
	require.ErrorIs(t, err, db.ErrGroupNameRequired)

	groups, err := store.ForUser(t.Context(), ada.ID)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestCreateGroupRejectsAnUnknownUser(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.Create(t.Context(), 404, "Sunday Parlay Club")
	require.Error(t, err)
}

func TestCreateGroupJoinsTheCallersTransaction(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	stop := errors.New("stop")
	err := store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.Create(t.Context(), ada.ID, "Rolled Back")
		require.NoError(t, err)
		return stop
	})
	require.ErrorIs(t, err, stop)

	groups, err := store.ForUser(t.Context(), ada.ID)
	require.NoError(t, err)
	assert.Empty(t, groups, "a rolled-back group leaves no membership behind")
}

func TestForUserListsOnlyTheirGroupsOldestFirst(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")

	first, err := store.Create(t.Context(), ada.ID, "First")
	require.NoError(t, err)
	second, err := store.Create(t.Context(), ada.ID, "Second")
	require.NoError(t, err)
	other, err := store.Create(t.Context(), bob.ID, "Other")
	require.NoError(t, err)

	// Pin the order so the test does not depend on two inserts landing in the
	// same second.
	_, err = database.ExecContext(t.Context(),
		"UPDATE group_members SET joined_at = '2026-01-01T00:00:00Z' WHERE group_id = ?", first.ID)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"UPDATE group_members SET joined_at = '2026-01-02T00:00:00Z' WHERE group_id = ?", second.ID)
	require.NoError(t, err)

	groups, err := store.ForUser(t.Context(), ada.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Group{first, second}, groups)
	assert.NotContains(t, groups, other)
}

func TestSlipNamesAreUniqueWithinAGroupAndWeek(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	first, err := store.Create(t.Context(), ada.ID, "First")
	require.NoError(t, err)
	second, err := store.Create(t.Context(), ada.ID, "Second")
	require.NoError(t, err)

	_, err = database.ExecContext(t.Context(),
		"INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026001, 2026, 2, 1)")
	require.NoError(t, err)

	const insert = "INSERT INTO slips (group_id, week_id, name) VALUES (?, 2026001, 'Sunday Six')"
	_, err = database.ExecContext(t.Context(), insert, first.ID)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), insert, second.ID)
	require.NoError(t, err, "the same slip name in another group is allowed")
	_, err = database.ExecContext(t.Context(), insert, first.ID)
	require.Error(t, err, "a group and week hold one slip by that name")
}

func TestForUserReturnsAnEmptySlice(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	groups, err := store.ForUser(t.Context(), ada.ID)
	require.NoError(t, err)
	assert.Empty(t, groups)
	assert.NotNil(t, groups)
}

func TestIsMemberIsFalseForOtherPeopleAndUnknownGroups(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	tests := map[string]struct {
		groupID int64
		userID  int64
	}{
		"another person":   {groupID: group.ID, userID: bob.ID},
		"unknown group":    {groupID: 404, userID: ada.ID},
		"unknown user":     {groupID: group.ID, userID: 404},
		"and unknown both": {groupID: 404, userID: 404},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			member, err := store.IsMember(t.Context(), test.groupID, test.userID)
			require.NoError(t, err)
			assert.False(t, member)
		})
	}
}
