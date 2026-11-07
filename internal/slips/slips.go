// Package slips holds the shared-slip rules from product.md "Editing and
// locking". It reads and writes through a Store, so the rules stay in one place
// and the database stays a thin query layer.
package slips

import (
	"context"
	"errors"
	"fmt"

	"github.com/kalverra/parlay-club/internal/db"
)

// DefaultLegsPerMember is the create-slip default when the group has no earlier
// slip to copy.
const DefaultLegsPerMember = 2

// Store is the slice of storage the slip rules need. *db.Store implements it.
type Store interface {
	PreviousSlip(ctx context.Context, groupID, weekID int64) (db.Slip, error)
	SlipMemberIDs(ctx context.Context, slipID int64) ([]int64, error)
}

var _ Store = (*db.Store)(nil)

// Defaults are the create-slip form values a group copies from its previous
// slip.
type Defaults struct {
	LegsPerMember int
	MemberIDs     []int64
}

// CreateDefaults returns the legs-per-member and members to start a new slip
// with, copied from the group's previous slip: the latest slip in weekID or any
// earlier week. With no previous slip it returns DefaultLegsPerMember and no
// members.
func CreateDefaults(ctx context.Context, store Store, groupID, weekID int64) (Defaults, error) {
	previous, err := store.PreviousSlip(ctx, groupID, weekID)
	if errors.Is(err, db.ErrSlipNotFound) {
		return Defaults{LegsPerMember: DefaultLegsPerMember, MemberIDs: []int64{}}, nil
	}
	if err != nil {
		return Defaults{}, fmt.Errorf("create-slip defaults for group %d in week %d: %w", groupID, weekID, err)
	}

	memberIDs, err := store.SlipMemberIDs(ctx, previous.ID)
	if err != nil {
		return Defaults{}, fmt.Errorf("create-slip defaults for group %d in week %d: %w", groupID, weekID, err)
	}
	return Defaults{LegsPerMember: previous.LegsPerMember, MemberIDs: memberIDs}, nil
}
