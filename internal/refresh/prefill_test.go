package refresh

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/odds"
)

func prefillBoard(upcomingKickoff, pastKickoff time.Time) espn.Scoreboard {
	return espn.Scoreboard{
		Season: espn.Season{Year: 2026, Type: 2},
		Week:   espn.Week{Number: 9},
		Games: []espn.Game{
			{
				ID:      "401",
				Kickoff: upcomingKickoff,
				Status:  espn.Scheduled,
				Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
				Away:    espn.Team{ID: "15", Abbreviation: "MIA", Name: "Miami Dolphins"},
				Venue:   espn.Venue{ID: "11938", Name: "Highmark Stadium", Indoor: false},
			},
			{
				ID:      "402",
				Kickoff: pastKickoff,
				Status:  espn.Live,
				Home:    espn.Team{ID: "17", Abbreviation: "NE", Name: "New England Patriots"},
				Away:    espn.Team{ID: "20", Abbreviation: "NYJ", Name: "New York Jets"},
				Venue:   espn.Venue{ID: "11939", Name: "Gillette Stadium", Indoor: false},
			},
		},
	}
}

func TestRunPrefillOrchestrationAndIdempotency(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 9, 30)
	clock := &fakeClock{now: t0}
	upcomingKickoff := novemberTime(8, 13, 0)
	pastKickoff := novemberTime(8, 8, 30)

	board := prefillBoard(upcomingKickoff, pastKickoff)
	espnClient := &fakeESPNResearch{
		rosters: map[string][]espn.RosterPlayer{
			"2":  {{ID: "3918298", FullName: "Josh Allen", Position: "QB"}},
			"15": {{ID: "4241457", FullName: "Tua Tagovailoa", Position: "QB"}},
		},
		summaries: map[string]string{
			"401": `[{"team":"Buffalo Bills","injuries":[]}]`,
			"402": `[]`,
		},
	}
	linesClient := &fakeLines{
		events: []odds.Event{{
			ID:           "evt-401",
			CommenceTime: upcomingKickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Buffalo Bills",
			Side:       odds.SideTeam,
			Price:      -150,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	propsClient := &fakePropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(265.5),
				Price:      -110,
			}},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}
	fallbackProps := &fakeFallbackPropsClient{}

	service := New(Deps{
		Store:         store,
		Scoreboard:    &countingScoreboard{board: board},
		ESPN:          espnClient,
		Forecast:      fakeForecast{},
		Lines:         linesClient,
		Props:         propsClient,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	report, err := service.RunPrefill(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int64(2026209), report.WeekID)
	assert.Equal(t, 1, report.UpcomingGames)
	assert.True(t, report.LinesSynced)
	assert.Equal(t, 1, report.PropsSynced)
	assert.Equal(t, 0, report.PropsSkipped)
	assert.Equal(t, 0, report.PropsFailed)
	require.NotNil(t, report.ParlayRemaining)
	assert.Equal(t, 394, *report.ParlayRemaining)

	assert.Equal(t, 1, linesClient.lineCalls)
	assert.Equal(t, 1, propsClient.propsCalls, "only upcoming game 401 should fetch props")
	assert.Equal(t, 0, propsClient.eventsCalls, "odds:lines already warmed odds_event_id and balance")
	assert.Equal(t, 0, fallbackProps.eventsCalls, "prefill never calls fallback events")
	assert.Equal(t, 0, fallbackProps.eventPropsCalls, "prefill never calls fallback EventProps")

	storedGame, err := store.GameByID(t.Context(), "401")
	require.NoError(t, err)
	assert.Equal(t, new("evt-401"), storedGame.OddsEventID)
	assert.Equal(t, new(`[{"team":"Buffalo Bills","injuries":[]}]`), storedGame.InjuriesJSON)

	storedOdds, err := store.OddsForGame(t.Context(), "401")
	require.NoError(t, err)
	assert.NotEmpty(t, storedOdds)

	// Retry 14 minutes later: 15-minute idempotency skips lines and props.
	clock.Advance(14 * time.Minute)
	retryReport, err := service.RunPrefill(t.Context())
	require.NoError(t, err)
	assert.False(t, retryReport.LinesSynced)
	assert.Equal(t, 0, retryReport.PropsSynced)
	assert.Equal(t, 1, retryReport.PropsSkipped)
	assert.Equal(t, 1, linesClient.lineCalls)
	assert.Equal(t, 1, propsClient.propsCalls)

	// One minute later (15 minutes total): idempotency window expires.
	clock.Advance(time.Minute)
	refreshReport, err := service.RunPrefill(t.Context())
	require.NoError(t, err)
	assert.True(t, refreshReport.LinesSynced)
	assert.Equal(t, 1, refreshReport.PropsSynced)
	assert.Equal(t, 2, linesClient.lineCalls)
	assert.Equal(t, 2, propsClient.propsCalls)
}

func TestRunPrefillSkipsPropsBelowCreditFloor(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 9, 30)
	upcomingKickoff := novemberTime(8, 13, 0)

	board := espn.Scoreboard{
		Season: espn.Season{Year: 2026, Type: 2},
		Week:   espn.Week{Number: 9},
		Games: []espn.Game{{
			ID:      "401",
			Kickoff: upcomingKickoff,
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
			Away:    espn.Team{ID: "15", Abbreviation: "MIA", Name: "Miami Dolphins"},
		}},
	}
	linesClient := &fakeLines{
		events: []odds.Event{{
			ID:           "evt-401",
			CommenceTime: upcomingKickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: OddsPropsFloor},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Buffalo Bills",
			Side:       odds.SideTeam,
			Price:      -150,
		}},
		lineCredits: odds.Credits{Remaining: OddsPropsFloor - 1},
	}
	propsClient := &fakePropsClient{}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: board},
		Lines:      linesClient,
		Props:      propsClient,
		Now:        func() time.Time { return t0 },
		Logger:     zerolog.Nop(),
	})

	report, err := service.RunPrefill(t.Context())
	require.NoError(t, err)
	assert.True(t, report.LinesSynced)
	assert.Equal(t, 0, report.PropsSynced)
	assert.Equal(t, 1, report.PropsSkipped)
	assert.Equal(t, 0, propsClient.propsCalls)
}

