package slips_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

func TestSetWagerAllowsGroupMemberAndPublishes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	// Pat is in the group, not on the slip.
	pat, err := f.store.CreateUserWithIdentity(t.Context(), "Pat", "", "", "")
	require.NoError(t, err)
	_, err = f.store.JoinByToken(t.Context(), pat.ID, f.group.InviteToken)
	require.NoError(t, err)

	saved, err := f.service.SetWager(t.Context(), pat.ID, f.slip.ID, slips.WagerInput{
		Book:     "  FanDuel  ",
		Stake:    25,
		Odds:     350,
		BoostPct: 25,
	})
	require.NoError(t, err)
	require.NotNil(t, saved.Book)
	assert.Equal(t, "FanDuel", *saved.Book)
	assert.InDelta(t, float64(25), saved.Stake, 1e-9)
	assert.Equal(t, 350, saved.Odds)
	assert.InDelta(t, float64(25), saved.BoostPct, 1e-9)
	assert.Contains(t, f.published.topics, fmt.Sprintf("slip:%d", f.slip.ID))

	got, found, err := f.store.WagerForUser(t.Context(), f.slip.ID, pat.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, saved, got)
}

func TestSetWagerRejectsOutsiderAndInvalidInputs(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	outsider, err := f.store.CreateUserWithIdentity(t.Context(), "Outsider", "", "", "")
	require.NoError(t, err)

	_, err = f.service.SetWager(t.Context(), outsider.ID, f.slip.ID, slips.WagerInput{
		Stake: 10,
		Odds:  200,
	})
	require.ErrorIs(t, err, slips.ErrNotMember)

	cases := []struct {
		name  string
		input slips.WagerInput
		want  error
	}{
		{name: "zero stake", input: slips.WagerInput{Stake: 0, Odds: 200}, want: slips.ErrWagerStakeInvalid},
		{name: "negative stake", input: slips.WagerInput{Stake: -5, Odds: 200}, want: slips.ErrWagerStakeInvalid},
		{name: "nan stake", input: slips.WagerInput{Stake: math.NaN(), Odds: 200}, want: slips.ErrWagerStakeInvalid},
		{name: "inf stake", input: slips.WagerInput{Stake: math.Inf(1), Odds: 200}, want: slips.ErrWagerStakeInvalid},
		{name: "zero odds", input: slips.WagerInput{Stake: 10, Odds: 0}, want: slips.ErrWagerOddsInvalid},
		{name: "odds 50", input: slips.WagerInput{Stake: 10, Odds: 50}, want: slips.ErrWagerOddsInvalid},
		{name: "odds -99", input: slips.WagerInput{Stake: 10, Odds: -99}, want: slips.ErrWagerOddsInvalid},
		{
			name:  "negative boost",
			input: slips.WagerInput{Stake: 10, Odds: -110, BoostPct: -1},
			want:  slips.ErrWagerBoostInvalid,
		},
		{
			name:  "nan boost",
			input: slips.WagerInput{Stake: 10, Odds: -110, BoostPct: math.NaN()},
			want:  slips.ErrWagerBoostInvalid,
		},
		{
			name:  "book too long",
			input: slips.WagerInput{Stake: 10, Odds: 100, Book: strings.Repeat("x", 41)},
			want:  slips.ErrWagerBookTooLong,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.service.SetWager(t.Context(), f.ada.ID, f.slip.ID, tc.input)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestDeleteWagerChecksMembershipAndRemovesRow(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	outsider, err := f.store.CreateUserWithIdentity(t.Context(), "Outsider", "", "", "")
	require.NoError(t, err)

	_, err = f.service.SetWager(t.Context(), f.ada.ID, f.slip.ID, slips.WagerInput{
		Stake: 20,
		Odds:  -110,
	})
	require.NoError(t, err)

	err = f.service.DeleteWager(t.Context(), outsider.ID, f.slip.ID)
	require.ErrorIs(t, err, slips.ErrNotMember)

	require.NoError(t, f.service.DeleteWager(t.Context(), f.ada.ID, f.slip.ID))

	_, found, err := f.store.WagerForUser(t.Context(), f.slip.ID, f.ada.ID)
	require.NoError(t, err)
	assert.False(t, found)

	// Missing slip returns ErrSlipNotFound.
	err = f.service.DeleteWager(t.Context(), f.ada.ID, 999999)
	require.ErrorIs(t, err, db.ErrSlipNotFound)
}
