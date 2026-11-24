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

func TestKickRunsGameDayPrefillOnceAndPersistsAcrossRestart(t *testing.T) {
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

	assert.Equal(t, 1, linesClient.lineCalls, "first game-day morning Kick runs odds:lines")
	assert.Equal(t, 1, propsClient.propsCalls, "first game-day morning Kick runs Parlay props")
	assert.Equal(t, 0, fallbackProps.eventPropsCalls, "game-day prefill never calls fallback EventProps")

	state, err := store.SyncState(t.Context(), GameDayPrefillSource)
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

	assert.Equal(t, 1, linesClient.lineCalls, "cold-start Kick on same game day omits odds:lines")
	assert.Equal(t, 1, propsClient.propsCalls, "cold-start Kick on same game day omits props")

	espnClient.mu.Lock()
	summariesAfterSecond := espnClient.summaryCalls
	espnClient.mu.Unlock()
	assert.Greater(t, summariesAfterSecond, summariesAfterFirst, "due free sources still refresh after prefill")
}

func TestKickGameDayPrefillSingleflightConcurrency(t *testing.T) {
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

	assert.Equal(t, 1, innerLines.lineCalls, "concurrent game-day Kick calls share one prefill flight")
	assert.Equal(t, 1, propsClient.propsCalls, "concurrent game-day Kick calls warm props once")
}

func TestKickGameDayPrefillZeroProgressBackoffAndFallback(t *testing.T) {
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
	_, err := store.SyncState(t.Context(), GameDayPrefillSource)
	require.ErrorIs(t, err, db.ErrSyncStateNotFound, "zero-progress failure does not record prefill:gameday")

	espnClient.mu.Lock()
	assert.Equal(t, 2, espnClient.summaryCalls, "freeSourcesFor fallback runs after zero-progress prefill failure")
	assert.Equal(t, 4, espnClient.rosterCalls, "freeSourcesFor fallback syncs rosters")
	espnClient.mu.Unlock()

	// Inside 5-minute backoff: Kick does not retry RunPrefill.
	clock.Advance(2 * time.Minute)
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 1, linesClient.lineCalls, "5m failure backoff suppresses immediate retry")

	// After 5-minute backoff expires and lines recover: Kick retries and records GameDayPrefillSource.
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

	_, err = store.SyncState(t.Context(), GameDayPrefillSource)
	require.NoError(t, err, "recovered prefill records prefill:gameday")
}

func TestKickGameDayPrefillPartialFailureRecordsCompletion(t *testing.T) {
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

	_, err := store.SyncState(t.Context(), GameDayPrefillSource)
	require.NoError(t, err, "partial odds progress records prefill:gameday so credits are not re-spent")

	clock.Advance(20 * time.Minute)
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 1, linesClient.lineCalls, "later Kick on same game day does not re-spend lines credits")
}

func TestRunPrefillIgnoresMissingOddsAPIKeyAndMarksGameDayDone(t *testing.T) {
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

	_, err = store.SyncState(t.Context(), GameDayPrefillSource)
	require.NoError(t, err, "RunPrefill records prefill:gameday")

	// A later KickContext on the same game day does not re-run RunPrefill.
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
	assert.Zero(t, linesClient.lineCalls, "Kick after RunPrefill on same game day does not re-run odds:lines")
}

