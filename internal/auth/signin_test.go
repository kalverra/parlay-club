package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
)

// seedIdentity creates a user and sets the Google identity columns the public
// store API does not write yet.
func seedIdentity(t *testing.T, f *fixture, name, email, sub, picture string) db.User {
	t.Helper()

	user, err := f.store.CreateUserWithIdentity(t.Context(), name, "", "", "")
	require.NoError(t, err)

	_, err = f.database.ExecContext(t.Context(), `
		UPDATE users
		SET email = NULLIF(?, ''), google_sub = NULLIF(?, ''), picture_url = NULLIF(?, '')
		WHERE id = ?`, email, sub, picture, user.ID)
	require.NoError(t, err)

	user, err = f.store.UserByID(t.Context(), user.ID)
	require.NoError(t, err)
	return user
}

// countUsers returns the number of stored users, active or not.
func countUsers(t *testing.T, f *fixture) int {
	t.Helper()

	var count int
	require.NoError(t,
		f.database.QueryRowContext(t.Context(), "SELECT count(*) FROM users").Scan(&count))
	return count
}

// soloGroup returns the only group user belongs to, failing when they have none.
func soloGroup(t *testing.T, f *fixture, userID int64) db.Group {
	t.Helper()

	groups, err := f.store.ForUser(t.Context(), userID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	return groups[0]
}

func TestSignInResolution(t *testing.T) {
	t.Parallel()

	t.Run("known subject refreshes the picture and keeps the name", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		ada := seedIdentity(t, f, "Ada", "ada@example.com", "google-ada", "old.png")

		user, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub:     "google-ada",
			Email:   "ada@example.com",
			Name:    "Google Ada",
			Picture: "new.png",
		})
		require.NoError(t, err)

		assert.Equal(t, ada.ID, user.ID)
		assert.Equal(t, "Ada", user.Name, "a linked user keeps the name they already have")
		assert.Equal(t, "new.png", user.PictureURL)
		assert.Equal(t, 1, countUsers(t, f))
	})

	t.Run("new email creates a user in a Just me group", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)

		user, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub:     "google-new",
			Email:   "new@example.com",
			Name:    "New Person",
			Picture: "new.png",
		})
		require.NoError(t, err)

		assert.Positive(t, user.ID)
		assert.Equal(t, "New Person", user.Name)
		assert.Equal(t, "new@example.com", user.Email)
		assert.Equal(t, "new.png", user.PictureURL)
		assert.Equal(t, db.AvatarColor("sky"), user.AvatarColor)
		assert.True(t, user.Active)
		assert.Equal(t, 1, countUsers(t, f))

		linked, err := f.store.UserByGoogleSub(t.Context(), "google-new")
		require.NoError(t, err)
		assert.Equal(t, user.ID, linked.ID)

		group := soloGroup(t, f, user.ID)
		assert.Equal(t, "Just me", group.Name)
		member, err := f.store.IsMember(t.Context(), group.ID, user.ID)
		require.NoError(t, err)
		assert.True(t, member, "the new user joins their solo group")
	})

	t.Run("signing in again keeps one solo group", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		identity := auth.Identity{
			Sub:   "google-new",
			Email: "new@example.com",
			Name:  "New Person",
		}

		first, err := auth.SignIn(t.Context(), f.store, identity)
		require.NoError(t, err)
		second, err := auth.SignIn(t.Context(), f.store, identity)
		require.NoError(t, err)

		assert.Equal(t, first.ID, second.ID)
		assert.Equal(t, 1, countUsers(t, f))
		assert.Equal(t, soloGroup(t, f, first.ID).ID, soloGroup(t, f, second.ID).ID)
	})

	t.Run("a stranger signs in and gets a solo group", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)

		user, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub:   "google-stranger",
			Email: "stranger@example.com",
			Name:  "Stranger",
		})
		require.NoError(t, err)

		assert.Positive(t, user.ID)
		assert.Equal(t, "stranger@example.com", user.Email)
		assert.Equal(t, 1, countUsers(t, f))

		group := soloGroup(t, f, user.ID)
		assert.Equal(t, "Just me", group.Name)
	})

	t.Run("matching email links the existing user", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		ada := seedIdentity(t, f, "Ada", "ada@example.com", "", "")

		user, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub:     "google-ada",
			Email:   "ada@example.com",
			Name:    "Google Ada",
			Picture: "ada.png",
		})
		require.NoError(t, err)

		assert.Equal(t, ada.ID, user.ID)
		assert.Equal(t, "Ada", user.Name)
		assert.Equal(t, "ada@example.com", user.Email)
		assert.Equal(t, "ada.png", user.PictureURL)
		assert.Equal(t, 1, countUsers(t, f))

		linked, err := f.store.UserByGoogleSub(t.Context(), "google-ada")
		require.NoError(t, err)
		assert.Equal(t, ada.ID, linked.ID)
	})

	t.Run("email case does not matter", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		ada := seedIdentity(t, f, "Ada", "ada@example.com", "", "")

		user, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub:   "google-ada",
			Email: "Ada@Example.com",
			Name:  "Ada",
		})
		require.NoError(t, err)

		assert.Equal(t, ada.ID, user.ID)
		assert.Equal(t, 1, countUsers(t, f))
	})

	t.Run("two Google users may share a display name", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)

		first, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub: "google-one", Email: "one@example.com", Name: "Alex",
		})
		require.NoError(t, err)
		second, err := auth.SignIn(t.Context(), f.store, auth.Identity{
			Sub: "google-two", Email: "two@example.com", Name: "Alex",
		})
		require.NoError(t, err)

		assert.NotEqual(t, first.ID, second.ID)
		assert.Equal(t, "Alex", first.Name)
		assert.Equal(t, "Alex", second.Name)
		assert.Equal(t, 2, countUsers(t, f))
		assert.NotEqual(t, soloGroup(t, f, first.ID).ID, soloGroup(t, f, second.ID).ID)
	})
}

func TestSignInRequiresSubjectAndEmail(t *testing.T) {
	t.Parallel()

	tests := map[string]auth.Identity{
		"missing subject": {Email: "ada@example.com"},
		"missing email":   {Sub: "google-ada"},
	}

	for name, identity := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)

			_, err := auth.SignIn(t.Context(), f.store, identity)
			require.ErrorIs(t, err, auth.ErrIncompleteIdentity)
			assert.Equal(t, 0, countUsers(t, f))
		})
	}
}
