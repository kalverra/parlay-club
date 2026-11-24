package slips_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

var ensureTestNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

// fakeStore answers the queries CreateDefaults and EnsureWeekSlips make and records their
// arguments.
type fakeStore struct {
	previous    db.Slip
	previousErr error
	memberIDs   []int64
	memberErr   error

	gotGroupID    int64
	gotWeekID     int64
	gotMemberSlip int64

	schedules    []db.GroupSlipSchedule
	schedulesErr error
	slips        []db.Slip
	slipsErr     error
	games        []db.Game
	gamesErr     error
	members      []db.User
	membersErr   error
	createSlipFn func(ctx context.Context, groupID, weekID int64, scheduleID *int64, name string, slateScope db.SlateScope, legsPerMember int, memberIDs []int64) (db.Slip, error)
	createdSlips []db.Slip
}

func (f *fakeStore) PreviousSlip(_ context.Context, groupID, weekID int64) (db.Slip, error) {
	f.gotGroupID = groupID
	f.gotWeekID = weekID
	return f.previous, f.previousErr
}

func (f *fakeStore) SlipMemberIDs(_ context.Context, slipID int64) ([]int64, error) {
	f.gotMemberSlip = slipID
	return f.memberIDs, f.memberErr
}

func (f *fakeStore) ActiveGroupSlipSchedules(_ context.Context, groupID int64) ([]db.GroupSlipSchedule, error) {
	f.gotGroupID = groupID
	return f.schedules, f.schedulesErr
}

func (f *fakeStore) SlipsForWeek(_ context.Context, groupID, weekID int64) ([]db.Slip, error) {
	f.gotGroupID = groupID
	f.gotWeekID = weekID
	return f.slips, f.slipsErr
}

func (f *fakeStore) GamesForWeek(_ context.Context, weekID int64) ([]db.Game, error) {
	f.gotWeekID = weekID
	return f.games, f.gamesErr
}

func (f *fakeStore) Members(_ context.Context, groupID int64) ([]db.User, error) {
	f.gotGroupID = groupID
	return f.members, f.membersErr
}

func (f *fakeStore) CreateSlip(
	ctx context.Context,
	groupID, weekID int64,
	scheduleID *int64,
	name string,
	slateScope db.SlateScope,
	legsPerMember int,
	memberIDs []int64,
) (db.Slip, error) {
	if f.createSlipFn != nil {
		return f.createSlipFn(ctx, groupID, weekID, scheduleID, name, slateScope, legsPerMember, memberIDs)
	}
	slip := db.Slip{
		ID:            int64(len(f.createdSlips) + 100),
		GroupID:       groupID,
		WeekID:        weekID,
		ScheduleID:    scheduleID,
		Name:          name,
		SlateScope:    slateScope,
		LegsPerMember: legsPerMember,
		Status:        db.SlipOpen,
	}
	f.createdSlips = append(f.createdSlips, slip)
	f.slips = append(f.slips, slip)
	return slip, nil
}

func TestCreateDefaultsUsesTheBuiltInValuesWithoutAPreviousSlip(t *testing.T) {
	t.Parallel()

	store := &fakeStore{previousErr: db.ErrSlipNotFound}

	defaults, err := slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.NoError(t, err)
	assert.Equal(t, slips.DefaultLegsPerMember, defaults.LegsPerMember)
	assert.NotNil(t, defaults.MemberIDs)
	assert.Empty(t, defaults.MemberIDs)
	assert.Equal(t, int64(7), store.gotGroupID)
	assert.Equal(t, int64(2026209), store.gotWeekID)
}

func TestCreateDefaultsCopiesThePreviousSlip(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		previous:  db.Slip{ID: 42, LegsPerMember: 4},
		memberIDs: []int64{2, 5, 9},
	}

	defaults, err := slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.NoError(t, err)
	assert.Equal(t, 4, defaults.LegsPerMember)
	assert.Equal(t, []int64{2, 5, 9}, defaults.MemberIDs)
	assert.Equal(t, int64(42), store.gotMemberSlip)
}

func TestCreateDefaultsReportsStoreErrors(t *testing.T) {
	t.Parallel()

	stop := errors.New("stop")
	store := &fakeStore{previousErr: stop}

	_, err := slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.ErrorIs(t, err, stop)

	store = &fakeStore{previous: db.Slip{ID: 42, LegsPerMember: 2}, memberErr: stop}
	_, err = slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.ErrorIs(t, err, stop)
}