func TestRunPrefillRespectsShutdown(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	service := New(Deps{
		Store:  store,
		Logger: zerolog.Nop(),
	})
	require.NoError(t, service.Shutdown(t.Context()))

	_, err := service.RunPrefill(t.Context())
	require.ErrorIs(t, err, context.Canceled)
}

func TestKickSyncsSummariesAndUniqueRostersWithoutOddsLines(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	// Outside the Sunday pre-fill window (Saturday 10:00 AM ET), Kick runs
	// only free sources and omits odds:lines.
	now := novemberTime(7, 10, 0)
	kickoff := novemberTime(8, 13, 0)

	board := espn.Scoreboard{
		Season: espn.Season{Year: 2026, Type: 2},
		Week:   espn.Week{Number: 9},
		Games: []espn.Game{
			{
				ID:      "401",
				Kickoff: kickoff,
				Status:  espn.Scheduled,
				Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
				Away:    espn.Team{ID: "15", Abbreviation: "MIA", Name: "Miami Dolphins"},
			},
			{
				ID:      "402",
				Kickoff: kickoff.Add(3 * time.Hour),
				Status:  espn.Scheduled,
				Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
				Away:    espn.Team{ID: "17", Abbreviation: "NE", Name: "New England Patriots"},
			},
			{
				ID:      "403",
				Kickoff: kickoff.Add(-24 * time.Hour),
				Status:  espn.Final,
				Home:    espn.Team{ID: "20", Abbreviation: "NYJ", Name: "New York Jets"},
				Away:    espn.Team{ID: "6", Abbreviation: "DAL", Name: "Dallas Cowboys"},
			},
		},
	}

	espnClient := &fakeESPNResearch{
		summaries: map[string]string{"401": `[]`, "402": `[]`},
		rosters:   map[string][]espn.RosterPlayer{},
	}
	linesClient := &fakeLines{
		eventsCredits: odds.Credits{Remaining: 400},
		lineCredits:   odds.Credits{Remaining: 397},
	}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: board},
		ESPN:       espnClient,
		Lines:      linesClient,
		Now:        func() time.Time { return now },
		Logger:     zerolog.Nop(),
	})

	service.KickContext(t.Context())
	service.wait()

	espnClient.mu.Lock()
	assert.Equal(t, 2, espnClient.summaryCalls, "Kick syncs summaries for non-final games only")
	assert.Equal(t, 3, espnClient.rosterCalls, "Kick syncs unique week teams for non-final games (BUF, MIA, NE)")
	espnClient.mu.Unlock()
	assert.Zero(t, linesClient.lineCalls, "Kick omits odds:lines")

	// RunOnce retains odds:lines for sandbox and full-pass callers.
	require.NoError(t, service.RunOnce(t.Context()))
	assert.Equal(t, 1, linesClient.lineCalls, "RunOnce still runs odds:lines")
}

