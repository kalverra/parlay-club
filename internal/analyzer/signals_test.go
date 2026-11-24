package analyzer_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
)

func TestPostseasonNFLverseMergesRegularAndPostseason(t *testing.T) {
	t.Parallel()

	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	now := time.Date(2027, 1, 16, 12, 0, 0, 0, time.UTC)
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now, nil))

	// Seed regular-season rows for all 32 teams, plus 1 postseason week for 2 teams.
	teamRows := make([]db.NFLverseTeamWeek, 0, 34)
	for i := range 32 {
		team := fmt.Sprintf("T%02d", i+1)
		teamRows = append(teamRows, db.NFLverseTeamWeek{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypeRegular,
			WeekNum:           18,
			Team:              team,
			OffPassEPA:        float64(i) * 0.01,
			DefPassEPAAllowed: float64(i) * 0.01,
			PlaysPerGame:      60 + float64(i)*0.2,
			PassRate:          0.50 + float64(i)*0.005,
			UpdatedAt:         now,
		})
	}
	teamRows = append(teamRows,
		db.NFLverseTeamWeek{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypePostseason,
			WeekNum:           1,
			Team:              "T01",
			DefPassEPAAllowed: -0.05,
			UpdatedAt:         now,
		},
		db.NFLverseTeamWeek{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypePostseason,
			WeekNum:           1,
			Team:              "T32",
			DefPassEPAAllowed: 0.40,
			UpdatedAt:         now,
		},
	)
	_, err = store.UpsertNFLverseTeamWeeks(t.Context(), teamRows)
	require.NoError(t, err)

	postWeek := db.Week{
		ID:         db.WeekID(2026, nflverse.SeasonTypePostseason, 2),
		Season:     2026,
		SeasonType: nflverse.SeasonTypePostseason,
		WeekNum:    2,
	}
	profiles, fresh := analyzer.LoadFreshTeamProfiles(t.Context(), store, now, postWeek)
	require.True(t, fresh)
	require.Len(t, profiles, 32)

	seenRanks := make(map[int]bool, 32)
	for _, prof := range profiles {
		seenRanks[prof.DefPassEPAAllowedRank] = true
	}
	for r := 1; r <= 32; r++ {
		assert.True(t, seenRanks[r], "expected rank %d to be present across 32 teams", r)
	}

	// Player usage in postseason merges regular-season and postseason weeks,
	// with postseason weeks ordered first for L3/L2 recency.
	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 16, Team: "T01", OffensePct: 0.80, TargetShare: 0.20, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 17, Team: "T01", OffensePct: 0.80, TargetShare: 0.20, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 18, Team: "T01", OffensePct: 0.80, TargetShare: 0.20, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypePostseason,
			WeekNum: 1, Team: "T01", OffensePct: 0.95, TargetShare: 0.35, UpdatedAt: now,
		},
	})
	require.NoError(t, err)

	usage := analyzer.LoadPlayerUsageForIDs(t.Context(), store, []string{"p1"}, 2026, nflverse.SeasonTypePostseason)
	require.NotNil(t, usage["p1"])
	require.Len(t, usage["p1"].Weeks, 4)
	assert.InDelta(t, 0.95, usage["p1"].Weeks[0].OffensePct, 1e-9, "postseason week should be most recent")
	assert.InDelta(t, 0.85, usage["p1"].Last3SnapAvg, 1e-9)
}

func TestBuildTeamDefenseProfiles(t *testing.T) {
	t.Parallel()

	assert.Nil(t, analyzer.BuildTeamDefenseProfiles(nil))

	rows := []db.NFLverseTeamWeek{
		{
			Team:              "BUF",
			WeekNum:           1,
			OffPassEPA:        0.10,
			DefPassEPAAllowed: 0.20,
			DefRushEPAAllowed: -0.10,
			PlaysPerGame:      60,
			PassRate:          0.55,
		},
		{
			Team:              "BUF",
			WeekNum:           2,
			OffPassEPA:        0.30,
			DefPassEPAAllowed: 0.00,
			DefRushEPAAllowed: -0.10,
			PlaysPerGame:      64,
			PassRate:          0.65,
		},
		{
			Team:              "MIA",
			WeekNum:           1,
			OffPassEPA:        0.05,
			DefPassEPAAllowed: 0.30,
			DefRushEPAAllowed: 0.05,
			PlaysPerGame:      58,
			PassRate:          0.50,
		},
	}
	profiles := analyzer.BuildTeamDefenseProfiles(rows)
	require.Len(t, profiles, 2)

	buf := profiles["BUF"]
	require.NotNil(t, buf)
	assert.InDelta(t, 0.20, buf.OffPassEPA, 1e-9)
	assert.InDelta(t, 0.10, buf.DefPassEPAAllowed, 1e-9)
	assert.InDelta(t, 0.00, buf.DefOverallEPAAllowed, 1e-9)
	assert.Equal(t, 1, buf.OffPassEPARank)
	assert.Equal(t, 1, buf.DefPassEPAAllowedRank)
	assert.Equal(t, 1, buf.DefOverallEPARank)
	assert.InDelta(t, 0.35, profiles["MIA"].DefOverallEPAAllowed, 1e-9)
	assert.Equal(t, 2, profiles["MIA"].OffPassEPARank)
	assert.Equal(t, 2, profiles["MIA"].DefPassEPAAllowedRank)
	assert.Equal(t, 2, profiles["MIA"].DefOverallEPARank)
}

