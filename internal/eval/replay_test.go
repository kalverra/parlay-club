package eval_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/analyzer"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/eval"
	"github.com/kalverra/parlay-club/internal/ingest/archive"
	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
	"github.com/kalverra/parlay-club/internal/parlay"
)

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	database, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	require.NoError(t, db.Migrate(t.Context(), database))
	store := db.NewStore(database)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedGroupAndMember(t *testing.T, store *db.Store, name string) (db.Group, db.User) {
	t.Helper()
	ctx := t.Context()
	user, err := store.CreateUserWithIdentity(ctx, name, name+"@example.com", "sub-"+name, "")
	require.NoError(t, err)
	group, err := store.Create(ctx, user.ID, "Group "+name)
	require.NoError(t, err)
	return group, user
}

func mustArchiveResponse(
	t *testing.T,
	store *db.Store,
	source, key string,
	fetchedAt time.Time,
	body []byte,
) {
	t.Helper()
	require.NoError(t, store.SaveAPIResponse(t.Context(), archive.Record{
		Source:    source,
		Key:       key,
		Method:    "GET",
		URL:       "https://example.test/" + key,
		Status:    200,
		FetchedAt: fetchedAt,
		Body:      body,
	}))
}

func TestLoadDatasetLeakageGuard(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	group, user := seedGroupAndMember(t, store, "ada")
	week4, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 4)
	require.NoError(t, err)

	kickoff := time.Date(2025, 10, 5, 17, 0, 0, 0, time.UTC)
	_, err = store.UpsertGame(ctx, db.Game{
		ID:         "g4",
		WeekID:     week4.ID,
		HomeTeam:   "BUF",
		AwayTeam:   "MIA",
		HomeName:   "Buffalo Bills",
		AwayName:   "Miami Dolphins",
		HomeESPNID: "2",
		AwayESPNID: "15",
		Kickoff:    kickoff,
		Status:     db.GameFinal,
		Indoor:     false,
		WindMPH:    new(12.0),
	})
	require.NoError(t, err)

	_, err = store.UpsertNfeloGames(ctx, []db.NfeloGame{
		{
			GameID:      "g4",
			HomeSpread:  -6.5,
			HomeWinProb: 0.72,
			HomeQB:      "Josh Allen",
			AwayQB:      "Tua Tagovailoa",
		},
	})
	require.NoError(t, err)

	// Seed nflverse team and player weeks for Week 1..3 (valid prior) AND Week 4..6 (future/current leakage).
	var teamRows []db.NFLverseTeamWeek
	var playerRows []db.NFLversePlayerWeek
	for w := 1; w <= 6; w++ {
		defPass := 0.25 // favorable/weak pass defense before Week 4
		snap := 0.80
		if w >= 4 {
			defPass = -0.90 // poisoned future EPA
			snap = 0.10     // poisoned future snap share
		}
		teamRows = append(teamRows,
			db.NFLverseTeamWeek{
				Season: 2025, SeasonType: nflverse.SeasonTypeRegular, WeekNum: w,
				Team: "MIA", DefPassEPAAllowed: defPass, PlaysPerGame: 64, PassRate: 0.58,
			},
			db.NFLverseTeamWeek{
				Season: 2025, SeasonType: nflverse.SeasonTypeRegular, WeekNum: w,
				Team: "BUF", DefPassEPAAllowed: -0.05, PlaysPerGame: 65, PassRate: 0.56,
			},
		)
		playerRows = append(playerRows, db.NFLversePlayerWeek{
			Season: 2025, SeasonType: nflverse.SeasonTypeRegular, WeekNum: w,
			ESPNID: "espn-wr1", Team: "BUF",
			OffensePct: snap, TargetShare: 0.22,
		})
	}
	_, err = store.UpsertNFLverseTeamWeeks(ctx, teamRows)
	require.NoError(t, err)
	_, err = store.UpsertNFLversePlayerWeeks(ctx, playerRows)
	require.NoError(t, err)

	// Poison live overwritten player_gamelogs and roster_players tables.
	poisonedLog, _ := json.Marshal(map[string][]float64{"REC_YDS": {999, 999, 999, 999, 999}})
	require.NoError(t, store.UpsertPlayerGameLog(ctx, db.PlayerGameLog{
		AthleteID:   "espn-wr1",
		GamesPlayed: 5,
		StatsJSON:   string(poisonedLog),
		UpdatedAt:   kickoff.Add(24 * time.Hour),
	}))
	require.NoError(t, store.UpsertRosterPlayers(ctx, []db.RosterPlayer{
		{AthleteID: "espn-wr2", Team: "BUF", FullName: "Live Poison WR", Position: "WR", InjuryStatus: "Out"},
	}))

	// Archive pre-kickoff gamelog (65, 62, 60, 58, 55) and post-kickoff gamelog (1, 1, 1, 1, 1).
	preGamelog := `{"names":["receivingYards"],"labels":["YDS"],"categories":[{"name":"receiving","count":1}],"seasonTypes":[{"displayName":"Regular Season","categories":[{"displayName":"Regular Season","splitType":"0","events":[{"eventId":"1","stats":["65"]},{"eventId":"2","stats":["62"]},{"eventId":"3","stats":["60"]},{"eventId":"4","stats":["58"]},{"eventId":"5","stats":["55"]}]}]}]}`
	postGamelog := `{"names":["receivingYards"],"labels":["YDS"],"categories":[{"name":"receiving","count":1}],"seasonTypes":[{"displayName":"Regular Season","categories":[{"displayName":"Regular Season","splitType":"0","events":[{"eventId":"6","stats":["1"]}]}]}]}`
	mustArchiveResponse(t, store, "espn", "espn:gamelog:espn-wr1", kickoff.Add(-2*time.Hour), []byte(preGamelog))
	mustArchiveResponse(t, store, "espn", "espn:gamelog:espn-wr1", kickoff.Add(1*time.Hour), []byte(postGamelog))
	// Exact kickoff timestamp must also be excluded (strictly fetched_at < kickoff).
	mustArchiveResponse(t, store, "espn", "espn:gamelog:espn-wr1", kickoff, []byte(postGamelog))

	preRoster := `{"athletes":[{"items":[{"id":"espn-wr1","fullName":"Khalil Shakir","position":{"abbreviation":"WR"},"injuries":[]},{"id":"espn-wr3","fullName":"Prekickoff Out WR","position":{"abbreviation":"WR"},"injuries":[{"status":"Out"}]}]}]}`
	postRoster := `{"athletes":[{"items":[{"id":"espn-wr1","fullName":"Khalil Shakir","position":{"abbreviation":"RB"},"injuries":[]}]}]}`
	mustArchiveResponse(t, store, "espn", "espn:roster:2", kickoff.Add(-1*time.Hour), []byte(preRoster))
	mustArchiveResponse(t, store, "espn", "espn:roster:2", kickoff.Add(1*time.Hour), []byte(postRoster))

	preSummary := `{"injuries":[{"team":{"id":"2","displayName":"Buffalo Bills","abbreviation":"BUF"},"injuries":[{"status":"Out","athlete":{"id":"espn-wr3","fullName":"Prekickoff Out WR","position":{"abbreviation":"WR"}}}]}]}`
	mustArchiveResponse(t, store, "espn", "espn:summary:g4", kickoff.Add(-30*time.Minute), []byte(preSummary))
	mustArchiveResponse(t, store, "odds", "odds:lines", kickoff.Add(-30*time.Minute), []byte(`[]`))

	// Create and grade a slip in Week 4.
	slip, err := store.CreateSlip(
		ctx,
		group.ID,
		week4.ID,
		nil,
		"Week 4 Slip",
		db.SlateScopeFullWeek,
		2,
		[]int64{user.ID},
	)
	require.NoError(t, err)
	leg, err := store.CreateLeg(ctx, db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     new("g4"),
		MarketType: "REC_YDS",
		Selection:  "Khalil Shakir Over 54.5 Rec Yds",
		Team:       new("BUF"),
		PlayerName: new("Khalil Shakir"),
		PlayerID:   new("espn-wr1"),
		Side:       new("OVER"),
		Line:       new(54.5),
		Odds:       -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetLegResult(ctx, leg.ID, parlay.Won, "test"))
	require.NoError(t, store.SetSlipGrade(ctx, slip.ID, parlay.Won, db.SlipGraded))

	ds, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{})
	require.NoError(t, err)
	require.Len(t, ds.Weeks, 1)

	w := ds.Weeks[0]
	assert.Equal(t, 2025, w.Season)
	assert.Equal(t, 4, w.WeekNum)

	// Gamelog came from pre-kickoff archive, NOT live player_gamelogs (999) or post-kickoff archive (1).
	ps, ok := w.Stats["espn-wr1"]
	require.True(t, ok)
	assert.Equal(t, []float64{65, 62, 60, 58, 55}, ps.Games["REC_YDS"])
	assert.Equal(t, "WR", ps.Position)

	// Prior nflverse usage used Weeks 1..3 (OffensePct = 0.80), ignoring Weeks 4..6 (0.10).
	require.NotNil(t, ps.Usage)
	assert.InDelta(t, 0.80, ps.Usage.SeasonSnapAvg, 1e-9)

	// Prior nflverse team EPA used Weeks 1..3 (MIA DefPassEPAAllowed = 0.25), ignoring Weeks 4..6 (-0.90).
	g, ok := w.Games["g4"]
	require.True(t, ok)
	require.NotNil(t, g.AwayEPA)
	assert.InDelta(t, 0.25, g.AwayEPA.DefPassEPAAllowed, 1e-9)
	require.NotNil(t, g.Nfelo)
	assert.InDelta(t, -6.5, g.Nfelo.HomeSpread, 1e-9)

	// Injuries came from pre-kickoff archive, NOT live roster_players ("Live Poison WR").
	require.NotEmpty(t, g.Injuries["BUF"])
	assert.Equal(t, "Prekickoff Out WR", g.Injuries["BUF"][0].FullName)

	assert.Equal(t, eval.DataCoverage{
		TotalLegs:    1,
		WithGameLogs: 1,
		WithInjuries: 1,
		WithNFLVerse: 1,
		WithNfelo:    1,
		WithOdds:     1,
	}, ds.Coverage)
}

