package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// testSlateGame builds the game the slate sync would write for one ESPN event.
func testSlateGame(week db.Week) db.Game {
	return db.Game{
		ID:         "401772001",
		WeekID:     week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
		Status:     db.GameScheduled,
		VenueID:    new("3915"),
		VenueName:  new("Highmark Stadium"),
		UpdatedAt:  time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC),
	}
}

func TestUpsertGameCreatesTheRow(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	week := createTestWeek(t, store, 9)
	want := testSlateGame(week)

	game, err := store.UpsertGame(t.Context(), want)
	require.NoError(t, err)
	assert.Equal(t, want, game)

	stored, err := store.GameByID(t.Context(), want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, stored)

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM games").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestUpsertGameRerunChangesNothing(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game := testSlateGame(week)

	_, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)

	// Later syncs own these columns; the slate sync must not touch them.
	_, err = database.ExecContext(t.Context(), `
		UPDATE games SET
			odds_event_id = 'evt-1',
			temp_f = 55.5,
			wind_mph = 12,
			wind_gust_mph = 22,
			precip_in = 0.1,
			injuries_json = '[]'
		WHERE id = ?`, game.ID)
	require.NoError(t, err)

	before, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)

	// A second fetch of the same slate arrives later, but nothing changed.
	game.UpdatedAt = game.UpdatedAt.Add(time.Hour)
	again, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, before, again, "an unchanged sync leaves every column alone")

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM games").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestUpsertGameUpdatesOnlyTheSlateFields(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game := testSlateGame(week)

	_, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), `
		UPDATE games SET
			odds_event_id = 'evt-1',
			temp_f = 55.5,
			wind_mph = 12,
			wind_gust_mph = 22,
			precip_in = 0.1,
			injuries_json = '[]'
		WHERE id = ?`, game.ID)
	require.NoError(t, err)

	game.Status = db.GameLive
	game.HomeScore = new(14)
	game.AwayScore = new(7)
	game.Kickoff = game.Kickoff.Add(10 * time.Minute)
	game.UpdatedAt = game.UpdatedAt.Add(2 * time.Hour)

	updated, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, db.GameLive, updated.Status)
	assert.Equal(t, new(14), updated.HomeScore)
	assert.Equal(t, new(7), updated.AwayScore)
	assert.Equal(t, game.Kickoff, updated.Kickoff)
	assert.Equal(t, game.UpdatedAt, updated.UpdatedAt)
	assert.Equal(t, new("evt-1"), updated.OddsEventID)
	assert.Equal(t, new(55.5), updated.TempF)
	assert.Equal(t, new(12.0), updated.WindMPH)
	assert.Equal(t, new(22.0), updated.WindGustMPH)
	assert.Equal(t, new(0.1), updated.PrecipIN)
	assert.Equal(t, new("[]"), updated.InjuriesJSON)

	// Repeating the changed game with a later fetch time is still a no-op.
	game.UpdatedAt = game.UpdatedAt.Add(time.Hour)
	again, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, updated, again)
}

func TestUpsertGameRefusesAnUnknownStatus(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game := testSlateGame(week)
	game.Status = db.GameStatus("POSTPONED")

	_, err := store.UpsertGame(t.Context(), game)
	require.ErrorIs(t, err, db.ErrGameStatusInvalid)
}

func TestGameByIDReportsAMissingGame(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.GameByID(t.Context(), "401772001")
	require.ErrorIs(t, err, db.ErrGameNotFound)
}
