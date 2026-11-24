package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestNormalizePlayerName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		// Suffixes
		{name: "jr with period", input: "Marvin Harrison Jr.", expected: "marvin harrison"},
		{name: "jr lowercase without period", input: "marvin harrison jr", expected: "marvin harrison"},
		{name: "sr with period", input: "Frank Gore Sr.", expected: "frank gore"},
		{name: "suffix ii", input: "Patrick Mahomes II", expected: "patrick mahomes"},
		{name: "suffix iii", input: "Kenneth Walker III", expected: "kenneth walker"},
		{name: "suffix iv", input: "John Doe IV", expected: "john doe"},
		{name: "suffix v", input: "John Doe V", expected: "john doe"},
		{name: "suffix preceded by spaces and comma", input: "Travis Etienne, Jr.", expected: "travis etienne"},
		{name: "suffix word without preceding space", input: "Junior", expected: "junior"},
		{name: "solo suffix", input: "Jr", expected: "jr"},

		// Punctuation
		{name: "periods in initials", input: "A.J. Brown", expected: "aj brown"},
		{name: "already stripped initials", input: "AJ Brown", expected: "aj brown"},
		{name: "apostrophe", input: "De'Von Achane", expected: "devon achane"},
		{name: "hyphen and period", input: "Amon-Ra St. Brown", expected: "amonra st brown"},
		{name: "hyphen without period", input: "Amon-Ra St Brown", expected: "amonra st brown"},
		{name: "dk metcalf", input: "D.K. Metcalf", expected: "dk metcalf"},
		{name: "tutu atwell with quotes", input: "Chatarius Tutu Atwell", expected: "chatarius tutu atwell"},

		// Whitespace and case
		{name: "excess whitespace and uppercase", input: "  JOSH   ALLEN  ", expected: "josh allen"},
		{name: "blank", input: "   ", expected: ""},
		{name: "empty", input: "", expected: ""},

		// Trailing parenthetical tags
		{name: "team tag with suffix", input: "Ollie Gordon II (MIA)", expected: "ollie gordon"},
		{name: "team tag without suffix", input: "Ollie Gordon (MIA)", expected: "ollie gordon"},
		{name: "team tag LAR", input: "Cooper Kupp (LAR)", expected: "cooper kupp"},
		{name: "team tag 2 letters", input: "Drake Maye (NE)", expected: "drake maye"},

		// Accents
		{name: "accented first name", input: "José Alvarado", expected: "jose alvarado"},
		{name: "accented spanish name", input: "Álvaro Núñez", expected: "alvaro nunez"},
		{name: "accented with tag and suffix", input: "José Ramírez Jr. (CLE)", expected: "jose ramirez"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, db.NormalizePlayerName(tc.input))
		})
	}
}

func TestStripPlayerTag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		input        string
		wantCleaned  string
		wantTeamHint string
	}{
		{
			name:         "3-letter team tag",
			input:        "Ollie Gordon II (MIA)",
			wantCleaned:  "Ollie Gordon II",
			wantTeamHint: "MIA",
		},
		{name: "3-letter team tag LAR", input: "Cooper Kupp (LAR)", wantCleaned: "Cooper Kupp", wantTeamHint: "LAR"},
		{name: "2-letter team tag", input: "Drake Maye (NE)", wantCleaned: "Drake Maye", wantTeamHint: "NE"},
		{name: "4-letter uppercase tag", input: "Player Name (TEAM)", wantCleaned: "Player Name", wantTeamHint: "TEAM"},
		{name: "extra whitespace", input: "  Josh Allen (BUF)  ", wantCleaned: "Josh Allen", wantTeamHint: "BUF"},
		{
			name:         "lowercase tag stripped without hint",
			input:        "Ollie Gordon (mia)",
			wantCleaned:  "Ollie Gordon",
			wantTeamHint: "",
		},
		{
			name:         "status tag stripped without hint",
			input:        "Ollie Gordon (Out)",
			wantCleaned:  "Ollie Gordon",
			wantTeamHint: "",
		},
		{
			name:         "single letter tag stripped without hint",
			input:        "Ollie Gordon (A)",
			wantCleaned:  "Ollie Gordon",
			wantTeamHint: "",
		},
		{
			name:         "5 letter tag stripped without hint",
			input:        "Ollie Gordon (ABCDE)",
			wantCleaned:  "Ollie Gordon",
			wantTeamHint: "",
		},
		{name: "no tag", input: "Ollie Gordon II", wantCleaned: "Ollie Gordon II", wantTeamHint: ""},
		{name: "only tag", input: "(MIA)", wantCleaned: "(MIA)", wantTeamHint: ""},
		{name: "blank", input: "   ", wantCleaned: "", wantTeamHint: ""},
		{name: "empty", input: "", wantCleaned: "", wantTeamHint: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cleaned, hint := db.StripPlayerTag(tc.input)
			assert.Equal(t, tc.wantCleaned, cleaned)
			assert.Equal(t, tc.wantTeamHint, hint)
		})
	}
}

