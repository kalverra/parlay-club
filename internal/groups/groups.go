// Package groups owns group membership: who belongs to which group, and the
// one check every scoped request makes.
package groups

import (
	"context"

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
