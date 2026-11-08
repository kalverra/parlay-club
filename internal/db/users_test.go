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

// createUser adds an unlinked user with just a name.
func createUser(t *testing.T, store *db.Store, name string) db.User {
	t.Helper()

	user, err := store.CreateUserWithIdentity(t.Context(), name, "", "", "")
	require.NoError(t, err)
	return user
}

func TestCreateUserAssignsNextPaletteColor(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	first := createUser(t, store, "Ada")
	second := createUser(t, store, "Grace")

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
		createUser(t, store, fmt.Sprintf("Guest %d", i))
	}
	ninth := createUser(t, store, "Ada")

	assert.Equal(t, db.AvatarColor("sky"), ninth.AvatarColor)
	assert.Equal(t, "bg-sky-500", ninth.AvatarColor.Class())
}

func TestCreateUserAllowsDuplicateNames(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := createUser(t, store, "Ada")
	second := createUser(t, store, "ada")

	assert.NotEqual(t, first.ID, second.ID, "Google display names may collide")

	users, err := store.ListActiveUsers(t.Context())
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestListActiveUsersSkipsInactiveAndOrdersByName(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	for _, name := range []string{"charlie", "Ada", "Bob"} {
		createUser(t, store, name)
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
	user := createUser(t, store, "Ada")
	_, err := database.ExecContext(t.Context(), `
		UPDATE users SET home_book = 'draftkings', active = 0 WHERE id = ?`, user.ID)
	require.NoError(t, err)

	stored, err := store.UserByID(t.Context(), user.ID)
	require.NoError(t, err)

	require.NotNil(t, stored.HomeBook)
	assert.Equal(t, "draftkings", *stored.HomeBook)
	assert.False(t, stored.Active)
}

func TestUserByNameMatchesIgnoringCaseAndTakesTheOldest(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	first := createUser(t, store, "Ada")
	createUser(t, store, "ada")

	found, err := store.UserByName(t.Context(), "ADA")
	require.NoError(t, err)
	assert.Equal(t, first, found, "duplicate display names resolve to the first person created")
}

func TestUserByNameReportsAMissingUser(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.UserByName(t.Context(), "Ada")
	require.ErrorIs(t, err, db.ErrUserNotFound)
}

func TestSetUserHomeBookSetsAndClears(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")

	book := "draftkings"
	updated, err := store.SetUserHomeBook(t.Context(), user.ID, &book)
	require.NoError(t, err)
	require.NotNil(t, updated.HomeBook)
	assert.Equal(t, book, *updated.HomeBook)

	cleared, err := store.SetUserHomeBook(t.Context(), user.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, cleared.HomeBook)

	_, err = store.SetUserHomeBook(t.Context(), 404, &book)
	require.ErrorIs(t, err, db.ErrUserNotFound)
}

func TestSetUserActiveSetsAndReportsAMissingUser(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	user := createUser(t, store, "Ada")

	updated, err := store.SetUserActive(t.Context(), user.ID, false)
	require.NoError(t, err)
	assert.False(t, updated.Active)

	updated, err = store.SetUserActive(t.Context(), user.ID, true)
	require.NoError(t, err)
	assert.True(t, updated.Active)

	_, err = store.SetUserActive(t.Context(), 404, true)
	require.ErrorIs(t, err, db.ErrUserNotFound)
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
