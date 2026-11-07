package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// createTestGroup makes a group owned by owner.
func createTestGroup(t *testing.T, store *db.Store, owner db.User) db.Group {
	t.Helper()

	group, err := store.Create(t.Context(), owner.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	return group
}

// createTestWeek upserts one week.
func createTestWeek(t *testing.T, store *db.Store, weekNum int) db.Week {
	t.Helper()

	week, err := store.UpsertWeek(t.Context(), 2026, 2, weekNum)
	require.NoError(t, err)
	return week
}

// createTestSlip creates a slip whose members are the given users.
func createTestSlip(
	t *testing.T,
	store *db.Store,
	group db.Group,
	week db.Week,
	name string,
	legsPerMember int,
	members ...db.User,
) db.Slip {
	t.Helper()

	ids := make([]int64, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.ID)
	}
	slip, err := store.CreateSlip(t.Context(), group.ID, week.ID, name, legsPerMember, ids)
	require.NoError(t, err)
	return slip
}

func TestCreateSlipStoresTheSlipAndMembers(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	slip := createTestSlip(t, store, group, week, "  Week 9  ", 3, ada, bob)

	assert.Positive(t, slip.ID)
	assert.Equal(t, group.ID, slip.GroupID)
	assert.Equal(t, week.ID, slip.WeekID)
	assert.Equal(t, "Week 9", slip.Name, "the name is trimmed")
	assert.Equal(t, 3, slip.LegsPerMember)
	assert.Equal(t, db.SlipOpen, slip.Status)
	assert.Equal(t, parlay.Pending, slip.Result)
	assert.False(t, slip.CreatedAt.IsZero())

	stored, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, slip, stored)

	members, err := store.SlipMembers(t.Context(), slip.ID)
	require.NoError(t, err)
	require.Len(t, members, 2)
	assert.Equal(t, ada, members[0].User)
	assert.Nil(t, members[0].LockedAt)
	assert.Equal(t, bob, members[1].User)
	assert.Nil(t, members[1].LockedAt)

	ids, err := store.SlipMemberIDs(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{ada.ID, bob.ID}, ids)
}

func TestCreateSlipValidatesItsInput(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	tests := map[string]struct {
		name          string
		legsPerMember int
		memberIDs     []int64
		want          error
	}{
		"blank name":    {name: "  ", legsPerMember: 2, memberIDs: []int64{ada.ID}, want: db.ErrSlipNameRequired},
		"zero legs":     {name: "Week 9", legsPerMember: 0, memberIDs: []int64{ada.ID}, want: db.ErrLegsPerMemberRange},
		"too many legs": {name: "Week 9", legsPerMember: 6, memberIDs: []int64{ada.ID}, want: db.ErrLegsPerMemberRange},
		"no members":    {name: "Week 9", legsPerMember: 2, memberIDs: nil, want: db.ErrSlipMembersRequired},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := store.CreateSlip(
				t.Context(), group.ID, week.ID, test.name, test.legsPerMember, test.memberIDs)
			require.ErrorIs(t, err, test.want)

			slips, err := store.SlipsForWeek(t.Context(), group.ID, week.ID)
			require.NoError(t, err)
			assert.Empty(t, slips, "a refused slip writes nothing")
		})
	}
}

func TestCreateSlipIgnoresDuplicateMembers(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	slip, err := store.CreateSlip(t.Context(), group.ID, week.ID, "Week 9", 2, []int64{ada.ID, ada.ID})
	require.NoError(t, err)

	ids, err := store.SlipMemberIDs(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{ada.ID}, ids)
}

func TestSlipsForWeekReturnsTheGroupsSlipsNewestFirst(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	other := createTestGroup(t, store, bob)
	week := createTestWeek(t, store, 9)
	older := createTestWeek(t, store, 8)

	first := createTestSlip(t, store, group, week, "Week 9", 2, ada)
	second := createTestSlip(t, store, group, week, "Week 9 #2", 2, ada)
	createTestSlip(t, store, other, week, "Their Week 9", 2, bob)
	createTestSlip(t, store, group, older, "Week 8", 2, ada)

	_, err := database.ExecContext(t.Context(),
		"UPDATE slips SET created_at = '2026-01-01T00:00:00Z' WHERE id = ?", first.ID)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"UPDATE slips SET created_at = '2026-01-02T00:00:00Z' WHERE id = ?", second.ID)
	require.NoError(t, err)
	first.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second.CreatedAt = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	slips, err := store.SlipsForWeek(t.Context(), group.ID, week.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Slip{second, first}, slips)

	newest, err := store.NewestSlipForWeek(t.Context(), group.ID, week.ID)
	require.NoError(t, err)
	assert.Equal(t, second.ID, newest.ID)
}

func TestSlipIDsForWeekReturnsEveryGroupsSlips(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	other := createTestGroup(t, store, bob)
	week := createTestWeek(t, store, 9)
	older := createTestWeek(t, store, 8)

	first := createTestSlip(t, store, group, week, "Week 9", 2, ada)
	second := createTestSlip(t, store, other, week, "Their Week 9", 2, bob)
	createTestSlip(t, store, group, older, "Week 8", 2, ada)

	ids, err := store.SlipIDsForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{first.ID, second.ID}, ids)
}

