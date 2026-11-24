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

// createTestSlip creates a slip whose members are the given users with default scope.
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
	return createTestSlipWithScope(t, store, group, week, name, db.SlateScopeSundayMain, legsPerMember, members...)
}

// createTestSlipWithScope creates a slip with an explicit slate scope.
func createTestSlipWithScope(
	t *testing.T,
	store *db.Store,
	group db.Group,
	week db.Week,
	name string,
	scope db.SlateScope,
	legsPerMember int,
	members ...db.User,
) db.Slip {
	t.Helper()

	ids := make([]int64, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.ID)
	}
	slip, err := store.CreateSlip(t.Context(), group.ID, week.ID, nil, name, scope, legsPerMember, ids)
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
	assert.Equal(t, db.SlateScopeSundayMain, slip.SlateScope)
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

func TestCreateSlipStoresExplicitSlateScope(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	thuSlip := createTestSlipWithScope(t, store, group, week, "Thursday Night", db.SlateScopeThursday, 2, ada)
	assert.Equal(t, db.SlateScopeThursday, thuSlip.SlateScope)

	fullSlip := createTestSlipWithScope(t, store, group, week, "Full Week", db.SlateScopeFullWeek, 2, ada)
	assert.Equal(t, db.SlateScopeFullWeek, fullSlip.SlateScope)

	// Blank slate scope defaults to SlateScopeSundayMain.
	defaultSlip, err := store.CreateSlip(t.Context(), group.ID, week.ID, nil, "Default Slip", "", 2, []int64{ada.ID})
	require.NoError(t, err)
	assert.Equal(t, db.SlateScopeSundayMain, defaultSlip.SlateScope)
}

func TestCreateSlipValidatesItsInput(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	tests := map[string]struct {
		name          string
		scope         db.SlateScope
		legsPerMember int
		memberIDs     []int64
		want          error
	}{
		"blank name": {
			name:          "  ",
			scope:         db.SlateScopeSundayMain,
			legsPerMember: 2,
			memberIDs:     []int64{ada.ID},
			want:          db.ErrSlipNameRequired,
		},
		"invalid scope": {
			name:          "Week 9",
			scope:         "bogus",
			legsPerMember: 2,
			memberIDs:     []int64{ada.ID},
			want:          db.ErrSlateScopeInvalid,
		},
		"zero legs": {
			name:          "Week 9",
			scope:         db.SlateScopeSundayMain,
			legsPerMember: 0,
			memberIDs:     []int64{ada.ID},
			want:          db.ErrLegsPerMemberRange,
		},
		"too many legs": {
			name:          "Week 9",
			scope:         db.SlateScopeSundayMain,
			legsPerMember: 6,
			memberIDs:     []int64{ada.ID},
			want:          db.ErrLegsPerMemberRange,
		},
		"no members": {
			name:          "Week 9",
			scope:         db.SlateScopeSundayMain,
			legsPerMember: 2,
			memberIDs:     nil,
			want:          db.ErrSlipMembersRequired,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := store.CreateSlip(
				t.Context(), group.ID, week.ID, nil, test.name, test.scope, test.legsPerMember, test.memberIDs)
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

	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeSundayMain,
		2,
		[]int64{ada.ID, ada.ID},
	)
	require.NoError(t, err)

	ids, err := store.SlipMemberIDs(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{ada.ID}, ids)
}

func TestCreateSlipScheduleIDAndDuplicateName(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)

	sched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Sunday Sched", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		&sched.ID,
		"Sunday Sched",
		db.SlateScopeSundayMain,
		2,
		[]int64{ada.ID},
	)
	require.NoError(t, err)
	require.NotNil(t, slip.ScheduleID)
	assert.Equal(t, sched.ID, *slip.ScheduleID)

	// Creating another slip with the same name in the same group and week must fail with ErrSlipNameTaken.
	_, err = store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Sunday Sched",
		db.SlateScopeSundayMain,
		2,
		[]int64{ada.ID},
	)
	assert.ErrorIs(t, err, db.ErrSlipNameTaken)
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