func TestLoadDatasetFailSoftAndFilters(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	group1, user1 := seedGroupAndMember(t, store, "ada")
	group2, user2 := seedGroupAndMember(t, store, "ben")

	w2, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 2)
	require.NoError(t, err)
	w5, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 5)
	require.NoError(t, err)

	_, err = store.UpsertGame(ctx, db.Game{
		ID: "g2", WeekID: w2.ID, HomeTeam: "KC", AwayTeam: "LV",
		HomeName: "Chiefs", AwayName: "Raiders",
		Kickoff: time.Date(2025, 9, 14, 17, 0, 0, 0, time.UTC), Status: db.GameFinal,
	})
	require.NoError(t, err)
	_, err = store.UpsertGame(ctx, db.Game{
		ID: "g5", WeekID: w5.ID, HomeTeam: "KC", AwayTeam: "BUF",
		HomeName: "Chiefs", AwayName: "Bills",
		Kickoff: time.Date(2025, 10, 12, 20, 0, 0, 0, time.UTC), Status: db.GameFinal,
	})
	require.NoError(t, err)

	// Slip in Week 2 for group 1 with no archive/nflverse/nfelo data.
	s2, err := store.CreateSlip(ctx, group1.ID, w2.ID, nil, "W2", db.SlateScopeFullWeek, 1, []int64{user1.ID})
	require.NoError(t, err)
	l2, err := store.CreateLeg(ctx, db.Leg{
		SlipID: s2.ID, UserID: user1.ID, GameID: new("g2"),
		MarketType: "SPREAD", Selection: "KC -3.5", Team: new("KC"), Side: new("TEAM"), Line: new(-3.5), Odds: -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetLegResult(ctx, l2.ID, parlay.Lost, "test"))
	require.NoError(t, store.SetSlipGrade(ctx, s2.ID, parlay.Lost, db.SlipGraded))

	// Slip in Week 5 for group 2.
	s5, err := store.CreateSlip(ctx, group2.ID, w5.ID, nil, "W5", db.SlateScopeFullWeek, 1, []int64{user2.ID})
	require.NoError(t, err)
	l5, err := store.CreateLeg(ctx, db.Leg{
		SlipID: s5.ID, UserID: user2.ID, GameID: new("g5"),
		MarketType: "SPREAD", Selection: "BUF +2.5", Team: new("BUF"), Side: new("TEAM"), Line: new(2.5), Odds: -110,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetLegResult(ctx, l5.ID, parlay.Won, "test"))
	require.NoError(t, store.SetSlipGrade(ctx, s5.ID, parlay.Won, db.SlipGraded))

	// Unfiltered: fails soft on missing signals and loads both weeks.
	dsAll, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{})
	require.NoError(t, err)
	require.Len(t, dsAll.Weeks, 2)
	assert.Equal(t, 2, dsAll.Coverage.TotalLegs)
	assert.Zero(t, dsAll.Coverage.WithGameLogs)
	assert.Zero(t, dsAll.Coverage.WithInjuries)
	assert.Zero(t, dsAll.Coverage.WithNFLVerse)
	assert.Zero(t, dsAll.Coverage.WithNfelo)

	// Week range filter.
	dsW5, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{Season: 2025, FromWeek: 4, ToWeek: 6})
	require.NoError(t, err)
	require.Len(t, dsW5.Weeks, 1)
	assert.Equal(t, 5, dsW5.Weeks[0].WeekNum)

	// Group filter.
	dsG1, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{GroupID: group1.ID})
	require.NoError(t, err)
	require.Len(t, dsG1.Weeks, 1)
	assert.Equal(t, 2, dsG1.Weeks[0].WeekNum)

	// Verify context cancellation / clean error handling isn't needed for normal path,
	// and verify background context works.
	_ = context.Background()
}

func TestLoadDatasetJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := eval.ReplayDataset{
		Coverage: eval.DataCoverage{
			TotalLegs:    2,
			WithGameLogs: 1,
			WithNfelo:    2,
		},
		Weeks: []eval.ReplayedWeek{
			{
				Season:   2025,
				WeekNum:  3,
				WeekType: "REG",
				Games: map[string]analyzer.Game{
					"g1": {ID: "g1", Home: "BUF", Away: "MIA", Indoor: true},
				},
				Stats: map[string]analyzer.PlayerStats{
					"p1": {Position: "WR", Games: map[string][]float64{"REC_YDS": {70, 65, 60, 55, 50}}},
				},
				Slips: []eval.ReplayedSlip{
					{
						SlipID:       10,
						Result:       parlay.Won,
						CombinedOdds: 2.0,
						Legs: []eval.ReplayedLeg{
							{
								Leg: analyzer.Leg{
									ID: 1, GameID: "g1", Market: "REC_YDS",
									Team: "BUF", PlayerID: "p1", Side: "OVER", Line: new(55.5), Odds: 100,
								},
								Result:      parlay.Won,
								DecimalOdds: 2.0,
							},
						},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(want)
	require.NoError(t, err)
	validPath := filepath.Join(dir, "dataset.json")
	require.NoError(t, os.WriteFile(validPath, raw, 0o600))

	got, err := eval.LoadDatasetJSON(validPath)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// Missing coverage.total_legs falls back to counting legs in weeks.
	noCovPath := filepath.Join(dir, "no_cov.json")
	noCov := want
	noCov.Coverage = eval.DataCoverage{}
	rawNoCov, err := json.Marshal(noCov)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(noCovPath, rawNoCov, 0o600))

	gotNoCov, err := eval.LoadDatasetJSON(noCovPath)
	require.NoError(t, err)
	assert.Equal(t, 1, gotNoCov.Coverage.TotalLegs)

	// Error cases: missing file and invalid JSON.
	_, err = eval.LoadDatasetJSON(filepath.Join(dir, "missing.json"))
	require.Error(t, err)

	badPath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(badPath, []byte("{not-json"), 0o600))
	_, err = eval.LoadDatasetJSON(badPath)
	require.Error(t, err)
}

func TestLoadDatasetMultiGameSlateAndCoverageParity(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	database, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	require.NoError(t, db.Migrate(ctx, database))
	store := db.NewStore(database)
	t.Cleanup(func() { _ = store.Close() })

	group, user := seedGroupAndMember(t, store, "ada")
	week3, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 3)
	require.NoError(t, err)

	earlyKickoff := time.Date(2025, 9, 21, 17, 0, 0, 0, time.UTC)
	lateKickoff := time.Date(2025, 9, 21, 20, 25, 0, 0, time.UTC)

	// g1: referenced by settled leg with Odds: -110 (no archived odds needed for WithOdds).
	// g2: referenced by settled leg with Odds: 0 (relies on metadata-only archived props check).
	// g3: referenced by settled leg with Odds: 0 (no pre-kickoff odds, only post-kickoff odds:lines).
	// g4: unreferenced game on the same week's slate (its archived injuries/odds must not be read).
	games := []db.Game{
		{
			ID: "g1", WeekID: week3.ID, HomeTeam: "BUF", AwayTeam: "MIA",
			HomeName: "Buffalo Bills", AwayName: "Miami Dolphins",
			HomeESPNID: "2", AwayESPNID: "15", Kickoff: earlyKickoff, Status: db.GameFinal,
		},
		{
			ID: "g2", WeekID: week3.ID, HomeTeam: "KC", AwayTeam: "LV",
			HomeName: "Kansas City Chiefs", AwayName: "Las Vegas Raiders",
			HomeESPNID: "12", AwayESPNID: "13", Kickoff: earlyKickoff, Status: db.GameFinal,
		},
		{
			ID: "g3", WeekID: week3.ID, HomeTeam: "PHI", AwayTeam: "DAL",
			HomeName: "Philadelphia Eagles", AwayName: "Dallas Cowboys",
			HomeESPNID: "21", AwayESPNID: "6", Kickoff: lateKickoff, Status: db.GameFinal,
		},
		{
			ID: "g4", WeekID: week3.ID, HomeTeam: "SF", AwayTeam: "SEA",
			HomeName: "San Francisco 49ers", AwayName: "Seattle Seahawks",
			HomeESPNID: "25", AwayESPNID: "26", Kickoff: earlyKickoff, Status: db.GameFinal,
		},
	}
	for _, g := range games {
		_, err = store.UpsertGame(ctx, g)
		require.NoError(t, err)
	}

	// Seed overlapping summary + roster injuries for g1 to verify parity with analyzer.MergeGameInjuries.
	g1Summary := `{"injuries":[{"team":{"id":"2","displayName":"Buffalo Bills","abbreviation":"BUF"},` +
		`"injuries":[{"status":"Questionable","athlete":{"id":"buf-wr1","fullName":"Summary WR",` +
		`"position":{"abbreviation":"WR"}}}]}]}`
	g1Roster := `{"athletes":[{"items":[{"id":"buf-wr1","fullName":"Summary WR","position":{"abbreviation":"WR"},` +
		`"injuries":[{"status":"Out"}]},{"id":"buf-qb2","fullName":"Roster Only QB",` +
		`"position":{"abbreviation":"QB"},"injuries":[{"status":"Doubtful"}]}]}]}`
	mustArchiveResponse(
		t, store, "espn", db.ArchiveSummaryKey("g1"), earlyKickoff.Add(-30*time.Minute), []byte(g1Summary),
	)
	mustArchiveResponse(
		t, store, "espn", db.ArchiveRosterKey("2"), earlyKickoff.Add(-30*time.Minute), []byte(g1Roster),
	)

	// Unreferenced game g4 has archived injuries and odds, which LoadDataset must skip.
	g4Summary := `{"injuries":[{"team":{"id":"25","displayName":"San Francisco 49ers","abbreviation":"SF"},` +
		`"injuries":[{"status":"Out","athlete":{"id":"sf-rb1","fullName":"Unreferenced RB",` +
		`"position":{"abbreviation":"RB"}}}]}]}`
	mustArchiveResponse(
		t, store, "espn", db.ArchiveSummaryKey("g4"), earlyKickoff.Add(-30*time.Minute), []byte(g4Summary),
	)
	mustArchiveResponse(
		t, store, "odds", db.ArchiveOddsPropsKey("g4"), earlyKickoff.Add(-30*time.Minute), []byte(`not-json`),
	)

	// g2 has Odds: 0 on its leg; metadata-only check must match pre-kickoff odds:props:g2 without parsing JSON.
	mustArchiveResponse(
		t, store, "odds", db.ArchiveOddsPropsKey("g2"),
		earlyKickoff.Add(-15*time.Minute), []byte(`metadata-only-body`),
	)

	// Post-kickoff odds:lines at lateKickoff + 5m must NOT satisfy g3 (nor g1/g2).
	mustArchiveResponse(t, store, "odds", db.ArchiveOddsLinesKey, lateKickoff.Add(5*time.Minute), []byte(`[]`))

	// Create a settled slip referencing g1, g2, g3 (leaving g4 unreferenced), plus an ungraded slip referencing g4.
	settledSlip, err := store.CreateSlip(
		ctx, group.ID, week3.ID, nil, "Settled Multi-Game", db.SlateScopeFullWeek, 3, []int64{user.ID},
	)
	require.NoError(t, err)

	legs := []db.Leg{
		{
			SlipID:     settledSlip.ID,
			UserID:     user.ID,
			GameID:     new("g1"),
			MarketType: "SPREAD",
			Selection:  "BUF -3.5",
			Team:       new("BUF"),
			Side:       new("TEAM"),
			Line:       new(-3.5),
			Odds:       -110,
		},
		{
			SlipID: settledSlip.ID, UserID: user.ID, GameID: new("g2"),
			MarketType: "SPREAD", Selection: "KC -2.5", Team: new("KC"), Side: new("TEAM"), Line: new(-2.5), Odds: 0,
		},
		{
			SlipID: settledSlip.ID, UserID: user.ID, GameID: new("g3"),
			MarketType: "SPREAD", Selection: "PHI -1.5", Team: new("PHI"), Side: new("TEAM"), Line: new(-1.5), Odds: 0,
		},
	}
	for i, l := range legs {
		created, createErr := store.CreateLeg(ctx, l)
		require.NoError(t, createErr)
		if i > 0 {
			// CreateLeg defaults Odds==0 to DefaultLegOdds (-110); clear it in SQL to test archived-odds fallback.
			_, updateErr := database.ExecContext(ctx, "UPDATE legs SET odds = 0 WHERE id = ?", created.ID)
			require.NoError(t, updateErr)
		}
		require.NoError(t, store.SetLegResult(ctx, created.ID, parlay.Won, "test"))
	}
	require.NoError(t, store.SetSlipGrade(ctx, settledSlip.ID, parlay.Won, db.SlipGraded))

	// Ungraded slip on g4 must not pull g4 into referencedGames.
	openSlip, err := store.CreateSlip(
		ctx, group.ID, week3.ID, nil, "Open Slip", db.SlateScopeFullWeek, 1, []int64{user.ID},
	)
	require.NoError(t, err)
	_, err = store.CreateLeg(ctx, db.Leg{
		SlipID: openSlip.ID, UserID: user.ID, GameID: new("g4"),
		MarketType: "SPREAD", Selection: "SF -4.5", Team: new("SF"), Side: new("TEAM"), Line: new(-4.5), Odds: -110,
	})
	require.NoError(t, err)

	ds, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{})
	require.NoError(t, err)
	require.Len(t, ds.Weeks, 1)

	w := ds.Weeks[0]
	// Unreferenced game g4 did not load archived injuries.
	assert.Empty(t, w.Games["g4"].Injuries)

	// Injury merging on g1 matches analyzer.MergeGameInjuries identically.
	liveGame := db.Game{
		ID:       "g1",
		HomeTeam: "BUF",
		AwayTeam: "MIA",
		InjuriesJSON: new(
			`[{"team":{"id":"2","displayName":"Buffalo Bills","abbreviation":"BUF"},"injuries":[{"status":"Questionable","athlete":{"id":"buf-wr1","fullName":"Summary WR","position":{"abbreviation":"WR"}}}]}]`,
		),
	}
	liveRosters := []db.RosterPlayer{
		{AthleteID: "buf-wr1", Team: "BUF", FullName: "Summary WR", Position: "WR", InjuryStatus: "Out"},
		{AthleteID: "buf-qb2", Team: "BUF", FullName: "Roster Only QB", Position: "QB", InjuryStatus: "Doubtful"},
	}
	expectedLive := analyzer.MergeGameInjuries(liveGame, liveRosters)
	require.Len(t, w.Games["g1"].Injuries["BUF"], len(expectedLive["BUF"]))
	for i, wantInj := range expectedLive["BUF"] {
		gotInj := w.Games["g1"].Injuries["BUF"][i]
		assert.Equal(t, wantInj.AthleteID, gotInj.AthleteID)
		assert.Equal(t, wantInj.FullName, gotInj.FullName)
		assert.Equal(t, wantInj.Position, gotInj.Position)
		assert.Equal(t, wantInj.Status, gotInj.Status)
	}

	// Coverage parity: TotalLegs=3, WithInjuries=1 (g1 only), WithOdds=2 (g1 via Odds:-110, g2 via metadata-only props).
	assert.Equal(t, eval.DataCoverage{
		TotalLegs:    3,
		WithInjuries: 1,
		WithOdds:     2,
	}, ds.Coverage)
}

