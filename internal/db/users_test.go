package db_test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// openTestStore opens a migrated database and returns a Store over it along
// with the raw handle, for tests that need to set columns no Store method
// writes yet.
func openTestStore(t *testing.T) (*db.Store, *sql.DB) {
	t.Helper()

	database := openTestDB(t)
	require.NoError(t, db.Migrate(t.Context(), database))

	return db.NewStore(database), database
}

func TestCreateUserAssignsNextPaletteColor(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	first, err := store.CreateUser(t.Context(), "Ada")
	require.NoError(t, err)
	second, err := store.CreateUser(t.Context(), "Grace")
	require.NoError(t, err)

	assert.Positive(t, first.ID)
	assert.Equal(t, "Ada", first.Name)
	assert.Equal(t, db.AvatarColor("sky"), first.AvatarColor)
	assert.True(t, first.Active)
	assert.Nil(t, first.HomeBook)
	assert.Greater(t, second.ID, first.ID)
	assert.Equal(t, db.AvatarColor("rose"), second.AvatarColor)

	stored, err := store.UserByID(t.Context(), first.ID)
	require.NoError(t, err)
	assert.Equal(t, first, stored)
}

func TestCreateUserWrapsPaletteAround(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	// The palette holds eight colors, so the ninth user starts it over.
	for i := range 8 {
		_, err := store.CreateUser(t.Context(), fmt.Sprintf("Guest %d", i))
		require.NoError(t, err)
	}
	ninth, err := store.CreateUser(t.Context(), "Ada")
	require.NoError(t, err)

	assert.Equal(t, db.AvatarColor("sky"), ninth.AvatarColor)
	assert.Equal(t, "bg-sky-500", ninth.AvatarColor.Class())
}

func TestCreateUserRejectsDuplicateNameIgnoringCase(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	_, err := store.CreateUser(t.Context(), "Ada")
	require.NoError(t, err)

	_, err = store.CreateUser(t.Context(), "ada")
	require.ErrorIs(t, err, db.ErrNameTaken)

	users, err := store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Len(t, users, 1)
}

func TestListActiveUsersSkipsInactiveAndOrdersByName(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	for _, name := range []string{"charlie", "Ada", "Bob"} {
		_, err := store.CreateUser(t.Context(), name)
		require.NoError(t, err)
	}
	_, err := database.ExecContext(t.Context(), "UPDATE users SET active = 0 WHERE name = 'Bob'")
	require.NoError(t, err)

	users, err := store.ListActiveUsers(t.Context())
	require.NoError(t, err)

	names := make([]string, len(users))
	for i, user := range users {
		names[i] = user.Name
	}
	assert.Equal(t, []string{"Ada", "charlie"}, names)
}

func TestListActiveUsersReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	users, err := store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Empty(t, users)
	assert.NotNil(t, users)
}

func TestUserByIDReturnsHomeBookAndActiveFlag(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	user, err := store.CreateUser(t.Context(), "Ada")
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), `
		UPDATE users SET home_book = 'draftkings', active = 0 WHERE id = ?`, user.ID)
	require.NoError(t, err)

	stored, err := store.UserByID(t.Context(), user.ID)
	require.NoError(t, err)

	require.NotNil(t, stored.HomeBook)
	assert.Equal(t, "draftkings", *stored.HomeBook)
	assert.False(t, stored.Active)
}

func TestUserByIDNotFound(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.UserByID(t.Context(), 404)
	require.ErrorIs(t, err, db.ErrUserNotFound)
}

func TestAvatarColorClass(t *testing.T) {
	t.Parallel()

	for _, color := range []db.AvatarColor{
		"sky", "rose", "amber", "emerald", "violet", "orange", "teal", "pink",
	} {
		assert.NotEmpty(t, color.Class(), color)
	}

	assert.Equal(t, "bg-slate-600", db.AvatarColor("chartreuse").Class())
}
