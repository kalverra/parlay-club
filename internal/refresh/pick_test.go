package refresh

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

type blockingLinesClient struct {
	events        []odds.Event
	eventsCredits odds.Credits
	lines         []odds.Line
	lineCredits   odds.Credits

	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingLinesClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return c.events, c.eventsCredits, nil
}

func (c *blockingLinesClient) Lines(ctx context.Context) ([]odds.Line, odds.Credits, error) {
	c.calls.Add(1)
	if c.started != nil {
		c.once.Do(func() { close(c.started) })
	}
	if c.release != nil {
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, odds.Credits{}, ctx.Err()
		}
	}
	return c.lines, c.lineCredits, nil
}

type blockingPropsClient struct {
	events        []odds.Event
	eventsCredits odds.Credits
	props         map[string][]odds.Prop
	propsCredits  odds.Credits

	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingPropsClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return c.events, c.eventsCredits, nil
}

func (c *blockingPropsClient) Props(ctx context.Context, eventID string) ([]odds.Prop, odds.Credits, error) {
	c.calls.Add(1)
	if c.started != nil {
		c.once.Do(func() { close(c.started) })
	}
	if c.release != nil {
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, odds.Credits{}, ctx.Err()
		}
	}
	return c.props[eventID], c.propsCredits, nil
}

func TestSyncPickLinesCadenceAndGuards(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	lines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "evt-401",
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
			CommenceTime: kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}

	// Wednesday: 2h window.
	clock := &fakeClock{now: novemberTime(4, 12, 0)}
	svc := New(Deps{
		Store:  store,
		Lines:  lines,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(1), lines.calls.Load())

	// 1h59m later on Wednesday: still fresh.
	clock.Advance(2*time.Hour - time.Minute)
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(1), lines.calls.Load())

	// At 2h on Wednesday: stale, fetches again.
	clock.Advance(time.Minute)
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(2), lines.calls.Load())

	// Sunday before kickoff: 15m window.
	clock.mu.Lock()
	clock.now = novemberTime(8, 10, 0)
	clock.mu.Unlock()
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(3), lines.calls.Load())

	clock.Advance(14 * time.Minute)
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(3), lines.calls.Load())

	clock.Advance(time.Minute)
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(4), lines.calls.Load())

	// After kickoff: skipped.
	clock.mu.Lock()
	clock.now = kickoff.Add(5 * time.Minute)
	clock.mu.Unlock()
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(4), lines.calls.Load())
}

func TestSyncPickGameRefreshesPropsAndSkipsKickedOff(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	lines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "evt-401",
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
			CommenceTime: kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	props := &blockingPropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(255.5),
				Price:      -115,
			}},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}

	clock := &fakeClock{now: novemberTime(8, 10, 0)}
	svc := New(Deps{
		Store:  store,
		Lines:  lines,
		Props:  props,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	// First SyncPickGame syncs lines (populating odds_event_id) and props.
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(1), lines.calls.Load())
	assert.Equal(t, int32(1), props.calls.Load())

	// Repeated calls (e.g., search typing) within 15m/30m make 0 extra API calls.
	clock.Advance(10 * time.Minute)
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(1), lines.calls.Load())
	assert.Equal(t, int32(1), props.calls.Load())

	// At 15m, lines refresh (15m game-day window) while props stay fresh (<30m).
	clock.Advance(5 * time.Minute)
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(2), lines.calls.Load())
	assert.Equal(t, int32(1), props.calls.Load())

	// At 30m, props refresh (30m game-day window) too.
	clock.Advance(15 * time.Minute)
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(3), lines.calls.Load())
	assert.Equal(t, int32(2), props.calls.Load())

	// Mismatched weekID skips props.
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID+99, game.ID))
	assert.Equal(t, int32(2), props.calls.Load())

	// After kickoff, props are skipped.
	clock.mu.Lock()
	clock.now = kickoff.Add(time.Minute)
	clock.mu.Unlock()
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(2), props.calls.Load())
}

