package db_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// createTestGame inserts the game row a leg's game_id points at.
func createTestGame(t *testing.T, database *sql.DB, week db.Week, id string) string {
	t.Helper()

	_, err := database.ExecContext(t.Context(), `
		INSERT INTO games (
			id, week_id, home_team, away_team, home_name, away_name,
			home_espn_id, away_espn_id, kickoff_utc, status, is_indoor, updated_at
		) VALUES (?, ?, 'BUF', 'MIA', 'Buffalo Bills', 'Miami Dolphins', '2', '15',
			'2026-11-08T18:00:00Z', 'SCHEDULED', 1, '2026-11-07T00:00:00Z')`,
		id, week.ID)
	require.NoError(t, err)
	return id
}

func TestCreateLegStoresEveryField(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)
	gameID := createTestGame(t, database, week, "401772001")

	want := db.Leg{
		SlipID:     slip.ID,
		UserID:     ada.ID,
		GameID:     &gameID,
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 245.5 Pass Yds",
		Team:       new("BUF"),
		PlayerName: new("Josh Allen"),
		PlayerID:   new("3918298"),
		Side:       new("OVER"),
		Line:       new(245.5),
		Odds:       -115,
	}
	leg, err := store.CreateLeg(t.Context(), want)
	require.NoError(t, err)

	assert.Positive(t, leg.ID)
	assert.False(t, leg.CreatedAt.IsZero())
	assert.Equal(t, parlay.Pending, leg.Result)
	assert.Nil(t, leg.GradedBy)

	want.ID = leg.ID
	want.Result = parlay.Pending
	want.CreatedAt = leg.CreatedAt
	assert.Equal(t, want, leg)

	stored, err := store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	assert.Equal(t, leg, stored)
}

func TestCreateLegDefaultsThePriceAndResult(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ada.ID,
		MarketType: "OTHER",
		Selection:  "Bills moneyline",
		Odds:       0,
		Result:     parlay.Won,
	})
	require.NoError(t, err)
	assert.Equal(t, db.DefaultLegOdds, leg.Odds, "an unset price takes the form default")
	assert.Equal(t, parlay.Pending, leg.Result, "a new leg starts pending")
}

func TestCreateLegLeavesOptionalFieldsNil(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     ada.ID,
		MarketType: "OTHER",
		Selection:  "Bills moneyline",
	})
	require.NoError(t, err)

	assert.Nil(t, leg.GameID)
	assert.Nil(t, leg.Team)
	assert.Nil(t, leg.PlayerName)
	assert.Nil(t, leg.PlayerID)
	assert.Nil(t, leg.Side)
	assert.Nil(t, leg.Line)
}

func TestLegsForSlipReturnsThemInAddedOrder(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	first, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, MarketType: "ML", Selection: "BUF ML", Odds: -140,
	})
	require.NoError(t, err)
	second, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, MarketType: "TOTAL", Selection: "BUF@MIA Over 47.5", Odds: -110,
	})
	require.NoError(t, err)

	legs, err := store.LegsForSlip(t.Context(), slip.ID)
	require.NoError(t, err)
	assert.Equal(t, []db.Leg{first, second}, legs)
}

func TestUpdateLegChangesOnlyThePick(t *testing.T) {
	t.Parallel()

	store, database := openTestStore(t)
	ada := createUser(t, store, "Ada")
	bob := createUser(t, store, "Bob")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada, bob)
	gameID := createTestGame(t, database, week, "401772001")

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, MarketType: "ML", Selection: "BUF ML", Odds: -140,
	})
	require.NoError(t, err)
	_, err = database.ExecContext(t.Context(),
		"UPDATE legs SET result = 'WON', graded_by = 'auto' WHERE id = ?", leg.ID)
	require.NoError(t, err)

	updated, err := store.UpdateLeg(t.Context(), db.Leg{
		ID:         leg.ID,
		SlipID:     slip.ID,
		UserID:     bob.ID,
		GameID:     &gameID,
		MarketType: "SPREAD",
		Selection:  "BUF -3.5",
		Team:       new("BUF"),
		Side:       new("TEAM"),
		Line:       new(-3.5),
		Odds:       -105,
		Result:     parlay.Lost,
	})
	require.NoError(t, err)

	assert.Equal(t, ada.ID, updated.UserID, "the owner stays put")
	assert.Equal(t, slip.ID, updated.SlipID)
	assert.Equal(t, "SPREAD", updated.MarketType)
	assert.Equal(t, "BUF -3.5", updated.Selection)
	assert.Equal(t, -105, updated.Odds)
	assert.Equal(t, parlay.Won, updated.Result, "the result and grader stay put")
	require.NotNil(t, updated.GradedBy)
	assert.Equal(t, "auto", *updated.GradedBy)

	_, err = store.UpdateLeg(t.Context(), db.Leg{ID: 404, MarketType: "ML", Selection: "X"})
	require.ErrorIs(t, err, db.ErrLegNotFound)
}

func TestSetLegResult(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, MarketType: "ML", Selection: "BUF ML",
	})
	require.NoError(t, err)

	require.NoError(t, store.SetLegResult(t.Context(), leg.ID, parlay.Won, "Ada"))
	stored, err := store.LegByID(t.Context(), leg.ID)
	require.NoError(t, err)
	assert.Equal(t, parlay.Won, stored.Result)
	require.NotNil(t, stored.GradedBy)
	assert.Equal(t, "Ada", *stored.GradedBy)

	require.ErrorIs(t,
		store.SetLegResult(t.Context(), leg.ID, parlay.Result("MAYBE"), "Ada"),
		db.ErrLegResultInvalid)
	require.ErrorIs(t,
		store.SetLegResult(t.Context(), 404, parlay.Won, "Ada"), db.ErrLegNotFound)
}

func TestDeleteLeg(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")
	group := createTestGroup(t, store, ada)
	week := createTestWeek(t, store, 9)
	slip := createTestSlip(t, store, group, week, "Week 9", 2, ada)

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID: slip.ID, UserID: ada.ID, MarketType: "ML", Selection: "BUF ML",
	})
	require.NoError(t, err)

	require.NoError(t, store.DeleteLeg(t.Context(), leg.ID))
	_, err = store.LegByID(t.Context(), leg.ID)
	require.ErrorIs(t, err, db.ErrLegNotFound)

	require.ErrorIs(t, store.DeleteLeg(t.Context(), leg.ID), db.ErrLegNotFound)
}
