package slips_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// nextNow is the injected clock the step tests run on: Sunday at noon UTC,
// before the 1:00 PM ET window.
var nextNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// nextUser is one person on the test slips.
func nextUser(id int64, name string) db.User {
	return db.User{ID: id, Name: name}
}

// nextMember is one membership, locked when locked is true.
func nextMember(user db.User, locked bool) db.SlipMember {
	member := db.SlipMember{User: user}
	if locked {
		at := nextNow.Add(-time.Hour)
		member.LockedAt = &at
	}
	return member
}

// nextLeg is one pick owned by userID.
func nextLeg(userID int64) db.Leg {
	return db.Leg{UserID: userID, Selection: "BUF ML"}
}

// nextGame is one scheduled game.
func nextGame(id string, kickoff time.Time) db.Game {
	return db.Game{ID: id, Kickoff: kickoff, Status: db.GameScheduled}
}

func TestNextStepCoversEveryState(t *testing.T) {
	t.Parallel()

	ada := nextUser(1, "Ada")
	sam := nextUser(2, "Sam")
	kim := nextUser(3, "Kim")
	open := db.Slip{LegsPerMember: 2, Status: db.SlipOpen, Result: parlay.Pending}
	upcoming := nextGame("401772001", nextNow.Add(2*time.Hour))
	kickedOff := nextGame("401772002", nextNow.Add(-time.Hour))

	tests := []struct {
		name      string
		slip      db.Slip
		members   []db.SlipMember
		legs      []db.Leg
		games     []db.Game
		viewerID  int64
		wantStep  slips.Step
		wantNeed  int
		wantOn    []string
		wantLabel string
	}{
		{
			name:      "a group member off the slip joins",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			viewerID:  99,
			wantStep:  slips.StepJoin,
			wantLabel: "Join this slip",
		},
		{
			name:      "fewer legs than the limit picks",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepPick,
			wantNeed:  2,
			wantLabel: "Pick 2 more",
		},
		{
			name:      "one leg left says pick one more",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			legs:      []db.Leg{nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepPick,
			wantNeed:  1,
			wantLabel: "Pick 1 more",
		},
		{
			name:      "every leg in asks for the lock",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepLockIn,
			wantLabel: "Ready to lock in",
		},
		{
			name:      "a locked viewer waits on the rest",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, true), nextMember(sam, false)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepWaiting,
			wantOn:    []string{"Sam"},
			wantLabel: "Waiting on Sam",
		},
		{
			name:      "everyone locked in can place",
			slip:      db.Slip{LegsPerMember: 2, Status: db.SlipLocked, Result: parlay.Pending},
			members:   []db.SlipMember{nextMember(ada, true), nextMember(sam, true)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepExport,
			wantLabel: "Ready to place",
		},
		{
			name:      "the last lock is ready even before the status catches up",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, true), nextMember(sam, true)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepExport,
			wantLabel: "Ready to place",
		},
		{
			name:      "a graded slip is done",
			slip:      db.Slip{LegsPerMember: 2, Status: db.SlipGraded, Result: parlay.Won},
			members:   []db.SlipMember{nextMember(ada, true)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{kickedOff},
			viewerID:  ada.ID,
			wantStep:  slips.StepDone,
			wantLabel: "Won",
		},
		{
			name:      "every game kicked off closes the picks",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			games:     []db.Game{kickedOff},
			viewerID:  ada.ID,
			wantStep:  slips.StepClosed,
			wantNeed:  2,
			wantLabel: "Picks are closed",
		},
		{
			name: "a locked viewer sees closed when someone never finished",
			slip: open,
			members: []db.SlipMember{
				nextMember(ada, true),
				nextMember(sam, false),
			},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID), nextLeg(sam.ID)},
			games:     []db.Game{kickedOff},
			viewerID:  ada.ID,
			wantStep:  slips.StepClosed,
			wantOn:    []string{"Sam"},
			wantLabel: "Picks are closed",
		},
		{
			name: "a locked viewer still waits when everyone has their legs",
			slip: open,
			members: []db.SlipMember{
				nextMember(ada, true),
				nextMember(sam, false),
			},
			legs: []db.Leg{
				nextLeg(ada.ID), nextLeg(ada.ID),
				nextLeg(sam.ID), nextLeg(sam.ID),
			},
			games:     []db.Game{kickedOff},
			viewerID:  ada.ID,
			wantStep:  slips.StepWaiting,
			wantOn:    []string{"Sam"},
			wantLabel: "Waiting on Sam",
		},
		{
			name:      "unlocking returns a full viewer to lock in",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepLockIn,
			wantLabel: "Ready to lock in",
		},
		{
			name:      "a past week has no deadline",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, false)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{kickedOff},
			viewerID:  ada.ID,
			wantStep:  slips.StepLockIn,
			wantLabel: "Ready to lock in",
		},
		{
			name:      "three people waiting names each in order",
			slip:      open,
			members:   []db.SlipMember{nextMember(ada, true), nextMember(sam, false), nextMember(kim, false)},
			legs:      []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)},
			games:     []db.Game{upcoming},
			viewerID:  ada.ID,
			wantStep:  slips.StepWaiting,
			wantOn:    []string{"Sam", "Kim"},
			wantLabel: "Waiting on Sam and Kim",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := slips.NextStep(
				test.slip, test.members, test.legs, test.games, test.viewerID, nextNow,
			)

			assert.Equal(t, test.wantStep, got.Step)
			assert.Equal(t, test.wantNeed, got.Need)
			assert.Equal(t, test.wantOn, got.WaitingOn)
			assert.Equal(t, test.wantLabel, got.Label())
			assert.Equal(t, test.slip.Result, got.Result)
		})
	}
}

