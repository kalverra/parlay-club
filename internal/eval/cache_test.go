package eval_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
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

	// First compute for group A and group B (evaluates all built-in engines side-by-side).
	wantEngines := len(analyzer.BuiltInEngines())
	reportA1, err := cache.GetOrCompute(ctx, store, groupA.ID, 2025)
	require.NoError(t, err)
	require.Len(t, reportA1.Reports, wantEngines)
	require.Len(t, reportA1.Diffs, wantEngines-1)
	assert.Equal(t, "v1", reportA1.Reports[0].RubricName)
	assert.Equal(t, "v2", reportA1.Reports[1].RubricName)
	assert.NotEmpty(t, reportA1.Reports[1].Calibration.Reliability)
	assert.Equal(t, 1, reportA1.Coverage.TotalLegs)

	reportB1, err := cache.GetOrCompute(ctx, store, groupB.ID, 2025)
	require.NoError(t, err)
	require.Len(t, reportB1.Reports, wantEngines)
	assert.Equal(t, 1, reportB1.Coverage.TotalLegs)

	global1, err := cache.GetOrCompute(ctx, store, 0, 2025)
	require.NoError(t, err)
	require.Len(t, global1.Reports, wantEngines)
	assert.Equal(t, 2, global1.Coverage.TotalLegs)

	// Add another settled slip to both groups in DB without invalidating yet.
	seedGradedSlipForGroup(t, store, groupA, userA, week1, "Slip A2", "g-cache-1", parlay.Won)
	seedGradedSlipForGroup(t, store, groupB, userB, week1, "Slip B2", "g-cache-1", parlay.Won)

	// Cache hits should still return 1 leg for both groups and 2 for global.
	hitA, err := cache.GetOrCompute(ctx, store, groupA.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 1, hitA.Coverage.TotalLegs)

	hitB, err := cache.GetOrCompute(ctx, store, groupB.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 1, hitB.Coverage.TotalLegs)

	hitGlobal, err := cache.GetOrCompute(ctx, store, 0, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, hitGlobal.Coverage.TotalLegs)

	// Invalidate only group A: group A recomputes (2 legs), global recomputes (4 legs), group B stays cached (1 leg).
	cache.InvalidateGroup(groupA.ID)

	reportA2, err := cache.GetOrCompute(ctx, store, groupA.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, reportA2.Coverage.TotalLegs)

	global2, err := cache.GetOrCompute(ctx, store, 0, 2025)
	require.NoError(t, err)
	assert.Equal(t, 4, global2.Coverage.TotalLegs)

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

func TestCacheModeIsolationAndInvalidation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	group, user := seedGroupAndMember(t, store, "mode-iso")
	week1, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 1)
	require.NoError(t, err)

	kickoff := time.Date(2025, 9, 7, 17, 0, 0, 0, time.UTC)
	homeScore1, awayScore1 := 24, 17
	_, err = store.UpsertGame(ctx, db.Game{
		ID: "g-iso-1", WeekID: week1.ID, HomeTeam: "BUF", AwayTeam: "MIA",
		HomeName: "Buffalo Bills", AwayName: "Miami Dolphins",
		Kickoff: kickoff, Status: db.GameFinal, HomeScore: &homeScore1, AwayScore: &awayScore1,
	})
	require.NoError(t, err)

	seedGradedSlipForGroup(t, store, group, user, week1, "Slip 1", "g-iso-1", parlay.Won)

	linesBody := `[
		{
			"id": "evt-iso-1",
			"commence_time": "2025-09-07T17:00:00Z",
			"home_team": "Buffalo Bills",
			"away_team": "Miami Dolphins",
			"bookmakers": [{
				"key": "pinnacle",
				"markets": [{
					"key": "spreads",
					"outcomes": [
						{"name": "Buffalo Bills", "price": -110, "point": -3.5},
						{"name": "Miami Dolphins", "price": -110, "point": 3.5}
					]
				}]
			}]
		},
		{
			"id": "evt-iso-2",
			"commence_time": "2025-09-07T17:00:00Z",
			"home_team": "Kansas City Chiefs",
			"away_team": "Baltimore Ravens",
			"bookmakers": [{
				"key": "pinnacle",
				"markets": [{
					"key": "spreads",
					"outcomes": [
						{"name": "Kansas City Chiefs", "price": -110, "point": -2.5},
						{"name": "Baltimore Ravens", "price": -110, "point": 2.5}
					]
				}]
			}]
		}
	]`
	require.NoError(t, store.SaveAPIResponse(ctx, archive.Record{
		Source:    "odds",
		Key:       db.ArchiveOddsLinesKey,
		Method:    "GET",
		URL:       "https://example.test/odds",
		Status:    200,
		FetchedAt: kickoff.Add(-30 * time.Minute),
		Body:      []byte(linesBody),
	}))

	cache := eval.NewCache()

	slips1, err := cache.GetOrComputeMode(ctx, store, eval.ModeSlips, group.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 1, slips1.Coverage.TotalLegs)

	market1, err := cache.GetOrComputeMode(ctx, store, eval.ModeMarket, 0, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, market1.Coverage.TotalLegs)

	// Add a second graded slip and a second FINAL game.
	seedGradedSlipForGroup(t, store, group, user, week1, "Slip 2", "g-iso-1", parlay.Lost)
	homeScore2, awayScore2 := 27, 20
	_, err = store.UpsertGame(ctx, db.Game{
		ID: "g-iso-2", WeekID: week1.ID, HomeTeam: "KC", AwayTeam: "BAL",
		HomeName: "Kansas City Chiefs", AwayName: "Baltimore Ravens",
		Kickoff: kickoff, Status: db.GameFinal, HomeScore: &homeScore2, AwayScore: &awayScore2,
	})
	require.NoError(t, err)

	// InvalidateGroup evicts ModeSlips only; ModeMarket stays cached at 2 legs.
	cache.InvalidateGroup(group.ID)

	slips2, err := cache.GetOrComputeMode(ctx, store, eval.ModeSlips, group.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, slips2.Coverage.TotalLegs)

	marketStillCached, err := cache.GetOrComputeMode(ctx, store, eval.ModeMarket, 0, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, marketStillCached.Coverage.TotalLegs)

	// Add a third graded slip, then InvalidateMarket: ModeMarket recomputes to 4 legs, ModeSlips stays cached at 2.
	seedGradedSlipForGroup(t, store, group, user, week1, "Slip 3", "g-iso-1", parlay.Won)
	cache.InvalidateMarket()

	market2, err := cache.GetOrComputeMode(ctx, store, eval.ModeMarket, 0, 2025)
	require.NoError(t, err)
	assert.Equal(t, 4, market2.Coverage.TotalLegs)

	slipsStillCached, err := cache.GetOrComputeMode(ctx, store, eval.ModeSlips, group.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 2, slipsStillCached.Coverage.TotalLegs)

	// InvalidateAll clears both modes.
	cache.InvalidateAll()
	slips3, err := cache.GetOrComputeMode(ctx, store, eval.ModeSlips, group.ID, 2025)
	require.NoError(t, err)
	assert.Equal(t, 3, slips3.Coverage.TotalLegs)

	// Invalid mode returns error.
	_, err = cache.GetOrComputeMode(ctx, store, eval.ReplayMode("bogus"), 0, 2025)
	require.Error(t, err)
}