func TestBuildPlayerUsageAndSummarizeRecentUsage(t *testing.T) {
	t.Parallel()

	_, _, _, ok := analyzer.SummarizeRecentUsage(nil)
	assert.False(t, ok)

	twoWeeks := []db.NFLversePlayerWeek{
		{WeekNum: 2, OffensePct: 0.90, TargetShare: 0.30, WOPR: 0.60, Carries: 4},
		{WeekNum: 1, OffensePct: 0.70, TargetShare: 0.20, WOPR: 0.40, Carries: 2},
	}
	assert.Nil(t, analyzer.BuildPlayerUsage(twoWeeks))

	snap, target, wopr, ok := analyzer.SummarizeRecentUsage(twoWeeks)
	require.True(t, ok)
	assert.InDelta(t, 0.80, snap, 1e-9)
	assert.InDelta(t, 0.25, target, 1e-9)
	assert.InDelta(t, 0.50, wopr, 1e-9)

	fourWeeks := []db.NFLversePlayerWeek{
		{WeekNum: 4, OffensePct: 0.90, TargetShare: 0.30, WOPR: 0.60, Carries: 10},
		{WeekNum: 3, OffensePct: 0.80, TargetShare: 0.24, WOPR: 0.50, Carries: 12},
		{WeekNum: 2, OffensePct: 0.70, TargetShare: 0.21, WOPR: 0.40, Carries: 14},
		{WeekNum: 1, OffensePct: 0.50, TargetShare: 0.05, WOPR: 0.10, Carries: 4},
	}
	usage := analyzer.BuildPlayerUsage(fourWeeks)
	require.NotNil(t, usage)
	assert.InDelta(t, 0.80, usage.Last3SnapAvg, 1e-9)
	assert.InDelta(t, 0.725, usage.SeasonSnapAvg, 1e-9)
	assert.InDelta(t, 0.25, usage.Last3TargetShare, 1e-9)
	assert.InDelta(t, 0.20, usage.SeasonTargetShare, 1e-9)
	assert.InDelta(t, 0.50, usage.Last3WOPR, 1e-9)
	assert.InDelta(t, 10.0, usage.AvgCarries, 1e-9)
}

func TestGameFromDBAndLegFromDB(t *testing.T) {
	t.Parallel()

	wind := 18.0
	gust := 25.0
	precip := 0.2
	temp := 34.0
	teamEPA := map[string]*analyzer.TeamDefenseProfile{
		"BUF": {OffPassEPARank: 2},
		"MIA": {DefPassEPAAllowedRank: 28},
	}
	proj := &db.NfeloGame{GameID: "g1", HomeSpread: -4.5, HomeWinProb: 0.65}

	scheduled := analyzer.GameFromDB(db.Game{
		ID:          "g1",
		HomeTeam:    "BUF",
		AwayTeam:    "MIA",
		Status:      db.GameScheduled,
		WindMPH:     &wind,
		WindGustMPH: &gust,
		PrecipIN:    &precip,
		TempF:       &temp,
	}, proj, teamEPA)
	require.NotNil(t, scheduled.Nfelo)
	assert.InDelta(t, -4.5, scheduled.Nfelo.HomeSpread, 1e-9)
	require.NotNil(t, scheduled.Weather)
	assert.InDelta(t, 18.0, scheduled.Weather.WindMPH, 1e-9)
	assert.Equal(t, teamEPA["BUF"], scheduled.HomeEPA)
	assert.Equal(t, teamEPA["MIA"], scheduled.AwayEPA)

	final := analyzer.GameFromDB(db.Game{
		ID:       "g1",
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Status:   db.GameFinal,
	}, proj, teamEPA)
	assert.Nil(t, final.Nfelo)

	gameID := "g1"
	team := "BUF"
	playerID := "p1"
	playerName := "Josh Allen"
	side := "OVER"
	line := 265.5
	leg := analyzer.LegFromDB(db.Leg{
		ID:         7,
		GameID:     &gameID,
		MarketType: "PASS_YDS",
		Team:       &team,
		PlayerID:   &playerID,
		PlayerName: &playerName,
		Side:       &side,
		Line:       &line,
		Odds:       -110,
	}, "Adam")
	assert.Equal(t, analyzer.Leg{
		ID:         7,
		Owner:      "Adam",
		GameID:     "g1",
		Market:     "PASS_YDS",
		Team:       "BUF",
		PlayerID:   "p1",
		PlayerName: "Josh Allen",
		Side:       "OVER",
		Line:       &line,
		Odds:       -110,
	}, leg)
}

func openSignalTestStore(t *testing.T) *db.Store {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "parlay.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	return store
}

