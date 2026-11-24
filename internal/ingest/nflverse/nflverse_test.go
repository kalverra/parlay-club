package nflverse_test

import (
	"bytes"
	"encoding/csv"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/nflverse"
)

// espnTeamCodes is ESPN's 32 NFL abbreviations.
var espnTeamCodes = map[string]struct{}{
	"ARI": {}, "ATL": {}, "BAL": {}, "BUF": {}, "CAR": {}, "CHI": {},
	"CIN": {}, "CLE": {}, "DAL": {}, "DEN": {}, "DET": {}, "GB": {},
	"HOU": {}, "IND": {}, "JAX": {}, "KC": {}, "LAC": {}, "LAR": {},
	"LV": {}, "MIA": {}, "MIN": {}, "NE": {}, "NO": {}, "NYG": {},
	"NYJ": {}, "PHI": {}, "PIT": {}, "SEA": {}, "SF": {}, "TB": {},
	"TEN": {}, "WSH": {},
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	return raw
}

func csvServer(t *testing.T, wantPath string, body []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, wantPath, r.URL.Path)
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func buildCSV(t *testing.T, header []string, rows ...[]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	require.NoError(t, w.Write(header))
	for _, row := range rows {
		require.NoError(t, w.Write(row))
	}
	w.Flush()
	require.NoError(t, w.Error())
	return buf.Bytes()
}

func TestPlayersParsesSavedFixture(t *testing.T) {
	t.Parallel()

	raw := readFixture(t, "players.csv")
	server := csvServer(t, "/players/players.csv", raw)

	players, err := nflverse.NewClient(server.URL, server.Client()).Players(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, players)

	byGSIS := make(map[string]nflverse.Player, len(players))
	for _, p := range players {
		assert.NotEmpty(t, p.GSISID)
		assert.NotEmpty(t, p.ESPNID)
		byGSIS[p.GSISID] = p
	}

	// Historical player without espn_id in fixture (Jon Abbate, 00-0024644) is dropped.
	_, hasAbbate := byGSIS["00-0024644"]
	assert.False(t, hasAbbate, "rows without espn_id are dropped")

	// Player with WAS in latest_team maps to ESPN's WSH (Isaako Aaitui, 00-0028830).
	aaitui, ok := byGSIS["00-0028830"]
	require.True(t, ok)
	assert.Equal(t, "14856", aaitui.ESPNID)
	assert.Equal(t, "AaitIs00", aaitui.PFRID)
	assert.Equal(t, "WSH", aaitui.LatestTeam)

	// Active player Israel Abanikanda.
	abanikanda, ok := byGSIS["00-0038389"]
	require.True(t, ok)
	assert.Equal(t, "4429202", abanikanda.ESPNID)
	assert.Equal(t, "AbanIs00", abanikanda.PFRID)
	assert.Equal(t, "DAL", abanikanda.LatestTeam)
	assert.Equal(t, "RB", abanikanda.Position)
}

func TestTeamStatsParsesSavedFixtureAndMapsAllTeams(t *testing.T) {
	t.Parallel()

	raw := readFixture(t, "stats_team_week_2026.csv")
	server := csvServer(t, "/stats_team/stats_team_week_2026.csv", raw)

	rows, err := nflverse.NewClient(server.URL, server.Client()).TeamStats(t.Context(), 2026)
	require.NoError(t, err)
	assert.Len(t, rows, 274)

	seenTeams := make(map[string]struct{}, len(espnTeamCodes))
	type key struct {
		week int
		team string
	}
	byKey := make(map[key]nflverse.TeamWeek, len(rows))
	for _, r := range rows {
		seenTeams[r.Team] = struct{}{}
		seenTeams[r.Opponent] = struct{}{}
		assert.Equal(t, 2026, r.Season)
		assert.Equal(t, nflverse.SeasonTypeRegular, r.SeasonType)
		byKey[key{week: r.Week, team: r.Team}] = r
	}

	assert.Len(t, seenTeams, len(espnTeamCodes), "fixture covers all 32 NFL teams")
	for code := range seenTeams {
		_, ok := espnTeamCodes[code]
		assert.True(t, ok, "team code %q is not an ESPN abbreviation", code)
	}

	// Verify opponent-allowed join for Week 1 ARI vs LAC.
	ari, ok := byKey[key{week: 1, team: "ARI"}]
	require.True(t, ok)
	lac, ok := byKey[key{week: 1, team: "LAC"}]
	require.True(t, ok)

	assert.Equal(t, "LAC", ari.Opponent)
	assert.Equal(t, "ARI", lac.Opponent)
	assert.InDelta(t, lac.PassingEPA, ari.OppPassingEPA, 1e-9)
	assert.InDelta(t, lac.RushingEPA, ari.OppRushingEPA, 1e-9)
	assert.Equal(t, lac.Attempts, ari.OppAttempts)
	assert.Equal(t, lac.Carries, ari.OppCarries)
	assert.Equal(t, lac.PassingYards, ari.OppPassingYards)
	assert.Equal(t, lac.RushingYards, ari.OppRushingYards)
}

