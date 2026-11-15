package refresh

import (
	"context"
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

type fakeESPNResearch struct {
	mu           sync.Mutex
	rosters      map[string][]espn.RosterPlayer
	rosterCalls  int
	summaries    map[string]string
	summaryCalls int
	gamelogs     map[string]espn.GameLog
	gamelogCalls int
}

func (f *fakeESPNResearch) Scoreboard(context.Context, espn.ScoreboardParams) (espn.Scoreboard, error) {
	return espn.Scoreboard{}, nil
}

func (f *fakeESPNResearch) Roster(_ context.Context, espnTeamID string) ([]espn.RosterPlayer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rosterCalls++
	return f.rosters[espnTeamID], nil
}

func (f *fakeESPNResearch) SummaryInjuries(_ context.Context, eventID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summaryCalls++
	return f.summaries[eventID], nil
}

func (f *fakeESPNResearch) GameLog(_ context.Context, athleteID string) (espn.GameLog, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gamelogCalls++
	return f.gamelogs[athleteID], nil
}

type fakePropsClient struct {
	mu            sync.Mutex
	events        []odds.Event
	eventsCredits odds.Credits
	eventsCalls   int
	props         map[string][]odds.Prop
	propsCredits  odds.Credits
	propsCalls    int
}

func (f *fakePropsClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventsCalls++
	return f.events, f.eventsCredits, nil
}

func (f *fakePropsClient) Props(_ context.Context, eventID string) ([]odds.Prop, odds.Credits, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.propsCalls++
	return f.props[eventID], f.propsCredits, nil
}

type fakeFallbackPropsClient struct {
	mu                sync.Mutex
	events            []odds.Event
	eventsCredits     odds.Credits
	eventsCalls       int
	eventProps        map[string][]odds.Prop
	eventPropsCredits odds.Credits
	eventPropsCalls   int
}

func (f *fakeFallbackPropsClient) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventsCalls++
	return f.events, f.eventsCredits, nil
}

func (f *fakeFallbackPropsClient) EventProps(_ context.Context, eventID string) ([]odds.Prop, odds.Credits, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventPropsCalls++
	return f.eventProps[eventID], f.eventPropsCredits, nil
}

func setupTestGame(t *testing.T, store *db.Store, kickoff time.Time, status db.GameStatus) db.Game {
	t.Helper()
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)

	game, err := store.UpsertGame(t.Context(), db.Game{
		ID:         "401",
		WeekID:     week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    kickoff,
		Status:     status,
		UpdatedAt:  kickoff.Add(-24 * time.Hour),
	})
	require.NoError(t, err)
	return game
}

