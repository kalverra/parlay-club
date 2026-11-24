package web

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
)

func openResearchUnitStore(t *testing.T) *db.Store {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, db.Migrate(t.Context(), conn))
	return db.NewStore(conn)
}

func TestBuildPlayerResearchViewDetails(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := openResearchUnitStore(t)
	now := time.Date(2026, time.November, 15, 13, 0, 0, 0, time.UTC)

	week, err := store.UpsertWeek(ctx, 2026, 2, 11)
	require.NoError(t, err)

	game, err := store.UpsertGame(ctx, db.Game{
		ID:       "g1",
		WeekID:   week.ID,
		HomeTeam: "CIN",
		AwayTeam: "BAL",
		Kickoff:  now.Add(2 * time.Hour),
		Status:   db.GameScheduled,
	})
	require.NoError(t, err)
	_, err = store.SetGameWeather(ctx, game.ID, 42.0, 14.0, 0, 0, now)
	require.NoError(t, err)

	fetched := now.Add(-time.Hour)
	_, err = store.UpsertOdds(ctx, []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "SPREAD",
			Team:       "CIN",
			Side:       "TEAM",
			Line:       new(-3.5),
			Price:      -110,
			FetchedAt:  fetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "TOTAL",
			Side:       "OVER",
			Line:       new(48.5),
			Price:      -110,
			FetchedAt:  fetched,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "CIN",
			PlayerName: "Ja'Marr Chase",
			Side:       "OVER",
			Line:       new(80.5),
			Price:      -110,
			FetchedAt:  fetched,
		},
	})
	require.NoError(t, err)

	require.NoError(t, store.UpsertRosterPlayers(ctx, []db.RosterPlayer{
		{
			AthleteID:    "4362628",
			Team:         "CIN",
			FullName:     "Ja'Marr Chase",
			Position:     "WR",
			InjuryStatus: "Questionable",
			UpdatedAt:    now,
		},
	}))

	// 6 games: L5 are [100, 50, 0, 120, 80.5] (100=most recent), 6th is 90.
	// Chronological L5: [80.5 (PUSH), 120 (OVER), 0 (UNDER), 50 (UNDER), 100 (OVER)].
	// L5 Hit (>80.5): 2/5. Season Hit (>80.5): 3/6. L5 Min-Max: 0–120.
	// maxVal = 120. LinePct = round(80.5/120 * 100) = 67.
	require.NoError(t, store.UpsertPlayerGameLog(ctx, db.PlayerGameLog{
		AthleteID:   "4362628",
		GamesPlayed: 6,
		StatsJSON:   `{"REC_YDS":[100,50,0,120,80.5,90]}`,
		GamesJSON: `[
			{"eventId":"e5","week":10,"opponent":"PIT","atVs":"@","result":"W","score":"24-17"},
			{"eventId":"e4","week":9,"opponent":"LV","atVs":"vs","result":"W","score":"31-14"},
			{"eventId":"e3","week":8,"opponent":"PHI","atVs":"vs","result":"L","score":"17-20"},
			{"eventId":"e2","week":7,"opponent":"CLE","atVs":"@","result":"W","score":"21-14"},
			{"eventId":"e1","week":6,"opponent":"NYG","atVs":"@","result":"W","score":"17-7"}
		]`,
		UpdatedAt: now,
	}))

	// Seed nflverse team & player weeks for defense ranks and opp defense L5.
	require.NoError(t, store.RecordSync(ctx, "nflverse:season:2026", now, nil))
	_, err = store.UpsertNFLverseTeamWeeks(ctx, []db.NFLverseTeamWeek{
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "BAL",
			DefPassEPAAllowed: 0.01,
			DefRushEPAAllowed: 0.01,
			UpdatedAt:         now,
		},
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "PIT",
			DefPassEPAAllowed: 0.05,
			DefRushEPAAllowed: 0.05,
			UpdatedAt:         now,
		},
		{
			Season:            2026,
			SeasonType:        2,
			WeekNum:           1,
			Team:              "LV",
			DefPassEPAAllowed: 0.25,
			DefRushEPAAllowed: 0.25,
			UpdatedAt:         now,
		},
	})
	require.NoError(t, err)

	_, err = store.UpsertNFLversePlayerWeeks(ctx, []db.NFLversePlayerWeek{
		{
			Season: 2026, SeasonType: 2, WeekNum: 8, ESPNID: "4362628", Team: "CIN",
			Opponent: "PHI", PlayerName: "Ja'Marr Chase", Position: "WR",
			OffensePct: 0.90, TargetShare: 0.30, AirYardsShare: 0.42, WOPR: 0.78, RecYds: 54, UpdatedAt: now,
		},
		{
			Season: 2026, SeasonType: 2, WeekNum: 9, ESPNID: "opp1", Team: "CLE",
			Opponent: "BAL", PlayerName: "Jerry Jeudy", Position: "WR",
			RecYds: 95, UpdatedAt: now,
		},
		{
			Season: 2026, SeasonType: 2, WeekNum: 10, ESPNID: "opp2", Team: "PIT",
			Opponent: "BAL", PlayerName: "George Pickens", Position: "WR",
			RecYds: 60, UpdatedAt: now,
		},
	})
	require.NoError(t, err)

	h := &researchHandlers{
		deps: Dependencies{
			Logger: zerolog.Nop(),
			Store:  store,
			Now:    func() time.Time { return now },
		},
	}

	view, err := h.loadPlayerResearch(ctx, "4362628", "", "REC_YDS", "")
	require.NoError(t, err)

	// Matchup context & injury status.
	assert.Equal(t, "Questionable", view.InjuryStatus)
	require.NotNil(t, view.Matchup)
	assert.Equal(t, "BAL", view.Matchup.Opponent)
	assert.Equal(t, "vs BAL", view.Matchup.OpponentLabel)
	assert.Equal(t, "-3.5", view.Matchup.Spread)
	assert.Equal(t, "48.5", view.Matchup.GameTotal)
	assert.Equal(t, "42°F · 14 mph", view.Matchup.Weather)

	// Hit rates, L5 min/max, proportional BarChartView.
	assert.Equal(t, "2/5", view.L5HitRate)
	assert.Equal(t, "3/6", view.SeasonHitRate)
	assert.Equal(t, "0–120", view.L5MinMax)
	assert.True(t, view.PlayerChart.HasLine)
	assert.Equal(t, "80.5", view.PlayerChart.LineStr)
	assert.Equal(t, 67, view.PlayerChart.LinePct)
	require.Len(t, view.PlayerChart.Bars, 5)
	assert.Equal(t, "@NYG", view.PlayerChart.Bars[0].OpponentLabel)
	assert.Equal(t, "PUSH", view.PlayerChart.Bars[0].ResultText)
	assert.Equal(t, 100, view.PlayerChart.Bars[1].HeightPct)
	assert.Equal(t, "OVER", view.PlayerChart.Bars[1].ResultText)
	assert.Equal(t, 0, view.PlayerChart.Bars[2].HeightPct)
	assert.Equal(t, "UNDER", view.PlayerChart.Bars[2].ResultText)
	assert.Equal(t, 42, view.PlayerChart.Bars[3].HeightPct)
	assert.Equal(t, "UNDER", view.PlayerChart.Bars[3].ResultText)
	assert.Equal(t, "@PIT", view.PlayerChart.Bars[4].OpponentLabel)
	require.NotNil(t, view.PlayerChart.Bars[4].OverallDef)
	assert.Equal(t, "Def #2", view.PlayerChart.Bars[4].OverallDef.Label)

	// Opposing defense L5 chart.
	assert.Equal(t, "BAL Defense · Last 5 vs WR · Rec Yds", view.OppDefenseTitle)
	require.NotNil(t, view.OppOverallBadge)
	assert.Equal(t, 1, view.OppOverallBadge.Rank)
	require.NotNil(t, view.OppDefenseChart)
	assert.True(t, view.OppDefenseChart.HasLine)
	require.Len(t, view.OppDefenseChart.Bars, 2)
	assert.Equal(t, "J. Jeudy", view.OppDefenseChart.Bars[0].Label)
	assert.Equal(t, "CLE · W9", view.OppDefenseChart.Bars[0].SubLabel)
	assert.Equal(t, "OVER", view.OppDefenseChart.Bars[0].ResultText)
	assert.Equal(t, "G. Pickens", view.OppDefenseChart.Bars[1].Label)
	assert.Equal(t, "UNDER", view.OppDefenseChart.Bars[1].ResultText)

	// WR usage tiles.
	assert.Equal(t, []UsageTileView{
		{Label: "Snap % (L3)", Value: "90%"},
		{Label: "Target share", Value: "30.0%"},
		{Label: "Air Yards %", Value: "42.0%"},
		{Label: "WOPR", Value: "0.78"},
	}, view.UsageTiles)
}

