package eval_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/ingest/espn"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func TestBuildMarketCandidatesDeduplicatesAndGuardsMissingData(t *testing.T) {
	t.Parallel()

	kickoff := time.Date(2025, 10, 5, 17, 0, 0, 0, time.UTC)
	finalGame := db.Game{
		ID:       "g1",
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		Kickoff:  kickoff,
		Status:   db.GameFinal,
	}
	schedGame := finalGame
	schedGame.Status = db.GameScheduled

	roster := []db.RosterPlayer{
		{AthleteID: "p-qb", Team: "BUF", FullName: "Josh Allen", Position: "QB"},
	}

	oddsRows := []db.Odds{
		// 5 books for BUF SPREAD -> median line -3.5, median price -110.
		{GameID: "g1", Book: "dk", MarketType: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-3.0), Price: -115},
		{GameID: "g1", Book: "fd", MarketType: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-3.5), Price: -112},
		{GameID: "g1", Book: "mgm", MarketType: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-3.5), Price: -110},
		{GameID: "g1", Book: "czr", MarketType: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-4.0), Price: -108},
		{GameID: "g1", Book: "pin", MarketType: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-4.5), Price: -105},
		// 3 books for PASS_YDS OVER -> median line 255.5, median price -110.
		{
			GameID: "g1", Book: "dk", MarketType: "PASS_YDS", Team: "BUF",
			PlayerName: "Josh Allen", Side: "OVER", Line: new(250.5), Price: -115,
		},
		{
			GameID: "g1", Book: "fd", MarketType: "PASS_YDS", Team: "BUF",
			PlayerName: "Josh Allen", Side: "OVER", Line: new(255.5), Price: -110,
		},
		{
			GameID: "g1", Book: "mgm", MarketType: "PASS_YDS", Team: "BUF",
			PlayerName: "Josh Allen", Side: "OVER", Line: new(260.5), Price: -105,
		},
		// Non-line markets (kept without Line).
		{GameID: "g1", Book: "dk", MarketType: "ML", Team: "BUF", Side: "BUF", Price: -180},
		{
			GameID: "g1", Book: "dk", MarketType: "ANYTIME_TD", Team: "BUF",
			PlayerName: "Josh Allen", Side: "YES", Price: 120,
		},
		// Dropped rows: OTHER, zero odds, missing line on line-based market.
		{GameID: "g1", Book: "dk", MarketType: "OTHER", Team: "BUF", Side: "YES", Line: new(1.5), Price: 100},
		{GameID: "g1", Book: "dk", MarketType: "TOTAL", Side: "OVER", Line: new(47.5), Price: 0},
		{GameID: "g1", Book: "dk", MarketType: "TOTAL", Side: "UNDER", Price: -110},
	}

	assert.Empty(t, eval.BuildMarketCandidates(schedGame, oddsRows, roster))

	cands := eval.BuildMarketCandidates(finalGame, oddsRows, roster)
	require.Len(t, cands, 4)

	byMarket := make(map[string]analyzer.Leg, len(cands))
	for _, c := range cands {
		byMarket[c.Market] = c
	}

	spread := byMarket["SPREAD"]
	require.NotNil(t, spread.Line)
	assert.InDelta(t, -3.5, *spread.Line, 1e-9)
	assert.Equal(t, -110, spread.Odds)
	assert.Equal(t, "BUF", spread.Team)

	pass := byMarket["PASS_YDS"]
	require.NotNil(t, pass.Line)
	assert.InDelta(t, 255.5, *pass.Line, 1e-9)
	assert.Equal(t, -110, pass.Odds)
	assert.Equal(t, "p-qb", pass.PlayerID)

	assert.Equal(t, -180, byMarket["ML"].Odds)
	assert.Nil(t, byMarket["ML"].Line)
	assert.Equal(t, 120, byMarket["ANYTIME_TD"].Odds)
	assert.Nil(t, byMarket["ANYTIME_TD"].Line)
}

