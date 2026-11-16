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
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
)

// openTestStore opens a migrated database in a temp dir.
func openTestStore(t *testing.T) *db.Store {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return store
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

func TestRunOnceWaitsForEverySource(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store := openTestStore(t)
	game := storedGame(0, now)
	lines := &fakeLines{
		events: []odds.Event{{
			ID: "evt-1", HomeTeam: game.HomeName, AwayTeam: game.AwayName,
			CommenceTime: game.Kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID: "evt-1", Book: "draftkings", MarketType: odds.MarketML,
			Team: game.HomeName, Side: odds.SideTeam, Price: -155,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: espnWeek()},
		Forecast:   fakeForecast{forecast: weather.Forecast{TempF: 40, WindMPH: 8}},
		Lines:      lines,
		Now:        func() time.Time { return now },
		Logger:     zerolog.Nop(),
	})

	require.NoError(t, service.RunOnce(t.Context()), "the pass returns after every source finishes")

	week, err := store.LatestWeek(t.Context())
	require.NoError(t, err)
	games, err := store.GamesForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, games, 1)
	assert.Equal(t, db.GameLive, games[0].Status)
	require.NotNil(t, games[0].TempF)
	assert.InDelta(t, 40.0, *games[0].TempF, 1e-9)

	for _, key := range []string{ScoreboardSource, WeatherSource(games[0].ID), OddsLinesSource} {
		state, err := store.SyncState(t.Context(), key)
		require.NoError(t, err, key)
		assert.Equal(t, now, state.FetchedAt, key)
	}

	rows, err := store.OddsForGame(t.Context(), games[0].ID)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	assert.Equal(t, 1, lines.lineCalls)
}

func TestRunOnceReturnsJoinedFailures(t *testing.T) {
	t.Parallel()

	now := novemberTime(7, 11, 0)
	store := openTestStore(t)
	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: espnWeek()},
		Forecast:   fakeForecast{err: errors.New("forecast is down")},
		Lines:      &fakeLines{eventsErr: errors.New("odds are down")},
		Now:        func() time.Time { return now },
		Logger:     zerolog.Nop(),
	})

	err := service.RunOnce(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forecast is down")
	assert.Contains(t, err.Error(), "odds are down")
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

func TestClearBackoffForgetsEveryPause(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: novemberTime(4, 12, 0)}
	service := testService(openTestStore(t), clock.Now)

	var (
		runs    atomic.Int32
		failing atomic.Bool
	)
	failing.Store(true)
	src := source{
		key: "test:clear-all",
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

	// Without clearing, the same clock is still inside the five-minute pause.
	failing.Store(false)
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(1), runs.Load())

	service.ClearBackoff()
	service.runSources(t.Context(), clock.Now(), []source{src})
	service.wait()
	assert.Equal(t, int32(2), runs.Load(), "ClearBackoff releases every paused source")
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
