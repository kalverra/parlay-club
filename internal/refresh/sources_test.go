package refresh

import (
	"context"
	"errors"
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
)

// fakeScoreboard serves a saved board or an error.
type fakeScoreboard struct {
	board espn.Scoreboard
	err   error
}

// Scoreboard returns the prepared answer.
func (f fakeScoreboard) Scoreboard(context.Context, espn.ScoreboardParams) (espn.Scoreboard, error) {
	return f.board, f.err
}

// countingScoreboard counts the fetches it serves.
type countingScoreboard struct {
	board espn.Scoreboard
	calls atomic.Int32
}

// Scoreboard returns the prepared board and counts the call.
func (f *countingScoreboard) Scoreboard(context.Context, espn.ScoreboardParams) (espn.Scoreboard, error) {
	f.calls.Add(1)
	return f.board, nil
}

// fakeForecast serves a prepared forecast or an error.
type fakeForecast struct {
	forecast weather.Forecast
	err      error
}

// Forecast returns the prepared answer.
func (f fakeForecast) Forecast(context.Context, weather.Venue, time.Time) (weather.Forecast, error) {
	return f.forecast, f.err
}

// fakeLines serves event and line lists and counts the paid calls.
type fakeLines struct {
	events        []odds.Event
	eventsCredits odds.Credits
	eventsErr     error
	lines         []odds.Line
	lineCredits   odds.Credits
	linesErr      error
	lineCalls     int
}

// Events returns the prepared event list.
func (f *fakeLines) Events(context.Context) ([]odds.Event, odds.Credits, error) {
	return f.events, f.eventsCredits, f.eventsErr
}

// Lines returns the prepared line list and counts the call.
func (f *fakeLines) Lines(context.Context) ([]odds.Line, odds.Credits, error) {
	f.lineCalls++
	return f.lines, f.lineCredits, f.linesErr
}

// espnWeek is one game's board, with a Sunday kickoff in week 9.
func espnWeek() espn.Scoreboard {
	homeScore := 24
	awayScore := 17
	live := espn.Game{
		ID:      "401",
		Kickoff: novemberTime(8, 13, 0),
		Status:  espn.Live,
		Home: espn.Team{
			ID:           "2",
			Abbreviation: "BUF",
			Name:         "Buffalo Bills",
			Logo:         "https://a.espncdn.com/buf.png",
			Score:        &homeScore,
		},
		Away: espn.Team{
			ID:           "15",
			Abbreviation: "MIA",
			Name:         "Miami Dolphins",
			Logo:         "https://a.espncdn.com/mia.png",
			Score:        &awayScore,
		},
		Venue: espn.Venue{ID: "11938", Name: "Highmark Stadium"},
	}
	return espn.Scoreboard{
		Season: espn.Season{Year: 2026, Type: 2},
		Week:   espn.Week{Number: 9},
		Games:  []espn.Game{live},
	}
}

// storedGame is the games row syncScoreboard would write for espnWeek's game.
func storedGame(weekID int64, now time.Time) db.Game {
	score := 24
	away := 17
	return db.Game{
		ID:         "401",
		WeekID:     weekID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		HomeLogo:   new("https://a.espncdn.com/buf.png"),
		AwayLogo:   new("https://a.espncdn.com/mia.png"),
		Kickoff:    novemberTime(8, 13, 0),
		Status:     db.GameLive,
		VenueID:    new("11938"),
		VenueName:  new("Highmark Stadium"),
		HomeScore:  &score,
		AwayScore:  &away,
		UpdatedAt:  now,
	}
}

func TestSyncScoreboardWritesTheSlateAndRecordsTheFetch(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: espnWeek()},
		Now:        func() time.Time { return time.Time{} },
		Logger:     zerolog.Nop(),
	})
	now := novemberTime(8, 13, 5)

	changed, err := service.syncScoreboard(t.Context(), now)
	require.NoError(t, err)
	assert.True(t, changed)

	week, err := store.WeekByID(t.Context(), db.WeekID(2026, 2, 9))
	require.NoError(t, err)
	game, err := store.GameByID(t.Context(), "401")
	require.NoError(t, err)
	assert.Equal(t, storedGame(week.ID, now), game)

	state, err := store.SyncState(t.Context(), ScoreboardSource)
	require.NoError(t, err)
	assert.Equal(t, now, state.FetchedAt)

	// The same response at a later time changes no game rows.
	changed, err = service.syncScoreboard(t.Context(), now.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestSyncScoreboardNotifiesMarketDataChangeOnlyOnNewlyFinalGames(t *testing.T) {
	t.Parallel()

	board := espnWeek()
	sb := &countingScoreboard{board: board}
	store := openTestStore(t)
	service := New(Deps{
		Store:      store,
		Scoreboard: sb,
		Now:        func() time.Time { return time.Time{} },
		Logger:     zerolog.Nop(),
	})

	var calls atomic.Int32
	service.SetOnMarketDataChange(func() {
		calls.Add(1)
	})

	now := novemberTime(8, 13, 5)

	// First sync: game is Live -> changed=true, but NOT final -> no callback.
	changed, err := service.syncScoreboard(t.Context(), now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, int32(0), calls.Load())

	// Transition game to Final -> callback fires once.
	sb.board.Games[0].Status = espn.Final
	changed, err = service.syncScoreboard(t.Context(), now.Add(time.Hour))
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, int32(1), calls.Load())

	// Rerun with game still Final -> no newly final games -> callback does not fire again.
	changed, err = service.syncScoreboard(t.Context(), now.Add(2*time.Hour))
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, int32(1), calls.Load())
}

