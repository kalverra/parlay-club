package slips

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// Errors a handler can map to a 422 with a short message.
var (
	// ErrNotMember is returned when the acting user is not on the slip.
	ErrNotMember = errors.New("you're not on this slip")
	// ErrNotOwner is returned when a member touches someone else's leg.
	ErrNotOwner = errors.New("that leg belongs to someone else")
	// ErrLegLimit is returned when a member already has legs_per_member legs.
	ErrLegLimit = errors.New("you've already added all your legs")
	// ErrLegsPerMemberTooLow is returned when a lower limit would leave someone
	// with too many legs.
	ErrLegsPerMemberTooLow = errors.New("a member already has more legs than that")
	// ErrKickoffLocked is returned once a leg's game has kicked off.
	ErrKickoffLocked = errors.New("that game already kicked off")
	// ErrGameOutsideScope is returned when a leg's game does not belong to the slip's slate scope.
	ErrGameOutsideScope = errors.New("that game is outside this slip's slate")
	// ErrLockedIn is returned when a locked-in member tries to change their legs.
	ErrLockedIn = errors.New("you've locked in")
	// ErrLegsNotReady is returned when a member locks in without exactly
	// legs_per_member legs.
	ErrLegsNotReady = errors.New("add all your legs before locking in")
	// ErrSlipNotOpen is returned when a slip setting changes after lock in.
	ErrSlipNotOpen = errors.New("the slip is locked")
	// ErrNotLocked is returned when unlock matches no lock.
	ErrNotLocked = errors.New("nobody is locked in")
	// ErrMemberHasLegs is returned when a member with legs is removed.
	ErrMemberHasLegs = errors.New("delete their legs first")
	// ErrOwnVote is returned when a member votes on their own leg.
	ErrOwnVote = errors.New("you can't vote on your own pick")
	// ErrVotesClosed is returned when the slip is no longer OPEN.
	ErrVotesClosed = errors.New("votes closed once everyone locked in")
	// ErrVoteInvalid is returned when a vote is not up, down, or clear.
	ErrVoteInvalid = errors.New("vote must be up, down, or clear")
)

// Publisher tells live listeners that a slip changed. *live.Hub implements it.
type Publisher interface {
	Publish(topic string)
}

// Service enforces docs/concepts/slips.md "Editing and locking". Every mutation
// checks the rules, writes the change and one activity entry in a transaction,
// updates the slip's status, then publishes slip:{id}. now is injected so the
// kickoff and locking rules stay testable.
type Service struct {
	store         *db.Store
	publish       Publisher
	now           func() time.Time
	onGradeChange func(groupID int64)
}

// New builds a Service over store. A nil publisher skips live updates, and a
// nil now uses the wall clock.
func New(store *db.Store, publish Publisher, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, publish: publish, now: now}
}

// SetOnGradeChange registers a callback invoked after a transaction commits
// leg or slip grade changes for a group. A nil fn disables the callback.
func (s *Service) SetOnGradeChange(fn func(groupID int64)) {
	s.onGradeChange = fn
}

// AddMember puts userID on the slip. Members can change only while the slip is
// OPEN, adding someone already there changes nothing, and a slip member or
// group member adding themselves is allowed. Everyone else gets ErrNotMember.
func (s *Service) AddMember(ctx context.Context, actorID, slipID, userID int64) error {
	changed := false
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, members, err := loadSlip(ctx, tx, slipID, actorID)
		if errors.Is(err, ErrNotMember) && actorID == userID {
			// A group member off the slip may join it themselves.
			slip, actor, members, err = loadJoin(ctx, tx, slipID, actorID)
		}
		if err != nil {
			return err
		}
		if slip.Status != db.SlipOpen {
			return ErrSlipNotOpen
		}
		for _, member := range members {
			if member.User.ID == userID {
				return nil
			}
		}

		added, err := tx.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		if err := tx.AddSlipMember(ctx, slipID, userID); err != nil {
			return err
		}
		message := fmt.Sprintf("%s added %s", actor.User.Name, added.Name)
		if actorID == userID {
			message = actor.User.Name + " joined"
		}
		if _, err := tx.AddActivity(ctx, slipID, &actorID, message); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("add user %d to slip %d: %w", userID, slipID, err)
	}
	if changed {
		s.notify(slipID)
	}
	return nil
}

