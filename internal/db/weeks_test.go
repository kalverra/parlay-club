package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestUpsertWeekCreatesThenReusesTheRow(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)

	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	assert.Equal(t, db.Week{ID: 2026209, Season: 2026, SeasonType: 2, WeekNum: 9}, week)

	again, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	assert.Equal(t, week, again, "a second upsert answers the same row")

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM weeks").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestUpsertWeekKeepsSeasonTypesApart(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	regular, err := store.UpsertWeek(t.Context(), 2026, 2, 18)
	require.NoError(t, err)
	postseason, err := store.UpsertWeek(t.Context(), 2026, 3, 1)
	require.NoError(t, err)

	assert.Equal(t, int64(2026218), regular.ID)
	assert.Equal(t, int64(2026301), postseason.ID)
	assert.NotEqual(t, regular.ID, postseason.ID)
}

func TestWeekByIDReportsAMissingWeek(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.WeekByID(t.Context(), 2026209)
	require.ErrorIs(t, err, db.ErrWeekNotFound)
}

func TestWeeksListsNewestFirstAndLatestWeekPicksTheFirst(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	earlier, err := store.UpsertWeek(t.Context(), 2025, 3, 4)
	require.NoError(t, err)
	later, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	weeks, err := store.Weeks(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []db.Week{later, earlier}, weeks)

	latest, err := store.LatestWeek(t.Context())
	require.NoError(t, err)
	assert.Equal(t, later, latest)
}

func TestLatestWeekReportsAnEmptyTable(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.LatestWeek(t.Context())
	require.ErrorIs(t, err, db.ErrWeekNotFound)
}

func TestUpcomingWeeksFiltersAndOrdersSoonestFirst(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	wkEmpty, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	wkPast, err := store.UpsertWeek(t.Context(), 2026, 2, 10)
	require.NoError(t, err)
	wkSoon, err := store.UpsertWeek(t.Context(), 2026, 2, 11)
	require.NoError(t, err)
	wkLater, err := store.UpsertWeek(t.Context(), 2026, 2, 12)
	require.NoError(t, err)
	_ = wkEmpty

	addGame := func(id string, weekID int64, kickoff time.Time, status db.GameStatus) {
		_, err := store.UpsertGame(t.Context(), db.Game{
			ID:         id,
			WeekID:     weekID,
			HomeTeam:   "BUF",
			AwayTeam:   "KC",
			HomeName:   "Buffalo Bills",
			AwayName:   "Kansas City Chiefs",
			HomeESPNID: "2",
			AwayESPNID: "12",
			Kickoff:    kickoff,
			Status:     status,
			UpdatedAt:  now,
		})
		require.NoError(t, err)
	}

	addGame("g-past", wkPast.ID, now.Add(-24*time.Hour), db.GameFinal)
	addGame("g-soon-past", wkSoon.ID, now.Add(-2*time.Hour), db.GameLive)
	addGame("g-soon-future", wkSoon.ID, now.Add(4*time.Hour), db.GameScheduled)
	addGame("g-later", wkLater.ID, now.Add(7*24*time.Hour), db.GameScheduled)

	upcoming, err := store.UpcomingWeeks(t.Context(), now)
	require.NoError(t, err)
	assert.Equal(t, []db.Week{wkSoon, wkLater}, upcoming)
}