func TestRosterPlayers(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)

	players := []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Josh Allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png",
			UpdatedAt:   now,
		},
		{
			AthleteID:    "4241463",
			Team:         "BUF",
			ESPNTeamID:   "2",
			FullName:     "James Cook",
			Position:     "RB",
			InjuryStatus: "Out",
			InjuryDate:   "2026-11-08T15:30Z",
			UpdatedAt:    now,
		},
		{
			AthleteID:  "4361579",
			Team:       "MIA",
			ESPNTeamID: "15",
			FullName:   "Tua Tagovailoa",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}

	require.NoError(t, store.UpsertRosterPlayers(t.Context(), players))

	// RosterPlayerByID
	player, err := store.RosterPlayerByID(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, "Josh Allen", player.FullName)
	assert.Equal(t, "josh allen", player.NormName)
	assert.Equal(t, "BUF", player.Team)
	assert.Equal(t, "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png", player.HeadshotURL)

	// RosterPlayersByIDs
	emptyByIDs, err := store.RosterPlayersByIDs(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, emptyByIDs)

	byIDs, err := store.RosterPlayersByIDs(t.Context(), []string{"3918298", "4361579", "missing"})
	require.NoError(t, err)
	require.Len(t, byIDs, 2)
	assert.Equal(t, "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png", byIDs["3918298"].HeadshotURL)
	assert.Empty(t, byIDs["4361579"].HeadshotURL)

	_, err = store.RosterPlayerByID(t.Context(), "missing")
	require.ErrorIs(t, err, db.ErrPlayerNotFound)

	// RosterForTeam
	bufRoster, err := store.RosterForTeam(t.Context(), "BUF")
	require.NoError(t, err)
	assert.Len(t, bufRoster, 2)
	assert.Equal(t, "James Cook", bufRoster[0].FullName)
	assert.Equal(t, "Out", bufRoster[0].InjuryStatus)
	assert.Equal(t, "2026-11-08T15:30Z", bufRoster[0].InjuryDate)
	assert.Equal(t, "Josh Allen", bufRoster[1].FullName)
	assert.Empty(t, bufRoster[1].InjuryStatus)

	// RosterForTeams
	bothRosters, err := store.RosterForTeams(t.Context(), []string{"BUF", "MIA"})
	require.NoError(t, err)
	assert.Len(t, bothRosters, 3)

	emptyRosters, err := store.RosterForTeams(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, emptyRosters)

	// Upsert update and clearing injury status when player returns to healthy
	players[0].Position = "WR" // traded or test change
	players[0].UpdatedAt = now.Add(time.Hour)
	players[1].InjuryStatus = ""
	players[1].InjuryDate = ""
	players[1].UpdatedAt = now.Add(time.Hour)
	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{players[0], players[1]}))

	updated, err := store.RosterPlayerByID(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, "WR", updated.Position)

	cleared, err := store.RosterPlayerByID(t.Context(), "4241463")
	require.NoError(t, err)
	assert.Empty(t, cleared.InjuryStatus)
	assert.Empty(t, cleared.InjuryDate)
}

