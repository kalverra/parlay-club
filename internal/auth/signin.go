package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kalverra/parlay-club/internal/db"
)

var (
	// ErrNotInvited is returned when a verified email is not on the allow list.
	ErrNotInvited = errors.New("email not invited")
	// ErrIncompleteIdentity is returned when a verified identity has no subject
	// or email, so it cannot own a user.
	ErrIncompleteIdentity = errors.New("sign-in identity needs a subject and email")
)

// Identity is a verified sign-in from Google or the dev login.
type Identity struct {
	Sub     string
	Email   string
	Name    string
	Picture string
}

// SignIn resolves a verified identity to a user, or returns ErrNotInvited.
//
// The resolution runs in one transaction, in this order:
//  1. the user linked to this Google subject, whose picture is refreshed;
//  2. the gate, which decides whether a stranger may exist at all;
//  3. the user with this email, ignoring case, which gains the subject;
//  4. an unlinked user whose name the gate entry claims, for history imports;
//  5. a new user built from Google's name and picture.
//
// A linked user keeps the name they already have. Google's name is used only
// when creating, so history never gets renamed under people.
func SignIn(
	ctx context.Context,
	store *db.Store,
	gate Gate,
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

		if !gate.Allows(email) {
			return ErrNotInvited
		}

		byEmail, err := tx.UserByEmail(ctx, email)
		switch {
		case err == nil:
			user, err = tx.LinkGoogle(ctx, byEmail.ID, sub, email, identity.Picture)
			return err
		case !errors.Is(err, db.ErrUserNotFound):
			return err
		}

		if linkName := gate.LinkName(email); linkName != "" {
			unlinked, err := tx.UnlinkedUserByName(ctx, linkName)
			switch {
			case err == nil:
				user, err = tx.LinkGoogle(ctx, unlinked.ID, sub, email, identity.Picture)
				return err
			case !errors.Is(err, db.ErrUserNotFound):
				return err
			}
		}

		user, err = tx.CreateUserWithIdentity(ctx, name, email, sub, identity.Picture)
		return err
	})
	if err != nil {
		return db.User{}, fmt.Errorf("sign in %s: %w", email, err)
	}
	return user, nil
}
