package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestCreateGroupSlipScheduleActiveWithTrimmedNameAndDefaultScope(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	sched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "  Sunday Main  ", "", 2)
	require.NoError(t, err)

	assert.Positive(t, sched.ID)
	assert.Equal(t, group.ID, sched.GroupID)
	assert.Equal(t, "Sunday Main", sched.Name)
	assert.Equal(t, db.SlateScopeSundayMain, sched.SlateScope)
	assert.Equal(t, 2, sched.LegsPerMember)
	assert.True(t, sched.IsActive)
	assert.False(t, sched.CreatedAt.IsZero())
}

func TestCreateGroupSlipScheduleExplicitThursdayScopeAndCustomLegs(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	sched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Thursday Night", db.SlateScopeThursday, 3)
	require.NoError(t, err)

	assert.Positive(t, sched.ID)
	assert.Equal(t, "Thursday Night", sched.Name)
	assert.Equal(t, db.SlateScopeThursday, sched.SlateScope)
	assert.Equal(t, 3, sched.LegsPerMember)
	assert.True(t, sched.IsActive)

	weekend, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Playoff Weekend", db.SlateScopeWeekend, 2)
	require.NoError(t, err)
	assert.Equal(t, db.SlateScopeWeekend, weekend.SlateScope)
}

func TestCreateGroupSlipScheduleRefusesBlankName(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	_, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "   ", db.SlateScopeSundayMain, 2)
	require.ErrorIs(t, err, db.ErrScheduleNameRequired)
}

func TestCreateGroupSlipScheduleRefusesInvalidSlateScope(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	_, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Invalid Scope", "tuesday_slate", 2)
	require.ErrorIs(t, err, db.ErrSlateScopeInvalid)
}

func TestCreateGroupSlipScheduleRefusesLegsOutsideRange(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	_, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Too Low", db.SlateScopeSundayMain, 0)
	require.ErrorIs(t, err, db.ErrLegsPerMemberRange)

	_, err = store.CreateGroupSlipSchedule(t.Context(), group.ID, "Too High", db.SlateScopeSundayMain, 6)
	require.ErrorIs(t, err, db.ErrLegsPerMemberRange)
}

func TestCreateGroupSlipScheduleRefusesDuplicateNameInSameGroup(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	_, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Sunday Main", db.SlateScopeFullWeek, 1)
	require.NoError(t, err)

	_, err = store.CreateGroupSlipSchedule(t.Context(), group.ID, "Sunday Main", db.SlateScopeFullWeek, 1)
	require.ErrorIs(t, err, db.ErrScheduleNameTaken)
}

func TestCreateGroupSlipScheduleAllowsSameNameInDifferentGroup(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	g1 := createTestGroup(t, store, user)
	g2 := createTestGroup(t, store, user)

	s1, err := store.CreateGroupSlipSchedule(t.Context(), g1.ID, "Sunday Main", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)
	s2, err := store.CreateGroupSlipSchedule(t.Context(), g2.ID, "Sunday Main", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	assert.Equal(t, "Sunday Main", s1.Name)
	assert.Equal(t, "Sunday Main", s2.Name)
	assert.NotEqual(t, s1.GroupID, s2.GroupID)
}

func TestGroupSlipScheduleByIDFindsExistingSchedule(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	created, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Main Slip", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	found, err := store.GroupSlipScheduleByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Main Slip", found.Name)
	assert.Equal(t, db.SlateScopeSundayMain, found.SlateScope)
	assert.Equal(t, 2, found.LegsPerMember)
	assert.True(t, found.IsActive)
}

func TestGroupSlipScheduleByIDMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.GroupSlipScheduleByID(t.Context(), 999999)
	require.ErrorIs(t, err, db.ErrScheduleNotFound)
}

func TestListGroupSlipSchedulesEmptyWhenNoneExist(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)

	_, err := database.ExecContext(
		t.Context(),
		"INSERT INTO groups (id, name, invite_token) VALUES (999, 'Empty', 'tok-999')",
	)
	require.NoError(t, err)
	schedules, err := store.GroupSlipSchedules(t.Context(), 999)
	require.NoError(t, err)
	assert.Empty(t, schedules)
	assert.NotNil(t, schedules)
}

func TestListGroupSlipSchedulesInOrderAndFiltersActive(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	initial, err := store.GroupSlipSchedules(t.Context(), group.ID)
	require.NoError(t, err)
	require.Len(t, initial, 1)

	s1 := initial[0]
	s2, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Slip 2", db.SlateScopeThursday, 1)
	require.NoError(t, err)
	s3, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Slip 3", db.SlateScopeFullWeek, 4)
	require.NoError(t, err)

	_, err = store.SetGroupSlipScheduleActive(t.Context(), s2.ID, false)
	require.NoError(t, err)

	all, err := store.GroupSlipSchedules(t.Context(), group.ID)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, s1.ID, all[0].ID)
	assert.Equal(t, s2.ID, all[1].ID)
	assert.Equal(t, s3.ID, all[2].ID)
	assert.True(t, all[0].IsActive)
	assert.False(t, all[1].IsActive)
	assert.True(t, all[2].IsActive)

	active, err := store.ActiveGroupSlipSchedules(t.Context(), group.ID)
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, s1.ID, active[0].ID)
	assert.Equal(t, s3.ID, active[1].ID)
}

