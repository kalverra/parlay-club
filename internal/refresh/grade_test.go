package refresh

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

type fakeBoxScoreClient struct {
	calls   atomic.Int32
	perGame map[string]int
	fn      func(eventID string) (espn.BoxScore, error)
}

func (f *fakeBoxScoreClient) BoxScore(_ context.Context, eventID string) (espn.BoxScore, error) {
	f.calls.Add(1)
	if f.perGame == nil {
		f.perGame = make(map[string]int)
	}
	f.perGame[eventID]++
	return f.fn(eventID)
}

type fakePublisher struct {
	topics []string
}

func (p *fakePublisher) Publish(topic string) {
	p.topics = append(p.topics, topic)
}

func seedGradeFixture(t *testing.T, store *db.Store, _ time.Time) (db.User, db.Week, db.Slip) {
	t.Helper()
	ctx := t.Context()

	user, err := store.CreateUserWithIdentity(ctx, "Ada", "ada@example.com", "google", "google-ada")
	require.NoError(t, err)
	group, err := store.Create(ctx, user.ID, "Sunday Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(ctx, 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(ctx, group.ID, week.ID, nil, "Week 9", db.SlateScopeFullWeek, 4, []int64{user.ID})
	require.NoError(t, err)
	return user, week, slip
}

func TestGradeSourceGameOnlyOptimization(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 21, 0, 0, 0, time.UTC)
	store := openTestStore(t)
	ada, week, slip := seedGradeFixture(t, store, now)

	game := storedGame(week.ID, now.Add(-4*time.Hour))
	game.Status = db.GameFinal
	game.HomeScore = new(27)
	game.AwayScore = new(20)
	game, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)

	leg1, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "ML", Selection: "BUF ML", Team: new("BUF"),
	})
	require.NoError(t, err)
	leg2, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "SPREAD", Selection: "BUF -3.5", Team: new("BUF"), Line: new(-3.5),
	})
	require.NoError(t, err)
	leg3, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "TOTAL", Selection: "Over 45.5", Side: new("OVER"), Line: new(45.5),
	})
	require.NoError(t, err)

	box := &fakeBoxScoreClient{
		fn: func(string) (espn.BoxScore, error) {
			return espn.BoxScore{}, errors.New("unexpected boxscore call")
		},
	}
	pub := &fakePublisher{}
	svc := New(Deps{
		Store:     store,
		BoxScore:  box,
		Publisher: pub,
		Now:       func() time.Time { return now },
		Logger:    zerolog.Nop(),
	})

	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(0), box.calls.Load(), "game-only markets must not fetch box score")

	for _, id := range []int64{leg1.ID, leg2.ID, leg3.ID} {
		got, err := store.LegByID(t.Context(), id)
		require.NoError(t, err)
		assert.Equal(t, parlay.Won, got.Result)
		require.NotNil(t, got.GradedBy)
		assert.Equal(t, "auto", *got.GradedBy)
	}

	gotSlip, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotSlip.Result)
	assert.Equal(t, db.SlipGraded, gotSlip.Status)
	assert.Contains(t, pub.topics, fmt.Sprintf("slip:%d", slip.ID))

	// Re-run does zero work.
	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(0), box.calls.Load())
}

func TestGradeSourcePlayerPropsAndZeroWasteReruns(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 21, 0, 0, 0, time.UTC)
	store := openTestStore(t)
	ada, week, slip := seedGradeFixture(t, store, now)

	game := storedGame(week.ID, now.Add(-4*time.Hour))
	game.Status = db.GameFinal
	game.HomeScore = new(27)
	game.AwayScore = new(20)
	game, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)

	passLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "PASS_YDS", Selection: "Josh Allen Over 245.5",
		Team: new("BUF"), PlayerID: new("3918298"), PlayerName: new("Josh Allen"),
		Side: new("OVER"), Line: new(245.5),
	})
	require.NoError(t, err)
	tdLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "ANYTIME_TD", Selection: "James Cook Anytime TD",
		Team: new("BUF"), PlayerID: new("4379399"), PlayerName: new("James Cook"),
		Side: new("YES"),
	})
	require.NoError(t, err)

	box := &fakeBoxScoreClient{
		fn: func(string) (espn.BoxScore, error) {
			return espn.BoxScore{
				Completed: true,
				HasStats:  true,
				Players: []espn.PlayerBoxStats{
					{
						PlayerID:    "3918298",
						DisplayName: "Josh Allen",
						TeamAbbrev:  "BUF",
						Passing:     espn.PassingStats{PassingYards: 280, PassingTDs: 2},
					},
					{
						PlayerID:    "4379399",
						DisplayName: "James Cook",
						TeamAbbrev:  "BUF",
						Rushing:     espn.RushingStats{RushingAttempts: 18, RushingYards: 95, RushingTDs: 0},
					},
				},
			}, nil
		},
	}
	svc := New(Deps{
		Store:    store,
		BoxScore: box,
		Now:      func() time.Time { return now },
		Logger:   zerolog.Nop(),
	})

	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(1), box.calls.Load())

	gotPass, err := store.LegByID(t.Context(), passLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotPass.Result)

	gotTD, err := store.LegByID(t.Context(), tdLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Lost, gotTD.Result)

	gotSlip, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Lost, gotSlip.Result)
	assert.Equal(t, db.SlipGraded, gotSlip.Status)

	// Subsequent RunOnce sees zero pending legs on final games and skips BoxScore.
	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(1), box.calls.Load())
}

