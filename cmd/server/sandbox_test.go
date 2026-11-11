package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/live"
	"github.com/kalverra/parlay-club/internal/refresh"
	"github.com/kalverra/parlay-club/internal/sandbox"
)

// sandboxFixtures is the repository root the fixture registry reads from.
func sandboxFixtures(t *testing.T) fs.FS {
	t.Helper()

	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	return os.DirFS(root)
}

func TestSandboxBootstrapReplaysTheMorningOffline(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	hub := &live.Hub{}
	fixtures := sandboxFixtures(t)

	runtime, err := newSandboxRuntime(t.Context(), database, store, hub, fixtures, zerolog.Nop())
	require.NoError(t, err)

	assert.Equal(t, sandbox.Morning, runtime.scenario.Stage())
	assert.Equal(t, sandbox.MorningTime(), runtime.scenario.Now())
	assert.Equal(t, sandbox.Morning, runtime.transport.Stage())
	assert.Same(t, runtime.transport, runtime.client.Transport,
		"every sandbox client is on the replay, with no network fallback")
	assert.Equal(t, ingestTimeout, runtime.client.Timeout)

	week, err := store.LatestWeek(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2026, week.Season)
	assert.Equal(t, 9, week.WeekNum)
	games, err := store.GamesForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, games, 14, "the whole Sunday and Monday slate is replayed")

	weatherGames := 0
	for _, game := range games {
		assert.False(t, runtime.scenario.Now().After(game.Kickoff), game.ID)
		if game.Indoor {
			continue
		}
		require.NotNil(t, game.TempF, "outdoor game %s has a forecast", game.ID)
		weatherGames++
		if _, err := store.SyncState(t.Context(), refresh.WeatherSource(game.ID)); err != nil {
			t.Errorf("outdoor game %s has no weather sync state: %v", game.ID, err)
		}
	}
	assert.Equal(t, 10, weatherGames)

	linesState, err := store.SyncState(t.Context(), refresh.OddsLinesSource)
	require.NoError(t, err)
	require.NotNil(t, linesState.OddsCreditsRemaining)
	assert.Greater(t, *linesState.OddsCreditsRemaining, refresh.OddsLinesFloor)

	featured, err := store.GameByID(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	require.NotNil(t, featured.OddsEventID)
	assert.Equal(t, sandbox.FeaturedEventID, *featured.OddsEventID)
	rows, err := store.OddsForGame(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	assert.NotEmpty(t, rows, "the featured game has matched prices")

	_, hasProjection, err := store.NfeloForGame(t.Context(), sandbox.FeaturedGameID)
	require.NoError(t, err)
	assert.True(t, hasProjection, "nfelo projections landed for the slate")

	responses, err := store.ListAPIResponses(t.Context(), db.APIResponseFilter{})
	require.NoError(t, err)
	assert.Empty(t, responses, "replayed calls never enter the response archive")

	saved, err := runtime.state.Load(t.Context())
	require.NoError(t, err)
	assert.Equal(t, sandbox.State{Stage: sandbox.Morning, Now: sandbox.MorningTime()}, saved)
}

func TestSandboxRestartRestoresStageAndClock(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	fixtures := sandboxFixtures(t)

	first, err := newSandboxRuntime(t.Context(), database, store, &live.Hub{}, fixtures, zerolog.Nop())
	require.NoError(t, err)

	weather, err := store.SyncState(t.Context(), refresh.ScoreboardSource)
	require.NoError(t, err)
	marker, err := store.CreateUserWithIdentity(
		t.Context(), "Marker", "marker@example.com", "dev:marker@example.com", "")
	require.NoError(t, err)

	restored := sandbox.State{
		Stage: sandbox.Stage("all-clear"),
		Now:   sandbox.MorningTime().Add(4 * time.Hour),
	}
	require.NoError(t, first.state.Save(t.Context(), restored))

	second, err := newSandboxRuntime(t.Context(), database, store, &live.Hub{}, fixtures, zerolog.Nop())
	require.NoError(t, err)

	assert.Equal(t, restored, second.scenario.State(), "the stage and clock survive a restart")
	assert.Equal(t, restored.Stage, second.transport.Stage())

	after, err := store.SyncState(t.Context(), refresh.ScoreboardSource)
	require.NoError(t, err)
	assert.Equal(t, weather.FetchedAt, after.FetchedAt,
		"a restart does not rebuild or re-fetch the morning")

	stillThere, err := store.UserByID(t.Context(), marker.ID)
	require.NoError(t, err)
	assert.Equal(t, marker.Name, stillThere.Name, "manual edits and people survive a restart")
}

func TestSandboxAuthNeverBuildsGoogle(t *testing.T) {
	t.Parallel()

	provider, devLogin, err := authSettings(t.Context(), config.Config{
		AppEnv:             config.EnvDev,
		Sandbox:            true,
		GoogleClientID:     "real-looking-client-id",
		GoogleClientSecret: "real-looking-client-secret",
		BaseURL:            "https://parlay.example.com",
	})
	require.NoError(t, err)
	assert.Nil(t, provider, "sandbox startup never contacts the Google issuer")
	assert.True(t, devLogin, "the dev sign-in form is always available in the sandbox")

	provider, devLogin, err = authSettings(t.Context(), config.Config{AppEnv: config.EnvDev})
	require.NoError(t, err)
	assert.Nil(t, provider)
	assert.True(t, devLogin, "normal dev with no Google values keeps the dev form")
}

func TestSandboxStartupLeavesTheNormalDevDatabaseAlone(t *testing.T) {
	t.Parallel()

	normalPath := filepath.Join(t.TempDir(), "parlay.db")
	sentinel := []byte("normal dev database sentinel")
	require.NoError(t, os.WriteFile(normalPath, sentinel, 0o600))

	database, err := db.Open(filepath.Join(t.TempDir(), "sunday-sandbox.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	fixtures := sandboxFixtures(t)
	_, err = newSandboxRuntime(
		t.Context(), database, db.NewStore(database), &live.Hub{}, fixtures, zerolog.Nop(),
	)
	require.NoError(t, err)

	//nolint:gosec // the test reads the file it just wrote in its own temp dir.
	unchanged, err := os.ReadFile(normalPath)
	require.NoError(t, err)
	assert.Equal(t, sentinel, unchanged, "sandbox startup never opens the normal dev file")
}
