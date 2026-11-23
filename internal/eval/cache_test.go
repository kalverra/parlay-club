package eval_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func seedGradedSlipForGroup(
	t *testing.T,
	store *db.Store,
	group db.Group,
	user db.User,
	week db.Week,
	title, gameID string,
	result parlay.Result,
) {
	t.Helper()
	ctx := t.Context()

	slip, err := store.CreateSlip(
		ctx,
		group.ID,
		week.ID,
		nil,
		title,
		db.SlateScopeFullWeek,
		1,
		[]int64{user.ID},
	)
	require.NoError(t, err)

	leg, err := store.CreateLeg(ctx, db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &gameID,
		MarketType: "ML",
		Selection:  "BUF ML",
	})
	require.NoError(t, err)
	require.NoError(t, store.SetLegResult(ctx, leg.ID, result, user.Name))
	require.NoError(t, store.SetSlipGrade(ctx, slip.ID, result, db.SlipGraded))
}

func TestCacheGetOrComputeAndInvalidate(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	groupA, userA := seedGroupAndMember(t, store, "ada")
	groupB, userB := seedGroupAndMember(t, store, "sam")

	week1, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 1)
	require.NoError(t, err)
	_, err = store.UpsertGame(ctx, db.Game{
		ID:       "g-cache-1",
		WeekID:   week1.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  time.Date(2025, 9, 7, 17, 0, 0, 0, time.UTC),
		Status:   db.GameFinal,
	})
	require.NoError(t, err)

	seedGradedSlipForGroup(t, store, groupA, userA, week1, "Slip A1", "g-cache-1", parlay.Won)
	seedGradedSlipForGroup(t, store, groupB, userB, week1, "Slip B1", "g-cache-1", parlay.Lost)

	cache := eval.NewCache()

	// First compute for group A and group B.
	reportA1, err := cache.GetOrCompute(ctx, store, groupA.ID, 2025)
	require.NoError(t, err)
	require.Len(t, reportA1.Reports, 1)
	assert.Equal(t, 1, reportA1.Coverage.TotalLegs)

	reportB1, err := cache.GetOrCompute(ctx, store, groupB.ID, 2025)
	require.NoError(t, err)
	require.Len(t, reportB1.Reports, 1)
	assert.Equal(t, 1, reportB1.Coverage.TotalLegs)

	// Add another settled slip to both groups in DB without invalidating yet.
	seedGradedSlipForGroup(t, store, groupA, userA, week1, "Slip A2", "g-cache-1", parlay.Won)
	seedGradedSlipForGroup(t, store, groupB, userB, week1, "Slip B2", "g-cache-1", parlay.Won)

	// Cache hits should still return 1 leg for both groups.
	hitA, err := cache.GetOrCompute(ctx, store, groupA.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 1, hitA.Coverage.TotalLegs)

	hitB, err := cache.GetOrCompute(ctx, store, groupB.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 1, hitB.Coverage.TotalLegs)

	// Invalidate only group A: group A recomputes (2 legs), group B stays cached (1 leg).
	cache.InvalidateGroup(groupA.ID)

	reportA2, err := cache.GetOrCompute(ctx, store, groupA.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, reportA2.Coverage.TotalLegs)

	stillCachedB, err := cache.GetOrCompute(ctx, store, groupB.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 1, stillCachedB.Coverage.TotalLegs)

	// InvalidateAll clears group B as well.
	cache.InvalidateAll()
	reportB2, err := cache.GetOrCompute(ctx, store, groupB.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, reportB2.Coverage.TotalLegs)
}

func TestCacheConcurrency(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	group, user := seedGroupAndMember(t, store, "concurrent")
	week1, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 1)
	require.NoError(t, err)
	_, err = store.UpsertGame(ctx, db.Game{
		ID:       "g-conc-1",
		WeekID:   week1.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  time.Date(2025, 9, 7, 17, 0, 0, 0, time.UTC),
		Status:   db.GameFinal,
	})
	require.NoError(t, err)
	seedGradedSlipForGroup(t, store, group, user, week1, "Conc Slip 1", "g-conc-1", parlay.Won)

	cache := eval.NewCache()

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func(id int) {
			defer wg.Done()
			for j := range 10 {
				report, err := cache.GetOrCompute(ctx, store, group.ID, 2025)
				assert.NoError(t, err)
				assert.Equal(t, 1, report.Coverage.TotalLegs)
				if (id+j)%5 == 0 {
					cache.InvalidateGroup(group.ID)
				}
				if (id+j)%7 == 0 {
					cache.InvalidateAll()
				}
			}
		}(i)
	}
	wg.Wait()
}
