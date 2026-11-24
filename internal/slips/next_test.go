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

func TestNextStepSlateScopeDeadlinesAndClosure(t *testing.T) {
	t.Parallel()

	ada := nextUser(1, "Ada")
	sam := nextUser(2, "Sam")

	// 2026-11-05 is Thursday, 2026-11-08 is Sunday, 2026-11-09 is Monday.
	thuKickoff := time.Date(2026, 11, 6, 1, 15, 0, 0, time.UTC)  // Thursday 20:15 ET
	sunKickoff := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)  // Sunday 13:00 ET
	monKickoff := time.Date(2026, 11, 10, 1, 15, 0, 0, time.UTC) // Monday 20:15 ET

	games := []db.Game{
		nextGame("thu", thuKickoff),
		nextGame("sun", sunKickoff),
		nextGame("mon", monKickoff),
	}

	thuSlip := db.Slip{
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeThursday,
		Result:        parlay.Pending,
	}
	sunSlip := db.Slip{
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeSundayMain,
		Result:        parlay.Pending,
	}
	fullSlip := db.Slip{
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeFullWeek,
		Result:        parlay.Pending,
	}

	// 1. Before Thursday kickoff: Thursday noon ET (17:00 UTC).
	thuNoon := time.Date(2026, 11, 5, 17, 0, 0, 0, time.UTC)

	// Thursday slip deadline is Thursday kickoff.
	gotThu := slips.NextStep(
		thuSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, thuNoon,
	)
	require.NotNil(t, gotThu.Deadline)
	assert.True(t, thuKickoff.Equal(*gotThu.Deadline))
	assert.Equal(t, slips.StepPick, gotThu.Step)

	// Sunday Main slip deadline ignores Thursday and snaps to Sunday kickoff.
	gotSun := slips.NextStep(
		sunSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, thuNoon,
	)
	require.NotNil(t, gotSun.Deadline)
	assert.True(t, sunKickoff.Equal(*gotSun.Deadline), "Sunday slip must ignore Thursday game")
	assert.Equal(t, slips.StepPick, gotSun.Step)

	// Full week slip deadline is the earliest game (Thursday).
	gotFull := slips.NextStep(
		fullSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, thuNoon,
	)
	require.NotNil(t, gotFull.Deadline)
	assert.True(t, thuKickoff.Equal(*gotFull.Deadline))
	assert.Equal(t, slips.StepPick, gotFull.Step)

	// 2. After Thursday kickoff but before Sunday: Friday noon ET (17:00 UTC).
	friNoon := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)

	// Thursday slip is closed because every Thursday game kicked off.
	gotThuFri := slips.NextStep(thuSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, friNoon)
	assert.Nil(t, gotThuFri.Deadline)
	assert.Equal(t, slips.StepClosed, gotThuFri.Step, "Thursday slip must close once Thursday kicks off")

	// Thursday slip with one member locked and another short also closes.
	members := []db.SlipMember{nextMember(ada, true), nextMember(sam, false)}
	legs := []db.Leg{nextLeg(ada.ID), nextLeg(ada.ID)}
	gotThuFriShort := slips.NextStep(
		thuSlip, members, legs, games, ada.ID, friNoon,
	)
	assert.Equal(t, slips.StepClosed, gotThuFriShort.Step)
	assert.Equal(t, []string{"Sam"}, gotThuFriShort.WaitingOn)

	// Sunday Main slip stays OPEN and deadline remains Sunday kickoff.
	gotSunFri := slips.NextStep(
		sunSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, friNoon,
	)
	require.NotNil(t, gotSunFri.Deadline)
	assert.True(t, sunKickoff.Equal(*gotSunFri.Deadline),
		"Sunday slip deadline stays Sunday kickoff after Thursday kicks off")
	assert.Equal(t, slips.StepPick, gotSunFri.Step, "Sunday slip remains open after Thursday kicks off")

	// Sunday Main slip with one locked member still waits on Sam (does NOT close).
	gotSunFriWait := slips.NextStep(
		sunSlip, members, legs, games, ada.ID, friNoon,
	)
	assert.Equal(t, slips.StepWaiting, gotSunFriWait.Step)
	assert.Equal(t, []string{"Sam"}, gotSunFriWait.WaitingOn)

	// Full week slip shifts deadline to Sunday kickoff and stays open.
	gotFullFri := slips.NextStep(
		fullSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, friNoon,
	)
	require.NotNil(t, gotFullFri.Deadline)
	assert.True(t, sunKickoff.Equal(*gotFullFri.Deadline))
	assert.Equal(t, slips.StepPick, gotFullFri.Step)

	// 3. After Monday night kickoff: Tuesday noon ET (17:00 UTC).
	tueNoon := time.Date(2026, 11, 10, 17, 0, 0, 0, time.UTC)

	// Both Sunday Main and Full week slips are now closed.
	gotSunTue := slips.NextStep(
		sunSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, tueNoon,
	)
	assert.Nil(t, gotSunTue.Deadline)
	assert.Equal(t, slips.StepClosed, gotSunTue.Step)

	gotFullTue := slips.NextStep(
		fullSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, tueNoon,
	)
	assert.Nil(t, gotFullTue.Deadline)
	assert.Equal(t, slips.StepClosed, gotFullTue.Step)
}

