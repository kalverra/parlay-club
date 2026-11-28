package web

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/progress"
)

func TestFormatImpliedChance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prob float64
		want string
	}{
		{name: "zero is empty", prob: 0, want: ""},
		{name: "negative is empty", prob: -0.25, want: ""},
		{name: "one is empty", prob: 1, want: ""},
		{name: "above one is empty", prob: 1.2, want: ""},
		{name: "NaN is empty", prob: math.NaN(), want: ""},
		{name: "positive Inf is empty", prob: math.Inf(1), want: ""},
		{name: "negative Inf is empty", prob: math.Inf(-1), want: ""},
		{name: "below 0.1 percent", prob: parlay.ImpliedProb(200000), want: "<0.1%"},
		{name: "just below 0.1 percent", prob: 0.00099, want: "<0.1%"},
		{name: "at 0.1 percent", prob: 0.001, want: "0.1%"},
		{name: "plus 5000", prob: parlay.ImpliedProb(5000), want: "2.0%"},
		{name: "plus 1200", prob: parlay.ImpliedProb(1200), want: "7.7%"},
		{name: "9.94 percent stays one decimal", prob: 0.0994, want: "9.9%"},
		{name: "9.96 percent rounds to whole 10", prob: 0.0996, want: "10%"},
		{name: "10 percent", prob: 0.10, want: "10%"},
		{name: "plus 150", prob: parlay.ImpliedProb(150), want: "40%"},
		{name: "minus 110", prob: parlay.ImpliedProb(-110), want: "52%"},
		{name: "99.4 percent stays 99", prob: 0.994, want: "99%"},
		{name: "99.5 percent caps at >99%", prob: 0.995, want: ">99%"},
		{name: "minus 50000", prob: parlay.ImpliedProb(-50000), want: ">99%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, formatImpliedChance(tt.prob))
		})
	}
}

func TestFormatProbDelta(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "+14%", formatProbDelta(0.142, "up"))
	assert.Equal(t, "-8%", formatProbDelta(-0.084, "down"))
	assert.Empty(t, formatProbDelta(0.002, "flat"))
	assert.Empty(t, formatProbDelta(0, ""))
	assert.Empty(t, formatProbDelta(math.NaN(), "up"))
	assert.Empty(t, formatProbDelta(math.Inf(1), "up"))
}

func TestLegLiveViewModes(t *testing.T) {
	t.Parallel()

	sched := progress.LegProgress{
		State:           progress.StateScheduled,
		GameProgressPct: 0,
		Tone:            "neutral",
		PreGameProb:     0.52,
		LiveProb:        0.52,
		ProbDelta:       0,
		ProbTrend:       "flat",
		Counts:          true,
		HasLive:         false,
	}
	assert.Nil(t, legLiveView(sched, false))

	lv := legLiveView(sched, true)
	require.NotNil(t, lv)
	assert.Equal(t, "SCHEDULED", lv.State)
	assert.Equal(t, "Upcoming", lv.StatLabel)
	assert.Equal(t, "52%", lv.PreGameChance)
	assert.Equal(t, "52%", lv.LiveChance)
	assert.Empty(t, lv.ProbDelta)
	assert.Equal(t, "flat", lv.ProbTrend)
	assert.Equal(t, 0, lv.GameProgressPct)

	live := progress.LegProgress{
		State:           progress.StateLive,
		ProgressPct:     78,
		GameProgressPct: 62.5,
		Tone:            "on_track",
		StatLabel:       "210 / 245.5 yds",
		ClockLabel:      "Q3 8:42",
		PreGameProb:     0.52,
		LiveProb:        0.66,
		ProbDelta:       0.14,
		ProbTrend:       "up",
		Counts:          true,
		HasLive:         true,
	}
	lvLive := legLiveView(live, true)
	require.NotNil(t, lvLive)
	assert.Equal(t, "52%", lvLive.PreGameChance)
	assert.Equal(t, "66%", lvLive.LiveChance)
	assert.Equal(t, "+14%", lvLive.ProbDelta)
	assert.Equal(t, "up", lvLive.ProbTrend)
	assert.Equal(t, 63, lvLive.GameProgressPct)
}