func TestSyncScoreboardReturnsTheFetchError(t *testing.T) {
	t.Parallel()

	service := New(Deps{
		Store:      openTestStore(t),
		Scoreboard: fakeScoreboard{err: errors.New("espn is down")},
		Logger:     zerolog.Nop(),
	})

	_, err := service.syncScoreboard(t.Context(), time.Now())
	require.Error(t, err)
}

type routedScoreboard struct {
	boards map[espn.ScoreboardParams]espn.Scoreboard
	calls  []espn.ScoreboardParams
}

func (r *routedScoreboard) Scoreboard(_ context.Context, params espn.ScoreboardParams) (espn.Scoreboard, error) {
	r.calls = append(r.calls, params)
	return r.boards[params], nil
}

func TestSyncScoreboardFetchesNextWeekWhenCurrentWeekIsAllFinal(t *testing.T) {
	t.Parallel()

	week11 := espnWeek()
	week11.Week.Number = 11
	week11.Games[0].ID = "401"
	week11.Games[0].Status = espn.Final
	week11.Games[0].Kickoff = novemberTime(24, 1, 15)

	week12 := espnWeek()
	week12.Week.Number = 12
	week12.Games[0].ID = "402"
	week12.Games[0].Status = espn.Scheduled
	week12.Games[0].Kickoff = novemberTime(26, 17, 30)

	sb := &routedScoreboard{
		boards: map[espn.ScoreboardParams]espn.Scoreboard{
			{}:                                      week11,
			{Season: 2026, SeasonType: 2, Week: 12}: week12,
		},
	}

	store := openTestStore(t)
	service := New(Deps{
		Store:      store,
		Scoreboard: sb,
		Logger:     zerolog.Nop(),
	})

	now := novemberTime(25, 3, 0)
	changed, err := service.syncScoreboard(t.Context(), now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, []espn.ScoreboardParams{
		{},
		{Season: 2026, SeasonType: 2, Week: 12},
	}, sb.calls)

	upcoming, err := store.UpcomingWeeks(t.Context(), now)
	require.NoError(t, err)
	require.Len(t, upcoming, 1)
	assert.Equal(t, db.WeekID(2026, 2, 12), upcoming[0].ID)

	_, err = store.GameByID(t.Context(), "402")
	require.NoError(t, err)
}

func TestSyncScoreboardFallsBackToPostseasonAfterRegularSeason(t *testing.T) {
	t.Parallel()

	week18 := espnWeek()
	week18.Week.Number = 18
	week18.Games[0].ID = "418"
	week18.Games[0].Status = espn.Final
	week18.Games[0].Kickoff = time.Date(2027, 1, 11, 1, 20, 0, 0, time.UTC)

	wildCard := espnWeek()
	wildCard.Season.Type = 3
	wildCard.Week.Number = 1
	wildCard.Games[0].ID = "501"
	wildCard.Games[0].Status = espn.Scheduled
	wildCard.Games[0].Kickoff = time.Date(2027, 1, 16, 21, 30, 0, 0, time.UTC)

	sb := &routedScoreboard{
		boards: map[espn.ScoreboardParams]espn.Scoreboard{
			{}: week18,
			{Season: 2026, SeasonType: 2, Week: 19}: {
				Season: espn.Season{Year: 2026, Type: 2},
				Week:   espn.Week{Number: 19},
			},
			{Season: 2026, SeasonType: 3, Week: 1}: wildCard,
		},
	}

	store := openTestStore(t)
	service := New(Deps{
		Store:      store,
		Scoreboard: sb,
		Logger:     zerolog.Nop(),
	})

	now := time.Date(2027, 1, 12, 15, 0, 0, 0, time.UTC)
	changed, err := service.syncScoreboard(t.Context(), now)
	require.NoError(t, err)
	assert.True(t, changed)

	upcoming, err := store.UpcomingWeeks(t.Context(), now)
	require.NoError(t, err)
	require.Len(t, upcoming, 1)
	assert.Equal(t, db.WeekID(2026, 3, 1), upcoming[0].ID)
}