// RemoveMember takes userID off the slip. Members can change only while the
// slip is OPEN, and a member who still has legs must have them deleted first.
func (s *Service) RemoveMember(ctx context.Context, actorID, slipID, userID int64) error {
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, members, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		if slip.Status != db.SlipOpen {
			return ErrSlipNotOpen
		}

		var removed *db.SlipMember
		for i := range members {
			if members[i].User.ID == userID {
				removed = &members[i]
				break
			}
		}
		if removed == nil {
			return db.ErrSlipMemberNotFound
		}

		legs, err := tx.LegsForSlip(ctx, slipID)
		if err != nil {
			return err
		}
		for _, leg := range legs {
			if leg.UserID == userID {
				return ErrMemberHasLegs
			}
		}
		if err := tx.DeleteUserVotesForSlip(ctx, slipID, userID); err != nil {
			return err
		}
		if err := tx.RemoveSlipMember(ctx, slipID, userID); err != nil {
			return err
		}
		_, err = tx.AddActivity(ctx, slipID, &actorID,
			fmt.Sprintf("%s removed %s", actor.User.Name, removed.User.Name))
		return err
	})
	if err != nil {
		return fmt.Errorf("remove user %d from slip %d: %w", userID, slipID, err)
	}
	s.notify(slipID)
	return nil
}

// UpdateSlip changes a slip's name and legs per member. Both can change only
// while the slip is OPEN, and the limit cannot drop below a member's current
// leg count. One activity entry records what changed; a no-op writes none.
func (s *Service) UpdateSlip(
	ctx context.Context,
	actorID, slipID int64,
	name string,
	legsPerMember int,
) (db.Slip, error) {
	var (
		updated db.Slip
		changed bool
	)
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, _, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		if slip.Status != db.SlipOpen {
			return ErrSlipNotOpen
		}
		if legsPerMember < slip.LegsPerMember {
			legs, err := tx.LegsForSlip(ctx, slipID)
			if err != nil {
				return err
			}
			counts := make(map[int64]int, len(legs))
			for _, leg := range legs {
				counts[leg.UserID]++
			}
			for _, count := range counts {
				if count > legsPerMember {
					return ErrLegsPerMemberTooLow
				}
			}
		}

		updated, err = tx.UpdateSlip(ctx, slipID, name, legsPerMember)
		if err != nil {
			return err
		}
		message := slipChange(actor.User.Name, slip, updated)
		if message == "" {
			return nil
		}
		if _, err := tx.AddActivity(ctx, slipID, &actorID, message); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return db.Slip{}, fmt.Errorf("update slip %d: %w", slipID, err)
	}
	if changed {
		s.notify(slipID)
	}
	return updated, nil
}

// AddLeg adds one pick for the acting member. Membership, the leg limit,
// kickoff locks, and lock in all apply. The leg's owner is the acting user.
func (s *Service) AddLeg(ctx context.Context, actorID int64, leg db.Leg) (db.Leg, error) {
	now := s.now()
	var created db.Leg
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, _, err := loadSlip(ctx, tx, leg.SlipID, actorID)
		if err != nil {
			return err
		}
		if actor.LockedAt != nil {
			return ErrLockedIn
		}

		legs, err := tx.LegsForSlip(ctx, leg.SlipID)
		if err != nil {
			return err
		}
		count := 0
		for _, existing := range legs {
			if existing.UserID == actorID {
				count++
			}
		}
		if count >= slip.LegsPerMember {
			return ErrLegLimit
		}
		if err := checkKickoff(ctx, tx, slip, leg.GameID, now); err != nil {
			return err
		}

		leg.UserID = actorID
		created, err = tx.CreateLeg(ctx, leg)
		if err != nil {
			return err
		}
		_, err = tx.AddActivity(ctx, leg.SlipID, &actorID,
			fmt.Sprintf("%s added %s", actor.User.Name, created.Selection))
		return err
	})
	if err != nil {
		return db.Leg{}, fmt.Errorf("add leg to slip %d: %w", leg.SlipID, err)
	}
	s.notify(leg.SlipID)
	return created, nil
}

