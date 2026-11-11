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

func TestStateStoreRejectsAnUnknownStage(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	state := sandbox.NewStateStore(database)
	require.NoError(t, state.Ensure(t.Context()))

	err = state.Save(t.Context(), sandbox.State{
		Stage: sandbox.Stage("playoffs"),
		Now:   sandbox.MorningTime(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown stage "playoffs"`)

	_, err = database.ExecContext(t.Context(),
		"INSERT INTO sandbox_state (id, stage, now) VALUES (1, ?, ?)",
		"playoffs", sandbox.MorningTime().Format(time.RFC3339Nano))
	require.NoError(t, err)
	_, err = state.Load(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown stage "playoffs"`)
}

func TestStageNamesAndTimesAreClosed(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []sandbox.Stage{
		sandbox.Morning, sandbox.EarlyLoss, sandbox.AllClear, sandbox.SundayComplete,
	}, sandbox.Stages())
	for _, stage := range sandbox.Stages() {
		byName, ok := sandbox.StageByName(string(stage))
		assert.True(t, ok, stage)
		assert.Equal(t, stage, byName)

		when, ok := sandbox.StageTime(stage)
		require.True(t, ok, stage)
		assert.False(t, when.IsZero(), stage)

		plan, ok := sandbox.PlanFor(stage)
		require.True(t, ok, stage)
		assert.Equal(t, stage, plan.Stage)
		assert.Equal(t, when, plan.Now)
	}

	_, ok := sandbox.StageByName(" playoffs ")
	assert.False(t, ok)
	_, ok = sandbox.StageTime(sandbox.Stage("playoffs"))
	assert.False(t, ok)
	_, ok = sandbox.PlanFor(sandbox.Stage("playoffs"))
	assert.False(t, ok)
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
