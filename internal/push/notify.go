package push

import (
	"context"
	"fmt"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// The push service is the slip notifier cmd/server wires when a key pair is
// set.
var _ slips.Notifier = (*Service)(nil)

// Notify sends a slip event to the people who should hear about it. It
// satisfies slips.Notifier.
//
// The send runs inside the request that caused the event, because Cloud Run
// throttles CPU once the response goes out. Every message goes to active
// members of the slip only, so one group's picks never reach an outsider.
func (s *Service) Notify(ctx context.Context, e slips.Event) {
	if !s.Enabled() {
		return
	}
	switch e.Kind {
	case slips.EventLegsAdded:
		s.notifyLegsAdded(ctx, e)
	case slips.EventSlipLocked:
		s.notifySlipLocked(ctx, e)
	case slips.EventSlipGraded:
		s.notifySlipGraded(ctx, e)
	}
}

// notifyLegsAdded tells the other members what the actor picked.
func (s *Service) notifyLegsAdded(ctx context.Context, e slips.Event) {
	if e.Count <= 0 {
		return
	}
	slip, err := s.store.SlipByID(ctx, e.SlipID)
	if err != nil {
		s.logger.Warn().Err(err).Int64("slip", e.SlipID).Msg("push: load slip")
		return
	}
	actor, err := s.store.UserByID(ctx, e.ActorID)
	if err != nil {
		s.logger.Warn().Err(err).Int64("user", e.ActorID).Msg("push: load actor")
		return
	}
	m := legsAddedMessage(slip, actor.Name, e.Count, e.Selection)
	s.SendToUsers(ctx, s.slipRecipients(ctx, e.SlipID, e.ActorID), m)
}

// notifySlipLocked tells the other members that the slip is ready to place.
func (s *Service) notifySlipLocked(ctx context.Context, e slips.Event) {
	slip, err := s.store.SlipByID(ctx, e.SlipID)
	if err != nil {
		s.logger.Warn().Err(err).Int64("slip", e.SlipID).Msg("push: load slip")
		return
	}
	s.SendToUsers(ctx, s.slipRecipients(ctx, e.SlipID, e.ActorID), slipLockedMessage(slip))
}

// notifySlipGraded tells every member how the slip ended. One reservation in
// push_log makes a later manual correction stay quiet: a missed nudge is fine,
// a doubled one is not.
func (s *Service) notifySlipGraded(ctx context.Context, e slips.Event) {
	slip, err := s.store.SlipByID(ctx, e.SlipID)
	if err != nil {
		s.logger.Warn().Err(err).Int64("slip", e.SlipID).Msg("push: load slip")
		return
	}
	m, ok := slipGradedMessage(slip)
	if !ok {
		return
	}
	reserved, err := s.store.ReservePushLog(ctx, db.PushLogGraded, e.SlipID, 0, "")
	if err != nil {
		s.logger.Warn().Err(err).Int64("slip", e.SlipID).Msg("push: reserve grade")
		return
	}
	if !reserved {
		return
	}
	s.SendToUsers(ctx, s.slipRecipients(ctx, e.SlipID, 0), m)
}

// slipRecipients returns the slip's members, minus actorID when it is set.
// Inactive people are dropped by the send itself, which joins users.active.
func (s *Service) slipRecipients(ctx context.Context, slipID, actorID int64) []int64 {
	memberIDs, err := s.store.SlipMemberIDs(ctx, slipID)
	if err != nil {
		s.logger.Warn().Err(err).Int64("slip", slipID).Msg("push: load members")
		return nil
	}
	recipients := make([]int64, 0, len(memberIDs))
	for _, id := range memberIDs {
		if id != actorID {
			recipients = append(recipients, id)
		}
	}
	return recipients
}

// legsAddedMessage describes what was added. One leg names the pick; a batch
// counts them.
func legsAddedMessage(slip db.Slip, actorName string, count int, selection string) Message {
	var body string
	switch {
	case count > 1:
		body = fmt.Sprintf("%s added %d legs", actorName, count)
	case selection != "":
		body = fmt.Sprintf("%s added %s", actorName, selection)
	default:
		body = actorName + " added a leg"
	}
	return slipMessage(slip, body)
}

// slipLockedMessage announces that the last member locked in.
func slipLockedMessage(slip db.Slip) Message {
	return slipMessage(slip, "Everyone's locked in. Time to place it.")
}

// slipGradedMessage announces the slip's final result. It reports false for a
// slip that has not been graded.
func slipGradedMessage(slip db.Slip) (Message, bool) {
	switch slip.Result {
	case parlay.Won:
		return slipMessage(slip, slip.Name+" cashed!"), true
	case parlay.Lost:
		return slipMessage(slip, slip.Name+" came up short."), true
	case parlay.Push, parlay.Void:
		return slipMessage(slip, slip.Name+" pushed. Stakes back."), true
	default:
		return Message{}, false
	}
}

// slipMessage titles a notification with the slip's name and points it at the
// slip. One tag per slip replaces an older notification on the device.
func slipMessage(slip db.Slip, body string) Message {
	return Message{
		Title: slip.Name,
		Body:  body,
		URL:   fmt.Sprintf("/slips/%d", slip.ID),
		Tag:   fmt.Sprintf("slip-%d", slip.ID),
	}
}
