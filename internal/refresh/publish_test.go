package refresh

import (
	"fmt"
	"sync"
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

// recordingPublisher collects the topics a service publishes.
type recordingPublisher struct {
	mu     sync.Mutex
	topics []string
}

// Publish records one topic.
func (p *recordingPublisher) Publish(topic string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topics = append(p.topics, topic)
}

// reset forgets the topics collected so far.
func (p *recordingPublisher) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topics = nil
}

// seen returns a copy of the collected topics.
func (p *recordingPublisher) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.topics...)
}

// publishTest store builds a week with one slip in it.
func publishTestStore(t *testing.T) (*db.Store, db.Week, db.Slip) {
	t.Helper()

	store := openTestStore(t)
	user, err := store.CreateUserWithIdentity(
		t.Context(), "Ada", "ada@example.com", "google-ada", "")
	require.NoError(t, err)
	group, err := store.Create(t.Context(), user.ID, "Sunday Parlay Club")
	require.NoError(t, err)
	week, err := store.UpsertWeek(t.Context(), 2026, 2, 9)
	require.NoError(t, err)
	slip, err := store.CreateSlip(
		t.Context(),
		group.ID,
		week.ID,
		nil,
		"Week 9",
		db.SlateScopeSundayMain,
		1,
		[]int64{user.ID},
	)
	require.NoError(t, err)

	return store, week, slip
}

// weekTopics is the board and slip signals a change in week should send.
func weekTopics(weekID, slipID int64) []string {
	return []string{fmt.Sprintf("week:%d", weekID), fmt.Sprintf("slip:%d", slipID)}
}

func TestPublishWeekSignalsTheBoardAndItsSlips(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	publisher := &recordingPublisher{}
	service := New(Deps{Store: store, Publisher: publisher, Logger: zerolog.Nop()})

	service.publishWeek(t.Context(), week.ID)

	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())
}

func TestChangedScoreboardPublishesOnce(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	publisher := &recordingPublisher{}
	service := New(Deps{
		Store:      store,
		Scoreboard: fakeScoreboard{board: espnWeek()},
		Publisher:  publisher,
		Logger:     zerolog.Nop(),
	})
	now := novemberTime(8, 13, 5)

	changed, err := service.syncScoreboard(t.Context(), now)
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	publisher.reset()
	changed, err = service.syncScoreboard(t.Context(), now.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, changed)
	assert.Empty(t, publisher.seen(), "a rerun that changes no rows publishes nothing")
}

func TestChangedWeatherPublishesTheWeek(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	now := novemberTime(7, 11, 0)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)

	publisher := &recordingPublisher{}
	service := New(Deps{
		Store: store,
		Forecast: fakeForecast{forecast: weather.Forecast{
			TempF: 48.5, WindMPH: 18, WindGustMPH: 29, PrecipIN: 0.08,
		}},
		Publisher: publisher,
		Logger:    zerolog.Nop(),
	})

	changed, err := service.syncWeather(t.Context(), game, now)
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	publisher.reset()
	changed, err = service.syncWeather(t.Context(), game, now.Add(2*time.Hour))
	require.NoError(t, err)
	require.False(t, changed)
	assert.Empty(t, publisher.seen(), "the same forecast publishes nothing")
}

func TestChangedOddsPublishTheWeek(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	now := novemberTime(7, 11, 0)
	game, err := store.UpsertGame(t.Context(), storedGame(week.ID, now))
	require.NoError(t, err)

	lines := &fakeLines{
		events: []odds.Event{{
			ID: "evt-1", HomeTeam: "Buffalo Bills", AwayTeam: "Miami Dolphins",
			CommenceTime: game.Kickoff,
		}},
		eventsCredits: odds.Credits{Remaining: 490},
		lines: []odds.Line{{
			EventID: "evt-1", Book: "draftkings", MarketType: odds.MarketML,
			Team: "Buffalo Bills", Side: odds.SideTeam, Price: -155,
		}},
		lineCredits: odds.Credits{Remaining: 487},
	}
	publisher := &recordingPublisher{}
	service := New(Deps{
		Store: store, Lines: lines, Publisher: publisher, Logger: zerolog.Nop(),
	})

	changed, err := service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	publisher.reset()
	changed, err = service.syncOdds(t.Context(), []db.Game{game}, now)
	require.NoError(t, err)
	require.False(t, changed)
	assert.Empty(t, publisher.seen(), "the same lines and fetch time publish nothing")
}