func TestForcePickRefreshCooldown(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	lines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "evt-401",
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
			CommenceTime: kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	props := &blockingPropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(255.5),
				Price:      -115,
			}},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}

	clock := &fakeClock{now: novemberTime(8, 10, 0)}
	svc := New(Deps{
		Store:  store,
		Lines:  lines,
		Props:  props,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	// Initial pick-board load syncs lines only.
	require.NoError(t, svc.SyncPickLines(t.Context(), game.WeekID))
	assert.Equal(t, int32(1), lines.calls.Load())
	assert.Equal(t, int32(0), props.calls.Load())

	// Immediate ForcePickRefresh with gameID: lines are in 2m cooldown, props are never fetched so they run.
	clock.Advance(30 * time.Second)
	require.NoError(t, svc.ForcePickRefresh(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(1), lines.calls.Load())
	assert.Equal(t, int32(1), props.calls.Load())

	// Double-click 10 seconds later: both lines and props are inside 2m cooldown.
	clock.Advance(10 * time.Second)
	require.NoError(t, svc.ForcePickRefresh(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(1), lines.calls.Load())
	assert.Equal(t, int32(1), props.calls.Load())

	// At 2m after initial lines fetch (1m30s after props fetch): lines refresh, props still in cooldown.
	clock.Advance(80 * time.Second)
	require.NoError(t, svc.ForcePickRefresh(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(2), lines.calls.Load())
	assert.Equal(t, int32(1), props.calls.Load())

	// 30s later (2m after props fetch): props refresh, lines in cooldown.
	clock.Advance(30 * time.Second)
	require.NoError(t, svc.ForcePickRefresh(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(2), lines.calls.Load())
	assert.Equal(t, int32(2), props.calls.Load())

	// With empty gameID after 2m: refreshes lines only.
	clock.Advance(2 * time.Minute)
	require.NoError(t, svc.ForcePickRefresh(t.Context(), game.WeekID, ""))
	assert.Equal(t, int32(3), lines.calls.Load())
	assert.Equal(t, int32(2), props.calls.Load())
}

func TestPickSyncSingleFlight(t *testing.T) {
	t.Parallel()

	t.Run("SyncPickLines coalesces concurrent callers", func(t *testing.T) {
		t.Parallel()

		store := openTestStore(t)
		kickoff := novemberTime(8, 13, 0)
		game := setupTestGame(t, store, kickoff, db.GameScheduled)

		lines := &blockingLinesClient{
			events: []odds.Event{{
				ID:           "evt-401",
				HomeTeam:     "Buffalo Bills",
				AwayTeam:     "Miami Dolphins",
				CommenceTime: kickoff,
			}},
			eventsCredits: odds.Credits{Remaining: 400},
			lineCredits:   odds.Credits{Remaining: 397},
			started:       make(chan struct{}),
			release:       make(chan struct{}),
		}
		svc := New(Deps{
			Store:  store,
			Lines:  lines,
			Now:    func() time.Time { return novemberTime(8, 10, 0) },
			Logger: zerolog.Nop(),
		})

		const callers = 6
		var wg sync.WaitGroup
		errs := make(chan error, callers)
		for range callers {
			wg.Go(func() {
				errs <- svc.SyncPickLines(t.Context(), game.WeekID)
			})
		}

		<-lines.started
		close(lines.release)
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}
		assert.Equal(t, int32(1), lines.calls.Load())
	})

	t.Run("SyncPickGame coalesces concurrent callers", func(t *testing.T) {
		t.Parallel()

		store := openTestStore(t)
		kickoff := novemberTime(8, 13, 0)
		game := setupTestGame(t, store, kickoff, db.GameScheduled)
		_, err := store.SetGameOddsEvent(t.Context(), game.ID, "evt-401")
		require.NoError(t, err)
		require.NoError(t, store.RecordSync(t.Context(), OddsLinesSource, novemberTime(8, 9, 55), new(400)))

		props := &blockingPropsClient{
			propsCredits: odds.Credits{Remaining: 397},
			started:      make(chan struct{}),
			release:      make(chan struct{}),
		}
		svc := New(Deps{
			Store:  store,
			Props:  props,
			Now:    func() time.Time { return novemberTime(8, 10, 0) },
			Logger: zerolog.Nop(),
		})

		const callers = 6
		var wg sync.WaitGroup
		errs := make(chan error, callers)
		for range callers {
			wg.Go(func() {
				errs <- svc.SyncPickGame(t.Context(), game.WeekID, game.ID)
			})
		}

		<-props.started
		close(props.release)
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}
		assert.Equal(t, int32(1), props.calls.Load())
	})

	t.Run("ForcePickRefresh coalesces concurrent callers", func(t *testing.T) {
		t.Parallel()

		store := openTestStore(t)
		kickoff := novemberTime(8, 13, 0)
		game := setupTestGame(t, store, kickoff, db.GameScheduled)

		lines := &blockingLinesClient{
			events: []odds.Event{{
				ID:           "evt-401",
				HomeTeam:     "Buffalo Bills",
				AwayTeam:     "Miami Dolphins",
				CommenceTime: kickoff,
			}},
			eventsCredits: odds.Credits{Remaining: 400},
			lineCredits:   odds.Credits{Remaining: 397},
			started:       make(chan struct{}),
			release:       make(chan struct{}),
		}
		props := &blockingPropsClient{
			propsCredits: odds.Credits{Remaining: 394},
			started:      make(chan struct{}),
			release:      make(chan struct{}),
		}
		svc := New(Deps{
			Store:  store,
			Lines:  lines,
			Props:  props,
			Now:    func() time.Time { return novemberTime(8, 10, 0) },
			Logger: zerolog.Nop(),
		})

		const callers = 6
		var wg sync.WaitGroup
		errs := make(chan error, callers)
		for range callers {
			wg.Go(func() {
				errs <- svc.ForcePickRefresh(t.Context(), game.WeekID, game.ID)
			})
		}

		<-lines.started
		close(lines.release)
		<-props.started
		close(props.release)
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}
		assert.Equal(t, int32(1), lines.calls.Load())
		assert.Equal(t, int32(1), props.calls.Load())
	})
}

func TestKickPickLinesAsyncSingleFlightAndPublish(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	_, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "401772001",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeName:  "Buffalo Bills",
		AwayName:  "Miami Dolphins",
		Kickoff:   kickoff,
		Status:    db.GameScheduled,
		UpdatedAt: novemberTime(4, 12, 0),
	})
	require.NoError(t, err)

	lines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "evt-401",
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
			CommenceTime: kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
		}},
		lineCredits: odds.Credits{Remaining: 397},
		started:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	publisher := &recordingPublisher{}
	svc := New(Deps{
		Store:     store,
		Lines:     lines,
		Publisher: publisher,
		Now:       func() time.Time { return novemberTime(8, 10, 0) },
		Logger:    zerolog.Nop(),
	})

	// Cancel caller context immediately after kicking to verify request
	// cancellation does not abort the background refresh.
	for range 4 {
		reqCtx, cancel := context.WithCancel(t.Context())
		svc.KickPickLines(reqCtx, week.ID)
		cancel()
	}

	// Upstream fetch should be in flight without blocking KickPickLines.
	<-lines.started
	assert.Empty(t, publisher.seen())

	close(lines.release)
	svc.wait()

	assert.Equal(t, int32(1), lines.calls.Load())
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	// Calling KickPickLines after Shutdown does not start new work.
	require.NoError(t, svc.Shutdown(t.Context()))
	svc.KickPickLines(t.Context(), week.ID)
	svc.wait()
	assert.Equal(t, int32(1), lines.calls.Load())
}

