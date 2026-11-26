package reminders_test

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/reminders"
)

// newReminderStore opens a migrated database for a reminder test.
func newReminderStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return store
}

// seedReminderSlip builds one open full-week slip with two members and one
// game per kickoff.
func seedReminderSlip(t *testing.T, kickoffs []time.Time) (*db.Store, db.Slip, []db.User) {
	t.Helper()

	store := newReminderStore(t)
	owner, err := store.CreateUserWithIdentity(t.Context(), "Ada", "", "", "")
	require.NoError(t, err)
	friend, err := store.CreateUserWithIdentity(t.Context(), "Ben", "", "", "")
	require.NoError(t, err)

	group, err := store.Create(t.Context(), owner.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	for i, kickoff := range kickoffs {
		_, err := store.UpsertGame(t.Context(), db.Game{
			ID:        "game-" + strconv.Itoa(i),
			WeekID:    week.ID,
			HomeTeam:  "BUF",
			AwayTeam:  "MIA",
			HomeName:  "Buffalo Bills",
			AwayName:  "Miami Dolphins",
			Kickoff:   kickoff,
			Status:    db.GameScheduled,
			UpdatedAt: kickoff.Add(-24 * time.Hour),
		})
		require.NoError(t, err)
	}

	slip, err := store.CreateSlip(
		t.Context(), group.ID, week.ID, nil, "Sunday Slate",
		db.SlateScopeFullWeek, 2, []int64{owner.ID, friend.ID},
	)
	require.NoError(t, err)
	return store, slip, []db.User{owner, friend}
}

// scheduled is one enqueue call.
type scheduled struct {
	slipID int64
	start  time.Time
	at     time.Time
}

// recordingScheduler records enqueues and returns a prepared error.
type recordingScheduler struct {
	calls []scheduled
	err   error
}

func (s *recordingScheduler) Enqueue(_ context.Context, slipID int64, start, at time.Time) error {
	s.calls = append(s.calls, scheduled{slipID: slipID, start: start, at: at})
	return s.err
}

func syncService(store *db.Store, scheduler reminders.Scheduler, now time.Time) *reminders.Service {
	return reminders.New(reminders.Deps{
		Store:     store,
		Sender:    &recordingSender{enabled: true},
		Scheduler: scheduler,
		Now:       func() time.Time { return now },
		Logger:    zerolog.Nop(),
	})
}

func TestSyncSchedulesANewSlipOnce(t *testing.T) {
	t.Parallel()

	now := sundayKickoff.Add(-24 * time.Hour)
	store, slip, _ := seedReminderSlip(t, []time.Time{sundayKickoff})
	scheduler := &recordingScheduler{}
	service := syncService(store, scheduler, now)

	require.NoError(t, service.Sync(t.Context(), now))
	require.Len(t, scheduler.calls, 1)
	assert.Equal(t, slip.ID, scheduler.calls[0].slipID)
	assert.True(t, sundayKickoff.Equal(scheduler.calls[0].start))
	assert.True(t, sundayKickoff.Add(-reminders.LeadTime).Equal(scheduler.calls[0].at))

	tasks, err := store.ReminderTasks(t.Context())
	require.NoError(t, err)
	require.Contains(t, tasks, slip.ID)
	assert.True(t, sundayKickoff.Equal(tasks[slip.ID].Start))

	require.NoError(t, service.Sync(t.Context(), now))
	assert.Len(t, scheduler.calls, 1, "the stored start keeps a second pass quiet")
}

func TestSyncEnqueuesAgainWhenTheStartMoves(t *testing.T) {
	t.Parallel()

	now := sundayKickoff.Add(-24 * time.Hour)
	store, slip, _ := seedReminderSlip(t, []time.Time{sundayKickoff})
	scheduler := &recordingScheduler{}
	service := syncService(store, scheduler, now)

	require.NoError(t, service.Sync(t.Context(), now))
	require.Len(t, scheduler.calls, 1)

	moved := sundayKickoff.Add(3 * time.Hour)
	games, err := store.GamesForWeek(t.Context(), slip.WeekID)
	require.NoError(t, err)
	require.Len(t, games, 1)
	games[0].Kickoff = moved
	_, err = store.UpsertGame(t.Context(), games[0])
	require.NoError(t, err)

	require.NoError(t, service.Sync(t.Context(), now))
	require.Len(t, scheduler.calls, 2)
	assert.True(t, moved.Equal(scheduler.calls[1].start))
	assert.True(t, moved.Add(-reminders.LeadTime).Equal(scheduler.calls[1].at))

	tasks, err := store.ReminderTasks(t.Context())
	require.NoError(t, err)
	assert.True(t, moved.Equal(tasks[slip.ID].Start), "the row follows the new start")
}

func TestSyncSkipsStartsOutsideTheHorizonOrInsideTheLead(t *testing.T) {
	t.Parallel()

	t.Run("more than 30 days out", func(t *testing.T) {
		t.Parallel()

		now := sundayKickoff
		start := now.Add(31 * 24 * time.Hour)
		store, _, _ := seedReminderSlip(t, []time.Time{start})
		scheduler := &recordingScheduler{}
		service := syncService(store, scheduler, now)

		require.NoError(t, service.Sync(t.Context(), now))
		assert.Empty(t, scheduler.calls)

		tasks, err := store.ReminderTasks(t.Context())
		require.NoError(t, err)
		assert.Empty(t, tasks)
	})

	t.Run("inside 20 minutes", func(t *testing.T) {
		t.Parallel()

		now := sundayKickoff
		store, _, _ := seedReminderSlip(t, []time.Time{now.Add(20 * time.Minute)})
		scheduler := &recordingScheduler{}
		service := syncService(store, scheduler, now)

		require.NoError(t, service.Sync(t.Context(), now))
		assert.Empty(t, scheduler.calls, "the task would fire immediately, and the person is in the app")
	})
}

func TestSyncTreatsAlreadyExistsAsSuccess(t *testing.T) {
	t.Parallel()

	now := sundayKickoff.Add(-24 * time.Hour)
	store, slip, _ := seedReminderSlip(t, []time.Time{sundayKickoff})
	scheduler := &recordingScheduler{err: reminders.ErrAlreadyExists}
	service := syncService(store, scheduler, now)

	require.NoError(t, service.Sync(t.Context(), now))
	require.Len(t, scheduler.calls, 1)

	tasks, err := store.ReminderTasks(t.Context())
	require.NoError(t, err)
	assert.Contains(t, tasks, slip.ID)

	require.NoError(t, service.Sync(t.Context(), now))
	assert.Len(t, scheduler.calls, 1)
}

func TestSyncRetriesAnEnqueueFailure(t *testing.T) {
	t.Parallel()

	now := sundayKickoff.Add(-24 * time.Hour)
	store, slip, _ := seedReminderSlip(t, []time.Time{sundayKickoff})
	scheduler := &recordingScheduler{err: errors.New("queue is down")}
	service := syncService(store, scheduler, now)

	require.Error(t, service.Sync(t.Context(), now))
	require.Len(t, scheduler.calls, 1)

	tasks, err := store.ReminderTasks(t.Context())
	require.NoError(t, err)
	assert.Empty(t, tasks, "a failed enqueue leaves no row")

	scheduler.err = nil
	require.NoError(t, service.Sync(t.Context(), now))
	assert.Len(t, scheduler.calls, 2, "the next pass retries")

	tasks, err = store.ReminderTasks(t.Context())
	require.NoError(t, err)
	assert.Contains(t, tasks, slip.ID)
}

func TestSyncIsANoOpWithoutASchedulerOrPush(t *testing.T) {
	t.Parallel()

	now := sundayKickoff.Add(-24 * time.Hour)
	store, slip, _ := seedReminderSlip(t, []time.Time{sundayKickoff})

	noScheduler := syncService(store, nil, now)
	require.NoError(t, noScheduler.Sync(t.Context(), now))

	scheduler := &recordingScheduler{}
	off := reminders.New(reminders.Deps{
		Store:     store,
		Sender:    &recordingSender{enabled: false},
		Scheduler: scheduler,
		Now:       func() time.Time { return now },
		Logger:    zerolog.Nop(),
	})
	require.NoError(t, off.Sync(t.Context(), now))
	assert.Empty(t, scheduler.calls)

	tasks, err := store.ReminderTasks(t.Context())
	require.NoError(t, err)
	assert.Empty(t, tasks)
	assert.NotZero(t, slip.ID)
}
