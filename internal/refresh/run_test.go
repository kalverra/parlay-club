package refresh

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// openTestStore opens a migrated database in a temp dir.
func openTestStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	return db.NewStore(database)
}

// testService builds a service with no clients; run tests never touch them.
func testService(store *db.Store, now Clock) *Service {
	return New(Deps{Store: store, Now: now, Logger: zerolog.Nop()})
}

// fakeClock is a Clock tests move by hand.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now returns the clock's current time.
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the clock forward.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// alwaysDue is a due rule that never inspects the state.
func alwaysDue(context.Context, db.SyncState) (bool, error) {
	return true, nil
}

func TestRunSourcesOnlyRunsDueSources(t *testing.T) {
	t.Parallel()

	service := testService(openTestStore(t), nil)
	var runs atomic.Int32
	src := source{
		key: "test:not-due",
		due: func(context.Context, db.SyncState) (bool, error) {
			return false, nil
		},
		run: func(context.Context) (bool, error) {
			runs.Add(1)
			return true, nil
		},
	}

	service.runSources(t.Context(), time.Now(), []source{src})
	service.wait()

	assert.Zero(t, runs.Load())
}

func TestRunSourcesNeverRunsOneSourceTwiceAtOnce(t *testing.T) {
	t.Parallel()

	service := testService(openTestStore(t), nil)

	var (
		runs        atomic.Int32
		running     atomic.Int32
		maxRunning  atomic.Int32
		release     = make(chan struct{})
		entered     = make(chan struct{})
		enteredOnce sync.Once
	)
	src := source{
		key: "test:single-flight",
		due: alwaysDue,
		run: func(context.Context) (bool, error) {
			runs.Add(1)
			current := running.Add(1)
			for {
				old := maxRunning.Load()
				if current <= old || maxRunning.CompareAndSwap(old, current) {
					break
				}
			}
			enteredOnce.Do(func() { close(entered) })
			<-release
			running.Add(-1)
			return true, nil
		},
	}

	var callers sync.WaitGroup
	for range 12 {
		callers.Go(func() {
			service.runSources(t.Context(), time.Now(), []source{src})
		})
	}

	<-entered
	// Give the other callers time to join the in-flight call before the first
	// one finishes. A single-flight makes them wait, so no second run starts.
	time.Sleep(250 * time.Millisecond)
	close(release)
	callers.Wait()
	service.wait()

	assert.Equal(t, int32(1), maxRunning.Load(), "a source must never run twice at once")
	assert.Equal(t, int32(1), runs.Load(), "callers join the in-flight run")
}

func TestFailureBacksOffForFiveMinutes(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: novemberTime(4, 12, 0)}
	service := testService(openTestStore(t), clock.Now)

	var runs atomic.Int32
	src := source{
		key: "test:backoff",
		due: alwaysDue,
		run: func(context.Context) (bool, error) {
			runs.Add(1)
			return false, errors.New("source is down")
		},
	}

	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	require.Equal(t, int32(1), runs.Load())

	clock.Advance(4 * time.Minute)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(1), runs.Load(), "a failure pauses the source for five minutes")

	clock.Advance(time.Minute)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(2), runs.Load(), "the source retries after the backoff")
}

func TestSuccessClearsTheBackoff(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: novemberTime(4, 12, 0)}
	service := testService(openTestStore(t), clock.Now)

	var (
		runs    atomic.Int32
		failing atomic.Bool
	)
	failing.Store(true)
	src := source{
		key: "test:recover",
		due: alwaysDue,
		run: func(context.Context) (bool, error) {
			runs.Add(1)
			if failing.Load() {
				return false, errors.New("source is down")
			}
			return true, nil
		},
	}

	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	require.Equal(t, int32(1), runs.Load())

	clock.Advance(5 * time.Minute)
	failing.Store(false)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(2), runs.Load())

	// The success cleared the pause, so the very next pass runs again.
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(3), runs.Load())
}
