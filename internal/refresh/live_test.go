package refresh

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
	"github.com/kalverra/parlay-club/internal/ingest/weather"
	"github.com/kalverra/parlay-club/internal/parlay"
	"github.com/kalverra/parlay-club/internal/slips"
)

func TestLiveWindowRule(t *testing.T) {
	t.Parallel()

	now := novemberTime(8, 12, 30)

	tests := []struct {
		name  string
		games []db.Game
		want  bool
	}{
		{name: "no games", want: false},
		{
			name:  "live game",
			games: []db.Game{{ID: "live", Status: db.GameLive, Kickoff: now.Add(-time.Hour)}},
			want:  true,
		},
		{
			name:  "kickoff in thirty minutes",
			games: []db.Game{{ID: "soon", Status: db.GameScheduled, Kickoff: now.Add(30 * time.Minute)}},
			want:  true,
		},
		{
			name:  "kickoff in thirty-one minutes",
			games: []db.Game{{ID: "later", Status: db.GameScheduled, Kickoff: now.Add(31 * time.Minute)}},
			want:  false,
		},
		{
			name:  "final game near kickoff",
			games: []db.Game{{ID: "final", Status: db.GameFinal, Kickoff: now.Add(10 * time.Minute)}},
			want:  false,
		},
		{
			name:  "scheduled game at kickoff",
			games: []db.Game{{ID: "started", Status: db.GameScheduled, Kickoff: now}},
			want:  false,
		},
		{
			name: "one game in the window is enough",
			games: []db.Game{
				{ID: "later", Status: db.GameScheduled, Kickoff: now.Add(4 * time.Hour)},
				{ID: "live", Status: db.GameLive, Kickoff: now.Add(-time.Hour)},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, liveWindow(tt.games, now))
		})
	}
}

func TestServiceLiveWindowReadsTheWeek(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	now := novemberTime(8, 12, 0)

	liveWeek, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), liveTestGame(
		liveWeek.ID, "soon", now.Add(15*time.Minute), db.GameScheduled))
	require.NoError(t, err)

	idleWeek, err := store.UpsertWeek(t.Context(), 2026, 2, 10)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), liveTestGame(
		idleWeek.ID, "later", now.Add(3*time.Hour), db.GameScheduled))
	require.NoError(t, err)

	service := New(Deps{
		Store:  store,
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})

	assert.True(t, service.LiveWindow(liveWeek.ID))
	assert.False(t, service.LiveWindow(idleWeek.ID))
	assert.False(t, service.LiveWindow(999999), "a week with no games is not live")
}

// liveTestGame is a stored game with the given kickoff and status.
//
//nolint:unparam // helper accepts any status for future test cases.
func liveTestGame(weekID int64, id string, kickoff time.Time, status db.GameStatus) db.Game {
	game := storedGame(weekID, kickoff)
	game.ID = id
	game.Kickoff = kickoff
	game.Status = status
	return game
}

type liveTestScoreboard struct {
	mu    sync.Mutex
	calls int
	board espn.Scoreboard
	err   error
	gate  chan struct{}
}

func (f *liveTestScoreboard) Scoreboard(ctx context.Context, _ espn.ScoreboardParams) (espn.Scoreboard, error) {
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return espn.Scoreboard{}, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.board, f.err
}

func (f *liveTestScoreboard) setBoard(board espn.Scoreboard) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.board = board
}

func (f *liveTestScoreboard) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type liveTestSummaryClient struct {
	fakeESPNResearch
	mu        sync.Mutex
	summaries map[string]espn.GameSummary
	calls     map[string]int
	errs      map[string]error
	gate      chan struct{}
}

func (f *liveTestSummaryClient) Summary(ctx context.Context, eventID string) (espn.GameSummary, error) {
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return espn.GameSummary{}, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[eventID]++
	if err := f.errs[eventID]; err != nil {
		return espn.GameSummary{}, err
	}
	return f.summaries[eventID], nil
}

func (f *liveTestSummaryClient) callsFor(eventID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[eventID]
}

func (f *liveTestSummaryClient) setErr(eventID string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errs == nil {
		f.errs = make(map[string]error)
	}
	f.errs[eventID] = err
}