func TestEnsureWeekSlipsCreatesMissingScheduledSlips(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		schedules: []db.GroupSlipSchedule{
			{
				ID:            1,
				GroupID:       7,
				Name:          "Main Sunday Slip",
				SlateScope:    db.SlateScopeSundayMain,
				LegsPerMember: 2,
				IsActive:      true,
			},
			{
				ID:            2,
				GroupID:       7,
				Name:          "Thursday Night",
				SlateScope:    db.SlateScopeThursday,
				LegsPerMember: 3,
				IsActive:      true,
			},
		},
		slips: []db.Slip{},
		members: []db.User{
			{ID: 10, Name: "Alice", Active: true},
			{ID: 20, Name: "Bob", Active: true},
		},
	}

	result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "Main Sunday Slip", result[0].Name)
	assert.Equal(t, db.SlateScopeSundayMain, result[0].SlateScope)
	assert.Equal(t, 2, result[0].LegsPerMember)

	assert.Equal(t, "Thursday Night", result[1].Name)
	assert.Equal(t, db.SlateScopeThursday, result[1].SlateScope)
	assert.Equal(t, 3, result[1].LegsPerMember)

	require.Len(t, store.createdSlips, 2)
	assert.Equal(t, int64(7), store.gotGroupID)
	assert.Equal(t, int64(2026209), store.gotWeekID)
}

func TestEnsureWeekSlipsIsIdempotentWhenSlipsAlreadyExist(t *testing.T) {
	t.Parallel()

	sched1ID := int64(1)
	sched2ID := int64(2)
	existing := []db.Slip{
		{
			ID:            101,
			GroupID:       7,
			WeekID:        2026209,
			ScheduleID:    &sched1ID,
			Name:          "Main Sunday Slip",
			SlateScope:    db.SlateScopeSundayMain,
			LegsPerMember: 2,
		},
		{
			ID:            102,
			GroupID:       7,
			WeekID:        2026209,
			ScheduleID:    &sched2ID,
			Name:          "Thursday Night",
			SlateScope:    db.SlateScopeThursday,
			LegsPerMember: 3,
		},
	}

	store := &fakeStore{
		schedules: []db.GroupSlipSchedule{
			{
				ID:            1,
				GroupID:       7,
				Name:          "Main Sunday Slip",
				SlateScope:    db.SlateScopeSundayMain,
				LegsPerMember: 2,
				IsActive:      true,
			},
			{
				ID:            2,
				GroupID:       7,
				Name:          "Thursday Night",
				SlateScope:    db.SlateScopeThursday,
				LegsPerMember: 3,
				IsActive:      true,
			},
		},
		slips: existing,
	}

	result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.NoError(t, err)
	assert.Equal(t, existing, result)
	assert.Empty(t, store.createdSlips, "should not create any slips when all exist")
}

func TestEnsureWeekSlipsCreatesOnlyMissingSchedules(t *testing.T) {
	t.Parallel()

	sched1ID := int64(1)
	existing := []db.Slip{
		{
			ID:            101,
			GroupID:       7,
			WeekID:        2026209,
			ScheduleID:    &sched1ID,
			Name:          "Main Sunday Slip",
			SlateScope:    db.SlateScopeSundayMain,
			LegsPerMember: 2,
		},
	}

	store := &fakeStore{
		schedules: []db.GroupSlipSchedule{
			{
				ID:            1,
				GroupID:       7,
				Name:          "Main Sunday Slip",
				SlateScope:    db.SlateScopeSundayMain,
				LegsPerMember: 2,
				IsActive:      true,
			},
			{
				ID:            2,
				GroupID:       7,
				Name:          "Thursday Night",
				SlateScope:    db.SlateScopeThursday,
				LegsPerMember: 3,
				IsActive:      true,
			},
		},
		slips: existing,
		members: []db.User{
			{ID: 10, Name: "Alice", Active: true},
		},
	}

	result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Main Sunday Slip", result[0].Name)
	assert.Equal(t, "Thursday Night", result[1].Name)

	require.Len(t, store.createdSlips, 1)
	assert.Equal(t, "Thursday Night", store.createdSlips[0].Name)
}

