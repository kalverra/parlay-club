package slips_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// fakeStore answers the two queries CreateDefaults makes and records their
// arguments.
type fakeStore struct {
	previous    db.Slip
	previousErr error
	memberIDs   []int64
	memberErr   error

	gotGroupID    int64
	gotWeekID     int64
	gotMemberSlip int64
}

func (f *fakeStore) PreviousSlip(_ context.Context, groupID, weekID int64) (db.Slip, error) {
	f.gotGroupID = groupID
	f.gotWeekID = weekID
	return f.previous, f.previousErr
}

func (f *fakeStore) SlipMemberIDs(_ context.Context, slipID int64) ([]int64, error) {
	f.gotMemberSlip = slipID
	return f.memberIDs, f.memberErr
}

func TestCreateDefaultsUsesTheBuiltInValuesWithoutAPreviousSlip(t *testing.T) {
	t.Parallel()

	store := &fakeStore{previousErr: db.ErrSlipNotFound}

	defaults, err := slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.NoError(t, err)
	assert.Equal(t, slips.DefaultLegsPerMember, defaults.LegsPerMember)
	assert.NotNil(t, defaults.MemberIDs)
	assert.Empty(t, defaults.MemberIDs)
	assert.Equal(t, int64(7), store.gotGroupID)
	assert.Equal(t, int64(2026209), store.gotWeekID)
}

func TestCreateDefaultsCopiesThePreviousSlip(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		previous:  db.Slip{ID: 42, LegsPerMember: 4},
		memberIDs: []int64{2, 5, 9},
	}

	defaults, err := slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.NoError(t, err)
	assert.Equal(t, 4, defaults.LegsPerMember)
	assert.Equal(t, []int64{2, 5, 9}, defaults.MemberIDs)
	assert.Equal(t, int64(42), store.gotMemberSlip)
}

func TestCreateDefaultsReportsStoreErrors(t *testing.T) {
	t.Parallel()

	stop := errors.New("stop")
	store := &fakeStore{previousErr: stop}

	_, err := slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.ErrorIs(t, err, stop)

	store = &fakeStore{previous: db.Slip{ID: 42, LegsPerMember: 2}, memberErr: stop}
	_, err = slips.CreateDefaults(t.Context(), store, 7, 2026209)
	require.ErrorIs(t, err, stop)
}