type countingForecast struct {
	calls atomic.Int32
}

func (f *countingForecast) Forecast(context.Context, weather.Venue, time.Time) (weather.Forecast, error) {
	f.calls.Add(1)
	return weather.Forecast{}, nil
}

type syncPublisher struct {
	mu     sync.Mutex
	topics []string
}

func (p *syncPublisher) Publish(topic string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topics = append(p.topics, topic)
}

func (p *syncPublisher) snapshot() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.topics))
	copy(out, p.topics)
	return out
}

func (p *syncPublisher) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topics = nil
}

func liveScoreboard(_ time.Time, games ...espn.Game) espn.Scoreboard {
	return espn.Scoreboard{
		Season: espn.Season{Year: 2026, Type: 2},
		Week:   espn.Week{Number: 9},
		Games:  games,
	}
}

func espnLiveGame(id, home, away string, kickoff time.Time, status espn.Status, homeScore, awayScore int) espn.Game {
	hs, as := homeScore, awayScore
	return espn.Game{
		ID:      id,
		Kickoff: kickoff,
		Status:  status,
		Home: espn.Team{
			ID:           id + "H",
			Abbreviation: home,
			Name:         home + " Team",
			Score:        &hs,
		},
		Away: espn.Team{
			ID:           id + "A",
			Abbreviation: away,
			Name:         away + " Team",
			Score:        &as,
		},
		Period: new(2),
		Clock:  new("05:12"),
	}
}

func TestSyncLiveScopingAndZeroPaidCalls(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	now := novemberTime(8, 14, 0)
	user, week, slip := seedGradeFixture(t, store)

	// G1: LIVE on slip; G2: SCHEDULED on slip; G3: LIVE but NOT on slip.
	board := liveScoreboard(now,
		espnLiveGame("G1", "BUF", "MIA", now.Add(-time.Hour), espn.Live, 14, 10),
		espnLiveGame("G2", "KC", "LV", now.Add(2*time.Hour), espn.Scheduled, 0, 0),
		espnLiveGame("G3", "DAL", "PHI", now.Add(-time.Hour), espn.Live, 7, 7),
	)
	for _, g := range board.Games {
		_, err := store.UpsertGame(t.Context(), slateGame(g, week.ID, now))
		require.NoError(t, err)
	}

	g1 := "G1"
	g2 := "G2"
	buf := "BUF"
	kc := "KC"
	_, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &g1,
		MarketType: odds.MarketML,
		Selection:  "BUF ML",
		Team:       &buf,
		Odds:       -130,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &g2,
		MarketType: odds.MarketML,
		Selection:  "KC ML",
		Team:       &kc,
		Odds:       -150,
	})
	require.NoError(t, err)

	sb := &liveTestScoreboard{board: board}
	espnClient := &liveTestSummaryClient{
		summaries: map[string]espn.GameSummary{
			"G1": {InjuriesJSON: `{"live":1}`},
			"G2": {InjuriesJSON: `{"sched":1}`},
			"G3": {InjuriesJSON: `{"other":1}`},
		},
	}
	lines := &fakeLines{}
	props := &fakePropsClient{}
	fallbackProps := &fakeFallbackPropsClient{}
	forecast := &countingForecast{}
	projections := &fakeProjections{}
	nflv := &fakeNFLverseClient{}

	svc := New(Deps{
		Store:         store,
		Scoreboard:    sb,
		ESPN:          espnClient,
		Forecast:      forecast,
		Lines:         lines,
		FallbackLines: lines,
		Props:         props,
		FallbackProps: fallbackProps,
		Projections:   projections,
		NFLverse:      nflv,
		Now:           func() time.Time { return now },
		Logger:        zerolog.Nop(),
	})

	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))

	assert.Equal(t, 1, sb.callCount())
	assert.Equal(t, 1, espnClient.callsFor("G1"), "LIVE game on slip should fetch summary")
	assert.Equal(t, 0, espnClient.callsFor("G2"), "SCHEDULED game on slip should not fetch summary")
	assert.Equal(t, 0, espnClient.callsFor("G3"), "LIVE game not on slip should not fetch summary")
	assert.Zero(t, espnClient.rosterCalls, "SyncLive must not fetch rosters")
	assert.Zero(t, espnClient.gamelogCalls, "SyncLive must not fetch game logs")
	assert.Zero(t, forecast.calls.Load(), "SyncLive must not fetch weather")
	assert.Zero(t, lines.eventsCalls, "SyncLive must not call paid odds events")
	assert.Zero(t, lines.lineCalls, "SyncLive must not call paid odds lines")
	assert.Zero(t, props.eventsCalls, "SyncLive must not call paid props events")
	assert.Zero(t, props.propsCalls, "SyncLive must not call paid props")
	assert.Zero(t, fallbackProps.eventsCalls, "SyncLive must not call fallback props events")
	assert.Zero(t, fallbackProps.eventPropsCalls, "SyncLive must not call fallback paid props")
	assert.Zero(t, fallbackProps.scopedCalls, "SyncLive must not call fallback scoped props")
	assert.Zero(t, projections.calls.Load(), "SyncLive must not call nfelo")
	assert.Zero(t, nflv.calls, "SyncLive must not call nflverse")
}

