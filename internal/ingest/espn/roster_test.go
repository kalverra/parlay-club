package espn_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/ingest/espn"
)

func TestRosterFromFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "roster.json"))
	require.NoError(t, err)

	requestedPath := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		assert.Equal(t, "parlay-club", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	players, err := client.Roster(t.Context(), "2")
	require.NoError(t, err)

	assert.Equal(t, "/teams/2/roster", requestedPath)
	assert.NotEmpty(t, players)

	var joshAllen *espn.RosterPlayer
	var tylerBass *espn.RosterPlayer
	for i := range players {
		if players[i].ID == "3918298" {
			joshAllen = &players[i]
		}
		if players[i].ID == "3917232" {
			tylerBass = &players[i]
		}
	}

	require.NotNil(t, joshAllen, "Josh Allen should be on the Bills roster")
	assert.Equal(t, "Josh Allen", joshAllen.FullName)
	assert.Equal(t, "QB", joshAllen.Position)
	assert.Equal(t, "https://a.espncdn.com/i/headshots/nfl/players/full/3918298.png", joshAllen.HeadshotURL)

	require.NotNil(t, tylerBass, "Tyler Bass should be on the Bills roster")
	assert.Equal(t, "Tyler Bass", tylerBass.FullName)
	assert.Equal(t, "PK", tylerBass.Position)
	assert.Equal(t, "https://a.espncdn.com/i/headshots/nfl/players/full/3917232.png", tylerBass.HeadshotURL)
}