func TestPlayerGameLogs(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)

	log := db.PlayerGameLog{
		AthleteID:   "3918298",
		GamesPlayed: 8,
		StatsJSON:   `{"PASS_YDS":[250.0,310.0,190.0]}`,
		GamesJSON:   `[{"eventId":"401","week":9,"opponent":"BAL","atVs":"vs","result":"W","score":"31-24"}]`,
		UpdatedAt:   now,
	}

	require.NoError(t, store.UpsertPlayerGameLog(t.Context(), log))

	// Get single
	fetched, err := store.PlayerGameLog(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, 8, fetched.GamesPlayed)
	assert.JSONEq(t, `{"PASS_YDS":[250.0,310.0,190.0]}`, fetched.StatsJSON)
	assert.JSONEq(
		t,
		`[{"eventId":"401","week":9,"opponent":"BAL","atVs":"vs","result":"W","score":"31-24"}]`,
		fetched.GamesJSON,
	)

	_, err = store.PlayerGameLog(t.Context(), "missing")
	require.ErrorIs(t, err, db.ErrGameLogNotFound)

	// Batch get (empty GamesJSON defaults to [])
	log2 := db.PlayerGameLog{
		AthleteID:   "4361579",
		GamesPlayed: 7,
		StatsJSON:   `{"PASS_YDS":[210.0,240.0]}`,
		UpdatedAt:   now,
	}
	require.NoError(t, store.UpsertPlayerGameLog(t.Context(), log2))

	batch, err := store.PlayerGameLogs(t.Context(), []string{"3918298", "4361579", "missing"})
	require.NoError(t, err)
	assert.Len(t, batch, 2)
	assert.Equal(t, 8, batch["3918298"].GamesPlayed)
	assert.Equal(t, 7, batch["4361579"].GamesPlayed)
	assert.Equal(t, "[]", batch["4361579"].GamesJSON)

	emptyBatch, err := store.PlayerGameLogs(t.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, emptyBatch)

	// Update existing
	log.GamesPlayed = 9
	log.UpdatedAt = now.Add(2 * time.Hour)
	require.NoError(t, store.UpsertPlayerGameLog(t.Context(), log))
	fetched, err = store.PlayerGameLog(t.Context(), "3918298")
	require.NoError(t, err)
	assert.Equal(t, 9, fetched.GamesPlayed)
}

