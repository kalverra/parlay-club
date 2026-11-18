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
		{Team: "BUF", WeekNum: 1, OffPassEPA: 0.10, DefPassEPAAllowed: 0.20, PlaysPerGame: 60, PassRate: 0.55},
		{Team: "BUF", WeekNum: 2, OffPassEPA: 0.30, DefPassEPAAllowed: 0.00, PlaysPerGame: 64, PassRate: 0.65},
		{Team: "MIA", WeekNum: 1, OffPassEPA: 0.05, DefPassEPAAllowed: 0.30, PlaysPerGame: 58, PassRate: 0.50},
	}
	profiles := analyzer.BuildTeamDefenseProfiles(rows)
	require.Len(t, profiles, 2)

	buf := profiles["BUF"]
	require.NotNil(t, buf)
	assert.InDelta(t, 0.20, buf.OffPassEPA, 1e-9)
	assert.InDelta(t, 0.10, buf.DefPassEPAAllowed, 1e-9)
	assert.Equal(t, 1, buf.OffPassEPARank)
	assert.Equal(t, 1, buf.DefPassEPAAllowedRank)
	assert.Equal(t, 2, profiles["MIA"].OffPassEPARank)
	assert.Equal(t, 2, profiles["MIA"].DefPassEPAAllowedRank)
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
			ESPNID:      "rb1",
			Season:      2026,
			SeasonType:  nflverse.SeasonTypeRegular,
			WeekNum:     5,
			Team:        "BUF",
			OffensePct:  0.70,
			TargetShare: 0.12,
			WOPR:        0.30,
			UpdatedAt:   now,
		},
		{
			ESPNID:      "rb1",
			Season:      2026,
			SeasonType:  nflverse.SeasonTypeRegular,
			WeekNum:     4,
			Team:        "BUF",
			OffensePct:  0.60,
			TargetShare: 0.10,
			WOPR:        0.20,
			UpdatedAt:   now,
		},
	})
	require.NoError(t, err)
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-24*time.Hour), nil))

	sig := analyzer.LoadPlayerResearchSignals(t.Context(), store, now, "rb1", "BUF", "RB", "")
	require.True(t, sig.HasUsage)
	assert.InDelta(t, 0.65, sig.SnapPct, 1e-9)
	assert.InDelta(t, 0.11, sig.TargetShare, 1e-9)
	assert.InDelta(t, 0.25, sig.WOPR, 1e-9)
	assert.Equal(t, "MIA", sig.Opponent)
	assert.Equal(t, "rush", sig.OppDefSplit)
	assert.Equal(t, 2, sig.OppDefRank)

	// Stale nflverse returns zeroed signals.
	require.NoError(t, store.RecordSync(t.Context(), analyzer.NFLverseSource(2026), now.Add(-8*24*time.Hour), nil))
	stale := analyzer.LoadPlayerResearchSignals(t.Context(), store, now, "rb1", "BUF", "RB", "REC_YDS")
	assert.False(t, stale.HasUsage)
	assert.Zero(t, stale.OppDefRank)
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