func TestEnsureWeekSlipsPreservesOneOffSlips(t *testing.T) {
	t.Parallel()

	existing := []db.Slip{
		{
			ID:            99,
			GroupID:       7,
			WeekID:        2026209,
			Name:          "Special One-Off Parlay",
			SlateScope:    db.SlateScopeFullWeek,
			LegsPerMember: 1,
		},
	}

	store := &fakeStore{
		schedules: []db.GroupSlipSchedule{
			{
				ID:            1,
				GroupID:       7,
				Name:          "Main Sunday Slip",
				SlateScope:    db.SlateScopeSundayMain,
				LegsPerMember: 2,
				IsActive:      true,
			},
		},
		slips: existing,
		members: []db.User{
			{ID: 10, Name: "Alice", Active: true},
		},
	}

	result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Special One-Off Parlay", result[0].Name)
	assert.Equal(t, "Main Sunday Slip", result[1].Name)
}

func TestEnsureWeekSlipsWithNoActiveSchedules(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		schedules: []db.GroupSlipSchedule{},
		slips:     []db.Slip{},
	}

	result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.Empty(t, store.createdSlips)
}

func TestEnsureWeekSlipsRefusesGroupWithNoMembers(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		schedules: []db.GroupSlipSchedule{
			{
				ID:            1,
				GroupID:       7,
				Name:          "Main Sunday Slip",
				SlateScope:    db.SlateScopeSundayMain,
				LegsPerMember: 2,
				IsActive:      true,
			},
		},
		slips:   []db.Slip{},
		members: []db.User{},
	}

	_, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.ErrorIs(t, err, db.ErrSlipMembersRequired)
	assert.Empty(t, store.createdSlips)
}

func TestEnsureWeekSlipsReportsStoreErrors(t *testing.T) {
	t.Parallel()

	stop := errors.New("boom")

	// Error fetching schedules
	store := &fakeStore{schedulesErr: stop}
	_, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.ErrorIs(t, err, stop)

	// Error fetching slips
	store = &fakeStore{
		schedules: []db.GroupSlipSchedule{{ID: 1, Name: "Main"}},
		slipsErr:  stop,
	}
	_, err = slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.ErrorIs(t, err, stop)

	// Error fetching games
	store = &fakeStore{
		schedules: []db.GroupSlipSchedule{{ID: 1, Name: "Main"}},
		slips:     []db.Slip{},
		gamesErr:  stop,
	}
	_, err = slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.ErrorIs(t, err, stop)

	// Error fetching members
	store = &fakeStore{
		schedules:  []db.GroupSlipSchedule{{ID: 1, Name: "Main"}},
		slips:      []db.Slip{},
		membersErr: stop,
	}
	_, err = slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.ErrorIs(t, err, stop)

	// Error creating slip
	store = &fakeStore{
		schedules: []db.GroupSlipSchedule{{ID: 1, Name: "Main"}},
		slips:     []db.Slip{},
		members:   []db.User{{ID: 1}},
		createSlipFn: func(context.Context, int64, int64, *int64, string, db.SlateScope, int, []int64) (db.Slip, error) {
			return db.Slip{}, stop
		},
	}
	_, err = slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.ErrorIs(t, err, stop)
}

func TestEnsureWeekSlipsHandlesConcurrentCreationConflict(t *testing.T) {
	t.Parallel()

	schedID := int64(1)
	concurrentSlip := db.Slip{
		ID:            201,
		GroupID:       7,
		WeekID:        2026209,
		ScheduleID:    &schedID,
		Name:          "Main Sunday Slip",
		SlateScope:    db.SlateScopeSundayMain,
		LegsPerMember: 2,
	}

	var store *fakeStore
	store = &fakeStore{
		schedules: []db.GroupSlipSchedule{
			{
				ID:            1,
				GroupID:       7,
				Name:          "Main Sunday Slip",
				SlateScope:    db.SlateScopeSundayMain,
				LegsPerMember: 2,
				IsActive:      true,
			},
		},
		slips: []db.Slip{},
		members: []db.User{
			{ID: 10, Name: "Alice", Active: true},
		},
		createSlipFn: func(context.Context, int64, int64, *int64, string, db.SlateScope, int, []int64) (db.Slip, error) {
			// Simulate unique constraint collision from concurrent creation
			store.slips = []db.Slip{concurrentSlip}
			return db.Slip{}, db.ErrSlipNameTaken
		},
	}

	result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, ensureTestNow)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, concurrentSlip, result[0])
}