func TestLoadSlipAndGameSignalsAndStaleness(t *testing.T) {
	t.Parallel()

	store := openSignalTestStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	week, err := store.UpsertWeek(t.Context(), 2026, nflverse.SeasonTypeRegular, 6)
	require.NoError(t, err)
	game, err := store.UpsertGame(t.Context(), db.Game{
		ID:       "g1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)

	_, err = store.UpsertNfeloGames(t.Context(), []db.NfeloGame{
		{GameID: "g1", HomeSpread: -5.5, HomeWinProb: 0.68, UpdatedAt: now},
	})
	require.NoError(t, err)
	_, err = store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{
		{
			Season:     2026,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    5,
			Team:       "BUF",
			OffPassEPA: 0.20,
			UpdatedAt:  now,
		},
		{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypeRegular,
			WeekNum:           5,
			Team:              "MIA",
			DefPassEPAAllowed: 0.25,
			UpdatedAt:         now,
		},
	})
	require.NoError(t, err)
	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID:      "p1",
			Season:      2026,
			SeasonType:  nflverse.SeasonTypeRegular,
			WeekNum:     5,
			Team:        "BUF",
			OffensePct:  0.9,
			TargetShare: 0.28,
			WOPR:        0.62,
			Carries:     1,
			UpdatedAt:   now,
		},
		{
			ESPNID:      "p1",
			Season:      2026,
			SeasonType:  nflverse.SeasonTypeRegular,
			WeekNum:     4,
			Team:        "BUF",
			OffensePct:  0.85,
			TargetShare: 0.24,
			WOPR:        0.55,
			Carries:     0,
			UpdatedAt:   now,
		},
		{
			ESPNID:      "p1",
			Season:      2026,
			SeasonType:  nflverse.SeasonTypeRegular,
			WeekNum:     3,
			Team:        "BUF",
			OffensePct:  0.80,
			TargetShare: 0.22,
			WOPR:        0.51,
			Carries:     2,
			UpdatedAt:   now,
		},
	})
	require.NoError(t, err)
	require.NoError(t, store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "p1",
		GamesPlayed: 3,
		StatsJSON:   `{"REC_YDS":[82,74,61]}`,
		UpdatedAt:   now,
	}))

	// Fresh nfelo (<= 72h) and fresh nflverse (<= 7d).
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NfeloSource, now.Add(-12*time.Hour), nil))
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-24*time.Hour), nil))

	playerID := "p1"
	gameID := "g1"
	legs := []db.Leg{{ID: 1, GameID: &gameID, PlayerID: &playerID, MarketType: "REC_YDS"}}

	gamesByID, stats := analyzer.LoadSlipSignals(t.Context(), store, now, week.ID, []db.Game{game}, legs)
	require.NotNil(t, gamesByID["g1"].Nfelo)
	assert.InDelta(t, -5.5, gamesByID["g1"].Nfelo.HomeSpread, 1e-9)
	assert.NotNil(t, gamesByID["g1"].HomeEPA)
	assert.NotNil(t, gamesByID["g1"].AwayEPA)
	require.Contains(t, stats, "p1")
	assert.Equal(t, []float64{82, 74, 61}, stats["p1"].Games["REC_YDS"])
	require.NotNil(t, stats["p1"].Usage)
	assert.InDelta(t, 0.56, stats["p1"].Usage.Last3WOPR, 1e-9)

	teamEPA, playerUsage := analyzer.LoadGameSignals(t.Context(), store, now, week.ID, []string{"p1"})
	require.Len(t, teamEPA, 2)
	require.NotNil(t, playerUsage["p1"])

	// Stale nfelo (> 72h): drops Game.Nfelo while keeping nflverse and gamelogs.
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NfeloSource, now.Add(-73*time.Hour), nil))
	gamesByID, stats = analyzer.LoadSlipSignals(t.Context(), store, now, week.ID, []db.Game{game}, legs)
	assert.Nil(t, gamesByID["g1"].Nfelo)
	assert.NotNil(t, gamesByID["g1"].HomeEPA)
	assert.NotNil(t, stats["p1"].Usage)

	// Stale nflverse (> 7d): drops EPA and usage while keeping gamelogs.
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-8*24*time.Hour), nil))
	gamesByID, stats = analyzer.LoadSlipSignals(t.Context(), store, now, week.ID, []db.Game{game}, legs)
	assert.Nil(t, gamesByID["g1"].HomeEPA)
	assert.Nil(t, stats["p1"].Usage)
	assert.Equal(t, []float64{82, 74, 61}, stats["p1"].Games["REC_YDS"])

	teamEPA, playerUsage = analyzer.LoadGameSignals(t.Context(), store, now, week.ID, []string{"p1"})
	assert.Nil(t, teamEPA)
	assert.Nil(t, playerUsage)
}

