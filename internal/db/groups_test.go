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

	schedules, err := store.GroupSlipSchedules(t.Context(), group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Main Sunday Slip", schedules[0].Name)
	assert.Equal(t, db.SlateScopeSundayMain, schedules[0].SlateScope)
	assert.Equal(t, 2, schedules[0].LegsPerMember)
	assert.True(t, schedules[0].IsActive)
}

func TestFirstGroupReturnsTheOldest(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	first, err := store.Create(t.Context(), ada.ID, "First")
	require.NoError(t, err)
	_, err = store.Create(t.Context(), ada.ID, "Second")
	require.NoError(t, err)

	found, err := store.FirstGroup(t.Context())
	require.NoError(t, err)
	assert.Equal(t, first, found)
}

func TestFirstGroupReportsNoGroups(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.FirstGroup(t.Context())
	require.ErrorIs(t, err, db.ErrGroupNotFound)
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

func TestByTokenFindsTheInvitedGroup(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	found, err := store.ByToken(t.Context(), group.InviteToken)
	require.NoError(t, err)
	assert.Equal(t, group, found)

	_, err = store.ByToken(t.Context(), "no-such-token")
	require.ErrorIs(t, err, db.ErrInviteNotFound)

	_, err = store.ByToken(t.Context(), "")
	require.ErrorIs(t, err, db.ErrInviteNotFound)
}

func TestJoinByTokenIsIdempotent(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	first, err := store.JoinByToken(t.Context(), bob.ID, group.InviteToken)
	require.NoError(t, err)
	assert.Equal(t, group, first)

	second, err := store.JoinByToken(t.Context(), bob.ID, group.InviteToken)
	require.NoError(t, err)
	assert.Equal(t, group, second, "joining twice still answers the group")

	groups, err := store.ForUser(t.Context(), bob.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Group{group}, groups, "joining twice adds one membership")
}

func TestJoinByTokenRejectsAnUnknownInvite(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	bob := createUser(t, store, "Bob")

	_, err := store.JoinByToken(t.Context(), bob.ID, "no-such-token")
	require.ErrorIs(t, err, db.ErrInviteNotFound)

	groups, err := store.ForUser(t.Context(), bob.ID)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestResetInviteKillsTheOldLink(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)

	token, err := store.ResetInvite(t.Context(), group.ID)
	require.NoError(t, err)
	assert.NotEqual(t, group.InviteToken, token)
	assert.Len(t, token, 32)

	_, err = store.ByToken(t.Context(), group.InviteToken)
	require.ErrorIs(t, err, db.ErrInviteNotFound, "the old link stops working")

	found, err := store.ByToken(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, group.ID, found.ID)
	assert.Equal(t, token, found.InviteToken)
}

func TestResetInviteRejectsAnUnknownGroup(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.ResetInvite(t.Context(), 404)
	require.ErrorIs(t, err, db.ErrGroupNotFound)
}

func TestLeaveRemovesOnlyTheMembership(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	_, err = store.JoinByToken(t.Context(), bob.ID, group.InviteToken)
	require.NoError(t, err)
	solo, err := store.Create(t.Context(), bob.ID, "Just me")
	require.NoError(t, err)

	_, err = database.ExecContext(t.Context(),
		"INSERT INTO weeks (id, season, season_type, week_num) VALUES (2026001, 2026, 2, 1)")
	require.NoError(t, err)
	result, err := database.ExecContext(t.Context(),
		"INSERT INTO slips (group_id, week_id, name) VALUES (?, 2026001, 'Sunday Six')", group.ID)
	require.NoError(t, err)
	slipID, err := result.LastInsertId()
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), `
		INSERT INTO legs (slip_id, user_id, market_type, selection)
		VALUES (?, ?, 'OTHER', 'Bills moneyline')`, slipID, bob.ID)
	require.NoError(t, err)

	require.NoError(t, store.Leave(t.Context(), group.ID, bob.ID))

	groups, err := store.ForUser(t.Context(), bob.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Group{solo}, groups, "only the membership row goes")

	found, err := store.ByToken(t.Context(), group.InviteToken)
	require.NoError(t, err)
	assert.Equal(t, group, found, "the group and its invite stay")

	var legs int
	require.NoError(t, database.QueryRowContext(t.Context(),
		"SELECT count(*) FROM legs WHERE slip_id = ? AND user_id = ?", slipID, bob.ID).Scan(&legs))
	assert.Equal(t, 1, legs, "the person's legs stay on the group's slips")
}

func TestLeaveRefusesTheLastGroup(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group, err := store.Create(t.Context(), ada.ID, "Just me")
	require.NoError(t, err)

	err = store.Leave(t.Context(), group.ID, ada.ID)
	require.ErrorIs(t, err, db.ErrLastGroup)

	groups, err := store.ForUser(t.Context(), ada.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Group{group}, groups)

	schedules, err := store.GroupSlipSchedules(t.Context(), group.ID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "Main Sunday Slip", schedules[0].Name)
	assert.Equal(t, db.SlateScopeSundayMain, schedules[0].SlateScope)
	assert.Equal(t, 2, schedules[0].LegsPerMember)
	assert.True(t, schedules[0].IsActive)
}

func TestLeaveRejectsANonMember(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	_, err = store.Create(t.Context(), bob.ID, "Just me")
	require.NoError(t, err)

	err = store.Leave(t.Context(), group.ID, bob.ID)
	require.ErrorIs(t, err, db.ErrNotMember)
}

func TestMembersListsActivePeopleOldestFirst(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	grace := createUser(t, store, "Grace")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	_, err = store.JoinByToken(t.Context(), bob.ID, group.InviteToken)
	require.NoError(t, err)
	_, err = store.JoinByToken(t.Context(), grace.ID, group.InviteToken)
	require.NoError(t, err)

	_, err = database.ExecContext(t.Context(),
		"UPDATE group_members SET joined_at = '2026-01-01T00:00:00Z' WHERE group_id = ? AND user_id = ?",
		group.ID, ada.ID)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"UPDATE group_members SET joined_at = '2026-01-02T00:00:00Z' WHERE group_id = ? AND user_id = ?",
		group.ID, bob.ID)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", grace.ID)
	require.NoError(t, err)

	members, err := store.Members(t.Context(), group.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.User{ada, bob}, members, "inactive people drop out of the list")
}

func TestGroupMembersKeepsInactivePeople(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	grace := createUser(t, store, "Grace")
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	_, err = store.JoinByToken(t.Context(), grace.ID, group.InviteToken)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE id = ?", grace.ID)
	require.NoError(t, err)

	members, err := store.GroupMembers(t.Context(), group.ID)
	require.NoError(t, err)
	require.Len(t, members, 2)
	assert.Equal(t, ada.ID, members[0].ID)
	assert.Equal(t, grace.ID, members[1].ID)
	assert.False(t, members[1].Active, "the inactive person stays listed so they can be turned back on")
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