func TestRunPrefillThenPickEnrichment(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 9, 30)
	clock := &fakeClock{now: t0}
	upcomingKickoff := novemberTime(8, 13, 0)
	pastKickoff := novemberTime(8, 8, 30)

	board := prefillBoard(upcomingKickoff, pastKickoff)
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
			"evt-401": {
				{
					EventID:    "evt-401",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(265.5),
					Price:      -110,
				},
				{
					EventID:    "evt-401",
					Book:       "espnbet",
					MarketType: odds.MarketPassTds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(1.5),
					Price:      -115,
				},
			},
		},
		propsCredits: odds.Credits{Remaining: 394},
	}
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{{
			ID:           "toa-401",
			CommenceTime: upcomingKickoff,
			HomeTeam:     "Buffalo Bills",
			AwayTeam:     "Miami Dolphins",
		}},
		eventsCredits: odds.Credits{Remaining: 250},
		eventProps: map[string][]odds.Prop{
			"toa-401": {
				{
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
				},
				{
					EventID:    "toa-401",
					Book:       "fanduel",
					MarketType: odds.MarketRushAtt,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(6.5),
					Price:      -120,
					Link:       "https://sportsbook.fanduel.com/addToBetslip?marketId=1&selectionId=2",
					Sid:        "fd-sid-1",
					EventSid:   "fd-evt-401",
				},
			},
		},
		eventPropsCredits: odds.Credits{Remaining: 249},
	}

	service := New(Deps{
		Store:         store,
		Scoreboard:    &countingScoreboard{board: board},
		Forecast:      fakeForecast{},
		Lines:         linesClient,
		Props:         propsClient,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	report, err := service.RunPrefill(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, report.PropsSynced)
	assert.Equal(t, 1, propsClient.propsCalls)
	assert.Equal(t, 0, fallbackProps.eventPropsCalls, "RunPrefill must remain Parlay-only")

	_, err = store.SyncState(t.Context(), OddsPropsSource("401"))
	require.NoError(t, err)
	_, err = store.SyncState(t.Context(), OddsPropsEnrichSource("401"))
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)

	// 5 minutes later, SyncPickGame sees fresh odds:props:401 so it skips Parlay API
	// and runs enrichment-only via The Odds API.
	clock.Advance(5 * time.Minute)
	t1 := clock.Now()
	require.NoError(t, service.SyncPickGame(t.Context(), report.WeekID, "401"))

	assert.Equal(t, 1, propsClient.propsCalls, "Parlay API Props() must not be called again")
	assert.Equal(t, 1, fallbackProps.eventPropsCalls, "The Odds API EventProps() must run once")

	enrichState, err := store.SyncState(t.Context(), OddsPropsEnrichSource("401"))
	require.NoError(t, err)
	assert.True(t, enrichState.FetchedAt.Equal(t1))

	propsState, err := store.SyncState(t.Context(), OddsPropsSource("401"))
	require.NoError(t, err)
	assert.True(t, propsState.FetchedAt.Equal(t1))

	storedOdds, err := store.OddsForGame(t.Context(), "401")
	require.NoError(t, err)

	var dkPass, parlayOnly, fdRush *db.Odds
	for i := range storedOdds {
		row := &storedOdds[i]
		switch {
		case row.Book == "draftkings" && row.MarketType == odds.MarketPassYds:
			dkPass = row
		case row.Book == "espnbet" && row.MarketType == odds.MarketPassTds:
			parlayOnly = row
		case row.Book == "fanduel" && row.MarketType == odds.MarketRushAtt:
			fdRush = row
		}
	}
	require.NotNil(t, dkPass, "DraftKings PASS_YDS row should exist")
	require.NotNil(t, dkPass.Link)
	assert.Equal(t, "https://sportsbook.draftkings.com/event/dk-evt-401?outcomes=dk-sid-1", *dkPass.Link)
	require.NotNil(t, dkPass.Sid)
	assert.Equal(t, "dk-sid-1", *dkPass.Sid)
	require.NotNil(t, dkPass.EventSid)
	assert.Equal(t, "dk-evt-401", *dkPass.EventSid)
	assert.True(t, dkPass.FetchedAt.Equal(t1))

	require.NotNil(t, parlayOnly, "Parlay-only prop row must remain visible under CurrentOdds")
	assert.True(t, parlayOnly.FetchedAt.Equal(t1), "Parlay-only row must be stamped at unified FetchedAt")

	require.NotNil(t, fdRush, "TOA-only FanDuel RUSH_ATT row should be added")
	require.NotNil(t, fdRush.Link)
	assert.True(t, fdRush.FetchedAt.Equal(t1))

	// Repeating SyncPickGame immediately makes zero upstream prop calls.
	require.NoError(t, service.SyncPickGame(t.Context(), report.WeekID, "401"))
	assert.Equal(t, 1, propsClient.propsCalls)
	assert.Equal(t, 1, fallbackProps.eventPropsCalls)
}

type recordingPropsClient struct {
	mu           sync.Mutex
	props        map[string][]odds.Prop
	propsCredits odds.Credits
	calledEvents []string
}