func TestBuildLiveGamesOrdering(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	games := []db.Game{
		{
			ID: "g-final", AwayTeam: "NYJ", HomeTeam: "NE",
			Kickoff: base.Add(-4 * time.Hour), Status: db.GameFinal,
			AwayScore: new(14), HomeScore: new(24),
		},
		{
			ID: "g-sched-late", AwayTeam: "SF", HomeTeam: "SEA",
			Kickoff: base.Add(3 * time.Hour), Status: db.GameScheduled,
		},
		{
			ID: "g-live-late", AwayTeam: "KC", HomeTeam: "BUF",
			Kickoff: base.Add(-30 * time.Minute), Status: db.GameLive,
			AwayScore: new(17), HomeScore: new(21), Period: new(3), Clock: new("8:42"),
		},
		{
			ID: "g-live-early", AwayTeam: "BAL", HomeTeam: "CIN",
			Kickoff: base.Add(-90 * time.Minute), Status: db.GameLive,
			AwayScore: new(28), HomeScore: new(24), Period: new(4), Clock: new("2:10"),
		},
		{
			ID: "g-sched-early", AwayTeam: "DAL", HomeTeam: "PHI",
			Kickoff: base.Add(1 * time.Hour), Status: db.GameScheduled,
		},
	}

	legs := []db.Leg{
		{ID: 1, GameID: new("g-final")},
		{ID: 2, GameID: new("g-sched-late")},
		{ID: 3, GameID: new("g-live-late")},
		{ID: 4, GameID: new("g-live-late")},
		{ID: 5, GameID: new("g-live-early")},
		{ID: 6, GameID: new("g-sched-early")},
		{ID: 7, GameID: nil},
	}

	legViewByID := map[int64]LegView{
		1: {ID: 1, Live: &LegLiveView{State: string(progress.StateCashed)}},
		2: {ID: 2, Live: &LegLiveView{State: string(progress.StateScheduled)}},
		3: {ID: 3, Live: &LegLiveView{State: string(progress.StateBusted)}},
		4: {ID: 4, Live: &LegLiveView{State: string(progress.StateLive)}},
		5: {ID: 5, Live: &LegLiveView{State: string(progress.StateLive)}},
		6: {ID: 6, Live: &LegLiveView{State: string(progress.StateScheduled)}},
		7: {ID: 7, Live: &LegLiveView{State: string(progress.StateScheduled)}},
	}

	media := legMediaIndex{
		teamLogos: map[string]string{
			"KC":  "/logo/kc.png",
			"BUF": "/logo/buf.png",
		},
	}

	groups := buildLiveGames(legs, games, media, legViewByID)
	require.Len(t, groups, 6)

	assert.Equal(t, "g-live-early", groups[0].GameID)
	assert.Equal(t, "LIVE", groups[0].Status)
	assert.Equal(t, "28 - 24", groups[0].ScoreLabel)

	assert.Equal(t, "g-live-late", groups[1].GameID)
	assert.Equal(t, "/logo/kc.png", groups[1].AwayLogo)
	assert.Equal(t, "/logo/buf.png", groups[1].HomeLogo)
	assert.Equal(t, "Q3 8:42", groups[1].ClockLabel)
	require.Len(t, groups[1].Legs, 2)
	// LIVE leg sorts before BUSTED leg within the same game.
	assert.Equal(t, int64(4), groups[1].Legs[0].ID)
	assert.Equal(t, int64(3), groups[1].Legs[1].ID)

	assert.Equal(t, "g-sched-early", groups[2].GameID)
	assert.Empty(t, groups[2].ScoreLabel)
	assert.Equal(t, 0, groups[2].GameProgressPct)

	assert.Equal(t, "g-sched-late", groups[3].GameID)

	assert.Equal(t, "g-final", groups[4].GameID)
	assert.Equal(t, 100, groups[4].GameProgressPct)
	assert.Equal(t, "Final", groups[4].ClockLabel)

	assert.Empty(t, groups[5].GameID)
	assert.Equal(t, "UNLINKED", groups[5].Status)
	assert.Equal(t, "Other legs", groups[5].Matchup)
}