func TestResolvePlayerMatch(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)

	// Setup rosters:
	// BUF: Josh Allen (QB, 3918298), A.J. Brown (WR, 4047646)
	// MIA: De'Von Achane (RB, 4429013), Chris Jones (DT, 11111)
	// KC:  Chris Jones (DT, 22222)
	// JAX: Josh Allen (DE, 4035687)
	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{AthleteID: "3918298", Team: "BUF", ESPNTeamID: "2", FullName: "Josh Allen", Position: "QB", UpdatedAt: now},
		{AthleteID: "4047646", Team: "BUF", ESPNTeamID: "2", FullName: "A.J. Brown", Position: "WR", UpdatedAt: now},
		{
			AthleteID:  "4429013",
			Team:       "MIA",
			ESPNTeamID: "15",
			FullName:   "De'Von Achane",
			Position:   "RB",
			UpdatedAt:  now,
		},
		{AthleteID: "11111", Team: "MIA", ESPNTeamID: "15", FullName: "Chris Jones", Position: "DT", UpdatedAt: now},
		{AthleteID: "22222", Team: "KC", ESPNTeamID: "12", FullName: "Chris Jones", Position: "DT", UpdatedAt: now},
		{AthleteID: "4035687", Team: "JAX", ESPNTeamID: "30", FullName: "Josh Allen", Position: "DE", UpdatedAt: now},
	}))

	// 1. Empty name returns empty
	athleteID, team, err := store.ResolvePlayerMatch(t.Context(), "", "BUF", "MIA")
	require.NoError(t, err)
	assert.Empty(t, athleteID)
	assert.Empty(t, team)

	// 2. Unambiguous match across game teams when another team has same-name player
	// Game BUF vs MIA: Josh Allen should match BUF's Josh Allen, ignoring JAX's Josh Allen.
	athleteID, team, err = store.ResolvePlayerMatch(t.Context(), "Josh Allen", "BUF", "MIA")
	require.NoError(t, err)
	assert.Equal(t, "3918298", athleteID)
	assert.Equal(t, "BUF", team)

	// Match is now cached in player_matches with manual = 0.
	// Verify second lookup succeeds via cache:
	athleteID, team, err = store.ResolvePlayerMatch(t.Context(), "Josh Allen", "BUF", "MIA")
	require.NoError(t, err)
	assert.Equal(t, "3918298", athleteID)
	assert.Equal(t, "BUF", team)

	// 3. Ambiguous same-name players on the game's teams:
	// Game MIA vs KC: Both MIA and KC have a "Chris Jones".
	athleteID, team, err = store.ResolvePlayerMatch(t.Context(), "Chris Jones", "MIA", "KC")
	require.NoError(t, err)
	assert.Empty(t, athleteID, "ambiguous matchup must leave player_id empty")
	assert.Empty(t, team)

	// 4. Punctuation match: "AJ Brown" matches "A.J. Brown"
	athleteID, team, err = store.ResolvePlayerMatch(t.Context(), "AJ Brown", "BUF", "MIA")
	require.NoError(t, err)
	assert.Equal(t, "4047646", athleteID)
	assert.Equal(t, "BUF", team)

	// 5. Punctuation match: "Devon Achane" matches "De'Von Achane"
	athleteID, team, err = store.ResolvePlayerMatch(t.Context(), "Devon Achane", "BUF", "MIA")
	require.NoError(t, err)
	assert.Equal(t, "4429013", athleteID)
	assert.Equal(t, "MIA", team)

	// 6. Manual match persistence:
	// If a match is manually set, auto-match never overwrites it.
	require.NoError(t, store.SaveManualPlayerMatch(t.Context(), "Chris Jones", "MIA", "11111"))
	athleteID, team, err = store.ResolvePlayerMatch(t.Context(), "Chris Jones", "MIA", "KC")
	require.NoError(t, err)
	assert.Equal(t, "11111", athleteID)
	assert.Equal(t, "MIA", team)
}