func TestSyncLiveHonorsFastAgeAndBackoff(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(8, 14, 0)}
	user, week, slip := seedGradeFixture(t, store)

	board := liveScoreboard(clock.Now(),
		espnLiveGame("G1", "BUF", "MIA", clock.Now().Add(-time.Hour), espn.Live, 14, 10),
	)
	_, err := store.UpsertGame(t.Context(), slateGame(board.Games[0], week.ID, clock.Now()))
	require.NoError(t, err)
	g1 := "G1"
	buf := "BUF"
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &g1,
		MarketType: odds.MarketML,
		Selection:  "BUF ML",
		Team:       &buf,
		Odds:       -130,
	})
	require.NoError(t, err)

	sb := &liveTestScoreboard{board: board}
	espnClient := &liveTestSummaryClient{
		summaries: map[string]espn.GameSummary{
			"G1": {InjuriesJSON: `{"v":1}`},
		},
	}

	svc := New(Deps{
		Store:      store,
		Scoreboard: sb,
		ESPN:       espnClient,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	// Initial run fetches scoreboard + G1 summary.
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Equal(t, 1, sb.callCount())
	assert.Equal(t, 1, espnClient.callsFor("G1"))

	// Within 20s liveFastAge: no upstream fetches.
	clock.Advance(19 * time.Second)
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Equal(t, 1, sb.callCount())
	assert.Equal(t, 1, espnClient.callsFor("G1"))

	// At 20s: both sources are stale and fetch again.
	clock.Advance(time.Second)
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Equal(t, 2, sb.callCount())
	assert.Equal(t, 2, espnClient.callsFor("G1"))

	// Failure triggers 5-minute failureBackoff.
	clock.Advance(liveFastAge)
	espnClient.setErr("G1", errors.New("espn 503"))
	err = svc.SyncLive(t.Context(), slip.ID, week.ID)
	require.Error(t, err)
	assert.Equal(t, 3, sb.callCount())
	assert.Equal(t, 3, espnClient.callsFor("G1"))

	// 1 minute later: scoreboard refreshes (wasn't failing), G1 summary is in backoff.
	espnClient.setErr("G1", nil)
	clock.Advance(time.Minute)
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Equal(t, 4, sb.callCount())
	assert.Equal(t, 3, espnClient.callsFor("G1"), "G1 summary should stay paused during failureBackoff")

	// After 5m failureBackoff expires, G1 summary fetches again.
	clock.Advance(failureBackoff)
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Equal(t, 4, espnClient.callsFor("G1"))
}

func TestSyncLiveDeduplicatesConcurrentCallers(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	now := novemberTime(8, 14, 0)
	user, week, slip := seedGradeFixture(t, store)

	board := liveScoreboard(now,
		espnLiveGame("G1", "BUF", "MIA", now.Add(-time.Hour), espn.Live, 14, 10),
	)
	_, err := store.UpsertGame(t.Context(), slateGame(board.Games[0], week.ID, now))
	require.NoError(t, err)

	g1 := "G1"
	buf := "BUF"
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &g1,
		MarketType: odds.MarketML,
		Selection:  "BUF ML",
		Team:       &buf,
		Odds:       -130,
	})
	require.NoError(t, err)

	sbGate := make(chan struct{})
	sumGate := make(chan struct{})
	sb := &liveTestScoreboard{board: board, gate: sbGate}
	espnClient := &liveTestSummaryClient{
		summaries: map[string]espn.GameSummary{"G1": {InjuriesJSON: `{}`}},
		gate:      sumGate,
	}

	svc := New(Deps{
		Store:      store,
		Scoreboard: sb,
		ESPN:       espnClient,
		Now:        func() time.Time { return now },
		Logger:     zerolog.Nop(),
	})

	const callers = 6
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			errs <- svc.SyncLive(t.Context(), slip.ID, week.ID)
		})
	}
	close(sbGate)
	close(sumGate)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, 1, sb.callCount(), "concurrent callers must share one scoreboard fetch")
	assert.Equal(t, 1, espnClient.callsFor("G1"), "concurrent callers must share one summary fetch")
}