func TestLoadPlayerResearchSignals(t *testing.T) {
	t.Parallel()

	store := openSignalTestStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	week, err := store.UpsertWeek(t.Context(), 2026, nflverse.SeasonTypeRegular, 6)
	require.NoError(t, err)
	_, err = store.UpsertGame(t.Context(), db.Game{
		ID:       "g1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)
	_, err = store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{
		{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypeRegular,
			WeekNum:           5,
			Team:              "BUF",
			DefPassEPAAllowed: -0.10,
			DefRushEPAAllowed: -0.05,
			UpdatedAt:         now,
		},
		{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypeRegular,
			WeekNum:           5,
			Team:              "MIA",
			DefPassEPAAllowed: 0.25,
			DefRushEPAAllowed: 0.15,
			UpdatedAt:         now,
		},
	})
	require.NoError(t, err)
	// Only 2 weeks exist; research still summarizes up to 3 weeks.
	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID:        "rb1",
			Season:        2026,
			SeasonType:    nflverse.SeasonTypeRegular,
			WeekNum:       5,
			Team:          "BUF",
			Opponent:      "NE",
			PlayerName:    "James Cook",
			Position:      "FB", // normalized to RB on write
			OffensePct:    0.70,
			TargetShare:   0.12,
			AirYardsShare: 0.06,
			WOPR:          0.30,
			Carries:       18,
			RushYds:       96,
			Targets:       4,
			RZOpps:        3,
			TotalEPA:      4.2,
			UpdatedAt:     now,
		},
		{
			ESPNID:        "rb1",
			Season:        2026,
			SeasonType:    nflverse.SeasonTypeRegular,
			WeekNum:       4,
			Team:          "BUF",
			Opponent:      "NYJ",
			PlayerName:    "James Cook",
			Position:      "RB",
			OffensePct:    0.60,
			TargetShare:   0.10,
			AirYardsShare: 0.04,
			WOPR:          0.20,
			Carries:       12,
			RushYds:       64,
			Targets:       2,
			RZOpps:        1,
			TotalEPA:      1.8,
			UpdatedAt:     now,
		},
		{
			ESPNID:     "rb-opp-1",
			Season:     2026,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    5,
			Team:       "NYJ",
			Opponent:   "MIA",
			PlayerName: "Breece Hall",
			Position:   "RB",
			OffensePct: 0.80,
			RushYds:    110,
			UpdatedAt:  now,
		},
		{
			ESPNID:     "rb-opp-2",
			Season:     2026,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    4,
			Team:       "NE",
			Opponent:   "MIA",
			PlayerName: "Rhamondre Stevenson",
			Position:   "RB",
			OffensePct: 0.72,
			RushYds:    84,
			UpdatedAt:  now,
		},
	})
	require.NoError(t, err)
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-24*time.Hour), nil))

	sig := analyzer.LoadPlayerResearchSignals(t.Context(), store, now, "rb1", "BUF", "FB", "")
	require.True(t, sig.HasUsage)
	assert.InDelta(t, 0.65, sig.SnapPct, 1e-9)
	assert.InDelta(t, 0.11, sig.TargetShare, 1e-9)
	assert.InDelta(t, 0.25, sig.WOPR, 1e-9)
	assert.InDelta(t, 15.0, sig.CarriesPerGame, 1e-9)
	assert.InDelta(t, 3.0, sig.TargetsPerGame, 1e-9)
	assert.InDelta(t, 0.05, sig.AirYardsShare, 1e-9)
	assert.InDelta(t, 2.0, sig.RZOppsPerGame, 1e-9)
	assert.InDelta(t, 3.0, sig.EPAPerGame, 1e-9)
	assert.Equal(t, "MIA", sig.Opponent)
	assert.Equal(t, "rush", sig.OppDefSplit)
	assert.Equal(t, 2, sig.OppDefRank)
	assert.Equal(t, 2, sig.OppOverallDefRank)
	require.NotNil(t, sig.TeamDefProfiles)
	assert.Equal(t, 1, sig.TeamDefProfiles["BUF"].DefOverallEPARank)
	require.NotNil(t, sig.PosDefRanks)
	assert.Equal(t, 3, sig.OppPosDefRank) // NYJ (64), NE (96), MIA (97 avg)
	require.Len(t, sig.OppDefenseL5, 2)
	assert.Equal(t, 4, sig.OppDefenseL5[0].WeekNum)
	assert.Equal(t, "Rhamondre Stevenson", sig.OppDefenseL5[0].PlayerName)
	assert.Equal(t, 5, sig.OppDefenseL5[1].WeekNum)
	assert.Equal(t, "Breece Hall", sig.OppDefenseL5[1].PlayerName)

	// Stale nflverse returns zeroed signals.
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-8*24*time.Hour), nil))
	stale := analyzer.LoadPlayerResearchSignals(t.Context(), store, now, "rb1", "BUF", "RB", "REC_YDS")
	assert.False(t, stale.HasUsage)
	assert.Zero(t, stale.OppDefRank)
	assert.Zero(t, stale.OppOverallDefRank)
	assert.Zero(t, stale.OppPosDefRank)
	assert.Nil(t, stale.TeamDefProfiles)
	assert.Nil(t, stale.PosDefRanks)
	assert.Nil(t, stale.OppDefenseL5)
}

func TestBuildPositionDefenseRanks(t *testing.T) {
	t.Parallel()

	assert.Nil(t, analyzer.BuildPositionDefenseRanks(nil, "RB", "RUSH_YDS"))
	assert.Nil(
		t,
		analyzer.BuildPositionDefenseRanks([]db.NFLversePlayerWeek{{Opponent: "BUF", Position: "RB"}}, "", ""),
	)

	rows := []db.NFLversePlayerWeek{
		// vs BUF: week 1 (60+40=100), week 2 (80) -> avg 90.0
		{Opponent: "BUF", Position: "RB", SeasonType: 2, WeekNum: 1, RushYds: 60},
		{Opponent: "BUF", Position: "FB", SeasonType: 2, WeekNum: 1, RushYds: 40}, // FB normalizes to RB
		{Opponent: "BUF", Position: "RB", SeasonType: 2, WeekNum: 2, RushYds: 80},
		// vs MIA: week 1 (90) -> avg 90.0 (tie with BUF; BUF < MIA alphabetically)
		{Opponent: "MIA", Position: "RB", SeasonType: 2, WeekNum: 1, RushYds: 90},
		// vs NYJ: week 1 (50) -> avg 50.0 (toughest, rank 1)
		{Opponent: "NYJ", Position: "HB", SeasonType: 2, WeekNum: 1, RushYds: 50},
		// WR row should be ignored for RB ranks
		{Opponent: "NYJ", Position: "WR", SeasonType: 2, WeekNum: 1, RushYds: 200, RecYds: 120},
	}

	ranks := analyzer.BuildPositionDefenseRanks(rows, "FB", "") // empty market falls back to RUSH_YDS
	require.Len(t, ranks, 3)
	assert.Equal(t, 1, ranks["NYJ"])
	assert.Equal(t, 2, ranks["BUF"])
	assert.Equal(t, 3, ranks["MIA"])

	wrRanks := analyzer.BuildPositionDefenseRanks(rows, "WR", "")
	require.Len(t, wrRanks, 1)
	assert.Equal(t, 1, wrRanks["NYJ"])
}