func TestSlipsWithLegsForWeekFiltersByGroupAndStatus(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	groupA := createTestGroup(t, store, ada)
	groupB := createTestGroup(t, store, bob)
	week9 := createTestWeek(t, store, 9)
	week8 := createTestWeek(t, store, 8)

	// Week 9, Group A: one GRADED with two legs, one OPEN with one leg.
	aGraded := createTestSlip(t, store, groupA, week9, "A Graded", 2, ada)
	aGradedLeg1, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: aGraded.ID, UserID: ada.ID, MarketType: "ML", Selection: "BUF ML", Odds: -140,
	})
	require.NoError(t, err)
	aGradedLeg2, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: aGraded.ID, UserID: ada.ID, MarketType: "TOTAL", Selection: "Over 47.5", Odds: -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetSlipGrade(t.Context(), aGraded.ID, parlay.Won, db.SlipGraded))
	aGraded, err = store.SlipByID(t.Context(), aGraded.ID)
	require.NoError(t, err)

	aOpen := createTestSlip(t, store, groupA, week9, "A Open", 2, ada)
	aOpenLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: aOpen.ID, UserID: ada.ID, MarketType: "ML", Selection: "KC ML", Odds: -120,
	})
	require.NoError(t, err)

	// Week 9, Group B: one GRADED with zero legs, one LOCKED with one leg.
	bGraded := createTestSlip(t, store, groupB, week9, "B Graded Empty", 2, bob)
	require.NoError(t, store.SetSlipGrade(t.Context(), bGraded.ID, parlay.Lost, db.SlipGraded))
	bGraded, err = store.SlipByID(t.Context(), bGraded.ID)
	require.NoError(t, err)

	bLocked := createTestSlip(t, store, groupB, week9, "B Locked", 2, bob)
	bLockedLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: bLocked.ID, UserID: bob.ID, MarketType: "SPREAD", Selection: "MIA +3.5", Odds: -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetSlipStatus(t.Context(), bLocked.ID, db.SlipLocked))
	bLocked, err = store.SlipByID(t.Context(), bLocked.ID)
	require.NoError(t, err)

	// Week 8, Group A: must not leak into Week 9 queries.
	otherWeek := createTestSlip(t, store, groupA, week8, "Week 8 Graded", 2, ada)
	require.NoError(t, store.SetSlipGrade(t.Context(), otherWeek.ID, parlay.Won, db.SlipGraded))

	// Invalid status is rejected.
	_, err = store.SlipsWithLegsForWeek(t.Context(), week9.ID, 0, db.SlipStatus("BOGUS"))
	require.ErrorIs(t, err, db.ErrSlipStatusInvalid)

	// All groups, all statuses in week 9 (oldest slip ID first).
	all, err := store.SlipsWithLegsForWeek(t.Context(), week9.ID, 0, "")
	require.NoError(t, err)
	assert.Equal(t, []db.SlipWithLegs{
		{Slip: aGraded, Legs: []db.Leg{aGradedLeg1, aGradedLeg2}},
		{Slip: aOpen, Legs: []db.Leg{aOpenLeg}},
		{Slip: bGraded, Legs: []db.Leg{}},
		{Slip: bLocked, Legs: []db.Leg{bLockedLeg}},
	}, all)

	// All groups, GRADED only.
	graded, err := store.SlipsWithLegsForWeek(t.Context(), week9.ID, 0, db.SlipGraded)
	require.NoError(t, err)
	assert.Equal(t, []db.SlipWithLegs{
		{Slip: aGraded, Legs: []db.Leg{aGradedLeg1, aGradedLeg2}},
		{Slip: bGraded, Legs: []db.Leg{}},
	}, graded)

	// Group A, all statuses.
	groupAAll, err := store.SlipsWithLegsForWeek(t.Context(), week9.ID, groupA.ID, "")
	require.NoError(t, err)
	assert.Equal(t, []db.SlipWithLegs{
		{Slip: aGraded, Legs: []db.Leg{aGradedLeg1, aGradedLeg2}},
		{Slip: aOpen, Legs: []db.Leg{aOpenLeg}},
	}, groupAAll)

	// Group A, GRADED only.
	groupAGraded, err := store.SlipsWithLegsForWeek(t.Context(), week9.ID, groupA.ID, db.SlipGraded)
	require.NoError(t, err)
	assert.Equal(t, []db.SlipWithLegs{
		{Slip: aGraded, Legs: []db.Leg{aGradedLeg1, aGradedLeg2}},
	}, groupAGraded)

	// Week with no matching slips returns empty non-nil slice.
	none, err := store.SlipsWithLegsForWeek(t.Context(), week8.ID, groupB.ID, "")
	require.NoError(t, err)
	assert.Empty(t, none)
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