func TestSlipPageDataLiveModeAndUpdatedStamp(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))

	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	kickoff := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	now := kickoff.Add(45 * time.Minute)

	ada, err := store.CreateUserWithIdentity(t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	ben, err := store.CreateUserWithIdentity(t.Context(), "Ben", "ben@example.com", "google-ben", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), ada.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	liveGame, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "g-live",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "KC",
		Kickoff:   kickoff,
		Status:    db.GameLive,
		HomeScore: new(21),
		AwayScore: new(17),
		Period:    new(3),
		Clock:     new("8:42"),
		UpdatedAt: now,
	})
	require.NoError(t, err)

	schedGame, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "g-sched",
		WeekID:    week.ID,
		HomeTeam:  "DAL",
		AwayTeam:  "PHI",
		Kickoff:   now.Add(2 * time.Hour),
		Status:    db.GameScheduled,
		UpdatedAt: now,
	})
	require.NoError(t, err)

	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Live Slip",
		db.SlateScopeSundayMain,
		1,
		[]int64{ada.ID, ben.ID},
	)
	require.NoError(t, err)

	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ada.ID,
		GameID:     new(liveGame.ID),
		MarketType: "SPREAD",
		Selection:  "BUF -3.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-3.5),
		Odds:       -110,
	})
	require.NoError(t, err)

	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ben.ID,
		GameID:     new(schedGame.ID),
		MarketType: "ML",
		Selection:  "DAL ML",
		Team:       new("DAL"),
		Side:       new("TEAM"),
		Odds:       +120,
	})
	require.NoError(t, err)

	lockAt := kickoff.Add(-time.Hour)
	require.NoError(t, store.SetSlipMemberLock(t.Context(), slip.ID, ada.ID, &lockAt))
	require.NoError(t, store.SetSlipMemberLock(t.Context(), slip.ID, ben.ID, &lockAt))
	require.NoError(t, store.SetSlipStatus(t.Context(), slip.ID, db.SlipLocked))

	sbTime := now.Add(-2 * time.Minute)
	sumTime := now.Add(-15 * time.Second)
	require.NoError(t, store.RecordSync(t.Context(), espnScoreboardSource, sbTime, nil))
	require.NoError(t, store.RecordSync(t.Context(), db.ArchiveSummaryKey(liveGame.ID), sumTime, nil))

	loaded, err := store.SlipByID(t.Context(), slip.ID)
	require.NoError(t, err)

	handlers := &slipHandlers{
		deps: Dependencies{
			Store: store,
			Now:   func() time.Time { return now },
			UserFrom: func(context.Context) (db.User, bool) {
				return ada, true
			},
		},
	}
	page, err := handlers.slipPageData(t.Context(), loaded)
	require.NoError(t, err)

	assert.True(t, page.LiveMode)
	require.NotNil(t, page.Slip)
	assert.True(t, page.Slip.LiveMode)
	assert.Equal(t, sumTime.UTC().Format(time.RFC3339), page.Slip.LiveUpdatedISO)
	assert.Equal(t, sumTime.UTC().Format("3:04 PM")+" UTC", page.Slip.LiveUpdatedLabel)
	assert.True(t, page.Slip.Live.Show)
	assert.Equal(t, 1, page.Slip.Live.ScheduledCount)
	assert.NotEmpty(t, page.Slip.Live.PreGameChance)
	assert.NotEmpty(t, page.Slip.Live.LiveChance)

	require.Len(t, page.Slip.LiveGames, 2)
	assert.Equal(t, liveGame.ID, page.Slip.LiveGames[0].GameID)
	assert.Equal(t, "KC @ BUF", page.Slip.LiveGames[0].Matchup)
	assert.Equal(t, "17 - 21", page.Slip.LiveGames[0].ScoreLabel)
	require.Len(t, page.Slip.LiveGames[0].Legs, 1)
	assert.Equal(t, ada.Name, page.Slip.LiveGames[0].Legs[0].Owner.Name)
	assert.NotEmpty(t, page.Slip.LiveGames[0].Legs[0].Owner.Initials)
	require.NotNil(t, page.Slip.LiveGames[0].Legs[0].Live)
	assert.NotEmpty(t, page.Slip.LiveGames[0].Legs[0].Live.PreGameChance)
	assert.NotEmpty(t, page.Slip.LiveGames[0].Legs[0].Live.LiveChance)

	assert.Equal(t, schedGame.ID, page.Slip.LiveGames[1].GameID)
	assert.Equal(t, "SCHEDULED", page.Slip.LiveGames[1].Status)
	require.Len(t, page.Slip.LiveGames[1].Legs, 1)
	assert.Equal(t, ben.Name, page.Slip.LiveGames[1].Legs[0].Owner.Name)
	require.NotNil(t, page.Slip.LiveGames[1].Legs[0].Live)
	assert.Equal(t, "SCHEDULED", page.Slip.LiveGames[1].Legs[0].Live.State)
	assert.Equal(t, "45%", page.Slip.LiveGames[1].Legs[0].Live.PreGameChance)
	assert.Equal(t, "45%", page.Slip.LiveGames[1].Legs[0].Live.LiveChance)
}
