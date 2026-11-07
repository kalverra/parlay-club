// Package groups owns group membership: who belongs to which group, and the
// one check every scoped request makes.
package groups

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
	webui "github.com/kalverra/parlay-club/internal/web"
)

// Group is one boundary for slips and their members.
type Group = db.Group

// Store is the membership slice of storage the app needs. *db.Store implements
// it.
type Store interface {
	Create(ctx context.Context, userID int64, name string) (Group, error)
	ForUser(ctx context.Context, userID int64) ([]Group, error)
	IsMember(ctx context.Context, groupID, userID int64) (bool, error)
	JoinByToken(ctx context.Context, userID int64, token string) (Group, error)
	ResetInvite(ctx context.Context, groupID int64) (string, error)
	Leave(ctx context.Context, groupID, userID int64) error
}

var _ Store = (*db.Store)(nil)

// InviteStore is the storage the invite screens need: Store plus reading the
// group and people an invite names.
type InviteStore interface {
	Store
	ByToken(ctx context.Context, token string) (Group, error)
	Members(ctx context.Context, groupID int64) ([]db.User, error)
}

var _ InviteStore = (*db.Store)(nil)

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

			memberships, err := store.ForUser(r.Context(), user.ID)
			if err != nil {
				logger.Error().Err(err).Int64("user", user.ID).Msg("list groups")
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
				return
			}
			if len(memberships) == 0 {
				logger.Error().
					Int64("user", user.ID).
					Msg("user belongs to no group")
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
				return
			}

			group := activeGroup(r, w, memberships, cookies)
			ctx := withMemberships(r.Context(), memberships)
			next.ServeHTTP(w, r.WithContext(withGroup(ctx, group)))
		})
	}
}

// activeGroup picks the group the request acts in. It writes the cookie when
// the browser's choice is missing or no longer valid, so the choice sticks.
func activeGroup(
	r *http.Request,
	w http.ResponseWriter,
	memberships []Group,
	cookies GroupCookie,
) Group {
	if wanted, ok := cookies.GroupID(r); ok {
		for _, group := range memberships {
			if group.ID == wanted {
				return group
			}
		}
	}

	group := memberships[0]
	cookies.SetGroup(w, group.ID)
	return group
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

// membershipsKey is the context key for every group the person belongs to.
type membershipsKey struct{}

// withMemberships returns ctx carrying the person's groups.
func withMemberships(ctx context.Context, memberships []Group) context.Context {
	return context.WithValue(ctx, membershipsKey{}, memberships)
}

// membershipsFrom returns the groups Scope loaded, oldest membership first.
func membershipsFrom(ctx context.Context) ([]Group, bool) {
	memberships, ok := ctx.Value(membershipsKey{}).([]Group)
	return memberships, ok
}

// Options returns the groups to offer in the top bar's switcher, with the
// active one marked. It reads what Scope put on the context, so it costs no
// extra query. A request outside Scope gets nil.
func Options(ctx context.Context) []webui.GroupOption {
	active, ok := From(ctx)
	if !ok {
		return nil
	}
	memberships, ok := membershipsFrom(ctx)
	if !ok {
		return nil
	}

	options := make([]webui.GroupOption, 0, len(memberships))
	for _, group := range memberships {
		options = append(options, webui.GroupOption{
			ID:      group.ID,
			Name:    group.Name,
			Current: group.ID == active.ID,
		})
	}
	return options
}