func TestSetSlipGrade(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	require.NoError(t,
		store.SetSlipGrade(t.Context(), slip.ID, parlay.Lost, db.SlipGraded))
	stored, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Lost, stored.Result)
	assert.Equal(t, db.SlipGraded, stored.Status)

	require.ErrorIs(t,
		store.SetSlipGrade(t.Context(), slip.ID, parlay.NeedsGrading, db.SlipGraded),
		db.ErrSlipResultInvalid)
	require.ErrorIs(t,
		store.SetSlipGrade(t.Context(), slip.ID, parlay.Won, db.SlipStatus("nope")),
		db.ErrSlipStatusInvalid)
	require.ErrorIs(t,
		store.SetSlipGrade(t.Context(), 404, parlay.Won, db.SlipGraded), db.ErrSlipNotFound)
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

func TestSlateScopeLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Sunday Slate", db.SlateScopeSundayMain.Label())
	assert.Equal(t, "Thursday Slate", db.SlateScopeThursday.Label())
	assert.Equal(t, "Full Week", db.SlateScopeFullWeek.Label())
	assert.Equal(t, "Sunday Slate", db.SlateScope("unknown").Label())
	assert.Equal(t, "Sunday Slate", db.SlateScope("").Label())
}

func TestSlateScopeMatches(t *testing.T) {
	t.Parallel()

	// 2026-11-05 is Thursday, 2026-11-06 is Friday, 2026-11-07 is Saturday,
	// 2026-11-08 is Sunday, 2026-11-09 is Monday, 2026-11-10 is Tuesday.
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	thuAfternoon := time.Date(2026, 11, 5, 13, 0, 0, 0, loc)
	thuNight := time.Date(2026, 11, 5, 20, 15, 0, 0, loc)
	fri := time.Date(2026, 11, 6, 15, 0, 0, 0, loc)
	sat := time.Date(2026, 11, 7, 16, 30, 0, 0, loc)
	sunLondon := time.Date(2026, 11, 8, 9, 30, 0, 0, loc)
	sunAfternoon := time.Date(2026, 11, 8, 13, 0, 0, 0, loc)
	sunNight := time.Date(2026, 11, 8, 20, 20, 0, 0, loc)
	monNight := time.Date(2026, 11, 9, 20, 15, 0, 0, loc)
	tue := time.Date(2026, 11, 10, 19, 0, 0, 0, loc)

	// SlateScopeThursday matches only Thursday kickoffs in Eastern time.
	assert.True(t, db.SlateScopeThursday.Matches(thuAfternoon))
	assert.True(t, db.SlateScopeThursday.Matches(thuNight))
	assert.True(t, db.SlateScopeThursday.Matches(thuNight.UTC())) // Friday in UTC
	assert.False(t, db.SlateScopeThursday.Matches(fri))
	assert.False(t, db.SlateScopeThursday.Matches(sat))
	assert.False(t, db.SlateScopeThursday.Matches(sunLondon))
	assert.False(t, db.SlateScopeThursday.Matches(sunAfternoon))
	assert.False(t, db.SlateScopeThursday.Matches(sunNight))
	assert.False(t, db.SlateScopeThursday.Matches(monNight))
	assert.False(t, db.SlateScopeThursday.Matches(tue))

	// SlateScopeSundayMain matches Sunday and Monday kickoffs in Eastern time.
	assert.False(t, db.SlateScopeSundayMain.Matches(thuAfternoon))
	assert.False(t, db.SlateScopeSundayMain.Matches(thuNight))
	assert.False(t, db.SlateScopeSundayMain.Matches(fri))
	assert.False(t, db.SlateScopeSundayMain.Matches(sat))
	assert.True(t, db.SlateScopeSundayMain.Matches(sunLondon))
	assert.True(t, db.SlateScopeSundayMain.Matches(sunAfternoon))
	assert.True(t, db.SlateScopeSundayMain.Matches(sunNight))
	assert.True(t, db.SlateScopeSundayMain.Matches(sunNight.UTC())) // Monday in UTC
	assert.True(t, db.SlateScopeSundayMain.Matches(monNight))
	assert.True(t, db.SlateScopeSundayMain.Matches(monNight.UTC())) // Tuesday in UTC
	assert.False(t, db.SlateScopeSundayMain.Matches(tue))

	// Blank / default scope behaves like Sunday Main.
	var defaultScope db.SlateScope
	assert.False(t, defaultScope.Matches(thuNight))
	assert.True(t, defaultScope.Matches(sunAfternoon))
	assert.True(t, defaultScope.Matches(monNight))

	// SlateScopeFullWeek matches any day of the week.
	for _, kickoff := range []time.Time{
		thuAfternoon, thuNight, fri, sat, sunLondon, sunAfternoon, sunNight, monNight, tue,
	} {
		assert.True(t, db.SlateScopeFullWeek.Matches(kickoff))
	}
}