func TestNextStepUsesTheEarliestKickoffStillAhead(t *testing.T) {
	t.Parallel()

	ada := nextUser(1, "Ada")
	london := time.Date(2026, 11, 8, 13, 30, 0, 0, time.UTC) // 9:30 AM ET
	afternoon := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	before := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	got := slips.NextStep(
		db.Slip{LegsPerMember: 2, Status: db.SlipOpen, Result: parlay.Pending},
		[]db.SlipMember{nextMember(ada, false)},
		nil,
		[]db.Game{
			nextGame("past", before.Add(-time.Hour)),
			nextGame("late", afternoon),
			nextGame("london", london),
		},
		ada.ID,
		before,
	)

	require.NotNil(t, got.Deadline)
	assert.True(t, london.Equal(*got.Deadline), "the London 9:30 a.m. ET kickoff comes first")
}

func TestNextStepWithoutGamesIsNotClosed(t *testing.T) {
	t.Parallel()

	ada := nextUser(1, "Ada")
	got := slips.NextStep(
		db.Slip{LegsPerMember: 2, Status: db.SlipOpen, Result: parlay.Pending},
		[]db.SlipMember{nextMember(ada, false)},
		nil,
		nil,
		ada.ID,
		nextNow,
	)

	assert.Equal(t, slips.StepPick, got.Step, "an empty week is unknown, not over")
	assert.Nil(t, got.Deadline)
}

func TestNextStepLabelsTheGrade(t *testing.T) {
	t.Parallel()

	ada := nextUser(1, "Ada")
	results := map[parlay.Result]string{
		parlay.Won:  "Won",
		parlay.Lost: "Lost",
		parlay.Push: "Push",
		parlay.Void: "Push",
	}
	for result, want := range results {
		got := slips.NextStep(
			db.Slip{LegsPerMember: 1, Status: db.SlipGraded, Result: result},
			[]db.SlipMember{nextMember(ada, true)},
			[]db.Leg{nextLeg(ada.ID)},
			nil,
			ada.ID,
			nextNow,
		)
		assert.Equal(t, slips.StepDone, got.Step)
		assert.Equal(t, want, got.Label())
	}
}
