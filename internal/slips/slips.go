// Package slips holds the shared-slip rules from docs/concepts/slips.md
// "Editing and locking". It reads and writes through a Store, so the rules stay
// in one place and the database stays a thin query layer.
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
	ActiveGroupSlipSchedules(ctx context.Context, groupID int64) ([]db.GroupSlipSchedule, error)
	SlipsForWeek(ctx context.Context, groupID, weekID int64) ([]db.Slip, error)
	Members(ctx context.Context, groupID int64) ([]db.User, error)
	CreateSlip(
		ctx context.Context,
		groupID, weekID int64,
		scheduleID *int64,
		name string,
		slateScope db.SlateScope,
		legsPerMember int,
		memberIDs []int64,
	) (db.Slip, error)
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

// EnsureWeekSlips checks for slips matching each active schedule for groupID.
// For any active schedule without a slip in weekID, it creates one with the
// schedule's name, slate scope, legs-per-member, and all active group members.
// It returns all slips for the group in weekID.
func EnsureWeekSlips(ctx context.Context, store Store, groupID, weekID int64) ([]db.Slip, error) {
	schedules, err := store.ActiveGroupSlipSchedules(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("ensure week slips for group %d in week %d: %w", groupID, weekID, err)
	}

	existingSlips, err := store.SlipsForWeek(ctx, groupID, weekID)
	if err != nil {
		return nil, fmt.Errorf("ensure week slips for group %d in week %d: %w", groupID, weekID, err)
	}
	if existingSlips == nil {
		existingSlips = []db.Slip{}
	}

	var missing []db.GroupSlipSchedule
	for _, sched := range schedules {
		found := false
		for _, slip := range existingSlips {
			if slip.ScheduleID != nil && *slip.ScheduleID == sched.ID {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, sched)
		}
	}

	if len(missing) == 0 {
		return existingSlips, nil
	}

	members, err := store.Members(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("ensure week slips for group %d in week %d: %w", groupID, weekID, err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf(
			"ensure week slips for group %d in week %d: %w",
			groupID,
			weekID,
			db.ErrSlipMembersRequired,
		)
	}

	memberIDs := make([]int64, len(members))
	for i, m := range members {
		memberIDs[i] = m.ID
	}

	for _, sched := range missing {
		schedID := sched.ID
		_, err := store.CreateSlip(
			ctx,
			groupID,
			weekID,
			&schedID,
			sched.Name,
			sched.SlateScope,
			sched.LegsPerMember,
			memberIDs,
		)
		if err != nil && !errors.Is(err, db.ErrSlipNameTaken) {
			return nil, fmt.Errorf("ensure week slips for group %d in week %d: %w", groupID, weekID, err)
		}
	}

	return store.SlipsForWeek(ctx, groupID, weekID)
}