// AddLegs atomically adds a batch of picks for the acting member. It requires
// the slip to be OPEN and the member unlocked, skips selections already on the
// member's slip (and duplicates within the batch), checks remaining capacity,
// validates kickoff/scope for each new leg, and publishes one slip-updated
// notification if any legs were inserted.
func (s *Service) AddLegs(
	ctx context.Context,
	actorID, slipID int64,
	legs []db.Leg,
) ([]db.Leg, error) {
	if len(legs) == 0 {
		return nil, nil
	}
	now := s.now()
	var created []db.Leg
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, _, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		if slip.Status != db.SlipOpen {
			return ErrSlipNotOpen
		}
		if actor.LockedAt != nil {
			return ErrLockedIn
		}

		existing, err := tx.LegsForSlip(ctx, slipID)
		if err != nil {
			return err
		}
		var mine []db.Leg
		for _, leg := range existing {
			if leg.UserID == actorID {
				mine = append(mine, leg)
			}
		}

		toInsert := make([]db.Leg, 0, len(legs))
		for _, candidate := range legs {
			if hasSameLegPick(mine, candidate) || hasSameLegPick(toInsert, candidate) {
				continue
			}
			toInsert = append(toInsert, candidate)
		}
		if len(toInsert) == 0 {
			return nil
		}
		if len(mine)+len(toInsert) > slip.LegsPerMember {
			return ErrLegLimit
		}

		created = make([]db.Leg, 0, len(toInsert))
		for _, leg := range toInsert {
			if err := checkKickoff(ctx, tx, slip, leg.GameID, now); err != nil {
				return err
			}
			leg.SlipID = slipID
			leg.UserID = actorID
			inserted, err := tx.CreateLeg(ctx, leg)
			if err != nil {
				return err
			}
			created = append(created, inserted)
			if _, err := tx.AddActivity(ctx, slipID, &actorID,
				fmt.Sprintf("%s added %s", actor.User.Name, inserted.Selection)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("add legs to slip %d: %w", slipID, err)
	}
	if len(created) > 0 {
		s.notify(slipID)
	}
	return created, nil
}

func hasSameLegPick(existing []db.Leg, candidate db.Leg) bool {
	for _, leg := range existing {
		if sameLegPick(leg, candidate) {
			return true
		}
	}
	return false
}

func sameLegPick(a, b db.Leg) bool {
	if db.StringValue(a.GameID) != "" || db.StringValue(b.GameID) != "" {
		return db.SameSelection(db.LegSelection(a), db.LegSelection(b))
	}
	return a.MarketType == b.MarketType &&
		strings.EqualFold(strings.TrimSpace(a.Selection), strings.TrimSpace(b.Selection))
}

// SetLegResult records one leg's result by hand. Any slip member can grade any
// leg, including one that was already graded. The leg records the acting member
// as its grader, the slip's result and status are recomputed from every leg,
// and one activity entry says what changed.
func (s *Service) SetLegResult(
	ctx context.Context,
	actorID, legID int64,
	result parlay.Result,
) (db.Slip, error) {
	var (
		updated db.Slip
		slipID  int64
		changed bool
	)
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		leg, err := tx.LegByID(ctx, legID)
		if err != nil {
			return err
		}
		slipID = leg.SlipID
		slip, actor, members, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}

		already := leg.Result == result && leg.GradedBy != nil && *leg.GradedBy == actor.User.Name
		if already {
			updated = slip
			return nil
		}
		if err := tx.SetLegResult(ctx, leg.ID, result, actor.User.Name); err != nil {
			return err
		}

		legs, err := tx.LegsForSlip(ctx, slipID)
		if err != nil {
			return err
		}
		slipResult, status := gradeSlip(legs, members)
		message := fmt.Sprintf("%s set %s to %s", actor.User.Name, leg.Selection, result)
		if _, err := tx.AddActivity(ctx, slipID, &actorID, message); err != nil {
			return err
		}
		if err := tx.SetSlipGrade(ctx, slipID, slipResult, status); err != nil {
			return err
		}
		updated, err = tx.SlipByID(ctx, slipID)
		changed = true
		return err
	})
	if err != nil {
		return db.Slip{}, fmt.Errorf("set result on leg %d: %w", legID, err)
	}
	if changed {
		s.notify(slipID)
		if s.onGradeChange != nil {
			s.onGradeChange(updated.GroupID)
		}
	}
	return updated, nil
}