type blockingLines struct {
	inner   *fakeLines
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingLines) Events(ctx context.Context) ([]odds.Event, odds.Credits, error) {
	return b.inner.Events(ctx)
}

func (b *blockingLines) Lines(ctx context.Context) ([]odds.Line, odds.Credits, error) {
	b.once.Do(func() { close(b.started) })
	select {
	case <-b.release:
	case <-ctx.Done():
		return nil, odds.Credits{}, ctx.Err()
	}
	return b.inner.Lines(ctx)
}

type partialErrPropsClient struct {
	inner     *fakePropsClient
	failEvent string
	err       error
}

func (p *partialErrPropsClient) Events(ctx context.Context) ([]odds.Event, odds.Credits, error) {
	return p.inner.Events(ctx)
}

func (p *partialErrPropsClient) Props(ctx context.Context, eventID string) ([]odds.Prop, odds.Credits, error) {
	if eventID == p.failEvent {
		return nil, odds.Credits{}, p.err
	}
	return p.inner.Props(ctx, eventID)
}

func TestKickRunsSundayPrefillOnceAndPersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 9, 30)
	clock := &fakeClock{now: t0}
	upcomingKickoff := novemberTime(8, 13, 0)
	pastKickoff := novemberTime(8, 8, 30)

	board := prefillBoard(upcomingKickoff, pastKickoff)
	espnClient := &fakeESPNResearch{
		rosters: map[string][]espn.RosterPlayer{
			"2":  {{ID: "3918298", FullName: "Josh Allen", Position: "QB"}},
			"15": {{ID: "4241457", FullName: "Tua Tagovailoa", Position: "QB"}},
		},
		summaries: map[string]string{
			"401": `[{"team":"Buffalo Bills","injuries":[]}]`,
			"402": `[]`,
		},
	}
	linesClient := &fakeLines{
		events: []odds.Event{{
			ID:           "evt-401",
			CommenceTime: upcomingKickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Buffalo Bills",
			Side:       odds.SideTeam,
			Price:      -150,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	propsClient := &fakePropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(265.5),
				Price:      -110,
			}},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}
	fallbackProps := &fakeFallbackPropsClient{}

	newService := func() *Service {
		return New(Deps{
			Store:         store,
			Scoreboard:    fakeScoreboard{board: board},
			ESPN:          espnClient,
			Forecast:      fakeForecast{},
			Lines:         linesClient,
			Props:         propsClient,
			FallbackProps: fallbackProps,
			Now:           clock.Now,
			Logger:        zerolog.Nop(),
		})
	}

	service1 := newService()
	service1.KickContext(t.Context())
	service1.wait()

	assert.Equal(t, 1, linesClient.lineCalls, "first Sunday morning Kick runs odds:lines")
	assert.Equal(t, 1, propsClient.propsCalls, "first Sunday morning Kick runs Parlay props")
	assert.Equal(t, 0, fallbackProps.eventPropsCalls, "Sunday prefill never calls fallback EventProps")

	state, err := store.SyncState(t.Context(), SundayPrefillSource)
	require.NoError(t, err)
	assert.True(t, state.FetchedAt.Equal(t0))

	espnClient.mu.Lock()
	summariesAfterFirst := espnClient.summaryCalls
	espnClient.mu.Unlock()
	assert.Positive(t, summariesAfterFirst)

	// Cold start on a brand-new Service sharing the same SQLite store 20m later
	// (past 15m summary game-day age) skips odds:lines/props and still refreshes
	// due free sources.
	clock.Advance(20 * time.Minute)
	service2 := newService()
	service2.KickContext(t.Context())
	service2.wait()

	assert.Equal(t, 1, linesClient.lineCalls, "cold-start Kick on same Sunday omits odds:lines")
	assert.Equal(t, 1, propsClient.propsCalls, "cold-start Kick on same Sunday omits props")

	espnClient.mu.Lock()
	summariesAfterSecond := espnClient.summaryCalls
	espnClient.mu.Unlock()
	assert.Greater(t, summariesAfterSecond, summariesAfterFirst, "due free sources still refresh after prefill")
}

