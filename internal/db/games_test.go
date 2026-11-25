package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

// testSlateGame builds the game the slate sync would write for one ESPN event.
func testSlateGame(week db.Week) db.Game {
	return db.Game{
		ID:         "401772001",
		WeekID:     week.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		HomeLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/buf.png"),
		AwayLogo:   new("https://a.espncdn.com/i/teamlogos/nfl/500/mia.png"),
		Kickoff:    time.Date(2026, time.November, 8, 18, 0, 0, 0, time.UTC),
		Status:     db.GameScheduled,
		VenueID:    new("3915"),
		VenueName:  new("Highmark Stadium"),
		UpdatedAt:  time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC),
	}
}

func TestUpsertGameCreatesTheRow(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	week := createTestWeek(t, store, 9)
	want := testSlateGame(week)

	game, err := store.UpsertGame(t.Context(), want)
	require.NoError(t, err)
	assert.Equal(t, want, game)

	stored, err := store.GameByID(t.Context(), want.ID)
	require.NoError(t, err)
	assert.Equal(t, want, stored)

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM games").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestUpsertGameRerunChangesNothing(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game := testSlateGame(week)

	_, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)

	// Later syncs own these columns; the slate sync must not touch them.
	_, err = database.ExecContext(t.Context(), `
		UPDATE games SET
			odds_event_id = 'evt-1',
			temp_f = 55.5,
			wind_mph = 12,
			wind_gust_mph = 22,
			precip_in = 0.1,
			injuries_json = '[]',
			boxscore_json = '{"hasStats":true}'
		WHERE id = ?`, game.ID)
	require.NoError(t, err)

	before, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)

	// A second fetch of the same slate arrives later, but nothing changed.
	game.UpdatedAt = game.UpdatedAt.Add(time.Hour)
	again, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, before, again, "an unchanged sync leaves every column alone")

	var count int
	require.NoError(t, database.QueryRowContext(t.Context(), "SELECT count(*) FROM games").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestUpsertGameUpdatesOnlyTheSlateFields(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game := testSlateGame(week)

	_, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(), `
		UPDATE games SET
			odds_event_id = 'evt-1',
			temp_f = 55.5,
			wind_mph = 12,
			wind_gust_mph = 22,
			precip_in = 0.1,
			injuries_json = '[]',
			boxscore_json = '{"hasStats":true}'
		WHERE id = ?`, game.ID)
	require.NoError(t, err)

	game.Status = db.GameLive
	game.HomeScore = new(14)
	game.AwayScore = new(7)
	game.Period = new(2)
	game.Clock = new("4:15")
	game.Kickoff = game.Kickoff.Add(10 * time.Minute)
	game.UpdatedAt = game.UpdatedAt.Add(2 * time.Hour)

	updated, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, db.GameLive, updated.Status)
	assert.Equal(t, new(14), updated.HomeScore)
	assert.Equal(t, new(7), updated.AwayScore)
	assert.Equal(t, new(2), updated.Period)
	assert.Equal(t, new("4:15"), updated.Clock)
	assert.Equal(t, game.Kickoff, updated.Kickoff)
	assert.Equal(t, game.UpdatedAt, updated.UpdatedAt)
	assert.Equal(t, new("evt-1"), updated.OddsEventID)
	assert.Equal(t, new(55.5), updated.TempF)
	assert.Equal(t, new(12.0), updated.WindMPH)
	assert.Equal(t, new(22.0), updated.WindGustMPH)
	assert.Equal(t, new(0.1), updated.PrecipIN)
	assert.Equal(t, new("[]"), updated.InjuriesJSON)
	assert.Equal(t, new(`{"hasStats":true}`), updated.BoxScoreJSON)

	// Repeating the changed game with a later fetch time is still a no-op.
	game.UpdatedAt = game.UpdatedAt.Add(time.Hour)
	again, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, updated, again)

	// Changing only period/clock updates the row and updated_at, and clearing them on FINAL works.
	game.Clock = new("Halftime")
	game.UpdatedAt = game.UpdatedAt.Add(time.Hour)
	halftime, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Equal(t, new("Halftime"), halftime.Clock)
	assert.Equal(t, game.UpdatedAt, halftime.UpdatedAt)

	game.Status = db.GameFinal
	game.Period = nil
	game.Clock = nil
	game.UpdatedAt = game.UpdatedAt.Add(time.Hour)
	finalGame, err := store.UpsertGame(t.Context(), game)
	require.NoError(t, err)
	assert.Nil(t, finalGame.Period)
	assert.Nil(t, finalGame.Clock)
	assert.Equal(t, game.UpdatedAt, finalGame.UpdatedAt)
}

func TestUpsertGameRefusesAnUnknownStatus(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game := testSlateGame(week)
	game.Status = db.GameStatus("POSTPONED")

	_, err := store.UpsertGame(t.Context(), game)
	require.ErrorIs(t, err, db.ErrGameStatusInvalid)
}

func TestGameByIDReportsAMissingGame(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.GameByID(t.Context(), "401772001")
	require.ErrorIs(t, err, db.ErrGameNotFound)
}