func (r *recordingPropsClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return nil, odds.Credits{}, nil
}

func (r *recordingPropsClient) Props(_ context.Context, eventID string) ([]odds.Prop, odds.Credits, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calledEvents = append(r.calledEvents, eventID)
	return r.props[eventID], r.propsCredits, nil
}

func (r *recordingPropsClient) eventsCalled() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calledEvents))
	copy(out, r.calledEvents)
	return out
}

func buildMultiDayTestBoard(
	season espn.Season,
	week espn.Week,
	games []espn.Game,
) (espn.Scoreboard, []odds.Event, map[string][]odds.Prop) {
	board := espn.Scoreboard{
		Season: season,
		Week:   week,
		Games:  games,
	}
	events := make([]odds.Event, 0, len(games))
	props := make(map[string][]odds.Prop, len(games))
	for _, g := range games {
		eventID := "evt-" + g.ID
		events = append(events, odds.Event{
			ID:           eventID,
			CommenceTime: g.Kickoff,
			HomeTeam:     g.Home.Name,
			AwayTeam:     g.Away.Name,
		})
		props[eventID] = []odds.Prop{{
			EventID:    eventID,
			Book:       "draftkings",
			MarketType: odds.MarketPassYds,
			PlayerName: "Test QB " + g.ID,
			Side:       odds.SideOver,
			Line:       new(245.5),
			Price:      -110,
		}}
	}
	return board, events, props
}

func thanksgivingTestGames() []espn.Game {
	return []espn.Game{
		{
			ID:      "tg-1",
			Kickoff: novemberTime(26, 12, 30),
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "8", Abbreviation: "DET", Name: "Detroit Lions"},
			Away:    espn.Team{ID: "9", Abbreviation: "GB", Name: "Green Bay Packers"},
		},
		{
			ID:      "tg-2",
			Kickoff: novemberTime(26, 16, 30),
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "6", Abbreviation: "DAL", Name: "Dallas Cowboys"},
			Away:    espn.Team{ID: "28", Abbreviation: "WSH", Name: "Washington Commanders"},
		},
		{
			ID:      "tg-3",
			Kickoff: novemberTime(26, 20, 20),
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "33", Abbreviation: "BAL", Name: "Baltimore Ravens"},
			Away:    espn.Team{ID: "4", Abbreviation: "CIN", Name: "Cincinnati Bengals"},
		},
		{
			ID:      "bf-1",
			Kickoff: novemberTime(27, 15, 0),
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "12", Abbreviation: "KC", Name: "Kansas City Chiefs"},
			Away:    espn.Team{ID: "13", Abbreviation: "LV", Name: "Las Vegas Raiders"},
		},
		{
			ID:      "sun-1",
			Kickoff: novemberTime(29, 13, 0),
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
			Away:    espn.Team{ID: "15", Abbreviation: "MIA", Name: "Miami Dolphins"},
		},
		{
			ID:      "sun-2",
			Kickoff: novemberTime(29, 16, 25),
			Status:  espn.Scheduled,
			Home:    espn.Team{ID: "17", Abbreviation: "NE", Name: "New England Patriots"},
			Away:    espn.Team{ID: "20", Abbreviation: "NYJ", Name: "New York Jets"},
		},
	}
}