func TestMarketGraderGameLinesAndTeamTotals(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	week4, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 4)
	require.NoError(t, err)

	finalGame := db.Game{
		ID:        "g-final",
		WeekID:    week4.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		Status:    db.GameFinal,
		HomeScore: new(24),
		AwayScore: new(21),
	}
	tieGame := db.Game{
		ID:        "g-tie",
		WeekID:    week4.ID,
		HomeTeam:  "KC",
		AwayTeam:  "LAC",
		Status:    db.GameFinal,
		HomeScore: new(20),
		AwayScore: new(20),
	}
	noScoreGame := db.Game{
		ID:       "g-noscore",
		WeekID:   week4.ID,
		HomeTeam: "DET",
		AwayTeam: "GB",
		Status:   db.GameFinal,
	}
	schedGame := db.Game{
		ID:        "g-sched",
		WeekID:    week4.ID,
		HomeTeam:  "SF",
		AwayTeam:  "SEA",
		Status:    db.GameScheduled,
		HomeScore: new(10),
		AwayScore: new(7),
	}

	mg := eval.NewMarketGrader(ctx, store, week4)

	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-2.5), Odds: -110,
	}))
	assert.Equal(t, parlay.Push, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-3.0), Odds: -110,
	}))
	assert.Equal(t, parlay.Lost, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "SPREAD", Team: "BUF", Side: "BUF", Line: new(-3.5), Odds: -110,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "ML", Team: "BUF", Side: "BUF", Odds: -150,
	}))
	assert.Equal(t, parlay.Lost, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "ML", Team: "MIA", Side: "MIA", Odds: 130,
	}))
	assert.Equal(t, parlay.Push, mg.GradeLeg(ctx, store, tieGame, analyzer.Leg{
		GameID: "g-tie", Market: "ML", Team: "KC", Side: "KC", Odds: -120,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "TOTAL", Side: "OVER", Line: new(44.5), Odds: -110,
	}))
	assert.Equal(t, parlay.Push, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "TOTAL", Side: "UNDER", Line: new(45.0), Odds: -110,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "TEAM_TOTAL", Team: "BUF", Side: "OVER", Line: new(23.5), Odds: -110,
	}))
	assert.Equal(t, parlay.Lost, mg.GradeLeg(ctx, store, finalGame, analyzer.Leg{
		GameID: "g-final", Market: "TEAM_TOTAL", Team: "MIA", Side: "OVER", Line: new(21.5), Odds: -110,
	}))

	// Missing scores and non-final games return NeedsGrading / Pending and are dropped by GradeCandidates.
	assert.Equal(t, parlay.NeedsGrading, mg.GradeLeg(ctx, store, noScoreGame, analyzer.Leg{
		GameID: "g-noscore", Market: "ML", Team: "DET", Side: "DET", Odds: -110,
	}))
	assert.Equal(t, parlay.Pending, mg.GradeLeg(ctx, store, schedGame, analyzer.Leg{
		GameID: "g-sched", Market: "ML", Team: "SF", Side: "SF", Odds: -110,
	}))

	gamesMap := map[string]db.Game{
		finalGame.ID:   finalGame,
		noScoreGame.ID: noScoreGame,
		schedGame.ID:   schedGame,
	}
	replayed := mg.GradeCandidates(ctx, store, gamesMap, []analyzer.Leg{
		{GameID: finalGame.ID, Market: "ML", Team: "BUF", Side: "BUF", Odds: -150},
		{GameID: noScoreGame.ID, Market: "ML", Team: "DET", Side: "DET", Odds: -110},
		{GameID: schedGame.ID, Market: "ML", Team: "SF", Side: "SF", Odds: -110},
	})
	require.Len(t, replayed, 1)
	assert.Equal(t, parlay.Won, replayed[0].Result)
}