func TestSyncLiveAutoGradesFinalGamesAndPublishesOnChange(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(8, 14, 0)}
	user, week, slip := seedGradeFixture(t, store)

	_, err := store.UpsertGame(t.Context(), liveTestGame(week.ID, "G1", clock.Now().Add(-time.Hour), db.GameScheduled))
	require.NoError(t, err)

	g1 := "G1"
	buf := "BUF"
	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &g1,
		MarketType: odds.MarketML,
		Selection:  "BUF ML",
		Team:       &buf,
		Odds:       -130,
	})
	require.NoError(t, err)

	pub := &syncPublisher{}
	sb := &liveTestScoreboard{
		board: liveScoreboard(clock.Now(),
			espnLiveGame("G1", "BUF", "MIA", clock.Now().Add(-time.Hour), espn.Live, 14, 10),
		),
	}
	espnClient := &liveTestSummaryClient{
		summaries: map[string]espn.GameSummary{
			"G1": {InjuriesJSON: `{"v":1}`},
		},
	}

	svc := New(Deps{
		Store:      store,
		Scoreboard: sb,
		ESPN:       espnClient,
		Slips:      slips.New(store, pub, clock.Now),
		Publisher:  pub,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	// First sync writes the game and summary -> publishes slip:{id}.
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Contains(t, pub.snapshot(), "slip:1")

	// Repeat sync after 20s with identical data -> zero publishes.
	pub.reset()
	clock.Advance(liveFastAge)
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))
	assert.Empty(t, pub.snapshot(), "unchanged data must not publish")

	// Game becomes FINAL on scoreboard -> SyncLive runs syncGrade immediately.
	clock.Advance(liveFastAge)
	sb.setBoard(liveScoreboard(clock.Now(),
		espnLiveGame("G1", "BUF", "MIA", clock.Now().Add(-3*time.Hour), espn.Final, 28, 17),
	))
	require.NoError(t, svc.SyncLive(t.Context(), slip.ID, week.ID))

	legs, err := store.LegsForSlip(t.Context(), slip.ID)
	require.NoError(t, err)
	require.Len(t, legs, 1)
	assert.Equal(t, leg.ID, legs[0].ID)
	assert.Equal(t, parlay.Won, legs[0].Result)
}