func TestKickPickGameAsyncSingleFlightAndPublish(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "401772001",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeName:  "Buffalo Bills",
		AwayName:  "Miami Dolphins",
		Kickoff:   kickoff,
		Status:    db.GameScheduled,
		UpdatedAt: novemberTime(4, 12, 0),
	})
	require.NoError(t, err)
	_, err = store.SetGameOddsEvent(t.Context(), game.ID, "evt-401")
	require.NoError(t, err)

	clock := &fakeClock{now: novemberTime(8, 10, 0)}
	require.NoError(t, store.RecordSync(t.Context(), OddsLinesSource, clock.Now().Add(-time.Minute), new(400)))

	props := &blockingPropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(265.5),
				Price:      -115,
			}},
		},
		propsCredits: odds.Credits{Remaining: 397},
		started:      make(chan struct{}),
		release:      make(chan struct{}),
	}
	publisher := &recordingPublisher{}
	svc := New(Deps{
		Store:     store,
		Props:     props,
		Publisher: publisher,
		Now:       clock.Now,
		Logger:    zerolog.Nop(),
	})

	for range 4 {
		reqCtx, cancel := context.WithCancel(t.Context())
		svc.KickPickGame(reqCtx, week.ID, game.ID)
		cancel()
	}

	<-props.started
	assert.Empty(t, publisher.seen())

	close(props.release)
	svc.wait()

	assert.Equal(t, int32(1), props.calls.Load())
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	// Subsequent stale refresh with no prop rows does not republish.
	publisher.reset()
	props.props = nil
	clock.Advance(31 * time.Minute)
	svc.KickPickGame(t.Context(), week.ID, game.ID)
	svc.wait()

	assert.Equal(t, int32(2), props.calls.Load())
	assert.Empty(t, publisher.seen())
}