func TestPlayerStatsParsesSavedFixture(t *testing.T) {
	t.Parallel()

	raw := readFixture(t, "stats_player_week_2026.csv")
	server := csvServer(t, "/stats_player/stats_player_week_2026.csv", raw)

	rows, err := nflverse.NewClient(server.URL, server.Client()).PlayerStats(t.Context(), 2026)
	require.NoError(t, err)
	// The fixture has 1,036 data rows; 1 row has an empty player_id and is dropped.
	assert.Len(t, rows, 1035)

	byID := make(map[string]nflverse.PlayerWeek, len(rows))
	for _, r := range rows {
		assert.NotEmpty(t, r.PlayerID)
		assert.NotEmpty(t, r.PlayerName)
		assert.NotEmpty(t, r.Position)
		assert.Equal(t, 2026, r.Season)
		assert.Equal(t, nflverse.SeasonTypeRegular, r.SeasonType)
		assert.Equal(t, 9, r.Week)
		_, ok := espnTeamCodes[r.Team]
		assert.True(t, ok, "team code %q is not an ESPN abbreviation", r.Team)
		_, ok = espnTeamCodes[r.Opponent]
		assert.True(t, ok, "opponent code %q is not an ESPN abbreviation", r.Opponent)
		byID[r.PlayerID] = r
	}

	stafford, ok := byID["00-0026498"]
	require.True(t, ok)
	assert.Equal(t, "Matthew Stafford", stafford.PlayerName)
	assert.Equal(t, "QB", stafford.Position)
	assert.Equal(t, "LAR", stafford.Team)
	assert.Equal(t, "WSH", stafford.Opponent)
	assert.Equal(t, 43, stafford.Attempts)
	assert.Equal(t, 228, stafford.PassingYards)
	assert.Equal(t, 1, stafford.PassingTDs)

	folk, ok := byID["00-0025565"]
	require.True(t, ok)
	assert.Equal(t, "Nick Folk", folk.PlayerName)
	assert.Equal(t, "K", folk.Position)
	assert.Equal(t, "ATL", folk.Team)
	assert.Equal(t, "CIN", folk.Opponent)
	assert.Equal(t, 1, folk.FGMade)
	assert.Equal(t, 2, folk.PATMade)
}

func TestSnapCountsParsesSavedFixture(t *testing.T) {
	t.Parallel()

	raw := readFixture(t, "snap_counts_2026.csv")
	server := csvServer(t, "/snap_counts/snap_counts_2026.csv", raw)

	rows, err := nflverse.NewClient(server.URL, server.Client()).SnapCounts(t.Context(), 2026)
	require.NoError(t, err)
	assert.Len(t, rows, 1401)

	for _, r := range rows {
		assert.NotEmpty(t, r.PFRPlayerID)
		assert.Equal(t, 2026, r.Season)
		assert.Equal(t, nflverse.SeasonTypeRegular, r.SeasonType)
		assert.Equal(t, 9, r.Week)
		_, ok := espnTeamCodes[r.Team]
		assert.True(t, ok, "team code %q is not an ESPN abbreviation", r.Team)
	}
}

func TestReorderedColumnsAndTeamMapping(t *testing.T) {
	t.Parallel()

	// Reverse the required column order for team stats and include LA/WAS.
	header := []string{
		"rushing_yards", "passing_yards", "carries", "attempts",
		"rushing_epa", "passing_epa", "opponent_team", "team",
		"season_type", "week", "season",
	}
	body := buildCSV(t, header,
		[]string{"110", "240", "28", "34", "1.5", "6.2", "LA", "WAS", "POST", "19", "2025"},
		[]string{"95", "280", "22", "38", "-0.5", "8.4", "WAS", "LA", "POST", "19", "2025"},
	)

	server := csvServer(t, "/stats_team/stats_team_week_2025.csv", body)
	rows, err := nflverse.NewClient(server.URL, server.Client()).TeamStats(t.Context(), 2025)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "WSH", rows[0].Team)
	assert.Equal(t, "LAR", rows[0].Opponent)
	assert.Equal(t, nflverse.SeasonTypePostseason, rows[0].SeasonType)
	assert.InDelta(t, 8.4, rows[0].OppPassingEPA, 1e-9)
}

func TestSnapCountsPostseasonGameTypes(t *testing.T) {
	t.Parallel()

	header := []string{"pfr_player_id", "player", "team", "season", "game_type", "week", "offense_snaps", "offense_pct"}
	body := buildCSV(t, header,
		[]string{"P1", "Player One", "WAS", "2025", "WC", "19", "50", "0.85"},
		[]string{"P2", "Player Two", "LA", "2025", "DIV", "20", "40", "0.75"},
		[]string{"P3", "Player Three", "OAK", "2025", "CON", "21", "60", "0.90"},
		[]string{"P4", "Player Four", "KC", "2025", "SB", "22", "65", "0.95"},
	)

	server := csvServer(t, "/snap_counts/snap_counts_2025.csv", body)
	rows, err := nflverse.NewClient(server.URL, server.Client()).SnapCounts(t.Context(), 2025)
	require.NoError(t, err)
	require.Len(t, rows, 4)

	assert.Equal(t, "WSH", rows[0].Team)
	assert.Equal(t, "LAR", rows[1].Team)
	assert.Equal(t, "LV", rows[2].Team)
	for _, r := range rows {
		assert.Equal(t, nflverse.SeasonTypePostseason, r.SeasonType)
	}
}

func TestParserErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing required column", func(t *testing.T) {
		t.Parallel()
		body := buildCSV(t, []string{"season", "week", "team"}, []string{"2026", "1", "BUF"})
		server := csvServer(t, "/stats_team/stats_team_week_2026.csv", body)
		_, err := nflverse.NewClient(server.URL, server.Client()).TeamStats(t.Context(), 2026)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required columns")
		assert.Contains(t, err.Error(), "opponent_team")
	})

	t.Run("bad numeric cell names row and column", func(t *testing.T) {
		t.Parallel()
		header := []string{
			"pfr_player_id",
			"player",
			"team",
			"season",
			"game_type",
			"week",
			"offense_snaps",
			"offense_pct",
		}
		body := buildCSV(t, header,
			[]string{"P1", "Player", "BUF", "2026", "REG", "1", "not-a-number", "0.5"},
		)
		server := csvServer(t, "/snap_counts/snap_counts_2026.csv", body)
		_, err := nflverse.NewClient(server.URL, server.Client()).SnapCounts(t.Context(), 2026)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "row 2")
		assert.Contains(t, err.Error(), "offense_snaps")
	})

	t.Run("unknown season_type errors", func(t *testing.T) {
		t.Parallel()
		header := []string{
			"pfr_player_id",
			"player",
			"team",
			"season",
			"game_type",
			"week",
			"offense_snaps",
			"offense_pct",
		}
		body := buildCSV(t, header,
			[]string{"P1", "Player", "BUF", "2026", "EXHIBITION", "1", "10", "0.5"},
		)
		server := csvServer(t, "/snap_counts/snap_counts_2026.csv", body)
		_, err := nflverse.NewClient(server.URL, server.Client()).SnapCounts(t.Context(), 2026)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown season type")
	})

	t.Run("empty optional cells default to zero", func(t *testing.T) {
		t.Parallel()
		header := []string{
			"player_id", "player_name", "player_display_name", "position",
			"team", "opponent_team", "season", "week", "season_type",
			"target_share", "air_yards_share", "wopr",
			"passing_epa", "rushing_epa", "receiving_epa",
			"targets", "carries", "attempts", "receptions",
			"receiving_yards", "rushing_yards", "passing_yards",
			"passing_tds", "rushing_tds", "receiving_tds",
			"fg_made", "pat_made",
		}
		body := buildCSV(
			t,
			header,
			[]string{
				"00-1",
				"J. Allen",
				"",
				"QB",
				"BUF",
				"WAS",
				"2026",
				"1",
				"REG",
				"",
				"NA",
				"",
				"1.25",
				"",
				"",
				"",
				"",
				"30",
				"",
				"",
				"",
				"200",
				"2",
				"",
				"",
				"",
				"",
			},
			[]string{
				"00-2", "P. Ricard", "Patrick Ricard", "FB", "BAL", "LA", "2026", "1", "REG",
				"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
			},
			[]string{
				"", "", "", "", "BUF", "KC", "2026", "1", "REG",
				"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
			},
		)
		server := csvServer(t, "/stats_player/stats_player_week_2026.csv", body)
		rows, err := nflverse.NewClient(server.URL, server.Client()).PlayerStats(t.Context(), 2026)
		require.NoError(t, err)
		require.Len(t, rows, 2, "row with empty player_id is dropped")
		assert.Equal(t, "J. Allen", rows[0].PlayerName, "falls back to player_name when player_display_name is empty")
		assert.Equal(t, "QB", rows[0].Position)
		assert.Equal(t, "WSH", rows[0].Opponent)
		assert.Equal(t, 30, rows[0].Attempts)
		assert.InDelta(t, 1.25, rows[0].PassingEPA, 1e-9)
		assert.Zero(t, rows[0].ReceivingEPA)
		assert.Zero(t, rows[0].AirYardsShare)
		assert.Equal(t, 200, rows[0].PassingYards)
		assert.Equal(t, 2, rows[0].PassingTDs)

		assert.Equal(t, "Patrick Ricard", rows[1].PlayerName)
		assert.Equal(t, "RB", rows[1].Position, "FB normalizes to RB")
		assert.Equal(t, "LAR", rows[1].Opponent)
	})

	t.Run("http error and invalid season", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(server.Close)

		c := nflverse.NewClient(server.URL, server.Client())
		_, err := c.Players(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "404")

		_, err = c.TeamStats(t.Context(), 0)
		require.Error(t, err)
		_, err = c.PlayerStats(t.Context(), -1)
		require.Error(t, err)
		_, err = c.SnapCounts(t.Context(), 0)
		require.Error(t, err)
	})
}