func TestKickSundayPrefillSingleflightConcurrency(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	now := novemberTime(8, 9, 30)
	upcomingKickoff := novemberTime(8, 13, 0)
	pastKickoff := novemberTime(8, 8, 30)

	innerLines := &fakeLines{
		events: []odds.Event{{
			ID:           "evt-401",
			CommenceTime: upcomingKickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Buffalo Bills",
			Side:       odds.SideTeam,
			Price:      -150,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	lines := &blockingLines{
		inner:   innerLines,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	propsClient := &fakePropsClient{
		props: map[string][]odds.Prop{
			"evt-401": {{
				EventID:    "evt-401",
				Book:       "draftkings",
				MarketType: odds.MarketPassYds,
				PlayerName: "Josh Allen",
				Side:       odds.SideOver,
				Line:       new(265.5),
				Price:      -110,
			}},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: prefillBoard(upcomingKickoff, pastKickoff)},
		Lines:      lines,
		Props:      propsClient,
		Now:        func() time.Time { return now },
		Logger:     zerolog.Nop(),
	})

	service.KickContext(t.Context())
	<-lines.started

	for range 5 {
		service.KickContext(t.Context())
	}
	close(lines.release)
	service.wait()

	assert.Equal(t, 1, innerLines.lineCalls, "concurrent Sunday Kick calls share one prefill flight")
	assert.Equal(t, 1, propsClient.propsCalls, "concurrent Sunday Kick calls warm props once")
}

func TestKickSundayPrefillZeroProgressBackoffAndFallback(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(8, 9, 30)}
	upcomingKickoff := novemberTime(8, 13, 0)
	pastKickoff := novemberTime(8, 8, 30)

	espnClient := &fakeESPNResearch{
		rosters:   map[string][]espn.RosterPlayer{},
		summaries: map[string]string{"401": `[]`, "402": `[]`},
	}
	linesClient := &fakeLines{
		events: []odds.Event{{
			ID:           "evt-401",
			CommenceTime: upcomingKickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 400},
		linesErr:      errors.New("odds outage"),
	}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: prefillBoard(upcomingKickoff, pastKickoff)},
		ESPN:       espnClient,
		Lines:      linesClient,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	service.KickContext(t.Context())
	service.wait()

	assert.Equal(t, 1, linesClient.lineCalls)
	_, err := store.SyncState(t.Context(), SundayPrefillSource)
	require.ErrorIs(t, err, db.ErrSyncStateNotFound, "zero-progress failure does not record prefill:sunday")

	espnClient.mu.Lock()
	assert.Equal(t, 2, espnClient.summaryCalls, "freeSourcesFor fallback runs after zero-progress prefill failure")
	assert.Equal(t, 4, espnClient.rosterCalls, "freeSourcesFor fallback syncs rosters")
	espnClient.mu.Unlock()

	// Inside 5-minute backoff: Kick does not retry RunPrefill.
	clock.Advance(2 * time.Minute)
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 1, linesClient.lineCalls, "5m failure backoff suppresses immediate retry")

	// After 5-minute backoff expires and lines recover: Kick retries and records SundayPrefillSource.
	clock.Advance(3 * time.Minute)
	linesClient.linesErr = nil
	linesClient.lines = []odds.Line{{
		EventID:    "evt-401",
		Book:       "draftkings",
		MarketType: odds.MarketML,
		Team:       "Buffalo Bills",
		Side:       odds.SideTeam,
		Price:      -150,
	}}
	linesClient.lineCredits = odds.Credits{Remaining: 397}

	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 2, linesClient.lineCalls, "Kick retries prefill after 5m backoff expires")

	_, err = store.SyncState(t.Context(), SundayPrefillSource)
	require.NoError(t, err, "recovered prefill records prefill:sunday")
}