func TestSyncGamePropsEnrichesPrefilledPropsAndEnforcesEnrichFloor(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	t0 := novemberTime(8, 9, 30)
	clock := &fakeClock{now: t0}

	// Seed Parlay-only props and odds:props:{gameID} as RunPrefill would.
	require.NoError(t, store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.UpsertOdds(t.Context(), []db.Odds{
			{
				GameID:     game.ID,
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(265.5),
				Price:      -110,
				FetchedAt:  t0,
			},
			{
				GameID:     game.ID,
				Book:       "espnbet",
				MarketType: odds.MarketPassTds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(1.5),
				Price:      -115,
				FetchedAt:  t0,
			},
		})
		if err != nil {
			return err
		}
		return tx.RecordSync(t.Context(), OddsPropsSource(game.ID), t0, nil)
	}))

	props := &blockingPropsClient{}
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{{
			ID:           "toa-401",
			CommenceTime: kickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: TheOddsAPIEnrichFloor - 1},
		eventProps: map[string][]odds.Prop{
			"toa-401": {{
				EventID:    "toa-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(265.5),
				Price:      -110,
				Link:       "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-sid-1",
				Sid:        "dk-sid-1",
				EventSid:   "dk-evt-401",
			}},
		},
		eventPropsCredits: odds.Credits{Remaining: TheOddsAPIEnrichFloor - 2},
	}

	svc := New(Deps{
		Store:         store,
		Props:         props,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	// Below TheOddsAPIEnrichFloor: EventProps is skipped and no failure backoff is recorded.
	clock.Advance(5 * time.Minute)
	require.NoError(t, svc.SyncGameProps(t.Context(), game.ID))
	assert.Equal(t, int32(0), props.calls.Load())
	assert.Equal(t, 0, fallbackProps.eventPropsCalls)
	assert.False(t, svc.paused(OddsPropsEnrichSource(game.ID), clock.Now()))

	_, err := store.SyncState(t.Context(), OddsPropsEnrichSource(game.ID))
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)

	// Once credits meet TheOddsAPIEnrichFloor, calling immediately (without advancing past
	// failureBackoff) enriches existing props and stamps a unified FetchedAt.
	fallbackProps.eventsCredits = odds.Credits{Remaining: TheOddsAPIEnrichFloor}
	t1 := clock.Now()
	require.NoError(t, svc.SyncGameProps(t.Context(), game.ID))
	assert.Equal(t, int32(0), props.calls.Load())
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)

	enrichState, err := store.SyncState(t.Context(), OddsPropsEnrichSource(game.ID))
	require.NoError(t, err)
	assert.True(t, enrichState.FetchedAt.Equal(t1))

	storedOdds, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, storedOdds, 2)
	for _, row := range storedOdds {
		assert.True(t, row.FetchedAt.Equal(t1), "all current prop rows share unified FetchedAt")
		if row.Book == "draftkings" {
			require.NotNil(t, row.Link)
			assert.Equal(t, "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-sid-1", *row.Link)
		}
	}

	// Repeating SyncGameProps while fresh makes zero upstream calls.
	require.NoError(t, svc.SyncGameProps(t.Context(), game.ID))
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
}

func TestSyncOddsPropsRecordsEnrichFreshnessWhenFallbackBought(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	lines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "evt-401",
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
			CommenceTime: kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	props := &blockingPropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(255.5),
				Price:      -115,
			}},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{{
			ID:           "toa-401",
			CommenceTime: kickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 250},
		eventProps: map[string][]odds.Prop{
			"toa-401": {{
				EventID:    "toa-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(255.5),
				Price:      -115,
				Link:       "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-sid-1",
				Sid:        "dk-sid-1",
				EventSid:   "dk-evt-401",
			}},
		},
		eventPropsCredits: odds.Credits{Remaining: 249},
	}

	clock := &fakeClock{now: novemberTime(8, 10, 0)}
	svc := New(Deps{
		Store:         store,
		Lines:         lines,
		Props:         props,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, int32(1), props.calls.Load())
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)

	enrichState, err := store.SyncState(t.Context(), OddsPropsEnrichSource(game.ID))
	require.NoError(t, err)
	assert.True(t, enrichState.FetchedAt.Equal(clock.Now()))

	// Subsequent call within freshness window does not re-run enrichment.
	clock.Advance(5 * time.Minute)
	require.NoError(t, svc.SyncPickGame(t.Context(), game.WeekID, game.ID))
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
}