func TestRosterSkipsEmptyAthletes(t *testing.T) {
	t.Parallel()

	fixture := `{
		"athletes": [
			{
				"position": "offense",
				"items": [
					{"id": "1", "fullName": "Valid Player", "position": {"abbreviation": "QB"}},
					{"id": "", "fullName": "No ID", "position": {"abbreviation": "RB"}},
					{"id": "2", "fullName": "", "position": {"abbreviation": "WR"}},
					{"id": "3", "fullName": "No Position"}
				]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	players, err := client.Roster(t.Context(), "10")
	require.NoError(t, err)

	require.Len(t, players, 2)
	assert.Equal(t, espn.RosterPlayer{ID: "1", FullName: "Valid Player", Position: "QB"}, players[0])
	assert.Equal(t, espn.RosterPlayer{ID: "3", FullName: "No Position", Position: ""}, players[1])
}

func TestRosterMissingAndInvalidHeadshots(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "roster_falcons.json"))
	require.NoError(t, err)

	falconsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(falconsServer.Close)

	players, err := espn.NewClient(falconsServer.URL, falconsServer.Client()).Roster(t.Context(), "1")
	require.NoError(t, err)
	require.NotEmpty(t, players)

	var missingPlayer *espn.RosterPlayer
	hasHeadshot := false
	for i := range players {
		if players[i].ID == "5344782" {
			missingPlayer = &players[i]
		}
		if players[i].HeadshotURL != "" {
			hasHeadshot = true
		}
	}
	require.NotNil(t, missingPlayer, "Jack Strand should be in roster_falcons.json")
	assert.Empty(t, missingPlayer.HeadshotURL)
	assert.True(t, hasHeadshot)

	fixture := `{
		"athletes": [
			{
				"items": [
					{"id": "1", "fullName": "HTTPS Padded", "position": {"abbreviation": "QB"}, "headshot": {"href": "  https://a.espncdn.com/1.png  "}},
					{"id": "2", "fullName": "HTTP Rejected", "position": {"abbreviation": "RB"}, "headshot": {"href": "http://a.espncdn.com/2.png"}},
					{"id": "3", "fullName": "Relative Rejected", "position": {"abbreviation": "WR"}, "headshot": {"href": "/3.png"}},
					{"id": "4", "fullName": "Empty Host", "position": {"abbreviation": "TE"}, "headshot": {"href": "https://"}}
				]
			}
		]
	}`
	normServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(normServer.Close)

	normPlayers, err := espn.NewClient(normServer.URL, normServer.Client()).Roster(t.Context(), "2")
	require.NoError(t, err)
	require.Len(t, normPlayers, 4)
	assert.Equal(t, "https://a.espncdn.com/1.png", normPlayers[0].HeadshotURL)
	assert.Empty(t, normPlayers[1].HeadshotURL)
	assert.Empty(t, normPlayers[2].HeadshotURL)
	assert.Empty(t, normPlayers[3].HeadshotURL)
}

func TestRosterError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := espn.NewClient(server.URL, server.Client())
	_, err := client.Roster(t.Context(), "2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestRosterInjuriesFromFixtures(t *testing.T) {
	t.Parallel()

	loadRoster := func(t *testing.T, raw []byte) map[string]espn.RosterPlayer {
		t.Helper()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
		}))
		t.Cleanup(server.Close)

		players, err := espn.NewClient(server.URL, server.Client()).Roster(t.Context(), "1")
		require.NoError(t, err)

		byID := make(map[string]espn.RosterPlayer, len(players))
		for _, p := range players {
			byID[p.ID] = p
		}
		return byID
	}

	billsRaw, err := os.ReadFile(filepath.Join("testdata", "roster.json"))
	require.NoError(t, err)
	bills := loadRoster(t, billsRaw)
	assert.Empty(t, bills["3918298"].InjuryStatus) // Josh Allen (healthy)
	assert.Empty(t, bills["3918298"].InjuryDate)
	assert.Equal(t, "Out", bills["4242433"].InjuryStatus) // Joshua Palmer
	assert.Equal(t, "2026-11-07T18:51Z", bills["4242433"].InjuryDate)
	assert.Equal(t, "Questionable", bills["3051324"].InjuryStatus) // Dion Dawkins
	assert.Equal(t, "2026-11-08T02:01Z", bills["3051324"].InjuryDate)
	assert.Equal(t, "Injured Reserve", bills["4685347"].InjuryStatus) // Zane Durant
	assert.Equal(t, "2026-09-08T20:42Z", bills["4685347"].InjuryDate)

	bengalsRaw, err := os.ReadFile(filepath.Join("testdata", "roster_bengals.json"))
	require.NoError(t, err)
	bengals := loadRoster(t, bengalsRaw)
	assert.Equal(t, "Out", bengals["4819215"].InjuryStatus) // Josh Kattus
	assert.Equal(t, "2026-10-27T23:24Z", bengals["4819215"].InjuryDate)
	assert.Equal(t, "Questionable", bengals["4401811"].InjuryStatus) // Kyle Dugger
	assert.Equal(t, "2026-11-10T15:44Z", bengals["4401811"].InjuryDate)
	assert.Equal(t, "Injured Reserve", bengals["4368003"].InjuryStatus) // Andrei Iosivas
	assert.Equal(t, "2026-11-09T19:43Z", bengals["4368003"].InjuryDate)

	falconsRaw, err := os.ReadFile(filepath.Join("testdata", "roster_falcons.json"))
	require.NoError(t, err)
	falcons := loadRoster(t, falconsRaw)
	assert.Equal(t, "Doubtful", falcons["4870612"].InjuryStatus) // Zachariah Branch
	assert.Equal(t, "2026-11-10T15:44Z", falcons["4870612"].InjuryDate)
	assert.Equal(t, "Out", falcons["4243457"].InjuryStatus) // DeAngelo Malone
	assert.Equal(t, "2026-08-04T22:34Z", falcons["4243457"].InjuryDate)
	assert.Equal(t, "Injured Reserve", falcons["4241474"].InjuryStatus) // Brian Robinson Jr.
	assert.Equal(t, "2026-11-07T21:13Z", falcons["4241474"].InjuryDate)
}

func TestRosterInjuryPriority(t *testing.T) {
	t.Parallel()

	fixture := `{
		"athletes": [
			{
				"items": [
					{
						"id": "1",
						"fullName": "Out Beats Questionable",
						"position": {"abbreviation": "WR"},
						"injuries": [
							{"status": "Questionable", "date": "2026-11-01T10:00Z"},
							{"status": "Out", "date": "2026-11-02T10:00Z"}
						]
					},
					{
						"id": "2",
						"fullName": "Doubtful Beats Questionable",
						"position": {"abbreviation": "TE"},
						"injuries": [
							{"status": "Questionable", "date": "2026-11-01T10:00Z"},
							{"status": "Doubtful", "date": "2026-11-02T10:00Z"}
						]
					},
					{
						"id": "3",
						"fullName": "Questionable Beats IR",
						"position": {"abbreviation": "RB"},
						"injuries": [
							{"status": "Injured Reserve", "date": "2026-09-01T10:00Z"},
							{"status": "Questionable", "date": "2026-11-02T10:00Z"}
						]
					},
					{
						"id": "4",
						"fullName": "IR Beats Unknown",
						"position": {"abbreviation": "QB"},
						"injuries": [
							{"status": "Day-to-Day", "date": "2026-11-01T10:00Z"},
							{"status": "Injured Reserve", "date": "2026-09-01T10:00Z"}
						]
					},
					{
						"id": "5",
						"fullName": "Skips Empty Status",
						"position": {"abbreviation": "WR"},
						"injuries": [
							{"status": "", "date": "2026-11-01T10:00Z"},
							{"status": "Day-to-Day", "date": "2026-11-02T10:00Z"}
						]
					}
				]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	t.Cleanup(server.Close)

	players, err := espn.NewClient(server.URL, server.Client()).Roster(t.Context(), "1")
	require.NoError(t, err)
	require.Len(t, players, 5)

	assert.Equal(t, "Out", players[0].InjuryStatus)
	assert.Equal(t, "2026-11-02T10:00Z", players[0].InjuryDate)
	assert.Equal(t, "Doubtful", players[1].InjuryStatus)
	assert.Equal(t, "2026-11-02T10:00Z", players[1].InjuryDate)
	assert.Equal(t, "Questionable", players[2].InjuryStatus)
	assert.Equal(t, "2026-11-02T10:00Z", players[2].InjuryDate)
	assert.Equal(t, "Injured Reserve", players[3].InjuryStatus)
	assert.Equal(t, "2026-09-01T10:00Z", players[3].InjuryDate)
	assert.Equal(t, "Day-to-Day", players[4].InjuryStatus)
	assert.Equal(t, "2026-11-02T10:00Z", players[4].InjuryDate)
}