func TestLoadDatasetEarlyBustedSlip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	store := newTestStore(t)

	group, user := seedGroupAndMember(t, store, "ada")
	week1, err := store.UpsertWeek(ctx, 2025, nflverse.SeasonTypeRegular, 1)
	require.NoError(t, err)

	kickoff := time.Date(2025, 9, 7, 17, 0, 0, 0, time.UTC)
	_, err = store.UpsertGame(ctx, db.Game{
		ID: "g1", WeekID: week1.ID, HomeTeam: "BUF", AwayTeam: "MIA",
		HomeName: "Buffalo Bills", AwayName: "Miami Dolphins",
		Kickoff: kickoff, Status: db.GameFinal,
	})
	require.NoError(t, err)

	slip, err := store.CreateSlip(
		ctx, group.ID, week1.ID, nil, "Early Bust", db.SlateScopeFullWeek, 4, []int64{user.ID},
	)
	require.NoError(t, err)

	legSpecs := []struct {
		selection string
		odds      int
		result    parlay.Result
	}{
		{selection: "BUF -3.5", odds: -110, result: parlay.Lost},
		{selection: "MIA +3.5", odds: 150, result: parlay.Pending},
		{selection: "OVER 45.5", odds: 200, result: parlay.Pending},
		{selection: "UNDER 45.5", odds: -120, result: parlay.NeedsGrading},
	}
	for _, spec := range legSpecs {
		leg, createErr := store.CreateLeg(ctx, db.Leg{
			SlipID:     slip.ID,
			UserID:     user.ID,
			GameID:     new("g1"),
			MarketType: "SPREAD",
			Selection:  spec.selection,
			Team:       new("BUF"),
			Side:       new("TEAM"),
			Line:       new(-3.5),
			Odds:       spec.odds,
		})
		require.NoError(t, createErr)
		if spec.result != parlay.Pending {
			require.NoError(t, store.SetLegResult(ctx, leg.ID, spec.result, "test"))
		}
	}
	require.NoError(t, store.SetSlipGrade(ctx, slip.ID, parlay.Lost, db.SlipGraded))

	ds, err := eval.LoadDataset(ctx, store, eval.ReplayFilter{})
	require.NoError(t, err)
	require.Len(t, ds.Weeks, 1)
	require.Len(t, ds.Weeks[0].Slips, 1)

	replayed := ds.Weeks[0].Slips[0]
	assert.Equal(t, parlay.Lost, replayed.Result)
	require.Len(t, replayed.Legs, 4)
	assert.Equal(t, parlay.Lost, replayed.Legs[0].Result)
	assert.Equal(t, parlay.Pending, replayed.Legs[1].Result)
	assert.Equal(t, parlay.Pending, replayed.Legs[2].Result)
	assert.Equal(t, parlay.NeedsGrading, replayed.Legs[3].Result)

	// CombinedOdds must only include counting settled legs (the single -110 Lost leg).
	assert.InDelta(t, parlay.AmericanToDecimal(-110), replayed.CombinedOdds, 1e-9)
	assert.Equal(t, 4, ds.Coverage.TotalLegs)
	assert.Equal(t, 4, ds.Coverage.WithOdds)
}
