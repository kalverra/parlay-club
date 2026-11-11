package slips_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// votesOn reads a slip's stored votes.
func votesOn(t *testing.T, f *fixture) []db.LegVote {
	t.Helper()

	votes, err := f.store.VotesForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	return votes
}

// voteOnSamVotes is the common setup: one leg of Ada's and Sam's up vote,
// with the activity and publish it made forgotten.
func voteOnSamVotes(t *testing.T, f *fixture) db.Leg {
	t.Helper()

	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")
	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	f.clearHistory(t)
	return leg
}

func TestVoteUpAndDown(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")

	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	votes := votesOn(t, f)
	require.Len(t, votes, 1)
	assert.Equal(t, f.sam.ID, votes[0].UserID)
	assert.Equal(t, 1, votes[0].Value)

	f.clearHistory(t)
	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, -1))
	votes = votesOn(t, f)
	require.Len(t, votes, 1)
	assert.Equal(t, f.sam.ID, votes[0].UserID)
	assert.Equal(t, -1, votes[0].Value, "a second vote replaces the first")
	assert.Equal(t, []string{"slip:1"}, f.published.topics)
	assert.Empty(t, activity(t, f.store, f.slip.ID), "a vote writes no activity entry")
}

func TestVoteClearRemovesTheVote(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := voteOnSamVotes(t, f)

	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 0))
	assert.Empty(t, votesOn(t, f))
	assert.Equal(t, []string{"slip:1"}, f.published.topics)
	assert.Empty(t, activity(t, f.store, f.slip.ID))
}

func TestVoteAgainChangesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := voteOnSamVotes(t, f)

	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	assert.Empty(t, f.published.topics, "a repeated vote publishes nothing")
	assert.Empty(t, activity(t, f.store, f.slip.ID))
	require.Len(t, votesOn(t, f), 1)
}

func TestVoteClearWithoutAVoteChangesNothing(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")

	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 0))
	assert.Empty(t, f.published.topics)
	assert.Empty(t, activity(t, f.store, f.slip.ID))
}

func TestVoteRefusesYourOwnLeg(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")

	assertRefused(t, f, f.service.Vote(t.Context(), f.ada.ID, leg.ID, 1), slips.ErrOwnVote)
}

func TestVoteRefusesANonMember(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")

	assertRefused(t, f, f.service.Vote(t.Context(), f.kim.ID, leg.ID, 1), slips.ErrNotMember)
}

func TestVoteRefusesAnUnknownValue(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")

	assertRefused(t, f, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 2), slips.ErrVoteInvalid)
}

func TestVoteClosesWhenEveryoneLocksInAndReopensOnUnlock(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")
	addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "MIA ML")
	addLeg(t, f.store, f.slip, f.sam, &f.upcoming, "MIA ML")
	addLeg(t, f.store, f.slip, f.sam, &f.upcoming, "BUF ML")
	require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID))
	require.NoError(t, f.service.LockIn(t.Context(), f.sam.ID, f.slip.ID))
	require.Equal(t, db.SlipLocked, reload(t, f.store, f.slip.ID).Status)
	f.clearHistory(t)

	assertRefused(t, f, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1), slips.ErrVotesClosed)

	require.NoError(t, f.service.Unlock(t.Context(), f.ada.ID, f.slip.ID))
	f.clearHistory(t)
	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	assert.Len(t, votesOn(t, f), 1)
	assert.Empty(t, activity(t, f.store, f.slip.ID))
}

func TestVoteStaysOpenWhileTheSlipIsOpen(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "BUF ML")
	addLeg(t, f.store, f.slip, f.ada, &f.upcoming, "MIA ML")
	addLeg(t, f.store, f.slip, f.sam, &f.upcoming, "MIA ML")
	require.NoError(t, f.service.LockIn(t.Context(), f.ada.ID, f.slip.ID), "one lock keeps the slip OPEN")
	f.clearHistory(t)

	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	assert.Len(t, votesOn(t, f), 1)
}

func TestVoteDoesNotCloseAtKickoff(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addLeg(t, f.store, f.slip, f.ada, &f.started, "BUF ML")

	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1),
		"a kicked-off game does not close the vote")
	assert.Len(t, votesOn(t, f), 1)
}

func TestUpdateLegClearsVotesWhenThePickChanges(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addSpreadLeg(t, f.store, f.slip, f.ada, "BUF -3.5", 3.5)
	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	f.clearHistory(t)

	changed := leg
	changed.Line = new(-4.5)
	changed.Selection = "BUF -4.5"
	_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, changed)
	require.NoError(t, err)

	assert.Empty(t, votesOn(t, f), "a new pick starts with no votes")
	assert.Len(t, activity(t, f.store, f.slip.ID), 1, "the edit still writes its activity entry")
}

func TestUpdateLegKeepsVotesWhenOnlyTheOddsChange(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := addSpreadLeg(t, f.store, f.slip, f.ada, "BUF -3.5", 3.5)
	require.NoError(t, f.service.Vote(t.Context(), f.sam.ID, leg.ID, 1))
	f.clearHistory(t)

	repriced := leg
	repriced.Odds = -120
	_, err := f.service.UpdateLeg(t.Context(), f.ada.ID, repriced)
	require.NoError(t, err)

	votes := votesOn(t, f)
	require.Len(t, votes, 1, "an odds-only edit is the same pick at a new price")
	assert.Equal(t, 1, votes[0].Value)
}

func TestDeleteLegCascadesItsVotes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg := voteOnSamVotes(t, f)
	require.Len(t, votesOn(t, f), 1)

	require.NoError(t, f.service.DeleteLeg(t.Context(), f.ada.ID, leg.ID))
	assert.Empty(t, votesOn(t, f))
}