func TestKickSlipExportEnrich(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}

	addTestGame := func(id, home, away, homeName, awayName string, kickoff time.Time) db.Game {
		g, err := store.UpsertGame(t.Context(), db.Game{
			ID:        id,
			WeekID:    week.ID,
			HomeTeam:  home,
			AwayTeam:  away,
			HomeName:  homeName,
			AwayName:  awayName,
			Kickoff:   kickoff,
			Status:    db.GameScheduled,
			UpdatedAt: novemberTime(4, 12, 0),
		})
		require.NoError(t, err)
		return g
	}

	unlinkedGame := addTestGame("401772101", "BUF", "MIA", "Buffalo Bills", "Miami Dolphins", novemberTime(8, 13, 0))
	linkedGame := addTestGame("401772102", "KC", "DEN", "Kansas City Chiefs", "Denver Broncos", novemberTime(8, 16, 0))
	lineOnlyGame := addTestGame(
		"401772103",
		"BAL",
		"CIN",
		"Baltimore Ravens",
		"Cincinnati Bengals",
		novemberTime(8, 16, 25),
	)
	startedGame := addTestGame("401772104", "NYJ", "NE", "New York Jets", "New England Patriots", novemberTime(8, 9, 0))

	require.NoError(t, store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.UpsertOdds(t.Context(), []db.Odds{
			{
				GameID:     unlinkedGame.ID,
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(255.5),
				Price:      -110,
				FetchedAt:  t0,
			},
			{
				GameID:     linkedGame.ID,
				Book:       "draftkings",
				MarketType: odds.MarketRushYds,
				PlayerName: "Isiah Pacheco",
				Side:       odds.SideOver,
				Line:       new(65.5),
				Price:      -110,
				Link:       new("https://sportsbook.draftkings.com/event/dk-kc?outcomes=dk-1"),
				Sid:        new("dk-1"),
				EventSid:   new("dk-kc"),
				FetchedAt:  t0,
			},
			{
				GameID:     linkedGame.ID,
				Book:       "fanduel",
				MarketType: odds.MarketRushYds,
				PlayerName: "Isiah Pacheco",
				Side:       odds.SideOver,
				Line:       new(65.5),
				Price:      -112,
				Link:       new("https://sportsbook.fanduel.com/addToBetslip?marketId=42.1&selectionId=fd-1"),
				Sid:        new("fd-1"),
				FetchedAt:  t0,
			},
			{
				GameID:     startedGame.ID,
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Drake Maye",
				Side:       odds.SideOver,
				Line:       new(210.5),
				Price:      -110,
				FetchedAt:  t0,
			},
		})
		if err != nil {
			return err
		}
		for _, id := range []string{unlinkedGame.ID, linkedGame.ID, lineOnlyGame.ID, startedGame.ID} {
			if err := tx.RecordSync(t.Context(), OddsPropsSource(id), t0, nil); err != nil {
				return err
			}
		}
		return nil
	}))

	members, err := store.SlipMembers(t.Context(), slip.ID)
	require.NoError(t, err)
	require.NotEmpty(t, members)
	ownerID := members[0].User.ID

	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ownerID,
		GameID:     new(unlinkedGame.ID),
		MarketType: odds.MarketPassYds,
		PlayerName: new("Josh Allen"),
		Side:       new(odds.SideOver),
		Line:       new(255.5),
		Selection:  "Josh Allen Over 255.5 Pass Yds",
		Odds:       -110,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ownerID,
		GameID:     new(linkedGame.ID),
		MarketType: odds.MarketRushYds,
		PlayerName: new("Isiah Pacheco"),
		Side:       new(odds.SideOver),
		Line:       new(65.5),
		Selection:  "Isiah Pacheco Over 65.5 Rush Yds",
		Odds:       -110,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ownerID,
		GameID:     new(lineOnlyGame.ID),
		MarketType: "ML",
		Team:       new("BAL"),
		Side:       new("TEAM"),
		Selection:  "BAL ML",
		Odds:       -140,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ownerID,
		GameID:     new(startedGame.ID),
		MarketType: odds.MarketPassYds,
		PlayerName: new("Drake Maye"),
		Side:       new(odds.SideOver),
		Line:       new(210.5),
		Selection:  "Drake Maye Over 210.5 Pass Yds",
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetSlipStatus(t.Context(), slip.ID, db.SlipLocked))

	props := &blockingPropsClient{}
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{
			{
				ID:           "toa-101",
				CommenceTime: unlinkedGame.Kickoff,
				HomeTeam:     "Buffalo Bills",
				AwayTeam:     "Miami Dolphins",
			},
			{
				ID:           "toa-102",
				CommenceTime: linkedGame.Kickoff,
				HomeTeam:     "Kansas City Chiefs",
				AwayTeam:     "Denver Broncos",
			},
			{
				ID:           "toa-103",
				CommenceTime: lineOnlyGame.Kickoff,
				HomeTeam:     "Baltimore Ravens",
				AwayTeam:     "Cincinnati Bengals",
			},
		},
		eventsCredits: odds.Credits{Remaining: 300},
		eventProps: map[string][]odds.Prop{
			"toa-101": {
				{
					EventID:    "toa-101",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(255.5),
					Price:      -110,
					Link:       "https://sportsbook.draftkings.com/event/dk-buf?outcomes=dk-allen",
					Sid:        "dk-allen",
					EventSid:   "dk-buf",
				},
				{
					EventID:    "toa-101",
					Book:       "fanduel",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(255.5),
					Price:      -115,
					Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.2&selectionId=fd-allen",
					Sid:        "fd-allen",
				},
			},
		},
		eventPropsCredits: odds.Credits{Remaining: 299},
	}

	publisher := &recordingPublisher{}
	svc := New(Deps{
		Store:         store,
		Props:         props,
		FallbackProps: fallbackProps,
		Publisher:     publisher,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	reqCtx, cancel := context.WithCancel(t.Context())
	svc.KickSlipExportEnrich(reqCtx, slip.ID)
	cancel()
	svc.wait()

	assert.Equal(t, int32(0), props.calls.Load())
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	_, err = store.SyncState(t.Context(), OddsPropsEnrichSource(unlinkedGame.ID))
	require.NoError(t, err)
	for _, skippedID := range []string{linkedGame.ID, lineOnlyGame.ID, startedGame.ID} {
		_, err = store.SyncState(t.Context(), OddsPropsEnrichSource(skippedID))
		require.ErrorIs(t, err, db.ErrSyncStateNotFound)
	}

	stored, err := store.OddsForGame(t.Context(), unlinkedGame.ID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	for _, row := range stored {
		require.NotNil(t, row.Link)
		assert.NotEmpty(t, *row.Link)
	}

	// Repeating KickSlipExportEnrich once both books are linked makes 0 upstream calls.
	publisher.reset()
	svc.KickSlipExportEnrich(t.Context(), slip.ID)
	svc.wait()
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
	assert.Empty(t, publisher.seen())
}

func TestEnrichGamePropsMergesUnresolvedVariantPlayerName(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	t0 := novemberTime(8, 6, 0)
	clock := &fakeClock{now: t0}

	// Seed an unlinked Parlay-only prop row with a punctuation variant ("D.J. Moore")
	// that is not canonicalized in roster_players, and mark odds:props:{game} fresh.
	require.NoError(t, store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.UpsertOdds(t.Context(), []db.Odds{{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: odds.MarketRecYds,
			PlayerName: "D.J. Moore",
			Side:       odds.SideOver,
			Line:       new(58.5),
			Price:      -110,
			FetchedAt:  t0,
		}})
		if err != nil {
			return err
		}
		return tx.RecordSync(t.Context(), OddsPropsSource(game.ID), t0, nil)
	}))

	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{{
			ID:           "toa-401",
			CommenceTime: kickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 250},
		eventProps: map[string][]odds.Prop{
			"toa-401": {{
				EventID:    "toa-401",
				Book:       "draftkings",
				MarketType: odds.MarketRecYds,
				PlayerName: "DJ Moore",
				Side:       odds.SideOver,
				Line:       new(58.5),
				Price:      -115,
				Link:       "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-moore",
				Sid:        "dk-moore",
				EventSid:   "dk-evt-401",
			}},
		},
		eventPropsCredits: odds.Credits{Remaining: 249},
	}

	svc := New(Deps{
		Store:         store,
		Props:         &blockingPropsClient{},
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	clock.Advance(5 * time.Minute)
	require.NoError(t, svc.SyncGameProps(t.Context(), game.ID))
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)

	stored, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, stored, 1, "variant player names should merge into a single row")
	require.NotNil(t, stored[0].Link)
	assert.Equal(t, "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-moore", *stored[0].Link)

	matched, ok := db.MatchBookOdds(db.Leg{
		GameID:     new(game.ID),
		MarketType: odds.MarketRecYds,
		PlayerName: new("D.J. Moore"),
		Side:       new(odds.SideOver),
	}, stored, "draftkings")
	require.True(t, ok)
	require.NotNil(t, matched.Link)
	assert.Equal(t, "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-moore", *matched.Link)
}