// AutoGraderName is the graded_by value recorded when the automatic grader
// settles a leg.
const AutoGraderName = "auto"

// AutoGrade is one leg outcome produced by the automatic grader.
type AutoGrade struct {
	LegID  int64
	Result parlay.Result
}

// AutoGradeLegs applies automatic grades inside one transaction, updating only
// legs that are still PENDING (so manual grades and prior NEEDS_GRADING marks
// are never overwritten). Each slip with at least one updated leg has its
// result and status recomputed, receives one system activity entry, and is
// published on slip:{id}.
func (s *Service) AutoGradeLegs(ctx context.Context, grades []AutoGrade) error {
	if len(grades) == 0 {
		return nil
	}
	for _, g := range grades {
		if !g.Result.IsLegResult() {
			return fmt.Errorf("auto-grade leg %d: %w", g.LegID, db.ErrLegResultInvalid)
		}
	}

	var (
		slipOrder  []int64
		groupIDs   []int64
		seenGroups = make(map[int64]struct{})
	)
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slipChanges := make(map[int64][]string)
		for _, g := range grades {
			if g.Result == parlay.Pending {
				continue
			}
			leg, err := tx.LegByID(ctx, g.LegID)
			if errors.Is(err, db.ErrLegNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			updated, err := tx.SetLegResultIfPending(ctx, g.LegID, g.Result, AutoGraderName)
			if err != nil {
				return err
			}
			if !updated {
				continue
			}
			if _, seen := slipChanges[leg.SlipID]; !seen {
				slipOrder = append(slipOrder, leg.SlipID)
			}
			slipChanges[leg.SlipID] = append(
				slipChanges[leg.SlipID],
				fmt.Sprintf("%s to %s", leg.Selection, g.Result),
			)
		}

		for _, slipID := range slipOrder {
			slip, err := tx.SlipByID(ctx, slipID)
			if err != nil {
				return err
			}
			if _, seen := seenGroups[slip.GroupID]; !seen {
				seenGroups[slip.GroupID] = struct{}{}
				groupIDs = append(groupIDs, slip.GroupID)
			}

			legs, err := tx.LegsForSlip(ctx, slipID)
			if err != nil {
				return err
			}
			members, err := tx.SlipMembers(ctx, slipID)
			if err != nil {
				return err
			}
			slipResult, status := gradeSlip(legs, members)
			if err := tx.SetSlipGrade(ctx, slipID, slipResult, status); err != nil {
				return err
			}

			changes := slipChanges[slipID]
			message := "Auto-graded " + changes[0]
			if len(changes) > 1 {
				message = fmt.Sprintf("Auto-graded %d legs: %s", len(changes), strings.Join(changes, ", "))
			}
			if _, err := tx.AddActivity(ctx, slipID, nil, message); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("auto-grade legs: %w", err)
	}

	for _, slipID := range slipOrder {
		s.notify(slipID)
	}
	if s.onGradeChange != nil {
		for _, groupID := range groupIDs {
			s.onGradeChange(groupID)
		}
	}
	return nil
}

// gradeSlip recomputes a slip's result and status from its legs and members.
func gradeSlip(legs []db.Leg, members []db.SlipMember) (parlay.Result, db.SlipStatus) {
	result := slipResult(legs)
	if result != parlay.Pending {
		return result, db.SlipGraded
	}
	for _, member := range members {
		if member.LockedAt == nil {
			return result, db.SlipOpen
		}
	}
	return result, db.SlipLocked
}

// slipResult grades a slip from its legs: any LOST busts it, a PENDING or
// NEEDS_GRADING leg holds it open, all PUSH or VOID legs make it a push, and
// anything else wins.
func slipResult(legs []db.Leg) parlay.Result {
	pending := false
	counted := false
	for _, leg := range legs {
		switch leg.Result {
		case parlay.Lost:
			return parlay.Lost
		case parlay.Pending, parlay.NeedsGrading:
			pending = true
		}
		counted = counted || leg.Result.Counts()
	}
	if pending {
		return parlay.Pending
	}
	if !counted {
		return parlay.Push
	}
	return parlay.Won
}

// UpdateLeg changes a leg's pick fields. Only the owner can edit it, and the
// leg cannot be locked in or kickoff-locked. The owner, slip, result, and
// grader stay put.
func (s *Service) UpdateLeg(ctx context.Context, actorID int64, leg db.Leg) (db.Leg, error) {
	now := s.now()
	var updated db.Leg
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		existing, err := tx.LegByID(ctx, leg.ID)
		if err != nil {
			return err
		}
		slip, actor, _, err := loadSlip(ctx, tx, existing.SlipID, actorID)
		if err != nil {
			return err
		}
		if existing.UserID != actorID {
			return ErrNotOwner
		}
		if actor.LockedAt != nil {
			return ErrLockedIn
		}
		if err := checkKickoff(ctx, tx, slip, existing.GameID, now); err != nil {
			return err
		}
		if err := checkKickoff(ctx, tx, slip, leg.GameID, now); err != nil {
			return err
		}

		leg.ID = existing.ID
		leg.SlipID = existing.SlipID
		leg.UserID = existing.UserID
		updated, err = tx.UpdateLeg(ctx, leg)
		if err != nil {
			return err
		}
		// A different pick is a different question, so its votes do not carry
		// over. An odds-only edit keeps them: it is the same pick at a new price.
		if pickChanged(existing, updated) {
			if err := tx.DeleteLegVotes(ctx, updated.ID); err != nil {
				return err
			}
		}
		message := fmt.Sprintf("%s edited %s", actor.User.Name, updated.Selection)
		if updated.Selection != existing.Selection {
			message = fmt.Sprintf(
				"%s changed %s to %s", actor.User.Name, existing.Selection, updated.Selection)
		}
		_, err = tx.AddActivity(ctx, leg.SlipID, &actorID, message)
		return err
	})
	if err != nil {
		return db.Leg{}, fmt.Errorf("update leg %d: %w", leg.ID, err)
	}
	s.notify(updated.SlipID)
	return updated, nil
}