func TestSaveManualPlayerMatch(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 9)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week)) // BUF vs MIA
	require.NoError(t, err)

	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)

	// Insert an existing auto-matched row for the opponent team (MIA) that conflicts
	// Plus an existing match for a 3rd team (JAX) that should not be touched
	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{AthleteID: "3918298", Team: "BUF", ESPNTeamID: "2", FullName: "Josh Allen", Position: "QB", UpdatedAt: now},
		{AthleteID: "99999", Team: "MIA", ESPNTeamID: "15", FullName: "Josh Allen", Position: "QB", UpdatedAt: now},
		{AthleteID: "4035687", Team: "JAX", ESPNTeamID: "30", FullName: "Josh Allen", Position: "DE", UpdatedAt: now},
	}))

	// Simulate auto-matches
	_, _, err = store.ResolvePlayerMatch(t.Context(), "Josh Allen", "MIA", "IND") // caches Josh Allen -> MIA (manual=0)
	require.NoError(t, err)
	_, _, err = store.ResolvePlayerMatch(t.Context(), "Josh Allen", "JAX", "IND") // caches Josh Allen -> JAX (manual=0)
	require.NoError(t, err)

	// Insert odds with empty team and sportsbook player_name
	oddsRows := []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -115,
			FetchedAt:  now,
		},
	}
	_, err = store.UpsertOdds(t.Context(), oddsRows)
	require.NoError(t, err)

	// Insert leg with player_name but empty team and player_id
	user := createUser(t, store, "Tester")
	group := createTestGroup(t, store, user)
	slip := createTestSlip(t, store, group, week, "Slip 1", 2, user)

	leg, err := store.CreateLeg(t.Context(), db.Leg{
		SlipID:     slip.ID,
		UserID:     user.ID,
		GameID:     new(game.ID),
		MarketType: "PASS_YDS",
		Selection:  "Josh Allen Over 265.5 Pass Yds",
		PlayerName: new("Josh Allen"),
		Side:       new("OVER"),
		Line:       new(265.5),
		Odds:       -115,
	})
	require.NoError(t, err)
	assert.Nil(t, leg.Team)
	assert.Nil(t, leg.PlayerID)

	// Save manual match for BUF
	require.NoError(t, store.SaveManualPlayerMatch(t.Context(), "Josh Allen", "BUF", "3918298"))

	// 1. player_matches has manual = 1 for BUF
	matches, err := store.PlayerMatchesForTeams(t.Context(), []string{"BUF", "MIA", "JAX"})
	require.NoError(t, err)
	var bufMatch, miaMatch, jaxMatch *db.PlayerMatch
	for i := range matches {
		m := &matches[i]
		if m.OddsName == "josh allen" {
			switch m.Team {
			case "BUF":
				bufMatch = m
			case "MIA":
				miaMatch = m
			case "JAX":
				jaxMatch = m
			}
		}
	}
	require.NotNil(t, bufMatch)
	assert.True(t, bufMatch.Manual)
	assert.Equal(t, "3918298", bufMatch.ESPNAthleteID)

	// Conflicting non-manual row for the opponent team in this game (MIA) was removed!
	assert.Nil(t, miaMatch, "conflicting non-manual row for opponent team must be deleted")

	// 3rd team (JAX) not in this matchup was preserved!
	assert.NotNil(t, jaxMatch, "match on third team not in matchup must be preserved")

	// 2. Backfilled odds row
	oddsList, err := store.OddsForGame(t.Context(), game.ID)
	require.NoError(t, err)
	require.Len(t, oddsList, 1)
	assert.Equal(t, "BUF", oddsList[0].Team)

	// 3. Backfilled leg row
	slipLegs, err := store.LegsForSlip(t.Context(), slip.ID)
	require.NoError(t, err)
	require.Len(t, slipLegs, 1)
	assert.Equal(t, new("BUF"), slipLegs[0].Team)
	assert.Equal(t, new("3918298"), slipLegs[0].PlayerID)
}

func TestWeekPlayerNames(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 10)
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)

	// No games in week
	names, err := store.WeekPlayerNames(t.Context(), week.ID)
	require.NoError(t, err)
	assert.Empty(t, names)

	game, err := store.UpsertGame(t.Context(), testSlateGame(week)) // BUF vs MIA
	require.NoError(t, err)

	// Add roster players
	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{
			AthleteID:   "3918298",
			Team:        "BUF",
			ESPNTeamID:  "2",
			FullName:    "Josh Allen",
			Position:    "QB",
			HeadshotURL: "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png",
			UpdatedAt:   now,
		},
		{
			AthleteID:  "4429013",
			Team:       "MIA",
			ESPNTeamID: "15",
			FullName:   "De'Von Achane",
			Position:   "RB",
			UpdatedAt:  now,
		},
		{
			AthleteID:  "4047646",
			Team:       "KC",
			ESPNTeamID: "12",
			FullName:   "Patrick Mahomes",
			Position:   "QB",
			UpdatedAt:  now,
		},
	}))

	// Add prop odds for the game with another player not on roster
	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen", // duplicate with roster
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -115,
			FetchedAt:  now,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Dalton Kincaid", // in odds, not in roster
			Side:       "OVER",
			Line:       new(45.5),
			Price:      -110,
			FetchedAt:  now,
		},
	})
	require.NoError(t, err)

	names, err = store.WeekPlayerNames(t.Context(), week.ID)
	require.NoError(t, err)
	// Expected: sorted and deduplicated, only for BUF and MIA teams/game
	assert.Equal(t, []string{"Dalton Kincaid", "De'Von Achane", "Josh Allen"}, names)
}

