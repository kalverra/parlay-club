package refresh

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	now := novemberTime(8, 10, 0)
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