func TestSyncScoreboardSkipsNextWeekWhenCurrentWeekHasNonFinalGame(t *testing.T) {
	t.Parallel()

	week11 := espnWeek()
	week11.Week.Number = 11
	week11.Games[0].Status = espn.Live

	sb := &countingScoreboard{board: week11}
	service := New(Deps{
		Store:      openTestStore(t),
		Scoreboard: sb,
		Logger:     zerolog.Nop(),
	})

	_, err := service.syncScoreboard(t.Context(), novemberTime(24, 2, 0))
	require.NoError(t, err)
	assert.Equal(t, int32(1), sb.calls.Load())
}

func TestSyncWeatherWritesTheForecastAndRecordsTheFetch(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	now := novemberTime(7, 11, 0)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)

	service := New(Deps{
		Store: store,
		Forecast: fakeForecast{forecast: weather.Forecast{
			TempF: 48.5, WindMPH: 18, WindGustMPH: 29, PrecipIN: 0.08,
		}},
		Logger: zerolog.Nop(),
	})

	changed, err := service.syncWeather(t.Context(), game, now)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(48.5), stored.TempF)
	assert.Equal(t, new(18.0), stored.WindMPH)
	assert.Equal(t, new(29.0), stored.WindGustMPH)
	assert.Equal(t, new(0.08), stored.PrecipIN)
	assert.Equal(t, now, stored.UpdatedAt)

	state, err := store.SyncState(t.Context(), WeatherSource(game.ID))
	require.NoError(t, err)
	assert.Equal(t, now, state.FetchedAt)

	changed, err = service.syncWeather(t.Context(), stored, now.Add(2*time.Hour))
	require.NoError(t, err)
	assert.False(t, changed, "the same forecast is not a board change")
}

func TestSyncOddsWritesOnlyMatchedGamesAndRecordsCredits(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	now := novemberTime(7, 11, 0)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)

	kickoff := game.Kickoff
	extra := novemberTime(15, 13, 0)
	lines := &fakeLines{
		events: []odds.Event{
			{
				ID: "evt-1", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins",
				CommenceTime: kickoff,
			},
			{
				ID: "evt-next", HomeTeam: "Dallas Cowboys", AwayTeam: "Philadelphia Eagles",
				CommenceTime: extra,
			},
		},
		eventsCredits: odds.Credits{Remaining: 490},
		lines: []odds.Line{
			{
				EventID: "evt-1", Book: "draftkings", MarketType: odds.MarketML,
				Team: "Buffalo Bills", Side: odds.SideTeam, Price: -155,
				Link: "https://sportsbook.draftkings.com/?outcomes=0ML1_1", Sid: "0ML1_1",
				EventSid: "34118128",
			},
			{
				EventID: "evt-1", Book: "draftkings", MarketType: odds.MarketSpread,
				Team: "Miami Dolphins", Side: odds.SideTeam, Line: new(2.5), Price: -110,
				EventSid: "34118128",
			},
			{
				EventID: "evt-1", Book: "draftkings", MarketType: odds.MarketTotal,
				Side: odds.SideOver, Line: new(45.5), Price: -108,
				EventSid: "34118128",
			},
			{
				EventID: "evt-next", Book: "draftkings", MarketType: odds.MarketML,
				Team: "Dallas Cowboys", Side: odds.SideTeam, Price: 120,
				EventSid: "99",
			},
		},
		lineCredits: odds.Credits{Remaining: 487},
	}
	service := New(Deps{Store: store, Lines: lines, Logger: zerolog.Nop()})

	changed, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new("evt-1"), stored.OddsEventID)

	rows, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, rows, 3, "the next week's event writes no rows")

	bySelection := make(map[string]db.Odds, len(rows))
	for _, row := range rows {
		bySelection[row.MarketType+"|"+row.Team+"|"+row.Side] = row
	}
	assert.Equal(t, db.Odds{
		GameID:     game.ID,
		Book:       "draftkings",
		MarketType: odds.MarketML,
		Team:       "BUF",
		Side:       odds.SideTeam,
		Price:      -155,
		Link:       new("https://sportsbook.draftkings.com/?outcomes=0ML1_1"),
		Sid:        new("0ML1_1"),
		EventSid:   new("34118128"),
		FetchedAt:  now,
	}, bySelection["ML|BUF|TEAM"])
	assert.Equal(t, db.Odds{
		GameID:     game.ID,
		Book:       "draftkings",
		MarketType: odds.MarketSpread,
		Team:       "MIA",
		Side:       odds.SideTeam,
		Line:       new(2.5),
		Price:      -110,
		EventSid:   new("34118128"),
		FetchedAt:  now,
	}, bySelection["SPREAD|MIA|TEAM"])
	assert.Equal(t, db.Odds{
		GameID:     game.ID,
		Book:       "draftkings",
		MarketType: odds.MarketTotal,
		Side:       odds.SideOver,
		Line:       new(45.5),
		Price:      -108,
		EventSid:   new("34118128"),
		FetchedAt:  now,
	}, bySelection["TOTAL||OVER"])

	state, err := store.SyncState(t.Context(), OddsLinesSource)
	require.NoError(t, err)
	require.NotNil(t, state.OddsCreditsRemaining)
	assert.Equal(t, 487, *state.OddsCreditsRemaining)
	assert.Equal(t, now, state.FetchedAt)
}