func TestBuildOppDefenseLast5(t *testing.T) {
	t.Parallel()

	assert.Nil(t, analyzer.BuildOppDefenseLast5(nil, "REC_YDS"))

	// <5 games early season, including a zero-value game and tie-break by usage then name.
	early := []db.NFLversePlayerWeek{
		{SeasonType: 2, WeekNum: 3, PlayerName: "Zero Guy", Team: "NE", Position: "WR", OffensePct: 0.40, RecYds: 0},
		{SeasonType: 2, WeekNum: 3, PlayerName: "Snap Leader", Team: "NE", Position: "WR", OffensePct: 0.85, RecYds: 0},
		{SeasonType: 2, WeekNum: 1, PlayerName: "B. Alpha", Team: "BUF", Position: "WR", OffensePct: 0.80, RecYds: 80},
		{SeasonType: 2, WeekNum: 1, PlayerName: "A. Alpha", Team: "BUF", Position: "WR", OffensePct: 0.80, RecYds: 80},
	}
	earlyL5 := analyzer.BuildOppDefenseLast5(early, "REC_YDS")
	require.Len(t, earlyL5, 2)
	assert.Equal(t, 1, earlyL5[0].WeekNum)
	assert.Equal(t, "A. Alpha", earlyL5[0].PlayerName)
	assert.InDelta(t, 80, earlyL5[0].Value, 1e-9)
	assert.Equal(t, 3, earlyL5[1].WeekNum)
	assert.Equal(t, "Snap Leader", earlyL5[1].PlayerName)
	assert.InDelta(t, 0, earlyL5[1].Value, 1e-9)

	// >5 games including postseason: keeps 5 most recent in chronological order.
	many := []db.NFLversePlayerWeek{
		{SeasonType: 2, WeekNum: 14, PlayerName: "W14", Team: "T1", Position: "WR", RecYds: 50},
		{SeasonType: 2, WeekNum: 15, PlayerName: "W15", Team: "T2", Position: "WR", RecYds: 60},
		{SeasonType: 2, WeekNum: 16, PlayerName: "W16", Team: "T3", Position: "WR", RecYds: 70},
		{SeasonType: 2, WeekNum: 17, PlayerName: "W17", Team: "T4", Position: "WR", RecYds: 80},
		{SeasonType: 2, WeekNum: 18, PlayerName: "W18", Team: "T5", Position: "WR", RecYds: 90},
		{SeasonType: 3, WeekNum: 1, PlayerName: "WC1", Team: "T6", Position: "WR", RecYds: 100},
	}
	l5 := analyzer.BuildOppDefenseLast5(many, "")
	require.Len(t, l5, 5)
	assert.Equal(
		t,
		[]int{15, 16, 17, 18, 1},
		[]int{l5[0].WeekNum, l5[1].WeekNum, l5[2].WeekNum, l5[3].WeekNum, l5[4].WeekNum},
	)
	assert.Equal(t, 3, l5[4].SeasonType)
	assert.Equal(t, "WC1", l5[4].PlayerName)
}

func TestPlayerUsageAccessorFallbacks(t *testing.T) {
	t.Parallel()

	fallback := &analyzer.PlayerUsage{
		Weeks: []analyzer.PlayerWeekUsage{
			{OffensePct: 0.90, TargetShare: 0.30, WOPR: 0.60, Carries: 14},
			{OffensePct: 0.70, TargetShare: 0.20, WOPR: 0.40, Carries: 10},
		},
	}
	assert.InDelta(t, 0.25, fallback.EffectiveTargetShare(), 1e-9)
	assert.InDelta(t, 0.80, fallback.EffectiveSnapAvg(), 1e-9)
	assert.InDelta(t, 0.80, fallback.EffectiveSeasonSnapAvg(), 1e-9)
	assert.InDelta(t, 0.50, fallback.RecentWOPR(), 1e-9)
	assert.InDelta(t, 12.0, fallback.AverageCarries(), 1e-9)

	precomputed := &analyzer.PlayerUsage{
		Last3SnapAvg:      0.85,
		SeasonSnapAvg:     0.75,
		Last3TargetShare:  0.27,
		SeasonTargetShare: 0.22,
		Last3WOPR:         0.58,
		AvgCarries:        15.5,
		Weeks:             []analyzer.PlayerWeekUsage{{OffensePct: 0.10, TargetShare: 0.05, WOPR: 0.10, Carries: 1}},
	}
	assert.InDelta(t, 0.27, precomputed.EffectiveTargetShare(), 1e-9)
	assert.InDelta(t, 0.85, precomputed.EffectiveSnapAvg(), 1e-9)
	assert.InDelta(t, 0.75, precomputed.EffectiveSeasonSnapAvg(), 1e-9)
	assert.InDelta(t, 0.58, precomputed.RecentWOPR(), 1e-9)
	assert.InDelta(t, 15.5, precomputed.AverageCarries(), 1e-9)
}

