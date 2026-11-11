package sandbox_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/sandbox"
)

func TestStateStoreStartsEmptyThenRoundTrips(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	state := sandbox.NewStateStore(database)
	require.NoError(t, state.Ensure(t.Context()))

	_, err = state.Load(t.Context())
	require.ErrorIs(t, err, sandbox.ErrStateNotFound)

	want := sandbox.State{Stage: sandbox.Morning, Now: sandbox.MorningTime()}
	require.NoError(t, state.Save(t.Context(), want))

	got, err := state.Load(t.Context())
	require.NoError(t, err)
	assert.Equal(t, want, got)

	later := sandbox.State{Stage: sandbox.Stage("early-loss"), Now: want.Now.Add(3 * time.Hour)}
	require.NoError(t, state.Save(t.Context(), later))

	got, err = state.Load(t.Context())
	require.NoError(t, err)
	assert.Equal(t, later, got, "saving replaces the one row")
}

func TestStateStoreRejectsABlankStage(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	state := sandbox.NewStateStore(database)
	require.NoError(t, state.Ensure(t.Context()))
	err = state.Save(t.Context(), sandbox.State{Now: sandbox.MorningTime()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs a stage")
}

func TestScenarioReadsAndWritesOneClock(t *testing.T) {
	t.Parallel()

	start := sandbox.MorningTime()
	scenario := sandbox.NewScenario(sandbox.Morning, start)
	assert.Equal(t, sandbox.Morning, scenario.Stage())
	assert.Equal(t, start, scenario.Now())

	next := sandbox.State{Stage: sandbox.Stage("all-clear"), Now: start.Add(5 * time.Hour)}
	scenario.Set(next)
	assert.Equal(t, next, scenario.State())
}
