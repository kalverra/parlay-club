package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestSyncStateReportsAMissingSource(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SyncState(t.Context(), "espn:scoreboard")
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)
}

func TestRecordSyncCreatesAndUpdatesTheRow(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	remaining := 500

	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", first, &remaining))
	state, err := store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, "odds:lines", state.Source)
	assert.Equal(t, first, state.FetchedAt)
	require.NotNil(t, state.OddsCreditsRemaining)
	assert.Equal(t, 500, *state.OddsCreditsRemaining)

	second := first.Add(time.Hour)
	remaining = 480
	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", second, &remaining))

	state, err = store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, second, state.FetchedAt)
	assert.Equal(t, 480, *state.OddsCreditsRemaining)

	require.NoError(t, store.RecordSync(t.Context(), "espn:scoreboard", second, nil))
	state, err = store.SyncState(t.Context(), "espn:scoreboard")
	require.NoError(t, err)
	assert.Nil(t, state.OddsCreditsRemaining)
}

func TestSetSyncCreditsLeavesFetchedAtAlone(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	fetchedAt := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	remaining := 500
	require.NoError(t, store.RecordSync(t.Context(), "odds:lines", fetchedAt, &remaining))

	require.NoError(t, store.SetSyncCredits(t.Context(), "odds:lines", 450))

	state, err := store.SyncState(t.Context(), "odds:lines")
	require.NoError(t, err)
	assert.Equal(t, fetchedAt, state.FetchedAt)
	assert.Equal(t, 450, *state.OddsCreditsRemaining)

	// A source with no row is untouched; there is nothing to refresh yet.
	require.NoError(t, store.SetSyncCredits(t.Context(), "odds:props:x", 450))
	_, err = store.SyncState(t.Context(), "odds:props:x")
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)
}

func TestGamesForWeekReturnsKickoffOrder(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)

	late := testSlateGame(week)
	late.ID = "late"
	late.Kickoff = late.Kickoff.Add(3 * time.Hour)
	early := testSlateGame(week)
	early.ID = "early"

	_, err := store.UpsertGame(t.Context(), late)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), early)
	require.NoError(t, err)

	games, err := store.GamesForWeek(t.Context(), week.ID)
	require.NoError(t, err)
	require.Len(t, games, 2)
	assert.Equal(t, "early", games[0].ID)
	assert.Equal(t, "late", games[1].ID)
}

func TestSetGameWeatherReportsRealChanges(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	first := time.Date(2026, time.November, 7, 13, 0, 0, 0, time.UTC)
	changed, err := store.SetGameWeather(t.Context(), game.ID, 48, 18, 29, 0.08, first)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(48.0), stored.TempF)
	assert.Equal(t, new(18.0), stored.WindMPH)
	assert.Equal(t, new(29.0), stored.WindGustMPH)
	assert.Equal(t, new(0.08), stored.PrecipIN)
	assert.Equal(t, first, stored.UpdatedAt)

	changed, err = store.SetGameWeather(t.Context(), game.ID, 48, 18, 29, 0.08, first.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, changed)
	stored, err = store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, first, stored.UpdatedAt, "an unchanged forecast leaves updated_at alone")

	changed, err = store.SetGameWeather(t.Context(), game.ID, 40, 22, 35, 0, first.Add(2*time.Hour))
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestSetGameWeatherReportsAMissingGame(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SetGameWeather(t.Context(), "missing", 48, 18, 29, 0, time.Now())
	require.ErrorIs(t, err, db.ErrGameNotFound)
}

func TestSetGameOddsEventReportsRealChanges(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	changed, err := store.SetGameOddsEvent(t.Context(), game.ID, "evt-1")
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new("evt-1"), stored.OddsEventID)

	changed, err = store.SetGameOddsEvent(t.Context(), game.ID, "evt-1")
	require.NoError(t, err)
	assert.False(t, changed)

	changed, err = store.SetGameOddsEvent(t.Context(), game.ID, "evt-2")
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestUpsertOddsInsertsUpdatesAndLists(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)
	fetchedAt := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)

	rows := []db.Odds{
		{
			GameID: game.ID, Book: "draftkings", MarketType: "SPREAD", Team: "BUF",
			Side: "TEAM", Line: new(-2.5), Price: -110, FetchedAt: fetchedAt,
		},
		{
			GameID: game.ID, Book: "draftkings", MarketType: "ML", Team: "BUF",
			Side: "TEAM", Price: -155, FetchedAt: fetchedAt,
		},
	}
	changed, err := store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	assert.Equal(t, "ML", stored[0].MarketType, "the list is ordered by market")
	assert.Equal(t, "SPREAD", stored[1].MarketType)

	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.False(t, changed, "the same prices at the same fetch time change nothing")

	rows[1].Price = -160
	changed, err = store.UpsertOdds(t.Context(), rows)
	require.NoError(t, err)
	assert.True(t, changed)
}