func TestEnsureWeekSlipsSkipsSchedulesWithoutUpcomingMatchingGames(t *testing.T) {
	t.Parallel()

	t.Run("playoff week skips thursday and creates weekend and sunday slips", func(t *testing.T) {
		t.Parallel()

		// Wild Card weekend: Saturday 2027-01-09 + Sunday 2027-01-10.
		now := time.Date(2027, 1, 8, 17, 0, 0, 0, time.UTC) // Friday noon ET
		satKick := time.Date(2027, 1, 9, 21, 30, 0, 0, time.UTC)
		sunKick := time.Date(2027, 1, 10, 18, 0, 0, 0, time.UTC)

		store := &fakeStore{
			schedules: []db.GroupSlipSchedule{
				{
					ID:            1,
					GroupID:       7,
					Name:          "Thursday Night",
					SlateScope:    db.SlateScopeThursday,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            2,
					GroupID:       7,
					Name:          "Main Sunday Slip",
					SlateScope:    db.SlateScopeSundayMain,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            3,
					GroupID:       7,
					Name:          "Playoff Weekend",
					SlateScope:    db.SlateScopeWeekend,
					LegsPerMember: 2,
					IsActive:      true,
				},
			},
			slips: []db.Slip{},
			games: []db.Game{
				{ID: "wc-sat", Kickoff: satKick, Status: db.GameScheduled},
				{ID: "wc-sun", Kickoff: sunKick, Status: db.GameScheduled},
			},
			members: []db.User{{ID: 10, Name: "Alice", Active: true}},
		}

		result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026301, now)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "Main Sunday Slip", result[0].Name)
		assert.Equal(t, "Playoff Weekend", result[1].Name)
		require.Len(t, store.createdSlips, 2)
	})

	t.Run("saturday-only playoff week creates weekend slip and skips thursday and sunday", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2027, 1, 15, 17, 0, 0, 0, time.UTC) // Friday noon ET
		satKick := time.Date(2027, 1, 16, 21, 30, 0, 0, time.UTC)

		store := &fakeStore{
			schedules: []db.GroupSlipSchedule{
				{
					ID:            1,
					GroupID:       7,
					Name:          "Thursday Night",
					SlateScope:    db.SlateScopeThursday,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            2,
					GroupID:       7,
					Name:          "Main Sunday Slip",
					SlateScope:    db.SlateScopeSundayMain,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            3,
					GroupID:       7,
					Name:          "Playoff Weekend",
					SlateScope:    db.SlateScopeWeekend,
					LegsPerMember: 2,
					IsActive:      true,
				},
			},
			slips: []db.Slip{},
			games: []db.Game{
				{ID: "div-sat", Kickoff: satKick, Status: db.GameScheduled},
			},
			members: []db.User{{ID: 10, Name: "Alice", Active: true}},
		}

		result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026302, now)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "Playoff Weekend", result[0].Name)
	})

	t.Run("completed week on monday night does not create dead sunday slip", func(t *testing.T) {
		t.Parallel()

		// Monday 2026-11-09 22:00 ET (2026-11-10 03:00 UTC), after Sun & Mon games kicked off.
		now := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
		sunKick := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
		monKick := time.Date(2026, 11, 10, 1, 15, 0, 0, time.UTC)

		store := &fakeStore{
			schedules: []db.GroupSlipSchedule{
				{
					ID:            1,
					GroupID:       7,
					Name:          "Main Sunday Slip",
					SlateScope:    db.SlateScopeSundayMain,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            2,
					GroupID:       7,
					Name:          "Weekend",
					SlateScope:    db.SlateScopeWeekend,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            3,
					GroupID:       7,
					Name:          "Full Week",
					SlateScope:    db.SlateScopeFullWeek,
					LegsPerMember: 2,
					IsActive:      true,
				},
			},
			slips: []db.Slip{},
			games: []db.Game{
				{ID: "sun", Kickoff: sunKick, Status: db.GameFinal},
				{ID: "mon", Kickoff: monKick, Status: db.GameLive},
			},
			members: []db.User{{ID: 10, Name: "Alice", Active: true}},
		}

		result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, now)
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Empty(t, store.createdSlips)
	})

	t.Run("final games with synthetic future kickoff are not treated as upcoming", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
		store := &fakeStore{
			schedules: []db.GroupSlipSchedule{
				{
					ID:            1,
					GroupID:       7,
					Name:          "Main Sunday Slip",
					SlateScope:    db.SlateScopeSundayMain,
					LegsPerMember: 2,
					IsActive:      true,
				},
			},
			slips: []db.Slip{},
			games: []db.Game{
				{ID: "sun", Kickoff: time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC), Status: db.GameFinal},
			},
			members: []db.User{{ID: 10, Name: "Alice", Active: true}},
		}

		result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, now)
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Empty(t, store.createdSlips)
	})

	t.Run("existing slips are preserved when week completes", func(t *testing.T) {
		t.Parallel()

		thuSchedID := int64(1)
		existingThu := db.Slip{
			ID:            55,
			GroupID:       7,
			WeekID:        2026209,
			ScheduleID:    &thuSchedID,
			Name:          "Thursday Night",
			SlateScope:    db.SlateScopeThursday,
			LegsPerMember: 2,
		}
		now := time.Date(2026, 11, 10, 17, 0, 0, 0, time.UTC)
		store := &fakeStore{
			schedules: []db.GroupSlipSchedule{
				{
					ID:            1,
					GroupID:       7,
					Name:          "Thursday Night",
					SlateScope:    db.SlateScopeThursday,
					LegsPerMember: 2,
					IsActive:      true,
				},
				{
					ID:            2,
					GroupID:       7,
					Name:          "Main Sunday Slip",
					SlateScope:    db.SlateScopeSundayMain,
					LegsPerMember: 2,
					IsActive:      true,
				},
			},
			slips: []db.Slip{existingThu},
			games: []db.Game{
				{ID: "thu", Kickoff: time.Date(2026, 11, 6, 1, 15, 0, 0, time.UTC), Status: db.GameFinal},
				{ID: "sun", Kickoff: time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC), Status: db.GameFinal},
			},
			members: []db.User{{ID: 10, Name: "Alice", Active: true}},
		}

		result, err := slips.EnsureWeekSlips(t.Context(), store, 7, 2026209, now)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, existingThu, result[0])
		assert.Empty(t, store.createdSlips)
	})
}