func TestSyncOddsSkipsTheLinesCallBelowTheFloor(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	now := novemberTime(7, 11, 0)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)

	lines := &fakeLines{
		events: []odds.Event{{
			ID: "evt-1", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins",
			CommenceTime: game.Kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: OddsLinesFloor - 1},
		lineCredits:   odds.Credits{Remaining: OddsLinesFloor - 4},
	}
	service := New(Deps{Store: store, Lines: lines, Logger: zerolog.Nop()})

	_, err = service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	assert.Zero(t, lines.lineCalls, "the paid lines call must not run below the floor")

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new("evt-1"), stored.OddsEventID, "matching is free and still runs")

	state, err := store.SyncState(t.Context(), OddsLinesSource)
	require.NoError(t, err)
	require.NotNil(t, state.OddsCreditsRemaining)
	assert.Equal(t, OddsLinesFloor-1, *state.OddsCreditsRemaining)
}

func TestKickUsesTheScoreboardStalenessClock(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	clock := &fakeClock{now: novemberTime(4, 12, 0)}
	scoreboard := &countingScoreboard{board: espnWeek()}
	lines := &fakeLines{eventsCredits: odds.Credits{Remaining: 10}}
	service := New(Deps{
		Store:      store,
		Scoreboard: scoreboard,
		Lines:      lines,
		Now:        clock.Now,
		Logger:     zerolog.Nop(),
	})

	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, int32(1), scoreboard.calls.Load())

	// A second page load inside the window does not fetch again.
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, int32(1), scoreboard.calls.Load())

	// Fifteen minutes later the midweek slate is stale.
	clock.Advance(15 * time.Minute)
	service.KickContext(t.Context())
	service.wait()
	assert.Equal(t, int32(2), scoreboard.calls.Load())
	assert.Zero(t, lines.lineCalls, "Kick() omits odds:lines")
}

func TestOddsRowRejectsATeamOutsideTheGame(t *testing.T) {
	t.Parallel()

	game := storedGame(1, time.Now())

	_, err := oddsRow(game, odds.Line{
		EventID: "evt-1", MarketType: odds.MarketML, Team: "Dallas Cowboys", Side: odds.SideTeam, Price: 120,
	}, time.Now())
	require.Error(t, err)
}

func TestOddsSourceBelowTheFloorStillUsesTheFreeEventList(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	now := novemberTime(7, 11, 0)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)

	// A stored balance below the floor and an old fetch: the source is due, so
	// it refreshes the free event list and reads a fresh balance before it
	// decides whether to spend on lines.
	low := 10
	require.NoError(t, store.RecordSync(t.Context(), OddsLinesSource, now.Add(-48*time.Hour), &low))

	lines := &fakeLines{
		events: []odds.Event{{
			ID: "evt-1", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins",
			CommenceTime: game.Kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: low},
		lineCredits:   odds.Credits{Remaining: low - 3},
	}
	service := New(Deps{
		Store:  store,
		Lines:  lines,
		Now:    func() time.Time { return now },
		Logger: zerolog.Nop(),
	})

	service.runSources(t.Context(), now, []source{service.oddsSource([]db.Game{game}, now)})
	service.wait()

	assert.Zero(t, lines.lineCalls, "the paid lines call must not run below the floor")

	state, err := store.SyncState(t.Context(), OddsLinesSource)
	require.NoError(t, err)
	assert.Equal(t, now, state.FetchedAt)
	require.NotNil(t, state.OddsCreditsRemaining)
	assert.Equal(t, low, *state.OddsCreditsRemaining)
}