func TestKickSlipExportEnrichNonPreferredFanDuelAndGameLines(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 40, TheOddsAPIEnrichFloor)

	store, week, slip := publishTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}

	lineGame, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "401772201",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeName:  "Buffalo Bills",
		AwayName:  "Miami Dolphins",
		Kickoff:   novemberTime(8, 13, 0),
		Status:    db.GameScheduled,
		UpdatedAt: novemberTime(4, 12, 0),
	})
	require.NoError(t, err)

	propGame, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "401772202",
		WeekID:    week.ID,
		HomeTeam:  "KC",
		AwayTeam:  "DEN",
		HomeName:  "Kansas City Chiefs",
		AwayName:  "Denver Broncos",
		Kickoff:   novemberTime(8, 16, 0),
		Status:    db.GameScheduled,
		UpdatedAt: novemberTime(4, 12, 0),
	})
	require.NoError(t, err)

	// Seed Parlay-only game lines (missing DK link, non-42 FD prefix) and
	// prop rows (DK linked, non-42 FD prefix).
	require.NoError(t, store.Tx(t.Context(), func(tx *db.Store) error {
		_, err := tx.UpsertOdds(t.Context(), []db.Odds{
			{
				GameID:     lineGame.ID,
				Book:       "draftkings",
				MarketType: odds.MarketSpread,
				Team:       "BUF",
				Side:       odds.SideTeam,
				Line:       new(-3.5),
				Price:      -110,
				FetchedAt:  t0,
			},
			{
				GameID:     lineGame.ID,
				Book:       "fanduel",
				MarketType: odds.MarketSpread,
				Team:       "BUF",
				Side:       odds.SideTeam,
				Line:       new(-3.5),
				Price:      -110,
				Link:       new("https://sportsbook.fanduel.com/addToBetslip?marketId=717.1001&selectionId=fd-buf"),
				Sid:        new("fd-buf"),
				FetchedAt:  t0,
			},
			{
				GameID:     propGame.ID,
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Patrick Mahomes",
				Side:       odds.SideOver,
				Line:       new(275.5),
				Price:      -110,
				Link:       new("https://sportsbook.draftkings.com/event/dk-kc?outcomes=dk-mahomes"),
				Sid:        new("dk-mahomes"),
				EventSid:   new("dk-kc"),
				FetchedAt:  t0,
			},
			{
				GameID:     propGame.ID,
				Book:       "fanduel",
				MarketType: odds.MarketPassYds,
				PlayerName: "Patrick Mahomes",
				Side:       odds.SideOver,
				Line:       new(275.5),
				Price:      -115,
				Link: new(
					"https://sportsbook.fanduel.com/addToBetslip?marketId=734.199156488&selectionId=fd-mahomes",
				),
				Sid:       new("fd-mahomes"),
				FetchedAt: t0,
			},
		})
		if err != nil {
			return err
		}
		if err := tx.RecordSync(t.Context(), OddsLinesSource, t0, nil); err != nil {
			return err
		}
		return tx.RecordSync(t.Context(), OddsPropsSource(propGame.ID), t0, nil)
	}))

	members, err := store.SlipMembers(t.Context(), slip.ID)
	require.NoError(t, err)
	require.NotEmpty(t, members)
	ownerID := members[0].User.ID

	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ownerID,
		GameID:     new(lineGame.ID),
		MarketType: odds.MarketSpread,
		Team:       new("BUF"),
		Side:       new(odds.SideTeam),
		Line:       new(-3.5),
		Selection:  "BUF -3.5",
		Odds:       -110,
	})
	require.NoError(t, err)
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ownerID,
		GameID:     new(propGame.ID),
		MarketType: odds.MarketPassYds,
		PlayerName: new("Patrick Mahomes"),
		Side:       new(odds.SideOver),
		Line:       new(275.5),
		Selection:  "Patrick Mahomes Over 275.5 Pass Yds",
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetSlipStatus(t.Context(), slip.ID, db.SlipLocked))

	primaryLines := &blockingLinesClient{}
	fallbackLines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "toa-201",
			CommenceTime: lineGame.Kickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: TheOddsAPIEnrichFloor - 1},
		lines: []odds.Line{
			{
				EventID:    "toa-201",
				Book:       "draftkings",
				MarketType: odds.MarketSpread,
				Side:       odds.SideTeam,
				Team:       "Buffalo Bills",
				Line:       new(-3.5),
				Price:      -110,
				Link:       "https://sportsbook.draftkings.com/event/dk-buf?outcomes=dk-buf-spread",
				Sid:        "dk-buf-spread",
				EventSid:   "dk-buf",
			},
			{
				EventID:    "toa-201",
				Book:       "fanduel",
				MarketType: odds.MarketSpread,
				Side:       odds.SideTeam,
				Team:       "Buffalo Bills",
				Line:       new(-3.5),
				Price:      -110,
				Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.2001&selectionId=fd-buf-42",
				Sid:        "fd-buf-42",
			},
		},
		lineCredits: odds.Credits{Remaining: TheOddsAPIEnrichFloor - 4},
	}

	primaryProps := &blockingPropsClient{}
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{{
			ID:           "toa-202",
			CommenceTime: propGame.Kickoff,
			HomeTeam:     "Kansas City Chiefs",
			AwayTeam:     "Denver Broncos",
		}},
		eventsCredits: odds.Credits{Remaining: TheOddsAPIEnrichFloor - 1},
		eventProps: map[string][]odds.Prop{
			"toa-202": {
				{
					EventID:    "toa-202",
					Book:       "fanduel",
					MarketType: odds.MarketPassYds,
					PlayerName: "Patrick Mahomes",
					Side:       odds.SideOver,
					Line:       new(275.5),
					Price:      -112,
					Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=fd-mahomes-42",
					Sid:        "fd-mahomes-42",
				},
			},
		},
		eventPropsCredits: odds.Credits{Remaining: TheOddsAPIEnrichFloor - 2},
	}

	publisher := &recordingPublisher{}
	svc := New(Deps{
		Store:         store,
		Lines:         primaryLines,
		FallbackLines: fallbackLines,
		Props:         primaryProps,
		FallbackProps: fallbackProps,
		Publisher:     publisher,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	// Below TheOddsAPIEnrichFloor (39 < 40): neither lines nor props are bought,
	// and no enrich sync_state or failure backoff is recorded.
	clock.Advance(time.Minute)
	svc.KickSlipExportEnrich(t.Context(), slip.ID)
	svc.wait()
	assert.Equal(t, int32(0), primaryLines.calls.Load())
	assert.Equal(t, int32(0), fallbackLines.calls.Load())
	assert.Equal(t, int32(0), primaryProps.calls.Load())
	assert.Equal(t, 0, fallbackProps.eventPropsCalls)
	_, err = store.SyncState(t.Context(), OddsLinesEnrichSource)
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)
	_, err = store.SyncState(t.Context(), OddsPropsEnrichSource(propGame.ID))
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)

	// At TheOddsAPIEnrichFloor (40): both game-line and prop legs are enriched.
	fallbackLines.eventsCredits = odds.Credits{Remaining: TheOddsAPIEnrichFloor}
	fallbackProps.eventsCredits = odds.Credits{Remaining: TheOddsAPIEnrichFloor}
	svc.KickSlipExportEnrich(t.Context(), slip.ID)
	svc.wait()

	assert.Equal(t, int32(0), primaryLines.calls.Load())
	assert.Equal(t, int32(1), fallbackLines.calls.Load())
	assert.Equal(t, int32(0), primaryProps.calls.Load())
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
	wantTopics := append(weekTopics(week.ID, slip.ID), weekTopics(week.ID, slip.ID)...)
	assert.ElementsMatch(t, wantTopics, publisher.seen())

	_, err = store.SyncState(t.Context(), OddsLinesEnrichSource)
	require.NoError(t, err)
	_, err = store.SyncState(t.Context(), OddsPropsEnrichSource(propGame.ID))
	require.NoError(t, err)

	lineOdds, err := store.OddsForGame(t.Context(), lineGame.ID)
	require.NoError(t, err)
	require.Len(t, lineOdds, 2)
	for _, row := range lineOdds {
		require.NotNil(t, row.Link)
		if row.Book == "draftkings" {
			assert.Equal(t, "https://sportsbook.draftkings.com/event/dk-buf?outcomes=dk-buf-spread", *row.Link)
		} else {
			assert.Equal(
				t,
				"https://sportsbook.fanduel.com/addToBetslip?marketId=42.2001&selectionId=fd-buf-42",
				*row.Link,
			)
		}
	}

	propOdds, err := store.OddsForGame(t.Context(), propGame.ID)
	require.NoError(t, err)
	require.Len(t, propOdds, 2)
	for _, row := range propOdds {
		require.NotNil(t, row.Link)
		if row.Book == "draftkings" {
			assert.Equal(t, "https://sportsbook.draftkings.com/event/dk-kc?outcomes=dk-mahomes", *row.Link)
		} else {
			assert.Equal(
				t,
				"https://sportsbook.fanduel.com/addToBetslip?marketId=42.627958622&selectionId=fd-mahomes-42",
				*row.Link,
			)
		}
	}

	// Repeating KickSlipExportEnrich once both books have DK links and 42.* FD
	// prefixes makes 0 upstream calls.
	publisher.reset()
	svc.KickSlipExportEnrich(t.Context(), slip.ID)
	svc.wait()
	assert.Equal(t, int32(1), fallbackLines.calls.Load())
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
	assert.Empty(t, publisher.seen())
}

