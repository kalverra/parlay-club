package slips_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

func TestAutoGradeLegsGradesSlipAndNotifies(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg1, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID: f.slip.ID, UserID: f.ada.ID, GameID: &f.upcoming,
		MarketType: "ML", Selection: "BUF ML",
	})
	require.NoError(t, err)
	leg2, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID: f.slip.ID, UserID: f.sam.ID, GameID: &f.upcoming,
		MarketType: "TOTAL", Selection: "Over 45.5",
	})
	require.NoError(t, err)

	err = f.service.AutoGradeLegs(t.Context(), []slips.AutoGrade{
		{LegID: leg1.ID, Result: parlay.Won},
		{LegID: leg2.ID, Result: parlay.Won},
	})
	require.NoError(t, err)

	stored1, err := f.store.LegByID(t.Context(), leg1.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, stored1.Result)
	require.NotNil(t, stored1.GradedBy)
	assert.Equal(t, "auto", *stored1.GradedBy)

	storedSlip, err := f.store.SlipByID(t.Context(), f.slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, storedSlip.Result)
	assert.Equal(t, db.SlipGraded, storedSlip.Status)

	activity, err := f.store.ActivityForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	require.NotEmpty(t, activity)
	assert.Nil(t, activity[0].UserID)
	assert.Contains(t, activity[0].Message, "Auto-graded")

	assert.Equal(t, []string{fmt.Sprintf("slip:%d", f.slip.ID)}, f.published.topics)
}

func TestAutoGradeLegsPreservesManualOverridesAndNoOpsQuietly(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	leg1, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID: f.slip.ID, UserID: f.ada.ID, GameID: &f.upcoming,
		MarketType: "ML", Selection: "BUF ML",
	})
	require.NoError(t, err)
	leg2, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID: f.slip.ID, UserID: f.sam.ID, GameID: &f.upcoming,
		MarketType: "TOTAL", Selection: "Over 45.5",
	})
	require.NoError(t, err)

	// Ada manually grades leg1 as VOID.
	_, err = f.service.SetLegResult(t.Context(), f.ada.ID, leg1.ID, parlay.Void)
	require.NoError(t, err)
	f.published.topics = nil
	beforeActivity, err := f.store.ActivityForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)

	// AutoGradeLegs tries to overwrite leg1 as LOST, and grades leg2 as WON.
	err = f.service.AutoGradeLegs(t.Context(), []slips.AutoGrade{
		{LegID: leg1.ID, Result: parlay.Lost},
		{LegID: leg2.ID, Result: parlay.Won},
	})
	require.NoError(t, err)

	// leg1 must keep Ada's VOID grade; leg2 becomes auto WON.
	stored1, err := f.store.LegByID(t.Context(), leg1.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Void, stored1.Result)
	require.NotNil(t, stored1.GradedBy)
	assert.Equal(t, f.ada.Name, *stored1.GradedBy)

	stored2, err := f.store.LegByID(t.Context(), leg2.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, stored2.Result)
	require.NotNil(t, stored2.GradedBy)
	assert.Equal(t, "auto", *stored2.GradedBy)

	storedSlip, err := f.store.SlipByID(t.Context(), f.slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, storedSlip.Result)
	assert.Equal(t, db.SlipGraded, storedSlip.Status)

	// One new system activity entry and one publish.
	afterActivity, err := f.store.ActivityForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	require.Len(t, afterActivity, len(beforeActivity)+1)
	assert.Nil(t, afterActivity[0].UserID)
	assert.Len(t, f.published.topics, 1)

	// Running AutoGradeLegs again when nothing is PENDING writes no activity and publishes nothing.
	f.published.topics = nil
	err = f.service.AutoGradeLegs(t.Context(), []slips.AutoGrade{
		{LegID: leg1.ID, Result: parlay.Lost},
		{LegID: leg2.ID, Result: parlay.Lost},
	})
	require.NoError(t, err)
	afterNoOp, err := f.store.ActivityForSlip(t.Context(), f.slip.ID)
	require.NoError(t, err)
	assert.Len(t, afterNoOp, len(afterActivity))
	assert.Empty(t, f.published.topics)
}

func TestOnGradeChangeCallback(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	var changedGroups []int64
	f.service.SetOnGradeChange(func(groupID int64) {
		changedGroups = append(changedGroups, groupID)
	})

	leg1, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID: f.slip.ID, UserID: f.ada.ID, GameID: &f.upcoming,
		MarketType: "ML", Selection: "BUF ML",
	})
	require.NoError(t, err)
	leg2, err := f.store.CreateLeg(t.Context(), db.Leg{
		SlipID: f.slip.ID, UserID: f.sam.ID, GameID: &f.upcoming,
		MarketType: "TOTAL", Selection: "Over 45.5",
	})
	require.NoError(t, err)

	// SetLegResult invokes callback once on change, and not on repeated identical grade.
	_, err = f.service.SetLegResult(t.Context(), f.ada.ID, leg1.ID, parlay.Won)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.group.ID}, changedGroups)

	_, err = f.service.SetLegResult(t.Context(), f.ada.ID, leg1.ID, parlay.Won)
	require.NoError(t, err)
	assert.Equal(t, []int64{f.group.ID}, changedGroups)

	// AutoGradeLegs invokes callback once per distinct group with updated legs.
	changedGroups = nil
	err = f.service.AutoGradeLegs(t.Context(), []slips.AutoGrade{
		{LegID: leg2.ID, Result: parlay.Won},
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{f.group.ID}, changedGroups)

	// No-op AutoGradeLegs does not invoke callback.
	changedGroups = nil
	err = f.service.AutoGradeLegs(t.Context(), []slips.AutoGrade{
		{LegID: leg2.ID, Result: parlay.Lost},
	})
	require.NoError(t, err)
	assert.Empty(t, changedGroups)
}