// Vote records the acting member's vote on someone else's leg: 1 for up, -1
// for down, and 0 to clear it. Only slip members can vote, they cannot vote on
// their own pick, and votes stop once the slip leaves OPEN. Voting writes no
// activity entry, and the same vote again changes nothing.
func (s *Service) Vote(ctx context.Context, actorID, legID int64, value int) error {
	if value != -1 && value != 0 && value != 1 {
		return fmt.Errorf("vote on leg %d: %w", legID, ErrVoteInvalid)
	}
	now := s.now()
	var (
		slipID  int64
		changed bool
	)
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		leg, err := tx.LegByID(ctx, legID)
		if err != nil {
			return err
		}
		slipID = leg.SlipID
		slip, _, _, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		if leg.UserID == actorID {
			return ErrOwnVote
		}
		if slip.Status != db.SlipOpen {
			return ErrVotesClosed
		}

		current, err := tx.LegVoteValue(ctx, legID, actorID)
		if err != nil {
			return err
		}
		if current == value {
			return nil
		}
		if value == 0 {
			if err := tx.ClearLegVote(ctx, legID, actorID); err != nil {
				return err
			}
		} else if err := tx.SetLegVote(ctx, legID, actorID, value, now); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("vote on leg %d: %w", legID, err)
	}
	if changed {
		s.notify(slipID)
	}
	return nil
}