func TestWeekendSlateScopeFilterAndNextStep(t *testing.T) {
	t.Parallel()

	ada := nextUser(1, "Ada")
	// 2027-01-07 is Thursday, 2027-01-09 is Saturday, 2027-01-10 is Sunday.
	thuKickoff := time.Date(2027, 1, 8, 1, 15, 0, 0, time.UTC)  // Thursday 20:15 ET
	satKickoff := time.Date(2027, 1, 9, 21, 30, 0, 0, time.UTC) // Saturday 16:30 ET
	sunKickoff := time.Date(2027, 1, 10, 18, 0, 0, 0, time.UTC) // Sunday 13:00 ET

	games := []db.Game{
		nextGame("thu", thuKickoff),
		nextGame("sat", satKickoff),
		nextGame("sun", sunKickoff),
	}

	filtered := slips.FilterGamesByScope(games, db.SlateScopeWeekend)
	require.Len(t, filtered, 2)
	assert.Equal(t, "sat", filtered[0].ID)
	assert.Equal(t, "sun", filtered[1].ID)

	weekendSlip := db.Slip{
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeWeekend,
		Result:        parlay.Pending,
	}

	satMorning := time.Date(2027, 1, 9, 15, 0, 0, 0, time.UTC)
	got := slips.NextStep(weekendSlip, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, satMorning)
	require.NotNil(t, got.Deadline)
	assert.True(t, satKickoff.Equal(*got.Deadline), "weekend slip deadline includes Saturday playoff kickoff")
	assert.Equal(t, slips.StepPick, got.Step)
}

func TestCustomScopeFilterGamesAndScopeBadge(t *testing.T) {
	t.Parallel()

	ada := db.User{ID: 1, Name: "Ada"}
	sunEarly := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	sunLate := time.Date(2026, 11, 8, 21, 25, 0, 0, time.UTC)
	monNight := time.Date(2026, 11, 10, 1, 15, 0, 0, time.UTC)
	games := []db.Game{
		{ID: "g-sun-1", Kickoff: sunEarly},
		{ID: "g-sun-2", Kickoff: sunLate},
		{ID: "g-mon", Kickoff: monNight},
	}

	customTwo := db.Slip{
		ID:            1,
		LegsPerMember: 2,
		Status:        db.SlipOpen,
		SlateScope:    db.SlateScopeCustom,
		GameIDs:       []string{"g-sun-2", "g-mon"},
		Result:        parlay.Pending,
	}

	filtered := slips.FilterGamesForSlip(games, customTwo)
	require.Len(t, filtered, 2)
	assert.Equal(t, []string{"g-sun-2", "g-mon"}, []string{filtered[0].ID, filtered[1].ID})

	// NextStep uses only the custom slip's games for its deadline.
	sunBeforeEarly := time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC)
	got := slips.NextStep(customTwo, []db.SlipMember{nextMember(ada, false)}, nil, games, ada.ID, sunBeforeEarly)
	require.NotNil(t, got.Deadline)
	assert.True(t, sunLate.Equal(*got.Deadline))
	assert.Equal(t, slips.StepPick, got.Step)

	assert.Equal(t, "2 Games", slips.ScopeBadge(customTwo))
	assert.Equal(t, "1 Game", slips.ScopeBadge(db.Slip{SlateScope: db.SlateScopeCustom, GameIDs: []string{"g-sun-1"}}))
	assert.Equal(t, "Sunday Slate", slips.ScopeBadge(db.Slip{SlateScope: db.SlateScopeSundayMain}))
}