func TestBuildUsageTilesByPosition(t *testing.T) {
	t.Parallel()

	sig := analyzer.PlayerResearchSignals{
		HasUsage:       true,
		SnapPct:        0.80,
		TargetShare:    0.15,
		AirYardsShare:  0.22,
		WOPR:           0.44,
		CarriesPerGame: 16.5,
		TargetsPerGame: 3.5,
		PassAttPerGame: 34.2,
		RZOppsPerGame:  4.0,
		EPAPerGame:     6.4,
		Opponent:       "BAL",
		OppDefRank:     4,
	}

	qb := buildUsageTiles("QB", sig, "Opp Pass Def", "#4 vs BAL")
	assert.Equal(t, []UsageTileView{
		{Label: "Pass Att/g (L3)", Value: "34.2"},
		{Label: "EPA/g (L3)", Value: "6.4"},
		{Label: "Rush Att/g (L3)", Value: "16.5"},
		{Label: "Opp Pass Def", Value: "#4 vs BAL"},
	}, qb)

	rb := buildUsageTiles("FB", sig, "Opp Rush Def", "#4 vs BAL")
	assert.Equal(t, []UsageTileView{
		{Label: "Snap % (L3)", Value: "80%"},
		{Label: "Carries/g (L3)", Value: "16.5"},
		{Label: "Targets/g (L3)", Value: "3.5"},
		{Label: "RZ Opps/g", Value: "4.0"},
	}, rb)

	k := buildUsageTiles("K", analyzer.PlayerResearchSignals{
		HasUsage:   true,
		OppDefRank: 12,
	}, "Opp Pass Def", "#12 vs BAL")
	assert.Equal(t, []UsageTileView{
		{Label: "Opp Pass Def", Value: "#12 vs BAL"},
	}, k)
}