// DeleteLeg removes one of the acting member's legs. A kickoff-locked leg or a
// locked-in member cannot delete it.
func (s *Service) DeleteLeg(ctx context.Context, actorID, legID int64) error {
	now := s.now()
	var slipID int64
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		existing, err := tx.LegByID(ctx, legID)
		if err != nil {
			return err
		}
		slipID = existing.SlipID
		slip, actor, _, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		if existing.UserID != actorID {
			return ErrNotOwner
		}
		if actor.LockedAt != nil {
			return ErrLockedIn
		}
		if err := checkKickoff(ctx, tx, slip, existing.GameID, now); err != nil {
			return err
		}
		if err := tx.DeleteLeg(ctx, legID); err != nil {
			return err
		}
		_, err = tx.AddActivity(ctx, slipID, &actorID,
			fmt.Sprintf("%s deleted %s", actor.User.Name, existing.Selection))
		return err
	})
	if err != nil {
		return fmt.Errorf("delete leg %d: %w", legID, err)
	}
	s.notify(slipID)
	return nil
}

// LockIn sets the acting member's locked_at once they have exactly
// legs_per_member legs. When every member is in, the slip becomes LOCKED.
// Locking in twice changes nothing.
func (s *Service) LockIn(ctx context.Context, actorID, slipID int64) error {
	now := s.now()
	changed := false
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, members, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		if actor.LockedAt != nil {
			return nil
		}

		legs, err := tx.LegsForSlip(ctx, slipID)
		if err != nil {
			return err
		}
		count := 0
		for _, leg := range legs {
			if leg.UserID == actorID {
				count++
			}
		}
		if count != slip.LegsPerMember {
			return ErrLegsNotReady
		}
		if err := tx.SetSlipMemberLock(ctx, slipID, actorID, &now); err != nil {
			return err
		}
		if _, err := tx.AddActivity(ctx, slipID, &actorID,
			actor.User.Name+" locked in"); err != nil {
			return err
		}

		locked := 0
		for _, member := range members {
			if member.User.ID == actorID || member.LockedAt != nil {
				locked++
			}
		}
		if locked == len(members) {
			if err := tx.SetSlipStatus(ctx, slipID, db.SlipLocked); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("lock in on slip %d: %w", slipID, err)
	}
	if changed {
		s.notify(slipID)
	}
	return nil
}

// Unlock is available to any member while the slip is LOCKED or anyone is
// locked in. It clears every lock, sets the slip OPEN, and records who did it.
func (s *Service) Unlock(ctx context.Context, actorID, slipID int64) error {
	err := s.store.Tx(ctx, func(tx *db.Store) error {
		slip, actor, members, err := loadSlip(ctx, tx, slipID, actorID)
		if err != nil {
			return err
		}
		locked := slip.Status == db.SlipLocked
		for _, member := range members {
			locked = locked || member.LockedAt != nil
		}
		if !locked {
			return ErrNotLocked
		}

		for _, member := range members {
			if member.LockedAt == nil {
				continue
			}
			if err := tx.SetSlipMemberLock(ctx, slipID, member.User.ID, nil); err != nil {
				return err
			}
		}
		if err := tx.SetSlipStatus(ctx, slipID, db.SlipOpen); err != nil {
			return err
		}
		_, err = tx.AddActivity(ctx, slipID, &actorID,
			"unlocked by "+actor.User.Name)
		return err
	})
	if err != nil {
		return fmt.Errorf("unlock slip %d: %w", slipID, err)
	}
	s.notify(slipID)
	return nil
}

// loadSlip reads a slip and its members and finds the acting member.
func loadSlip(
	ctx context.Context,
	store *db.Store,
	slipID, actorID int64,
) (db.Slip, db.SlipMember, []db.SlipMember, error) {
	slip, err := store.SlipByID(ctx, slipID)
	if err != nil {
		return db.Slip{}, db.SlipMember{}, nil, err
	}
	members, err := store.SlipMembers(ctx, slipID)
	if err != nil {
		return db.Slip{}, db.SlipMember{}, nil, err
	}
	for _, member := range members {
		if member.User.ID == actorID {
			return slip, member, members, nil
		}
	}
	return db.Slip{}, db.SlipMember{}, nil, ErrNotMember
}

