package slips_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

// recordingNotifier keeps the events the rules raise.
type recordingNotifier struct {
	events []slips.Event
}

func (n *recordingNotifier) Notify(_ context.Context, e slips.Event) {
	n.events = append(n.events, e)
}

func TestAddLegRaisesLegsAdded(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)

	_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID:     f.slip.ID,
		GameID:     &f.upcoming,
		MarketType: "ML",
		Selection:  "BUF ML",
	})
	require.NoError(t, err)

	require.Len(t, notifier.events, 1)
	assert.Equal(t, slips.Event{
		Kind:      slips.EventLegsAdded,
		SlipID:    f.slip.ID,
		ActorID:   f.ada.ID,
		Count:     1,
		Selection: "BUF ML",
	}, notifier.events[0])
}

func TestAddLegsRaisesOneLegsAdded(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)

	created, err := f.service.AddLegs(t.Context(), f.ada.ID, f.slip.ID, []db.Leg{
		{GameID: &f.upcoming, MarketType: "ML", Selection: "BUF ML"},
		{GameID: &f.upcoming, MarketType: "TOTAL", Selection: "Over 45.5"},
	})
	require.NoError(t, err)
	require.Len(t, created, 2)

	require.Len(t, notifier.events, 1)
	assert.Equal(t, slips.EventLegsAdded, notifier.events[0].Kind)
	assert.Equal(t, 2, notifier.events[0].Count)
	assert.Empty(t, notifier.events[0].Selection, "a batch does not name one pick")
}

func TestAddLegsWithOnlyDuplicatesRaisesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	_, err := f.service.AddLeg(t.Context(), f.ada.ID, db.Leg{
		SlipID:     f.slip.ID,
		GameID:     &f.upcoming,
		MarketType: "ML",
		Selection:  "BUF ML",
	})
	require.NoError(t, err)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)

	created, err := f.service.AddLegs(t.Context(), f.ada.ID, f.slip.ID, []db.Leg{
		{GameID: &f.upcoming, MarketType: "ML", Selection: "BUF ML"},
	})
	require.NoError(t, err)
	assert.Empty(t, created)
	assert.Empty(t, notifier.events)
}

func TestLockInRaisesSlipLockedOnlyForTheFinalLock(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)
	addTwoLegs(t, f, f.ada)

	notifier.events = nil
	require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
	assert.Empty(t, notifier.events, "one member locking in does not lock the slip")

	addTwoLegs(t, f, f.sam)
	notifier.events = nil
	require.NoError(t, f.service.LockIn(t.Context(), f.sam.ID, f.slip.ID))

	require.Len(t, notifier.events, 1)
	assert.Equal(t, slips.Event{
		Kind:    slips.EventSlipLocked,
		SlipID:  f.slip.ID,
		ActorID: f.sam.ID,
	}, notifier.events[0])
}

func TestLockInThatFailsRaisesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)

	err := f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID)
	require.ErrorIs(t, err, slips.ErrLegsNotReady)
	assert.Empty(t, notifier.events, "a rolled-back transaction raises nothing")
}

func TestSetLegResultRaisesSlipGradedOnlyOnTheMoveToGraded(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	legs := addFourLegs(t, f)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)

	for _, leg := range legs[:3] {
		_, err := f.service.SetLegResult(t.Context(), f.ada.ID, leg.ID, parlay.Won)
		require.NoError(t, err)
	}
	assert.Empty(t, notifier.events, "a partially graded slip stays pending")

	_, err := f.service.SetLegResult(t.Context(), f.ada.ID, legs[3].ID, parlay.Won)
	require.NoError(t, err)
	require.Len(t, notifier.events, 1)
	assert.Equal(t, slips.Event{
		Kind:    slips.EventSlipGraded,
		SlipID:  f.slip.ID,
		ActorID: f.ada.ID,
	}, notifier.events[0])

	notifier.events = nil
	_, err = f.service.SetLegResult(t.Context(), f.ada.ID, legs[3].ID, parlay.Lost)
	require.NoError(t, err)
	assert.Empty(t, notifier.events, "a correction after grading does not raise again")
}

func TestAutoGradeLegsRaisesSlipGradedOnce(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	legs := addFourLegs(t, f)
	notifier := &recordingNotifier{}
	f.service.SetNotifier(notifier)

	grades := make([]slips.AutoGrade, 0, len(legs))
	for _, leg := range legs {
		grades = append(grades, slips.AutoGrade{LegID: leg.ID, Result: parlay.Won})
	}
	require.NoError(t, f.service.AutoGradeLegs(t.Context(), grades))

	require.Len(t, notifier.events, 1)
	assert.Equal(t, slips.Event{
		Kind:   slips.EventSlipGraded,
		SlipID: f.slip.ID,
	}, notifier.events[0], "the auto grader has no actor")

	notifier.events = nil
	require.NoError(t, f.service.AutoGradeLegs(t.Context(), grades))
	assert.Empty(t, notifier.events, "a second pass over graded legs raises nothing")
}

// addTwoLegs gives one member exactly the two legs the fixture slip expects.
func addTwoLegs(t *testing.T, f *fixture, member db.User) {
	t.Helper()

	for i := range 2 {
		_, err := f.service.AddLeg(t.Context(), member.ID, db.Leg{
			SlipID:     f.slip.ID,
			GameID:     &f.upcoming,
			MarketType: "ML",
			Selection:  fmt.Sprintf("%s leg %d", member.Name, i+1),
		})
		require.NoError(t, err)
	}
}

// addFourLegs writes two legs for each member straight to the store, so the
// grading tests start with picks and no events.
func addFourLegs(t *testing.T, f *fixture) []db.Leg {
	t.Helper()

	members := []db.User{f.ada, f.sam}
	var legs []db.Leg
	for _, member := range members {
		for i := range 2 {
			leg, err := f.store.CreateLeg(t.Context(), db.Leg{
				SlipID:     f.slip.ID,
				UserID:     member.ID,
				GameID:     &f.upcoming,
				MarketType: "ML",
				Selection:  fmt.Sprintf("%s leg %d", member.Name, i+1),
			})
			require.NoError(t, err)
			legs = append(legs, leg)
		}
	}
	return legs
}
