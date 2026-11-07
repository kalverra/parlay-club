package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
)

// soloGroupName is the group every new user starts in. It is a group of one
// until they invite someone or create another group.
const soloGroupName = "Just me"

// ErrIncompleteIdentity is returned when a verified identity has no subject
// or email, so it cannot own a user.
var ErrIncompleteIdentity = errors.New("sign-in identity needs a subject and email")

// Identity is a verified sign-in from Google or the dev login.
type Identity struct {
	Sub     string
	Email   string
	Name    string
	Picture string
}

// SignIn resolves a verified identity to a user, or returns
// ErrIncompleteIdentity.
//
// The resolution runs in one transaction, in this order:
//  1. the user linked to this Google subject, whose picture is refreshed;
//  2. the user with this email, ignoring case, which gains the subject;
//  3. a new user built from Google's name and picture, with a "Just me" group.
//
// A linked user keeps the name they already have. Google's name is used only
// when creating, so history never gets renamed under people.
func SignIn(
	ctx context.Context,
	store *db.Store,
	identity Identity,
) (db.User, error) {
	sub := strings.TrimSpace(identity.Sub)
	email := normalizeEmail(identity.Email)
	if sub == "" || email == "" {
		return db.User{}, ErrIncompleteIdentity
	}
	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name = email
	}

	var user db.User
	err := store.Tx(ctx, func(tx *db.Store) error {
		linked, err := tx.UserByGoogleSub(ctx, sub)
		switch {
		case err == nil:
			user, err = tx.SetUserPicture(ctx, linked.ID, identity.Picture)
			return err
		case !errors.Is(err, db.ErrUserNotFound):
			return err
		}

		byEmail, err := tx.UserByEmail(ctx, email)
		switch {
		case err == nil:
			user, err = tx.LinkGoogle(ctx, byEmail.ID, sub, email, identity.Picture)
			return err
		case !errors.Is(err, db.ErrUserNotFound):
			return err
		}

		user, err = tx.CreateUserWithIdentity(ctx, name, email, sub, identity.Picture)
		if err != nil {
			return err
		}
		_, err = tx.Create(ctx, user.ID, soloGroupName)
		return err
	})
	if err != nil {
		return db.User{}, fmt.Errorf("sign in %s: %w", email, err)
	}
	return user, nil
}

// normalizeEmail trims and lowercases an address for comparison.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