// loadJoin reads a slip for a group member who is not on it, so a person can
// add themselves. A person outside the group still gets ErrNotMember.
func loadJoin(
	ctx context.Context,
	store *db.Store,
	slipID, actorID int64,
) (db.Slip, db.SlipMember, []db.SlipMember, error) {
	slip, err := store.SlipByID(ctx, slipID)
	if err != nil {
		return db.Slip{}, db.SlipMember{}, nil, err
	}
	member, err := store.IsMember(ctx, slip.GroupID, actorID)
	if err != nil {
		return db.Slip{}, db.SlipMember{}, nil, err
	}
	if !member {
		return db.Slip{}, db.SlipMember{}, nil, ErrNotMember
	}
	actor, err := store.UserByID(ctx, actorID)
	if err != nil {
		return db.Slip{}, db.SlipMember{}, nil, err
	}
	members, err := store.SlipMembers(ctx, slipID)
	if err != nil {
		return db.Slip{}, db.SlipMember{}, nil, err
	}
	return slip, db.SlipMember{User: actor}, members, nil
}

// checkKickoff refuses a game that has kicked off or does not belong to the
// slip's slate scope, unless the slip is for a past week, where history can
// be entered.
func checkKickoff(
	ctx context.Context,
	store *db.Store,
	slip db.Slip,
	gameID *string,
	now time.Time,
) error {
	if gameID == nil {
		return nil
	}
	past, err := store.WeekPast(ctx, slip.WeekID, now)
	if err != nil {
		return err
	}
	if past {
		return nil
	}

	kickoff, err := store.GameKickoff(ctx, *gameID)
	if err != nil {
		return err
	}
	if !now.Before(kickoff) {
		return ErrKickoffLocked
	}
	if !slip.SlateScope.Matches(kickoff) {
		return ErrGameOutsideScope
	}
	return nil
}

// pickChanged reports whether an edit changed the pick itself. Odds-only edits
// and board-synced line/price updates on the same canonical selection preserve
// votes; changing the player, team, market, side, or custom OTHER pick clears
// them.
func pickChanged(before, after db.Leg) bool {
	exactSamePick := sameText(before.GameID, after.GameID) &&
		before.MarketType == after.MarketType &&
		before.Selection == after.Selection &&
		sameText(before.Team, after.Team) &&
		sameText(before.PlayerName, after.PlayerName) &&
		sameText(before.Side, after.Side) &&
		db.SameLine(before.Line, after.Line)
	if exactSamePick {
		return false
	}
	if before.MarketType != "OTHER" && after.MarketType != "OTHER" &&
		db.SameSelection(db.LegSelection(before), db.LegSelection(after)) {
		return false
	}
	return true
}

// sameText compares two optional leg fields.
func sameText(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// slipChange describes a name or legs-per-member change for the activity feed.
func slipChange(actor string, before, after db.Slip) string {
	nameChanged := before.Name != after.Name
	legsChanged := before.LegsPerMember != after.LegsPerMember
	switch {
	case nameChanged && legsChanged:
		return fmt.Sprintf("%s renamed the slip to %q and set %d legs per member",
			actor, after.Name, after.LegsPerMember)
	case nameChanged:
		return fmt.Sprintf("%s renamed the slip to %q", actor, after.Name)
	case legsChanged:
		return fmt.Sprintf("%s set %d legs per member", actor, after.LegsPerMember)
	default:
		return ""
	}
}

// notify publishes one signal for the slip. The live hub coalesces bursts, so
// a slow reader cannot block a mutation.
func (s *Service) notify(slipID int64) {
	if s.publish == nil {
		return
	}
	s.publish.Publish(fmt.Sprintf("slip:%d", slipID))
}