func TestMergeGameInjuries(t *testing.T) {
	t.Parallel()

	assert.Nil(t, analyzer.MergeGameInjuries(db.Game{HomeTeam: "BUF", AwayTeam: "MIA"}, nil))

	summaryJSON := `[{
		"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
		"injuries": [
			{
				"status": "Out",
				"athlete": {"id": "101", "fullName": "Keon Coleman", "position": {"abbreviation": "WR"}},
				"details": {"type": "Wrist", "detail": "Out"}
			},
			{
				"status": "Questionable",
				"athlete": {"id": "102", "fullName": "James Cook", "position": {"abbreviation": "RB"}},
				"details": {"type": "Toe", "detail": "Questionable"}
			},
			{
				"status": "Out",
				"athlete": {"id": "103", "fullName": "Dion Dawkins", "position": {"abbreviation": "LT"}},
				"details": {"type": "Knee", "detail": "Out"}
			},
			{
				"status": "Out",
				"athlete": {"id": "104", "fullName": "Taron Johnson", "position": {"abbreviation": "CB"}},
				"details": {"type": "Forearm", "detail": "Out"}
			},
			{
				"status": "Out",
				"athlete": {"id": "105", "fullName": "Taylor Rapp", "position": {"abbreviation": "S"}},
				"details": {"type": "Concussion", "detail": "Out"}
			}
		]
	}]`

	game := db.Game{
		ID:           "g1",
		HomeTeam:     "BUF",
		AwayTeam:     "MIA",
		InjuriesJSON: &summaryJSON,
	}
	rosters := []db.RosterPlayer{
		// Duplicate of summary player: summary Detail should be preserved.
		{AthleteID: "101", Team: "BUF", FullName: "Keon Coleman", Position: "WR", InjuryStatus: "Out"},
		// 6th key-position Out player truncated from summary: must be appended.
		{AthleteID: "106", Team: "BUF", FullName: "Dalton Kincaid", Position: "TE", InjuryStatus: "Out"},
		// Non-key position: must be filtered out.
		{AthleteID: "107", Team: "BUF", FullName: "Matt Milano", Position: "LB", InjuryStatus: "Out"},
		// Healthy key-position player: must be filtered out.
		{AthleteID: "108", Team: "BUF", FullName: "Dawson Knox", Position: "TE", InjuryStatus: ""},
		// Away team roster-only injury.
		{AthleteID: "201", Team: "MIA", FullName: "Tyreek Hill", Position: "WR", InjuryStatus: "Questionable"},
		// Unrelated team: must be ignored.
		{AthleteID: "301", Team: "KC", FullName: "Travis Kelce", Position: "TE", InjuryStatus: "Out"},
	}

	merged := analyzer.MergeGameInjuries(game, rosters)
	require.Len(t, merged, 2)

	buf := merged["BUF"]
	require.Len(t, buf, 6)
	assert.Equal(t, espn.Injury{
		AthleteID: "101",
		FullName:  "Keon Coleman",
		Position:  "WR",
		Status:    "Out",
		Detail:    "Wrist (Out)",
	}, buf[0])
	assert.Equal(t, espn.Injury{
		AthleteID: "106",
		FullName:  "Dalton Kincaid",
		Position:  "TE",
		Status:    "Out",
	}, buf[5])

	mia := merged["MIA"]
	require.Len(t, mia, 1)
	assert.Equal(t, "Tyreek Hill", mia[0].FullName)
	assert.Equal(t, "Questionable", mia[0].Status)
}

func TestLoadSlipSignalsPopulatesInjuriesAndPositions(t *testing.T) {
	t.Parallel()

	store := openSignalTestStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	week, err := store.UpsertWeek(t.Context(), 2026, nflverse.SeasonTypeRegular, 6)
	require.NoError(t, err)

	summaryJSON := `[{
		"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
		"injuries": [{
			"status": "Out",
			"athlete": {"id": "101", "fullName": "Keon Coleman", "position": {"abbreviation": "WR"}},
			"details": {"type": "Wrist", "detail": "Out"}
		}]
	}]`
	game, err := store.UpsertGame(t.Context(), db.Game{
		ID:       "g1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)
	game.InjuriesJSON = &summaryJSON

	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID: "knox", Team: "BUF", ESPNTeamID: "2",
			FullName: "Dawson Knox", NormName: "dawson knox", Position: "TE",
			UpdatedAt: now,
		},
		{
			AthleteID: "kincaid", Team: "BUF", ESPNTeamID: "2",
			FullName: "Dalton Kincaid", NormName: "dalton kincaid", Position: "TE",
			InjuryStatus: "Out", InjuryDate: "2026-10-14", UpdatedAt: now,
		},
		{
			AthleteID: "waddle", Team: "MIA", ESPNTeamID: "15",
			FullName: "Jaylen Waddle", NormName: "jaylen waddle", Position: "WR",
			UpdatedAt: now,
		},
	}))

	require.NoError(t, store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "knox",
		GamesPlayed: 3,
		StatsJSON:   `{"REC_YDS":[12,7,4]}`,
		UpdatedAt:   now,
	}))

	gameID := "g1"
	knoxID := "knox"
	waddleID := "waddle"
	legs := []db.Leg{
		{ID: 1, GameID: &gameID, PlayerID: &knoxID, MarketType: "REC_YDS"},
		{ID: 2, GameID: &gameID, PlayerID: &waddleID, MarketType: "REC_YDS"},
	}

	gamesByID, stats := analyzer.LoadSlipSignals(t.Context(), store, now, week.ID, []db.Game{game}, legs)

	bufInjuries := gamesByID["g1"].Injuries["BUF"]
	require.Len(t, bufInjuries, 2)
	assert.Equal(t, analyzer.Injury{
		AthleteID: "101",
		FullName:  "Keon Coleman",
		Position:  "WR",
		Status:    "Out",
	}, bufInjuries[0])
	assert.Equal(t, analyzer.Injury{
		AthleteID: "kincaid",
		FullName:  "Dalton Kincaid",
		Position:  "TE",
		Status:    "Out",
	}, bufInjuries[1])

	require.Contains(t, stats, "knox")
	assert.Equal(t, "TE", stats["knox"].Position)
	assert.Equal(t, []float64{12, 7, 4}, stats["knox"].Games["REC_YDS"])

	require.Contains(t, stats, "waddle")
	assert.Equal(t, "WR", stats["waddle"].Position)
}