func TestEnsureWeekSlipsDatabaseIntegration(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	owner, err := store.CreateUserWithIdentity(t.Context(), "Owner", "", "", "")
	require.NoError(t, err)
	member, err := store.CreateUserWithIdentity(t.Context(), "Member", "", "", "")
	require.NoError(t, err)

	group, err := store.Create(t.Context(), owner.ID, "The Boys")
	require.NoError(t, err)
	_, err = store.JoinByToken(t.Context(), member.ID, group.InviteToken)
	require.NoError(t, err)

	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	// Group already has "Main Sunday Slip" from Create; add Thursday Night
	thuSched, err := store.CreateGroupSlipSchedule(t.Context(), group.ID, "Thursday Night", db.SlateScopeThursday, 2)
	require.NoError(t, err)
	assert.True(t, thuSched.IsActive)

	// First call creates both slips
	slipsList, err := slips.EnsureWeekSlips(t.Context(), store, group.ID, week.ID, ensureTestNow)
	require.NoError(t, err)
	require.Len(t, slipsList, 2)

	assert.Equal(t, "Thursday Night", slipsList[0].Name)
	assert.Equal(t, db.SlateScopeThursday, slipsList[0].SlateScope)
	assert.Equal(t, "Main Sunday Slip", slipsList[1].Name)
	assert.Equal(t, db.SlateScopeSundayMain, slipsList[1].SlateScope)

	// Verify both users are members of both slips
	for _, s := range slipsList {
		members, err := store.SlipMembers(t.Context(), s.ID)
		require.NoError(t, err)
		require.Len(t, members, 2)
	}

	// Second call is idempotent and creates nothing new
	slipsList2, err := slips.EnsureWeekSlips(t.Context(), store, group.ID, week.ID, ensureTestNow)
	require.NoError(t, err)
	require.Len(t, slipsList2, 2)
	assert.Equal(t, slipsList[0].ID, slipsList2[0].ID)
	assert.Equal(t, slipsList[1].ID, slipsList2[1].ID)
}
