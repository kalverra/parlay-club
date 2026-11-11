package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// voteFixture is a slip with one leg and two members, ready for votes.
type voteFixture struct {
	store *db.Store
	slip  db.Slip
	leg   db.Leg
	ada   db.User
	sam   db.User
}

// newVoteFixture builds a slip owned by Ada with Sam on it and one leg for Ada.
func newVoteFixture(t *testing.T) voteFixture {
	t.Helper()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	sam := createUser(t, store, "Sam")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada, sam)
	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ada.ID,
		MarketType: "ML",
		Selection:  "BUF ML",
	})
	require.NoError(t, err)
	return voteFixture{store: store, slip: slip, leg: leg, ada: ada, sam: sam}
}

func TestSetLegVoteUpsertsOneVotePerPerson(t *testing.T) {
	t.Parallel()

	f := newVoteFixture(t)
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	require.NoError(t, f.store.SetLegVote(t.Context(), f.leg.ID, f.sam.ID, 1, at))

	value, err := f.store.LegVoteValue(t.Context(), f.leg.ID, f.sam.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, value)

	later := at.Add(time.Hour)
	require.NoError(t, f.store.SetLegVote(t.Context(), f.leg.ID, f.sam.ID, -1, later))

	value, err = f.store.LegVoteValue(t.Context(), f.leg.ID, f.sam.ID)
	require.NoError(t, err)
	assert.Equal(t, -1, value, "voting again replaces the row")

	votes, err := f.store.VotesForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	require.Len(t, votes, 1)
	assert.Equal(t, f.sam.ID, votes[0].UserID)
	assert.Equal(t, -1, votes[0].Value)
	assert.Equal(t, later, votes[0].UpdatedAt)
}

func TestLegVoteValueIsZeroWithoutAVote(t *testing.T) {
	t.Parallel()

	f := newVoteFixture(t)

	value, err := f.store.LegVoteValue(t.Context(), f.leg.ID, f.sam.ID)
	require.NoError(t, err)
	assert.Zero(t, value)
}

func TestClearLegVoteRemovesOnlyThatVote(t *testing.T) {
	t.Parallel()

	f := newVoteFixture(t)
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	require.NoError(t, f.store.SetLegVote(t.Context(), f.leg.ID, f.sam.ID, 1, at))
	require.NoError(t, f.store.SetLegVote(t.Context(), f.leg.ID, f.ada.ID, -1, at))

	require.NoError(t, f.store.ClearLegVote(t.Context(), f.leg.ID, f.sam.ID))

	votes, err := f.store.VotesForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	require.Len(t, votes, 1)
	assert.Equal(t, f.ada.ID, votes[0].UserID)

	value, err := f.store.LegVoteValue(t.Context(), f.leg.ID, f.sam.ID)
	require.NoError(t, err)
	assert.Zero(t, value)
}

func TestVotesForSlipFollowsTheLegs(t *testing.T) {
	t.Parallel()

	f := newVoteFixture(t)
	other, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     f.slip.ID,
		UserID:     f.sam.ID,
		MarketType: "ML",
		Selection:  "MIA ML",
	})
	require.NoError(t, err)
	second := createTestSlip(t, f.store, createTestGroup(t, f.store, f.ada), createTestWeek(t, f.store, 9),
		"Week 9", 2, f.ada, f.sam)
	elsewhere, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID:     second.ID,
		UserID:     f.sam.ID,
		MarketType: "ML",
		Selection:  "NE ML",
	})
	require.NoError(t, err)

	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	require.NoError(t, f.store.SetLegVote(t.Context(), f.leg.ID, f.sam.ID, 1, at))
	require.NoError(t, f.store.SetLegVote(t.Context(), other.ID, f.ada.ID, -1, at))
	require.NoError(t, f.store.SetLegVote(t.Context(), elsewhere.ID, f.ada.ID, 1, at))

	votes, err := f.store.VotesForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	require.Len(t, votes, 2)
	assert.Equal(t, f.leg.ID, votes[0].LegID)
	assert.Equal(t, other.ID, votes[1].LegID)

	require.NoError(t, f.store.DeleteLegVotes(t.Context(), f.leg.ID))
	votes, err = f.store.VotesForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	require.Len(t, votes, 1)
	assert.Equal(t, other.ID, votes[0].LegID)
}

func TestDeleteLegCascadesItsVotes(t *testing.T) {
	t.Parallel()

	f := newVoteFixture(t)
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	require.NoError(t, f.store.SetLegVote(t.Context(), f.leg.ID, f.sam.ID, 1, at))

	require.NoError(t, f.store.DeleteLeg(t.Context(), f.leg.ID))

	votes, err := f.store.VotesForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	assert.Empty(t, votes)
}

func TestSetLegVoteRefusesAZeroValue(t *testing.T) {
	t.Parallel()

	f := newVoteFixture(t)

	err := f.store.SetLegVote(t.Context(), f.leg.ID, f.sam.ID, 0,
		time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC))
	require.Error(t, err, "the schema only stores -1 and 1; clearing deletes the row")
}