func TestChangedSummaryPublishesTheWeek(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	now := novemberTime(8, 14, 0)
	game, err := store.UpsertGame(t.Context(), db.Game{
		ID: "401", WeekID: week.ID, HomeTeam: "BUF", AwayTeam: "MIA",
		HomeName: "Buffalo Bills", AwayName: "Miami Dolphins",
		Kickoff: now.Add(-time.Hour), Status: db.GameLive, UpdatedAt: now,
	})
	require.NoError(t, err)

	espnClient := &fakeESPNSummaryClient{
		gameSummaries: map[string]espn.GameSummary{
			"401": {
				InjuriesJSON: `[{"team":"BUF"}]`,
				BoxScore: espn.BoxScore{
					HasStats: true,
					Players: []espn.PlayerBoxStats{{
						PlayerID: "3918298", DisplayName: "Josh Allen", TeamAbbrev: "BUF",
						Passing: espn.PassingStats{PassingYards: 120},
					}},
				},
			},
		},
	}
	publisher := &recordingPublisher{}
	service := New(Deps{
		Store: store, ESPN: espnClient, Publisher: publisher, Logger: zerolog.Nop(),
	})

	changed, err := service.syncSummary(t.Context(), game, now)
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	// Identical rerun publishes nothing.
	publisher.reset()
	changed, err = service.syncSummary(t.Context(), game, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.False(t, changed)
	assert.Empty(t, publisher.seen())

	// Updated live box score (even with identical injuries) publishes again.
	espnClient.gameSummaries["401"] = espn.GameSummary{
		InjuriesJSON: `[{"team":"BUF"}]`,
		BoxScore: espn.BoxScore{
			HasStats: true,
			Players: []espn.PlayerBoxStats{{
				PlayerID: "3918298", DisplayName: "Josh Allen", TeamAbbrev: "BUF",
				Passing: espn.PassingStats{PassingYards: 185},
			}},
		},
	}
	changed, err = service.syncSummary(t.Context(), game, now.Add(4*time.Minute))
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())
}

func TestChangedScoreboardPeriodAndClockPublishTheWeek(t *testing.T) {
	t.Parallel()

	store, week, slip := publishTestStore(t)
	board := espnWeek()
	period := 2
	clock := "4:15"
	board.Games[0].Status = espn.Live
	board.Games[0].Period = &period
	board.Games[0].Clock = &clock

	sb := &fakeScoreboard{board: board}
	publisher := &recordingPublisher{}
	service := New(Deps{
		Store: store, Scoreboard: sb, Publisher: publisher, Logger: zerolog.Nop(),
	})
	now := novemberTime(8, 14, 0)

	changed, err := service.syncScoreboard(t.Context(), now)
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())

	got, err := store.GameByID(t.Context(), board.Games[0].ID)
	require.NoError(t, err)
	require.NotNil(t, got.Period)
	assert.Equal(t, 2, *got.Period)
	require.NotNil(t, got.Clock)
	assert.Equal(t, "4:15", *got.Clock)

	// Changing only the clock publishes again.
	publisher.reset()
	nextClock := "2:00"
	sb.board.Games[0].Clock = &nextClock
	changed, err = service.syncScoreboard(t.Context(), now.Add(5*time.Minute))
	require.NoError(t, err)
	require.True(t, changed)
	assert.ElementsMatch(t, weekTopics(week.ID, slip.ID), publisher.seen())
}