func TestOpeningSameGameTwiceWithinCacheWindowSpendsCreditsOnce(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}
	kickoff := novemberTime(8, 13, 0)
	setupTestGame(t, store, kickoff, db.GameScheduled)

	espnClient := &fakeESPNResearch{
		rosters: map[string][]espn.RosterPlayer{
			"2": {{
				ID:          "3918298",
				FullName:    "Josh Allen",
				Position:    "QB",
				HeadshotURL: "https://a.espncdn.com/allen.png",
			}},
			"15": {{ID: "4241479", FullName: "Tua Tagovailoa", Position: "QB"}},
		},
	}
	propsClient := &fakePropsClient{
		events: []odds.Event{
			{ID: "parlay-401", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins", CommenceTime: kickoff},
		},
		eventsCredits: odds.Credits{Remaining: 200},
		props: map[string][]odds.Prop{
			"parlay-401": {
				{
					EventID:    "parlay-401",
					Book:       "fanduel",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(250.5),
					Price:      -110,
				},
			},
		},
		propsCredits: odds.Credits{Remaining: 197},
	}

	service := New(Deps{
		Store:  store,
		ESPN:   espnClient,
		Props:  propsClient,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	// 1. First open: cache is empty, fetches props and spends credits.
	err := service.SyncGameProps(t.Context(), "401")
	require.NoError(t, err)

	propsClient.mu.Lock()
	assert.Equal(t, 1, propsClient.eventsCalls)
	assert.Equal(t, 1, propsClient.propsCalls)
	propsClient.mu.Unlock()

	oddsRows, err := store.OddsForGame(t.Context(), "401")
	require.NoError(t, err)
	require.Len(t, oddsRows, 1)
	assert.Equal(t, "Josh Allen", oddsRows[0].PlayerName)
	assert.Equal(t, "BUF", oddsRows[0].Team, "resolved team via player matching")

	allen, err := store.RosterPlayerByID(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, "https://a.espncdn.com/allen.png", allen.HeadshotURL)

	// 2. Open again within the 2-hour cache window (1 hour later).
	clock.Advance(1 * time.Hour)
	err = service.SyncGameProps(t.Context(), "401")
	require.NoError(t, err)

	propsClient.mu.Lock()
	assert.Equal(t, 1, propsClient.propsCalls, "within 2h window, no second props call was made")
	propsClient.mu.Unlock()

	// 3. Open after the 2-hour cache window expires (another 61 minutes later = 2h 1m total).
	clock.Advance(61 * time.Minute)
	err = service.SyncGameProps(t.Context(), "401")
	require.NoError(t, err)

	propsClient.mu.Lock()
	assert.Equal(t, 2, propsClient.propsCalls, "after 2h window, a second props call was made")
	propsClient.mu.Unlock()
}

func TestPropsSkippedAfterKickoff(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	kickoff := novemberTime(8, 13, 0)
	clock := &fakeClock{now: kickoff.Add(5 * time.Minute)} // 5 min after kickoff
	setupTestGame(t, store, kickoff, db.GameLive)

	propsClient := &fakePropsClient{
		events: []odds.Event{
			{ID: "parlay-401", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins", CommenceTime: kickoff},
		},
		eventsCredits: odds.Credits{Remaining: 200},
	}
	service := New(Deps{
		Store:  store,
		Props:  propsClient,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	err := service.SyncGameProps(t.Context(), "401")
	require.NoError(t, err)

	propsClient.mu.Lock()
	assert.Equal(t, 0, propsClient.propsCalls, "props skipped after kickoff")
	propsClient.mu.Unlock()
}

func TestParlayAPICreditFloorAndFallback(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}
	kickoff := novemberTime(8, 13, 0)
	setupTestGame(t, store, kickoff, db.GameScheduled)

	// Parlay API sits below the 150-credit floor (140 remaining).
	propsClient := &fakePropsClient{
		events: []odds.Event{
			{ID: "parlay-401", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins", CommenceTime: kickoff},
		},
		eventsCredits: odds.Credits{Remaining: 140},
	}
	// The Odds API has 50 credits (>= 25 fallback floor).
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{
			{ID: "toa-401", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins", CommenceTime: kickoff},
		},
		eventsCredits: odds.Credits{Remaining: 50},
		eventProps: map[string][]odds.Prop{
			"toa-401": {
				{
					EventID:    "toa-401",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(245.5),
					Price:      -115,
					Link:       "https://sportsbook.draftkings.com/event/toa-401",
				},
			},
		},
		eventPropsCredits: odds.Credits{Remaining: 49},
	}

	service := New(Deps{
		Store:         store,
		Props:         propsClient,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	err := service.SyncGameProps(t.Context(), "401")
	require.NoError(t, err)

	propsClient.mu.Lock()
	assert.Equal(t, 0, propsClient.propsCalls, "parlay API skipped below 150 credit floor")
	propsClient.mu.Unlock()

	fallbackProps.mu.Lock()
	assert.Equal(t, 1, fallbackProps.eventPropsCalls, "fallback to The Odds API occurred")
	fallbackProps.mu.Unlock()

	oddsRows, err := store.OddsForGame(t.Context(), "401")
	require.NoError(t, err)
	require.Len(t, oddsRows, 1)
	assert.Equal(t, "draftkings", oddsRows[0].Book)
	assert.Equal(t, new("https://sportsbook.draftkings.com/event/toa-401"), oddsRows[0].Link)
}

func TestTheOddsAPIEnrichment(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}
	kickoff := novemberTime(8, 13, 0)
	setupTestGame(t, store, kickoff, db.GameScheduled)

	// Parlay API succeeds (200 credits remaining).
	propsClient := &fakePropsClient{
		events: []odds.Event{
			{ID: "parlay-401", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins", CommenceTime: kickoff},
		},
		eventsCredits: odds.Credits{Remaining: 200},
		props: map[string][]odds.Prop{
			"parlay-401": {
				{
					EventID:    "parlay-401",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(250.5),
					Price:      -110,
					// No link on primary
				},
			},
		},
		propsCredits: odds.Credits{Remaining: 197},
	}
	// The Odds API has 150 credits (>= 100 enrichment floor).
	fallbackProps := &fakeFallbackPropsClient{
		events: []odds.Event{
			{ID: "toa-401", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins", CommenceTime: kickoff},
		},
		eventsCredits: odds.Credits{Remaining: 150},
		eventProps: map[string][]odds.Prop{
			"toa-401": {
				{
					EventID:    "toa-401",
					Book:       "draftkings",
					MarketType: odds.MarketPassYds,
					PlayerName: "Josh Allen",
					Side:       odds.SideOver,
					Line:       new(250.5),
					Price:      -110,
					Link:       "https://sportsbook.draftkings.com/event/toa-401",
					Sid:        "dk-sid-1",
				},
			},
		},
		eventPropsCredits: odds.Credits{Remaining: 149},
	}

	service := New(Deps{
		Store:         store,
		Props:         propsClient,
		FallbackProps: fallbackProps,
		Now:           clock.Now,
		Logger:        zerolog.Nop(),
	})

	err := service.SyncGameProps(t.Context(), "401")
	require.NoError(t, err)

	propsClient.mu.Lock()
	assert.Equal(t, 1, propsClient.propsCalls)
	propsClient.mu.Unlock()

	fallbackProps.mu.Lock()
	assert.Equal(t, 1, fallbackProps.eventPropsCalls, "enrichment with The Odds API occurred")
	fallbackProps.mu.Unlock()

	oddsRows, err := store.OddsForGame(t.Context(), "401")
	require.NoError(t, err)
	require.Len(t, oddsRows, 1)
	assert.Equal(
		t,
		new("https://sportsbook.draftkings.com/event/toa-401"),
		oddsRows[0].Link,
		"DraftKings link enriched",
	)
	assert.Equal(t, new("dk-sid-1"), oddsRows[0].Sid)
}

func TestSyncGameResearchInjuriesAndRosters(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}
	kickoff := novemberTime(8, 13, 0)
	setupTestGame(t, store, kickoff, db.GameScheduled)

	espnClient := &fakeESPNResearch{
		rosters: map[string][]espn.RosterPlayer{
			"2":  {{ID: "3918298", FullName: "Josh Allen", Position: "QB"}},
			"15": {{ID: "4241479", FullName: "Tua Tagovailoa", Position: "QB"}},
		},
		summaries: map[string]string{
			"401": `[{"athlete_id":"3918298","status":"Questionable"}]`,
		},
	}

	service := New(Deps{
		Store:  store,
		ESPN:   espnClient,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	err := service.SyncGameResearch(t.Context(), "401")
	require.NoError(t, err)

	espnClient.mu.Lock()
	assert.Equal(t, 1, espnClient.summaryCalls)
	assert.Equal(t, 2, espnClient.rosterCalls)
	espnClient.mu.Unlock()

	game, err := store.GameByID(t.Context(), "401")
	require.NoError(t, err)
	require.NotNil(t, game.InjuriesJSON)
	assert.JSONEq(t, `[{"athlete_id":"3918298","status":"Questionable"}]`, *game.InjuriesJSON)

	roster, err := store.RosterForTeam(t.Context(), "BUF")
	require.NoError(t, err)
	require.Len(t, roster, 1)
	assert.Equal(t, "Josh Allen", roster[0].FullName)
}

func TestSyncPlayerResearchGameLog(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}

	espnClient := &fakeESPNResearch{
		gamelogs: map[string]espn.GameLog{
			"3918298": {
				AthleteID:   "3918298",
				GamesPlayed: 6,
				Markets: map[string][]float64{
					"PASS_YDS": {280.0, 310.0, 240.0},
				},
			},
		},
	}

	service := New(Deps{
		Store:  store,
		ESPN:   espnClient,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	err := service.SyncPlayerResearch(t.Context(), "3918298")
	require.NoError(t, err)

	espnClient.mu.Lock()
	assert.Equal(t, 1, espnClient.gamelogCalls)
	espnClient.mu.Unlock()

	log, err := store.PlayerGameLog(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, 6, log.GamesPlayed)
	assert.JSONEq(t, `{"PASS_YDS":[280.0,310.0,240.0]}`, log.StatsJSON)

	// Within 1h game-day window, no second fetch.
	clock.Advance(30 * time.Minute)
	err = service.SyncPlayerResearch(t.Context(), "3918298")
	require.NoError(t, err)

	espnClient.mu.Lock()
	assert.Equal(t, 1, espnClient.gamelogCalls)
	espnClient.mu.Unlock()

	// After 1h, re-fetches.
	clock.Advance(31 * time.Minute)
	err = service.SyncPlayerResearch(t.Context(), "3918298")
	require.NoError(t, err)

	espnClient.mu.Lock()
	assert.Equal(t, 2, espnClient.gamelogCalls)
	espnClient.mu.Unlock()
}

func TestKickRefreshesSlipLegGameLogs(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	t0 := novemberTime(8, 10, 0)
	clock := &fakeClock{now: t0}
	kickoff := novemberTime(8, 13, 0)
	game := setupTestGame(t, store, kickoff, db.GameScheduled)

	// Create user, group, slip, leg with PlayerID
	user, err := store.CreateUserWithIdentity(t.Context(), "Alice", "alice@test.com", "gid-alice", "blue")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Club")
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		game.WeekID,
		nil,
		"Week 9 Slip",
		db.SlateScopeSundayMain,
		1,
		[]int64{user.ID},
	)
	require.NoError(t, err)
	playerID := "3918298"
	_, err = store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     &game.ID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 250.5",
		Line:       new(250.5),
		Odds:       -110,
		PlayerName: new("Josh Allen"),
		PlayerID:   &playerID,
	})
	require.NoError(t, err)

	espnClient := &fakeESPNResearch{
		gamelogs: map[string]espn.GameLog{
			"3918298": {
				AthleteID:   "3918298",
				GamesPlayed: 8,
				Markets:     map[string][]float64{"PASS_YDS": {290.0}},
			},
		},
	}

	service := New(Deps{
		Store:  store,
		ESPN:   espnClient,
		Now:    clock.Now,
		Logger: zerolog.Nop(),
	})

	service.KickContext(t.Context())
	service.wait()

	espnClient.mu.Lock()
	assert.Equal(t, 1, espnClient.gamelogCalls, "Kick() synced gamelog for slip leg athlete")
	espnClient.mu.Unlock()

	log, err := store.PlayerGameLog(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, 8, log.GamesPlayed)
}