func TestUpdateGroupSlipScheduleSuccess(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	s1, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Original Name", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	updated, err := store.UpdateGroupSlipSchedule(t.Context(), s1.ID, "  Updated Name  ", db.SlateScopeFullWeek, 4)
	require.NoError(t, err)
	assert.Equal(t, s1.ID, updated.ID)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, db.SlateScopeFullWeek, updated.SlateScope)
	assert.Equal(t, 4, updated.LegsPerMember)

	fetched, err := store.GroupSlipScheduleByID(t.Context(), s1.ID)
	require.NoError(t, err)
	assert.Equal(t, updated, fetched)
}

func TestUpdateGroupSlipScheduleRefusesBlankName(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	s1, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Original Name", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	_, err = store.UpdateGroupSlipSchedule(t.Context(), s1.ID, "", db.SlateScopeFullWeek, 4)
	require.ErrorIs(t, err, db.ErrScheduleNameRequired)
}

func TestUpdateGroupSlipScheduleRefusesInvalidSlateScope(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	s1, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Original Name", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	_, err = store.UpdateGroupSlipSchedule(t.Context(), s1.ID, "Updated", "bogus", 4)
	require.ErrorIs(t, err, db.ErrSlateScopeInvalid)
}

func TestUpdateGroupSlipScheduleRefusesLegsOutsideRange(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	s1, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Original Name", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	_, err = store.UpdateGroupSlipSchedule(t.Context(), s1.ID, "Updated", db.SlateScopeFullWeek, 0)
	require.ErrorIs(t, err, db.ErrLegsPerMemberRange)

	_, err = store.UpdateGroupSlipSchedule(t.Context(), s1.ID, "Updated", db.SlateScopeFullWeek, 6)
	require.ErrorIs(t, err, db.ErrLegsPerMemberRange)
}

func TestUpdateGroupSlipScheduleRefusesDuplicateNameInGroup(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	s1, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Original Name", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)
	s2, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Second Name", db.SlateScopeThursday, 2)
	require.NoError(t, err)

	_, err = store.UpdateGroupSlipSchedule(t.Context(), s1.ID, s2.Name, db.SlateScopeFullWeek, 2)
	require.ErrorIs(t, err, db.ErrScheduleNameTaken)
}

func TestUpdateGroupSlipScheduleMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.UpdateGroupSlipSchedule(t.Context(), 999999, "Nonexistent", db.SlateScopeFullWeek, 2)
	require.ErrorIs(t, err, db.ErrScheduleNotFound)
}

func TestSetGroupSlipScheduleActiveExplicitly(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	sched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Toggle Me", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)
	assert.True(t, sched.IsActive)

	set, err := store.SetGroupSlipScheduleActive(t.Context(), sched.ID, false)
	require.NoError(t, err)
	assert.False(t, set.IsActive)

	set, err = store.SetGroupSlipScheduleActive(t.Context(), sched.ID, true)
	require.NoError(t, err)
	assert.True(t, set.IsActive)
}

func TestSetGroupSlipScheduleActiveMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SetGroupSlipScheduleActive(t.Context(), 999999, false)
	require.ErrorIs(t, err, db.ErrScheduleNotFound)
}

func TestDeleteGroupSlipScheduleSuccessAndMissing(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	sched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Delete Me", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	err = store.DeleteGroupSlipSchedule(t.Context(), sched.ID)
	require.NoError(t, err)

	_, err = store.GroupSlipScheduleByID(t.Context(), sched.ID)
	require.ErrorIs(t, err, db.ErrScheduleNotFound)

	err = store.DeleteGroupSlipSchedule(t.Context(), sched.ID)
	require.ErrorIs(t, err, db.ErrScheduleNotFound)
}

func TestGroupSlipScheduleRejectsCustomSlateScope(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")
	group := createTestGroup(t, store, user)

	_, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Custom", db.SlateScopeCustom, 2)
	require.ErrorIs(t, err, db.ErrSlateScopeInvalid)

	sched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Sunday", db.SlateScopeSundayMain, 2)
	require.NoError(t, err)

	_, err = store.UpdateGroupSlipSchedule(t.Context(), sched.ID, "Sunday", db.SlateScopeCustom, 2)
	require.ErrorIs(t, err, db.ErrSlateScopeInvalid)
}