func TestLatestPropForPlayer(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 10)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)
	now := time.Now().UTC()

	// 1. No props stored -> returns false, nil
	_, ok, err := store.LatestPropForPlayer(t.Context(), "3918298", "Josh Allen", "BUF")
	require.NoError(t, err)
	assert.False(t, ok)

	// 2. Insert older and newer props
	older := now.Add(-1 * time.Hour)
	newer := now
	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(260.5),
			Price:      -110,
			FetchedAt:  older,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "PASS_YDS",
			Team:       "BUF",
			PlayerName: "Josh Allen",
			Side:       "OVER",
			Line:       new(265.5),
			Price:      -115,
			FetchedAt:  newer,
		},
	})
	require.NoError(t, err)

	prop, ok, err := store.LatestPropForPlayer(t.Context(), "3918298", "Josh Allen", "BUF")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.InDelta(t, 265.5, *prop.Line, 0.001)

	// 3. Match via player_matches
	require.NoError(t, store.SaveManualPlayerMatch(t.Context(), "Gabe Davis", "BUF", "4241479"))
	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "BUF",
			PlayerName: "Gabe Davis",
			Side:       "OVER",
			Line:       new(42.5),
			Price:      -110,
			FetchedAt:  newer,
		},
	})
	require.NoError(t, err)

	prop, ok, err = store.LatestPropForPlayer(t.Context(), "4241479", "Gabriel Davis", "BUF")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.InDelta(t, 42.5, *prop.Line, 0.001)
}