func TestKickSundayPrefillPartialFailureRecordsCompletion(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(8, 9, 30)}
	kickoff1 := novemberTime(8, 13, 0)
	kickoff2 := novemberTime(8, 16, 25)

	board := espn.Scoreboard{
		Season: espn.Season{Year: 2026, Type: 2},
		Week:   espn.Week{Number: 9},
		Games: []espn.Game{
			{
				ID:      "401",
				Kickoff: kickoff1,
				Status:  espn.Scheduled,
				Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
				Away:    espn.Team{ID: "15", Abbreviation: "MIA", Name: "Miami Dolphins"},
			},
			{
				ID:      "402",
				Kickoff: kickoff2,
				Status:  espn.Scheduled,
				Home:    espn.Team{ID: "17", Abbreviation: "NE", Name: "New England Patriots"},
				Away:    espn.Team{ID: "20", Abbreviation: "NYJ", Name: "New York Jets"},
			},
		},
	}
	linesClient := &fakeLines{
		events: []odds.Event{
			{ID: "evt-401", CommenceTime: kickoff1, HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins"},
			{ID: "evt-402", CommenceTime: kickoff2, HomeTeam: "New England Patriots", AwayTeam: "New York Jets"},
		},
		eventsCredits: odds.Credits{Remaining: 400},
		lines: []odds.Line{{
			EventID:    "evt-401",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Buffalo Bills",
			Side:       odds.SideTeam,
			Price:      -150,
		}},
		lineCredits: odds.Credits{Remaining: 397},
	}
	propsClient := &partialErrPropsClient{
		inner: &fakePropsClient{
			props: map[string][]odds.Prop{
				"evt-401": {{
					EventID:    "evt-401",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(265.5),
					Price:      -110,
				}},
			},
			propsCredits: odds.Credits{Remaining: 394},
		},
		failEvent: "evt-402",
		err:       errors.New("prop timeout"),
	}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: board},
		Lines:      linesClient,
		Props:      propsClient,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	service.KickContext(t.Context())
	service.wait()

	_, err := store.SyncState(t.Context(), SundayPrefillSource)
	require.NoError(t, err, "partial odds progress records prefill:sunday so credits are not re-spent")

	clock.Advance(20 * time.Minute)
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 1, linesClient.lineCalls, "later Kick on same Sunday does not re-spend lines credits")
}

func TestRunPrefillIgnoresMissingOddsAPIKeyAndMarksSundayDone(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(8, 9, 30)}
	upcomingKickoff := novemberTime(8, 13, 0)
	pastKickoff := novemberTime(8, 8, 30)

	linesClient := &fakeLines{
		eventsErr: odds.ErrNoAPIKey,
	}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: prefillBoard(upcomingKickoff, pastKickoff)},
		Lines:      linesClient,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	report, err := service.RunPrefill(t.Context())
	require.NoError(t, err, "odds.ErrNoAPIKey on lines does not fail RunPrefill")
	assert.False(t, report.LinesSynced)

	_, err = store.SyncState(t.Context(), SundayPrefillSource)
	require.NoError(t, err, "RunPrefill records prefill:sunday")

	// A later KickContext on the same Sunday does not re-run RunPrefill.
	clock.Advance(20 * time.Minute)
	linesClient.eventsErr = nil
	linesClient.events = []odds.Event{{
		ID:           "evt-401",
		CommenceTime: upcomingKickoff,
		HomeTeam:     "Buffalo Bills",
		AwayTeam:     "Miami Dolphins",
	}}
	linesClient.eventsCredits = odds.Credits{Remaining: 400}

	service.KickContext(t.Context())
	service.wait()
	assert.Zero(t, linesClient.lineCalls, "Kick after RunPrefill on Sunday does not re-run odds:lines")
}
