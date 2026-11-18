package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestUpsertNfeloGamesInsertsAndKeepsUnchangedRows(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	first := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	row := db.NfeloGame{
		GameID:      game.ID,
		HomeSpread:  -7.5,
		HomeWinProb: 0.72,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   18.5,
		AwayQBAdj:   -4.25,
		UpdatedAt:   first,
	}

	changed, err := store.UpsertNfeloGames(t.Context(), []db.NfeloGame{row})
	require.NoError(t, err)
	assert.True(t, changed)

	stored, ok, err := store.NfeloForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, row, stored)

	again := row
	again.UpdatedAt = first.Add(time.Hour)
	changed, err = store.UpsertNfeloGames(t.Context(), []db.NfeloGame{again})
	require.NoError(t, err)
	assert.False(t, changed, "the same values change nothing")

	stored, ok, err = store.NfeloForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, first, stored.UpdatedAt, "an unchanged projection leaves updated_at alone")

	again.HomeSpread = -3.5
	again.UpdatedAt = first.Add(2 * time.Hour)
	changed, err = store.UpsertNfeloGames(t.Context(), []db.NfeloGame{again})
	require.NoError(t, err)
	assert.True(t, changed)

	stored, ok, err = store.NfeloForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, -3.5, stored.HomeSpread, 0)
	assert.Equal(t, again.UpdatedAt, stored.UpdatedAt)
}

func TestNfeloForGameReportsAMissingRow(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	_, ok, err := store.NfeloForGame(t.Context(), "missing")
	require.NoError(t, err)
	assert.False(t, ok, "a game with no projection is a normal empty read")
}

func TestNfeloForGames(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)

	g1 := testSlateGame(week)
	g1.ID = "game-1"
	g2 := testSlateGame(week)
	g2.ID = "game-2"
	g2.HomeTeam = "KC"
	g2.AwayTeam = "LAC"
	_, err := store.UpsertGame(t.Context(), g1)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), g2)
	require.NoError(t, err)

	now := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)
	row1 := db.NfeloGame{
		GameID:      g1.ID,
		HomeSpread:  -7.5,
		HomeWinProb: 0.72,
		HomeQB:      "Josh Allen",
		AwayQB:      "Tua Tagovailoa",
		HomeQBAdj:   18.5,
		AwayQBAdj:   -4.25,
		UpdatedAt:   now,
	}
	row2 := db.NfeloGame{
		GameID:      g2.ID,
		HomeSpread:  -3.5,
		HomeWinProb: 0.61,
		HomeQB:      "Patrick Mahomes",
		AwayQB:      "Justin Herbert",
		HomeQBAdj:   22.0,
		AwayQBAdj:   10.0,
		UpdatedAt:   now,
	}
	_, err = store.UpsertNfeloGames(t.Context(), []db.NfeloGame{row1, row2})
	require.NoError(t, err)

	empty, err := store.NfeloForGames(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	partial, err := store.NfeloForGames(t.Context(), []string{g1.ID, "missing-game"})
	require.NoError(t, err)
	assert.Equal(t, map[string]db.NfeloGame{g1.ID: row1}, partial)

	full, err := store.NfeloForGames(t.Context(), []string{g1.ID, g2.ID})
	require.NoError(t, err)
	assert.Equal(t, map[string]db.NfeloGame{g1.ID: row1, g2.ID: row2}, full)
}