func TestGradeSourceManualOverrideSurvivesReruns(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 21, 0, 0, 0, time.UTC)
	store := openTestStore(t)
	ada, week, slip := seedGradeFixture(t, store, now)

	game := storedGame(week.ID, now.Add(-4*time.Hour))
	game.Status = db.GameFinal
	game.HomeScore = new(27)
	game.AwayScore = new(20)
	game, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)

	overrideLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "ML", Selection: "MIA ML", Team: new("MIA"),
	})
	require.NoError(t, err)
	autoLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "ML", Selection: "BUF ML", Team: new("BUF"),
	})
	require.NoError(t, err)

	slipsSvc := slips.New(store, nil, func() time.Time { return now })
	// Ada manually overrides overrideLeg to VOID before auto-grade runs.
	_, err = slipsSvc.SetLegResult(t.Context(), ada.ID, overrideLeg.ID, parlay.Void)
	require.NoError(t, err)

	svc := New(Deps{
		Store:  store,
		Slips:  slipsSvc,
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})

	require.NoError(t, svc.RunOnce(t.Context()))

	// overrideLeg kept Ada's VOID; autoLeg became auto WON.
	gotOverride, err := store.LegByID(t.Context(), overrideLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Void, gotOverride.Result)
	require.NotNil(t, gotOverride.GradedBy)
	assert.Equal(t, "Ada", *gotOverride.GradedBy)

	gotAuto, err := store.LegByID(t.Context(), autoLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotAuto.Result)
	require.NotNil(t, gotAuto.GradedBy)
	assert.Equal(t, "auto", *gotAuto.GradedBy)

	// Ada now overrides autoLeg to LOST, and a third PENDING leg triggers another RunOnce.
	_, err = slipsSvc.SetLegResult(t.Context(), ada.ID, autoLeg.ID, parlay.Lost)
	require.NoError(t, err)

	thirdLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game.ID,
		MarketType: "TOTAL", Selection: "Over 45.5", Side: new("OVER"), Line: new(45.5),
	})
	require.NoError(t, err)

	require.NoError(t, svc.RunOnce(t.Context()))

	// Both manual overrides must still be intact; thirdLeg is auto WON.
	gotOverride, err = store.LegByID(t.Context(), overrideLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Void, gotOverride.Result)
	assert.Equal(t, "Ada", *gotOverride.GradedBy)

	gotAuto, err = store.LegByID(t.Context(), autoLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Lost, gotAuto.Result)
	assert.Equal(t, "Ada", *gotAuto.GradedBy)

	gotThird, err := store.LegByID(t.Context(), thirdLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotThird.Result)
	assert.Equal(t, "auto", *gotThird.GradedBy)
}

func TestGradeSourceBoxScoreErrorBackoff(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 8, 21, 0, 0, 0, time.UTC)
	store := openTestStore(t)
	ada, week, slip := seedGradeFixture(t, store, now)

	game1 := storedGame(week.ID, now.Add(-4*time.Hour))
	game1.ID = "401772001"
	game1.Status = db.GameFinal
	game1.HomeScore = new(27)
	game1.AwayScore = new(20)
	game1, err := store.UpsertGame(t.Context(), game1)
	require.NoError(t, err)

	game2 := storedGame(week.ID, now.Add(-4*time.Hour))
	game2.ID = "401772002"
	game2.HomeTeam = "KC"
	game2.AwayTeam = "LAC"
	game2.Status = db.GameFinal
	game2.HomeScore = new(24)
	game2.AwayScore = new(17)
	game2, err = store.UpsertGame(t.Context(), game2)
	require.NoError(t, err)

	propLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game1.ID,
		MarketType: "PASS_YDS", Selection: "Josh Allen Over 245.5",
		Team: new("BUF"), PlayerID: new("3918298"), PlayerName: new("Josh Allen"),
		Side: new("OVER"), Line: new(245.5),
	})
	require.NoError(t, err)
	mlLeg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, GameID: &game2.ID,
		MarketType: "ML", Selection: "KC ML", Team: new("KC"),
	})
	require.NoError(t, err)

	failBox := true
	box := &fakeBoxScoreClient{
		fn: func(string) (espn.BoxScore, error) {
			if failBox {
				return espn.BoxScore{}, errors.New("espn 500")
			}
			return espn.BoxScore{
				Completed: true,
				HasStats:  true,
				Players: []espn.PlayerBoxStats{
					{
						PlayerID:    "3918298",
						DisplayName: "Josh Allen",
						TeamAbbrev:  "BUF",
						Passing:     espn.PassingStats{PassingYards: 300},
					},
				},
			}, nil
		},
	}

	clock := now
	svc := New(Deps{
		Store:    store,
		BoxScore: box,
		Now:      func() time.Time { return clock },
		Logger:   zerolog.Nop(),
	})

	// First RunOnce: game1 BoxScore fails (1 call, backed off 5m), game2 ML still grades!
	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(1), box.calls.Load())

	gotML, err := store.LegByID(t.Context(), mlLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotML.Result)

	gotProp, err := store.LegByID(t.Context(), propLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Pending, gotProp.Result)

	// Within 5m backoff: BoxScore is not called again.
	clock = now.Add(2 * time.Minute)
	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(1), box.calls.Load())

	// After 5m backoff expires: BoxScore retries and succeeds.
	failBox = false
	clock = now.Add(6 * time.Minute)
	require.NoError(t, svc.RunOnce(t.Context()))
	assert.Equal(t, int32(2), box.calls.Load())

	gotProp, err = store.LegByID(t.Context(), propLeg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, gotProp.Result)
}
