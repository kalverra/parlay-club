package db_test

import (
	"testing"

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
