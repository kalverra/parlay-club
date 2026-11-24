package slips

import (
	"fmt"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// Step is the one thing a viewer should do next on a slip. The hub banner and
// each group card read the same value, so they always agree.
type Step string

// The steps a slip can be in.
const (
	// StepJoin means the viewer belongs to the group but not to the slip.
	StepJoin Step = "join"
	// StepPick means the viewer still needs legs.
	StepPick Step = "pick"
	// StepClosed means the week's picks are over, either because the viewer
	// still needs legs or because someone else never finished theirs.
	StepClosed Step = "closed"
	// StepLockIn means the viewer has every leg and can lock in.
	StepLockIn Step = "lock_in"
	// StepWaiting means the viewer is locked in and someone else is not.
	StepWaiting Step = "waiting"
	// StepExport means everyone is locked in and the picks can be placed.
	StepExport Step = "export"
	// StepDone means the slip has been graded.
	StepDone Step = "done"
)

// Next is the viewer's next step on a slip.
type Next struct {
	Step      Step
	Need      int        // legs still to pick
	WaitingOn []string   // names not locked in, in member order
	Deadline  *time.Time // first kickoff after now; nil when none is left
	Result    parlay.Result
}

// NextStep decides what viewerID should do next on slip. It is pure: the
// handler passes the stored slip, members, legs, and week games plus the app
// clock, so kickoff, locking, and the deadline stay testable.
func NextStep(
	slip db.Slip,
	members []db.SlipMember,
	legs []db.Leg,
	games []db.Game,
	viewerID int64,
	now time.Time,
) Next {
	games = FilterGamesForSlip(games, slip)

	next := Next{
		Result:   slip.Result,
		Deadline: firstKickoff(games, now),
	}

	member := findMember(members, viewerID)
	if member == nil {
		next.Step = StepJoin
		return next
	}

	picked := 0
	for _, leg := range legs {
		if leg.UserID == viewerID {
			picked++
		}
	}
	if picked < slip.LegsPerMember {
		next.Need = slip.LegsPerMember - picked
	}

	if slip.Status == db.SlipGraded {
		next.Step = StepDone
		return next
	}

	if member.LockedAt != nil {
		if everyoneLocked(members) || slip.Status == db.SlipLocked {
			next.Step = StepExport
			return next
		}
		// A member who never finished picking can no longer catch up once every
		// game has kicked off. The slip cannot lock in this week, so say so
		// instead of waiting on a pick that cannot arrive.
		if noGamesLeft(games, now) {
			if short := membersShort(members, legs, slip.LegsPerMember); len(short) > 0 {
				next.Step = StepClosed
				next.WaitingOn = short
				return next
			}
		}
		next.Step = StepWaiting
		next.WaitingOn = waitingOn(members, viewerID)
		return next
	}

	if picked < slip.LegsPerMember {
		if noGamesLeft(games, now) {
			next.Step = StepClosed
		} else {
			next.Step = StepPick
		}
		return next
	}

	next.Step = StepLockIn
	return next
}

// Label is the short next-step summary the hub and group cards show.
func (n Next) Label() string {
	switch n.Step {
	case StepPick:
		return fmt.Sprintf("Pick %d more", n.Need)
	case StepLockIn:
		return "Ready to lock in"
	case StepWaiting:
		return "Waiting on " + NameList(n.WaitingOn)
	case StepExport:
		return "Ready to place"
	case StepDone:
		switch n.Result {
		case parlay.Won:
			return "Won"
		case parlay.Lost:
			return "Lost"
		case parlay.Push, parlay.Void:
			return "Push"
		default:
			return "Done"
		}
	case StepClosed:
		return "Picks are closed"
	case StepJoin:
		return "Join this slip"
	default:
		return ""
	}
}

// findMember returns the viewer's membership, or nil when they are not on the
// slip.
func findMember(members []db.SlipMember, viewerID int64) *db.SlipMember {
	for i := range members {
		if members[i].User.ID == viewerID {
			return &members[i]
		}
	}
	return nil
}

// everyoneLocked reports whether every member has locked in.
func everyoneLocked(members []db.SlipMember) bool {
	for _, member := range members {
		if member.LockedAt == nil {
			return false
		}
	}
	return len(members) > 0
}

// waitingOn names the members who have not locked in, in member order. The
// viewer is always locked in when this is used, but it skips them anyway.
func waitingOn(members []db.SlipMember, viewerID int64) []string {
	names := make([]string, 0, len(members))
	for _, member := range members {
		if member.User.ID != viewerID && member.LockedAt == nil {
			names = append(names, member.User.Name)
		}
	}
	return names
}

// membersShort names the members with fewer legs than the slip requires, in
// member order. A locked member always has every leg, so this normally names
// the people who never finished picking.
func membersShort(members []db.SlipMember, legs []db.Leg, limit int) []string {
	counts := make(map[int64]int, len(members))
	for _, leg := range legs {
		counts[leg.UserID]++
	}
	names := make([]string, 0, len(members))
	for _, member := range members {
		if counts[member.User.ID] < limit {
			names = append(names, member.User.Name)
		}
	}
	return names
}

// FilterGamesForSlip retains only games eligible for slip. Custom slips keep
// only games listed in slip.GameIDs; all other scopes delegate to
// FilterGamesByScope.
func FilterGamesForSlip(games []db.Game, slip db.Slip) []db.Game {
	if slip.SlateScope == db.SlateScopeCustom {
		if len(games) == 0 || len(slip.GameIDs) == 0 {
			return nil
		}
		allowed := make(map[string]struct{}, len(slip.GameIDs))
		for _, id := range slip.GameIDs {
			allowed[id] = struct{}{}
		}
		filtered := make([]db.Game, 0, len(slip.GameIDs))
		for _, game := range games {
			if _, ok := allowed[game.ID]; ok {
				filtered = append(filtered, game)
			}
		}
		return filtered
	}
	return FilterGamesByScope(games, slip.SlateScope)
}

// ScopeBadge returns the display label for a slip's slate scope badge.
// Custom slips render "1 Game" or "N Games".
func ScopeBadge(slip db.Slip) string {
	if slip.SlateScope == db.SlateScopeCustom {
		if len(slip.GameIDs) == 1 {
			return "1 Game"
		}
		return fmt.Sprintf("%d Games", len(slip.GameIDs))
	}
	return slip.SlateScope.Label()
}

// FilterGamesByScope retains only games that match the slip's slate scope.
func FilterGamesByScope(games []db.Game, scope db.SlateScope) []db.Game {
	if len(games) == 0 {
		return nil
	}
	filtered := make([]db.Game, 0, len(games))
	for _, game := range games {
		if scope.Matches(game.Kickoff) {
			filtered = append(filtered, game)
		}
	}
	return filtered
}

// firstKickoff is the earliest game still ahead of now, or nil when the week
// has none left.
func firstKickoff(games []db.Game, now time.Time) *time.Time {
	var first *time.Time
	for _, game := range games {
		if !now.Before(game.Kickoff) {
			continue
		}
		when := game.Kickoff
		if first == nil || when.Before(*first) {
			first = &when
		}
	}
	return first
}

// noGamesLeft reports whether every stored game in the week has kicked off. An
// empty week is unknown rather than closed, so the pick action still leads
// somewhere.
func noGamesLeft(games []db.Game, now time.Time) bool {
	if len(games) == 0 {
		return false
	}
	for _, game := range games {
		if now.Before(game.Kickoff) {
			return false
		}
	}
	return true
}

// NameList joins names the way a sentence does: "Ben", "Ben and Sam", or
// "Ben, Sam, and Kim".
func NameList(names []string) string {
	switch len(names) {
	case 0:
		return "everyone"
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}