func TestForceLiveRefreshEnforcesPerSlipCooldown(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(8, 14, 0)}
	user, week, slip1 := seedGradeFixture(t, store)

	slip2, err := store.CreateSlip(
		t.Context(), slip1.GroupID, week.ID, nil, "Week 9 #2",
		db.SlateScopeFullWeek, 4, []int64{user.ID},
	)
	require.NoError(t, err)

	board := liveScoreboard(clock.Now(),
		espnLiveGame("G1", "BUF", "MIA", clock.Now().Add(-time.Hour), espn.Live, 14, 10),
	)
	_, err = store.UpsertGame(t.Context(), liveTestGame(week.ID, "G1", clock.Now().Add(-time.Hour), db.GameScheduled))
	require.NoError(t, err)

	g1 := "G1"
	buf := "BUF"
	for _, sID := range []int64{slip1.ID, slip2.ID} {
		_, err := store.CreateLeg(t.Context(), db.Leg{
			SlipID:     sID,
			UserID:     user.ID,
			GameID:     &g1,
			MarketType: odds.MarketML,
			Selection:  "BUF ML",
			Team:       &buf,
			Odds:       -130,
		})
		require.NoError(t, err)
	}

	sb := &liveTestScoreboard{board: board}
	espnClient := &liveTestSummaryClient{
		summaries: map[string]espn.GameSummary{"G1": {InjuriesJSON: `{}`}},
	}

	svc := New(Deps{
		Store:      store,
		Scoreboard: sb,
		ESPN:       espnClient,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	// Initial SyncLive populates sync_state.
	require.NoError(t, svc.SyncLive(t.Context(), slip1.ID, week.ID))
	assert.Equal(t, 1, sb.callCount())
	assert.Equal(t, 1, espnClient.callsFor("G1"))

	// 1s later, ForceLiveRefresh bypasses liveFastAge.
	clock.Advance(time.Second)
	require.NoError(t, svc.ForceLiveRefresh(t.Context(), slip1.ID, week.ID))
	assert.Equal(t, 2, sb.callCount())
	assert.Equal(t, 2, espnClient.callsFor("G1"))

	// 9s later (still inside 10s cooldown for slip1), ForceLiveRefresh is a no-op.
	clock.Advance(9 * time.Second)
	require.NoError(t, svc.ForceLiveRefresh(t.Context(), slip1.ID, week.ID))
	assert.Equal(t, 2, sb.callCount())
	assert.Equal(t, 2, espnClient.callsFor("G1"))

	// Different slip is not blocked by slip1's cooldown.
	require.NoError(t, svc.ForceLiveRefresh(t.Context(), slip2.ID, week.ID))
	assert.Equal(t, 3, sb.callCount())
	assert.Equal(t, 3, espnClient.callsFor("G1"))

	// After 10s cooldown expires for slip1, ForceLiveRefresh runs again.
	clock.Advance(time.Second)
	require.NoError(t, svc.ForceLiveRefresh(t.Context(), slip1.ID, week.ID))
	assert.Equal(t, 4, sb.callCount())
	assert.Equal(t, 4, espnClient.callsFor("G1"))
}

func TestKickLiveRunsInBackgroundAndHonorsShutdown(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	now := novemberTime(8, 14, 0)
	user, week, slip := seedGradeFixture(t, store)

	board := liveScoreboard(now,
		espnLiveGame("G1", "BUF", "MIA", now.Add(-time.Hour), espn.Live, 14, 10),
	)
	_, err := store.UpsertGame(t.Context(), liveTestGame(week.ID, "G1", now.Add(-time.Hour), db.GameScheduled))
	require.NoError(t, err)

	g1 := "G1"
	buf := "BUF"
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &g1,
		MarketType: odds.MarketML,
		Selection:  "BUF ML",
		Team:       &buf,
		Odds:       -130,
	})
	require.NoError(t, err)

	sb := &liveTestScoreboard{board: board}
	espnClient := &liveTestSummaryClient{
		summaries: map[string]espn.GameSummary{"G1": {InjuriesJSON: `{}`}},
	}

	svc := New(Deps{
		Store:      store,
		Scoreboard: sb,
		ESPN:       espnClient,
		Now:        func() time.Time { return now },
		Logger:     zerolog.Nop(),
	})

	svc.KickLive(slip.ID, week.ID)
	svc.wait()

	assert.Equal(t, 1, sb.callCount())
	assert.Equal(t, 1, espnClient.callsFor("G1"))

	// KickLive after Shutdown is a no-op.
	require.NoError(t, svc.Shutdown(t.Context()))
	svc.KickLive(slip.ID, week.ID)
	svc.wait()
	assert.Equal(t, 1, sb.callCount())
}