func TestDistinctLegPlayerIDs(t *testing.T) {
	t.Parallel()

	p1 := "101"
	p2 := "202"
	empty := ""
	legs := []db.Leg{
		{PlayerID: nil},
		{PlayerID: &empty},
		{PlayerID: &p1},
		{PlayerID: &p2},
		{PlayerID: &p1},
	}

	assert.Equal(t, []string{"101", "202"}, analyzer.DistinctLegPlayerIDs(legs))
	assert.Nil(t, analyzer.DistinctLegPlayerIDs(nil))
}

func TestNFLverseSeasonTypes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int{nflverse.SeasonTypeRegular}, analyzer.NFLverseSeasonTypes(nflverse.SeasonTypeRegular))
	assert.Equal(
		t,
		[]int{nflverse.SeasonTypeRegular, nflverse.SeasonTypePostseason},
		analyzer.NFLverseSeasonTypes(nflverse.SeasonTypePostseason),
	)
}

func TestMergeInjuryFeeds(t *testing.T) {
	t.Parallel()

	assert.Nil(t, analyzer.MergeInjuryFeeds(nil, nil))

	summaryByTeam := map[string][]espn.Injury{
		"BUF": {
			{AthleteID: "101", FullName: "Keon Coleman", Position: "WR", Status: "Out", Detail: "Wrist"},
		},
	}
	rosterByTeam := map[string][]espn.Injury{
		"BUF": {
			{AthleteID: "101", FullName: "Keon Coleman", Position: "WR", Status: "Questionable"},
			{AthleteID: "102", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
		},
		"MIA": {
			{AthleteID: "201", FullName: "Tyreek Hill", Position: "WR", Status: "Questionable"},
		},
	}

	merged := analyzer.MergeInjuryFeeds(summaryByTeam, rosterByTeam)
	require.Len(t, merged, 2)
	assert.Equal(t, []analyzer.Injury{
		{AthleteID: "101", FullName: "Keon Coleman", Position: "WR", Status: "Out"},
		{AthleteID: "102", FullName: "Dalton Kincaid", Position: "TE", Status: "Out"},
	}, merged["BUF"])
	assert.Equal(t, []analyzer.Injury{
		{AthleteID: "201", FullName: "Tyreek Hill", Position: "WR", Status: "Questionable"},
	}, merged["MIA"])
}

func TestLoadSlipSignalsPopulatesSharpBenchmarks(t *testing.T) {
	t.Parallel()

	store := openSignalTestStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	week, err := store.UpsertWeek(t.Context(), 2026, nflverse.SeasonTypeRegular, 6)
	require.NoError(t, err)
	g1, err := store.UpsertGame(t.Context(), db.Game{
		ID:       "g1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)
	g2, err := store.UpsertGame(t.Context(), db.Game{
		ID:       "g2",
		WeekID:   week.ID,
		HomeTeam: "KC",
		AwayTeam: "BAL",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)

	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     g1.ID,
			Book:       "lowvig",
			MarketType: "SPREAD",
			Team:       "BUF",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     g1.ID,
			Book:       "lowvig",
			MarketType: "SPREAD",
			Team:       "MIA",
			Side:       "TEAM",
			Line:       new(3.5),
			Price:      -110,
			FetchedAt:  now,
		},
		{
			GameID:     g1.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -105,
			FetchedAt:  now,
		},
		{
			GameID:     g1.ID,
			Book:       "pinnacle",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "UNDER",
			Line:       new(265.5),
			Price:      -115,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	gamesByID, _ := analyzer.LoadSlipSignals(t.Context(), store, now, week.ID, []db.Game{g1, g2}, nil)
	require.Contains(t, gamesByID, g1.ID)
	require.Contains(t, gamesByID, g2.ID)
	assert.Nil(t, gamesByID[g2.ID].Sharp, "games without stored odds should fail soft")

	sharp := gamesByID[g1.ID].Sharp
	require.NotNil(t, sharp)

	spreadKey := analyzer.SharpKey{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"}
	spreadQuote, ok := sharp[spreadKey]
	require.True(t, ok)
	assert.Equal(t, "LowVig", spreadQuote.Source)
	assert.Equal(t, 2, spreadQuote.Tier)
	require.NotNil(t, spreadQuote.Line)
	assert.InDelta(t, -3.5, *spreadQuote.Line, 1e-9)
	assert.InDelta(t, 0.5, spreadQuote.FairProb, 1e-6)

	propKey := analyzer.SharpKey{MarketType: "PASS_YDS", Team: "BUF", PlayerName: "josh allen", Side: "OVER"}
	propQuote, ok := sharp[propKey]
	require.True(t, ok)
	assert.Equal(t, "Pinnacle", propQuote.Source)
	assert.Equal(t, 1, propQuote.Tier)
	require.NotNil(t, propQuote.Line)
	assert.InDelta(t, 265.5, *propQuote.Line, 1e-9)

	leg := analyzer.Leg{
		ID:     1,
		GameID: g1.ID,
		Market: "SPREAD",
		Team:   "BUF",
		Side:   "TEAM",
		Line:   new(-2.5),
		Odds:   -110,
	}
	grade := analyzer.ScoreLeg(leg, gamesByID, nil)
	require.Len(t, grade.Factors, 1)
	assert.Equal(t, "sharp_line_edge", grade.Factors[0].Rule)
	assert.Equal(t, 8, grade.Factors[0].Delta)

	emptyGames, emptyStats := analyzer.LoadSlipSignals(t.Context(), store, now, week.ID, nil, nil)
	assert.Empty(t, emptyGames)
	assert.Nil(t, emptyStats)
}

func TestLoadBoardSignals(t *testing.T) {
	t.Parallel()

	store := openSignalTestStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	week, err := store.UpsertWeek(t.Context(), 2026, nflverse.SeasonTypeRegular, 6)
	require.NoError(t, err)

	summaryJSON := `[{
		"team": {"id": "2", "displayName": "Buffalo Bills", "abbreviation": "BUF"},
		"injuries": [{
			"status": "Out",
			"athlete": {"id": "101", "fullName": "Keon Coleman", "position": {"abbreviation": "WR"}},
			"details": {"type": "Wrist", "detail": "Out"}
		}]
	}]`
	g1, err := store.UpsertGame(t.Context(), db.Game{
		ID:       "g1",
		WeekID:   week.ID,
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  now.Add(48 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)
	g1.InjuriesJSON = &summaryJSON

	_, err = store.UpsertNfeloGames(t.Context(), []db.NfeloGame{
		{GameID: "g1", HomeSpread: -5.5, HomeWinProb: 0.68, UpdatedAt: now},
	})
	require.NoError(t, err)
	_, err = store.UpsertNFLverseTeamWeeks(t.Context(), []db.NFLverseTeamWeek{
		{
			Season:     2026,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    5,
			Team:       "BUF",
			OffPassEPA: 0.22,
			UpdatedAt:  now,
		},
		{
			Season:            2026,
			SeasonType:        nflverse.SeasonTypeRegular,
			WeekNum:           5,
			Team:              "MIA",
			DefPassEPAAllowed: 0.24,
			UpdatedAt:         now,
		},
	})
	require.NoError(t, err)
	_, err = store.UpsertNFLversePlayerWeeks(t.Context(), []db.NFLversePlayerWeek{
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 5, Team: "BUF", OffensePct: 0.90, TargetShare: 0.28, WOPR: 0.62, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 4, Team: "BUF", OffensePct: 0.85, TargetShare: 0.24, WOPR: 0.55, UpdatedAt: now,
		},
		{
			ESPNID: "p1", Season: 2026, SeasonType: nflverse.SeasonTypeRegular,
			WeekNum: 3, Team: "BUF", OffensePct: 0.80, TargetShare: 0.22, WOPR: 0.51, UpdatedAt: now,
		},
	})
	require.NoError(t, err)
	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID: "p1", Team: "BUF", ESPNTeamID: "2",
			FullName: "Khalil Shakir", NormName: "khalil shakir", Position: "WR",
			UpdatedAt: now,
		},
	}))
	require.NoError(t, store.UpsertPlayerGameLog(t.Context(), db.PlayerGameLog{
		AthleteID:   "p1",
		GamesPlayed: 3,
		StatsJSON:   `{"REC_YDS":[82,74,61]}`,
		UpdatedAt:   now,
	}))
	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID: g1.ID, Book: "lowvig", MarketType: "SPREAD", Team: "BUF", Side: "TEAM",
			Line: new(-3.5), Price: -110, FetchedAt: now,
		},
		{
			GameID: g1.ID, Book: "lowvig", MarketType: "SPREAD", Team: "MIA", Side: "TEAM",
			Line: new(3.5), Price: -110, FetchedAt: now,
		},
	})
	require.NoError(t, err)
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NfeloSource, now.Add(-12*time.Hour), nil))
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-24*time.Hour), nil))

	// Duplicate and empty playerIDs should be deduplicated and ignored cleanly.
	gamesByID, stats := analyzer.LoadBoardSignals(
		t.Context(), store, now, week.ID, []db.Game{g1}, []string{"", "p1", "p1", "unknown"},
	)
	require.Contains(t, gamesByID, "g1")
	require.NotNil(t, gamesByID["g1"].Nfelo)
	assert.InDelta(t, -5.5, gamesByID["g1"].Nfelo.HomeSpread, 1e-9)
	assert.NotNil(t, gamesByID["g1"].HomeEPA)
	assert.NotNil(t, gamesByID["g1"].AwayEPA)
	require.Len(t, gamesByID["g1"].Injuries["BUF"], 1)
	assert.Contains(t, gamesByID["g1"].Sharp, analyzer.SharpKey{MarketType: "SPREAD", Team: "BUF", Side: "TEAM"})

	require.Contains(t, stats, "p1")
	assert.NotContains(t, stats, "unknown")
	assert.NotContains(t, stats, "")
	assert.Equal(t, "WR", stats["p1"].Position)
	assert.Equal(t, []float64{82, 74, 61}, stats["p1"].Games["REC_YDS"])
	require.NotNil(t, stats["p1"].Usage)
	assert.InDelta(t, 0.56, stats["p1"].Usage.Last3WOPR, 1e-9)

	// Empty inputs fail soft.
	emptyGames, emptyStats := analyzer.LoadBoardSignals(t.Context(), store, now, week.ID, nil, nil)
	assert.Empty(t, emptyGames)
	assert.Nil(t, emptyStats)
}
