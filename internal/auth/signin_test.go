package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/config"
	"github.com/kalverra/parlay-club/internal/db"
)

// signInGate builds a gate from address/name pairs, where an empty name means
// the entry has no history-import name.
func signInGate(entries ...config.AllowedEmail) auth.Gate {
	return auth.NewGate(entries)
}

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

func TestSignInResolution(t *testing.T) {
	t.Parallel()

	t.Run("known subject refreshes the picture and keeps the name", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		ada := seedIdentity(t, f, "Ada", "ada@example.com", "google-ada", "old.png")

		user, err := auth.SignIn(t.Context(), f.store, signInGate(), auth.Identity{
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

	t.Run("allowed new email creates a user", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)

		user, err := auth.SignIn(
			t.Context(),
			f.store,
			signInGate(config.AllowedEmail{Email: "new@example.com"}),
			auth.Identity{
				Sub:     "google-new",
				Email:   "new@example.com",
				Name:    "New Person",
				Picture: "new.png",
			},
		)
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
	})

	t.Run("matching email links the existing user", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		ada := seedIdentity(t, f, "Ada", "ada@example.com", "", "")

		user, err := auth.SignIn(
			t.Context(),
			f.store,
			signInGate(config.AllowedEmail{Email: "ada@example.com"}),
			auth.Identity{
				Sub:     "google-ada",
				Email:   "ada@example.com",
				Name:    "Google Ada",
				Picture: "ada.png",
			},
		)
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

	t.Run("gate name links an unlinked history user", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		bob := seedIdentity(t, f, "Bob", "", "", "")

		user, err := auth.SignIn(
			t.Context(),
			f.store,
			signInGate(config.AllowedEmail{Email: "bob@example.com", Name: "Bob"}),
			auth.Identity{
				Sub:     "google-bob",
				Email:   "bob@example.com",
				Name:    "Robert",
				Picture: "bob.png",
			},
		)
		require.NoError(t, err)

		assert.Equal(t, bob.ID, user.ID)
		assert.Equal(t, "Bob", user.Name, "the history name wins over Google's")
		assert.Equal(t, "bob@example.com", user.Email)
		assert.Equal(t, 1, countUsers(t, f))

		linked, err := f.store.UserByGoogleSub(t.Context(), "google-bob")
		require.NoError(t, err)
		assert.Equal(t, bob.ID, linked.ID)
	})

	t.Run("a linked name is not stolen", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		bob := seedIdentity(t, f, "Bob", "old@example.com", "google-old", "old.png")

		user, err := auth.SignIn(
			t.Context(),
			f.store,
			signInGate(config.AllowedEmail{Email: "bob@example.com", Name: "Bob"}),
			auth.Identity{
				Sub:     "google-new",
				Email:   "bob@example.com",
				Name:    "Bob",
				Picture: "new.png",
			},
		)
		require.NoError(t, err)

		assert.NotEqual(t, bob.ID, user.ID, "the linked Bob stays with their own account")
		assert.Equal(t, "Bob", user.Name)
		assert.Equal(t, 2, countUsers(t, f))

		old, err := f.store.UserByGoogleSub(t.Context(), "google-old")
		require.NoError(t, err)
		assert.Equal(t, bob.ID, old.ID)
		assert.Equal(t, "old@example.com", old.Email)
	})

	t.Run("a stranger is refused and writes nothing", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)

		_, err := auth.SignIn(
			t.Context(),
			f.store,
			signInGate(config.AllowedEmail{Email: "friend@example.com"}),
			auth.Identity{
				Sub:   "google-stranger",
				Email: "stranger@example.com",
				Name:  "Stranger",
			},
		)
		require.ErrorIs(t, err, auth.ErrNotInvited)
		assert.Equal(t, 0, countUsers(t, f))

		_, err = f.store.UserByGoogleSub(t.Context(), "google-stranger")
		require.ErrorIs(t, err, db.ErrUserNotFound)
	})

	t.Run("email case does not matter", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		ada := seedIdentity(t, f, "Ada", "ada@example.com", "", "")

		user, err := auth.SignIn(
			t.Context(),
			f.store,
			signInGate(config.AllowedEmail{Email: "ADA@EXAMPLE.COM"}),
			auth.Identity{
				Sub:   "google-ada",
				Email: "Ada@Example.com",
				Name:  "Ada",
			},
		)
		require.NoError(t, err)

		assert.Equal(t, ada.ID, user.ID)
		assert.Equal(t, 1, countUsers(t, f))
	})

	t.Run("two Google users may share a display name", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		gate := signInGate(
			config.AllowedEmail{Email: "one@example.com"},
			config.AllowedEmail{Email: "two@example.com"},
		)

		first, err := auth.SignIn(t.Context(), f.store, gate, auth.Identity{
			Sub: "google-one", Email: "one@example.com", Name: "Alex",
		})
		require.NoError(t, err)
		second, err := auth.SignIn(t.Context(), f.store, gate, auth.Identity{
			Sub: "google-two", Email: "two@example.com", Name: "Alex",
		})
		require.NoError(t, err)

		assert.NotEqual(t, first.ID, second.ID)
		assert.Equal(t, "Alex", first.Name)
		assert.Equal(t, "Alex", second.Name)
		assert.Equal(t, 2, countUsers(t, f))
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

			_, err := auth.SignIn(t.Context(), f.store, signInGate(), identity)
			require.ErrorIs(t, err, auth.ErrIncompleteIdentity)
			assert.Equal(t, 0, countUsers(t, f))
		})
	}
}

func TestGate(t *testing.T) {
	t.Parallel()

	gate := auth.NewGate([]config.AllowedEmail{
		{Email: "ada@example.com"},
		{Email: "BOB@example.com", Name: "Bob"},
	})

	assert.True(t, gate.Allows("ada@example.com"))
	assert.True(t, gate.Allows("ADA@EXAMPLE.COM"), "emails compare without case")
	assert.True(t, gate.Allows("bob@example.com"))
	assert.False(t, gate.Allows("stranger@example.com"))
	assert.Equal(t, "Bob", gate.LinkName("bob@example.com"))
	assert.Empty(t, gate.LinkName("ada@example.com"))
	assert.Empty(t, gate.LinkName("stranger@example.com"))

	assert.True(t, auth.NewGate(nil).Allows("anyone@example.com"), "an empty list allows everyone in dev")
}