func TestLatestPropForPlayerMarket(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	week := createTestWeek(t, store, 10)
	game, err := store.UpsertGame(t.Context(), testSlateGame(week))
	require.NoError(t, err)
	now := time.Now().UTC()
	older := now.Add(-1 * time.Hour)
	newer := now

	require.NoError(t, store.SaveManualPlayerMatch(t.Context(), "Ja'Marr Chase", "CIN", "4362628"))

	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RECEPTIONS",
			Team:       "CIN",
			PlayerName: "Ja'Marr Chase",
			Side:       "OVER",
			Line:       new(5.5),
			Price:      -115,
			FetchedAt:  older,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "REC_YDS",
			Team:       "CIN",
			PlayerName: "Ja'Marr Chase",
			Side:       "OVER",
			Line:       new(68.5),
			Price:      -110,
			FetchedAt:  older,
		},
		{
			GameID:     game.ID,
			Book:       "fanduel",
			MarketType: "REC_YDS",
			Team:       "CIN",
			PlayerName: "Ja'Marr Chase",
			Side:       "OVER",
			Line:       new(72.5),
			Price:      -112,
			FetchedAt:  newer,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "ANYTIME_TD",
			Team:       "CIN",
			PlayerName: "Ja'Marr Chase",
			Side:       "YES",
			Line:       nil,
			Price:      +105,
			FetchedAt:  newer,
		},
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RUSH_YDS",
			Team:       "BUF",
			PlayerName: "Ja'Marr Chase",
			Side:       "OVER",
			Line:       new(12.5),
			Price:      -110,
			FetchedAt:  newer,
		},
	})
	require.NoError(t, err)

	// Empty market falls back to newest prop across all markets.
	prop, ok, err := store.LatestPropForPlayerMarket(t.Context(), "4362628", "Ja'Marr Chase", "CIN", "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "REC_YDS", prop.MarketType)
	assert.InDelta(t, 72.5, *prop.Line, 0.001)

	// Specific market returns that market's line without bleeding from newer REC_YDS.
	prop, ok, err = store.LatestPropForPlayerMarket(t.Context(), "4362628", "Ja'Marr Chase", "CIN", "RECEPTIONS")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "RECEPTIONS", prop.MarketType)
	assert.InDelta(t, 5.5, *prop.Line, 0.001)

	// Specific market returns newest row for that market.
	prop, ok, err = store.LatestPropForPlayerMarket(t.Context(), "4362628", "Ja'Marr Chase", "CIN", "REC_YDS")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "REC_YDS", prop.MarketType)
	assert.InDelta(t, 72.5, *prop.Line, 0.001)

	// Match via player_matches when caller's display name differs from odds name.
	require.NoError(t, store.SaveManualPlayerMatch(t.Context(), "Gabe Davis", "CIN", "4241479"))
	_, err = store.UpsertOdds(t.Context(), []db.Odds{
		{
			GameID:     game.ID,
			Book:       "draftkings",
			MarketType: "RECEPTIONS",
			Team:       "CIN",
			PlayerName: "Gabe Davis",
			Side:       "OVER",
			Line:       new(3.5),
			Price:      -110,
			FetchedAt:  newer,
		},
	})
	require.NoError(t, err)
	prop, ok, err = store.LatestPropForPlayerMarket(t.Context(), "4241479", "Gabriel Davis", "CIN", "RECEPTIONS")
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, 3.5, *prop.Line, 0.001)

	// Market with no line (ANYTIME_TD) returns false.
	_, ok, err = store.LatestPropForPlayerMarket(t.Context(), "4362628", "Ja'Marr Chase", "CIN", "ANYTIME_TD")
	require.NoError(t, err)
	assert.False(t, ok)

	// Market belonging to a different team is excluded when team is specified.
	_, ok, err = store.LatestPropForPlayerMarket(t.Context(), "4362628", "Ja'Marr Chase", "CIN", "RUSH_YDS")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestUpsertPlayerMatches(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	// Empty slice is a no-op.
	require.NoError(t, store.UpsertPlayerMatches(t.Context(), nil))

	// Seed a manual match.
	require.NoError(t, store.SaveManualPlayerMatch(t.Context(), "Ollie Gordon II", "MIA", "5123456"))

	// Batch upsert: attempts to overwrite the manual match with manual=0, plus adds a new match.
	err := store.UpsertPlayerMatches(t.Context(), []db.PlayerMatch{
		{OddsName: "ollie gordon", Team: "MIA", ESPNAthleteID: "9999999", Manual: false},
		{OddsName: "josh allen", Team: "BUF", ESPNAthleteID: "3918298", Manual: false},
	})
	require.NoError(t, err)

	matches, err := store.PlayerMatchesForTeams(t.Context(), []string{"MIA", "BUF"})
	require.NoError(t, err)
	require.Len(t, matches, 2)

	byTeam := make(map[string]db.PlayerMatch, 2)
	for _, m := range matches {
		byTeam[m.Team] = m
	}
	assert.Equal(t, "5123456", byTeam["MIA"].ESPNAthleteID, "manual=1 match must not be overwritten")
	assert.True(t, byTeam["MIA"].Manual)
	assert.Equal(t, "3918298", byTeam["BUF"].ESPNAthleteID)
	assert.False(t, byTeam["BUF"].Manual)
}

