// Package groups owns group membership: who belongs to which group, and the
// one check every scoped request makes.
package groups

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
)

// Group is one boundary for slips and their members.
type Group = db.Group

// Store is the membership slice of storage the app needs. *db.Store implements
// it.
type Store interface {
	Create(ctx context.Context, userID int64, name string) (Group, error)
	ForUser(ctx context.Context, userID int64) ([]Group, error)
	IsMember(ctx context.Context, groupID, userID int64) (bool, error)
}

var _ Store = (*db.Store)(nil)

// UserFrom returns the signed-in user for a request. The server passes
// auth.UserFrom; tests pass a stub.
type UserFrom func(context.Context) (db.User, bool)

// GroupCookie reads and writes the signed cookie that names the active group.
// *auth.Cookies implements it.
type GroupCookie interface {
	GroupID(r *http.Request) (int64, bool)
	SetGroup(w http.ResponseWriter, id int64)
}

// Scope resolves the request's active group and puts it on the context. Mount
// it after the sign-in check, which is what supplies userFrom.
//
// The signed pc_group cookie is a preference, not a permission: Scope uses it
// only when it names a group the person belongs to. A missing, forged, or
// stale cookie falls back to their first group.
func Scope(
	logger zerolog.Logger,
	store Store,
	userFrom UserFrom,
	cookies GroupCookie,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := userFrom(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			group, err := activeGroup(r, w, user.ID, store, cookies)
			if err != nil {
				logger.Error().Err(err).Int64("user", user.ID).Msg("resolve active group")
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
				return
			}

			next.ServeHTTP(w, r.WithContext(withGroup(r.Context(), group)))
		})
	}
}

// activeGroup picks the group the request acts in. It writes the cookie when
// the browser's choice is missing or no longer valid, so the choice sticks.
func activeGroup(
	r *http.Request,
	w http.ResponseWriter,
	userID int64,
	store Store,
	cookies GroupCookie,
) (Group, error) {
	memberships, err := store.ForUser(r.Context(), userID)
	if err != nil {
		return Group{}, err
	}
	if len(memberships) == 0 {
		return Group{}, fmt.Errorf("user %d belongs to no group", userID)
	}

	if wanted, ok := cookies.GroupID(r); ok {
		for _, group := range memberships {
			if group.ID == wanted {
				return group, nil
			}
		}
	}

	group := memberships[0]
	cookies.SetGroup(w, group.ID)
	return group, nil
}

// groupKey is the context key for the group Scope chose.
type groupKey struct{}

// withGroup returns ctx carrying group.
func withGroup(ctx context.Context, group Group) context.Context {
	return context.WithValue(ctx, groupKey{}, group)
}

// From returns the group Scope put on ctx. A request outside Scope reports
// false.
func From(ctx context.Context) (Group, bool) {
	group, ok := ctx.Value(groupKey{}).(Group)
	return group, ok
}