func TestMarketGraderESPNBoxScoreAndNFLverseFallback(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	week4, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 4)
	require.NoError(t, err)

	// Game 1: completed archived ESPN summary box score (including true DNP -> Void).
	summaryBody := []byte(`{
		"header":{"competitions":[{"status":{"type":{"completed":true}}}]},
		"boxscore":{"players":[
			{"team":{"abbreviation":"BUF"},"statistics":[
				{"name":"passing","keys":["passingYards","passingTouchdowns"],"athletes":[
					{"athlete":{"id":"qb-1","displayName":"Josh Allen"},"stats":["280","2"]}
				]},
				{"name":"rushing","keys":["rushingAttempts","rushingYards","rushingTouchdowns"],"athletes":[
					{"athlete":{"id":"qb-1","displayName":"Josh Allen"},"stats":["6","32","0"]}
				]}
			]}
		]}
	}`)
	mustArchiveResponse(t, store, "espn", db.ArchiveSummaryKey("g-espn"),
		time.Date(2025, 10, 5, 20, 30, 0, 0, time.UTC), summaryBody)

	// Game 2: BoxScoreJSON populated directly on db.Game.
	boxJSONBytes, err := json.Marshal(espn.BoxScore{
		Completed: true,
		HasStats:  true,
		Players: []espn.PlayerBoxStats{
			{
				PlayerID:    "wr-2",
				DisplayName: "Stefon Diggs",
				TeamAbbrev:  "HOU",
				Receiving:   espn.ReceivingStats{Receptions: 7, ReceivingYards: 95, ReceivingTDs: 1},
			},
		},
	})
	require.NoError(t, err)

	// Game 3: Incomplete archived ESPN box score -> falls back to same-week nflverse_player_weeks.
	incompleteBody := []byte(`{
		"header":{"competitions":[{"status":{"type":{"completed":false}}}]},
		"boxscore":{"players":[]}
	}`)
	mustArchiveResponse(t, store, "espn", db.ArchiveSummaryKey("g-nflv"),
		time.Date(2025, 10, 5, 20, 30, 0, 0, time.UTC), incompleteBody)

	// Seed same-week (Week 4) and other-week (Week 3) nflverse_player_weeks.
	_, err = store.UpsertNFLversePlayerWeeks(ctx, []db.NFLversePlayerWeek{
		{
			ESPNID:     "qb-3",
			Season:     2025,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    4,
			Team:       "KC",
			Opponent:   "LAC",
			PlayerName: "Patrick Mahomes",
			Position:   "QB",
			PassYds:    310,
			PassTDs:    3,
			Carries:    4,
			RushYds:    18,
			RushTDs:    0,
		},
		{
			ESPNID:     "rb-3",
			Season:     2025,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    4,
			Team:       "KC",
			Opponent:   "LAC",
			PlayerName: "Isiah Pacheco",
			Position:   "RB",
			Carries:    16,
			RushYds:    84,
			RushTDs:    1,
			Receptions: 3,
			RecYds:     22,
		},
		{
			ESPNID:     "k-3",
			Season:     2025,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    4,
			Team:       "LAC",
			Opponent:   "KC",
			PlayerName: "Cameron Dicker",
			Position:   "K",
			KickerPts:  11,
		},
		// Week 3 row for a Week 4 game without Week 4 data must not leak.
		{
			ESPNID:     "wr-other",
			Season:     2025,
			SeasonType: nflverse.SeasonTypeRegular,
			WeekNum:    3,
			Team:       "DAL",
			Opponent:   "NYG",
			PlayerName: "CeeDee Lamb",
			Position:   "WR",
			RecYds:     120,
		},
	})
	require.NoError(t, err)

	gESPN := db.Game{
		ID:        "g-espn",
		WeekID:    week4.ID,
		HomeTeam:  "BUF",
		AwayTeam:  "MIA",
		Status:    db.GameFinal,
		HomeScore: new(27),
		AwayScore: new(17),
	}
	gBoxJSON := db.Game{
		ID:           "g-boxjson",
		WeekID:       week4.ID,
		HomeTeam:     "HOU",
		AwayTeam:     "IND",
		Status:       db.GameFinal,
		HomeScore:    new(24),
		AwayScore:    new(20),
		BoxScoreJSON: new(string(boxJSONBytes)),
	}
	gNFLV := db.Game{
		ID:        "g-nflv",
		WeekID:    week4.ID,
		HomeTeam:  "KC",
		AwayTeam:  "LAC",
		Status:    db.GameFinal,
		HomeScore: new(28),
		AwayScore: new(17),
	}
	gMissing := db.Game{
		ID:        "g-missing",
		WeekID:    week4.ID,
		HomeTeam:  "DAL",
		AwayTeam:  "NYG",
		Status:    db.GameFinal,
		HomeScore: new(21),
		AwayScore: new(14),
	}

	mg := eval.NewMarketGrader(ctx, store, week4)

	// 1. Completed archived ESPN summary: passing yards won, passing TDs do NOT count as ANYTIME_TD, absent player -> Void.
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gESPN, analyzer.Leg{
		GameID:     gESPN.ID,
		Market:     "PASS_YDS",
		Team:       "BUF",
		PlayerID:   "qb-1",
		PlayerName: "Josh Allen",
		Side:       "OVER",
		Line:       new(275.5),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Lost, mg.GradeLeg(ctx, store, gESPN, analyzer.Leg{
		GameID:     gESPN.ID,
		Market:     "ANYTIME_TD",
		Team:       "BUF",
		PlayerID:   "qb-1",
		PlayerName: "Josh Allen",
		Side:       "YES",
		Odds:       150,
	}))
	assert.Equal(t, parlay.Void, mg.GradeLeg(ctx, store, gESPN, analyzer.Leg{
		GameID:     gESPN.ID,
		Market:     "REC_YDS",
		Team:       "BUF",
		PlayerID:   "dnp-1",
		PlayerName: "Inactive WR",
		Side:       "OVER",
		Line:       new(45.5),
		Odds:       -110,
	}))

	// 2. BoxScoreJSON on db.Game.
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gBoxJSON, analyzer.Leg{
		GameID:     gBoxJSON.ID,
		Market:     "RECEPTIONS",
		Team:       "HOU",
		PlayerID:   "wr-2",
		PlayerName: "Stefon Diggs",
		Side:       "OVER",
		Line:       new(6.5),
		Odds:       -110,
	}))

	// 3. Same-week nflverse fallback across all player prop markets (by ID and by name+team).
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "PASS_YDS",
		Team:       "KC",
		PlayerID:   "qb-3",
		PlayerName: "Patrick Mahomes",
		Side:       "OVER",
		Line:       new(299.5),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "PASS_TDS",
		Team:       "KC",
		PlayerID:   "qb-3",
		PlayerName: "Patrick Mahomes",
		Side:       "OVER",
		Line:       new(2.5),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Lost, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "ANYTIME_TD",
		Team:       "KC",
		PlayerID:   "qb-3",
		PlayerName: "Patrick Mahomes",
		Side:       "YES",
		Odds:       220,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "RUSH_ATT",
		Team:       "KC",
		PlayerName: "Isiah Pacheco",
		Side:       "OVER",
		Line:       new(15.5),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "RUSH_YDS",
		Team:       "KC",
		PlayerName: "Isiah Pacheco",
		Side:       "OVER",
		Line:       new(75.5),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "REC_YDS",
		Team:       "KC",
		PlayerName: "Isiah Pacheco",
		Side:       "OVER",
		Line:       new(20.5),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Push, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "RECEPTIONS",
		Team:       "KC",
		PlayerName: "Isiah Pacheco",
		Side:       "OVER",
		Line:       new(3.0),
		Odds:       -110,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID: gNFLV.ID, Market: "ANYTIME_TD", Team: "KC", PlayerName: "Isiah Pacheco", Side: "YES", Odds: -115,
	}))
	assert.Equal(t, parlay.Won, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "KICKER_PTS",
		Team:       "LAC",
		PlayerName: "Cameron Dicker",
		Side:       "OVER",
		Line:       new(8.5),
		Odds:       -110,
	}))

	// 4. Unmatched player in nflverse-populated game -> NeedsGrading (not Void), dropped by GradeCandidates.
	assert.Equal(t, parlay.NeedsGrading, mg.GradeLeg(ctx, store, gNFLV, analyzer.Leg{
		GameID:     gNFLV.ID,
		Market:     "REC_YDS",
		Team:       "KC",
		PlayerID:   "unmapped-99",
		PlayerName: "Unknown WR",
		Side:       "OVER",
		Line:       new(35.5),
		Odds:       -110,
	}))

	// 5. Game with no Week 4 nflverse rows for either team -> NeedsGrading (Week 3 row does not leak).
	assert.Equal(t, parlay.NeedsGrading, mg.GradeLeg(ctx, store, gMissing, analyzer.Leg{
		GameID:     gMissing.ID,
		Market:     "REC_YDS",
		Team:       "DAL",
		PlayerID:   "wr-other",
		PlayerName: "CeeDee Lamb",
		Side:       "OVER",
		Line:       new(85.5),
		Odds:       -110,
	}))
}