func TestBatchPlayerResolverOpposingTeamDisambiguation(t *testing.T) {
	t.Parallel()

	t.Run("non-manual cache match refused when untagged and multiple roster players share norm", func(t *testing.T) {
		t.Parallel()

		resolver := db.NewBatchPlayerResolver(
			[]db.RosterPlayer{
				{AthleteID: "1001", Team: "BUF", FullName: "Josh Allen"},
				{AthleteID: "2002", Team: "MIA", FullName: "Josh Allen"},
			},
			[]db.PlayerMatch{
				{OddsName: "josh allen", Team: "MIA", ESPNAthleteID: "2002", Manual: false},
			},
		)

		// Tagged "(MIA)" still resolves to MIA, without writing a manual=0 match.
		hinted := resolver.ResolveGamePlayer("g1", "Josh Allen (MIA)", "", "BUF", "MIA")
		assert.True(t, hinted.Linked)
		assert.Equal(t, "MIA", hinted.Team)
		assert.Equal(t, "2002", hinted.AthleteID)

		// Untagged "Josh Allen" must stay unmatched despite the non-manual MIA cache row.
		untagged := resolver.ResolveGamePlayer("g1", "Josh Allen", "", "BUF", "MIA")
		assert.False(t, untagged.Linked)
		assert.Empty(t, untagged.Team)
		assert.Empty(t, untagged.AthleteID)
		assert.Empty(t, resolver.NewMatches(), "must not cache manual=0 when multiple roster players share norm")
	})

	t.Run("valid team hint or existing team is not overridden by single opposing candidate", func(t *testing.T) {
		t.Parallel()

		resolver := db.NewBatchPlayerResolver(
			[]db.RosterPlayer{
				{AthleteID: "1001", Team: "BUF", FullName: "Josh Allen"},
			},
			[]db.PlayerMatch{
				{OddsName: "josh allen", Team: "BUF", ESPNAthleteID: "1001", Manual: false},
			},
		)

		// Both cache and roster only have BUF, but existingTeam/hint is MIA.
		res := resolver.ResolveGamePlayer("g1", "Josh Allen", "MIA", "BUF", "MIA")
		assert.False(t, res.Linked)
		assert.Equal(t, "MIA", res.Team)
		assert.Empty(t, res.AthleteID)
	})

	t.Run("populates Position and HeadshotURL from roster and cache", func(t *testing.T) {
		t.Parallel()

		resolver := db.NewBatchPlayerResolver(
			[]db.RosterPlayer{
				{
					AthleteID:   "3918298",
					Team:        "BUF",
					FullName:    "Josh Allen",
					Position:    "QB",
					HeadshotURL: "https://example.com/josh.png",
				},
				{
					AthleteID:   "4429013",
					Team:        "MIA",
					FullName:    "De'Von Achane",
					Position:    "RB",
					HeadshotURL: "https://example.com/achane.png",
				},
			},
			[]db.PlayerMatch{
				{OddsName: "josh allen", Team: "BUF", ESPNAthleteID: "3918298", Manual: true},
			},
		)

		cached := resolver.ResolveGamePlayer("g1", "Josh Allen", "", "BUF", "MIA")
		assert.True(t, cached.Linked)
		assert.Equal(t, "QB", cached.Position)
		assert.Equal(t, "https://example.com/josh.png", cached.HeadshotURL)

		fromRoster := resolver.ResolveGamePlayer("g1", "De'Von Achane", "", "BUF", "MIA")
		assert.True(t, fromRoster.Linked)
		assert.Equal(t, "RB", fromRoster.Position)
		assert.Equal(t, "https://example.com/achane.png", fromRoster.HeadshotURL)
	})
}

func TestResolvePlayerMatchTeamHint(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	now := time.Date(2026, time.November, 9, 12, 0, 0, 0, time.UTC)

	require.NoError(t, store.UpsertRosterPlayers(t.Context(), []db.RosterPlayer{
		{AthleteID: "5001", Team: "BUF", ESPNTeamID: "2", FullName: "Ollie Gordon", Position: "RB", UpdatedAt: now},
		{AthleteID: "5002", Team: "MIA", ESPNTeamID: "15", FullName: "Ollie Gordon II", Position: "RB", UpdatedAt: now},
	}))

	// Untagged name is ambiguous across BUF and MIA.
	aid, team, err := store.ResolvePlayerMatch(t.Context(), "Ollie Gordon", "BUF", "MIA")
	require.NoError(t, err)
	assert.Empty(t, aid)
	assert.Empty(t, team)

	// Tagged name uses (MIA) team hint to resolve to MIA's Ollie Gordon II.
	aid, team, err = store.ResolvePlayerMatch(t.Context(), "Ollie Gordon II (MIA)", "BUF", "MIA")
	require.NoError(t, err)
	assert.Equal(t, "5002", aid)
	assert.Equal(t, "MIA", team)

	// Hinted resolution must not cache manual=0, so untagged lookups remain ambiguous.
	aid, team, err = store.ResolvePlayerMatch(t.Context(), "Ollie Gordon", "BUF", "MIA")
	require.NoError(t, err)
	assert.Empty(t, aid)
	assert.Empty(t, team)
}
