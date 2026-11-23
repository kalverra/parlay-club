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