func TestKickMultiDayHolidayPrefillIdempotencyAndCreditConservation(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(24, 10, 0)} // Tuesday (non-game day)
	board, events, propsMap := buildMultiDayTestBoard(
		espn.Season{Year: 2026, Type: 2},
		espn.Week{Number: 12},
		thanksgivingTestGames(),
	)

	linesClient := &fakeLines{
		events:        events,
		eventsCredits: odds.Credits{Remaining: 450},
		lines: []odds.Line{{
			EventID:    "evt-tg-1",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Detroit Lions",
			Side:       odds.SideTeam,
			Price:      -140,
		}},
		lineCredits: odds.Credits{Remaining: 447},
	}
	propsClient := &recordingPropsClient{
		props:        propsMap,
		propsCredits: odds.Credits{Remaining: 440},
	}
	fallbackProps := &fakeFallbackPropsClient{}

	service := New(Deps{
		Store:         store,
		Scoreboard:    fakeScoreboard{board: board},
		Lines:         linesClient,
		Props:         propsClient,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	// 1. Tuesday (non-game day): Kick must not trigger RunPrefill.
	service.KickContext(t.Context())
	service.wait()
	assert.Zero(t, linesClient.lineCalls)
	assert.Empty(t, propsClient.eventsCalled())
	_, err := store.SyncState(t.Context(), GameDayPrefillSource)
	require.ErrorIs(t, err, db.ErrSyncStateNotFound)

	// 2. Thanksgiving Thursday 5:55 AM ET (before 6:00 AM ET window): no prefill.
	clock.Advance(novemberTime(26, 5, 55).Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()
	assert.Zero(t, linesClient.lineCalls)
	assert.Empty(t, propsClient.eventsCalled())

	// 3. Thanksgiving Thursday 6:05 AM ET: runs prefill, warms ONLY Thursday's 3 games.
	tThu := novemberTime(26, 6, 5)
	clock.Advance(tThu.Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()

	assert.Equal(t, 1, linesClient.lineCalls)
	calledAfterThu := propsClient.eventsCalled()
	assert.ElementsMatch(t, []string{"evt-tg-1", "evt-tg-2", "evt-tg-3"}, calledAfterThu)
	state, err := store.SyncState(t.Context(), GameDayPrefillSource)
	require.NoError(t, err)
	assert.True(t, state.FetchedAt.Equal(tThu))

	// 4. Subsequent Thanksgiving Thursday visit at 10:00 AM ET: idempotent on same ET date.
	clock.Advance(novemberTime(26, 10, 0).Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 1, linesClient.lineCalls)
	assert.Len(t, propsClient.eventsCalled(), 3)

	// 5. Black Friday 6:05 AM ET: new ET game date triggers prefill, warms ONLY Friday's game.
	tFri := novemberTime(27, 6, 5)
	clock.Advance(tFri.Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()

	assert.Equal(t, 2, linesClient.lineCalls)
	calledAfterFri := propsClient.eventsCalled()
	require.Len(t, calledAfterFri, 4)
	assert.Equal(t, []string{"evt-bf-1"}, calledAfterFri[3:])
	state, err = store.SyncState(t.Context(), GameDayPrefillSource)
	require.NoError(t, err)
	assert.True(t, state.FetchedAt.Equal(tFri))

	// 6. Repeat Black Friday visit at 11:00 AM ET: skipped.
	clock.Advance(novemberTime(27, 11, 0).Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 2, linesClient.lineCalls)
	assert.Len(t, propsClient.eventsCalled(), 4)

	// 7. Saturday (non-game day): skipped.
	clock.Advance(novemberTime(28, 10, 0).Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 2, linesClient.lineCalls)
	assert.Len(t, propsClient.eventsCalled(), 4)

	// 8. Sunday 6:05 AM ET: triggers prefill, warms ONLY Sunday's 2 games.
	tSun := novemberTime(29, 6, 5)
	clock.Advance(tSun.Sub(clock.Now()))
	service.KickContext(t.Context())
	service.wait()

	assert.Equal(t, 3, linesClient.lineCalls)
	calledAfterSun := propsClient.eventsCalled()
	require.Len(t, calledAfterSun, 6)
	assert.ElementsMatch(t, []string{"evt-sun-1", "evt-sun-2"}, calledAfterSun[4:])
	assert.Zero(t, fallbackProps.eventsCalls, "automatic prefill must stay Parlay-only")
	assert.Zero(t, fallbackProps.eventPropsCalls, "automatic prefill never calls fallback EventProps")
}

func TestKickIrregularGameDaysPrefill(t *testing.T) {
	t.Parallel()

	t.Run("playoff saturday triggers once and skips after last kickoff", func(t *testing.T) {
		t.Parallel()

		store := openTestStore(t)
		board, events, propsMap := buildMultiDayTestBoard(
			espn.Season{Year: 2025, Type: 3},
			espn.Week{Number: 1},
			[]espn.Game{
				{
					ID:      "sat-1",
					Kickoff: easternTime(2026, time.January, 10, 16, 30),
					Status:  espn.Scheduled,
					Home:    espn.Team{ID: "2", Abbreviation: "BUF", Name: "Buffalo Bills"},
					Away:    espn.Team{ID: "15", Abbreviation: "MIA", Name: "Miami Dolphins"},
				},
				{
					ID:      "sat-2",
					Kickoff: easternTime(2026, time.January, 10, 20, 0),
					Status:  espn.Scheduled,
					Home:    espn.Team{ID: "33", Abbreviation: "BAL", Name: "Baltimore Ravens"},
					Away:    espn.Team{ID: "4", Abbreviation: "CIN", Name: "Cincinnati Bengals"},
				},
				{
					ID:      "sun-1",
					Kickoff: easternTime(2026, time.January, 11, 13, 0),
					Status:  espn.Scheduled,
					Home:    espn.Team{ID: "12", Abbreviation: "KC", Name: "Kansas City Chiefs"},
					Away:    espn.Team{ID: "13", Abbreviation: "LV", Name: "Las Vegas Raiders"},
				},
			},
		)

		clock := &fakeClock{now: easternTime(2026, time.January, 10, 6, 5)}
		linesClient := &fakeLines{
			events:        events,
			eventsCredits: odds.Credits{Remaining: 400},
			lines: []odds.Line{{
				EventID:    "evt-sat-1",
				Book:       "draftkings",
				MarketType: odds.MarketML,
				Team:       "Buffalo Bills",
				Side:       odds.SideTeam,
				Price:      -150,
			}},
			lineCredits: odds.Credits{Remaining: 397},
		}
		propsClient := &recordingPropsClient{
			props:        propsMap,
			propsCredits: odds.Credits{Remaining: 394},
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
		assert.Equal(t, 1, linesClient.lineCalls)
		assert.ElementsMatch(t, []string{"evt-sat-1", "evt-sat-2"}, propsClient.eventsCalled())

		// Repeat Saturday visit skips prefill.
		clock.Advance(2 * time.Hour)
		service.KickContext(t.Context())
		service.wait()
		assert.Equal(t, 1, linesClient.lineCalls)
		assert.Len(t, propsClient.eventsCalled(), 2)

		// Fresh store after Saturday's last kickoff does not trigger prefill.
		lateStore := openTestStore(t)
		lateLines := &fakeLines{events: events, eventsCredits: odds.Credits{Remaining: 400}}
		lateProps := &recordingPropsClient{props: propsMap, propsCredits: odds.Credits{Remaining: 394}}
		lateService := New(Deps{
			Store:      lateStore,
			Scoreboard: fakeScoreboard{board: board},
			Lines:      lateLines,
			Props:      lateProps,
			Now:        func() time.Time { return easternTime(2026, time.January, 10, 20, 5) },
			Logger:     zerolog.Nop(),
		})
		lateService.KickContext(t.Context())
		lateService.wait()
		assert.Zero(t, lateLines.lineCalls)
		assert.Empty(t, lateProps.eventsCalled())
	})

	t.Run("christmas wednesday triggers once for wednesday games", func(t *testing.T) {
		t.Parallel()

		store := openTestStore(t)
		board, events, propsMap := buildMultiDayTestBoard(
			espn.Season{Year: 2024, Type: 2},
			espn.Week{Number: 17},
			[]espn.Game{
				{
					ID:      "xmas-1",
					Kickoff: easternTime(2024, time.December, 25, 13, 0),
					Status:  espn.Scheduled,
					Home:    espn.Team{ID: "23", Abbreviation: "PIT", Name: "Pittsburgh Steelers"},
					Away:    espn.Team{ID: "12", Abbreviation: "KC", Name: "Kansas City Chiefs"},
				},
				{
					ID:      "xmas-2",
					Kickoff: easternTime(2024, time.December, 25, 16, 30),
					Status:  espn.Scheduled,
					Home:    espn.Team{ID: "34", Abbreviation: "HOU", Name: "Houston Texans"},
					Away:    espn.Team{ID: "33", Abbreviation: "BAL", Name: "Baltimore Ravens"},
				},
				{
					ID:      "thu-1",
					Kickoff: easternTime(2024, time.December, 26, 20, 15),
					Status:  espn.Scheduled,
					Home:    espn.Team{ID: "3", Abbreviation: "CHI", Name: "Chicago Bears"},
					Away:    espn.Team{ID: "26", Abbreviation: "SEA", Name: "Seattle Seahawks"},
				},
			},
		)

		clock := &fakeClock{now: easternTime(2024, time.December, 25, 6, 5)}
		linesClient := &fakeLines{
			events:        events,
			eventsCredits: odds.Credits{Remaining: 400},
			lines: []odds.Line{{
				EventID:    "evt-xmas-1",
				Book:       "draftkings",
				MarketType: odds.MarketML,
				Team:       "Pittsburgh Steelers",
				Side:       odds.SideTeam,
				Price:      120,
			}},
			lineCredits: odds.Credits{Remaining: 397},
		}
		propsClient := &recordingPropsClient{
			props:        propsMap,
			propsCredits: odds.Credits{Remaining: 394},
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
		assert.Equal(t, 1, linesClient.lineCalls)
		assert.ElementsMatch(t, []string{"evt-xmas-1", "evt-xmas-2"}, propsClient.eventsCalled())

		_, err := store.SyncState(t.Context(), GameDayPrefillSource)
		require.NoError(t, err)

		// Repeat Christmas Wednesday visit skips prefill.
		clock.Advance(3 * time.Hour)
		service.KickContext(t.Context())
		service.wait()
		assert.Equal(t, 1, linesClient.lineCalls)
		assert.Len(t, propsClient.eventsCalled(), 2)
	})
}

func TestRunPrefillNonGameDayFallsBackToAllUpcomingWeekGames(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	board, events, propsMap := buildMultiDayTestBoard(
		espn.Season{Year: 2026, Type: 2},
		espn.Week{Number: 12},
		thanksgivingTestGames(),
	)

	linesClient := &fakeLines{
		events:        events,
		eventsCredits: odds.Credits{Remaining: 450},
		lines: []odds.Line{{
			EventID:    "evt-tg-1",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Detroit Lions",
			Side:       odds.SideTeam,
			Price:      -140,
		}},
		lineCredits: odds.Credits{Remaining: 447},
	}
	propsClient := &recordingPropsClient{
		props:        propsMap,
		propsCredits: odds.Credits{Remaining: 440},
	}

	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: board},
		Lines:      linesClient,
		Props:      propsClient,
		Now:        func() time.Time { return novemberTime(24, 10, 0) }, // Tuesday
		Logger:     zerolog.Nop(),
	})

	report, err := service.RunPrefill(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 6, report.UpcomingGames)
	assert.Equal(t, 6, report.PropsSynced)
	assert.ElementsMatch(
		t,
		[]string{"evt-tg-1", "evt-tg-2", "evt-tg-3", "evt-bf-1", "evt-sun-1", "evt-sun-2"},
		propsClient.eventsCalled(),
	)
}

func TestKickGameDayPrefillHaltsPropsBelowCreditFloor(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	board, events, propsMap := buildMultiDayTestBoard(
		espn.Season{Year: 2026, Type: 2},
		espn.Week{Number: 12},
		thanksgivingTestGames(),
	)

	clock := &fakeClock{now: novemberTime(26, 6, 5)}
	linesClient := &fakeLines{
		events:        events,
		eventsCredits: odds.Credits{Remaining: OddsPropsFloor},
		lines: []odds.Line{{
			EventID:    "evt-tg-1",
			Book:       "draftkings",
			MarketType: odds.MarketML,
			Team:       "Detroit Lions",
			Side:       odds.SideTeam,
			Price:      -140,
		}},
		lineCredits: odds.Credits{Remaining: OddsPropsFloor - 1},
	}
	propsClient := &recordingPropsClient{
		props:        propsMap,
		propsCredits: odds.Credits{Remaining: OddsPropsFloor - 2},
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

	assert.Equal(t, 1, linesClient.lineCalls)
	assert.Empty(t, propsClient.eventsCalled(), "Parlay props must halt below credit floor")
	_, err := store.SyncState(t.Context(), GameDayPrefillSource)
	require.NoError(t, err, "lines progress still marks game-day prefill done")

	clock.Advance(30 * time.Minute)
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, 1, linesClient.lineCalls)
	assert.Empty(t, propsClient.eventsCalled())
}