func TestKickSlipExportEnrichSkipsAfterEnrichedSyncOdds(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}

	game, err := store.UpsertGame(t.Context(), db.Game{
		ID:        "401772301",
		WeekID:    week.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		HomeName:  "Buffalo Bills",
		AwayName:  "Miami Dolphins",
		Kickoff:   novemberTime(8, 13, 0),
		Status:    db.GameScheduled,
		UpdatedAt: novemberTime(4, 12, 0),
	})
	require.NoError(t, err)

	members, err := store.SlipMembers(t.Context(), slip.ID)
	require.NoError(t, err)
	require.NotEmpty(t, members)

	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     members[0].User.ID,
		GameID:     new(game.ID),
		MarketType: odds.MarketSpread,
		Team:       new("BUF"),
		Side:       new(odds.SideTeam),
		Line:       new(-3.5),
		Selection:  "BUF -3.5",
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetSlipStatus(t.Context(), slip.ID, db.SlipLocked))

	primaryLines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "parlay-301",
			CommenceTime: game.Kickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 200},
		lines: []odds.Line{{
			EventID:    "parlay-301",
			Book:       "fanduel",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.3001&selectionId=fd-buf",
			Sid:        "fd-buf",
		}},
		lineCredits: odds.Credits{Remaining: 199},
	}
	fallbackLines := &blockingLinesClient{
		events: []odds.Event{{
			ID:           "toa-301",
			CommenceTime: game.Kickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 100},
		lines: []odds.Line{{
			EventID:    "toa-301",
			Book:       "fanduel",
			MarketType: odds.MarketSpread,
			Side:       odds.SideTeam,
			Team:       "Buffalo Bills",
			Line:       new(-3.5),
			Price:      -110,
			Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=42.3001&selectionId=fd-buf",
			Sid:        "fd-buf",
		}},
		lineCredits: odds.Credits{Remaining: 97},
	}

	svc := New(Deps{
		Store:         store,
		Lines:         primaryLines,
		FallbackLines: fallbackLines,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	// Enriched syncOdds buys both providers and must stamp OddsLinesEnrichSource.
	changed, err := svc.syncOdds(t.Context(), []db.Game{game}, clock.Now())
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, int32(1), primaryLines.calls.Load())
	assert.Equal(t, int32(1), fallbackLines.calls.Load())

	enrichState, err := store.SyncState(t.Context(), OddsLinesEnrichSource)
	require.NoError(t, err)
	assert.Equal(t, t0, enrichState.FetchedAt)

	// Even though DraftKings still has no link for the game-line leg,
	// KickSlipExportEnrich must not re-buy lines right after an enriched syncOdds.
	clock.Advance(time.Minute)
	svc.KickSlipExportEnrich(t.Context(), slip.ID)
	svc.wait()
	assert.Equal(t, int32(1), fallbackLines.calls.Load())
}