func TestNewestSlipForWeekReportsAMissingWeek(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	_, err := store.NewestSlipForWeek(t.Context(), group.ID, week.ID)
	require.ErrorIs(t, err, db.ErrSlipNotFound)
}

func TestPreviousSlipCopiesFromTheSameWeekThenEarlierWeeks(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	other := createTestGroup(t, store, bob)
	week8 := createTestWeek(t, store, 8)
	week9 := createTestWeek(t, store, 9)
	week10 := createTestWeek(t, store, 10)

	_, err := store.PreviousSlip(t.Context(), group.ID, week10.ID)
	require.ErrorIs(t, err, db.ErrSlipNotFound, "a group with no slip has no previous one")

	first := createTestSlip(t, store, group, week9, "Week 9", 3, ada)
	createTestSlip(t, store, other, week9, "Their Week 9", 5, bob)
	_, err = database.ExecContext(t.Context(),
		"UPDATE slips SET created_at = '2026-01-01T00:00:00Z' WHERE id = ?", first.ID)
	require.NoError(t, err)

	previous, err := store.PreviousSlip(t.Context(), group.ID, week10.ID)
	require.NoError(t, err)
	assert.Equal(t, first.ID, previous.ID, "the latest earlier week is the previous slip")

	second := createTestSlip(t, store, group, week9, "Week 9 #2", 4, ada)
	_, err = database.ExecContext(t.Context(),
		"UPDATE slips SET created_at = '2026-12-01T00:00:00Z' WHERE id = ?", second.ID)
	require.NoError(t, err)

	previous, err = store.PreviousSlip(t.Context(), group.ID, week9.ID)
	require.NoError(t, err)
	assert.Equal(t, second.ID, previous.ID, "a second slip in the week copies that week's latest")

	previous, err = store.PreviousSlip(t.Context(), group.ID, week8.ID)
	require.ErrorIs(t, err, db.ErrSlipNotFound, "nothing before the first slip")
}

func TestSlipMembersReadsLockTimes(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada, bob)

	lockedAt := time.Date(2026, 11, 7, 17, 30, 0, 0, time.UTC)
	require.NoError(t, store.SetSlipMemberLock(t.Context(), slip.ID, ada.ID, &lockedAt))

	members, err := store.SlipMembers(t.Context(), slip.ID)
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.NotNil(t, members[0].LockedAt)
	assert.True(t, lockedAt.Equal(*members[0].LockedAt))
	assert.Nil(t, members[1].LockedAt)

	require.NoError(t, store.SetSlipMemberLock(t.Context(), slip.ID, ada.ID, nil))
	members, err = store.SlipMembers(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Nil(t, members[0].LockedAt, "a nil time unlocks")
}

func TestAddAndRemoveSlipMember(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	require.NoError(t, store.AddSlipMember(t.Context(), slip.ID, bob.ID))
	require.NoError(t, store.AddSlipMember(t.Context(), slip.ID, bob.ID), "adding twice changes nothing")

	ids, err := store.SlipMemberIDs(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{ada.ID, bob.ID}, ids)

	require.NoError(t, store.RemoveSlipMember(t.Context(), slip.ID, bob.ID))
	ids, err = store.SlipMemberIDs(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{ada.ID}, ids)

	require.ErrorIs(t,
		store.RemoveSlipMember(t.Context(), slip.ID, bob.ID), db.ErrSlipMemberNotFound)
	require.ErrorIs(t,
		store.SetSlipMemberLock(t.Context(), slip.ID, bob.ID, nil), db.ErrSlipMemberNotFound)
}

func TestUpdateSlipChangesNameAndLegCount(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	updated, err := store.UpdateSlip(t.Context(), slip.ID, "  Sunday Six  ", 4)
	require.NoError(t, err)
	assert.Equal(t, "Sunday Six", updated.Name)
	assert.Equal(t, 4, updated.LegsPerMember)

	stored, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, updated, stored)

	_, err = store.UpdateSlip(t.Context(), slip.ID, " ", 4)
	require.ErrorIs(t, err, db.ErrSlipNameRequired)
	_, err = store.UpdateSlip(t.Context(), slip.ID, "Sunday Six", 0)
	require.ErrorIs(t, err, db.ErrLegsPerMemberRange)
	_, err = store.UpdateSlip(t.Context(), 404, "Sunday Six", 2)
	require.ErrorIs(t, err, db.ErrSlipNotFound)
}

func TestSetSlipStatus(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	require.NoError(t, store.SetSlipStatus(t.Context(), slip.ID, db.SlipLocked))
	stored, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, db.SlipLocked, stored.Status)

	require.NoError(t, store.SetSlipStatus(t.Context(), slip.ID, db.SlipGraded))
	stored, err = store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, db.SlipGraded, stored.Status)

	require.ErrorIs(t,
		store.SetSlipStatus(t.Context(), slip.ID, db.SlipStatus("nope")), db.ErrSlipStatusInvalid)
	require.ErrorIs(t,
		store.SetSlipStatus(t.Context(), 404, db.SlipOpen), db.ErrSlipNotFound)
}