func TestTeamLogos(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week8 := createTestWeek(t, store, 8)
	week9 := createTestWeek(t, store, 9)

	empty, err := store.TeamLogos(t.Context())
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Earlier game has an older logo for BUF and no logo for KC.
	_, err = store.UpsertGame(t.Context(), db.Game{
		ID:         "401772000",
		WeekID:     week8.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "KC",
		HomeName:   "Buffalo Bills",
		AwayName:   "Kansas City Chiefs",
		HomeESPNID: "2",
		AwayESPNID: "12",
		HomeLogo:   new("https://a.espncdn.com/old-buf.png"),
		AwayLogo:   new(""),
		Kickoff:    time.Date(2026, time.November, 1, 18, 0, 0, 0, time.UTC),
		Status:     db.GameFinal,
		UpdatedAt:  time.Date(2026, time.November, 1, 22, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	// Later game updates BUF's logo and provides MIA's logo.
	_, err = store.UpsertGame(t.Context(), testSlateGame(week9))
	require.NoError(t, err)

	logos, err := store.TeamLogos(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"BUF": "https://a.espncdn.com/i/teamlogos/nfl/500/buf.png",
		"MIA": "https://a.espncdn.com/i/teamlogos/nfl/500/mia.png",
	}, logos)
}

func TestSetGameSummaryReportsRealChanges(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)

	// Scheduled game: injuries only (nil boxscoreJSON).
	changed, err := store.SetGameSummary(t.Context(), game.ID, `[{"athlete":"Player 1"}]`, nil)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err := store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(`[{"athlete":"Player 1"}]`), stored.InjuriesJSON)
	assert.Nil(t, stored.BoxScoreJSON)

	// Repeating the same injuries with nil boxscore is a no-op.
	changed, err = store.SetGameSummary(t.Context(), game.ID, `[{"athlete":"Player 1"}]`, nil)
	require.NoError(t, err)
	assert.False(t, changed)

	// Adding live box score JSON reports changed.
	changed, err = store.SetGameSummary(
		t.Context(),
		game.ID,
		`[{"athlete":"Player 1"}]`,
		new(`{"hasStats":true}`),
	)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err = store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(`{"hasStats":true}`), stored.BoxScoreJSON)

	// Repeating identical injuries and box score is a no-op.
	changed, err = store.SetGameSummary(
		t.Context(),
		game.ID,
		`[{"athlete":"Player 1"}]`,
		new(`{"hasStats":true}`),
	)
	require.NoError(t, err)
	assert.False(t, changed)

	// Passing nil boxscore preserves existing boxscore_json and reports no-op when injuries match.
	changed, err = store.SetGameSummary(t.Context(), game.ID, `[{"athlete":"Player 1"}]`, nil)
	require.NoError(t, err)
	assert.False(t, changed)

	stored, err = store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(`{"hasStats":true}`), stored.BoxScoreJSON)

	// Updating only box score reports changed.
	changed, err = store.SetGameSummary(
		t.Context(),
		game.ID,
		`[{"athlete":"Player 1"}]`,
		new(`{"hasStats":true,"homeScore":14}`),
	)
	require.NoError(t, err)
	assert.True(t, changed)

	// Updating only injuries while passing nil box score reports changed and preserves box score.
	changed, err = store.SetGameSummary(t.Context(), game.ID, `[{"athlete":"Player 2"}]`, nil)
	require.NoError(t, err)
	assert.True(t, changed)

	stored, err = store.GameByID(t.Context(), game.ID)
	require.NoError(t, err)
	assert.Equal(t, new(`[{"athlete":"Player 2"}]`), stored.InjuriesJSON)
	assert.Equal(t, new(`{"hasStats":true,"homeScore":14}`), stored.BoxScoreJSON)
}

func TestSetGameSummaryReportsAMissingGame(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.SetGameSummary(t.Context(), "missing", `[]`, nil)
	require.ErrorIs(t, err, db.ErrGameNotFound)
}

func TestLiveGameIDsWithSlipLegs(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week8 := createTestWeek(t, store, 8)
	week9 := createTestWeek(t, store, 9)
	slip9 := createTestSlip(t, store, group, week9, "Week 9", 2, ada)
	slip8 := createTestSlip(t, store, group, week8, "Week 8", 2, ada)

	liveWithLeg := testSlateGame(week9)
	liveWithLeg.ID = "401772001"
	liveWithLeg.Status = db.GameLive
	_, err := store.UpsertGame(t.Context(), liveWithLeg)
	require.NoError(t, err)

	liveWithoutLeg := testSlateGame(week9)
	liveWithoutLeg.ID = "401772002"
	liveWithoutLeg.Status = db.GameLive
	_, err = store.UpsertGame(t.Context(), liveWithoutLeg)
	require.NoError(t, err)

	scheduledWithLeg := testSlateGame(week9)
	scheduledWithLeg.ID = "401772003"
	scheduledWithLeg.Status = db.GameScheduled
	_, err = store.UpsertGame(t.Context(), scheduledWithLeg)
	require.NoError(t, err)

	finalWithLeg := testSlateGame(week9)
	finalWithLeg.ID = "401772004"
	finalWithLeg.Status = db.GameFinal
	_, err = store.UpsertGame(t.Context(), finalWithLeg)
	require.NoError(t, err)

	otherWeekLive := testSlateGame(week8)
	otherWeekLive.ID = "401772005"
	otherWeekLive.Status = db.GameLive
	_, err = store.UpsertGame(t.Context(), otherWeekLive)
	require.NoError(t, err)

	for _, tc := range []struct {
		slipID int64
		gameID string
	}{
		{slip9.ID, liveWithLeg.ID},
		{slip9.ID, liveWithLeg.ID}, // duplicate leg on same game
		{slip9.ID, scheduledWithLeg.ID},
		{slip9.ID, finalWithLeg.ID},
		{slip8.ID, otherWeekLive.ID},
	} {
		_, err := store.CreateLeg(t.Context(), db.Leg{
			SlipID:     tc.slipID,
			UserID:     ada.ID,
			GameID:     new(tc.gameID),
			MarketType: "ML",
			Selection:  "BUF ML",
			Team:       new("BUF"),
			Side:       new("TEAM"),
			Odds:       -110,
		})
		require.NoError(t, err)
	}

	got, err := store.LiveGameIDsWithSlipLegs(t.Context(), week9.ID)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{liveWithLeg.ID: true}, got)
}
